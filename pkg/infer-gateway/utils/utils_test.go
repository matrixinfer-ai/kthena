package utils

import (
	"os"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

func TestLoadSchedulerConfig(t *testing.T) {
	testData, err := os.ReadFile("testdata/configmap.yaml")
	if err != nil {
		t.Fatalf("Failed to read Yaml:%v", err)
	}

	testCases := []struct {
		name       string
		fn         func(patches *gomonkey.Patches) *gomonkey.Patches
		expectErrs string
	}{
		{
			name: "LoadSchedulerConfig success",
			fn: func(patches *gomonkey.Patches) *gomonkey.Patches {
				return patches.ApplyFunc(os.ReadFile, func(string) ([]byte, error) {
					return testData, nil
				})
			},
			expectErrs: "",
		}, {
			name: "empty plugins config",
			fn: func(patches *gomonkey.Patches) *gomonkey.Patches {
				return patches.ApplyFunc(os.ReadFile, func(string) ([]byte, error) {
					return []byte{}, nil
				})
			},
			expectErrs: "failed to Unmarshal Plugins: Plugins is nil",
		}, {
			name: "invalid YAML syntax",
			fn: func(patches *gomonkey.Patches) *gomonkey.Patches {
				return patches.ApplyFunc(os.ReadFile, func(string) ([]byte, error) {
					return []byte("{invalid: syntax}"), nil
				})
			},
			expectErrs: "failed to Unmarshal Plugins: Plugins is nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			patches := gomonkey.NewPatches()
			defer patches.Reset()
			patches = tc.fn(patches)

			_, _, errs := LoadSchedulerConfig()
			if errs == nil && tc.expectErrs != "" {
				t.Errorf("expected error containing %q, got nil", tc.expectErrs)
			} else if errs != nil && !strings.Contains(errs.Error(), tc.expectErrs) {
				t.Errorf("unexpected error, got %v, want %q", errs, tc.expectErrs)
			}
		})
	}
}
