package plugins

import (
	"istio.io/istio/pkg/slices"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

const MaxWaitingRequests = 100
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

	// 1. Calculate the base score (running reqs + 100 * waiting reqs) for each pod
	baseScores := make(map[*datastore.PodInfo]float64)
	maxScore := 0.0
	for _, info := range pods {
		base := info.RequestRunningNum + 100*info.RequestWaitingNum
		baseScores[info] = base
		if base > maxScore {
			maxScore = base
		}
	}

	// 2. Calculate the score for each pod as a percentage of the max base score
	for _, info := range pods {
		score := ((maxScore - baseScores[info]) / maxScore) * 100
		scoreResults[info] = int(score)
	}

	return scoreResults
}
