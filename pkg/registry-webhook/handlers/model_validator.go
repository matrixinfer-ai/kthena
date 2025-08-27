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
	"fmt"
	"net/http"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// ModelValidator handles validation of Model resources
type ModelValidator struct {
}

// NewModelValidator creates a new ModelValidator
func NewModelValidator() *ModelValidator {
	return &ModelValidator{}
}

// Handle handles admission requests for Model resources
func (v *ModelValidator) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse the admission request
	admissionReview, model, err := parseAdmissionRequest[registryv1alpha1.Model](r)
	if err != nil {
		klog.Errorf("Failed to parse admission request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the Model
	allowed, reason := v.validateModel(model)

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
func (v *ModelValidator) validateModel(model *registryv1alpha1.Model) (bool, string) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateScaleToZeroGracePeriod(model)...)
	allErrs = append(allErrs, validateBackendReplicaBounds(model)...)
	allErrs = append(allErrs, validateWorkerImages(model)...)
	allErrs = append(allErrs, validateAutoScalingPolicyScope(model)...)
	allErrs = append(allErrs, validateBackendWorkerTypes(model)...)
	allErrs = append(allErrs, validateLoraAdapterName(model)...)

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

func validateBackendWorkerTypes(model *registryv1alpha1.Model) field.ErrorList {
	var allErrs field.ErrorList
	backendsPath := field.NewPath("spec").Child("backends")

	for i, backend := range model.Spec.Backends {
		workers := backend.Workers

		// Rule 1: vLLM, SGLang, MindIE -> exactly one worker, type 'server'
		if backend.Type == registryv1alpha1.ModelBackendTypeVLLM ||
			backend.Type == registryv1alpha1.ModelBackendTypeSGLang ||
			backend.Type == registryv1alpha1.ModelBackendTypeMindIE {
			if len(workers) != 1 {
				allErrs = append(allErrs, field.Invalid(
					backendsPath.Index(i).Child("workers"),
					len(workers),
					fmt.Sprintf("If backend type is '%s', there must be exactly one worker", backend.Type),
				))
			} else if workers[0].Type != registryv1alpha1.ModelWorkerTypeServer {
				allErrs = append(allErrs, field.Invalid(
					backendsPath.Index(i).Child("workers").Index(0).Child("type"),
					workers[0].Type,
					fmt.Sprintf("If backend type is '%s', the worker type must be 'server'", backend.Type),
				))
			}
		}

		// Rule 2: vLLMDisaggregated -> all workers must be 'prefill' or 'decode'
		if backend.Type == registryv1alpha1.ModelBackendTypeVLLMDisaggregated {
			for j, w := range workers {
				if w.Type != registryv1alpha1.ModelWorkerTypePrefill && w.Type != registryv1alpha1.ModelWorkerTypeDecode {
					allErrs = append(allErrs, field.Invalid(
						backendsPath.Index(i).Child("workers").Index(j).Child("type"),
						w.Type,
						"If backend type is 'vLLMDisaggregated', all workers must be type 'prefill' or 'decode'",
					))
				}
			}
		}

		// Rule 3: MindIEDisaggregated -> all workers must be 'prefill', 'decode', 'controller', or 'coordinator'
		if backend.Type == registryv1alpha1.ModelBackendTypeMindIEDisaggregated {
			validTypes := map[registryv1alpha1.ModelWorkerType]struct{}{
				registryv1alpha1.ModelWorkerTypePrefill:     {},
				registryv1alpha1.ModelWorkerTypeDecode:      {},
				registryv1alpha1.ModelWorkerTypeController:  {},
				registryv1alpha1.ModelWorkerTypeCoordinator: {},
			}
			for j, w := range workers {
				if _, ok := validTypes[w.Type]; !ok {
					allErrs = append(allErrs, field.Invalid(
						backendsPath.Index(i).Child("workers").Index(j).Child("type"),
						w.Type,
						"If backend type is 'MindIEDisaggregated', all workers must be type 'prefill', 'decode', 'controller', or 'coordinator' (not 'server')",
					))
				}
			}
		}
	}

	return allErrs
}

func validateBackendReplicaBounds(model *registryv1alpha1.Model) field.ErrorList {
	var allErrs field.ErrorList
	path := field.NewPath("spec").Child("backends")
	const maxTotalReplicas = 1000000
	totalMaxReplicas := int32(0)
	for i, backend := range model.Spec.Backends {
		if backend.MinReplicas > backend.MaxReplicas {
			allErrs = append(allErrs, field.Invalid(
				path.Index(i).Child("minReplicas"),
				backend.MinReplicas,
				"minReplicas cannot be greater than maxReplicas",
			))
		}
		totalMaxReplicas += backend.MaxReplicas
	}
	if totalMaxReplicas > maxTotalReplicas {
		allErrs = append(allErrs, field.Invalid(
			path,
			totalMaxReplicas,
			fmt.Sprintf("sum of maxReplicas across all backends (%d) cannot exceed %d", totalMaxReplicas, maxTotalReplicas),
		))
	}
	return allErrs
}

func validateScaleToZeroGracePeriod(model *registryv1alpha1.Model) field.ErrorList {
	const maxScaleToZeroSeconds = 1800
	var allErrs field.ErrorList
	for i, backend := range model.Spec.Backends {
		if backend.ScaleToZeroGracePeriod == nil {
			continue
		}
		d := backend.ScaleToZeroGracePeriod.Duration
		if d > time.Duration(maxScaleToZeroSeconds)*time.Second {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("backends").Index(i).Child("scaleToZeroGracePeriod"),
				d.String(),
				fmt.Sprintf("scaleToZeroGracePeriod cannot exceed %d seconds", maxScaleToZeroSeconds),
			))
		}
		if d < 0 {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("backends").Index(i).Child("scaleToZeroGracePeriod"),
				d.String(),
				"scaleToZeroGracePeriod cannot be negative",
			))
		}
	}
	return allErrs
}

func validateWorkerImages(model *registryv1alpha1.Model) field.ErrorList {
	var allErrs field.ErrorList
	for i, backend := range model.Spec.Backends {
		for j, worker := range backend.Workers {
			if worker.Image != "" {
				if err := validateImageField(worker.Image); err != nil {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("backends").Index(i).Child("workers").Index(j).Child("image"),
						worker.Image,
						fmt.Sprintf("invalid container image reference: %v", err),
					))
				}
			}
		}
	}
	return allErrs
}

// validateImageField checks if a container image string is a valid Docker reference.
func validateImageField(image string) error {
	if image == "" {
		// Optional: return the error if you want to require the image field
		return nil
	}

	// Simple validation: check if image contains at least one character and no spaces
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("image cannot be empty or whitespace only")
	}

	if strings.Contains(image, " ") {
		return fmt.Errorf("image cannot contain spaces")
	}

	// Basic format check: should contain at least one character
	if len(strings.TrimSpace(image)) == 0 {
		return fmt.Errorf("invalid image format")
	}

	return nil
}

// validateAutoScalingPolicyScope validates the autoscaling field usage rules for Model.
func validateAutoScalingPolicyScope(model *registryv1alpha1.Model) field.ErrorList {
	spec := model.Spec
	var allErrs field.ErrorList

	modelAutoScalingEmpty := spec.AutoscalingPolicy == nil
	allBackendAutoScalingEmpty := true
	for _, backend := range spec.Backends {
		if backend.AutoscalingPolicy != nil {
			allBackendAutoScalingEmpty = false
			break
		}
	}

	if modelAutoScalingEmpty {
		for i, backend := range spec.Backends {
			if backend.ScalingCost != 0 {
				allErrs = append(allErrs, field.Forbidden(
					field.NewPath("spec").Child("backends").Index(i).Child("cost"),
					"cost must not be provided when model-level autoscaling is not set",
				))
			}
			if backend.ScaleToZeroGracePeriod != nil {
				allErrs = append(allErrs, field.Forbidden(
					field.NewPath("spec").Child("backends").Index(i).Child("scaleToZeroGracePeriod"),
					"scaleToZeroGracePeriod must not be provided when model-level autoscaling is not set",
				))
			}
		}
		if spec.CostExpansionRatePercent != nil {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec").Child("costExpansionRatePercent"),
				"costExpansionRatePercent must not be provided when model-level autoscaling is not set",
			))
		}
		for i, backend := range spec.Backends {
			if backend.AutoscalingPolicy != nil && backend.MinReplicas < 1 {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("backends").Index(i).Child("minReplicas"),
					backend.MinReplicas,
					"minReplicas must be >= 1 when backend-level autoscaling is set",
				))
			}
		}
		if allBackendAutoScalingEmpty {
			// Case 1 (No Auto Scaling): All backend autoscaling empty
			// minReplicas == maxReplicas for all backends
			for i, backend := range spec.Backends {
				if backend.MinReplicas != backend.MaxReplicas {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("backends").Index(i),
						fmt.Sprintf("minReplicas=%d, maxReplicas=%d", backend.MinReplicas, backend.MaxReplicas),
						"minReplicas and maxReplicas must be equal and > 0 when no autoscaling is set",
					))
				}
			}
		}
	} else {
		if allBackendAutoScalingEmpty {
			// Case 3 (Global Scope): Model autoscaling set, all backend autoscaling empty
			// minReplicas >= 0 for all backends, sum(minReplicas) >= 1
			// Cost, ScaleToZeroGracePeriod, CostExpansionRatePercent must be provided
			minSum := int32(0)
			for i, backend := range spec.Backends {
				if backend.MinReplicas < 0 {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("backends").Index(i).Child("minReplicas"),
						backend.MinReplicas,
						"minReplicas must be >= 0 when model-level autoscaling is set",
					))
				}
				minSum += backend.MinReplicas
				if backend.ScalingCost == 0 {
					allErrs = append(allErrs, field.Required(
						field.NewPath("spec").Child("backends").Index(i).Child("cost"),
						"cost must be provided when model-level autoscaling is set",
					))
				}
				if backend.ScaleToZeroGracePeriod == nil {
					allErrs = append(allErrs, field.Required(
						field.NewPath("spec").Child("backends").Index(i).Child("scaleToZeroGracePeriod"),
						"scaleToZeroGracePeriod must be provided when model-level autoscaling is set",
					))
				}
			}
			if minSum < 1 {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("backends"),
					minSum,
					"sum of all minReplicas must be >= 1 when model-level autoscaling is set",
				))
			}
			if spec.CostExpansionRatePercent == nil {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec").Child("costExpansionRatePercent"),
					"costExpansionRatePercent must be provided and > 0 when model-level autoscaling is set",
				))
			}
		} else {
			// Case 4: Both model and at least one backend set autoscaling -> error
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec").Child("autoscalingPolicyRef"),
				"spec.autoscalingPolicyRef and spec.backends[].autoscalingPolicyRef cannot both be set; choose model-level or backend-level autoscaling, not both",
			))
		}
	}
	return allErrs
}

func validateLoraAdapterName(model *registryv1alpha1.Model) field.ErrorList {
	var allErrs field.ErrorList
	modelName := model.Name
	spec := model.Spec

	for i, backend := range spec.Backends {
		if backend.LoraAdapters != nil {
			loraName := make(map[string]struct{})
			for j, lora := range backend.LoraAdapters {
				loraPath := field.NewPath("spec").Child("backends").Index(i).Child("loraAdapters").Index(j)
				if lora.Name == modelName {
					allErrs = append(allErrs, field.Invalid(
						loraPath.Child("name"),
						lora.Name,
						"lora name cannot be the same as model name"))
				}
				if _, exists := loraName[lora.Name]; exists {
					allErrs = append(allErrs, field.Invalid(
						loraPath.Child("name"),
						lora.Name,
						"lora name must be unique within the backend",
					))
				} else {
					loraName[lora.Name] = struct{}{}
				}
			}
		}
	}

	return allErrs
}
