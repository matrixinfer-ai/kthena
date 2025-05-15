package plugins

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

const PrefixCachePluginName = "prefix-cache"

var _ framework.ScorePlugin = &PrefixCache{}

type PrefixCache struct {
	name string
}

func NewPrefixCache() *PrefixCache {
	return &PrefixCache{
		name: PrefixCachePluginName,
	}
}

func (p *PrefixCache) Name() string {
	return p.name
}

func (p *PrefixCache) Score(pods []*datastore.PodInfo, ctx *framework.Context) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)

	return scoreResults
}

func (p *PrefixCache) PostSchedule(ctx *framework.Context) {

}
