package plugins

import (
	"istio.io/istio/pkg/slices"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

const LoraAffinityPluginName = "lora-affinity"

type LoraAffinity struct {
	name string
}

var _ framework.Plugin = &LoraAffinity{}

func NewLoraAffinity() *LoraAffinity {
	return &LoraAffinity{
		name: LoraAffinityPluginName,
	}
}

func (l *LoraAffinity) Name() string {
	return l.name
}

func (l *LoraAffinity) Filter(ctx *framework.Context, pods []*datastore.PodInfo) []*datastore.PodInfo {
	return slices.FilterInPlace(pods, func(info *datastore.PodInfo) bool {
		// TODO: add lock protection
		_, ok := info.Models[ctx.Model]
		return ok
	})
}

func (l *LoraAffinity) Score(ctx *framework.Context, pods []*datastore.PodInfo) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)

	// Initialize all pods with score 0
	for _, pod := range pods {
		scoreResults[pod] = 0
	}
	return scoreResults
}
