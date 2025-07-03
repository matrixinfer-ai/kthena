package scheduler

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

func init() {
	framework.RegisterPluginBuilder(plugins.KVCachePluginName, plugins.NewGPUCacheUsage())
	framework.RegisterPluginBuilder(plugins.LeastLatencyPluginName, plugins.NewLeastLatency())
	framework.RegisterPluginBuilder(plugins.LeastRequestPluginName, plugins.NewLeastRequest())
	framework.RegisterPluginBuilder(plugins.LoraAffinityPluginName, plugins.NewLoraAffinity())
	// The prefix-cache type is handled separately
	framework.RegisterPluginBuilder(plugins.PrefixCachePluginName, nil)
}
