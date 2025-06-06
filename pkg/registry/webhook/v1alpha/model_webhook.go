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

package v1alpha

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	registryv1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var modellog = logf.Log.WithName("model-resource")

// SetupModelWebhookWithManager registers the webhook for Model in the manager.
func SetupModelWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&registryv1.Model{}).
		WithValidator(&ModelCustomValidator{}).
		WithDefaulter(&ModelCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-registry-matrixinfer-ai-v1-model,mutating=true,failurePolicy=fail,sideEffects=None,groups=registry.matrixinfer.ai,resources=models,verbs=create;update,versions=v1,name=mmodel-v1.kb.io,admissionReviewVersions=v1

// ModelCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Model when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ModelCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &ModelCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Model.
func (d *ModelCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	model, ok := obj.(*registryv1.Model)

	if !ok {
		return fmt.Errorf("expected an Model object but got %T", obj)
	}
	modellog.Info("Defaulting for Model", "name", model.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-registry-matrixinfer-ai-v1-model,mutating=false,failurePolicy=fail,sideEffects=None,groups=registry.matrixinfer.ai,resources=models,verbs=create;update,versions=v1,name=vmodel-v1.kb.io,admissionReviewVersions=v1

// ModelCustomValidator struct is responsible for validating the Model resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ModelCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &ModelCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Model.
func (v *ModelCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	model, ok := obj.(*registryv1.Model)
	if !ok {
		return nil, fmt.Errorf("expected a Model object but got %T", obj)
	}
	modellog.Info("Validation for Model upon creation", "name", model.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Model.
func (v *ModelCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	model, ok := newObj.(*registryv1.Model)
	if !ok {
		return nil, fmt.Errorf("expected a Model object for the newObj but got %T", newObj)
	}
	modellog.Info("Validation for Model upon update", "name", model.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Model.
func (v *ModelCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	model, ok := obj.(*registryv1.Model)
	if !ok {
		return nil, fmt.Errorf("expected a Model object but got %T", obj)
	}
	modellog.Info("Validation for Model upon deletion", "name", model.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
