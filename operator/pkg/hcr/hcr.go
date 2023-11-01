package hcr

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	reportPath = "/_data/"
	status     = `
    phase: ""
    diskUsage: ""
    transitions: []
  `
)

var logger = log.Logger().Named("hcr.reconciler")

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

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

func NewReconciler(srw client.SubResourceWriter, ctx context.Context, cfg *hcrv1.Config) Reconciler {
	return &reconciler{srw, ctx, cfg, [][]string{}, []string{}}
}

func (rec *reconciler) Run() (ctrl.Result, error) {
	cmd := runner.NewCmdRunner()
	rec.statusCheck()
	err := rec.statusAddPhase("extracting")
	if err != nil {
		return ctrl.Result{}, err
	}
	rec.setLogLevel()
	rec.apiResources, err = cmd.KcApiResources()
	if err != nil {
		return ctrl.Result{}, err
	}
	rec.ns, err = cmd.KcNs()
	if err != nil {
		return ctrl.Result{}, err
	}
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
	reportHome := reportPath + rec.cfg.Name
	apiResourcesYaml := "api-resources: []"
	cmd.MkDir(reportHome)
	if cmd.Err() != nil {
		return cmd.Err()
	}
	for i, r := range rec.apiResources {
		verbs := `"` + strings.ReplaceAll(r[5], ";", `","`) + `"`
		if !strings.Contains(verbs, "get") {
			continue
		}
		name := r[0]
		gv := r[2]
		fileName := name + "." + strings.Replace(gv, "/", ".", -1) + ".yaml"
		fullName := name
		cmd.Echo(gv).Sed("s;/?v[^\\.]*$;;g")
		if !cmd.Empty() {
			fullName = fullName + "." + cmd.String()
		}
		shortNames := r[1]
		if len(shortNames) > 0 {
			shortNames = `"` + strings.ReplaceAll(shortNames, ";", `","`) + `"`
		}
		yq := `.api-resources += {"kind":"%s", "name":"%s", "shortNames": [%s], "groupVersion":"%s", "namespaced":"%s", "verbs": [%s], "category":"%s", "fileName":"%s"}`
		apiResourcesYaml = cmd.
			Echo(apiResourcesYaml).
			Yq(fmt.Sprintf(yq, r[4], name, shortNames, gv, r[3], verbs, r[6], fileName)).
			String()
		if cmd.Err() != nil {
			return cmd.Err()
		}
		rec.writeResourceList(reportHome+"/"+fileName, fullName)
		if i%9 == 0 {
			rec.statusAddDiskUsage()
		}
	}
	logger.Info("extraction done")
	return os.WriteFile(reportHome+"/api-resources.yaml", []byte(apiResourcesYaml), fs.ModePerm)
}

func (rec *reconciler) writeResourceList(filePath string, fullName string) error {
	cmd := runner.NewCmdRunner()
	if !cmd.Kc("get --ignore-not-found=true -A " + fullName).Empty() {
		// cleaning
		cmd.Yq(`with(.[].[].metadata; del(.uid) | del(.generation) | del(.resourceVersion) | del(.annotations.["kubectl.kubernetes.io/last-applied-configuration"]) | del(.labels.["kubernetes.io/metadata.name"]))`)
		// hide secrets
		if fullName == "secrets" {
			cmd.Yq(`.[].[].data.[] = ""`)
		}
		cmd.WriteFile(filePath)
		// extract logs
		if fullName == "pods" {
			logDir := filepath.Dir(filePath) + "/log/"
			cmd.MkDir(logDir)
			for _, p := range cmd.Yq(`.[].[].metadata | [.namespace, .name] | join(",")`).List() {
				pod := strings.Split(p, ",")
				cmd.KcCmd([]string{"logs", "--all-containers=true", pod[1], "-n", pod[0]})
				if !cmd.Empty() {
					cmd.WriteFile(logDir + pod[0] + "." + pod[1] + ".log").IgnoreError()
					rec.statusAddDiskUsage()
				}
			}
		}
	}
	return cmd.IgnoreError("NotFound", "NotAllowed").Err()
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
	cmd := runner.NewCmdRunner()
	if cmd.Echo(rec.cfg.Status).Jq(jqExpr).Err() == nil {
		rec.cfg.Status = cmd.Bytes()
		if err := rec.srw.Update(rec.ctx, rec.cfg); err != nil {
			logger.Error("unable to update status", zap.Error(err))
			return err
		}
	}
	return nil
}
