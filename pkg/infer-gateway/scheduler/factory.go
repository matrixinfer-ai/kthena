package scheduler

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

func init() {
	// scorePlugin
	framework.RegisterScorePluginBuilder(plugins.KVCachePluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return &plugins.GPUCacheUsage{}
	})
	framework.RegisterScorePluginBuilder(plugins.LeastLatencyPluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return &plugins.LeastLatency{}
	})
	framework.RegisterScorePluginBuilder(plugins.LeastRequestPluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return &plugins.LeastRequest{}
	})
	framework.RegisterScorePluginBuilder(plugins.PrefixCachePluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return &plugins.PrefixCache{}
	})

	// filterPlugin
	framework.RegisterFilterPluginBuilder(plugins.LeastRequestPluginName, func(args map[string]interface{}) framework.FilterPlugin {
		return &plugins.LeastRequest{}
	})
	framework.RegisterFilterPluginBuilder(plugins.LoraAffinityPluginName, func(args map[string]interface{}) framework.FilterPlugin {
		return &plugins.LoraAffinity{}
	})
}
