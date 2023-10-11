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

package v1

import (
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var logger = log.Logger().Named("hcr.cfg.hook")

func (r *Config) SetupWebhookWithManager(mgr ctrl.Manager) error {
	logger.Info("webhook setup", zap.String("name", r.Name))
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-hcreport-csa-latam-redhat-com-v1-config,mutating=true,failurePolicy=fail,sideEffects=None,groups=hcreport.csa.latam.redhat.com,resources=configs,verbs=create;update,versions=v1,name=mconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Config{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Config) Default() {
	logger.Info("default", zap.String("name", r.Name))

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-hcreport-csa-latam-redhat-com-v1-config,mutating=false,failurePolicy=fail,sideEffects=None,groups=hcreport.csa.latam.redhat.com,resources=configs,verbs=create;update;delete,versions=v1,name=vconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Config{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Config) ValidateCreate() (admission.Warnings, error) {
	logger.Info("validate create", zap.String("name", r.Name))
	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Config) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	logger.Info("validate update", zap.String("name", r.Name))
	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Config) ValidateDelete() (admission.Warnings, error) {
	logger.Info("validate delete", zap.String("name", r.Name))

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
