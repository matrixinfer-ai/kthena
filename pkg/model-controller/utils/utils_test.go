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

package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"

	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

func TestGetMountPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal case",
			input:    "models/llama-2-7b",
			expected: "/8590cc9fef9361779a5bd7862eb82b6d",
		},
		{
			name:     "empty modelURI",
			input:    "",
			expected: "/d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name:     "special characters",
			input:    "model_@#$",
			expected: "/1f8d57abec22d679835ba0c38f634b06",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMountPath(tt.input); got != tt.expected {
				t.Errorf("getMountPath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildModelInferCR(t *testing.T) {
	tests := []struct {
		name         string
		input        *registry.Model
		expected     []*workload.ModelInfer
		expectErrMsg string
	}{
		{
			name: "CacheVolume_HuggingFace_HostPath",
			input: &registry.Model{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
					UID:       "randomUID",
				},
				Spec: registry.ModelSpec{
					Backends: []registry.ModelBackend{
						{
							Name: "backend1",
							Type: registry.ModelBackendTypeVLLM,
							Config: apiextensionsv1.JSON{
								Raw: []byte(`{"max-model-len": 32768, "block-size": 128, "trust-remote-code": "", "tensor-parallel-size": 2, "gpu-memory-utilization": 0.9}`),
							},
							MinReplicas: 1,
							ModelURI:    "s3://aios_models/deepseek-ai/DeepSeek-V3-W8A8/vllm-ascend",
							CacheURI:    "hostpath:///tmp/test",
							Env: []corev1.EnvVar{
								{
									Name:  "ENDPOINT",
									Value: "https://obs.test.com",
								},
								{
									Name:  "RUNTIME_PORT",
									Value: "8900",
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "test-secret",
										},
									},
								},
							},
							Workers: []registry.ModelWorker{
								{
									Type:  registry.ModelWorkerTypeServer,
									Pods:  1,
									Image: "vllm-server:latest",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:                            resource.MustParse("100m"),
											corev1.ResourceMemory:                         resource.MustParse("1Gi"),
											corev1.ResourceName("huawei.com/ascend-1980"): resource.MustParse("1"),
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []*workload.ModelInfer{loadTestYAML(t, "testdata/expectModelInfer.yaml")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildModelInferCR(tt.input)
			if tt.expectErrMsg != "" {
				assert.Contains(t, err.Error(), tt.expectErrMsg)
				return
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.expected, got)
		})
	}
}

func loadTestYAML(t *testing.T, path string) *workload.ModelInfer {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read YAML: %v", err)
	}
	var infer workload.ModelInfer
	if err := yaml.Unmarshal(data, &infer); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}
	return &infer
}
