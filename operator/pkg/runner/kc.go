package runner

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coreybutler/go-fsutil"
	"github.com/pieterclaerhout/go-waitgroup"
	"go.uber.org/zap"

	"github.com/mauricioscastro/hcreport/pkg/kc"
	"github.com/mauricioscastro/hcreport/pkg/yjq"
)

type KcCmdRunner interface {
	PipeCmdRunner
	KcVersion() string
	KcGet(api string) CmdRunner
	KcGetWithParams(api string, queryParams map[string]string) CmdRunner
	KcGetAsync(apis []string, separator string, poolSize int, queryParams map[string]string) CmdRunner
	KcApiResources() CmdRunner
	KcNs() CmdRunner
	KcDump(path string, poolSize int, progress func()) CmdRunner
	initKc()
}

var DefaultCleaningYqQuery = `del(.metadata) | with(.items[].metadata; del(.uid) | del(.resourceVersion) | del(.creationTimestamp) | del(.annotations["kubectl.kubernetes.io/last-applied-configuration"]) | del(.managedFields))`

// Kube Get resource
func (r *runner) KcGet(api string) CmdRunner {
	r.initKc()
	if r.err == nil {
		o, e := r.kc.Get(api)
		if e == nil {
			r.write(o)
		}
		r.error(e, "kcGet api="+api)
	}
	return r
}

func (r *runner) KcGetWithParams(api string, queryParams map[string]string) CmdRunner {
	if queryParams != nil {
		r.initKc()
		r.kc.SetGetParams(queryParams)
	}
	return r.KcGet(api)
}

// Kube Get resource list asynchronously adding a separator
// between each response with a worker pool size
func (r *runner) KcGetAsync(apis []string, separator string, poolSize int, queryParams map[string]string) CmdRunner {
	kcList := make([]kc.Kc, len(apis))
	wg := waitgroup.NewWaitGroup(poolSize)
	for i, api := range apis {
		_kc := kc.NewKc()
		if queryParams != nil {
			_kc.SetGetParams(queryParams)
		}
		kcList[i] = _kc
		wg.BlockAdd()
		go func(api string) {
			defer wg.Done()
			_kc.Get(api)
		}(api)
	}
	wg.Wait()
	var sb strings.Builder
	for i, v := range kcList {
		sb.WriteString(v.Response())
		if len(separator) > 0 && i+1 < len(kcList) {
			sb.WriteString(separator)
		}
	}
	return r.Echo(sb.String())
}

func (r *runner) KcApiResources() CmdRunner {
	wr := NewCmdRunner()
	apiList :=
		wr.KcGet("/apis").
			Yq(`.groups[].preferredVersion.groupVersion`).
			Sed("s;^;/apis/;g").
			Append().Echo("\n/api/" + wr.KcVersion()).
			List()
	if r.err = wr.Err(); r.err != nil {
		return r
	}
	kcList := make([]kc.Kc, len(apiList))
	wg := waitgroup.NewWaitGroup(0)
	for i, api := range apiList {
		_kc := kc.NewKc()
		kcList[i] = _kc
		wg.BlockAdd()
		go func(api string) {
			defer wg.Done()
			_kc.GetTransform(api, func(kcApi string, kcResp string, kcErr error) (string, error) {
				if kcErr != nil {
					kcResp = "- groupVersion: " + strings.TrimPrefix(kcApi, "/apis/") + "\n  available: false"
					kcErr = nil
				} else {
					e := `.resources[] += {"groupVersion": .groupVersion} | .resources[] += {"available": true} | .resources[] | select(.singularName != "") | del(.storageVersionHash) | del(.singularName) | [.]`
					kcResp, kcErr = yjq.YqEval(e, kcResp)
				}
				return kcResp, kcErr
			})
		}(api)
	}
	wg.Wait()
	var sb strings.Builder
	for i, _kc := range kcList {
		sb.WriteString(_kc.Response())
		if i+1 < len(apiList) {
			sb.WriteString("\n")
		}
	}
	return r.Echo("kind: APIResourceList\nresources:\n").Append().Echo(sb.String())
}

func (r *runner) KcNs() CmdRunner {
	if r.err != nil {
		return r
	}
	wr := NewCmdRunner()
	return r.Copy(wr.KcGet("/api/" + wr.KcVersion() + "/namespaces").
		Yq(DefaultCleaningYqQuery))
}

func (r *runner) KcVersion() string {
	r.initKc()
	if r.kc.Version() == "" {
		r.error(errors.New("kc client has empty version"))
	}
	return r.kc.Version()
}

func (r *runner) KcDump(path string, poolSize int, progress func()) CmdRunner {
	fsutil.Clean(path)
	path = path + "/"
	R().KcGet("/version").WriteFile(path + "version.yaml")
	nsList := R().
		KcNs().
		WriteFile(path + "namespaces_" + r.KcVersion() + ".yaml").
		Yq(".items[].metadata.name").
		List()
	for _, ns := range nsList {
		R().MkDir(path + strings.ReplaceAll(ns, "-", "_") + "/log")
	}
	apiList := R().
		KcApiResources().
		WriteFile(path + "api_resources.yaml").
		Yq(`with(.resources[]; .verbs = (.verbs | to_entries)) | .resources[] | select(.available and .verbs[].value == "get") | .name + ";" + .groupVersion + ";" + .namespaced`).
		List()
	wg := waitgroup.NewWaitGroup(poolSize)
	for _, le := range apiList {
		_le := strings.Split(le, ";")
		name := _le[0]
		gv := _le[1]
		if name == "namespaces" && gv == r.KcVersion() {
			continue
		}
		namespaced, _ := strconv.ParseBool(_le[2])
		baseName := "/apis/"
		if gv == r.KcVersion() {
			baseName = "/api/"
		}
		logger.Debug("KcDump", zap.String("baseName", baseName), zap.String("name", name), zap.Bool("namespaced", namespaced), zap.String("gv", gv))
		wg.BlockAdd()
		go func() {
			defer wg.Done()
			writeResourceList(path, baseName, name, gv, namespaced, progress)
		}()
	}
	wg.Wait()
	return r
}

func writeResourceList(path string, baseName string, name string, gv string, namespaced bool, progress func()) {
	fileName := name + "_" + strings.ReplaceAll(gv, "/", "_")
	fileName = strings.ReplaceAll(fileName, ".", "_") + ".yaml"
	r := R().KcGet(baseName + gv + "/" + name).Yq(DefaultCleaningYqQuery)
	if name == "secrets" {
		r.Yq(`.items[].data.[] = ""`)
	}
	if !namespaced {
		r.WriteFile(path + fileName)
	} else {
		for _, ns := range r.Clone().Yq("[.items[].metadata.namespace] | unique | .[]").List() {
			nsPath := path + strings.ReplaceAll(ns, "-", "_") + "/"
			nsr := r.Clone().
				Yq(`.items = [.items[] | select(.metadata.namespace=="` + ns + `")]`).
				WriteFile(nsPath + fileName)
			if name == "pods" {
				podNameContainerExpr := `.items[].metadata.name + ";" + .items[].spec.containers[].name`
				for _, p := range nsr.Yq(podNameContainerExpr).List() {
					_p := strings.Split(p, ";")
					podName := _p[0]
					containerName := _p[1]
					fileName := podName + "-" + containerName + ".log"
					qp := map[string]string{"container": containerName}
					apiFormat := "%s%s/namespaces/%s/pods/%s/log"
					R().KcGetWithParams(fmt.Sprintf(apiFormat, baseName, gv, ns, podName), qp).
						WriteFile(nsPath + "log/" + fileName)
				}
			}
		}
	}
	if progress != nil {
		progress()
	}
}

func (r *runner) initKc() {
	if r.kc == nil {
		r.kc = kc.NewKc()
		logger.Debug("kcVersion=" + r.kc.Version())
	}
}
