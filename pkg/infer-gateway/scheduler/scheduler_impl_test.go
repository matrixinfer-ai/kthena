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
