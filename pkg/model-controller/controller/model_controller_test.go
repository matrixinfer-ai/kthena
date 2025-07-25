package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	matrixinferfake "matrixinfer.ai/matrixinfer/client-go/clientset/versioned/fake"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
)

// TestReconcile tests the reconcile function for ModelController
func TestReconcile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create fake clients for Kubernetes and MatrixInfer
	kubeClient := fake.NewSimpleClientset()
	matrixinferClient := matrixinferfake.NewSimpleClientset()

	controller := NewModelController(kubeClient, matrixinferClient)
	assert.NotNil(t, controller)
	go controller.Run(ctx, 1)

	model := utils.LoadYAML[registry.Model](t, "../utils/testdata/input/model.yaml")

	// test create model
	t.Run("ModelCreate", func(t *testing.T) {
		createdModel, err := matrixinferClient.RegistryV1alpha1().Models("default").Create(ctx, model, metav1.CreateOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, createdModel)

		err = controller.reconcile(ctx, "default/test-model")
		assert.NoError(t, err)

		modelInfers, err := matrixinferClient.WorkloadV1alpha1().ModelInfers("default").List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		assert.Len(t, modelInfers.Items, 1, "Expected 1 ModelInfer to be created")

		get, err := matrixinferClient.RegistryV1alpha1().Models("default").Get(ctx, "test-model", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, int32(1), get.Status.ObservedGeneration, "ObservedGeneration not updated")
	})
}

// TestUpdateModelStatus tests the updateModelStatus function
func TestUpdateModelStatus(t *testing.T) {
	ctx := context.Background()
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewClientset()
	controller := NewModelController(kubeClient, matrixinferClient)

	// Create test Model with backend
	model := &registry.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "status-test-model",
			Namespace: "default",
			UID:       "status-test-uid",
		},
		Spec: registry.ModelSpec{
			Backends: []registry.ModelBackend{
				{
					Name: "status-test-backend",
				},
			},
		},
	}
	_, err := matrixinferClient.RegistryV1alpha1().Models("default").Create(ctx, model, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create associated ModelInfer
	modelInfer := &workload.ModelInfer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "status-test-model-infer",
			Namespace: "default",
			Labels: map[string]string{
				"owner": string(model.UID),
			},
		},
		Status: workload.ModelInferStatus{
			Replicas: 2,
		},
	}
	_, err = matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Create(ctx, modelInfer, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Update model status
	err = controller.updateModelStatus(ctx, model)
	assert.NoError(t, err)

	// Verify status updates
	updatedModel, err := matrixinferClient.RegistryV1alpha1().Models("default").Get(ctx, "status-test-model", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, int32(1), updatedModel.Status.ObservedGeneration)
	assert.Len(t, updatedModel.Status.BackendStatuses, 1)
	assert.Equal(t, int32(2), updatedModel.Status.BackendStatuses[0].Replicas)
}

// TestIsModelInferActive tests the isModelInferActive function
func TestIsModelInferActive(t *testing.T) {
	ctx := context.Background()
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewClientset()
	controller := NewModelController(kubeClient, matrixinferClient)

	// Create test Model
	model := &registry.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "active-test-model",
			Namespace: "default",
			UID:       "active-test-uid",
		},
	}
	_, err := matrixinferClient.RegistryV1alpha1().Models("default").Create(ctx, model, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create active ModelInfer
	modelInfer := &workload.ModelInfer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "active-test-model-infer",
			Namespace: "default",
			Labels: map[string]string{
				"owner": string(model.UID),
			},
		},
		Status: workload.ModelInferStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(workload.ModelInferAvailable),
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	_, err = matrixinferClient.WorkloadV1alpha1().ModelInfers("default").Create(ctx, modelInfer, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Check if ModelInfer is active
	active, err := controller.isModelInferActive(ctx, model)
	assert.NoError(t, err)
	assert.True(t, active, "Expected ModelInfer to be active")
}
