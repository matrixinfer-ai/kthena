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

package testutil

import (
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// MockStore implements datastore.Store for testing
type MockStore struct {
	callbacks map[string][]datastore.CallbackFunc
}

var _ datastore.Store = &MockStore{}

// NewMockStore creates a new MockStore instance
func NewMockStore() *MockStore {
	return &MockStore{
		callbacks: make(map[string][]datastore.CallbackFunc),
	}
}

func (m *MockStore) Run(stop <-chan struct{}) {
}

// RegisterCallback registers a callback function for a specific event type
func (m *MockStore) RegisterCallback(kind string, callback datastore.CallbackFunc) {
	if _, exists := m.callbacks[kind]; !exists {
		m.callbacks[kind] = make([]datastore.CallbackFunc, 0)
	}
	m.callbacks[kind] = append(m.callbacks[kind], callback)
}

// TriggerCallback triggers all registered callbacks for a specific event type
func (m *MockStore) TriggerCallback(kind string, data datastore.EventData) {
	if callbacks, exists := m.callbacks[kind]; exists {
		for _, callback := range callbacks {
			callback(data)
		}
	}
}

// Implement other required Store interface methods with empty implementations
func (m *MockStore) AddOrUpdateModelServer(name types.NamespacedName, modelServer *aiv1alpha1.ModelServer, pods []*corev1.Pod) error {
	return nil
}

func (m *MockStore) DeleteModelServer(modelServer *aiv1alpha1.ModelServer) error {
	return nil
}

func (m *MockStore) GetModelNameByModelServer(name types.NamespacedName) *string {
	return nil
}

func (m *MockStore) GetPodsByModelServer(name types.NamespacedName) ([]*datastore.PodInfo, error) {
	return nil, nil
}

func (m *MockStore) GetPDGroupByModelServer(name types.NamespacedName) *aiv1alpha1.PDGroup {
	return nil
}

func (m *MockStore) AddOrUpdatePod(pod *corev1.Pod, modelServer []*aiv1alpha1.ModelServer) error {
	return nil
}

func (m *MockStore) DeletePod(podName types.NamespacedName) error {
	return nil
}

func (m *MockStore) MatchModelServer(modelName string, request *http.Request) (types.NamespacedName, bool, error) {
	return types.NamespacedName{}, false, nil
}

func (m *MockStore) AddOrUpdateModelRoute(mr *aiv1alpha1.ModelRoute) error {
	return nil
}

func (m *MockStore) DeleteModelRoute(namespacedName string) error {
	return nil
}

func (m *MockStore) GetModelServer(name types.NamespacedName) *aiv1alpha1.ModelServer {
	return nil
}
