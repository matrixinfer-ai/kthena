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

package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// TestGetEnvValueOrDefault tests the GetEnvValueOrDefault function with various scenarios
func TestGetEnvValueOrDefault(t *testing.T) {
	testCases := []struct {
		name          string
		backend       *v1alpha1.ModelBackend
		envName       string
		defaultValue  any
		expectedValue any
	}{
		{
			name: "String type - env exists",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_STRING",
					Value: "test-value",
				}},
			},
			envName:       "TEST_STRING",
			defaultValue:  "default-string",
			expectedValue: "test-value",
		}, {
			name: "String type - env not exists",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{},
			},
			envName:       "TEST_STRING",
			defaultValue:  "default-string",
			expectedValue: "default-string",
		}, {
			name: "Int type - env exists and valid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_INT",
					Value: "42",
				}},
			},
			envName:       "TEST_INT",
			defaultValue:  0,
			expectedValue: 42,
		}, {
			name: "Int type - env exists but invalid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_INT",
					Value: "invalid-int",
				}},
			},
			envName:       "TEST_INT",
			defaultValue:  100,
			expectedValue: 100,
		},
		{
			name: "Uint type - env exists and valid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_UINT",
					Value: "42",
				}},
			},
			envName:       "TEST_UINT",
			defaultValue:  uint(0),
			expectedValue: uint(42),
		},
		{
			name: "Uint type - env exists but invalid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_UINT",
					Value: "invalid-uint",
				}},
			},
			envName:       "TEST_UINT",
			defaultValue:  uint(100),
			expectedValue: uint(100),
		},
		{
			name: "Bool type - env exists and valid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_BOOL",
					Value: "true",
				}},
			},
			envName:       "TEST_BOOL",
			defaultValue:  false,
			expectedValue: true,
		}, {
			name: "Bool type - env exists but invalid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_BOOL",
					Value: "invalid-bool",
				}},
			},
			envName:       "TEST_BOOL",
			defaultValue:  true,
			expectedValue: true,
		}, {
			name: "Float type - env exists and valid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_FLOAT",
					Value: "3.14",
				}},
			},
			envName:       "TEST_FLOAT",
			defaultValue:  0.0,
			expectedValue: 3.14,
		}, {
			name: "Float type - env exists but invalid",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_FLOAT",
					Value: "invalid-float",
				}},
			},
			envName:       "TEST_FLOAT",
			defaultValue:  0.0,
			expectedValue: 0.0,
		}, {
			name: "[]corev1.EnvVar type - env exists",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_ENV_VAR",
					Value: "env-value",
				}},
			},
			envName:       "TEST_ENV_VAR",
			defaultValue:  []corev1.EnvVar{{Name: "TEST_ENV_VAR", Value: ""}},
			expectedValue: []corev1.EnvVar{{Name: "TEST_ENV_VAR", Value: "env-value"}},
		}, {
			name: "[]corev1.EnvVar type - env not exists",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_ENV_VAR",
					Value: "env-value",
				}},
			},
			envName:       "TEST_ENV_VAR_NOT_EXISTS",
			defaultValue:  []corev1.EnvVar{{Name: "TEST_ENV_VAR", Value: ""}},
			expectedValue: []corev1.EnvVar{{Name: "TEST_ENV_VAR", Value: ""}},
		}, {
			name: "Unsupported type",
			backend: &v1alpha1.ModelBackend{
				Env: []corev1.EnvVar{{
					Name:  "TEST_UNSUPPORTED",
					Value: "value",
				}},
			},
			envName:       "TEST_UNSUPPORTED",
			defaultValue:  struct{}{},
			expectedValue: struct{}{},
		}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			switch defaultValue := tc.defaultValue.(type) {
			case string:
				result := GetEnvValueOrDefault[string](tc.backend, tc.envName, defaultValue)
				assert.Equal(t, tc.expectedValue, result)
			case int:
				result := GetEnvValueOrDefault[int](tc.backend, tc.envName, defaultValue)
				assert.Equal(t, tc.expectedValue, result)
			case uint:
				result := GetEnvValueOrDefault[uint](tc.backend, tc.envName, defaultValue)
				assert.Equal(t, tc.expectedValue, result)
			case bool:
				result := GetEnvValueOrDefault[bool](tc.backend, tc.envName, defaultValue)
				assert.Equal(t, tc.expectedValue, result)
			case float64:
				result := GetEnvValueOrDefault[float64](tc.backend, tc.envName, defaultValue)
				assert.Equal(t, tc.expectedValue, result)
			case []corev1.EnvVar:
				result := GetEnvValueOrDefault[[]corev1.EnvVar](tc.backend, tc.envName, defaultValue)
				assert.Equal(t, tc.expectedValue, result)
			case struct{}:
				// Testing unsupported type
				result := GetEnvValueOrDefault[struct{}](tc.backend, tc.envName, defaultValue)
				assert.Equal(t, tc.expectedValue, result)
			default:
				t.Errorf("Unsupported test type: %T", defaultValue)
			}
		})
	}
}
