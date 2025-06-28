package plugins

import (
	"math"

	"istio.io/istio/pkg/slices"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

const MaxWaitingRequests = 10
const LeastRequestPluginName = "least-request"

var _ framework.FilterPlugin = &LeastRequest{}
var _ framework.ScorePlugin = &LeastRequest{}

type LeastRequest struct {
	name              string
	maxWaitingRequest int
}

func NewLeastRequest() *LeastRequest {
	return &LeastRequest{
		name:              LeastRequestPluginName,
		maxWaitingRequest: MaxWaitingRequests,
	}
}

func (l *LeastRequest) Name() string {
	return l.name
}

func (l *LeastRequest) Filter(ctx *framework.Context, pods []*datastore.PodInfo) []*datastore.PodInfo {
	return slices.FilterInPlace(pods, func(info *datastore.PodInfo) bool {
		return info.RequestWaitingNum < float64(l.maxWaitingRequest)
	})
}

func (l *LeastRequest) Score(ctx *framework.Context, pods []*datastore.PodInfo) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)
	if len(pods) == 0 {
		return scoreResults
	}

	// 1. Calculate the base score (running reqs + 2 * waiting reqs) for each pod
	baseScores := make(map[*datastore.PodInfo]float64)
	minScore := math.MaxFloat64
	maxScore := 0.0
	for _, info := range pods {
		base := info.RequestRunningNum + 2*info.RequestWaitingNum
		baseScores[info] = base
		if base < minScore {
			minScore = base
		}
		if base > maxScore {
			maxScore = base
		}
	}

	const MaxScore = 100.0
	// 2. Normalize the score: lower base score means higher normalized score (range 0-100)
	for _, info := range pods {
		score := MaxScore
		if maxScore > minScore {
			score = MaxScore * (maxScore - baseScores[info]) / (maxScore - minScore)
		}
		scoreResults[info] = int(score)
	}
	return scoreResults
}
