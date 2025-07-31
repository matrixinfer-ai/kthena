package autoscaler

import (
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/algorithm"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/datastructure"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/util"
)

type Status struct {
	PanicModeEndsAt           int64
	PanicModeHoldMilliseconds int64
	History                   *algorithm.History
}

func NewStatus(behavior *v1alpha1.AutoscalingPolicyBehavior) *Status {
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
	return &Status{
		PanicModeEndsAt:           0,
		PanicModeHoldMilliseconds: panicModeHoldMilliseconds,
		History: &algorithm.History{
			MaxRecommendation:     datastructure.NewMaximumRecordSlidingWindow[int32](scaleDownStabilizationWindowMilliseconds),
			MinRecommendation:     datastructure.NewMinimumRecordSlidingWindow[int32](scaleUpStabilizationWindowMilliseconds),
			MaxCorrected:          datastructure.NewMinimumLineChartSlidingWindow[int32](scaleDownPeriodMilliseconds),
			MinCorrectedForStable: datastructure.NewMinimumLineChartSlidingWindow[int32](scaleUpStablePolicyPeriodMilliseconds),
			MinCorrectedForPanic:  datastructure.NewMinimumLineChartSlidingWindow[int32](behavior.ScaleUp.PanicPolicy.Period.Milliseconds()),
		},
	}
}

func (s *Status) AppendRecommendation(recommendedInstances int32) {
	s.History.MaxRecommendation.Append(recommendedInstances)
	s.History.MinRecommendation.Append(recommendedInstances)
}

func (s *Status) AppendCorrected(correctedInstances int32) {
	s.History.MaxCorrected.Append(correctedInstances)
	s.History.MinCorrectedForStable.Append(correctedInstances)
	s.History.MinCorrectedForPanic.Append(correctedInstances)
}

func (s *Status) RefreshPanicMode() {
	if s.PanicModeHoldMilliseconds == 0 {
		s.PanicModeEndsAt = 0
	} else {
		s.PanicModeEndsAt = util.GetCurrentTimestamp() + s.PanicModeHoldMilliseconds
	}
}

func (s *Status) IsPanicMode() bool {
	return s.PanicModeHoldMilliseconds > 0 && util.GetCurrentTimestamp() <= s.PanicModeEndsAt
}
