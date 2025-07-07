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

package plugins

import (
	"istio.io/istio/pkg/slices"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
)

const LeastRequestPluginName = "least-request"

var _ framework.Plugin = &LeastRequest{}

type LeastRequest struct {
	name              string
	maxWaitingRequest int
}

func NewLeastRequest() *LeastRequest {
	return &LeastRequest{
		name:              LeastRequestPluginName,
		maxWaitingRequest: conf.PluginsArgs[LeastRequestPluginName].MaxWaitingRequests,
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
