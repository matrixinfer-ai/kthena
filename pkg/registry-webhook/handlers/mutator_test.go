package handlers

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

func TestCreatePatch(t *testing.T) {
	// Create an original model
	original := &registryv1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: registryv1alpha1.ModelSpec{
			AutoscalingPolicyRef: corev1.LocalObjectReference{
				Name: "test-policy",
			},
			Backends: []registryv1alpha1.ModelBackend{
				{
					Name:        "backend1",
					Type:        "vLLM",
					ModelURI:    "hf://test/model",
					MinReplicas: 1,
					MaxReplicas: 10,
					Workers: []registryv1alpha1.ModelWorker{
						{
							Type:     "server",
							Image:    "test-image",
							Replicas: 1,
						},
					},
				},
			},
		},
	}

	// Create a mutated model and apply the actual mutations
	mutated := original.DeepCopy()

	mutator := NewModelMutator(nil, nil) // nil clients are fine since mutateModel doesn't use them
	mutator.mutateModel(mutated)

	// Test the createPatch function
	patch, err := createPatch(original, mutated)
	if err != nil {
		t.Fatalf("Error creating patch: %v", err)
	}

	// Verify that we got a valid patch
	if len(patch) == 0 {
		t.Fatal("Expected non-empty patch")
	}

	// Parse the patch to verify it's valid JSON
	var patchObj []interface{}
	if err := json.Unmarshal(patch, &patchObj); err != nil {
		t.Fatalf("Error unmarshaling patch: %v", err)
	}

	// Verify that we have patch operations
	if len(patchObj) == 0 {
		t.Fatal("Expected patch operations")
	}

	t.Logf("Patch created successfully with %d operations", len(patchObj))

	// Log the patch for debugging
	for i, op := range patchObj {
		t.Logf("Operation %d: %+v", i+1, op)
	}
}

func TestCreatePatchNoChanges(t *testing.T) {
	// Create a model
	original := &registryv1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: registryv1alpha1.ModelSpec{
			Backends: []registryv1alpha1.ModelBackend{
				{
					Name:        "backend1",
					Type:        "vLLM",
					ModelURI:    "hf://test/model",
					MinReplicas: 1,
					MaxReplicas: 10,
					Workers: []registryv1alpha1.ModelWorker{
						{
							Type:     "server",
							Image:    "test-image",
							Replicas: 1,
						},
					},
				},
			},
		},
	}

	// Create an identical copy
	mutated := original.DeepCopy()

	// Test the createPatch function
	patch, err := createPatch(original, mutated)
	if err != nil {
		t.Fatalf("Error creating patch: %v", err)
	}

	// Parse the patch
	var patchObj []interface{}
	if err := json.Unmarshal(patch, &patchObj); err != nil {
		t.Fatalf("Error unmarshaling patch: %v", err)
	}

	// Should have no operations for identical objects
	if len(patchObj) != 0 {
		t.Fatalf("Expected no patch operations for identical objects, got %d", len(patchObj))
	}

	t.Log("No patch operations created for identical objects - correct behavior")
}
