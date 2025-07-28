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
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

type ScorePluginFactory = func(arg runtime.RawExtension) framework.ScorePlugin
type FilterPluginFactory = func(arg runtime.RawExtension) framework.FilterPlugin

// PluginRegistry manages the registration and retrieval of scheduler plugins
type PluginRegistry struct {
	mutex                sync.RWMutex
	scorePluginBuilders  map[string]ScorePluginFactory
	filterPluginBuilders map[string]FilterPluginFactory
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		scorePluginBuilders:  make(map[string]ScorePluginFactory),
		filterPluginBuilders: make(map[string]FilterPluginFactory),
	}
}

// RegisterScorePlugin registers a score plugin builder in this registry
func (r *PluginRegistry) RegisterScorePlugin(name string, sp ScorePluginFactory) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.scorePluginBuilders[name] = sp
}

// GetScorePlugin retrieves a score plugin builder from this registry
func (r *PluginRegistry) GetScorePlugin(name string) (ScorePluginFactory, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	sp, exist := r.scorePluginBuilders[name]
	return sp, exist
}

// RegisterFilterPlugin registers a filter plugin builder in this registry
func (r *PluginRegistry) RegisterFilterPlugin(name string, fp FilterPluginFactory) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.filterPluginBuilders[name] = fp
}

// GetFilterPlugin retrieves a filter plugin builder from this registry
func (r *PluginRegistry) GetFilterPlugin(name string) (FilterPluginFactory, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	fp, exist := r.filterPluginBuilders[name]
	return fp, exist
}

// registerDefaultPlugins registers all default plugins to the given registry
func registerDefaultPlugins(registry *PluginRegistry) {
	// scorePlugin
	registry.RegisterScorePlugin(plugins.KVCachePluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return plugins.NewGPUCacheUsage()
	})
	registry.RegisterScorePlugin(plugins.LeastLatencyPluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return plugins.NewLeastLatency(args)
	})
	registry.RegisterScorePlugin(plugins.LeastRequestPluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return plugins.NewLeastRequest(args)
	})
	// PrefixCache requires two parameters and is instantiated during use
	registry.RegisterScorePlugin(plugins.PrefixCachePluginName, func(args runtime.RawExtension) framework.ScorePlugin {
		return &plugins.PrefixCache{}
	})

	// filterPlugin
	registry.RegisterFilterPlugin(plugins.LeastRequestPluginName, func(args runtime.RawExtension) framework.FilterPlugin {
		return plugins.NewLeastRequest(args)
	})
	registry.RegisterFilterPlugin(plugins.LoraAffinityPluginName, func(args runtime.RawExtension) framework.FilterPlugin {
		return plugins.NewLoraAffinity()
	})
}

func getFilterPlugins(registry *PluginRegistry, filterPluginMap []string, pluginsArgMap map[string]runtime.RawExtension) []framework.FilterPlugin {
	var list []framework.FilterPlugin
	// TODO: enable lora affinity when models from metrics are available.
	for _, pluginName := range filterPluginMap {
		if factory, exist := registry.GetFilterPlugin(pluginName); !exist {
			klog.Errorf("Failed to get plugin %s.", pluginName)
			continue
		} else {
			plugin := factory(pluginsArgMap[pluginName])
			list = append(list, plugin)
		}
	}
	return list
}

func getScorePlugins(registry *PluginRegistry, prefixCache *plugins.PrefixCache, scorePluginMap map[string]int, pluginsArgMap map[string]runtime.RawExtension) []*scorePlugin {
	var list []*scorePlugin
	for pluginName, weight := range scorePluginMap {
		if weight < 0 {
			klog.Errorf("Weight for plugin '%s' is invalid, value is %d. Setting to 0", pluginName, weight)
			weight = 0
		}

		if pluginName == plugins.PrefixCachePluginName {
			list = append(list, &scorePlugin{
				plugin: prefixCache,
				weight: weight,
			})
			continue
		}

		if pb, exist := registry.GetScorePlugin(pluginName); !exist {
			klog.Errorf("Failed to get plugin %s.", pluginName)
		} else {
			list = append(list, &scorePlugin{
				plugin: pb(pluginsArgMap[pluginName]),
				weight: weight,
			})
		}
	}
	return list
}
