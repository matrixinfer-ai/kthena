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

package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	matrixinferfake "matrixinfer.ai/matrixinfer/client-go/clientset/versioned/fake"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"sigs.k8s.io/yaml"
)

// TestReconcile first creates a model and then checks if the ModelInfer, ModelServer and ModelRoute are created as expected.
// Then the model is updated, check if ModelRoute is updated. At last, model will be deleted.
func TestReconcile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create fake clients for Kubernetes and MatrixInfer
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewSimpleClientset()
	controller := NewModelController(kubeClient, matrixinferClient)
	assert.NotNil(t, controller)
	// Start controller
	go controller.Run(ctx, 1)
	// Load test data
	model := loadYaml[registry.Model](t, "../utils/testdata/input/model.yaml")

	// Case1: create model, and then model infer, model server, model route should be created.
	createdModel, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Create(ctx, model, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, createdModel)
	// model infer should be created
	var modelInfers *workload.ModelInferList
	assert.True(t, waitForCondition(func() bool {
		modelInfers, err = matrixinferClient.WorkloadV1alpha1().ModelInfers(model.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false
		}
		return len(modelInfers.Items) == 1
	}))
	// model server should be created
	modelServers, err := matrixinferClient.NetworkingV1alpha1().ModelServers(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, modelServers.Items, 1, "Expected 1 ModelServer to be created")
	// model route should be created
	modelRoutes, err := matrixinferClient.NetworkingV1alpha1().ModelRoutes(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, modelRoutes.Items, 1, "Expected 1 ModelRoute to be create")
	// model should not be updated
	get, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), get.Status.ObservedGeneration, "ObservedGeneration not updated")
	// mock model infer status available
	modelInfer := &modelInfers.Items[0]
	meta.SetStatusCondition(&modelInfer.Status.Conditions, newCondition(string(workload.ModelInferAvailable),
		metav1.ConditionTrue, "AllGroupsReady", "AllGroupsReady"))
	modelInfer, err = matrixinferClient.WorkloadV1alpha1().ModelInfers(model.Namespace).UpdateStatus(ctx, modelInfer, metav1.UpdateOptions{})
	assert.NoError(t, err)
	// model infer is available, model condition will be updated, so mock generation to 1
	createdModel.Generation += 1
	_, err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Update(ctx, createdModel, metav1.UpdateOptions{})
	assert.NoError(t, err)
	assert.Equal(t, string(workload.ModelInferAvailable), modelInfer.Status.Conditions[0].Type, "ModelInfer condition type should be Available")
	// model condition should be active
	assert.True(t, waitForCondition(func() bool {
		model, err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return int64(1) == model.Status.ObservedGeneration
	}))
	assert.Equal(t, true, meta.IsStatusConditionPresentAndEqual(model.Status.Conditions,
		string(registry.ModelStatusConditionTypeActive), metav1.ConditionTrue))
	assert.Equal(t, model.Generation, model.Status.ObservedGeneration)

	// Case2: update model
	weight := uint32(50)
	model.Spec.Backends[0].RouteWeight = &weight
	model.Generation += 1
	_, err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Update(ctx, model, metav1.UpdateOptions{})
	assert.NoError(t, err)
	assert.True(t, waitForCondition(func() bool {
		modelRoutes, err = matrixinferClient.NetworkingV1alpha1().ModelRoutes(model.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false
		}
		return weight == *modelRoutes.Items[0].Spec.Rules[0].TargetModels[0].Weight
	}))

	// Case3: delete model
	err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Delete(ctx, model.Name, metav1.DeleteOptions{})
	assert.NoError(t, err)
}

// loadYaml transfer yaml data into a struct of type T.
// Used for test.
func loadYaml[T any](t *testing.T, path string) *T {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read YAML: %v", err)
	}
	var expected T
	if err := yaml.Unmarshal(data, &expected); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}
	return &expected
}

func waitForCondition(checkFunc func() bool) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if checkFunc() {
				return true
			}
		}
	}
}
