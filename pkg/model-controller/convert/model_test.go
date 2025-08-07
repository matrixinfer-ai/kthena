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

package convert

import (
	"os"
	"testing"

	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"

	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
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
			if got := GetMountPath(tt.input); got != tt.expected {
				t.Errorf("GetMountPath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetCachePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal case",
			input:    "pvc://my-cache-path",
			expected: "my-cache-path",
		},
		{
			name:     "empty cache path",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid cache path",
			input:    "invalidpath",
			expected: "",
		},
		{
			name:     "multiple separators",
			input:    "pvc://path/with/multiple/separators",
			expected: "path/with/multiple/separators",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCachePath(tt.input); got != tt.expected {
				t.Errorf("GetCachePath() = %v, want %v", got, tt.expected)
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
			name:     "CacheVolume_HuggingFace_HostPath",
			input:    loadYAML[registry.Model](t, "testdata/input/model.yaml"),
			expected: []*workload.ModelInfer{loadYAML[workload.ModelInfer](t, "testdata/expected/model-infer.yaml")},
		},
		{
			name:     "PD disaggregation",
			input:    loadYAML[registry.Model](t, "testdata/input/pd-disaggregated-model.yaml"),
			expected: []*workload.ModelInfer{loadYAML[workload.ModelInfer](t, "testdata/expected/disaggregated-model-infer.yaml")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateModelInferResources(tt.input)
			if tt.expectErrMsg != "" {
				assert.Contains(t, err.Error(), tt.expectErrMsg)
				return
			} else {
				assert.NoError(t, err)
			}
			actualYAML, _ := yaml.Marshal(got)
			expectedYAML, _ := yaml.Marshal(tt.expected)
			assert.Equal(t, string(expectedYAML), string(actualYAML))
		})
	}
}

func TestBuildModelServer(t *testing.T) {
	tests := []struct {
		name         string
		input        *registry.Model
		expected     []*networking.ModelServer
		expectErrMsg string
	}{
		{
			name:     "PD disaggregation",
			input:    loadYAML[registry.Model](t, "testdata/input/pd-disaggregated-model.yaml"),
			expected: []*networking.ModelServer{loadYAML[networking.ModelServer](t, "testdata/expected/pd-model-server.yaml")},
		},
		{
			name:     "normal case",
			input:    loadYAML[registry.Model](t, "testdata/input/model.yaml"),
			expected: []*networking.ModelServer{loadYAML[networking.ModelServer](t, "testdata/expected/model-server.yaml")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildModelServer(tt.input)
			if tt.expectErrMsg != "" {
				assert.Contains(t, err.Error(), tt.expectErrMsg)
				return
			} else {
				assert.NoError(t, err)
			}
			actualYAML, _ := yaml.Marshal(got)
			expectedYAML, _ := yaml.Marshal(tt.expected)
			assert.Equal(t, string(expectedYAML), string(actualYAML))
		})
	}
}

func TestBuildModelRoute(t *testing.T) {
	tests := []struct {
		name     string
		input    *registry.Model
		expected *networking.ModelRoute
	}{
		{
			name:     "simple backend",
			input:    loadYAML[registry.Model](t, "testdata/input/model.yaml"),
			expected: loadYAML[networking.ModelRoute](t, "testdata/expected/model-route.yaml"),
		},
		{
			name:     "model with multiple backends",
			input:    loadYAML[registry.Model](t, "testdata/input/multi-backends-model.yaml"),
			expected: loadYAML[networking.ModelRoute](t, "testdata/expected/model-route-subset.yaml"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildModelRoute(tt.input)
			actualYAML, _ := yaml.Marshal(got)
			expectedYAML, _ := yaml.Marshal(tt.expected)
			assert.Equal(t, string(expectedYAML), string(actualYAML))
		})
	}
}

func TestBuildScalingPolicyBinding(t *testing.T) {
	tests := []struct {
		name     string
		input    *registry.Model
		expected *registry.AutoscalingPolicyBinding
	}{
		{
			name:     "simple backend",
			input:    loadYAML[registry.Model](t, "testdata/input/model.yaml"),
			expected: loadYAML[registry.AutoscalingPolicyBinding](t, "testdata/expected/scaling-asp-binding.yaml"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, backend := range tt.input.Spec.Backends {
				got := BuildScalingPolicyBinding(tt.input, &backend, utils.GetBackendResourceName(tt.input.Name, backend.Name))
				actualYAML, _ := yaml.Marshal(got)
				expectedYAML, _ := yaml.Marshal(tt.expected)
				assert.Equal(t, string(expectedYAML), string(actualYAML))
			}
		})
	}
}

func TestBuildOptimizePolicyBinding(t *testing.T) {
	tests := []struct {
		name     string
		input    *registry.Model
		expected *registry.AutoscalingPolicyBinding
	}{
		{
			name:     "model with multiple backends",
			input:    loadYAML[registry.Model](t, "testdata/input/multi-backends-model.yaml"),
			expected: loadYAML[registry.AutoscalingPolicyBinding](t, "testdata/expected/optimize-asp-binding.yaml"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildOptimizePolicyBinding(tt.input, utils.GetBackendResourceName(tt.input.Name, ""))
			actualYAML, _ := yaml.Marshal(got)
			expectedYAML, _ := yaml.Marshal(tt.expected)
			assert.Equal(t, string(expectedYAML), string(actualYAML))
		})
	}
}

func TestBuildAutoscalingPolicy(t *testing.T) {
	tests := []struct {
		name     string
		input    *registry.Model
		expected *registry.AutoscalingPolicy
	}{
		{
			name:     "simple-backend",
			input:    loadYAML[registry.Model](t, "testdata/input/model.yaml"),
			expected: loadYAML[registry.AutoscalingPolicy](t, "testdata/expected/scaling-asp.yaml"),
		},
		{
			name:     "multi-backends",
			input:    loadYAML[registry.Model](t, "testdata/input/multi-backends-model.yaml"),
			expected: loadYAML[registry.AutoscalingPolicy](t, "testdata/expected/optimize-asp.yaml"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input.Spec.AutoscalingPolicy != nil {
				got := BuildAutoscalingPolicy(tt.input.Spec.AutoscalingPolicy, tt.input, "")
				actualYAML, _ := yaml.Marshal(got)
				expectedYAML, _ := yaml.Marshal(tt.expected)
				assert.Equal(t, string(expectedYAML), string(actualYAML))
			} else {
				for _, backend := range tt.input.Spec.Backends {
					if backend.AutoscalingPolicy == nil {
						continue
					}
					got := BuildAutoscalingPolicy(backend.AutoscalingPolicy, tt.input, backend.Name)
					actualYAML, _ := yaml.Marshal(got)
					expectedYAML, _ := yaml.Marshal(tt.expected)
					assert.Equal(t, string(expectedYAML), string(actualYAML))
				}
			}
		})
	}
}

func loadYAML[T any](t *testing.T, path string) *T {
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
