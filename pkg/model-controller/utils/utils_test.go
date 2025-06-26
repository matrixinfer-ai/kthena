package utils

import (
	"testing"

	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

func TestGetMountPath(t *testing.T) {
	tests := []struct {
		name     string
		input    *registry.ModelBackend
		expected string
	}{
		{
			name:     "normal case",
			input:    &registry.ModelBackend{ModelURI: "models/llama-2-7b"},
			expected: "/8590cc9fef9361779a5bd7862eb82b6d",
		},
		{
			name:     "empty modelURI",
			input:    &registry.ModelBackend{ModelURI: ""},
			expected: "/d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name:     "special characters",
			input:    &registry.ModelBackend{ModelURI: "model_@#$"},
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

// todo: more test case
