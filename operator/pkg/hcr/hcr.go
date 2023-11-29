package hcr

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"

	fsutil "github.com/coreybutler/go-fsutil"
	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/pkg/runner"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	kcw "github.com/mauricioscastro/hcreport/pkg/wrapper/kc"
	yqw "github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"time"
)

const (
	reportPath       = "/_data/"
	apiResourcesFile = "api_resources.yaml"
	status           = `
    phase: ""
    diskUsage: ""
    transitions: []
  `
)

var (
	logger = log.Logger().Named("hcr.reconciler")
	// lck    *sync.Mutex
)

type reconciler struct {
	srw client.SubResourceWriter
	ctx context.Context
	cfg *hcrv1.Config
}

type Reconciler interface {
	Run() (ctrl.Result, error)
}

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

func NewReconciler(srw client.SubResourceWriter, ctx context.Context, cfg *hcrv1.Config) Reconciler {
	// lck = &sync.Mutex{}
	return &reconciler{srw, ctx, cfg}
}

func (rec *reconciler) Run() (ctrl.Result, error) {
	rec.statusCheck()
	err := rec.statusAddPhase("extracting")
	if err != nil {
		return ctrl.Result{}, err
	}
	rec.setLogLevel()
	err = rec.extract()
	if err != nil {
		return ctrl.Result{}, err
	}
	err = rec.statusAddPhase("building")
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (rec *reconciler) extract() error {
	cmd := runner.NewCmdRunner()
	reportHome := reportPath + strings.ReplaceAll(rec.cfg.Name, "-", "_") + "/"
	apiResourcesYaml := "api-resources: []"
	if cmd.MkDir(reportHome); cmd.Err() != nil {
		return cmd.Err()
	}
	cmd.Kc("version").WriteFile(reportHome + "version.yaml")
	nsList, err := cmd.KcNs()
	if err != nil {
		return err
	} else {
		for _, ns := range nsList {
			if cmd.MkDir(reportHome + strings.ReplaceAll(ns, "-", "_") + "/log"); cmd.Err() != nil {
				return cmd.Err()
			}
		}
	}
	apiResources, _ := cmd.KcApiResources()
	if cmd.Err() != nil {
		return cmd.Err()
	}
	// var wg sync.WaitGroup
	// resourceWorkerPool, _ := ants.NewPool(16)
	// defer resourceWorkerPool.Release()
	for _, r := range apiResources {
		verbs := `"` + strings.ReplaceAll(r[5], ";", `","`) + `"`
		if !strings.Contains(verbs, "get") {
			continue
		}
		name := r[0]
		gv := r[2]
		fileName := name + "_" + strings.ReplaceAll(gv, "/", "_")
		fileName = strings.ReplaceAll(fileName, ".", "_") + ".yaml"
		fullName := name
		cmd.Echo(gv).Sed("s;/?v[^\\.]*$;;g")
		if !cmd.Empty() {
			fullName = fullName + "." + cmd.String()
		}
		shortNames := r[1]
		if len(shortNames) > 0 {
			shortNames = `"` + strings.ReplaceAll(shortNames, ";", `","`) + `"`
		}
		cat := r[6]
		if len(cat) > 0 {
			cat = `"` + strings.ReplaceAll(cat, ";", `","`) + `"`
		}
		yq := `.api-resources += {"kind":"%s", "name":"%s", "shortNames": [%s], "groupVersion":"%s", "namespaced":"%s", "verbs": [%s], "categories": [%s], "fileName":"%s"}`
		apiResourcesYaml = cmd.Echo(apiResourcesYaml).
			Yq(fmt.Sprintf(yq, r[4], name, shortNames, gv, r[3], verbs, cat, fileName)).
			String()
		if err != nil {
			return cmd.Err()
		}
		namespaced, err := strconv.ParseBool(r[3])
		if err != nil {
			return err
		}
		// resourceWorkerPool.Submit(func() {
		// 	wg.Add(1)
		writeResourceList(rec, reportHome, fileName, fullName, nsList, namespaced)
		// 	wg.Done()
		// })
	}
	// wg.Wait()
	logger.Info("extraction done")
	return os.WriteFile(reportHome+apiResourcesFile, []byte(apiResourcesYaml), fs.ModePerm)
}

func writeResourceList(rec *reconciler, path string, name string, fullName string, nsList []string, namespaced bool) {
	cmd := runner.NewCmdRunner()
	if cmd.Kc("get --ignore-not-found=true -A " + fullName).Empty() {
		return
	}
	// cleaning
	cmd.Yq(`with(.[].[].metadata; del(.uid) | del(.generation) | del(.resourceVersion) | del(.annotations.["kubectl.kubernetes.io/last-applied-configuration"]) | del(.labels.["kubernetes.io/metadata.name"]))`)
	// hide secrets
	if fullName == "secrets" {
		cmd.Yq(`.[].[].data.[] = ""`)
	}
	if !namespaced {
		cmd.WriteFile(path + name)
	} else {
		splitYq := `{ "kind": "List", "apiVersion": "v1", "items": [.items[].metadata.namespace | select(. == "%s") | parent | parent] }`
		emptyRe, _ := regexp.Compile(`(?m)^items: \[\]`)
		for _, ns := range nsList {
			nsCmd := cmd.Clone().Yq(fmt.Sprintf(splitYq, ns))
			if emptyRe.Match(nsCmd.Bytes()) {
				continue
			}
			nsDir := path + strings.ReplaceAll(ns, "-", "_") + "/"
			nsCmd.WriteFile(nsDir + name)
			if fullName == "pods" {
				for _, podName := range nsCmd.Yq(".items[].metadata.name").List() {
					if !nsCmd.KcCmd([]string{"logs", "--all-containers=true", podName, "-n", ns}).Empty() {
						nsCmd.WriteFile(nsDir + "log/" + podName + ".log")
					}
				}
			}
		}
	}
	rec.statusAddDiskUsage()
}

func (rec *reconciler) statusCheck() {
	if len(rec.cfg.Status) == 0 {
		rec.cfg.Status = runner.NewCmdRunner().Echo(status).ToJson().Bytes()
	}
}

func (rec *reconciler) setLogLevel() {
	cmd := runner.NewCmdRunner()
	cmd.Echo(rec.cfg.Spec).Yq(`.logLevel // ""`)
	if cmd.Err() == nil && !cmd.Empty() {
		logger.Debug("setting log level", zap.String("level", cmd.String()))
		SetLoggerLevel(cmd.String())
		yqw.SetLoggerLevel(cmd.String())
		kcw.SetLoggerLevel(cmd.String())
		runner.SetLoggerLevel(cmd.String())
	}
}

func (rec *reconciler) statusAddPhase(phase string) error {
	ts := time.Now().Format(time.RFC3339)
	du, _ := fsutil.Size(reportPath)
	jq := fmt.Sprintf(`.phase = "%s" | .diskUsage = "%s" | .transitions += [ {"phase": "%s", "transitionTime": "%s"} ]`, phase, du, phase, ts)
	return rec.updateStatus(jq)
}

func (rec *reconciler) statusAddDiskUsage() error {
	du, _ := fsutil.Size(reportPath)
	return rec.updateStatus(fmt.Sprintf(`.diskUsage = "%s"`, du))
}

func (rec *reconciler) updateStatus(jqExpr string) error {
	// defer lck.Unlock()
	cmd := runner.NewCmdRunner()
	if cmd.Echo(rec.cfg.Status).Jq(jqExpr).Err() == nil {
		// lck.Lock()
		rec.cfg.Status = cmd.Bytes()
		if err := rec.srw.Update(rec.ctx, rec.cfg); err != nil && !strings.Contains(err.Error(), "try again") {
			logger.Debug("unable to update status", zap.Error(err))
			return err
		}
	}
	return cmd.Err()
}
