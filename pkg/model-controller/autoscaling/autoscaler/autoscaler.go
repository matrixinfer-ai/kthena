package autoscaler

import (
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/autoscaling/datastructure"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/autoscaling/histogram"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/autoscaling/util"
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
	return &Autoscaler{
		PanicModeEndsAt:           0,
		PanicModeHoldMilliseconds: behavior.ScaleUp.PanicPolicy.PanicModeHold.Milliseconds(),
		PastHistograms:            datastructure.NewSnapshotSlidingWindow[map[string]HistogramInfo](util.SecondToTimestamp(util.SloQuantileSlidingWindowSeconds), util.SecondToTimestamp(util.SloQuantileDataKeepSeconds)),
		MaxRecommendation:         datastructure.NewMaximumRecordSlidingWindow[int32](behavior.ScaleDown.StabilizationWindow.Duration.Milliseconds()),
		MinRecommendation:         datastructure.NewMinimumRecordSlidingWindow[int32](behavior.ScaleUp.StablePolicy.StabilizationWindow.Duration.Milliseconds()),
		MaxCorrected:              datastructure.NewMinimumLineChartSlidingWindow[int32](behavior.ScaleDown.Period.Milliseconds()),
		MinCorrectedForStable:     datastructure.NewMinimumLineChartSlidingWindow[int32](behavior.ScaleUp.StablePolicy.Period.Milliseconds()),
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
				cost:     int64(backend.Cost),
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
				cost:     int64(backend.Cost) * int64(currentLen),
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
