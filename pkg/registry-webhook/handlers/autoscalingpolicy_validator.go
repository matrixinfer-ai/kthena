/*
Copyright MatrixInfer-AI Authors.

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

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"math"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	registryv1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// AutoscalingPolicyValidator handles validation of AutoscalingPolicy resources
type AutoscalingPolicyValidator struct {
	kubeClient        kubernetes.Interface
	matrixinferClient clientset.Interface
}

// NewAutoscalingPolicyValidator creates a new AutoscalingPolicyValidator
func NewAutoscalingPolicyValidator(kubeClient kubernetes.Interface, matrixinferClient clientset.Interface) *AutoscalingPolicyValidator {
	return &AutoscalingPolicyValidator{
		kubeClient:        kubeClient,
		matrixinferClient: matrixinferClient,
	}
}

// Handle handles admission requests for AutoscalingPolicy resources
func (v *AutoscalingPolicyValidator) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse the admission request
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		klog.Errorf("Failed to decode admission review: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if admissionReview.Request == nil {
		http.Error(w, "invalid admission review request", http.StatusBadRequest)
		return
	}

	// Deserialize the object
	var policy registryv1.AutoscalingPolicy
	if err := json.Unmarshal(admissionReview.Request.Object.Raw, &policy); err != nil {
		klog.Errorf("Failed to unmarshal object: %v", err)
		http.Error(w, fmt.Sprintf("could not unmarshal object: %v", err), http.StatusBadRequest)
		return
	}

	// Validate the AutoscalingPolicy
	allowed, reason := v.validateAutoscalingPolicy(&policy)

	// Create the admission response
	admissionResponse := admissionv1.AdmissionResponse{
		Allowed: allowed,
		UID:     admissionReview.Request.UID,
	}

	if !allowed {
		admissionResponse.Result = &metav1.Status{
			Message: reason,
		}
	}

	// Create the admission review response
	admissionReview.Response = &admissionResponse

	// Send the response
	if err := json.NewEncoder(w).Encode(admissionReview); err != nil {
		klog.Errorf("Failed to encode admission review: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// validateAutoscalingPolicy validates the AutoscalingPolicy resource
func (v *AutoscalingPolicyValidator) validateAutoscalingPolicy(policy *registryv1.AutoscalingPolicy) (bool, string) {
	var allErrs field.ErrorList

	metricNames := make(map[string]bool)
	for _, metric := range policy.Spec.Metrics {
		if metric.TargetValue.AsFloat64Slow() <= 0 || math.IsInf(metric.TargetValue.AsFloat64Slow(), 0) {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("metrics"),
				metric.TargetValue,
				"metric target value must be greater than 0 and not equal to infinity",
			))
		}
		if metricNames[metric.MetricName] {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("metrics"),
				metric.MetricName,
				fmt.Sprintf("duplicate metric name %s is not allowed", metric.MetricName),
			))
		}
		metricNames[metric.MetricName] = true
	}

	stablePolicy := policy.Spec.Behavior.ScaleDown
	if stablePolicy.Period.Seconds() < 0 || stablePolicy.Period.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleDown").Child("period"),
			stablePolicy.Period,
			"stable policy period must be between 0 and 30 minutes",
		))
	}
	if stablePolicy.StabilizationWindow.Seconds() < 0 || stablePolicy.StabilizationWindow.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleDown").Child("stabilizationWindow"),
			stablePolicy.StabilizationWindow,
			"stable policy stabilization window must be between 0 and 30 minutes",
		))
	}
	if *stablePolicy.Instances < 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleDown").Child("instances"),
			*stablePolicy.Instances,
			"stable policy instances must be greater than or equal to 0",
		))
	}
	if *stablePolicy.Percent < 0 || *stablePolicy.Percent > 100 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleDown").Child("percent"),
			*stablePolicy.Percent,
			"stable policy percent must be between 0 and 100",
		))
	}

	stablePolicy = policy.Spec.Behavior.ScaleUp.StablePolicy
	if stablePolicy.Period.Seconds() < 0 || stablePolicy.Period.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("stablePolicy").Child("period"),
			stablePolicy.Period,
			"stable policy period must be between 0 and 30 minutes",
		))
	}
	if stablePolicy.StabilizationWindow.Seconds() < 0 || stablePolicy.StabilizationWindow.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("stablePolicy").Child("stabilizationWindow"),
			stablePolicy.StabilizationWindow,
			"stable policy stabilization window must be between 0 and 30 minutes",
		))
	}
	if *stablePolicy.Instances < 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("stablePolicy").Child("instances"),
			*stablePolicy.Instances,
			"stable policy instances must be greater than or equal to 0",
		))
	}
	if *stablePolicy.Percent < 0 || *stablePolicy.Percent > 1000 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("stablePolicy").Child("percent"),
			*stablePolicy.Percent,
			"stable policy percent must be between 0 and 1000",
		))
	}

	panicPolicy := policy.Spec.Behavior.ScaleUp.PanicPolicy
	if panicPolicy.Period.Seconds() < 0 || panicPolicy.Period.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("panicPolicy").Child("period"),
			panicPolicy.Period,
			"panic policy period must be between 0 and 30 minutes",
		))
	}
	if panicPolicy.PanicModeHold.Seconds() < 0 || panicPolicy.PanicModeHold.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("panicPolicy").Child("panicModeHold"),
			panicPolicy.PanicModeHold,
			"panic policy panic mode hold must be between 0 and 30 minutes",
		))
	}
	if *panicPolicy.Percent < 0 || *panicPolicy.Percent > 1000 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("panicPolicy").Child("percent"),
			*panicPolicy.Percent,
			"panic policy percent must be between 0 and 1000",
		))
	}
	if *panicPolicy.PanicThresholdPercent < 0 || *panicPolicy.PanicThresholdPercent > 1000 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("behavior").Child("scaleUp").Child("panicPolicy").Child("panicThresholdPercent"),
			*panicPolicy.PanicThresholdPercent,
			"panic policy panic threshold percent must be between 0 and 1000",
		))
	}

	if len(allErrs) > 0 {
		var messages []string
		for _, err := range allErrs {
			messages = append(messages, fmt.Sprintf("  - %s", err.Error()))
		}
		return false, fmt.Sprintf("validation failed:\n%s", strings.Join(messages, "\n"))
	}
	return true, ""
}
