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

	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
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
			name:     "CacheVolume_HuggingFace_HostPath",
			input:    loadInputYAML(t, "testdata/inputModel.yaml"),
			expected: []*workload.ModelInfer{loadExpectedYAML(t, "testdata/expectModelInfer.yaml")},
		},
		{
			name:     "PD disaggregation",
			input:    loadInputYAML(t, "testdata/inputPDModel.yaml"),
			expected: []*workload.ModelInfer{loadExpectedYAML(t, "testdata/expectModelInferDisaggregation.yaml")},
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

func loadExpectedYAML(t *testing.T, path string) *workload.ModelInfer {
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

func loadInputYAML(t *testing.T, path string) *registry.Model {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read YAML: %v", err)
	}
	var model registry.Model
	if err := yaml.Unmarshal(data, &model); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}
	return &model
}
