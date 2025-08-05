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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AutoscalingPolicyBindingSpec defines the desired state of AutoscalingPolicyBinding.
// +kubebuilder:validation:XValidation:rule="has(self.optimizerConfiguration) != has(self.scalingConfiguration)",message="Either optimizerConfiguration or scalingConfiguration must be set, but not both."
type AutoscalingPolicyBindingSpec struct {
	// PolicyRef references the autoscaling policy to be optimized scaling base on multiple targets.
	// +optional
	PolicyRef *corev1.LocalObjectReference `json:"policyRef,omitempty"`

	OptimizerConfiguration *OptimizerConfiguration `json:"optimizerConfiguration,omitempty"`

	ScalingConfiguration *ScalingConfiguration `json:"scalingConfiguration,omitempty"`
}

type AutoscalingTargetType string

const (
	ModelInferenceTargetType AutoscalingTargetType = "ModelInfer"
)

type MetricFrom struct {
	// The metric uri, e.g. /metrics
	// +optional
	// +kubebuilder:default="/metrics"
	Uri string `json:"uri,omitempty"`
	// +optional
	// +kubebuilder:default=8100
	Port int32 `json:"port,omitempty"`
}

type ScalingConfiguration struct {
	Target Target `json:"target,omitempty"`
	// MinReplicas is the minimum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000000
	MinReplicas int32 `json:"minReplicas"`
	// MaxReplicas is the maximum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000000
	MaxReplicas int32 `json:"maxReplicas"`
}

type OptimizerConfiguration struct {
	Params []OptimizerParam `json:"params,omitempty"`
	// CostExpansionRatePercent is the percentage rate at which the cost expands.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	CostExpansionRatePercent int32 `json:"costExpansionRatePercent,omitempty"`
}

type Target struct {
	Name      string                       `json:"name"`
	Kind      AutoscalingTargetType        `json:"kind"`
	TargetRef *corev1.LocalObjectReference `json:"targetRef,omitempty"`
	// +optional
	AdditionalMatchLabels map[string]string `json:"additionalMatchLabels,omitempty"`
	MetricFrom            MetricFrom        `json:"metricFrom,omitempty"`
}

type OptimizerParam struct {
	Target Target `json:"target,omitempty"`
	// Cost is the cost associated with running this backend.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Cost int32 `json:"cost,omitempty"`
	// MinReplicas is the minimum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000000
	MinReplicas int32 `json:"minReplicas"`
	// MaxReplicas is the maximum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000000
	MaxReplicas int32 `json:"maxReplicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient

// AutoscalingPolicyBinding is the Schema for the models API.
type AutoscalingPolicyBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoscalingPolicyBindingSpec   `json:"spec,omitempty"`
	Status AutoscalingPolicyBindingStatus `json:"status,omitempty"`
}

// AutoscalingPolicyBindingStatus defines the status of a autoscaling policy binding.
type AutoscalingPolicyBindingStatus struct {
}

// +kubebuilder:object:root=true

// AutoscalingPolicyBindingList contains a list of AutoscalingPolicyBinding.
type AutoscalingPolicyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AutoscalingPolicyBinding `json:"items"`
}
