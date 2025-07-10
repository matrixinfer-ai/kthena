package scheduler

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

func init() {
	// scorePlugin
	framework.RegisterScorePluginBuilder(plugins.KVCachePluginName, &plugins.GPUCacheUsage{})
	framework.RegisterScorePluginBuilder(plugins.LeastLatencyPluginName, &plugins.LeastLatency{})
	framework.RegisterScorePluginBuilder(plugins.LeastRequestPluginName, &plugins.LeastRequest{})
	// The prefix-cache type is handled separately
	framework.RegisterScorePluginBuilder(plugins.PrefixCachePluginName, &plugins.PrefixCache{})

	// filterPlugin
	framework.RegisterFilterPluginBuilder(plugins.LeastRequestPluginName, &plugins.LeastRequest{})
	framework.RegisterFilterPluginBuilder(plugins.LoraAffinityPluginName, &plugins.LoraAffinity{})
}
