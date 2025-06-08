package plugins

import (
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
	for _, info := range pods {
		score := int((float64(l.maxWaitingRequest) - info.RequestWaitingNum) * 100 / float64(l.maxWaitingRequest))
		scoreResults[info] = score
	}
	return scoreResults
}
