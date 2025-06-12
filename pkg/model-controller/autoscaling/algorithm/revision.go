package algorithm

import (
	"math"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/autoscaling/autoscaler"
)

type GetCorrectedInstancesArgs struct {
	autoscaler           *autoscaler.Autoscaler
	behavior             *v1alpha1.AutoscalingPolicyBehavior
	minInstances         int32
	maxInstances         int32
	currentInstances     int32
	recommendedInstances int32
}

const (
	SelectPolicyOr  = "Or"
	SelectPolicyAnd = "And"
)

func GetCorrectedInstances(args GetCorrectedInstancesArgs) int32 {
	var corrected int32
	if args.autoscaler.IsPanicMode() {
		corrected = getCorrectedInstancesForPanic(args)
	} else {
		corrected = getCorrectedInstancesForStable(args)
	}
	return min(max(corrected, args.minInstances), args.maxInstances)
}

func getCorrectedInstancesForPanic(args GetCorrectedInstancesArgs) int32 {
	corrected := args.recommendedInstances
	if pastSample, ok := args.autoscaler.MinCorrectedForPanic.GetBest(args.currentInstances); ok {
		relativeConstraint := pastSample + int32(pastSample*args.behavior.ScaleUp.PanicPolicy.Percent/100)
		corrected = min(corrected, relativeConstraint)
	}
	corrected = max(corrected, args.currentInstances)
	return corrected
}

func getCorrectedInstancesForStable(args GetCorrectedInstancesArgs) int32 {
	var corrected int32
	switch {
	case args.recommendedInstances < args.currentInstances:
		corrected = getCorrectedInstancesForStableScaleDown(args)
	case args.recommendedInstances > args.currentInstances:
		corrected = getCorrectedInstancesForStableScaleUp(args)
	default:
		corrected = args.recommendedInstances
	}
	return corrected
}

func getCorrectedInstancesForStableScaleDown(args GetCorrectedInstancesArgs) int32 {
	corrected := args.recommendedInstances
	if betterRecommendation, ok := args.autoscaler.MaxRecommendation.GetBest(); ok {
		corrected = max(corrected, betterRecommendation)
	}
	if pastSample, ok := args.autoscaler.MaxCorrected.GetBest(args.currentInstances); ok {
		absoluteConstraint := pastSample - args.behavior.ScaleDown.Instances
		relativeConstraint := pastSample - int32(pastSample*args.behavior.ScaleDown.Percent/100)
		var constraint int32
		switch args.behavior.ScaleDown.SelectPolicy {
		case SelectPolicyOr:
			constraint = min(absoluteConstraint, relativeConstraint)
		case SelectPolicyAnd:
			constraint = max(absoluteConstraint, relativeConstraint)
		default:
			constraint = math.MinInt32
		}
		corrected = max(corrected, constraint)
	}
	corrected = min(corrected, args.currentInstances)
	return corrected
}

func getCorrectedInstancesForStableScaleUp(args GetCorrectedInstancesArgs) int32 {
	corrected := args.recommendedInstances
	if betterRecommendation, ok := args.autoscaler.MinRecommendation.GetBest(); ok {
		corrected = min(corrected, betterRecommendation)
	}
	if pastSample, ok := args.autoscaler.MinCorrectedForStable.GetBest(args.currentInstances); ok {
		absoluteConstraint := pastSample + args.behavior.ScaleUp.StablePolicy.Instances
		relativeConstraint := pastSample + int32(pastSample*args.behavior.ScaleUp.StablePolicy.Percent/100)
		var constraint int32
		switch args.behavior.ScaleUp.StablePolicy.SelectPolicy {
		case SelectPolicyOr:
			constraint = max(absoluteConstraint, relativeConstraint)
		case SelectPolicyAnd:
			constraint = min(absoluteConstraint, relativeConstraint)
		default:
			constraint = math.MaxInt32
		}
		corrected = min(corrected, constraint)
	}
	corrected = max(corrected, args.currentInstances)
	return corrected
}
