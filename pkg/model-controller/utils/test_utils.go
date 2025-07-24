package utils

import (
	"os"
	"testing"

	"sigs.k8s.io/yaml"
)

// LoadYAML transfer yaml data into a struct of type T.
// Used for test.
func LoadYAML[T any](t *testing.T, path string) *T {
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
