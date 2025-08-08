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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/pkg/hcr"
	"github.com/mauricioscastro/hcreport/pkg/util/log"

	apierr "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	//+kubebuilder:scaffold:imports
)

type ConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var logger = log.Logger().Named("hcr.cfg.cntlr")

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

//+kubebuilder:rbac:groups=hcreport.csa.latam.redhat.com,resources=configs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hcreport.csa.latam.redhat.com,resources=configs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hcreport.csa.latam.redhat.com,resources=configs/finalizers,verbs=update

func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg hcrv1.Config
	logger.Info("req.Namespace=" + req.Namespace)
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, &cfg)
	if err != nil {
		if apierr.IsNotFound(err) {
			logger.Warn("hcreport config resource not found")
			return ctrl.Result{}, nil
		}
		logger.Error("can not go past this error. returning", zap.Error(err))
		return ctrl.Result{}, err
	}
	return hcr.NewReconciler(r.Status(), ctx, &cfg).Run()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hcrv1.Config{}).
		WithEventFilter(filterUpdate()).
		Complete(r)
}

func filterUpdate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()
		},
	}
}
