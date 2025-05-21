/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AutoscalingPolicySpec defines the desired state of AutoscalingPolicy.
type AutoscalingPolicySpec struct {
	// TolerancePercent is the percentage of deviation tolerated before scaling actions are triggered.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	TolerancePercent int32 `json:"tolerancePercent"`
	// Metrics is the list of metrics used to evaluate scaling decisions.
	// +kubebuilder:validation:MinItems=1
	Metrics []AutoscalingPolicyMetric `json:"metrics"`
	// Behavior defines the scaling behavior for both scale up and scale down.
	Behavior AutoscalingPolicyBehavior `json:"behavior"`
}

// AutoscalingPolicyMetric defines a metric and its target value for scaling.
type AutoscalingPolicyMetric struct {
	// MetricName is the name of the metric to monitor.
	MetricName string `json:"metricName"`
	// TargetValue is the target value for the metric to trigger scaling.
	TargetValue int32 `json:"targetValue"`
}

// AutoscalingPolicyBehavior defines the scaling behaviors for up and down actions.
type AutoscalingPolicyBehavior struct {
	// ScaleUp defines the policy for scaling up (increasing replicas).
	ScaleUp AutoscalingPolicyStablePolicy `json:"stable"`
	// ScaleDown defines the policy for scaling down (decreasing replicas).
	ScaleDown AutoscalingPolicyPanicPolicy `json:"scaleDown"`
}

// AutoscalingPolicyStablePolicy defines the policy for stable scaling up.
type AutoscalingPolicyStablePolicy struct {
	// Instances is the minimum number of instances to scale up.
	// +kubebuilder:validation:Minimum=0
	Instances int32 `json:"instances"`
	// Percent is the percentage of instances to add during scaling up.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Percent int32 `json:"percent"`
	// Period is the duration over which scaling up is evaluated.
	Period metav1.Duration `json:"period"`
	// SelectPolicy determines the selection strategy for scaling up (e.g., max, min).
	SelectPolicy string `json:"selectPolicy"`
	// StabilizationWindow is the time window to stabilize scaling up actions.
	StabilizationWindow metav1.Duration `json:"stabilizationWindow"`
}

// AutoscalingPolicyPanicPolicy defines the policy for panic scaling down.
type AutoscalingPolicyPanicPolicy struct {
	// Percent is the percentage of instances to remove during scaling down.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Percent int32 `json:"percent"`
	// Period is the duration over which scaling down is evaluated.
	Period metav1.Duration `json:"period"`
	// PanicThresholdPercent is the threshold percent to enter panic mode.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	PanicThresholdPercent int32 `json:"panicThresholdPercent"`
	// PanicModeHold is the duration to hold in panic mode before returning to normal.
	PanicModeHold metav1.Duration `json:"panicModeHold"`
}

// AutoscalingPolicyStatus defines the observed state of AutoscalingPolicy.
type AutoscalingPolicyStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
//
// AutoscalingPolicy is the Schema for the autoscalingpolicies API.
type AutoscalingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoscalingPolicySpec   `json:"spec,omitempty"`
	Status AutoscalingPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AutoscalingPolicyList contains a list of AutoscalingPolicy.
type AutoscalingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AutoscalingPolicy `json:"items"`
}
