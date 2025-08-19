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

package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"matrixinfer.ai/matrixinfer/client-go/clientset/versioned/fake"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

func TestValidateAutoscalingBinding(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&registryv1alpha1.AutoscalingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummy-policy",
			Namespace: "default",
		},
		Spec: registryv1alpha1.AutoscalingPolicySpec{},
	})
	validator := NewAutoscalingBindingValidator(fakeClient)

	tests := []struct {
		name     string
		input    *registryv1alpha1.AutoscalingPolicyBinding
		expected []string
	}{
		{
			name: "optimizer and scaling config both set to nil",
			input: &registryv1alpha1.AutoscalingPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy-model",
					Namespace: "default",
				},
				Spec: registryv1alpha1.AutoscalingPolicyBindingSpec{
					PolicyRef: corev1.LocalObjectReference{
						Name: "dummy-policy",
					},
					OptimizerConfiguration: nil,
					ScalingConfiguration:   nil,
				},
			},
			expected: []string{"  - spec.ScalingConfiguration: Required value: spec.ScalingConfiguration should be set if spec.OptimizerConfiguration does not exist"},
		},
		{
			name: "optimizer and scaling config both are not nil",
			input: &registryv1alpha1.AutoscalingPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy-model",
					Namespace: "default",
				},
				Spec: registryv1alpha1.AutoscalingPolicyBindingSpec{
					PolicyRef: corev1.LocalObjectReference{
						Name: "dummy-policy",
					},
					OptimizerConfiguration: &registryv1alpha1.OptimizerConfiguration{
						Params: []registryv1alpha1.OptimizerParam{
							{
								Target: registryv1alpha1.Target{
									TargetRef: corev1.ObjectReference{
										Name: "target-name",
									},
								},
								MinReplicas: 1,
								MaxReplicas: 2,
							},
						},
						CostExpansionRatePercent: 100,
					},
					ScalingConfiguration: &registryv1alpha1.ScalingConfiguration{
						Target: registryv1alpha1.Target{
							TargetRef: corev1.ObjectReference{
								Name: "target-name",
							},
						},
						MinReplicas: 1,
						MaxReplicas: 2,
					},
				},
			},
			expected: []string{"  - spec.ScalingConfiguration: Forbidden: both spec.OptimizerConfiguration and spec.ScalingConfiguration can not be set at the same time"},
		},
		{
			name: "different autoscaling policy name",
			input: &registryv1alpha1.AutoscalingPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy-model",
					Namespace: "default",
				},
				Spec: registryv1alpha1.AutoscalingPolicyBindingSpec{
					PolicyRef: corev1.LocalObjectReference{
						Name: "not-exist-policy",
					},
					OptimizerConfiguration: nil,
					ScalingConfiguration: &registryv1alpha1.ScalingConfiguration{
						Target: registryv1alpha1.Target{
							TargetRef: corev1.ObjectReference{
								Name: "target-name",
							},
						},
						MinReplicas: 1,
						MaxReplicas: 2,
					},
				},
			},
			expected: []string{"  - spec.PolicyRef: Invalid value: \"not-exist-policy\": autoscaling policy resource not-exist-policy does not exist"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errorMsg := validator.validateAutoscalingBinding(tt.input)
			if len(tt.expected) == 0 {
				assert.True(t, valid)
				return
			}
			// Should not be valid due to multiple errors
			assert.False(t, valid)
			assert.NotEmpty(t, errorMsg)

			// Check that the error message is properly formatted
			assert.True(t, strings.HasPrefix(errorMsg, "validation failed:\n"))
			errorMsg = strings.TrimPrefix(errorMsg, "validation failed:\n")

			lines := strings.Split(errorMsg, "\n")

			assert.Equal(t, tt.expected, lines)
		})
	}
}
