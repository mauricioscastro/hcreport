package hcr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	fsutil "github.com/coreybutler/go-fsutil"
	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"

	"github.com/mauricioscastro/hcreport/pkg/kc"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"github.com/mauricioscastro/hcreport/pkg/yjq"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"time"
)

const (
	reportPath       = "/_data/"
	apiResourcesFile = "api_resources.yaml"
	status           = `{"phase": "", "diskUsage": "", "transitions": []}`
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
	setLogLevel() error
	statusAddPhase(phase string) error
	statusAddDiskUsage() error
	updateStatus(jqExpr string) error
}

func NewReconciler(srw client.SubResourceWriter, ctx context.Context, cfg *hcrv1.Config) Reconciler {
	duLock = &sync.Mutex{}
	return &reconciler{srw, ctx, cfg}
}

func (rec *reconciler) Run() (ctrl.Result, error) {
	if len(rec.cfg.Status) == 0 {
		rec.cfg.Status = json.RawMessage(status)
	}
	if err := rec.statusAddPhase("extracting"); err != nil {
		return ctrl.Result{}, err
	}
	if err := rec.setLogLevel(); err != nil {
		return ctrl.Result{}, err
	}
	if err := rec.extract(); err != nil {
		return ctrl.Result{}, err
	}
	if err := rec.statusAddPhase("building"); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (rec *reconciler) extract() error {
	reportHome := reportPath + strings.ReplaceAll(rec.cfg.Name, "-", "_") + "/"
	nslist := []string{}
	gvklist := []string{}
	nologs := false
	tgz := false
	gz := false
	format := kc.YAML
	splitns := false
	prune := false
	return kc.NewKc().Dump(reportHome, nslist, gvklist, nologs, gz, tgz, prune, splitns, format, 0, func() {
		duLock.Lock()
		rec.statusAddDiskUsage()
		duLock.Unlock()
	})
}

func (rec *reconciler) setLogLevel() error {
	if s, e := yjq.JqEval(`.logLevel // "info"`, string(rec.cfg.Spec)); e != nil {
		return e
	} else {
		logger.Debug("setting log level", zap.String("level", s))
		logger = log.ResetLoggerLevel(logger, s)
	}
	return nil
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
	if s, e := yjq.JqEval(jqExpr, string(rec.cfg.Status)); e != nil {
		logger.Error("hcr reconciler updateStatus", zap.Error(e))
		return e
	} else {
		rec.cfg.Status = json.RawMessage(s)
		if e = rec.srw.Update(rec.ctx, rec.cfg); e != nil && !strings.Contains(e.Error(), "try again") {
			logger.Warn("unable to update status", zap.Error(e))
			return e
		}
	}
	return nil
}
