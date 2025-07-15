package scheduler

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

func init() {
	// scorePlugin
	framework.RegisterScorePluginBuilder(plugins.KVCachePluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return plugins.NewGPUCacheUsage()
	})
	framework.RegisterScorePluginBuilder(plugins.LeastLatencyPluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return plugins.NewLeastLatency(args)
	})
	framework.RegisterScorePluginBuilder(plugins.LeastRequestPluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return plugins.NewLeastRequest(args)
	})
	// PrefixCache requires two parameters and is instantiated during use
	framework.RegisterScorePluginBuilder(plugins.PrefixCachePluginName, func(args map[string]interface{}) framework.ScorePlugin {
		return &plugins.PrefixCache{}
	})

	// filterPlugin
	framework.RegisterFilterPluginBuilder(plugins.LeastRequestPluginName, func(args map[string]interface{}) framework.FilterPlugin {
		return plugins.NewLeastRequest(args)
	})
	framework.RegisterFilterPluginBuilder(plugins.LoraAffinityPluginName, func(args map[string]interface{}) framework.FilterPlugin {
		return plugins.NewLoraAffinity()
	})
}
