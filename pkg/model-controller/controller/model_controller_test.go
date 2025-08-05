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
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	matrixinferfake "matrixinfer.ai/matrixinfer/client-go/clientset/versioned/fake"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"os"
	"sigs.k8s.io/yaml"
	"testing"
)

// TestReconcile first creates a model and then checks if the ModelInfer, ModelServer and ModelRoute are created as expected.
// Then the model is updated, check if the ModelInfer, ModelServer and ModelRoute are updated.
// Eventually, model will be deleted, and ensure ModelInfer, ModelServer and ModelRoute are deleted as well.
func TestReconcile(t *testing.T) {
	ctx := context.Background()
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
	waitForReconcile(ctx, controller)
	// model infer should be created
	modelInfers, err := matrixinferClient.WorkloadV1alpha1().ModelInfers(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, modelInfers.Items, 1, "Expected 1 ModelInfer to be created")
	// model server should be created
	modelServers, err := matrixinferClient.NetworkingV1alpha1().ModelServers(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, modelServers.Items, 1, "Expected 1 ModelServer to be created")
	// model route should be created
	modelRoutes, err := matrixinferClient.NetworkingV1alpha1().ModelRoutes(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, modelRoutes.Items, 1, "Expected 1 ModelRoute to be create")
	// model should be not updated
	get, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), get.Status.ObservedGeneration, "ObservedGeneration not updated")
	// mock model infer status available
	modelInfer := &modelInfers.Items[0]
	meta.SetStatusCondition(&modelInfer.Status.Conditions, newCondition(string(workload.ModelInferAvailable),
		metav1.ConditionTrue, "AllGroupsReady", "AllGroupsReady"))
	modelInfer, err = matrixinferClient.WorkloadV1alpha1().ModelInfers(model.Namespace).UpdateStatus(ctx, modelInfer, metav1.UpdateOptions{})
	assert.NoError(t, err)
	assert.Equal(t, string(workload.ModelInferAvailable), modelInfer.Status.Conditions[0].Type, "ModelInfer condition type should be Available")
	waitForReconcile(ctx, controller)
	// model condition should be active
	model, err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, true, meta.IsStatusConditionPresentAndEqual(model.Status.Conditions,
		string(registry.ModelStatusConditionTypeActive), metav1.ConditionTrue))
	// todo Case2: update model, and then model infer, model server, model route should be updated.

	// delete model and check if ModelInfer and ModelServer are deleted
	err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Delete(ctx, model.Name, metav1.DeleteOptions{})
	assert.NoError(t, err)
}

// waitForReconcile waits for the controller to process the next work item.
func waitForReconcile(ctx context.Context, controller *ModelController) {
	stopCh := make(chan bool, 1)
	select {
	case <-ctx.Done():
	case stopCh <- controller.processNextWorkItem(ctx):
	}
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
