package scheduler

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

func init() {
	// scorePlugin
	framework.RegisterScorePluginBuilder(plugins.KVCachePluginName, plugins.NewGPUCacheUsage())
	framework.RegisterScorePluginBuilder(plugins.LeastLatencyPluginName, plugins.NewLeastLatency())
	framework.RegisterScorePluginBuilder(plugins.LeastRequestPluginName, plugins.NewLeastRequest())
	// The prefix-cache type is handled separately
	framework.RegisterScorePluginBuilder(plugins.PrefixCachePluginName, nil)

	// filterPlugin
	framework.RegisterFilterPluginBuilder(plugins.LeastRequestPluginName, plugins.NewLeastRequest())
	framework.RegisterFilterPluginBuilder(plugins.LoraAffinityPluginName, plugins.NewLoraAffinity())
}
