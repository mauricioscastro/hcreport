package kc

import (
	gz "compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/coreybutler/go-fsutil"
	"github.com/mauricioscastro/hcreport/pkg/yjq"
	"github.com/pieterclaerhout/go-waitgroup"
	"go.uber.org/zap"
)

const (
	YAML               int = 0
	JSON               int = 1
	JSON_LINES         int = 2
	JSON_LINES_WRAPPED int = 3
)

var (
	DefaultCleaningQuery = `.items = [.items[] | del(.metadata.managedFields) | del(.metadata.uid) | del (.metadata.creationTimestamp) | del (.metadata.generation) | del(.metadata.resourceVersion) | del (.metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"])] | del(.metadata)`
	dumpWorkerErrors     atomic.Value
)

func Ns() (string, error) {
	kc := NewKc()
	r, e := kc.GetJson("/api/" + kc.Version() + "/namespaces")
	if e != nil {
		return r, e
	}
	r, e = yjq.YqEvalJ2Y(DefaultCleaningQuery, r)
	if e != nil {
		return r, e
	}
	return r, e
}

func ApiResources() (string, error) {
	kc := NewKc()
	r, e := kc.GetJson("/apis")
	if e != nil {
		return "", e
	}
	apisList, e := yjq.Eval2List(yjq.JqEval, `["/api/v1", "/apis/" + .groups[].preferredVersion.groupVersion] | .[]`, r)
	if e != nil {
		return "", e
	}
	var kcList []Kc
	wg := waitgroup.NewWaitGroup(0)
	for _, api := range apisList {
		_kc := NewKc()
		wg.BlockAdd()
		go func(api string) {
			defer wg.Done()
			_kc.SetResponseTransformer(apiResourcesResponseTransformer).
				GetJson(api)
		}(api)
		kcList = append(kcList, _kc)
	}
	wg.Wait()
	var sb strings.Builder
	sb.WriteString("kind: APIResourceList\nitems:\n")
	for i, _kc := range kcList {
		if len(_kc.Response()) == 0 || _kc.Err() != nil {
			continue
		}
		sb.WriteString(_kc.Response())
		if i+1 < len(kcList) {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

func apiResourcesResponseTransformer(kc Kc) (string, error) {
	var resp string
	err := kc.Err()
	logger.Debug("transforming resource " + kc.Api())
	unavailableApiResp := `[{"groupVersion": "%s", "available": false, "error": "%s"}]`
	api := strings.TrimPrefix(kc.Api(), "/apis/")
	if err != nil {
		logger.Warn("ApiResources bad api", zap.String("api", kc.Api()), zap.Error(err))
		if resp, err = yjq.JqEval2Y(unavailableApiResp, "", api, err.Error()); err != nil {
			logger.Error("ApiResources bad api JqEval2Y", zap.Error(err))
		}
	} else {
		resp, err = yjq.JqEval2Y(`.resources[] += {"groupVersion": .groupVersion} | .resources[] += {"available": true} | [.resources[] | select(.name | contains("/") | not) | del(.storageVersionHash) | del(.singularName)]`, kc.Response())
		if err != nil {
			logger.Warn("ApiResources JqEval2Y", zap.String("api", kc.Api()), zap.Error(err))
			resp, err = yjq.JqEval2Y(unavailableApiResp, "", api, err.Error())
		} else if resp == "[]" {
			logger.Warn("ApiResources empty reply from api", zap.String("api", kc.Api()))
			resp, err = yjq.JqEval2Y(unavailableApiResp, "", api, "empty reply from api")
		}
	}
	return resp, err
}

// Dumps whole cluster information to path. each api resource listed with
// KcApiResources is treated in a separate thread. The pool size for the
// the threads can be expressed through poolsize (0 or -1 to unbound it).
// progress will be called at the end. You need to add thread safety mechanisms
// to the code inside progress func().
func Dump(path string, nsExclusionList []string, gvkExclusionList []string, nologs bool, gz bool, format int, poolSize int, progress func()) error {
	kc := NewKc()
	fsutil.Clean(filepath.FromSlash(path))
	path = path + "/"
	ns, err := Ns()
	if err != nil {
		return err
	}
	// filter out ns based on regex list
	for _, re := range nsExclusionList {
		ns, err = yjq.YqEval(`del(.items[] | select(.metadata.name | test("%s")))`, ns, re)
		if err != nil {
			return err
		}
	}
	if err = writeTextFile(path+"namespaces_"+kc.Version()+".yaml", ns, gz, format); err != nil {
		return err
	}
	nsList, err := yjq.Eval2List(yjq.YqEval, ".items[].metadata.name", ns)
	if err != nil {
		return err
	}
	for _, n := range nsList {
		fsutil.Mkdirp(filepath.FromSlash(path + strings.ReplaceAll(n, "-", "_") + "/log"))
	}
	apis, err := ApiResources()
	if err != nil {
		return err
	}
	// filter out gvk based on regex list
	for _, re := range gvkExclusionList {
		r := strings.Split(re, ",")
		if len(r) == 1 {
			r = append(r, ".*")
		}
		if len(r[0]) == 0 {
			r[0] = ".*"
		}
		if len(r[1]) == 0 {
			r[1] = ".*"
		}
		apis, err = yjq.YqEval(`del(.items[] | select(.groupVersion | test("%s") and .kind | test("%s")))`, apis, r[0], r[1])
		if err != nil {
			return err
		}
	}
	if err = writeTextFile(path+"api_resources.yaml", apis, gz, format); err != nil {
		return err
	}
	apiList, err := yjq.Eval2List(yjq.YqEval, `with(.items[]; .verbs = (.verbs | to_entries)) | .items[] | select(.available and .verbs[].value == "get") | .name + ";" + .groupVersion + ";" + .namespaced`, apis)
	if err != nil {
		return err
	}
	dumpWorkerErrors.Store(make([]error, 0))
	wg := waitgroup.NewWaitGroup(poolSize)
	for _, le := range apiList {
		_le := strings.Split(le, ";")
		name := _le[0]
		gv := _le[1]
		if name == "namespaces" && gv == kc.Version() {
			continue
		}
		namespaced, _ := strconv.ParseBool(_le[2])
		baseName := "/apis/"
		if gv == kc.Version() {
			baseName = "/api/"
		}
		logger.Debug("KcDump", zap.String("baseName", baseName), zap.String("name", name), zap.Bool("namespaced", namespaced), zap.String("gv", gv))
		wg.BlockAdd()
		go func() {
			defer wg.Done()
			writeResourceList(path, baseName, name, gv, namespaced, nologs, gz, format, progress)
		}()
	}
	wg.Wait()
	version, err := kc.Get("/version")
	if err != nil {
		return err
	}
	if err = writeTextFile(path+"version.yaml", version, gz, format); err != nil {
		return err
	}
	if len(dumpWorkerErrors.Load().([]error)) > 0 {
		var collectedErrors strings.Builder
		for _, e := range dumpWorkerErrors.Load().([]error) {
			collectedErrors.WriteString(e.Error())
			collectedErrors.WriteString("\n")
		}
		return errors.New(collectedErrors.String())
	}
	return nil
}

func writeResourceList(path string, baseName string, name string, gv string, namespaced bool, nologs bool, gz bool, format int, progress func()) error {
	if progress != nil {
		defer progress()
	}
	logLine := fmt.Sprintf("baseName=%s name=%s namespaced=%s gv=%t", baseName, name, gv, namespaced)
	kc := NewKc()
	fileName := name + "_" + strings.ReplaceAll(gv, "/", "_")
	fileName = strings.ReplaceAll(fileName, ".", "_") + ".yaml"
	apiResources, err := kc.
		SetResponseTransformer(apiIgnoreNotFoundResponseTransformer).
		Get(baseName + gv + "/" + name)
	if err != nil {
		return writeResourceListLog("get "+logLine, err)
	}
	if len(apiResources) == 0 {
		return writeResourceListLog("empty "+logLine, errors.New("empty list"))
	}
	apiResources, err = yjq.YqEval(DefaultCleaningQuery, apiResources)
	if err != nil {
		return writeResourceListLog("DefaultCleaningQuery "+logLine, err)
	}
	if name == "secrets" {
		apiResources, err = yjq.YqEval(`.items[].data.[] = ""`, apiResources)
		if err != nil {
			return writeResourceListLog("secrets "+logLine, err)
		}
	}
	if !namespaced {
		if err = writeTextFile(path+fileName, apiResources, gz, format); err != nil {
			return writeResourceListLog("write resource "+logLine, err)
		}
	} else {
		nsList, err := yjq.Eval2List(yjq.YqEval, "[.items[].metadata.namespace] | unique | .[]", apiResources)
		if err != nil {
			return writeResourceListLog("read ns list from apiResources"+logLine, err)
		}
		for _, ns := range nsList {
			nsPath := path + strings.ReplaceAll(ns, "-", "_") + "/"
			// skip unwanted ns as per nsExclusionList in kcDump
			if !fsutil.Exists(filepath.FromSlash(nsPath)) {
				continue
			}
			apiByNs, err := yjq.YqEval(`.items = [.items[] | select(.metadata.namespace=="%s")]`, apiResources, ns)
			if err != nil {
				return writeResourceListLog("apiByNs "+logLine, err)
			}
			if err = writeTextFile(nsPath+fileName, apiByNs, gz, format); err != nil {
				return writeResourceListLog("write resource "+logLine, err)
			}
			// get pods' logs or no
			if !nologs && name == "pods" && gv == kc.Version() {
				podContainerList, err := yjq.Eval2List(yjq.YqEval, `.items[] | .metadata.name + ";" + .spec.containers[].name`, apiByNs)
				if err != nil {
					return writeResourceListLog("podContainerList "+logLine, err)
				}
				for _, p := range podContainerList {
					_p := strings.Split(p, ";")
					podName := _p[0]
					containerName := _p[1]
					fileName := podName + "-" + containerName + ".log"
					qp := map[string]string{"container": containerName}
					logApi := fmt.Sprintf("%s%s/namespaces/%s/pods/%s/log", baseName, gv, ns, podName)
					log, err := kc.
						SetResponseTransformer(apiIgnoreNotFoundResponseTransformer).
						SetGetParams(qp).
						Get(logApi)
					if err != nil {
						return writeResourceListLog("get pod log "+logLine, err)
					}
					if len(log) > 0 {
						if err = writeTextFile(nsPath+"log/"+fileName, log, gz, format); err != nil {
							return writeResourceListLog("writing pod log "+logLine, err)
						}
					}
				}
			}
		}
	}
	return nil
}

func apiIgnoreNotFoundResponseTransformer(kc Kc) (string, error) {
	if kc.Status() == http.StatusNotFound || kc.Status() == http.StatusMethodNotAllowed {
		return "", nil
	}
	return kc.Response(), kc.Err()
}

func writeResourceListLog(msg string, err error) error {
	dumpWorkerErrors.Store(append(dumpWorkerErrors.Load().([]error), fmt.Errorf("%s %w", msg, err)))
	logger.Error("writeResourceList "+msg, zap.Error(err))
	return err
}

func writeTextFile(path string, contents string, gz bool, format int) error {
	var err error
	switch format {
	case YAML:
		path = rewriteFileSuffix(path, "yaml")
	case JSON:
		if contents, err = yjq.Y2JC(contents); err != nil {
			return err
		}
		path = rewriteFileSuffix(path, "json")
	case JSON_LINES:
		if contents, err = yjq.YqEval2JC(".items[]", contents); err != nil {
			return err
		}
		path = rewriteFileSuffix(path, "json")
	case JSON_LINES_WRAPPED:
		if contents, err = yjq.YqEval2JC(`{"_": .items[]}`, contents); err != nil {
			return err
		}
		path = rewriteFileSuffix(path, "json")
	default:
		return fmt.Errorf("writeTextFile unknown output format requested")
	}
	if err = fsutil.WriteTextFile(path, contents); err != nil {
		return err
	}
	if gz {
		return gzip(path)
	}
	return nil
}

func gzip(file string) error {
	originalFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer originalFile.Close()

	gzippedFile, err := os.Create(file + ".gz")
	if err != nil {
		return err
	}
	defer gzippedFile.Close()

	gzipWriter := gz.NewWriter(gzippedFile)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, originalFile)
	if err != nil {
		return err
	}

	gzipWriter.Flush()
	os.Remove(file)
	return nil
}

func FormatCodeFromString(format string) (int, error) {
	switch format {
	case "yaml":
		return YAML, nil
	case "json":
		return JSON, nil
	case "json_lines":
		return JSON_LINES, nil
	case "json_lines_wrapped":
		return JSON_LINES_WRAPPED, nil
	default:
		return -1, fmt.Errorf("unknown string format %s", format)
	}
}

func rewriteFileSuffix(path string, suffix string) string {
	d := filepath.Dir(path)
	f := filepath.Base(path)
	p := strings.Split(f, ".")
	if len(p) == 1 {
		p = append(p, suffix)
	} else {
		p[len(p)-1] = suffix
	}
	f = strings.Join(p, ".")
	return filepath.FromSlash(d + "/" + f)
}
