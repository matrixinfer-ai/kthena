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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the desired state of Model.
type ModelSpec struct {
	// Name is the name of the model.
	// +optional
	Name string `json:"name,omitempty"`
	// Owner is the owner of the model.
	// +optional
	Owner string `json:"owner,omitempty"`
	// Backends is the list of model backends associated with this model.
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Backends []ModelBackend `json:"backends"`
	// AutoscalingPolicyRef references the autoscaling policy to be used for this model.
	// +optional
	AutoscalingPolicyRef corev1.LocalObjectReference `json:"autoscalingPolicyRef,omitempty"`
	// CostExpansionRatePercent is the percentage rate at which the cost expands.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	CostExpansionRatePercent int32 `json:"costExpansionRatePercent"`
}

// ModelBackend defines the configuration for a model backend.
type ModelBackend struct {
	// Name is the name of the backend.
	Name string `json:"name"`
	// Type is the type of the backend.
	Type ModelBackendType `json:"type"`
	// Config contains backend-specific configuration in JSON format.
	// +optional
	Config apiextensionsv1.JSON `json:"config,omitempty"`
	// ModelURI is the URI where the model is stored.
	ModelURI string `json:"modelURI"`
	// CacheURI is the URI where the model cache is stored.
	// +optional
	CacheURI string `json:"cacheURI,omitempty"`
	// Env variables to be added to the server process.
	Env map[string]string `json:"env,omitempty"`
	// Env variables to be added to the server process from Secret or ConfigMap.
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// MinReplicas is the minimum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=0
	MinReplicas int32 `json:"minReplicas"`
	// MaxReplicas is the maximum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`
	// Cost is the cost associated with running this backend.
	// +kubebuilder:validation:Minimum=0
	Cost int64 `json:"cost"`
	// ScaleToZeroGracePeriod is the duration to wait before scaling to zero.
	ScaleToZeroGracePeriod metav1.Duration `json:"scaleToZeroGracePeriod"`
	// Workers is the list of workers associated with this backend.
	// +kubebuilder:validation:MinItems=1
	Workers []ModelWorker `json:"workers"`
	// LoraAdapterRefs is a list of references to LoRA adapters.
	// +optional
	LoraAdapterRefs []corev1.LocalObjectReference `json:"loraAdapterRefs,omitempty"`
	// AutoscalingPolicyRef references the autoscaling policy for this backend.
	// +optional
	AutoscalingPolicyRef corev1.LocalObjectReference `json:"autoscalingPolicyRef,omitempty"`
}

// ModelBackendType defines the type of model backend.
// +kubebuilder:validation:Enum=vLLM;vLLMDisaggregated;SGLang;MindIE;MindIEDisaggregated
type ModelBackendType string

const (
	// ModelBackendTypeVLLM represents a vLLM backend.
	ModelBackendTypeVLLM ModelBackendType = "vLLM"
	// ModelBackendTypeVLLMDisaggregated represents a disaggregated vLLM backend.
	ModelBackendTypeVLLMDisaggregated ModelBackendType = "vLLMDisaggregated"
	// ModelBackendTypeSGLang represents an SGLang backend.
	ModelBackendTypeSGLang ModelBackendType = "SGLang"
	// ModelBackendTypeMindIE represents a MindIE backend.
	ModelBackendTypeMindIE ModelBackendType = "MindIE"
	// ModelBackendTypeMindIEDisaggregated represents a disaggregated MindIE backend.
	ModelBackendTypeMindIEDisaggregated ModelBackendType = "MindIEDisaggregated"
)

// ModelWorker defines the model worker configuration.
type ModelWorker struct {
	// Type is the type of the model worker.
	// +kubebuilder:default=server
	Type ModelWorkerType `json:"type,omitempty"`
	// Image is the container image for the worker.
	Image string `json:"image,omitempty"`
	// Replicas is the number of replicas for the worker.
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`
	// Pods is the number of pods for the worker.
	// +kubebuilder:validation:Minimum=0
	Pods int32 `json:"pods,omitempty"`
	// Resources specifies the resource requirements for the worker.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Affinity specifies the affinity rules for scheduling the worker pods.
	// +optional
	Affinity corev1.Affinity `json:"affinity,omitempty"`
}

// ModelWorkerType defines the type of model worker.
// +kubebuilder:validation:Enum=server;prefill;decode;controller;coordinator
type ModelWorkerType string

const (
	// ModelWorkerTypeServer represents a server worker.
	ModelWorkerTypeServer ModelWorkerType = "server"
	// ModelWorkerTypePrefill represents a prefill worker.
	ModelWorkerTypePrefill ModelWorkerType = "prefill"
	// ModelWorkerTypeDecode represents a decode worker.
	ModelWorkerTypeDecode ModelWorkerType = "decode"
	// ModelWorkerTypeController represents a controller worker.
	ModelWorkerTypeController ModelWorkerType = "controller"
	// ModelWorkerTypeCoordinator represents a coordinator worker.
	ModelWorkerTypeCoordinator ModelWorkerType = "coordinator"
)

// ModelStatus defines the observed state of Model.
type ModelStatus struct {
	// Conditions represents the latest available observations of the model's state.
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// BackendStatuses contains the status of each backend.
	// +listType=atomic
	BackendStatuses []ModelBackendStatus `json:"backendStatuses,omitempty"`
}

type ModelStatusConditionType string

const (
	ModelStatusConditionTypeInitialized ModelStatusConditionType = "Initialized"
	ModelStatusConditionTypeReady       ModelStatusConditionType = "Ready"
)

// ModelBackendStatus defines the status of a model backend.
type ModelBackendStatus struct {
	// Name is the name of the backend.
	Name string `json:"name"`
	// Hash is a hash representing the backend configuration.
	Hash string `json:"hash"`
	// Replicas is the number of replicas currently running for the backend.
	Replicas int32 `json:"replicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient

// Model is the Schema for the models API.
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelSpec   `json:"spec,omitempty"`
	Status ModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model.
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}
