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
var autoscalingpolicylog = logf.Log.WithName("autoscalingpolicy-resource")

// SetupAutoscalingPolicyWebhookWithManager registers the webhook for AutoscalingPolicy in the manager.
func SetupAutoscalingPolicyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&registryv1.AutoscalingPolicy{}).
		WithValidator(&AutoscalingPolicyCustomValidator{}).
		WithDefaulter(&AutoscalingPolicyCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-registry-matrixinfer-ai-v1-autoscalingpolicy,mutating=true,failurePolicy=fail,sideEffects=None,groups=registry.matrixinfer.ai,resources=autoscalingpolicies,verbs=create;update,versions=v1,name=mautoscalingpolicy-v1.kb.io,admissionReviewVersions=v1

// AutoscalingPolicyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind AutoscalingPolicy when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type AutoscalingPolicyCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &AutoscalingPolicyCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind AutoscalingPolicy.
func (d *AutoscalingPolicyCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	autoscalingpolicy, ok := obj.(*registryv1.AutoscalingPolicy)

	if !ok {
		return fmt.Errorf("expected an AutoscalingPolicy object but got %T", obj)
	}
	autoscalingpolicylog.Info("Defaulting for AutoscalingPolicy", "name", autoscalingpolicy.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-registry-matrixinfer-ai-v1-autoscalingpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=registry.matrixinfer.ai,resources=autoscalingpolicies,verbs=create;update,versions=v1,name=vautoscalingpolicy-v1.kb.io,admissionReviewVersions=v1

// AutoscalingPolicyCustomValidator struct is responsible for validating the AutoscalingPolicy resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type AutoscalingPolicyCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &AutoscalingPolicyCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type AutoscalingPolicy.
func (v *AutoscalingPolicyCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	autoscalingpolicy, ok := obj.(*registryv1.AutoscalingPolicy)
	if !ok {
		return nil, fmt.Errorf("expected a AutoscalingPolicy object but got %T", obj)
	}
	autoscalingpolicylog.Info("Validation for AutoscalingPolicy upon creation", "name", autoscalingpolicy.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type AutoscalingPolicy.
func (v *AutoscalingPolicyCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	autoscalingpolicy, ok := newObj.(*registryv1.AutoscalingPolicy)
	if !ok {
		return nil, fmt.Errorf("expected a AutoscalingPolicy object for the newObj but got %T", newObj)
	}
	autoscalingpolicylog.Info("Validation for AutoscalingPolicy upon update", "name", autoscalingpolicy.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type AutoscalingPolicy.
func (v *AutoscalingPolicyCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	autoscalingpolicy, ok := obj.(*registryv1.AutoscalingPolicy)
	if !ok {
		return nil, fmt.Errorf("expected a AutoscalingPolicy object but got %T", obj)
	}
	autoscalingpolicylog.Info("Validation for AutoscalingPolicy upon deletion", "name", autoscalingpolicy.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
