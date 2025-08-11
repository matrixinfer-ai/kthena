package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// AutoscalingBindingValidator handles validation of Model resources
type AutoscalingBindingValidator struct {
	kubeClient kubernetes.Interface
	client     clientset.Interface
}

// NewAutoscalingBindingValidator creates a new AutoscalingBindingValidator
func NewAutoscalingBindingValidator(kubeClient kubernetes.Interface, client clientset.Interface) *AutoscalingBindingValidator {
	return &AutoscalingBindingValidator{
		kubeClient: kubeClient,
		client:     client,
	}
}

func (v *AutoscalingBindingValidator) Handle(w http.ResponseWriter, r *http.Request) {
	klog.V(3).Infof("received request: %s", r.URL.String())

	// Parse the admission request
	admissionReview, asp_binding, err := parseAdmissionRequest[registryv1alpha1.AutoscalingPolicyBinding](r)
	if err != nil {
		klog.Errorf("Failed to parse admission request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the Model
	allowed, reason := v.validateAutoscalingBinding(asp_binding)
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
	if err := sendAdmissionResponse(w, admissionReview); err != nil {
		klog.Errorf("Failed to send admission response: %v", err)
		http.Error(w, fmt.Sprintf("could not send response: %v", err), http.StatusInternalServerError)
		return
	}

}

// validateModel validates the Model resource
func (v *AutoscalingBindingValidator) validateAutoscalingBinding(asp_binding *registryv1alpha1.AutoscalingPolicyBinding) (bool, string) {
	ctx := context.Background()
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateOptimizeAndScalingPolicyExistence(ctx, asp_binding)...)

	if len(allErrs) > 0 {
		// Convert field errors to a formatted multi-line error message
		var messages []string
		for _, err := range allErrs {
			messages = append(messages, fmt.Sprintf("  - %s", err.Error()))
		}
		return false, fmt.Sprintf("validation failed:\n%s", strings.Join(messages, "\n"))
	}
	return true, ""
}

func validateOptimizeAndScalingPolicyExistence(ctx context.Context, asp_binding *registryv1alpha1.AutoscalingPolicyBinding) field.ErrorList {
	var allErrs field.ErrorList
	if asp_binding.Spec.OptimizerConfiguration == nil && asp_binding.Spec.ScalingConfiguration == nil {
		allErrs = append(allErrs, field.NotFound(field.NewPath("spec", "OptimizerConfiguration", "ScalingConfiguration"), asp_binding.Spec.OptimizerConfiguration))
	}
	if asp_binding.Spec.OptimizerConfiguration != nil && asp_binding.Spec.ScalingConfiguration != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "OptimizerConfiguration", "ScalingConfiguration"), "both spec.OptimizerConfiguration and spec.ScalingConfiguration can not exist at the same time"))
	}
	return allErrs
}
