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
	"strings"
	"testing"
)

func TestLoadSchedulerConfig(t *testing.T) {
	testCases := []struct {
		name       string
		configFile string
		expectErrs string
	}{
		{
			name:       "LoadSchedulerConfig success",
			configFile: "testdata/configmap.yaml",
			expectErrs: "",
		},
		{
			name:       "empty plugins config",
			configFile: "non-existent-file.yaml",
			expectErrs: "no such file or directory",
		},
		{
			name:       "invalid YAML syntax",
			configFile: "testdata/configmap-invalid.yaml",
			expectErrs: "failed to Unmarshal schedulerConfiguration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, errs := LoadSchedulerConfig(tc.configFile)
			if errs == nil && tc.expectErrs != "" {
				t.Errorf("expected error containing %q, got nil", tc.expectErrs)
			} else if errs != nil && !strings.Contains(errs.Error(), tc.expectErrs) {
				t.Errorf("unexpected error, got %v, want %q", errs, tc.expectErrs)
			}
		})
	}
}
