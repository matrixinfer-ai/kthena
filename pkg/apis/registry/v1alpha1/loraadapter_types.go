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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LoraAdapterSpec defines the desired state of LoraAdapter.
type LoraAdapterSpec struct {
	// Name is the name of the lora adapter.
	// +kubebuilder:validation:MaxLength=128
	// +optional
	Name string `json:"name,omitempty"`
	// Owner is the owner of the lora adapter.
	// +kubebuilder:validation:MaxLength=128
	// +optional
	Owner string `json:"owner,omitempty"`
	// ModelURI is the URI of the lora adapter.
	// +kubebuilder:validation:MaxLength=256
	ModelURI string `json:"modelURI"`
}

// LoraAdapterStatus defines the observed state of LoraAdapter.
type LoraAdapterStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
//
// LoraAdapter is the Schema for the loraadapters API.
type LoraAdapter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoraAdapterSpec   `json:"spec,omitempty"`
	Status LoraAdapterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoraAdapterList contains a list of LoraAdapter.
type LoraAdapterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoraAdapter `json:"items"`
}
