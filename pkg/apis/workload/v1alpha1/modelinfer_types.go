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
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ModelInferNameLabelKey is the pod label key for the model infer name.
	ModelInferNameLabelKey = "modelinfer.matrixinfer.ai/name"
	// GroupNameLabelKey is the pod label key for the group name.
	GroupNameLabelKey = "modelinfer.matrixinfer.ai/group-name"
	// RoleLabelKey is the pod label key for the role.
	RoleLabelKey = "modelinfer.matrixinfer.ai/role"
	// EntryLabelKey is the entry pod label key.
	EntryLabelKey = "modelinfer.matrixinfer.ai/entry"

	// RevisionLabelKey is the revision label for the model infer.
	RevisionLabelKey = "modelinfer.matrixinfer.ai/revision"

	// Environment injected to the worker pods.
	EntryAddressEnv = "ENTRY_ADDRESS"
	// WorkerIndexEnv is the environment variable for the worker index.
	// The entry pod always has a worker index of 0, while the other worker pods has a unique index from 1 to GroupSize-1.
	WorkerIndexEnv = "WORKER_INDEX"
	// GroupSizeEnv is the environment variable for the group size.
	GroupSizeEnv = "GROUP_SIZE"
)

// ModelInferSpec defines the specification of the ModelInfer resource.
type ModelInferSpec struct {
	// Number of InferGroups. That is the number of instances that run infer tasks
	// Default to 1.
	//
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// SchedulerName defines the name of the scheduler used by ModelInfer
	SchedulerName string `json:"schedulerName"`

	// Template defines the template for InferGroup
	Template InferGroup `json:"template"`

	// RolloutStrategy defines the strategy that will be applied to update replicas
	// +optional
	RolloutStrategy RolloutStrategy `json:"rolloutStrategy,omitempty"`

	// RecoveryPolicy defines the recovery policy for the inferGroup
	// +kubebuilder:default=InferGroupRestart
	// +kubebuilder:validation:Enum={InferGroupRestart,None}
	// +optional
	RecoveryPolicy            RecoveryPolicy             `json:"recoveryPolicy,omitempty"`
	TopologySpreadConstraints []TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

type RecoveryPolicy string

const (
	// InferGroupRestart will recreate all the pods in the InferGroup if
	// 1. Any individual pod in the group is recreated; 2. Any containers/init-containers
	// in a pod is restarted. This is to ensure all pods/containers in the group will be
	// started in the same time.
	InferGroupRestart RecoveryPolicy = "InferGroupRestart"

	// NoneRestartPolicy will follow the same behavior as the default pod or deployment.
	NoneRestartPolicy RecoveryPolicy = "None"
)

// RolloutStrategy defines the strategy that the ModelInfer controller
// will use to perform replica updates.
type RolloutStrategy struct {
	// Type defines the rollout strategy, it can only be “InferGroupRollingUpdate” for now.
	//
	// +kubebuilder:validation:Enum={InferGroupRollingUpdate}
	// +kubebuilder:default=InferGroupRollingUpdate
	Type RolloutStrategyType `json:"type"`

	// RollingUpdateConfiguration defines the parameters to be used when type is RollingUpdateStrategyType.
	// +optional
	RollingUpdateConfiguration *RollingUpdateConfiguration `json:"rollingUpdateConfiguration,omitempty"`
}

type RolloutStrategyType string

const (
	// InferGroupRollingUpdate indicates that InferGroup replicas will be updated one by one.
	InferGroupRollingUpdate RolloutStrategyType = "InferGroupRollingUpdate"
)

// RollingUpdateConfiguration defines the parameters to be used for RollingUpdateStrategyType.
type RollingUpdateConfiguration struct {
	// The maximum number of replicas that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of total replicas at the start of update (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// By default, a fixed value of 1 is used.
	// +kubebuilder:validation:XIntOrString
	// +kubebuilder:default=1
	MaxUnavailable intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// The maximum number of replicas that can be scheduled above the original number of
	// replicas.
	// Value can be an absolute number (ex: 5) or a percentage of total replicas at
	// the start of the update (ex: 10%).
	// Absolute number is calculated from percentage by rounding up.
	// By default, a value of 0 is used.
	// +kubebuilder:validation:XIntOrString
	// +kubebuilder:default=0
	MaxSurge intstr.IntOrString `json:"maxSurge,omitempty"`
}

// TopologySpreadConstraint defines the topology spread constraint.
type TopologySpreadConstraint struct {
	// MaxSkew describes the degree to which inferGroup may be unevenly distributed.
	MaxSkew int32 `json:"maxSkew,omitempty"`

	// TopologyKey is the key of node labels. Nodes that have a label with this key
	// and identical values are considered to be in the same topology.
	TopologyKey string `json:"topologyKey,omitempty"`

	// WhenUnsatisfiable indicates how to deal with an inferGroup if it doesn't satisfy
	// the spread constraint.
	WhenUnsatisfiable string `json:"whenUnsatisfiable,omitempty"`

	// LabelSelector is used to find matching inferGroups.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type ModelInferSetConditionType string

// There is a condition type of a modelInfer
const (
	// ModelInferSetAvailable means the modelInfer is available,
	// at least the minimum available groups are up and running.
	ModelInferSetAvailable ModelInferSetConditionType = "Available"

	// The ModelInfer enters the ModelInferSetProgressing state whenever there are ongoing changes,
	// such as the creation of new groups or the scaling of pods within a group.
	// A group remains in the progressing state until all its pods become ready.
	// As long as at least one group is progressing, the entire ModelInferSet is also considered progressing.
	ModelInferSetProgressing ModelInferSetConditionType = "Progressing"
)

// ModelInferStatus defines the observed state of ModelInfer
type ModelInferStatus struct {
	// Replicas track the total number of InferGroup that have been created (updated or not, ready or not)
	Replicas int32 `json:"replicas,omitempty"`

	// UpdatedReplicas track the number of InferGroup that have been updated (ready or not).
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// AvailableReplicas track the number of InferGroup that are in ready state (updated or not).
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Conditions track the condition of the ModelInfer.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient

// ModelInfer is the Schema for the LLM infer API
type ModelInfer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ModelInferSpec   `json:"spec,omitempty"`
	Status            ModelInferStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModelInferList contains a list of ModelInfer
type ModelInferList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelInfer `json:"items"`
}
