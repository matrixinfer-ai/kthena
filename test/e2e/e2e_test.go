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

package e2e

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	matrixinferclientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

const (
	testNamespace = "dev"
)

func TestModelCR(t *testing.T) {
	ctx := context.Background()
	// Initialize Kubernetes clients
	config, err := getKubeConfig()
	require.NoError(t, err, "Failed to get kubeconfig")
	kubeClient, err := kubernetes.NewForConfig(config)
	require.NoError(t, err, "Failed to create Kubernetes client")
	matrixinferClient, err := matrixinferclientset.NewForConfig(config)
	require.NoError(t, err, "Failed to create MatrixInfer client")
	// Create a Model CR in the test namespace
	model := createTestModel()
	createdModel, err := matrixinferClient.RegistryV1alpha1().Models(testNamespace).Create(ctx, model, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create Model CR")
	assert.NotNil(t, createdModel)
	t.Logf("Created Model CR: %s/%s", createdModel.Namespace, createdModel.Name)
	// Wait for the Model to be Active
	require.Eventually(t, func() bool {
		model, err := matrixinferClient.RegistryV1alpha1().Models(testNamespace).Get(ctx, model.Name, metav1.GetOptions{})
		if err != nil {
			t.Logf("Get model error: %v", err)
			return false
		}
		return true == meta.IsStatusConditionPresentAndEqual(model.Status.Conditions,
			string(registryv1alpha1.ModelStatusConditionTypeActive), metav1.ConditionTrue)
	}, 5*time.Minute, 5*time.Second, "Model did not become Active")
	// Get infer-gateway Service IP
	svc, err := kubeClient.CoreV1().Services(testNamespace).Get(ctx, "networking-infer-gateway", metav1.GetOptions{})
	require.NoError(t, err, "Failed to get infer-gateway service")
	ip := svc.Spec.ClusterIP
	require.NotEmpty(t, ip, "infer-gateway ClusterIP is empty")
	// Test chat
	chatURL := "http://" + ip + "/v1/chat/completions"
	payload := `{
		"model": "test-model",
		"messages": [
			{"role": "user", "content": "Where is the capital of China?"}
		],
		"stream": false
	}`
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", chatURL, strings.NewReader(payload))
	require.NoError(t, err, "Failed to create chat request")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err, "Chat request failed")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Chat response status code should be 200")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read chat response body")
	t.Logf("Chat response: %s", string(body))
	assert.Contains(t, string(body), "Beijing", "Chat response should mention Beijing")
}

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
}

func createTestModel() *registryv1alpha1.Model {
	// Create a simple config as JSON
	config := &apiextensionsv1.JSON{}
	configRaw := `{
		"served-model-name": "test-model",
		"max-model-len": 32768,
		"max-num-batched-tokens": 65536,
		"block-size": 128,
		"enable-prefix-caching": ""
	}`
	config.Raw = []byte(configRaw)

	return &registryv1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: testNamespace,
		},
		Spec: registryv1alpha1.ModelSpec{
			Name: "test-model",
			Backends: []registryv1alpha1.ModelBackend{
				{
					Name:        "backend1",
					Type:        registryv1alpha1.ModelBackendTypeVLLM,
					ModelURI:    "hf://Qwen/Qwen2.5-0.5B-Instruct",
					CacheURI:    "hostpath:///tmp/cache",
					MinReplicas: 1,
					MaxReplicas: 1,
					Workers: []registryv1alpha1.ModelWorker{
						{
							Type:     registryv1alpha1.ModelWorkerTypeServer,
							Image:    "public.ecr.aws/q9t5s3a7/vllm-cpu-release-repo:latest",
							Replicas: 1,
							Pods:     1,
							Config:   *config,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("4Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("6Gi"),
								},
							},
						},
					},
				},
			},
		},
	}
}
