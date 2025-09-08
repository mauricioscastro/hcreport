package hcr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	hcrv1 "adoption.latam/hcr/api/v1"
	fsutil "github.com/coreybutler/go-fsutil"
	"github.com/mauricioscastro/kcdump/pkg/yjq"

	"adoption.latam/hcr/internal/pkg/util/log"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"time"
)

const (
	reportPath = "/data/kcdump"
)

var (
	logger       = log.Logger().Named("hcr.reconciler")
	progressLock *sync.Mutex
)

type reconciler struct {
	srw client.SubResourceWriter
	ctx context.Context
	cfg *hcrv1.Config
}

type Reconciler interface {
	Run() (ctrl.Result, error)
	extract() error
	// setLogLevel() error
	statusAddPhase(phase string) error
	statusAddDiskUsage() error
	getNextBuildTime() string
	updateStatus(jqExpr string) error
}

func NewReconciler(srw client.SubResourceWriter, ctx context.Context, cfg *hcrv1.Config) Reconciler {
	progressLock = &sync.Mutex{}
	return &reconciler{srw, ctx, cfg}
}

func (rec *reconciler) Run() (ctrl.Result, error) {
	firstRun := false
	if len(rec.cfg.Status) == 0 {
		rec.cfg.Status = json.RawMessage("{}")
		firstRun = true
	}
	nextTimeDuration := rec.getNextBuildTime()
	if firstRun || (!firstRun && nextTimeDuration != "never") {
		logger.Debug("will rebuild every " + nextTimeDuration)
		// err := rec.setLogLevel()
		// if err != nil {
		// 	return ctrl.Result{}, err
		// }
		if err := rec.statusAddPhase("extracting"); err != nil {
			return ctrl.Result{}, err
		}
		// if err := rec.extract(); err != nil {
		// 	logger.Error("extracting", zap.Error(err))
		// 	return ctrl.Result{}, err
		// }
		// logger.Info("finished extracting")
		if err := rec.statusAddDiskUsage(); err != nil {
			return ctrl.Result{}, err
		}
		if err := rec.statusAddPhase("building"); err != nil {
			logger.Error("building", zap.Error(err))
			return ctrl.Result{}, err
		}
		if err := rec.statusAddPhase("finished"); err != nil {
			logger.Error("finished", zap.Error(err))
			return ctrl.Result{}, err
		}
		logger.Info("finished building")
	}
	if nextTimeDuration == "never" {
		nextTimeDuration = "10s"
	}
	duration, err := time.ParseDuration(nextTimeDuration)
	if err != nil {
		logger.Error("ParseDuration", zap.Error(err))
		return ctrl.Result{}, err
	}
	err = rec.updateStatus(`.lastReconciliation = "` + time.Now().Format(time.RFC3339) + `"`)
	if err != nil {
		logger.Error("add lastReconciliation", zap.Error(err))
		return ctrl.Result{}, err
	}
	// logger.Debug(nextTimeDuration)
	return ctrl.Result{RequeueAfter: duration}, nil
}

func (rec *reconciler) extract() error {
	// reportHome := reportPath
	// nslist := []string{}  //[]string{"open.*"}
	// gvklist := []string{} //[]string{".*,CustomResourceDefinition", ".*,APIRequestCount"}
	// nologs := true
	// tgz := false
	// gz := true
	// format := kc.JSON_LINES
	// splitns := false
	// splitgv := false
	// prune := false
	// routines := 10
	// chunkSize := 25
	return nil
	// return kc.NewKc().Dump(reportHome, nslist, gvklist, nologs, gz, tgz, prune, splitns, splitgv, format, routines, chunkSize, func() {
	// 	progressLock.Lock()
	// 	rec.statusAddDiskUsage()
	// 	progressLock.Unlock()
	// })
}

func (rec *reconciler) getNextBuildTime() string {
	next, e := yjq.JqEval(`.rebuildAfter // "never"`, string(rec.cfg.Spec))
	if e != nil {
		logger.Error("scheduleNextBuild", zap.Error(e))
	}
	return next
}

// func (rec *reconciler) setLogLevel() error {
// 	s, e := yjq.JqEval(`.logLevel // "info"`, string(rec.cfg.Spec))
// 	if e != nil {
// 		return e
// 	} else if s != "info" {
// 		logger.Debug("setting log level", zap.String("level", s))
// 		logger = log.ResetLoggerLevel(logger, s)
// 	}
// 	return nil
// }

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
