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
	"time"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	registryv1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// AutoscalingPolicyMutator handles mutation of AutoscalingPolicy resources
type AutoscalingPolicyMutator struct {
	kubeClient        kubernetes.Interface
	matrixinferClient clientset.Interface
}

// NewAutoscalingPolicyMutator creates a new AutoscalingPolicyMutator
func NewAutoscalingPolicyMutator(kubeClient kubernetes.Interface, matrixinferClient clientset.Interface) *AutoscalingPolicyMutator {
	return &AutoscalingPolicyMutator{
		kubeClient:        kubeClient,
		matrixinferClient: matrixinferClient,
	}
}

// Handle handles admission requests for AutoscalingPolicy resources
func (m *AutoscalingPolicyMutator) Handle(w http.ResponseWriter, r *http.Request) {
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

	// Create a copy of the policy to mutate
	mutatedPolicy := policy.DeepCopy()

	// Apply mutations
	patch := mutateAutoscalingPolicy(mutatedPolicy)

	patchBytes, err := createPolicyPatch(patch)
	if err != nil {
		klog.Errorf("Failed to create patch: %v", err)
		http.Error(w, fmt.Sprintf("could not create patch: %v", err), http.StatusInternalServerError)
		return
	}

	// Create the admission response
	patchType := admissionv1.PatchTypeJSONPatch
	admissionResponse := admissionv1.AdmissionResponse{
		Allowed:   true,
		UID:       admissionReview.Request.UID,
		Patch:     patchBytes,
		PatchType: &patchType,
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

// mutateAutoscalingPolicy applies mutations to the AutoscalingPolicy resource
func mutateAutoscalingPolicy(policy *registryv1.AutoscalingPolicy) []jsonpatch.Operation {
	// Define default values
	DefaultScaleDown := registryv1.AutoscalingPolicyStablePolicy{
		Instances:           pointer.Int32(0),
		Percent:             pointer.Int32(100),
		Period:              &metav1.Duration{Duration: time.Minute},
		SelectPolicy:        registryv1.SelectPolicyOr,
		StabilizationWindow: &metav1.Duration{Duration: time.Minute * 5},
	}
	DefaultScaleUpStablePolicy := registryv1.AutoscalingPolicyStablePolicy{
		Instances:           pointer.Int32(4),
		Percent:             pointer.Int32(100),
		Period:              &metav1.Duration{Duration: time.Minute},
		SelectPolicy:        registryv1.SelectPolicyOr,
		StabilizationWindow: &metav1.Duration{Duration: 0},
	}
	DefaultScaleUpPanicPolicy := registryv1.AutoscalingPolicyPanicPolicy{
		Percent:               pointer.Int32(0),
		Period:                metav1.Duration{Duration: 0},
		PanicThresholdPercent: pointer.Int32(200),
		PanicModeHold:         &metav1.Duration{Duration: 0},
	}

	DefaultScaleUp := registryv1.AutoscalingPolicyScaleUpPolicy{
		StablePolicy: DefaultScaleUpStablePolicy,
		PanicPolicy:  DefaultScaleUpPanicPolicy,
	}
	var patch []jsonpatch.JsonPatchOperation

	// Only set default behavior if behavior doesn't exist
	if policy.Spec.Behavior == (registryv1.AutoscalingPolicyBehavior{}) {
		DefaultBehavior := registryv1.AutoscalingPolicyBehavior{
			ScaleUp:   DefaultScaleUp,
			ScaleDown: DefaultScaleDown,
		}
		patch = append(patch, jsonpatch.NewOperation("add", "/spec/behavior", DefaultBehavior))
		return patch
	}

	// Only set default scaleDown if it doesn't exist
	if policy.Spec.Behavior.ScaleDown == (registryv1.AutoscalingPolicyStablePolicy{}) {
		patch = append(patch, jsonpatch.NewOperation("add", "/spec/behavior/scaleDown", DefaultScaleDown))
	} else if policy.Spec.Behavior.ScaleDown.StabilizationWindow == nil {
		patch = append(patch, jsonpatch.NewOperation("add", "/spec/behavior/scaleDown/stabilizationWindow", "5m"))
	}

	if policy.Spec.Behavior.ScaleUp == (registryv1.AutoscalingPolicyScaleUpPolicy{}) {
		patch = append(patch, jsonpatch.NewOperation("add", "/spec/behavior/scaleUp", DefaultScaleUp))
		return patch
	}

	// Only set default scaleUp/stablePolicy if it doesn't exist
	if policy.Spec.Behavior.ScaleUp.StablePolicy == (registryv1.AutoscalingPolicyStablePolicy{}) {
		patch = append(patch, jsonpatch.NewOperation("add", "/spec/behavior/scaleUp/stablePolicy", DefaultScaleUpStablePolicy))
	} else if policy.Spec.Behavior.ScaleUp.StablePolicy.StabilizationWindow == nil {
		patch = append(patch, jsonpatch.NewOperation("add", "/spec/behavior/scaleUp/stablePolicy/stabilizationWindow", "0s"))
	}

	// Only set default scaleUp/panicPolicy if it doesn't exist
	if policy.Spec.Behavior.ScaleUp.PanicPolicy == (registryv1.AutoscalingPolicyPanicPolicy{}) {
		patch = append(patch, jsonpatch.NewOperation("add", "/spec/behavior/scaleUp/panicPolicy", DefaultScaleUpPanicPolicy))
	}
	return patch
}

// createPolicyPatch creates a JSON patch between the original and mutated policy
// It handles missing parent paths by creating them step by step
func createPolicyPatch(patch []jsonpatch.Operation) ([]byte, error) {

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %v", err)
	}

	return patchBytes, nil
}
