/*
Copyright The Volcano Authors.

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
	"github.com/volcano-sh/kthena/client-go/clientset/versioned/fake"
	"github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateAutoscalingBinding(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(&v1alpha1.AutoscalingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummy-policy",
			Namespace: "default",
		},
		Spec: v1alpha1.AutoscalingPolicySpec{},
	})
	validator := NewAutoscalingBindingValidator(fakeClient)

	tests := []struct {
		name     string
		input    *v1alpha1.AutoscalingPolicyBinding
		expected []string
	}{
		{
			name: "heterogeneous and homogeneous config both set to nil",
			input: &v1alpha1.AutoscalingPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy-model",
					Namespace: "default",
				},
				Spec: v1alpha1.AutoscalingPolicyBindingSpec{
					PolicyRef: corev1.LocalObjectReference{
						Name: "dummy-policy",
					},
					Heterogeneous: nil,
					Homogeneous:   nil,
				},
			},
			expected: []string{"  - spec.homogeneous: Required value: spec.homogeneous should be set if spec.heterogeneous does not exist"},
		},
		{
			name: "heterogeneous and homogeneous config both are not nil",
			input: &v1alpha1.AutoscalingPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy-model",
					Namespace: "default",
				},
				Spec: v1alpha1.AutoscalingPolicyBindingSpec{
					PolicyRef: corev1.LocalObjectReference{
						Name: "dummy-policy",
					},
					Heterogeneous: &v1alpha1.Heterogeneous{
						Params: []v1alpha1.HeterogeneousParam{
							{
								Target: v1alpha1.Target{
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
					Homogeneous: &v1alpha1.Homogeneous{
						Target: v1alpha1.Target{
							TargetRef: corev1.ObjectReference{
								Name: "target-name",
							},
						},
						MinReplicas: 1,
						MaxReplicas: 2,
					},
				},
			},
			expected: []string{"  - spec.homogeneous: Forbidden: both spec.heterogeneous and spec.homogeneous can not be set at the same time"},
		},
		{
			name: "different autoscaling policy name",
			input: &v1alpha1.AutoscalingPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy-model",
					Namespace: "default",
				},
				Spec: v1alpha1.AutoscalingPolicyBindingSpec{
					PolicyRef: corev1.LocalObjectReference{
						Name: "not-exist-policy",
					},
					Heterogeneous: nil,
					Homogeneous: &v1alpha1.Homogeneous{
						Target: v1alpha1.Target{
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
