package histogram

import (
	"math"
	"testing"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

const Epsilon = 1e-6

func newSnapshot(sum float64, count int64, buckets []Bucket) *Snapshot {
	return &Snapshot{
		sum:     sum,
		count:   count,
		buckets: buckets,
	}
}

func toPtr[T any](value T) *T {
	return &value
}

func Test_givenPrometheusHistogram_whenConvertToSnapshot_thenOk(t *testing.T) {
	assert := assert.New(t)

	histogram := io_prometheus_client.Histogram{
		SampleCount: toPtr(uint64(100)),
		SampleSum:   toPtr(256.7),
		Bucket: []*io_prometheus_client.Bucket{
			{
				CumulativeCount: toPtr(uint64(10)),
				UpperBound:      toPtr(0.1),
			},
			{
				CumulativeCount: toPtr(uint64(30)),
				UpperBound:      toPtr(0.5),
			},
			{
				CumulativeCount: toPtr(uint64(100)),
				UpperBound:      toPtr(math.Inf(1)),
			},
		},
	}

	snapshot := NewSnapshotOfHistogram(&histogram)

	assert.InDelta(256.7, snapshot.sum, Epsilon)
	assert.Equal(int64(100), snapshot.count)
	assert.Equal(3, len(snapshot.buckets))

	assert.InDelta(0.1, snapshot.buckets[0].leValue, Epsilon)
	assert.Equal(int64(10), snapshot.buckets[0].count)

	assert.InDelta(0.5, snapshot.buckets[1].leValue, Epsilon)
	assert.Equal(int64(30), snapshot.buckets[1].count)

	assert.True(math.IsInf(snapshot.buckets[2].leValue, 1))
	assert.Equal(int64(100), snapshot.buckets[2].count)
}

func Test_givenNegativeCountInDiff_whenQuantileInDiff_thenError(t *testing.T) {
	assert := assert.New(t)

	past := newSnapshot(3.0, 5, []Bucket{
		{0.5, 2},
		{math.Inf(1), 5},
	})
	now := newSnapshot(2.0, 4, []Bucket{
		{0.5, 2},
		{math.Inf(1), 4},
	})

	_, err := QuantileInDiff(95, now, past)

	assert.EqualError(err, "invalid totalCountDiff (-1 invalid out of [0, +Inf))")
}

func Test_givenZeroCountInDiff_whenQuantileInDiff_thenReturnZero(t *testing.T) {
	assert := assert.New(t)

	past := newSnapshot(3.0, 5, []Bucket{
		{0.5, 2},
		{math.Inf(1), 5},
	})

	result, err := QuantileInDiff(95, past, past)

	assert.InDelta(0.0, result, Epsilon)
	assert.Nil(err)
}

func Test_givenUnmatchedBucketSizes_whenQuantileInDiff_thenError(t *testing.T) {
	assert := assert.New(t)

	past := newSnapshot(3.0, 5, []Bucket{
		{0.5, 2},
		{math.Inf(1), 5},
	})
	now := newSnapshot(4.0, 8, []Bucket{
		{0.5, 2},
		{0.75, 4},
		{math.Inf(1), 8},
	})

	_, err := QuantileInDiff(95, now, past)

	assert.EqualError(err, "unmatched buckets lengths (len(now.buckets): 3, len(past.buckets): 2)")
}

func Test_givenNonAscendingHistograms_whenQuantileInDiff_thenError(t *testing.T) {
	assert := assert.New(t)

	past := newSnapshot(3.0, 5, []Bucket{
		{0.5, 2},
		{0.75, 4},
		{math.Inf(1), 5},
	})
	now := newSnapshot(4.0, 8, []Bucket{
		{0.5, 4},
		{0.75, 5},
		{math.Inf(1), 8},
	})

	_, err := QuantileInDiff(95, now, past)

	assert.EqualError(err, "non-decreasing is broken")
}

func Test_givenTwoHistogramsWithUniqueAnswerInLastBucket_whenQuantileInDiff_thenReturnDoubleBound(t *testing.T) {
	assert := assert.New(t)

	past := newSnapshot(3.0, 5, []Bucket{
		{0.5, 2},
		{0.75, 4},
		{math.Inf(1), 5},
	})
	now := newSnapshot(6.0, 10, []Bucket{
		{0.5, 4},
		{0.75, 8},
		{math.Inf(1), 10},
	})

	result, err := QuantileInDiff(100, now, past)

	assert.InDelta(1.5, result, Epsilon)
	assert.Nil(err)
}

func Test_givenOneDefaultHistogram_whenQuantileInDiff_thenOk(t *testing.T) {
	assert := assert.New(t)

	past := NewDefaultSnapshot()
	now := newSnapshot(4.0, 8, []Bucket{
		{0.5, 2},
		{0.75, 4},
		{math.Inf(1), 8},
	})

	result, err := QuantileInDiff(95, now, past)

	assert.InDelta(1.5, result, Epsilon)
	assert.Nil(err)
}

func Test_givenTwoDefaultHistograms_whenQuantileInDiff_thenReturnZero(t *testing.T) {
	assert := assert.New(t)

	past := NewDefaultSnapshot()

	result, err := QuantileInDiff(95, past, past)

	assert.InDelta(0.0, result, Epsilon)
	assert.Nil(err)
}

func Test_givenTwoHistogramsWithUniqueAnswerInFirstBucket_whenQuantileInDiff_thenReturnFirstBound(t *testing.T) {
	assert := assert.New(t)

	past := newSnapshot(3.0, 5, []Bucket{
		{0.5, 2},
		{0.75, 4},
		{math.Inf(1), 5},
	})
	now := newSnapshot(6.0, 10, []Bucket{
		{0.5, 3},
		{0.75, 8},
		{math.Inf(1), 10},
	})

	result, err := QuantileInDiff(1, now, past)

	assert.InDelta(0.5, result, Epsilon)
	assert.Nil(err)
}

func Test_givenTwoHistogramsWithEverythingInOneBucket_whenQuantileInDiff_thenReturnLinearInterpolationResult(t *testing.T) {
	assert := assert.New(t)

	past := newSnapshot(3.0, 5, []Bucket{
		{0.5, 2},
		{0.75, 4},
		{math.Inf(1), 5},
	})
	now := newSnapshot(6.0, 10, []Bucket{
		{0.5, 2},
		{0.75, 9},
		{math.Inf(1), 10},
	})

	result, err := QuantileInDiff(50, now, past)

	assert.InDelta(0.65, result, Epsilon)
	assert.Nil(err)
}
