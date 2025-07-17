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

package scheduler

import (
	"k8s.io/apimachinery/pkg/runtime"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

func init() {
	// scorePlugin
	framework.RegisterScorePluginBuilder(plugins.KVCachePluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return plugins.NewGPUCacheUsage()
	})
	framework.RegisterScorePluginBuilder(plugins.LeastLatencyPluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return plugins.NewLeastLatency(args)
	})
	framework.RegisterScorePluginBuilder(plugins.LeastRequestPluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return plugins.NewLeastRequest(args)
	})
	// PrefixCache requires two parameters and is instantiated during use
	framework.RegisterScorePluginBuilder(plugins.PrefixCachePluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return &plugins.PrefixCache{}
	})

	// filterPlugin
	framework.RegisterFilterPluginBuilder(plugins.LeastRequestPluginName, func(args runtime.RawExtension) framework.FilterPlugin {
		return plugins.NewLeastRequest(args)
	})
	framework.RegisterFilterPluginBuilder(plugins.LoraAffinityPluginName, func(args runtime.RawExtension) framework.FilterPlugin {
		return plugins.NewLoraAffinity()
	})
}
