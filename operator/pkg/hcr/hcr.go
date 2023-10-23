package hcr

import (
	"context"
	"fmt"

	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/pkg/hcr/template"
	kcr "github.com/mauricioscastro/hcreport/pkg/runner"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"time"
)

const (
	reportPath = "/_data"
)

var (
	cmdr   kcr.CmdRunner
	logger = log.Logger().Named("hcr.reconciler")
)

type reconciler struct {
	srw client.SubResourceWriter
	ctx context.Context
	cfg *hcrv1.Config
}

type Reconciler interface {
	Run() (ctrl.Result, error)
}

func init() {
	cmdr = kcr.NewCmdRunner()
}

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

func NewReconciler(srw client.SubResourceWriter, ctx context.Context, cfg *hcrv1.Config) Reconciler {
	return &reconciler{srw, ctx, cfg}
}

func (rec *reconciler) Run() (ctrl.Result, error) {
	rec.statusCheck()
	if err := rec.statusAdd("extracting"); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (rec *reconciler) statusCheck() {
	if len(rec.cfg.Status) == 0 {
		rec.cfg.Status = cmdr.Echo(template.Status).ToJson().Bytes()
	}
}

func (rec *reconciler) statusAdd(phase string) error {
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
