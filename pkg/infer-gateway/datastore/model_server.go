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

	"k8s.io/apimachinery/pkg/types"
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

type modelServer struct {
	mutex sync.RWMutex

	modelServer *aiv1alpha1.ModelServer
	pods        map[types.NamespacedName]*PodInfo
}

func (m *modelServer) getPods() []*PodInfo {
	if m == nil {
		return nil
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	podInfos := make([]*PodInfo, 0, len(m.pods))
	for _, podInfo := range m.pods {
		podInfos = append(podInfos, podInfo)
	}
	return podInfos
}

func (m *modelServer) addPod(pod *PodInfo) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.pods == nil {
		m.pods = make(map[types.NamespacedName]*PodInfo)
	}
	m.pods[utils.GetNamespaceName(pod.Pod)] = pod
}

func (m *modelServer) deletePod(podName types.NamespacedName) {
	if m == nil {
		return
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.pods, podName)
}
