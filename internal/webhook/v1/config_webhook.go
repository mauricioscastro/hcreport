/*
Copyright 2025.

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
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcrv1 "adoption.latam/hcr/api/v1"
	"adoption.latam/hcr/internal/pkg/util/log"
)

// nolint:unused
// log is for logging in this package.
var logger = log.Logger().Named("hcr.cfg.hook")

// SetupConfigWebhookWithManager registers the webhook for Config in the manager.
func SetupConfigWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&hcrv1.Config{}).
		WithValidator(&ConfigCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-hcr-adoption-latam-v1-config,mutating=false,failurePolicy=fail,sideEffects=None,groups=hcr.adoption.latam,resources=configs,verbs=create;update;delete,versions=v1,name=vconfig-v1.kb.io,admissionReviewVersions=v1

// ConfigCustomValidator struct is responsible for validating the Config resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ConfigCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &ConfigCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Config.
func (v *ConfigCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	config, ok := obj.(*hcrv1.Config)
	if !ok {
		return nil, fmt.Errorf("expected a Config object but got %T", obj)
	}
	logger.Info("Validation for Config upon creation", zap.String("name", config.GetName()))
	logger.Info("config", zap.String("spec", string(config.Spec)))
	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Config.
func (v *ConfigCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	config, ok := newObj.(*hcrv1.Config)
	if !ok {
		return nil, fmt.Errorf("expected a Config object for the newObj but got %T", newObj)
	}
	logger.Info("Validation for Config upon update", zap.String("name", config.GetName()))
	logger.Info("config", zap.String("spec", string(config.Spec)))
	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Config.
func (v *ConfigCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	config, ok := obj.(*hcrv1.Config)
	if !ok {
		return nil, fmt.Errorf("expected a Config object but got %T", obj)
	}
	logger.Info("Validation for Config upon deletion", zap.String("name", config.GetName()))
	logger.Info("config", zap.String("spec", string(config.Spec)))
	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
