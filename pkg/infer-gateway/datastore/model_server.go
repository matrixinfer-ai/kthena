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

package datastore

import (
	"sync"

	"istio.io/istio/pkg/util/sets"
	"k8s.io/apimachinery/pkg/types"
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

type modelServer struct {
	mutex sync.RWMutex

	modelServer *aiv1alpha1.ModelServer
	pods        sets.Set[types.NamespacedName]

	// PDGroup categorization for efficient PD scheduling
	// Key: PD group value (the actual value of the group key label)
	// Value: PDGroupPods containing categorized decode/prefill pods
	pdGroups map[string]*PDGroupPods
}

func newModelServer(ms *aiv1alpha1.ModelServer) *modelServer {
	return &modelServer{
		modelServer: ms,
		pods:        sets.New[types.NamespacedName](),
		pdGroups:    make(map[string]*PDGroupPods),
	}
}

func (m *modelServer) getPods() []types.NamespacedName {
	podNames := make([]types.NamespacedName, 0, m.pods.Len())
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for podName := range m.pods {
		podNames = append(podNames, podName)
	}
	return podNames
}

func (m *modelServer) addPod(podName types.NamespacedName) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.pods.Insert(podName)
}

func (m *modelServer) deletePod(podName types.NamespacedName) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.pods.Delete(podName)
}

// categorizePodForPDGroup categorizes a pod based on PDGroup labels and adds it to appropriate categories
func (m *modelServer) categorizePodForPDGroup(podName types.NamespacedName, podLabels map[string]string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if this modelServer has PDGroup configuration
	if m.modelServer.Spec.WorkloadSelector == nil || m.modelServer.Spec.WorkloadSelector.PDGroup == nil {
		return
	}

	// Check if podLabels is nil
	if podLabels == nil {
		return
	}

	pdGroup := m.modelServer.Spec.WorkloadSelector.PDGroup

	// Get the PD group value from pod labels
	pdGroupValue, hasPDGroupKey := podLabels[pdGroup.GroupKey]
	if !hasPDGroupKey {
		return // Pod doesn't have the required PD group key
	}

	// Get or create PDGroupPods for this group value
	if _, exists := m.pdGroups[pdGroupValue]; !exists {
		m.pdGroups[pdGroupValue] = NewPDGroupPods()
	}
	pdGroupPods := m.pdGroups[pdGroupValue]

	// Check if pod matches decode labels
	isDecodePod := m.matchesLabels(podLabels, pdGroup.DecodeLabels)
	if isDecodePod {
		pdGroupPods.AddDecodePod(podName)
	}

	// Check if pod matches prefill labels
	isPrefillPod := m.matchesLabels(podLabels, pdGroup.PrefillLabels)
	if isPrefillPod {
		pdGroupPods.AddPrefillPod(podName)
	}
}

// removePodFromPDGroups removes a pod from all PDGroup categorizations
func (m *modelServer) removePodFromPDGroups(podName types.NamespacedName) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Remove pod from all PD groups
	for groupValue, pdGroupPods := range m.pdGroups {
		pdGroupPods.RemovePod(podName)

		// Clean up empty PDGroupPods
		if pdGroupPods.IsEmpty() {
			delete(m.pdGroups, groupValue)
		}
	}
}

// matchesLabels checks if pod labels match the required labels
func (m *modelServer) matchesLabels(podLabels map[string]string, requiredLabels map[string]string) bool {
	if len(requiredLabels) == 0 {
		return false // No required labels means no match
	}

	for key, value := range requiredLabels {
		if podLabels[key] != value {
			return false
		}
	}
	return true
}

// getDecodePods returns decode pods for a specific PD group value
func (m *modelServer) getDecodePods(pdGroupValue string) []types.NamespacedName {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if pdGroupPods, exists := m.pdGroups[pdGroupValue]; exists {
		return pdGroupPods.GetDecodePods()
	}
	return nil
}

// getPrefillPods returns prefill pods for a specific PD group value
func (m *modelServer) getPrefillPods(pdGroupValue string) []types.NamespacedName {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if pdGroupPods, exists := m.pdGroups[pdGroupValue]; exists {
		return pdGroupPods.GetPrefillPods()
	}
	return nil
}

// getAllDecodePods returns all decode pods across all PD groups
func (m *modelServer) getAllDecodePods() []types.NamespacedName {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var result []types.NamespacedName
	for _, pdGroupPods := range m.pdGroups {
		result = append(result, pdGroupPods.GetDecodePods()...)
	}
	return result
}

// getAllPrefillPods returns all prefill pods across all PD groups
func (m *modelServer) getAllPrefillPods() []types.NamespacedName {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var result []types.NamespacedName
	for _, pdGroupPods := range m.pdGroups {
		result = append(result, pdGroupPods.GetPrefillPods()...)
	}
	return result
}

// getPrefillPodsForDecodeGroup returns prefill pods that match the same PD group as a decode pod
func (m *modelServer) getPrefillPodsForDecodeGroup(decodePodName types.NamespacedName, podInfoMap map[types.NamespacedName]*PodInfo) []types.NamespacedName {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Check if this modelServer has PDGroup configuration
	if m.modelServer.Spec.WorkloadSelector == nil || m.modelServer.Spec.WorkloadSelector.PDGroup == nil {
		return nil
	}

	pdGroup := m.modelServer.Spec.WorkloadSelector.PDGroup

	// Get the decode pod's PD group value
	decodePodInfo, exists := podInfoMap[decodePodName]
	if !exists || decodePodInfo.Pod.Labels == nil {
		return nil
	}

	pdGroupValue, hasPDGroupKey := decodePodInfo.Pod.Labels[pdGroup.GroupKey]
	if !hasPDGroupKey {
		return nil
	}

	// Return prefill pods for the same PD group value
	if pdGroupPods, exists := m.pdGroups[pdGroupValue]; exists {
		return pdGroupPods.GetPrefillPodsByGroupValue(pdGroupValue, podInfoMap, pdGroup.GroupKey)
	}

	return nil
}
