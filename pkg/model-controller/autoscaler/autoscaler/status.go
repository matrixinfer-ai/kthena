package autoscaler

import (
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/datastructure"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/util"
)

type Status struct {
	PanicModeEndsAt           int64
	PanicModeHoldMilliseconds int64
	History                   *History
}

func NewStatus(behavior *v1alpha1.AutoscalingPolicyBehavior) *Status {
	return &Status{
		PanicModeEndsAt:           0,
		PanicModeHoldMilliseconds: behavior.ScaleUp.PanicPolicy.PanicModeHold.Milliseconds(),
		History: &History{
			MaxRecommendation:     datastructure.NewMaximumRecordSlidingWindow[int32](behavior.ScaleDown.StabilizationWindow.Duration.Milliseconds()),
			MinRecommendation:     datastructure.NewMinimumRecordSlidingWindow[int32](behavior.ScaleUp.StablePolicy.StabilizationWindow.Duration.Milliseconds()),
			MaxCorrected:          datastructure.NewMinimumLineChartSlidingWindow[int32](behavior.ScaleDown.Period.Milliseconds()),
			MinCorrectedForStable: datastructure.NewMinimumLineChartSlidingWindow[int32](behavior.ScaleUp.StablePolicy.Period.Milliseconds()),
			MinCorrectedForPanic:  datastructure.NewMinimumLineChartSlidingWindow[int32](behavior.ScaleUp.PanicPolicy.Period.Milliseconds()),
		},
	}
}

type History struct {
	MaxRecommendation     *datastructure.RmqRecordSlidingWindow[int32]
	MinRecommendation     *datastructure.RmqRecordSlidingWindow[int32]
	MaxCorrected          *datastructure.RmqLineChartSlidingWindow[int32]
	MinCorrectedForStable *datastructure.RmqLineChartSlidingWindow[int32]
	MinCorrectedForPanic  *datastructure.RmqLineChartSlidingWindow[int32]
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
