package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"sigs.k8s.io/yaml"
)

func TestBuildModelRoute(t *testing.T) {
	tests := []struct {
		name     string
		input    *registry.Model
		expected *networking.ModelRoute
	}{
		{
			name:     "simple backend",
			input:    loadYaml[registry.Model](t, "testdata/input/model.yaml"),
			expected: loadYaml[networking.ModelRoute](t, "testdata/expected/model-route.yaml"),
		},
		{
			name:     "model with multiple backends",
			input:    loadYaml[registry.Model](t, "testdata/input/multi-backends-model.yaml"),
			expected: loadYaml[networking.ModelRoute](t, "testdata/expected/model-route-subset.yaml"),
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
