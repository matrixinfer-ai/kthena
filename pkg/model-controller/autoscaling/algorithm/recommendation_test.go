package algorithm

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_givenInstancesLessThanMinInstances_thenReturnMinInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(5),
		MaxInstances:          int32(10),
		CurrentInstancesCount: int32(4),
		Tolerance:             0.1,
		MetricTargets:         MetricsMap{},
		UnreadyInstancesCount: int32(4),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MinInstances, recommended)
	assert.False(skip)
}

func Test_givenInstancesGreaterThanMaxInstances_thenReturnMaxInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(5),
		MaxInstances:          int32(10),
		CurrentInstancesCount: int32(11),
		Tolerance:             0.1,
		MetricTargets:         MetricsMap{},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MaxInstances, recommended)
	assert.False(skip)
}

func Test_givenNoAvailableMetrics_thenSkip(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(5),
		MaxInstances:          int32(10),
		CurrentInstancesCount: int32(7),
		Tolerance:             0.1,
		MetricTargets: MetricsMap{
			"a": 0.5,
			"b": 8.0,
		},
		UnreadyInstancesCount: int32(7),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{},
	}
	_, skip := GetRecommendedInstances(args)

	assert.True(skip)
}

func Test_givenMultipleMetrics_thenReturnMaximumRecommendation(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(1000000),
		CurrentInstancesCount: int32(3),
		Tolerance:             0.0,
		MetricTargets: MetricsMap{
			"a": 3.0,
			"b": 5.0,
			"c": 4.0,
		},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{
			"a": 6.0,
			"b": 500.0,
			"c": 20.0,
		}}, 3),
		ExternalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(300), recommended)
	assert.False(skip)
}

func Test_givenTargetWithZeroValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(1),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 0.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{{"a": 1.0}},
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MaxInstances, recommended)
	assert.False(skip)
}

func Test_givenTargetWithExtremelySmallValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(1),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1e-100},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{{"a": 1e100}},
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MaxInstances, recommended)
	assert.False(skip)
}

func Test_givenReadyInstances_shouldCalculateAverage(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(3),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{
			{"a": 19.1},
			{"a": 3.1},
			{"a": 7.1},
		},
		ExternalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(30), recommended)
	assert.False(skip)
}

func Test_whenDesiredIsLessThanMinInstances_thenReturnMinInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(5),
		MaxInstances:          int32(20),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.1}}, 10),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MinInstances, recommended)
	assert.False(skip)
}

func Test_whenDesiredIsGreaterThanMaxInstances_thenReturnMaxInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(5),
		MaxInstances:          int32(20),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.0}}, 10),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MaxInstances, recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenWithinLowestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(9),
		Tolerance:             0.5,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.51}}, 10),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.CurrentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenOutOfLowestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(9),
		Tolerance:             0.5,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.49}}, 10),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(5), recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenWithinHighestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(9),
		Tolerance:             0.5,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 1.49}}, 10),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.CurrentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenNoUnreadyInstancesAndNoMissingInstances_whenOutOfHighestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(9),
		Tolerance:             0.5,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 1.51}}, 10),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(16), recommended)
	assert.False(skip)
}

func Test_givenUnreadyInstances_whenShouldScaleDown_thenIgnoreUnreadyInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(58),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(50),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.15}}, 8),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(2), recommended)
	assert.False(skip)
}

func Test_givenUnreadyInstances_whenShouldScaleUp_thenTreatUnreadyInstancesAsZero(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(18),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(10),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.9}}, 8),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(32), recommended)
	assert.False(skip)
}

func Test_givenTooManyUnreadyInstances_whenEstimatedResultIsOpposite_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(58),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(50),
		ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.9}}, 8),
		ExternalMetrics:       MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.CurrentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenMissingInstances_whenShouldScaleDown_thenTreatMissingInstancesAsTarget(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 2),
			slices.Repeat([]MetricsMap{{"a": 0.5}}, 8),
		),
		ExternalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(6), recommended)
	assert.False(skip)
}

func Test_givenMissingInstances_whenShouldScaleUp_thenTreatMissingInstancesAsZero(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 2),
			slices.Repeat([]MetricsMap{{"a": 2.9}}, 8),
		),
		ExternalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(24), recommended)
	assert.False(skip)
}

func Test_givenTooManyMissingInstances_whenEstimatedResultIsOpposite_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(58),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 50),
			slices.Repeat([]MetricsMap{{"a": 2.9}}, 8),
		),
		ExternalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.CurrentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenTooManyNonExistingInstances_whenEstimatedResultIsOpposite_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(58),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: slices.Concat(
			slices.Repeat([]MetricsMap{{}}, 4),
			slices.Repeat([]MetricsMap{{"a": 2.9}}, 8),
		),
		ExternalMetrics: MetricsMap{},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.CurrentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenWithinLowestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.51,
		MetricTargets:         MetricsMap{"a": 3.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{"a": 14.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.CurrentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenOutOfLowestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.49,
		MetricTargets:         MetricsMap{"a": 3.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{"a": 14.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(5), recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenWithinHighestTolerance_thenReturnCurrent(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.51,
		MetricTargets:         MetricsMap{"a": 3.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{"a": 44.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.CurrentInstancesCount, recommended)
	assert.False(skip)
}

func Test_givenExternalMetric_whenOutOfHighestTolerance_thenReturnDesired(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(10),
		Tolerance:             0.49,
		MetricTargets:         MetricsMap{"a": 3.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{"a": 44.9},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(int32(15), recommended)
	assert.False(skip)
}

func Test_givenExternalTargetWithZeroValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(1),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 0.0},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{"a": 1.0},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MaxInstances, recommended)
	assert.False(skip)
}

func Test_givenExternalTargetWithExtremelySmallValue_thenReturnMaximumInstances(t *testing.T) {
	assert := assert.New(t)

	args := GetRecommendedInstancesArgs{
		MinInstances:          int32(1),
		MaxInstances:          int32(100),
		CurrentInstancesCount: int32(1),
		Tolerance:             0.0,
		MetricTargets:         MetricsMap{"a": 1e-100},
		UnreadyInstancesCount: int32(0),
		ReadyInstancesMetrics: []MetricsMap{},
		ExternalMetrics:       MetricsMap{"a": 1e100},
	}
	recommended, skip := GetRecommendedInstances(args)

	assert.Equal(args.MaxInstances, recommended)
	assert.False(skip)
}
