package kc

import (
	"archive/tar"
	gz "compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	JSON_PRETTY        int = 4
)

var (
	DefaultCleaningQuery  = `.items = [.items[] | del(.metadata.managedFields) | del(.metadata.uid) | del (.metadata.creationTimestamp) | del (.metadata.generation) | del(.metadata.resourceVersion) | del (.metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"])] | del(.metadata)`
	apiAvailableListQuery = `with(.items[]; .verbs = (.verbs | to_entries)) | .items[] | select(.available and .verbs[].value == "get") | .name + ";" + .groupVersion + ";" + .namespaced`
	dumpWorkerErrors      atomic.Value
)

func (kc *kc) Ns() (string, error) {
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

func (kc *kc) ApiResources() (string, error) {
	r, e := kc.GetJson("/apis")
	if e != nil {
		return "", e
	}
	apisList, e := yjq.Eval2List(yjq.JqEval, `["/api/%s", "/apis/" + .groups[].preferredVersion.groupVersion] | .[]`, r, kc.Version())
	if e != nil {
		return "", e
	}
	var kcList []Kc
	var wg sync.WaitGroup
	for _, api := range apisList {
		_kc := NewKc()
		wg.Add(1)
		go func(api string) {
			defer wg.Done()
			_kc.SetResponseTransformer(apiResourcesResponseTransformer).GetJson(api)
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
func (kc *kc) Dump(path string, nsExclusionList []string, gvkExclusionList []string, nologs bool, gz bool, tgz bool, format int, poolSize int, progress func()) error {
	if tgz {
		gz = false
	}
	dumpDir := strings.Replace(kc.cluster, "https://", "", -1)
	dumpDir = strings.Replace(dumpDir, ":", ".", -1)
	path = path + "/" + dumpDir
	fsutil.Clean(filepath.FromSlash(path))
	path = path + "/"
	ns, err := kc.Ns()
	if err != nil {
		return err
	}
	// filter out ns based on regex list
	if ns, err = FilterNS(ns, nsExclusionList); err != nil {
		return err
	}
	if err = writeTextFile(path+"namespaces_"+kc.Version()+".yaml", ns, gz, format); err != nil {
		return err
	}
	nsList, err := yjq.Eval2List(yjq.YqEval, ".items[].metadata.name", ns)
	if err != nil {
		return err
	}
	for _, n := range nsList {
		ns_dir_name := strings.ReplaceAll(n, "-", "_")
		fsutil.Mkdirp(filepath.FromSlash(path + ns_dir_name))
		if !nologs {
			fsutil.Mkdirp(filepath.FromSlash(path + ns_dir_name + "/log"))
		}
	}
	apis, err := kc.ApiResources()
	if err != nil {
		return err
	}
	// filter out gvk based on regex list
	if apis, err = FilterApiResources(apis, gvkExclusionList); err != nil {
		return err
	}
	if err = writeTextFile(path+"api_resources.yaml", apis, gz, format); err != nil {
		return err
	}
	apiList, err := yjq.Eval2List(yjq.YqEval, apiAvailableListQuery, apis)
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
		if collectedErrors.Len() > 0 {
			return errors.New(collectedErrors.String())
		}
	}
	if tgz {
		tgzpath := path + dumpDir + ".tar"
		if err = archive(path, tgzpath); err != nil {
			logger.Error("tape archiving error", zap.Error(err))
			return err
		}
		if err = gzip(tgzpath); err != nil {
			logger.Error("gzip error", zap.Error(err))
		}
	}
	return nil
}

func writeResourceList(path string, baseName string, name string, gv string, namespaced bool, nologs bool, gz bool, format int, progress func()) error {
	if progress != nil {
		defer progress()
	}
	logLine := fmt.Sprintf("writeResourceList: baseName=%s name=%s  gv=%s namespaced=%t", baseName, name, gv, namespaced)
	logger.Debug(logLine)
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
		logger.Info("empty api resource list " + logLine)
		return nil
		// return writeResourceListLog("empty "+logLine, errors.New("empty list"))
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
						if err = fsutil.WriteTextFile(nsPath+"log/"+fileName, log); err != nil {
							return writeResourceListLog("writing pod log "+logLine, err)
						}
						if gz {
							if err = gzip(nsPath + "log/" + fileName); err != nil {
								return writeResourceListLog("gzipping pod log "+logLine, err)
							}
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
	case JSON_PRETTY:
		if contents, err = yjq.Y2JP(contents); err != nil {
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

func archive(source string, target string) error {
	var err error

	if _, err = os.Stat(source); err != nil {
		return fmt.Errorf("inexistent tar source: %v", err.Error())
	}

	outFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer outFile.Close()

	tw := tar.NewWriter(outFile)
	defer tw.Close()

	return filepath.Walk(source, func(file string, fi os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		if strings.Contains(target, fi.Name()) {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(strings.Replace(file, source, "", -1), string(filepath.Separator))

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// open files for taring
		f, err := os.Open(file)
		if err != nil {
			return err
		}

		// copy file data into tar writer
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		f.Close()

		return nil
	})
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
	case "json_pretty":
		return JSON_PRETTY, nil
	default:
		return -1, fmt.Errorf("unknown string coded format %s. please use one of 'yaml', 'json', 'json_pretty', 'json_lines', 'json_lines_wrapped'", format)
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

func FilterApiResources(apis string, gvkExclusionList []string) (string, error) {
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
		apis, err := yjq.YqEval(`del(.items[] | select(.groupVersion | test("%s") and .kind | test("%s")))`, apis, r[0], r[1])
		if err != nil {
			return apis, err
		}
	}
	return apis, nil
}

func FilterNS(ns string, nsExclusionList []string) (string, error) {
	for _, re := range nsExclusionList {
		ns, err := yjq.YqEval(`del(.items[] | select(.metadata.name | test("%s")))`, ns, re)
		if err != nil {
			return ns, err
		}
	}
	return ns, nil
}
