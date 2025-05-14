package plugins

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

var _ framework.ScorePlugin = &GPUCacheUsage{}

const KVCachePluginName = "kv-cache"

type GPUCacheUsage struct {
	name string
}

func NewGPUCacheUsage() *GPUCacheUsage {
	return &GPUCacheUsage{
		name: KVCachePluginName,
	}
}

func (g *GPUCacheUsage) Name() string {
	return g.name
}
func (g *GPUCacheUsage) Score(pods []*datastore.PodInfo, ctx *framework.Context) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)
	for _, info := range pods {
		score := int((1.0 - info.GPUCacheUsage) * 100)
		scoreResults[info] = score
	}

	return scoreResults
}
