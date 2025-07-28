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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"istio.io/istio/pkg/util/sets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

func Test_updateHistogramMetrics(t *testing.T) {
	sum1 := float64(2)
	count1 := uint64(2)
	sum2 := float64(1)
	count2 := uint64(1)
	type args struct {
		podinfo          *PodInfo
		histogramMetrics map[string]*dto.Histogram
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "update histogram metrics",
			args: args{
				podinfo: &PodInfo{
					TimePerOutputToken: &dto.Histogram{
						SampleSum:   &sum1,
						SampleCount: &count1,
					},
					TimeToFirstToken: &dto.Histogram{
						SampleSum:   &sum1,
						SampleCount: &count1,
					},
				},
				histogramMetrics: map[string]*dto.Histogram{
					utils.TPOT: {
						SampleSum:   &sum2,
						SampleCount: &count2,
					},
					utils.TTFT: {
						SampleSum:   &sum2,
						SampleCount: &count2,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateHistogramMetrics(tt.args.podinfo, tt.args.histogramMetrics)
			assert.Equal(t, tt.args.podinfo.TimePerOutputToken.SampleSum, &sum2)
			assert.Equal(t, tt.args.podinfo.TimePerOutputToken.SampleCount, &count2)
			assert.Equal(t, tt.args.podinfo.TimeToFirstToken.SampleSum, &sum2)
			assert.Equal(t, tt.args.podinfo.TimeToFirstToken.SampleCount, &count2)
		})
	}
}

func TestGetPreviousHistogram(t *testing.T) {
	sum1 := float64(2)
	count1 := uint64(2)

	type args struct {
		podinfo *PodInfo
	}
	tests := []struct {
		name string
		args args
		want map[string]*dto.Histogram
	}{
		{
			name: "get previous histogram",
			args: args{
				podinfo: &PodInfo{
					TimePerOutputToken: &dto.Histogram{
						SampleSum:   &sum1,
						SampleCount: &count1,
					},
					TimeToFirstToken: &dto.Histogram{
						SampleSum:   &sum1,
						SampleCount: &count1,
					},
				},
			},
			want: map[string]*dto.Histogram{
				utils.TPOT: {
					SampleSum:   &sum1,
					SampleCount: &count1,
				},
				utils.TTFT: {
					SampleSum:   &sum1,
					SampleCount: &count1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPreviousHistogram(tt.args.podinfo)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestStoreUpdatePodMetrics(t *testing.T) {
	sum1 := float64(1)
	count1 := uint64(1)
	sum2 := float64(2)
	count2 := uint64(2)
	podinfo := PodInfo{
		engine: "vLLM",
		TimePerOutputToken: &dto.Histogram{
			SampleSum:   &sum1,
			SampleCount: &count1,
		},
		TimeToFirstToken: &dto.Histogram{
			SampleSum:   &sum1,
			SampleCount: &count1,
		},
		GPUCacheUsage:     0.5,
		RequestWaitingNum: 10,
		RequestRunningNum: 5,
		TPOT:              100,
		TTFT:              200,
		modelServer: sets.New[types.NamespacedName](types.NamespacedName{
			Namespace: "default",
			Name:      "model1",
		}),
	}
	s := &store{
		pods:        sync.Map{},
		modelServer: sync.Map{},
	}

	podName := types.NamespacedName{
		Namespace: "default",
		Name:      "pod1",
	}
	modelServerName := types.NamespacedName{
		Namespace: "default",
		Name:      "model1",
	}

	s.pods.Store(podName, &podinfo)
	s.modelServer.Store(modelServerName, &modelServer{
		pods: sets.New[types.NamespacedName](podName),
	})

	patch := gomonkey.NewPatches()
	patch.ApplyFunc(backend.GetPodMetrics, func(backend string, pod *corev1.Pod, previousHistogram map[string]*dto.Histogram) (map[string]float64, map[string]*dto.Histogram) {
		return map[string]float64{
				utils.GPUCacheUsage:     0.8,
				utils.RequestWaitingNum: 15,
				utils.RequestRunningNum: 10,
				utils.TPOT:              120,
				utils.TTFT:              210,
			}, map[string]*dto.Histogram{
				utils.TPOT: {
					SampleSum:   &sum2,
					SampleCount: &count2,
				},
				utils.TTFT: {
					SampleSum:   &sum2,
					SampleCount: &count2,
				},
			}
	})
	defer patch.Reset()

	s.updatePodMetrics(&podinfo)

	name := types.NamespacedName{
		Namespace: "default",
		Name:      "pod1",
	}

	// Get pod info from sync.Map
	if value, ok := s.pods.Load(name); ok {
		podInfo := value.(*PodInfo)
		assert.Equal(t, podInfo.GPUCacheUsage, 0.8)
		assert.Equal(t, podInfo.RequestWaitingNum, float64(15))
		assert.Equal(t, podInfo.RequestRunningNum, float64(10))
		assert.Equal(t, podInfo.TPOT, float64(120))
		assert.Equal(t, podInfo.TTFT, float64(210))
		assert.Equal(t, podInfo.TimePerOutputToken.SampleSum, &sum2)
		assert.Equal(t, podInfo.TimePerOutputToken.SampleCount, &count2)
		assert.Equal(t, podInfo.TimeToFirstToken.SampleSum, &sum2)
		assert.Equal(t, podInfo.TimeToFirstToken.SampleCount, &count2)
	} else {
		t.Errorf("Pod not found in store")
	}
}

func TestStoreAddOrUpdatePod(t *testing.T) {
	s := &store{
		modelServer: sync.Map{},
		pods:        sync.Map{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "pod1",
		},
	}
	ms1 := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "model1",
		},
	}
	ms2 := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "model2",
		},
	}

	// Add model server first
	s.AddOrUpdateModelServer(ms1, nil)
	s.AddOrUpdateModelServer(ms2, nil)

	modelServers := []*aiv1alpha1.ModelServer{ms1, ms2}
	err := s.AddOrUpdatePod(pod, modelServers)
	assert.NoError(t, err)

	podName := utils.GetNamespaceName(pod)
	// Check pod is stored and references model servers
	if value, ok := s.pods.Load(podName); ok {
		podInfo := value.(*PodInfo)
		for _, ms := range modelServers {
			msName := utils.GetNamespaceName(ms)
			assert.True(t, podInfo.modelServer.Contains(msName))
		}
		assert.Equal(t, podInfo.Pod.Name, pod.Name, "pod should be stored correctly")
		assert.Equal(t, podInfo.modelServer.Len(), 2, "pod should reference both model servers")
	} else {
		t.Errorf("Pod not found in store")
	}

	// Update pod with only one model server
	err = s.AddOrUpdatePod(pod, []*aiv1alpha1.ModelServer{ms1})
	assert.NoError(t, err)

	if value, ok := s.pods.Load(podName); ok {
		podInfo := value.(*PodInfo)
		assert.True(t, podInfo.modelServer.Contains(utils.GetNamespaceName(ms1)))
		assert.False(t, podInfo.modelServer.Contains(utils.GetNamespaceName(ms2)))
	}

	// Check model server references
	if value, ok := s.modelServer.Load(utils.GetNamespaceName(ms1)); ok {
		ms1Info := value.(*modelServer)
		assert.Equal(t, ms1Info.pods.Len(), 1, "model server 1 should still reference the pod")
	}
	if value, ok := s.modelServer.Load(utils.GetNamespaceName(ms2)); ok {
		ms2Info := value.(*modelServer)
		assert.Equal(t, ms2Info.pods.Len(), 0, "model server 2 should not reference the pod")
	}
}

func TestStoreDeletePod(t *testing.T) {
	podName := types.NamespacedName{Namespace: "default", Name: "pod1"}
	modelServerName := types.NamespacedName{Namespace: "default", Name: "model1"}

	pod := &corev1.Pod{}
	podInfo := &PodInfo{
		Pod:         pod,
		modelServer: sets.New[types.NamespacedName](modelServerName),
		models:      sets.New[string](),
	}

	ms := newModelServer(&aiv1alpha1.ModelServer{})
	ms.addPod(podName)

	s := &store{
		pods:        sync.Map{},
		modelServer: sync.Map{},
		callbacks:   make(map[string][]CallbackFunc),
	}

	s.pods.Store(podName, podInfo)
	s.modelServer.Store(modelServerName, ms)

	// Normal delete
	err := s.DeletePod(podName)
	assert.NoError(t, err)
	_, exists := s.pods.Load(podName)
	assert.False(t, exists, "pod should be deleted from store")
	assert.False(t, ms.pods.Contains(podName), "pod should be removed from modelServer set")

	// Delete non-existent pod
	err = s.DeletePod(types.NamespacedName{Namespace: "default", Name: "notfound"})
	assert.NoError(t, err)
}

func TestStoreDeletePod_MultiModelServers(t *testing.T) {
	podName := types.NamespacedName{Namespace: "default", Name: "pod1"}
	ms1Name := types.NamespacedName{Namespace: "default", Name: "model1"}
	ms2Name := types.NamespacedName{Namespace: "default", Name: "model2"}

	pod := &corev1.Pod{}
	podInfo := &PodInfo{
		Pod:         pod,
		modelServer: sets.New[types.NamespacedName](ms1Name, ms2Name),
		models:      sets.New[string](),
	}

	ms1 := newModelServer(&aiv1alpha1.ModelServer{})
	ms2 := newModelServer(&aiv1alpha1.ModelServer{})
	ms1.addPod(podName)
	ms2.addPod(podName)

	s := &store{
		pods:        sync.Map{},
		modelServer: sync.Map{},
		callbacks:   make(map[string][]CallbackFunc),
	}

	s.pods.Store(podName, podInfo)
	s.modelServer.Store(ms1Name, ms1)
	s.modelServer.Store(ms2Name, ms2)

	err := s.DeletePod(podName)
	assert.NoError(t, err)
	assert.False(t, ms1.pods.Contains(podName))
	assert.False(t, ms2.pods.Contains(podName))
}

func TestStoreAddOrUpdateModelServer(t *testing.T) {
	s := &store{
		modelServer: sync.Map{},
	}
	ms := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "model1",
		},
	}
	pods := sets.New[types.NamespacedName](types.NamespacedName{Namespace: "default", Name: "pod1"})
	err := s.AddOrUpdateModelServer(ms, pods)
	assert.NoError(t, err)

	msName := utils.GetNamespaceName(ms)
	if value, ok := s.modelServer.Load(msName); ok {
		msInfo := value.(*modelServer)
		assert.NotNil(t, msInfo)
		assert.True(t, msInfo.pods.Contains(types.NamespacedName{Namespace: "default", Name: "pod1"}))
	} else {
		t.Errorf("ModelServer not found in store")
	}

	// Update with new pods
	pods2 := sets.New[types.NamespacedName](types.NamespacedName{Namespace: "default", Name: "pod2"})
	err = s.AddOrUpdateModelServer(ms, pods2)
	assert.NoError(t, err)

	if value, ok := s.modelServer.Load(msName); ok {
		msInfo := value.(*modelServer)
		assert.True(t, msInfo.pods.Contains(types.NamespacedName{Namespace: "default", Name: "pod2"}))
		assert.False(t, msInfo.pods.Contains(types.NamespacedName{Namespace: "default", Name: "pod1"}))
	}
}

func TestStoreDeleteModelServer(t *testing.T) {
	s := &store{
		modelServer: sync.Map{},
		pods:        sync.Map{},
	}
	ms := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "model1",
		},
	}
	msName := utils.GetNamespaceName(ms)
	podName := types.NamespacedName{Namespace: "default", Name: "pod1"}
	modelSrv := newModelServer(ms)
	modelSrv.addPod(podName)
	s.modelServer.Store(msName, modelSrv)
	podInfo := &PodInfo{
		Pod:         &corev1.Pod{},
		modelServer: sets.New[types.NamespacedName](msName),
		models:      sets.New[string](),
	}
	s.pods.Store(podName, podInfo)

	err := s.DeleteModelServer(msName)
	assert.NoError(t, err)
	_, exists := s.modelServer.Load(msName)
	assert.False(t, exists, "modelServer should be deleted")
	assert.False(t, podInfo.modelServer.Contains(msName), "modelServer ref should be removed from podInfo")
	_, podExists := s.pods.Load(podName)
	assert.False(t, podExists, "pod should be deleted if no modelServer left")
}

func TestStoreGetPodsByModelServer(t *testing.T) {
	s := &store{
		modelServer: sync.Map{},
		pods:        sync.Map{},
	}
	ms := &aiv1alpha1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "model1",
		},
	}
	msName := utils.GetNamespaceName(ms)
	podName := types.NamespacedName{Namespace: "default", Name: "pod1"}
	modelSrv := newModelServer(ms)
	modelSrv.addPod(podName)
	s.modelServer.Store(msName, modelSrv)
	podInfo := &PodInfo{
		Pod:         &corev1.Pod{},
		modelServer: sets.New[types.NamespacedName](msName),
		models:      sets.New[string](),
	}
	s.pods.Store(podName, podInfo)

	pods, err := s.GetPodsByModelServer(msName)
	assert.NoError(t, err)
	assert.Len(t, pods, 1)
	assert.Equal(t, podInfo, pods[0])

	_, err = s.GetPodsByModelServer(types.NamespacedName{Namespace: "default", Name: "notfound"})
	assert.Error(t, err)
}
