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

package algorithm

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRecommendedInstances(t *testing.T) {
	type TestCase struct {
		name                string
		args                GetRecommendedInstancesArgs
		expectedRecommended int32
		expectedSkip        bool
	}

	testcases := []TestCase{
		{
			name: "givenInstancesLessThanMinInstances_thenReturnMinInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(5),
				MaxInstances:          int32(10),
				CurrentInstancesCount: int32(4),
				Tolerance:             0.1,
				MetricTargets:         MetricsMap{},
				UnreadyInstancesCount: int32(4),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(5),
			expectedSkip:        false,
		},
		{
			name: "givenInstancesGreaterThanMaxInstances_thenReturnMaxInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(5),
				MaxInstances:          int32(10),
				CurrentInstancesCount: int32(11),
				Tolerance:             0.1,
				MetricTargets:         MetricsMap{},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(10),
			expectedSkip:        false,
		},
		{
			name: "givenNoAvailableMetrics_thenSkip",
			args: GetRecommendedInstancesArgs{
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
			},
			expectedSkip: true,
		},
		{
			name: "givenMultipleMetrics_thenReturnMaximumRecommendation",
			args: GetRecommendedInstancesArgs{
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
			},
			expectedRecommended: int32(300),
			expectedSkip:        false,
		},
		{
			name: "givenTargetWithZeroValue_thenReturnMaximumInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(1),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 0.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{{"a": 1.0}},
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(100),
			expectedSkip:        false,
		},
		{
			name: "givenTargetWithExtremelySmallValue_thenReturnMaximumInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(1),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 1e-100},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{{"a": 1e100}},
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(100),
			expectedSkip:        false,
		},
		{
			name: "givenReadyInstances_shouldCalculateAverage",
			args: GetRecommendedInstancesArgs{
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
			},
			expectedRecommended: int32(30),
			expectedSkip:        false,
		},
		{
			name: "whenDesiredIsLessThanMinInstances_thenReturnMinInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(5),
				MaxInstances:          int32(20),
				CurrentInstancesCount: int32(10),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.1}}, 10),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(5),
			expectedSkip:        false,
		},
		{
			name: "whenDesiredIsGreaterThanMaxInstances_thenReturnMaxInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(5),
				MaxInstances:          int32(20),
				CurrentInstancesCount: int32(10),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.0}}, 10),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(20),
			expectedSkip:        false,
		},
		{
			name: "givenNoUnreadyInstancesAndNoMissingInstances_whenWithinLowestTolerance_thenReturnCurrent",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(9),
				Tolerance:             0.5,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.51}}, 10),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(9),
			expectedSkip:        false,
		},
		{
			name: "givenNoUnreadyInstancesAndNoMissingInstances_whenOutOfLowestTolerance_thenReturnDesired",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(9),
				Tolerance:             0.5,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.49}}, 10),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(5),
			expectedSkip:        false,
		},
		{
			name: "givenNoUnreadyInstancesAndNoMissingInstances_whenWithinHighestTolerance_thenReturnCurrent",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(9),
				Tolerance:             0.5,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 1.49}}, 10),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(9),
			expectedSkip:        false,
		},
		{
			name: "givenNoUnreadyInstancesAndNoMissingInstances_whenOutOfHighestTolerance_thenReturnDesired",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(9),
				Tolerance:             0.5,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 1.51}}, 10),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(16),
			expectedSkip:        false,
		},
		{
			name: "givenUnreadyInstances_whenShouldScaleDown_thenIgnoreUnreadyInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(58),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(50),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 0.15}}, 8),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(2),
			expectedSkip:        false,
		},
		{
			name: "givenUnreadyInstances_whenShouldScaleUp_thenThreatUnreadyInstancesAsZero",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(18),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(10),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.9}}, 8),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(32),
			expectedSkip:        false,
		},
		{
			name: "givenTooManyUnreadyInstances_whenEstimatedResultIsOpposite_thenReturnCurrent",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(58),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 1.0},
				UnreadyInstancesCount: int32(50),
				ReadyInstancesMetrics: slices.Repeat([]MetricsMap{{"a": 3.9}}, 8),
				ExternalMetrics:       MetricsMap{},
			},
			expectedRecommended: int32(58),
			expectedSkip:        false,
		},
		{
			name: "givenMissingInstances_whenShouldScaleDown_thenTreatMissingInstancesAsTarget",
			args: GetRecommendedInstancesArgs{
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
			},
			expectedRecommended: int32(6),
			expectedSkip:        false,
		},
		{
			name: "givenMissingInstances_whenShouldScaleUp_thenTreatMissingInstancesAsZero",
			args: GetRecommendedInstancesArgs{
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
			},
			expectedRecommended: int32(24),
			expectedSkip:        false,
		},
		{
			name: "givenTooManyMissingInstances_whenEstimatedResultIsOpposite_thenReturnCurrent",
			args: GetRecommendedInstancesArgs{
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
			},
			expectedRecommended: int32(58),
			expectedSkip:        false,
		},
		{
			name: "givenTooManyNonExistingInstances_whenEstimatedResultIsOpposite_thenReturnCurrent",
			args: GetRecommendedInstancesArgs{
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
			},
			expectedRecommended: int32(58),
			expectedSkip:        false,
		},
		{
			name: "givenExternalMetric_whenWithinLowestTolerance_thenReturnCurrent",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(10),
				Tolerance:             0.51,
				MetricTargets:         MetricsMap{"a": 3.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{"a": 14.9},
			},
			expectedRecommended: int32(10),
			expectedSkip:        false,
		},
		{
			name: "givenExternalMetric_whenOutOfLowestTolerance_thenReturnDesired",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(10),
				Tolerance:             0.49,
				MetricTargets:         MetricsMap{"a": 3.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{"a": 14.9},
			},
			expectedRecommended: int32(5),
			expectedSkip:        false,
		},
		{
			name: "givenExternalMetric_whenWithinHighestTolerance_thenReturnCurrent",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(10),
				Tolerance:             0.51,
				MetricTargets:         MetricsMap{"a": 3.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{"a": 44.9},
			},
			expectedRecommended: int32(10),
			expectedSkip:        false,
		},
		{
			name: "givenExternalMetric_whenOutOfHighestTolerance_thenReturnDesired",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(10),
				Tolerance:             0.49,
				MetricTargets:         MetricsMap{"a": 3.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{"a": 44.9},
			},
			expectedRecommended: int32(15),
			expectedSkip:        false,
		},
		{
			name: "givenExternalTargetWithZeroValue_thenReturnMaximumInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(1),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 0.0},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{"a": 1.0},
			},
			expectedRecommended: int32(100),
			expectedSkip:        false,
		},
		{
			name: "givenExternalTargetWithExtremelySmallValue_thenReturnMaximumInstances",
			args: GetRecommendedInstancesArgs{
				MinInstances:          int32(1),
				MaxInstances:          int32(100),
				CurrentInstancesCount: int32(1),
				Tolerance:             0.0,
				MetricTargets:         MetricsMap{"a": 1e-100},
				UnreadyInstancesCount: int32(0),
				ReadyInstancesMetrics: []MetricsMap{},
				ExternalMetrics:       MetricsMap{"a": 1e100},
			},
			expectedRecommended: int32(100),
			expectedSkip:        false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			recommended, skip := GetRecommendedInstances(tc.args)
			assert.Equal(tc.expectedRecommended, recommended)
			assert.Equal(tc.expectedSkip, skip)
		})
	}
}
