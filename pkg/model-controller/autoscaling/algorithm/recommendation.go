package algorithm

import "math"

type MetricsMap = map[string]float64

type GetRecommendedInstancesArgs struct {
	minInstances          int32
	maxInstances          int32
	currentInstancesCount int32
	tolerance             float64
	metricTargets         MetricsMap
	unreadyInstancesCount int32
	readyInstancesMetrics []MetricsMap
	externalMetrics       MetricsMap
}

func GetRecommendedInstances(args GetRecommendedInstancesArgs) (recommendedInstances int32, skip bool) {
	if args.currentInstancesCount < args.minInstances {
		return args.minInstances, false
	}
	if args.currentInstancesCount > args.maxInstances {
		return args.maxInstances, false
	}
	recommendedInstances = 0
	skip = true
	for name, target := range args.metricTargets {
		externalMetric, ok := args.externalMetrics[name]
		if ok {
			updateRecommendation(&recommendedInstances, &skip,
				getDesiredInstancesForSingleExternalMetric(
					args.currentInstancesCount,
					args.tolerance,
					target,
					externalMetric,
				))
		} else {
			if desired, ok := getDesiredInstancesForSingleInstanceMetric(
				args.currentInstancesCount,
				args.tolerance,
				name,
				target,
				args.unreadyInstancesCount,
				args.readyInstancesMetrics,
			); ok {
				updateRecommendation(&recommendedInstances, &skip, desired)
			}
		}
	}
	if !skip {
		recommendedInstances = min(max(recommendedInstances, args.minInstances), args.maxInstances)
	}
	return recommendedInstances, skip
}

func updateRecommendation(recommendedInstances *int32, skip *bool, desired int32) {
	if *skip {
		*recommendedInstances = desired
		*skip = false
	} else {
		*recommendedInstances = max(*recommendedInstances, desired)
	}
}

func getDesiredInstancesForSingleExternalMetric(
	currentCount int32,
	tolerance float64,
	target float64,
	metric float64,
) int32 {
	desired := metric / target
	ratio := desired / float64(currentCount)
	if math.Abs(ratio-1.0) <= tolerance {
		return currentCount
	}
	return getCeilDesiredInstances(desired)
}

func getDesiredInstancesForSingleInstanceMetric(
	currentCount int32,
	tolerance float64,
	name string,
	target float64,
	unreadyCount int32,
	readyMetrics []MetricsMap,
) (desired int32, ok bool) {
	currentMetricSum := 0.0
	missingCount := int32(0)
	metricsCount := int32(0)
	for _, readyInstance := range readyMetrics {
		metric, ok := readyInstance[name]
		if ok {
			metricsCount++
			currentMetricSum += metric
		} else {
			missingCount++
		}
	}
	if metricsCount == 0 {
		return 0, false
	}
	ratio := currentMetricSum / float64(metricsCount) / target
	shouldAddUnready := unreadyCount > 0 && getDirection(ratio) > 0
	if !shouldAddUnready && missingCount == 0 {
		if math.Abs(ratio-1.0) <= tolerance {
			return currentCount, true
		}
		return getCeilDesiredInstances(ratio * float64(metricsCount)), true
	}
	metricsCount += missingCount
	if getDirection(ratio) < 0 {
		currentMetricSum += float64(missingCount) * target
	}
	if shouldAddUnready {
		metricsCount += unreadyCount
	}
	newRatio := currentMetricSum / float64(metricsCount) / target
	if math.Abs(newRatio-1.0) <= tolerance || getDirection(ratio) != getDirection(newRatio) {
		return currentCount, true
	}
	desired = getCeilDesiredInstances(newRatio * float64(metricsCount))
	if (getDirection(newRatio) < 0 && desired > currentCount) ||
		(getDirection(newRatio) > 0 && desired < currentCount) {
		return currentCount, true
	}
	return desired, true
}

func getDirection(ratio float64) int32 {
	if ratio >= 1.0 {
		return 1
	} else {
		return -1
	}
}

func getCeilDesiredInstances(value float64) int32 {
	if math.IsNaN(value) {
		return 0
	}
	value = math.Ceil(value)
	const bound = int32(1000000000)
	if value < float64(bound) {
		return max(0, int32(value))
	}
	return bound
}
