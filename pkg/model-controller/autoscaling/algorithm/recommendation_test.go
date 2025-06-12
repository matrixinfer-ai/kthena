package algorithm

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_givenInstancesLessThanMinInstances_thenReturnMinInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(5),
		maxInstances:          int32(10),
		currentInstancesCount: int32(4),
		tolerance:             0.1,
		metricTargets:         MetricsMap{},
		unreadyInstancesCount: int32(4),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.minInstances, recommended)
	assert.False(skip)
}

func Test_givenInstancesGreaterThanMaxInstances_thenReturnMaxInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(5),
		maxInstances:          int32(10),
		currentInstancesCount: int32(11),
		tolerance:             0.1,
		metricTargets:         MetricsMap{},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.maxInstances, recommended)
	assert.False(skip)
}

func Test_givenNoAvailableMetrics_thenSkip(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(5),
		maxInstances:          int32(10),
		currentInstancesCount: int32(7),
		tolerance:             0.1,
		metricTargets: MetricsMap{
			"a": 0.5,
			"b": 8.0,
		},
		unreadyInstancesCount: int32(7),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{},
	}
	_, skip := GetRecommendedInstances(args)

	assert.True(skip)
}

func Test_givenMultipleMetrics_thenReturnMaximumRecommendation(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(1000000),
		currentInstancesCount: int32(3),
		tolerance:             0.0,
		metricTargets: MetricsMap{
			"a": 3.0,
			"b": 5.0,
			"c": 4.0,
		},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{
			"a": 6.0,
			"b": 500.0,
			"c": 20.0,
		}}, 3),
		externalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(300), recommended)
	assert.False(skip)
}

func Test_givenTargetWithZeroValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(1),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 0.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{{"a": 1.0}},
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.maxInstances, recommended)
	assert.False(skip)
}

func Test_givenTargetWithExtremelySmallValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(1),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1e-100},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{{"a": 1e100}},
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.maxInstances, recommended)
	assert.False(skip)
}

func Test_givenReadyInstances_shouldCalculateAverage(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(3),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{
			{"a": 19.1},
			{"a": 3.1},
			{"a": 7.1},
		},
		externalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(30), recommended)
	assert.False(skip)
}

func Test_whenDesiredIsLessThanMinInstances_thenReturnMinInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(5),
		maxInstances:          int32(20),
		currentInstancesCount: int32(10),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.1}}, 10),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.minInstances, recommended)
	assert.False(skip)
}

func Test_whenDesiredIsGreaterThanMaxInstances_thenReturnMaxInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(5),
		maxInstances:          int32(20),
		currentInstancesCount: int32(10),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.0}}, 10),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.maxInstances, recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenWithinLowestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(9),
		tolerance:             0.5,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.51}}, 10),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.currentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenOutOfLowestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(9),
		tolerance:             0.5,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.49}}, 10),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(5), recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenWithinHighestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(9),
		tolerance:             0.5,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 1.49}}, 10),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.currentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenOutOfHighestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(9),
		tolerance:             0.5,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 1.51}}, 10),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(16), recommended)
	assert.False(skip)
}

func Test_givenUnreadyInstances_whenShouldScaleDown_thenIgnoreUnreadyInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(58),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(50),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.15}}, 8),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(2), recommended)
	assert.False(skip)
}

func Test_givenUnreadyInstances_whenShouldScaleUp_thenTreatUnreadyInstancesAsZero(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(18),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(10),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.9}}, 8),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(32), recommended)
	assert.False(skip)
}

func Test_givenTooManyUnreadyInstances_whenEstimatedResultIsOpposite_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(58),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(50),
		readyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.9}}, 8),
		externalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.currentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenMissingInstances_whenShouldScaleDown_thenTreatMissingInstancesAsTarget(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(10),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 2),
			slices.Repeat([]MetricsMap{{"a": 0.5}}, 8),
		),
		externalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(6), recommended)
	assert.False(skip)
}

func Test_givenMissingInstances_whenShouldScaleUp_thenTreatMissingInstancesAsZero(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(10),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 2),
			slices.Repeat([]MetricsMap{{"a": 2.9}}, 8),
		),
		externalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(24), recommended)
	assert.False(skip)
}

func Test_givenTooManyMissingInstances_whenEstimatedResultIsOpposite_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(58),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 50),
			slices.Repeat([]MetricsMap{{"a": 2.9}}, 8),
		),
		externalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.currentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenTooManyNonExistingInstances_whenEstimatedResultIsOpposite_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(58),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 4),
			slices.Repeat([]MetricsMap{{"a": 2.9}}, 8),
		),
		externalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.currentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenWithinLowestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(10),
		tolerance:             0.51,
		metricTargets:         MetricsMap{"a": 3.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{"a": 14.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.currentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenOutOfLowestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(10),
		tolerance:             0.49,
		metricTargets:         MetricsMap{"a": 3.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{"a": 14.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(5), recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenWithinHighestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(10),
		tolerance:             0.51,
		metricTargets:         MetricsMap{"a": 3.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{"a": 44.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.currentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenOutOfHighestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(10),
		tolerance:             0.49,
		metricTargets:         MetricsMap{"a": 3.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{"a": 44.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(15), recommended)
	assert.False(skip)
}

func Test_givenExternalTargetWithZeroValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(1),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 0.0},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{"a": 1.0},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.maxInstances, recommended)
	assert.False(skip)
}

func Test_givenExternalTargetWithExtremelySmallValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		minInstances:          int32(1),
		maxInstances:          int32(100),
		currentInstancesCount: int32(1),
		tolerance:             0.0,
		metricTargets:         MetricsMap{"a": 1e-100},
		unreadyInstancesCount: int32(0),
		readyInstancesMetrics: []MetricsMap{},
		externalMetrics:       MetricsMap{"a": 1e100},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.maxInstances, recommended)
	assert.False(skip)
}
