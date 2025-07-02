package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// ModelMutator handles mutation of Model resources
type ModelMutator struct {
	kubeClient        kubernetes.Interface
	matrixinferClient clientset.Interface
}

// NewModelMutator creates a new ModelMutator
func NewModelMutator(kubeClient kubernetes.Interface, matrixinferClient clientset.Interface) *ModelMutator {
	return &ModelMutator{
		kubeClient:        kubeClient,
		matrixinferClient: matrixinferClient,
	}
}

// Handle handles admission requests for Model resources
func (m *ModelMutator) Handle(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// Verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type=%s, expected application/json", contentType)
		http.Error(w, "invalid Content-Type, expected application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Parse the AdmissionReview request
	var admissionReview admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		klog.Errorf("Failed to decode body: %v", err)
		http.Error(w, "could not decode body", http.StatusBadRequest)
		return
	}

	// Get the Model from the request
	var model registryv1alpha1.Model
	if err := json.Unmarshal(admissionReview.Request.Object.Raw, &model); err != nil {
		klog.Errorf("Failed to decode Model: %v", err)
		http.Error(w, "could not decode Model", http.StatusBadRequest)
		return
	}

	// Create a copy of the model to mutate
	mutatedModel := model.DeepCopy()

	// Apply mutations
	m.mutateModel(mutatedModel)

	// Create the patch
	patch, err := createPatch(&model, mutatedModel)
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
		Patch:     patch,
		PatchType: &patchType,
	}

	// Create the admission review response
	admissionReview.Response = &admissionResponse

	// Send the response
	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Failed to encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	klog.V(4).Infof("Sending response: %s", string(resp))
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Failed to write response: %v", err)
	}
}

// mutateModel applies mutations to the Model resource
func (m *ModelMutator) mutateModel(model *registryv1alpha1.Model) {
	klog.Infof("Defaulting for Model %s", model.GetName())

	// Default ScaleToZeroGracePeriod for all backends if AutoscalingPolicyRef is set
	if model.Spec.AutoscalingPolicyRef.Name != "" {
		for i := range model.Spec.Backends {
			backend := &model.Spec.Backends[i]
			if backend.ScaleToZeroGracePeriod == nil {
				backend.ScaleToZeroGracePeriod = &metav1.Duration{Duration: 30 * time.Second}
			}
		}

		if model.Spec.CostExpansionRatePercent == nil {
			var value int32 = 200
			model.Spec.CostExpansionRatePercent = &value
		}
	}
}

// createPatch creates a JSON patch between the original and mutated model
func createPatch(original, mutated *registryv1alpha1.Model) ([]byte, error) {
	// Convert both objects to JSON
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original: %v", err)
	}

	mutatedJSON, err := json.Marshal(mutated)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mutated: %v", err)
	}

	// Create a JSON patch
	jp := &jsonpatch{}
	patch, err := jp.CreatePatch(originalJSON, mutatedJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create patch: %v", err)
	}

	// Marshal the patch
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %v", err)
	}

	return patchBytes, nil
}

// jsonpatch is a simple implementation of JSON patch
type jsonpatch struct{}

// CreatePatch creates a JSON patch between two JSON documents
func (jsonpatch) CreatePatch(original, modified []byte) ([]interface{}, error) {
	var originalObj, modifiedObj interface{}

	if err := json.Unmarshal(original, &originalObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal original: %v", err)
	}

	if err := json.Unmarshal(modified, &modifiedObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal modified: %v", err)
	}

	// Create a patch
	patch := createJSONPatch("", originalObj, modifiedObj)
	return patch, nil
}

// createJSONPatch recursively creates a JSON patch between two objects
func createJSONPatch(path string, original, modified interface{}) []interface{} {
	var patch []interface{}

	// Handle nil values
	if original == nil && modified == nil {
		return patch
	}

	if original == nil {
		// Add operation
		patch = append(patch, map[string]interface{}{
			"op":    "add",
			"path":  path,
			"value": modified,
		})
		return patch
	}

	if modified == nil {
		// Remove operation
		patch = append(patch, map[string]interface{}{
			"op":   "remove",
			"path": path,
		})
		return patch
	}

	// Handle different types
	switch originalValue := original.(type) {
	case map[string]interface{}:
		modifiedValue, ok := modified.(map[string]interface{})
		if !ok {
			// Replace operation
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  path,
				"value": modified,
			})
			return patch
		}

		// Process each key in original
		for key, value := range originalValue {
			var newPath string
			if path == "" {
				newPath = "/" + key
			} else {
				newPath = path + "/" + key
			}

			if modifiedValue, ok := modifiedValue[key]; ok {
				// Key exists in both, recurse
				patch = append(patch, createJSONPatch(newPath, value, modifiedValue)...)
			} else {
				// Key removed in modified
				patch = append(patch, map[string]interface{}{
					"op":   "remove",
					"path": newPath,
				})
			}
		}

		// Process keys added in modified
		for key, value := range modifiedValue {
			if _, ok := originalValue[key]; !ok {
				// Key added in modified
				var newPath string
				if path == "" {
					newPath = "/" + key
				} else {
					newPath = path + "/" + key
				}
				patch = append(patch, map[string]interface{}{
					"op":    "add",
					"path":  newPath,
					"value": value,
				})
			}
		}

	case []interface{}:
		modifiedValue, ok := modified.([]interface{})
		if !ok {
			// Replace operation
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  path,
				"value": modified,
			})
			return patch
		}

		// For arrays, we'll just replace the entire array
		if !equalArrays(originalValue, modifiedValue) {
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  path,
				"value": modified,
			})
		}

	default:
		// For primitive types, just compare values
		if !equalValues(original, modified) {
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  path,
				"value": modified,
			})
		}
	}

	return patch
}

// equalArrays checks if two arrays are equal
func equalArrays(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equalValues(a[i], b[i]) {
			return false
		}
	}
	return true
}

// equalValues checks if two values are equal
func equalValues(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
