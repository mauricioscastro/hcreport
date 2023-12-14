package hcr

import (
	"context"
	"fmt"
	"strings"
	"sync"

	fsutil "github.com/coreybutler/go-fsutil"
	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/pkg/runner"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
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
	duLock *sync.Mutex
)

type reconciler struct {
	srw client.SubResourceWriter
	ctx context.Context
	cfg *hcrv1.Config
}

type Reconciler interface {
	Run() (ctrl.Result, error)
	extract() error
	statusCheck()
	setLogLevel()
	statusAddPhase(phase string) error
	statusAddDiskUsage() error
	updateStatus(jqExpr string) error
}

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

func NewReconciler(srw client.SubResourceWriter, ctx context.Context, cfg *hcrv1.Config) Reconciler {
	duLock = &sync.Mutex{}
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
	reportHome := reportPath + strings.ReplaceAll(rec.cfg.Name, "-", "_") + "/"
	return runner.Run().KcDump(reportHome, 0, func() {
		duLock.Lock()
		defer duLock.Unlock()
		rec.statusAddDiskUsage()
	}).Err()
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
		if err := rec.srw.Update(rec.ctx, rec.cfg); err != nil && !strings.Contains(err.Error(), "try again") {
			logger.Debug("unable to update status", zap.Error(err))
			return err
		}
	} else {
		logger.Debug("cmd runner error", zap.Error(cmd.Err()))
	}
	return cmd.Err()
}
