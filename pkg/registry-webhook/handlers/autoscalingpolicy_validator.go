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
	"io"
	"math"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	klog.V(4).Info("Handling AutoscalingPolicy validation request")

	// Parse the admission request
	var body []byte
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			klog.Errorf("Failed to read request body: %v", err)
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		body = data
	}

	// Verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Invalid Content-Type: %s", contentType)
		http.Error(w, "invalid Content-Type, expected application/json", http.StatusBadRequest)
		return
	}

	// Parse the AdmissionReview request
	var admissionReview admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		klog.Errorf("Failed to decode admission review: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if admissionReview.Request == nil {
		klog.Error("Admission review request is nil")
		http.Error(w, "invalid admission review request", http.StatusBadRequest)
		return
	}

	// Deserialize the AutoscalingPolicy object
	var policy registryv1.AutoscalingPolicy
	if err := json.Unmarshal(admissionReview.Request.Object.Raw, &policy); err != nil {
		klog.Errorf("Failed to unmarshal AutoscalingPolicy: %v", err)
		http.Error(w, fmt.Sprintf("could not unmarshal AutoscalingPolicy: %v", err), http.StatusBadRequest)
		return
	}

	klog.V(4).Infof("Validating AutoscalingPolicy: %s/%s", policy.Namespace, policy.Name)

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
		klog.V(2).Infof("AutoscalingPolicy validation failed: %s", reason)
	} else {
		klog.V(4).Info("AutoscalingPolicy validation passed")
	}

	// Create the admission review response
	admissionReview.Response = &admissionResponse

	// Send the response
	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Failed to encode admission review response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	klog.V(4).Infof("Sending response: %s", string(resp))
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Failed to write response: %v", err)
		return
	}
}

// validateAutoscalingPolicy validates the AutoscalingPolicy resource
func (v *AutoscalingPolicyValidator) validateAutoscalingPolicy(policy *registryv1.AutoscalingPolicy) (bool, string) {
	var allErrs field.ErrorList

	// Validate metrics
	allErrs = append(allErrs, v.validateMetrics(policy)...)

	// Validate scale down behavior
	allErrs = append(allErrs, v.validateScaleDownBehavior(policy)...)

	// Validate scale up behavior
	allErrs = append(allErrs, v.validateScaleUpBehavior(policy)...)

	if len(allErrs) > 0 {
		var messages []string
		for _, err := range allErrs {
			messages = append(messages, fmt.Sprintf("  - %s", err.Error()))
		}
		return false, fmt.Sprintf("validation failed:\n%s", strings.Join(messages, "\n"))
	}
	return true, ""
}

// validateMetrics validates the metrics configuration
func (v *AutoscalingPolicyValidator) validateMetrics(policy *registryv1.AutoscalingPolicy) field.ErrorList {
	var allErrs field.ErrorList
	metricNames := make(map[string]bool)

	for i, metric := range policy.Spec.Metrics {
		metricPath := field.NewPath("spec").Child("metrics").Index(i)

		// Validate target value
		if metric.TargetValue.AsFloat64Slow() <= 0 || math.IsInf(metric.TargetValue.AsFloat64Slow(), 0) {
			allErrs = append(allErrs, field.Invalid(
				metricPath.Child("targetValue"),
				metric.TargetValue,
				"metric target value must be greater than 0 and not equal to infinity",
			))
		}

		// Validate metric name uniqueness
		if metricNames[metric.MetricName] {
			allErrs = append(allErrs, field.Invalid(
				metricPath.Child("metricName"),
				metric.MetricName,
				fmt.Sprintf("duplicate metric name %s is not allowed", metric.MetricName),
			))
		}
		metricNames[metric.MetricName] = true
	}

	return allErrs
}

// validateScaleDownBehavior validates the scale down behavior configuration
func (v *AutoscalingPolicyValidator) validateScaleDownBehavior(policy *registryv1.AutoscalingPolicy) field.ErrorList {
	var allErrs field.ErrorList
	scaleDownPath := field.NewPath("spec").Child("behavior").Child("scaleDown")
	stablePolicy := policy.Spec.Behavior.ScaleDown

	// Validate period
	if stablePolicy.Period.Seconds() < 0 || stablePolicy.Period.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			scaleDownPath.Child("period"),
			stablePolicy.Period,
			"stable policy period must be between 0 and 30 minutes",
		))
	}

	// Validate stabilization window
	if stablePolicy.StabilizationWindow != nil &&
		(stablePolicy.StabilizationWindow.Seconds() < 0 || stablePolicy.StabilizationWindow.Minutes() > 30) {
		allErrs = append(allErrs, field.Invalid(
			scaleDownPath.Child("stabilizationWindow"),
			stablePolicy.StabilizationWindow,
			"stable policy stabilization window must be between 0 and 30 minutes",
		))
	}

	return allErrs
}

// validateScaleUpBehavior validates the scale up behavior configuration
func (v *AutoscalingPolicyValidator) validateScaleUpBehavior(policy *registryv1.AutoscalingPolicy) field.ErrorList {
	var allErrs field.ErrorList
	scaleUpPath := field.NewPath("spec").Child("behavior").Child("scaleUp")

	// Validate stable policy
	allErrs = append(allErrs, v.validateStablePolicy(policy, scaleUpPath)...)

	// Validate panic policy
	allErrs = append(allErrs, v.validatePanicPolicy(policy, scaleUpPath)...)

	return allErrs
}

// validateStablePolicy validates the stable policy configuration for scale up
func (v *AutoscalingPolicyValidator) validateStablePolicy(policy *registryv1.AutoscalingPolicy, scaleUpPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	stablePolicyPath := scaleUpPath.Child("stablePolicy")
	stablePolicy := policy.Spec.Behavior.ScaleUp.StablePolicy

	// Validate period
	if stablePolicy.Period.Seconds() < 0 || stablePolicy.Period.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			stablePolicyPath.Child("period"),
			stablePolicy.Period,
			"stable policy period must be between 0 and 30 minutes",
		))
	}

	// Validate stabilization window
	if stablePolicy.StabilizationWindow != nil &&
		(stablePolicy.StabilizationWindow.Seconds() < 0 || stablePolicy.StabilizationWindow.Minutes() > 30) {
		allErrs = append(allErrs, field.Invalid(
			stablePolicyPath.Child("stabilizationWindow"),
			stablePolicy.StabilizationWindow,
			"stable policy stabilization window must be between 0 and 30 minutes",
		))
	}

	return allErrs
}

// validatePanicPolicy validates the panic policy configuration for scale up
func (v *AutoscalingPolicyValidator) validatePanicPolicy(policy *registryv1.AutoscalingPolicy, scaleUpPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	panicPolicyPath := scaleUpPath.Child("panicPolicy")
	panicPolicy := policy.Spec.Behavior.ScaleUp.PanicPolicy

	// Validate period
	if panicPolicy.Period.Seconds() < 0 || panicPolicy.Period.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			panicPolicyPath.Child("period"),
			panicPolicy.Period,
			"panic policy period must be between 0 and 30 minutes",
		))
	}

	// Validate panic mode hold
	if panicPolicy.PanicModeHold.Seconds() < 0 || panicPolicy.PanicModeHold.Minutes() > 30 {
		allErrs = append(allErrs, field.Invalid(
			panicPolicyPath.Child("panicModeHold"),
			panicPolicy.PanicModeHold,
			"panic policy panic mode hold must be between 0 and 30 minutes",
		))
	}

	return allErrs
}
