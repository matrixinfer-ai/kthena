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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferGroupSpec defines the specification of the InferGroup.
type InferGroupSpec struct {
	// RestartGracePeriodSeconds defines the grace time for the controller to rebuild the infergroup when an error occurs
	// Defaults to 0 (infergroup will be rebuilt immediately after an error)
	// +optional
	// +kubebuilder:default=0
	RestartGracePeriodSeconds *int64 `json:"restartGracePeriodSeconds,omitempty"`

	// NetworkTopology defines the NetworkTopology config, this field works in conjunction with network topology feature and hyperNode CRD.
	// +optional
	NetworkTopology *NetworkTopologySpec `json:"networkTopology,omitempty"`

	// GangSchedule defines the GangSchedule config.
	// +optional
	GangSchedule GangSchedule `json:"gangSchedule,omitempty"`
	// +kubebuilder:validation:MaxItems=4
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.exists_one(y, y.name == x.name))", message="roles name must be unique"
	Roles []Role `json:"roles"`
}

// GangSchedule defines the gang scheduling configuration.
type GangSchedule struct {
	// Enable indicates whether users want to enforce gang-scheduling,
	// default true
	// +kubebuilder:default=true
	Enable *bool `json:"enable,omitempty"`
	// todo: add more gang scheduling configuration fields
}

// Role defines the specific pod instance role that performs the inference task.
type Role struct {
	// The name of a role. Name must be unique within an infergroup
	// +kubebuilder:validation:MaxLength=12
	// +kubebuilder:validation:Pattern=^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?$
	Name string `json:"name"`

	// The number of a certain role.
	// For example, in Disaggregated Prefilling, setting the replica count for both the P and D roles to 1 results in 1P1D deployment configuration.
	// This approach can similarly be applied to configure a xPyD deployment scenario.
	// Default to 1.
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// NetworkTopology defines the NetworkTopology config, this field works in conjunction with network topology feature and hyperNode CRD.
	// +optional
	NetworkTopology *NetworkTopologySpec `json:"networkTopology,omitempty"`

	// EntryTemplate defines the template for the entry pod of a role.
	// Required: Currently, a role must have only one entry-pod.
	EntryTemplate corev1.PodTemplateSpec `json:"entryTemplate"`

	// WorkerReplicas defines the number for the worker pod of a role.
	// Required: Need to set the number of worker-pod replicas.
	WorkerReplicas int32 `json:"workerReplicas"`

	// WorkerTemplate defines the template for the worker pod of a role.
	// +optional
	WorkerTemplate *corev1.PodTemplateSpec `json:"workerTemplate,omitempty"`
}

type NetworkTopologySpec struct {
	// Mode specifies the mode of the network topology constrain.
	// +kubebuilder:default=hard
	// +optional
	Mode NetworkTopologyMode `json:"mode,omitempty"`

	// HighestTierAllowed specifies the highest tier that a job allowed to cross when scheduling.
	// +kubebuilder:default=1
	// +optional
	HighestTierAllowed *int `json:"highestTierAllowed,omitempty"`
}

type NetworkTopologyMode string

const (
	// HardNetworkTopologyMode represents a strict network topology constraint that jobs must adhere to.
	HardNetworkTopologyMode NetworkTopologyMode = "hard"

	// SoftNetworkTopologyMode represents a flexible network topology constraint that
	// allows jobs to cross network boundaries under certain conditions.
	SoftNetworkTopologyMode NetworkTopologyMode = "soft"
)

// InferGroup is the smallest unit to complete the inference task
type InferGroup struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              InferGroupSpec `json:"spec,omitempty"`
}
