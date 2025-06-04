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
	callbacks map[datastore.EventType][]datastore.CallbackFunc
}

// NewMockStore creates a new MockStore instance
func NewMockStore() *MockStore {
	return &MockStore{
		callbacks: make(map[datastore.EventType][]datastore.CallbackFunc),
	}
}

// RegisterCallback registers a callback function for a specific event type
func (m *MockStore) RegisterCallback(eventType datastore.EventType, callback datastore.CallbackFunc) {
	if _, exists := m.callbacks[eventType]; !exists {
		m.callbacks[eventType] = make([]datastore.CallbackFunc, 0)
	}
	m.callbacks[eventType] = append(m.callbacks[eventType], callback)
}

// UnregisterCallback removes a callback function for a specific event type
func (m *MockStore) UnregisterCallback(eventType datastore.EventType, callback datastore.CallbackFunc) {
	// Not needed for tests
}

// TriggerCallback triggers all registered callbacks for a specific event type
func (m *MockStore) TriggerCallback(eventType datastore.EventType, data datastore.EventData) {
	if callbacks, exists := m.callbacks[eventType]; exists {
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

func (m *MockStore) GetPodsByModelServer(name types.NamespacedName) []*datastore.PodInfo {
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

func (m *MockStore) GetModelServerEndpoints(name types.NamespacedName) ([]*datastore.PodInfo, *string, int32, error) {
	return nil, nil, 0, nil
}

func (m *MockStore) AddOrUpdateModelRoute(mr *aiv1alpha1.ModelRoute) error {
	return nil
}

func (m *MockStore) DeleteModelRoute(namespacedName string) error {
	return nil
}
