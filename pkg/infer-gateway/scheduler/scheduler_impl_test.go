package scheduler

import (
	"os"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

func TestNewScheduler(t *testing.T) {
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
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			patches := gomonkey.NewPatches()
			defer patches.Reset()
			patches = tc.fn(patches)

			_, _, _, errs := utils.LoadSchedulerConfig()
			if errs == nil && tc.expectErrs != "" {
				t.Errorf("expected error containing %q, got nil", tc.expectErrs)
			} else if errs != nil && !strings.Contains(errs.Error(), tc.expectErrs) {
				t.Errorf("unexpected error, got %v, want %q", errs, tc.expectErrs)
			}
			NewScheduler(datastore.New())
		})
	}
}
