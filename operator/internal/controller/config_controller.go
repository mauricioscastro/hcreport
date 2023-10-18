/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcr "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/pkg/runner"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	k8slogger "sigs.k8s.io/controller-runtime/pkg/log"
	//+kubebuilder:scaffold:imports
)

var logger = log.Logger().Named("hcr.cfg.cntlr")

type ConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func SetLoggerLevel(level zapcore.Level) {
	logger = log.ResetLoggerLevel(logger, level)
}

//+kubebuilder:rbac:groups=hcreport.csa.latam.redhat.com,resources=configs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hcreport.csa.latam.redhat.com,resources=configs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hcreport.csa.latam.redhat.com,resources=configs/finalizers,verbs=update

func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger.Info("reconcile...")
	log := k8slogger.FromContext(ctx)

	cfg := &hcr.Config{}
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, cfg)

	if err != nil {
		if apierr.IsNotFound(err) {
			log.Info("hcreport config resource not found. error ignored")
			return ctrl.Result{}, nil
		}
		log.Error(err, "can not go past this error. returning")
		return ctrl.Result{}, err
	}

	spec := string(cfg.Spec)
	logLevel := runner.NewCmdRunner().Echo(spec).Yq(".logLevel").Out()
	zLevel, _ := zapcore.ParseLevel(logLevel)
	logger.Info("setting log level to " + logLevel)
	SetLoggerLevel(zLevel)
	logger.Debug("log level set")

	cfg.Status = cfg.Spec

	r.Status().Update(ctx, cfg)

	log.Info("reconcile loop", "spec", cfg.Spec, "status", cfg.Status)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hcr.Config{}).
		Complete(r)
}
