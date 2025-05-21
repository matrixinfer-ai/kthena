/*
Copyright 2024.

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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
//
// ModelServer is the Schema for the modelservers API.
type ModelServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelServerSpec   `json:"spec,omitempty"`
	Status ModelServerStatus `json:"status,omitempty"`
}

// ModelServerSpec defines the desired state of ModelServer.
type ModelServerSpec struct {
	// The real model that the modelServers are running.
	// If the `model` in LLM inference request is different from this field, it should be overwritten by this field.
	// Otherwise, the `model` in LLM inference request will not be mutated.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	Model *string `json:"model,omitempty"`
	// The port of the model server.
	// +kube:validation:Required
	Port *Port `json:"port"`
	// The inference framework used to manage the model.
	// +kube:validation:Required
	InferenceFramework InferenceFramework `json:"inferenceEngine"`
	// WorkloadSelector is used to match the model servring instances.
	// Currently they must be pods within the same namespace as modelServer object.
	//
	// +kube:validation:Required
	WorkloadSelector *WorkloadSelector `json:"workloadSelector"`
	// Traffic Policy for accessing the model server instance.
	// +optional
	TrafficPolicy *TrafficPolicy `json:"trafficPolicy,omitempty"`
}

type Port struct {
	// A valid non-negative integer port number.
	// +kubebuilder:validation:XValidation:message="port must be between 1-65535",rule="0 < self && self <= 65535"
	Number int32 `json:"number"`
	// The protocol of the model server.
	// MUST be one of HTTP|HTTPS
	// +kubebuilder:validation:Enum=HTTP;HTTPS
	// +kubebuilder:default=HTTP
	Protocol string `json:"protocol"`
}

// InferenceFramework defines the inference framework used by the modelServer to manage the LLM.
//
// +kubebuilder:validation:Enum=vLLM;sgLang
type InferenceFramework string

const (
	// https://github.com/vllm-project/vllm
	VLLM InferenceFramework = "vLLM"
	// https://github.com/sgl-project/sglang
	SGLang InferenceFramework = "SGLang"
)

type WorkloadSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type TrafficPolicy struct {
	// The request timeout for the inference request.
	// By default, there is no timeout.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// The retry policy for the inference request.
	// +optional
	Retry *Retry `json:"retry,omitempty"`

	// TODO: add LoadBalancer policy
}

type Retry struct {
	// The maximum number of times an individual inference request to a model server should be retried.
	// If the maximum number of retries is exceeded without a successgful response, the request will be considered failed.
	// +optional
	Attempts int32 `json:"attempts"`
	// RetryInterval is the interval between retries.
	// +kubebuilder:default="100ms"
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`
}

// ModelServerStatus defines the observed state of ModelServer.
type ModelServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// ModelServerList contains a list of ModelServer.
type ModelServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelServer `json:"items"`
}
