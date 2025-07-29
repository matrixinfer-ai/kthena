package controller

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	matrixinferfake "matrixinfer.ai/matrixinfer/client-go/clientset/versioned/fake"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
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
	model := utils.LoadYAML[registry.Model](t, "../utils/testdata/input/model.yaml")
	// create model
	createdModel, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Create(ctx, model, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, createdModel)
	// wait for the controller to process the model creation
	select {
	case <-ctx.Done():
	case <-time.After(1 * time.Second):
	}
	// check if model infer is created
	modelInfers, err := matrixinferClient.WorkloadV1alpha1().ModelInfers(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, modelInfers.Items, 1, "Expected 1 ModelInfer to be created")

	get, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), get.Status.ObservedGeneration, "ObservedGeneration not updated")

	// make model infer status available
	modelInfer := &modelInfers.Items[0]
	meta.SetStatusCondition(&modelInfer.Status.Conditions, newCondition(string(workload.ModelInferAvailable),
		metav1.ConditionTrue, "AllGroupsReady", "AllGroupsReady"))
	modelInfer, err = matrixinferClient.WorkloadV1alpha1().ModelInfers(model.Namespace).UpdateStatus(ctx, modelInfer, metav1.UpdateOptions{})
	assert.NoError(t, err)
	assert.Equal(t, string(workload.ModelInferAvailable), modelInfer.Status.Conditions[0].Type, "ModelInfer condition type should be Available")

	select {
	case <-ctx.Done():
	case <-time.After(1 * time.Second):
	}

	// model condition should be active
	model, err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Get(ctx, model.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, true, meta.IsStatusConditionPresentAndEqual(model.Status.Conditions,
		string(registry.ModelStatusConditionTypeActive), metav1.ConditionTrue))

	modelServers, err := matrixinferClient.NetworkingV1alpha1().ModelServers(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, modelServers.Items, 1, "Expected 1 ModelServer to be created")
}
