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

package autoscaler

import (
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/datastructure"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/histogram"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/util"
)

type Autoscaler struct {
	PanicModeEndsAt           int64
	PanicModeHoldMilliseconds int64
	PastHistograms            *datastructure.SnapshotSlidingWindow[map[string]HistogramInfo]
	MaxRecommendation         *datastructure.RmqRecordSlidingWindow[int32]
	MinRecommendation         *datastructure.RmqRecordSlidingWindow[int32]
	MaxCorrected              *datastructure.RmqLineChartSlidingWindow[int32]
	MinCorrectedForStable     *datastructure.RmqLineChartSlidingWindow[int32]
	MinCorrectedForPanic      *datastructure.RmqLineChartSlidingWindow[int32]
	GlobalInfo                *GlobalInfo
	MetricTargets             map[string]float64
}

type HistogramInfo struct {
	PodStartTime *metav1.Time
	HistogramMap map[string]*histogram.Snapshot
}

func NewAutoscaler(behavior *v1alpha1.AutoscalingPolicyBehavior, globalInfo *GlobalInfo, metricTargets map[string]float64) *Autoscaler {
	panicModeHoldMilliseconds := int64(0)
	if behavior.ScaleUp.PanicPolicy.PanicModeHold != nil {
		panicModeHoldMilliseconds = behavior.ScaleUp.PanicPolicy.PanicModeHold.Milliseconds()
	}
	scaleDownStabilizationWindowMilliseconds := int64(0)
	if behavior.ScaleDown.StabilizationWindow != nil {
		scaleDownStabilizationWindowMilliseconds = behavior.ScaleDown.StabilizationWindow.Milliseconds()
	}
	scaleUpStabilizationWindowMilliseconds := int64(0)
	if behavior.ScaleUp.StablePolicy.StabilizationWindow != nil {
		scaleUpStabilizationWindowMilliseconds = behavior.ScaleUp.StablePolicy.StabilizationWindow.Milliseconds()
	}
	scaleUpStablePolicyPeriodMilliseconds := int64(0)
	if behavior.ScaleUp.StablePolicy.Period != nil {
		scaleUpStablePolicyPeriodMilliseconds = behavior.ScaleUp.StablePolicy.Period.Milliseconds()
	}
	scaleDownPeriodMilliseconds := int64(0)
	if behavior.ScaleDown.Period != nil {
		scaleDownPeriodMilliseconds = behavior.ScaleDown.Period.Milliseconds()
	}

	return &Autoscaler{
		PanicModeEndsAt:           0,
		PanicModeHoldMilliseconds: panicModeHoldMilliseconds,
		PastHistograms:            datastructure.NewSnapshotSlidingWindow[map[string]HistogramInfo](util.SecondToTimestamp(util.SloQuantileSlidingWindowSeconds), util.SecondToTimestamp(util.SloQuantileDataKeepSeconds)),
		MaxRecommendation:         datastructure.NewMaximumRecordSlidingWindow[int32](scaleDownStabilizationWindowMilliseconds),
		MinRecommendation:         datastructure.NewMinimumRecordSlidingWindow[int32](scaleUpStabilizationWindowMilliseconds),
		MaxCorrected:              datastructure.NewMinimumLineChartSlidingWindow[int32](scaleDownPeriodMilliseconds),
		MinCorrectedForStable:     datastructure.NewMinimumLineChartSlidingWindow[int32](scaleUpStablePolicyPeriodMilliseconds),
		MinCorrectedForPanic:      datastructure.NewMinimumLineChartSlidingWindow[int32](behavior.ScaleUp.PanicPolicy.Period.Milliseconds()),
		GlobalInfo:                globalInfo,
		MetricTargets:             metricTargets,
	}
}

func (autoscaler *Autoscaler) AppendRecommendation(recommendedInstances int32) {
	autoscaler.MaxRecommendation.Append(recommendedInstances)
	autoscaler.MinRecommendation.Append(recommendedInstances)
}

func (autoscaler *Autoscaler) AppendCorrected(correctedInstances int32) {
	autoscaler.MaxCorrected.Append(correctedInstances)
	autoscaler.MinCorrectedForStable.Append(correctedInstances)
	autoscaler.MinCorrectedForPanic.Append(correctedInstances)
}

func (autoscaler *Autoscaler) RefreshPanicMode() {
	if autoscaler.PanicModeHoldMilliseconds == 0 {
		autoscaler.PanicModeEndsAt = 0
	} else {
		autoscaler.PanicModeEndsAt = util.GetCurrentTimestamp() + autoscaler.PanicModeHoldMilliseconds
	}
}

func (autoscaler *Autoscaler) IsPanicMode() bool {
	return autoscaler.PanicModeHoldMilliseconds > 0 && util.GetCurrentTimestamp() <= autoscaler.PanicModeEndsAt
}

type GlobalInfo struct {
	backendsInfo []*BackendInfo
	MinReplicas  int32
	MaxReplicas  int32
	scalingOrder []*ReplicaBlock
}

type BackendInfo struct {
	name        string
	minReplicas int32
	maxReplicas int32
}

type ReplicaBlock struct {
	name     string
	index    int32
	replicas int32
	cost     int64
}

func NewGlobalInfo(backends []v1alpha1.ModelBackend, costExpansionRatePercent int32) *GlobalInfo {
	backendsInfo := make([]*BackendInfo, 0, len(backends))
	minReplicas := int32(0)
	maxReplicas := int32(0)
	scalingOrder := []*ReplicaBlock{}
	for index, backend := range backends {
		backendsInfo = append(backendsInfo, &BackendInfo{
			name:        backend.Name,
			minReplicas: backend.MinReplicas,
			maxReplicas: backend.MaxReplicas,
		})
		minReplicas += backend.MinReplicas
		maxReplicas += backend.MaxReplicas
		replicas := backend.MaxReplicas - backend.MinReplicas
		if replicas <= 0 {
			continue
		}
		if costExpansionRatePercent == 100 {
			scalingOrder = append(scalingOrder, &ReplicaBlock{
				index:    int32(index),
				name:     backend.Name,
				replicas: replicas,
				cost:     int64(backend.ScalingCost),
			})
			continue
		}
		packageLen := 1.0
		for replicas > 0 {
			currentLen := min(replicas, max(int32(packageLen), 1))
			scalingOrder = append(scalingOrder, &ReplicaBlock{
				name:     backend.Name,
				index:    int32(index),
				replicas: currentLen,
				cost:     int64(backend.ScalingCost) * int64(currentLen),
			})
			replicas -= currentLen
			packageLen = packageLen * float64(costExpansionRatePercent) / 100
		}
	}
	sort.Slice(scalingOrder, func(i, j int) bool {
		if scalingOrder[i].cost != scalingOrder[j].cost {
			return scalingOrder[i].cost < scalingOrder[j].cost
		}
		return scalingOrder[i].index < scalingOrder[j].index
	})
	return &GlobalInfo{
		backendsInfo: backendsInfo,
		MinReplicas:  minReplicas,
		MaxReplicas:  maxReplicas,
		scalingOrder: scalingOrder,
	}
}

func (globalInfo *GlobalInfo) RestoreReplicasOfEachBackend(replicas int32) map[string]int32 {
	replicasMap := make(map[string]int32, len(globalInfo.backendsInfo))
	for _, backend := range globalInfo.backendsInfo {
		replicasMap[backend.name] = backend.minReplicas
	}
	replicas = min(max(replicas, globalInfo.MinReplicas), globalInfo.MaxReplicas)
	replicas -= globalInfo.MinReplicas
	for _, block := range globalInfo.scalingOrder {
		slot := min(replicas, block.replicas)
		replicasMap[block.name] += slot
		replicas -= slot
		if replicas <= 0 {
			break
		}
	}
	return replicasMap
}
