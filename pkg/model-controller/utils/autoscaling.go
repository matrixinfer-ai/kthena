package utils

import (
	registry "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

func SetAutoscalingPolicyWithDefault(autoscalingConfig *registry.AutoscalingPolicyConfig, autoscalingPolicy *registry.AutoscalingPolicySpec) {
	autoscalingPolicy.TolerancePercent = autoscalingConfig.TolerancePercent
	autoscalingPolicy.Metrics = append(autoscalingPolicy.Metrics, autoscalingConfig.Metrics...)
	autoscalingPolicy.Behavior.ScaleUp.StablePolicy = autoscalingConfig.Behavior
	autoscalingPolicy.Behavior.ScaleDown = autoscalingConfig.Behavior
}
