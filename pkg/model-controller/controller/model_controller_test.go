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
	model := loadYaml[registry.Model](t, "../convert/testdata/input/model.yaml")

	// Case1: Create a model with ASP, and then model infer, model server, model route, ASP, ASP binding should be created.
	// Step1. Create model
	createdModel, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Create(ctx, model, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, createdModel)
	// Step2. Check that ASP, ASP binding, model infer, model server, model route are created
	assert.True(t, waitForCondition(func() bool {
		aspBindings, err := matrixinferClient.RegistryV1alpha1().AutoscalingPolicyBindings(model.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false
		}
		return len(aspBindings.Items) == 1
	}))
	// ASP should be created
	aspList, err := matrixinferClient.RegistryV1alpha1().AutoscalingPolicies(model.Namespace).List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, aspList.Items, 1, "Expected 1 AutoscalingPolicy to be created")
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
	// Step3. mock model infer status available
	modelInfer := &modelInfers.Items[0]
	meta.SetStatusCondition(&modelInfer.Status.Conditions, newCondition(string(workload.ModelInferAvailable),
		metav1.ConditionTrue, "AllGroupsReady", "AllGroupsReady"))
	_, err = matrixinferClient.WorkloadV1alpha1().ModelInfers(model.Namespace).UpdateStatus(ctx, modelInfer, metav1.UpdateOptions{})
	assert.NoError(t, err)
	// todo: Step4. Check that model condition should be active

	// Case2: update model weight, and model route should be updated.
	// Step1. update weight
	weight := uint32(50)
	model.Spec.Backends[0].RouteWeight = &weight
	model.Generation += 1
	_, err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Update(ctx, model, metav1.UpdateOptions{})
	assert.NoError(t, err)
	// Step2. Check that model route is updated
	assert.True(t, waitForCondition(func() bool {
		modelRoutes, err = matrixinferClient.NetworkingV1alpha1().ModelRoutes(model.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false
		}
		return weight == *modelRoutes.Items[0].Spec.Rules[0].TargetModels[0].Weight
	}))

	// Case3: delete model. Because we are not running in a real K8s cluster, model server, model route, model infer,
	// ASP and ASP binding will not be deleted automatically. So here only check if model is deleted.
	err = matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Delete(ctx, model.Name, metav1.DeleteOptions{})
	assert.NoError(t, err)
	assert.True(t, waitForCondition(func() bool {
		modelList, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false
		}
		return len(modelList.Items) == 0
	}))
}

func TestReconcile_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create fake clients for Kubernetes and MatrixInfer
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewSimpleClientset()
	controller := NewModelController(kubeClient, matrixinferClient)
	assert.NotNil(t, &controller)
	// start informers
	go controller.modelsInformer.RunWithContext(ctx)
	go controller.modelInfersInformer.RunWithContext(ctx)
	go controller.autoscalingPoliciesInformer.RunWithContext(ctx)
	go controller.autoscalingPolicyBindingsInformer.RunWithContext(ctx)
	// Case1: Invalid namespaceAndName
	t.Run("InvalidNameSpaceAndName", func(t *testing.T) {
		err := controller.reconcile(ctx, "//")
		assert.Errorf(t, err, "invalid resource key: //")
	})

	// Case2: Create Model infer failed
	t.Run("CreateModelInferFailed", func(t *testing.T) {
		model := &registry.Model{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "not-supported-model",
				Namespace: "default",
			},
			Spec: registry.ModelSpec{
				Backends: []registry.ModelBackend{
					{
						Name: "not-supported-backend-type",
						Type: registry.ModelBackendTypeMindIEDisaggregated,
					},
				},
			},
		}
		createdModel, err := matrixinferClient.RegistryV1alpha1().Models(model.Namespace).Create(ctx, model, metav1.CreateOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, createdModel)
		assert.True(t, waitForCondition(func() bool {
			err = controller.reconcile(ctx, model.Namespace+"/"+model.Name)
			return err.Error() == "not support model backend type: MindIEDisaggregated"
		}))
		// todo check status condition
	})
}

func TestCreateModel(t *testing.T) {
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewClientset()
	controller := NewModelController(kubeClient, matrixinferClient)
	controller.createModel("wrong")
}

func TestUpdateModel(t *testing.T) {
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewClientset()
	controller := NewModelController(kubeClient, matrixinferClient)
	assert.NotNil(t, &controller)
	model := loadYaml[registry.Model](t, "../convert/testdata/input/model.yaml")
	// invalid old
	controller.updateModel("invalid", model)
	assert.Equal(t, 0, controller.workQueue.Len())
	// invalid new
	controller.updateModel(model, "invalid")
	assert.Equal(t, 0, controller.workQueue.Len())
}

func TestDeleteModel(t *testing.T) {
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewClientset()
	controller := NewModelController(kubeClient, matrixinferClient)
	controller.deleteModel("invalid")
}

func TestTriggerModel(t *testing.T) {
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewClientset()
	controller := NewModelController(kubeClient, matrixinferClient)
	assert.NotNil(t, &controller)
	modelInfer := loadYaml[workload.ModelInfer](t, "../convert/testdata/expected/model-infer.yaml")
	// invalid new
	controller.triggerModel(modelInfer, "invalid")
	assert.Equal(t, 0, controller.workQueue.Len())
	// invalid old
	controller.triggerModel("invalid", modelInfer)
	assert.Equal(t, 0, controller.workQueue.Len())
}

func TestHasOnlyLoraAdaptersChanged(t *testing.T) {
	kubeClient := fake.NewClientset()
	matrixinferClient := matrixinferfake.NewClientset()
	controller := NewModelController(kubeClient, matrixinferClient)

	tests := []struct {
		name     string
		oldModel *registry.Model
		newModel *registry.Model
		expected bool
	}{
		{
			name: "No changes at all",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
					},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Only LoRA adapters changed - added new adapter",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
								{Name: "adapter3", ArtifactURL: "uri3"},
							},
						},
					},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
								{Name: "adapter2", ArtifactURL: "uri2"},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Only LoRA adapters changed - modified adapter",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
					},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1-modified"},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Backend name changed",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
					},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend2",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Model URI changed",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
					},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri-changed",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
								{Name: "adapter2", ArtifactURL: "uri2"},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Number of backends changed",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
					},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "backend1",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
						{
							Name:     "backend2",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri2",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "VLLM backend with LoRA adapters changed and other backend unchanged",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "vllm-backend",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
							},
						},
						{
							Name:     "sglang-backend",
							Type:     registry.ModelBackendTypeSGLang,
							ModelURI: "model-uri2",
						},
					},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name:     "vllm-backend",
							Type:     registry.ModelBackendTypeVLLM,
							ModelURI: "model-uri",
							LoraAdapters: []registry.LoraAdapter{
								{Name: "adapter1", ArtifactURL: "uri1"},
								{Name: "adapter2", ArtifactURL: "uri2"},
							},
						},
						{
							Name:     "sglang-backend",
							Type:     registry.ModelBackendTypeSGLang,
							ModelURI: "model-uri2",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Empty backends",
			oldModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{},
				},
			},
			newModel: &registry.Model{
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := controller.hasOnlyLoraAdaptersChanged(tt.oldModel, tt.newModel)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
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

// waitForCondition repeatedly checks a condition function until it returns true or a timeout occurs.
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
