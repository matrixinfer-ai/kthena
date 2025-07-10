package algorithm

import (
	"math"

	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/autoscaling/autoscaler"
)

type GetCorrectedInstancesArgs struct {
	Autoscaler           *autoscaler.Autoscaler
	Behavior             *v1alpha1.AutoscalingPolicyBehavior
	MinInstances         int32
	MaxInstances         int32
	CurrentInstances     int32
	RecommendedInstances int32
}

func GetCorrectedInstances(args GetCorrectedInstancesArgs) int32 {
	var corrected int32
	if args.Autoscaler.IsPanicMode() {
		corrected = getCorrectedInstancesForPanic(args)
	} else {
		corrected = getCorrectedInstancesForStable(args)
	}
	return min(max(corrected, args.MinInstances), args.MaxInstances)
}

func getCorrectedInstancesForPanic(args GetCorrectedInstancesArgs) int32 {
	corrected := args.RecommendedInstances
	if pastSample, ok := args.Autoscaler.MinCorrectedForPanic.GetBest(args.CurrentInstances); ok {
		relativeConstraint := pastSample + pastSample*(*args.Behavior.ScaleUp.PanicPolicy.Percent)/100
		corrected = min(corrected, relativeConstraint)
	}
	corrected = max(corrected, args.CurrentInstances)
	return corrected
}

func getCorrectedInstancesForStable(args GetCorrectedInstancesArgs) int32 {
	var corrected int32
	switch {
	case args.RecommendedInstances < args.CurrentInstances:
		corrected = getCorrectedInstancesForStableScaleDown(args)
	case args.RecommendedInstances > args.CurrentInstances:
		corrected = getCorrectedInstancesForStableScaleUp(args)
	default:
		corrected = args.RecommendedInstances
	}
	return corrected
}

func getCorrectedInstancesForStableScaleDown(args GetCorrectedInstancesArgs) int32 {
	corrected := args.RecommendedInstances
	if betterRecommendation, ok := args.Autoscaler.MaxRecommendation.GetBest(); ok {
		corrected = max(corrected, betterRecommendation)
	}
	if pastSample, ok := args.Autoscaler.MaxCorrected.GetBest(args.CurrentInstances); ok {
		absoluteConstraint := pastSample - *args.Behavior.ScaleDown.Instances
		relativeConstraint := pastSample - pastSample*(*args.Behavior.ScaleDown.Percent)/100
		var constraint int32
		switch args.Behavior.ScaleDown.SelectPolicy {
		case v1alpha1.SelectPolicyOr:
			constraint = min(absoluteConstraint, relativeConstraint)
		case v1alpha1.SelectPolicyAnd:
			constraint = max(absoluteConstraint, relativeConstraint)
		default:
			constraint = math.MinInt32
		}
		corrected = max(corrected, constraint)
	}
	corrected = min(corrected, args.CurrentInstances)
	return corrected
}

func getCorrectedInstancesForStableScaleUp(args GetCorrectedInstancesArgs) int32 {
	corrected := args.RecommendedInstances
	if betterRecommendation, ok := args.Autoscaler.MinRecommendation.GetBest(); ok {
		corrected = min(corrected, betterRecommendation)
	}
	if pastSample, ok := args.Autoscaler.MinCorrectedForStable.GetBest(args.CurrentInstances); ok {
		absoluteConstraint := pastSample + *args.Behavior.ScaleUp.StablePolicy.Instances
		relativeConstraint := pastSample + pastSample*(*args.Behavior.ScaleUp.StablePolicy.Percent)/100
		var constraint int32
		switch args.Behavior.ScaleUp.StablePolicy.SelectPolicy {
		case v1alpha1.SelectPolicyOr:
			constraint = max(absoluteConstraint, relativeConstraint)
		case v1alpha1.SelectPolicyAnd:
			constraint = min(absoluteConstraint, relativeConstraint)
		default:
			constraint = math.MaxInt32
		}
		corrected = min(corrected, constraint)
	}
	corrected = max(corrected, args.CurrentInstances)
	return corrected
}
