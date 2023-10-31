package hcr

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/pkg/hcr/template"
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
	reportPath = "/_data/"
)

var (
	cmdr   runner.CmdRunner
	logger = log.Logger().Named("hcr.reconciler")
)

type reconciler struct {
	srw          client.SubResourceWriter
	ctx          context.Context
	cfg          *hcrv1.Config
	apiResources [][]string
	ns           []string
}

type Reconciler interface {
	Run() (ctrl.Result, error)
}

func init() {
	cmdr = runner.NewCmdRunner()
}

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

func NewReconciler(srw client.SubResourceWriter, ctx context.Context, cfg *hcrv1.Config) Reconciler {
	return &reconciler{srw, ctx, cfg, [][]string{}, []string{}}
}

func (rec *reconciler) Run() (ctrl.Result, error) {
	rec.statusCheck()
	err := rec.statusAddPhase("extracting")
	if err != nil {
		return ctrl.Result{}, err
	}
	rec.setLogLevel()
	rec.apiResources, err = cmdr.KcApiResources()
	if err != nil {
		return ctrl.Result{}, err
	}
	rec.ns, err = cmdr.KcNs()
	if err != nil {
		return ctrl.Result{}, err
	}
	err = rec.extract()
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (rec *reconciler) extract() error {
	reportHome := reportPath + rec.cfg.Name
	apiResourcesYaml := "api-resources: []"
	cmdr.MkDir(reportHome)
	if cmdr.Err() != nil {
		return cmdr.Err()
	}
	for _, r := range rec.apiResources {
		verbs := `"` + strings.ReplaceAll(r[5], ";", `","`) + `"`
		if !strings.Contains(verbs, "get") {
			continue
		}
		name := r[0]
		gv := r[2]
		fileName := name + "." + strings.Replace(gv, "/", ".", -1) + ".yaml"
		fullName := name
		cmdr.Echo(gv).Sed("s;/?v[^\\.]*$;;g")
		if !cmdr.Empty() {
			fullName = fullName + "." + cmdr.String()
		}
		shortNames := r[1]
		if len(shortNames) > 0 {
			shortNames = `"` + strings.ReplaceAll(shortNames, ";", `","`) + `"`
		}
		yq := `.api-resources += {"kind":"%s", "name":"%s", "shortNames": [%s], "groupVersion":"%s", "namespaced":"%s", "verbs": [%s], "category":"%s", "fileName":"%s"}`
		apiResourcesYaml = cmdr.
			Echo(apiResourcesYaml).
			Yq(fmt.Sprintf(yq, r[4], name, shortNames, gv, r[3], verbs, r[6], fileName)).
			String()
		if cmdr.Err() != nil {
			return cmdr.Err()
		}
		if namespaced, err := strconv.ParseBool(r[3]); err == nil && namespaced {
			for _, n := range rec.ns {
				nsDir := reportHome + "/" + n
				if err = os.MkdirAll(nsDir, fs.ModePerm); err != nil {
					return err
				}
				if err = writeResourceList(nsDir+"/"+fileName, fullName, n); err != nil {
					return err
				}
			}
		} else if err == nil {
			if err = writeResourceList(reportHome+"/"+fileName, fullName, ""); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return os.WriteFile(reportHome+"/api-resources.yaml", []byte(apiResourcesYaml), fs.ModePerm)
}

func writeResourceList(filePath string, fullName string, namespace string) error {
	kcCmd := []string{"get", "-o", "yaml", fullName}
	if len(namespace) > 0 {
		kcCmd = append(kcCmd, "-n", namespace)
	}
	cmdr.KcCmd(kcCmd).
		Yq(`with(.[].[].metadata; del(.uid) | del(.generation) | del(.annotations.["kubectl.kubernetes.io/last-applied-configuration"]))`)
	// hide secrets
	if fullName == "secrets" {
		cmdr.Yq(`.[].[].data.[] = ""`)
	}
	if cmdr.Err() != nil {
		return cmdr.Err()
	}
	resourceList := cmdr.Bytes()
	if !cmdr.Yq(`.items[0] // ""`).Empty() {
		if err := os.WriteFile(filePath, resourceList, fs.ModePerm); err != nil {
			return err
		}
		// extract logs
		if fullName == "pods" {
			logDir := filepath.Dir(filePath) + "/log/"
			cmdr.MkDir(logDir).
				Echo(resourceList).
				Yq(".[].[].metadata.name")
			if cmdr.Err() != nil {
				return cmdr.Err()
			}
			for _, pod := range cmdr.List() {
				cmdr.KcCmd([]string{"logs", "--all-containers=true", pod, "-n", namespace}).
					WriteFile(logDir + pod + ".log")
				if cmdr.Err() != nil {
					return cmdr.Err()
				}
			}
		}
	}
	return nil
}

func (rec *reconciler) statusCheck() {
	if len(rec.cfg.Status) == 0 {
		rec.cfg.Status = cmdr.Echo(template.Status).ToJson().Bytes()
	}
}

func (rec *reconciler) setLogLevel() {
	cmdr.Echo(rec.cfg.Spec).Yq(`.logLevel // ""`)
	if cmdr.Err() == nil && !cmdr.Empty() {
		logger.Debug("setting log level", zap.String("level", cmdr.String()))
		SetLoggerLevel(cmdr.String())
		yqw.SetLoggerLevel(cmdr.String())
		kcw.SetLoggerLevel(cmdr.String())
		runner.SetLoggerLevel(cmdr.String())
	}
}

func (rec *reconciler) statusAddPhase(phase string) error {
	ts := time.Now().Format(time.RFC3339)
	jq := fmt.Sprintf(`.phase = "%s" | .transitions.last = "%s" | .transitions.next = "unscheduled"`, phase, ts)
	if cmdr.Echo(rec.cfg.Status).Jq(jq).Err() == nil {
		rec.cfg.Status = cmdr.Bytes()
		if err := rec.srw.Update(rec.ctx, rec.cfg); err != nil {
			logger.Error("unable to update status", zap.Error(err))
			return err
		}
	}
	return nil
}

// TODO: split to files example from Kind: List example
// r := runner.NewCmdRunner().
// Kc("get pods -A").
// YqSplit(".items.[] | with(.metadata; del(.creationTimestamp) | del(.resourceVersion) | del(.uid) | del(.generateName) | del(.labels.pod-template-hash))",
// 	".metadata.name",
// 	"/tmp/yq")
