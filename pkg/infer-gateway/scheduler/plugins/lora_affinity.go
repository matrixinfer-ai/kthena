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

var _ framework.FilterPlugin = &LoraAffinity{}

func NewLoraAffinity() *LoraAffinity {
	return &LoraAffinity{
		name: LoraAffinityPluginName,
	}
}

func (l *LoraAffinity) Name() string {
	return l.name
}

func (l *LoraAffinity) Filter(pods []*datastore.PodInfo, ctx *framework.Context) []*datastore.PodInfo {
	return slices.FilterInPlace(pods, func(info *datastore.PodInfo) bool {
		// TODO: add lock protection
		_, ok := info.Models[ctx.Model]
		return ok
	})
}
