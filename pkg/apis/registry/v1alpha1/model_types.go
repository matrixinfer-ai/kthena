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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networking "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

// ModelSpec defines the desired state of Model.
type ModelSpec struct {
	// Name is the name of the model.
	// +optional
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name,omitempty"`
	// Owner is the owner of the model.
	// +optional
	Owner string `json:"owner,omitempty"`
	// Backends is the list of model backends associated with this model.
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Backends []ModelBackend `json:"backends"`
	// AutoscalingPolicy is the model-level autoscaling policy. There are two kinds of autoscaling policies:
	// one for model and one for backend. The model-level autoscaling policy is used to control the overall
	// scaling behavior of the model. The backend-level autoscaling policy is used to control the scaling
	// behavior of each individual backend. Webhook will reject the CR if both are specified.
	// +optional
	AutoscalingPolicy *AutoscalingPolicySpec `json:"autoscalingPolicy,omitempty"`
	// CostExpansionRatePercent is the percentage rate at which the cost expands.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	CostExpansionRatePercent *int32 `json:"costExpansionRatePercent,omitempty"`
	// ModelMatch defines the predicate used to match LLM inference requests to a given
	// TargetModels. Multiple match conditions are ANDed together, i.e. the match will
	// evaluate to true only if all conditions are satisfied.
	// +optional
	ModelMatch networking.ModelMatch `json:"modelMatch,omitempty"`
}

// ModelBackend defines the configuration for a model backend.
type ModelBackend struct {
	// Name is the name of the backend.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`
	// Type is the type of the backend.
	Type ModelBackendType `json:"type"`
	// ModelURI is the URI where the model is stored.
	// +kubebuilder:validation:Pattern=`^(hf://|s3://|pvc://).+`
	ModelURI string `json:"modelURI"`
	// CacheURI is the URI where the model cache is stored.
	// +optional
	// +kubebuilder:validation:Pattern=`^(hostpath://|pvc://).+`
	CacheURI string `json:"cacheURI,omitempty"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty" protobuf:"bytes,19,rep,name=envFrom"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
	// MinReplicas is the minimum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000000
	MinReplicas int32 `json:"minReplicas"`
	// MaxReplicas is the maximum number of replicas for the backend.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000000
	MaxReplicas int32 `json:"maxReplicas"`
	// ScalingCost is the cost associated with running this backend.
	// +kubebuilder:validation:Minimum=0
	// +optional
	ScalingCost int32 `json:"scalingCost,omitempty"`
	// RouteWeight is used to specify the percentage of traffic should be sent to the target backend.
	// It's used to create model route.
	// +optional
	// +kubebuilder:default=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	RouteWeight int32 `json:"routeWeight,omitempty"`
	// ScaleToZeroGracePeriod is the duration to wait before scaling to zero.
	// +optional
	ScaleToZeroGracePeriod *metav1.Duration `json:"scaleToZeroGracePeriod,omitempty"`
	// Workers is the list of workers associated with this backend.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1000
	Workers []ModelWorker `json:"workers"`
	// LoraAdapter is a list of LoRA adapters.
	// +optional
	LoraAdapters []LoraAdapter `json:"loraAdapters,omitempty"`
	// AutoscalingPolicy is the backend-level autoscaling policy.
	// +optional
	AutoscalingPolicy *AutoscalingPolicySpec `json:"autoscalingPolicy,omitempty"`
}

// LoraAdapter defines a LoRA (Low-Rank Adaptation) adapter configuration.
type LoraAdapter struct {
	// Name is the name of the LoRA adapter.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`
	// ArtifactURL is the URL where the LoRA adapter artifact is stored.
	// +kubebuilder:validation:Pattern=`^(hf://|s3://|pvc://).+`
	ArtifactURL string `json:"artifactURL"`
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
	// +kubebuilder:validation:Maximum=1000000
	Replicas int32 `json:"replicas,omitempty"`
	// Pods is the number of pods for the worker.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000000
	Pods int32 `json:"pods,omitempty"`
	// Resources specifies the resource requirements for the worker.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Affinity specifies the affinity rules for scheduling the worker pods.
	// +optional
	Affinity corev1.Affinity `json:"affinity,omitempty"`
	// Config contains worker-specific configuration in JSON format.
	// +optional
	Config apiextensionsv1.JSON `json:"config,omitempty"`
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
	// ObservedGeneration track of generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type ModelStatusConditionType string

const (
	ModelStatusConditionTypeInitializing ModelStatusConditionType = "Initializing"
	ModelStatusConditionTypeActive       ModelStatusConditionType = "Active"
	ModelStatusConditionTypeUpdating     ModelStatusConditionType = "Updating"
	ModelStatusConditionTypeFailed       ModelStatusConditionType = "Failed"
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
