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
		backend: "vLLM",
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
		pods: map[types.NamespacedName]*PodInfo{
			{
				Namespace: "default",
				Name:      "pod1",
			}: &podinfo,
		},
		modelServer: map[types.NamespacedName]*modelServer{
			{
				Namespace: "default",
				Name:      "model1",
			}: {
				pods: map[types.NamespacedName]*PodInfo{
					{
						Namespace: "default",
						Name:      "pod1",
					}: &podinfo,
				},
			},
		},
	}

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
	modelName := types.NamespacedName{
		Namespace: "default",
		Name:      "model1",
	}
	assert.Equal(t, s.pods[name].GPUCacheUsage, 0.8)
	assert.Equal(t, s.pods[name].RequestWaitingNum, float64(15))
	assert.Equal(t, s.pods[name].RequestRunningNum, float64(10))
	assert.Equal(t, s.pods[name].TPOT, float64(120))
	assert.Equal(t, s.pods[name].TTFT, float64(210))
	assert.Equal(t, s.pods[name].TimePerOutputToken.SampleSum, &sum2)
	assert.Equal(t, s.pods[name].TimePerOutputToken.SampleCount, &count2)
	assert.Equal(t, s.pods[name].TimeToFirstToken.SampleSum, &sum2)
	assert.Equal(t, s.pods[name].TimeToFirstToken.SampleCount, &count2)
	assert.Equal(t, s.modelServer[modelName].pods[name], s.pods[name])
}

func TestStoreAddOrUpdatePod(t *testing.T) {
	sum := float64(1)
	count := uint64(1)
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
					SampleSum:   &sum,
					SampleCount: &count,
				},
				utils.TTFT: {
					SampleSum:   &sum,
					SampleCount: &count,
				},
			}
	})
	defer patch.Reset()

	type args struct {
		pod          *corev1.Pod
		modelServers []*aiv1alpha1.ModelServer
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "pod1",
					},
				},
				modelServers: []*aiv1alpha1.ModelServer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "model1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      "model2",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &store{
				modelServer: make(map[types.NamespacedName]*modelServer),
				pods:        make(map[types.NamespacedName]*PodInfo),
			}
			err := s.AddOrUpdatePod(tt.args.pod, tt.args.modelServers)
			assert.NoError(t, err)
			podName := utils.GetNamespaceName(tt.args.pod)
			for _, ms := range tt.args.modelServers {
				msName := utils.GetNamespaceName(ms)
				assert.Equal(t, s.pods[podName].modelServer.Contains(msName), true)
				assert.Equal(t, s.pods[podName], s.modelServer[msName].pods[podName])
			}
		})
	}
}
