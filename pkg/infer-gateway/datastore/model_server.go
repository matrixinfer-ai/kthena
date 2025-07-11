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
}

func newModelServer(ms *aiv1alpha1.ModelServer) *modelServer {
	return &modelServer{
		modelServer: ms,
		pods:        sets.New[types.NamespacedName](),
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
