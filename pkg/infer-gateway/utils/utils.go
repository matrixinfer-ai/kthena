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

package utils

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
	"sigs.k8s.io/yaml"
)

var (
	log = logger.NewLogger("utils")

	GPUCacheUsage     = "gpu_usage"
	RequestWaitingNum = "request_waiting_num"
	RequestRunningNum = "request_running_num"
	TPOT              = "TPOT"
	TTFT              = "TTFT"
)

const ConfigMapPath = "/etc/config/schedulerConfiguration.yaml"

func GetNamespaceName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func GetPrompt(body map[string]interface{}) (string, error) {
	if prompt, ok := body["prompt"]; ok {
		res, ok := prompt.(string)
		if !ok {
			return "", fmt.Errorf("prompt is not a string")
		}
		return res, nil
	}

	if messages, ok := body["messages"]; ok {
		messageList, ok := messages.([]interface{})
		if !ok {
			return "", fmt.Errorf("messages is not a list")
		}

		res := ""
		for _, message := range messageList {
			msgMap, ok := message.(map[string]interface{})
			if !ok {
				continue
			}

			content, ok := msgMap["content"]
			if !ok {
				continue
			}

			contentStr, ok := content.(string)
			if !ok {
				continue
			}

			role, ok := msgMap["role"]
			if !ok {
				continue
			}

			roleStr, ok := role.(string)
			if !ok {
				continue
			}

			res += fmt.Sprintf("<|im_start|>%s\n%s<|im_end|>\n", roleStr, contentStr)
		}

		return res, nil
	}

	return "", fmt.Errorf("prompt or messages not found in request body")
}

func LoadSchedulerConfig() (map[string]int, []string, map[string]runtime.RawExtension, error) {
	data, err := os.ReadFile(ConfigMapPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read config file %s: %w", ConfigMapPath, err)
	}
	var kubeSchedulerConfiguration conf.SchedulerConfiguration
	if err := yaml.Unmarshal(data, &kubeSchedulerConfiguration); err != nil {
		log.Errorf("failed to Unmarshal schedulerConfiguration: %v", err)
		return nil, nil, nil, fmt.Errorf("failed to Unmarshal schedulerConfiguration: %v", err)
	}

	scorePluginMap, filterPlugins, err := unmarshalPlugins(&kubeSchedulerConfiguration)
	if err != nil {
		log.Errorf("failed to Unmarshal Plugins: %v", err)
		return nil, nil, nil, fmt.Errorf("failed to Unmarshal Plugins: %v", err)
	}

	pluginsArgMap, err := unmarshalPluginsConfig(&kubeSchedulerConfiguration)
	if err != nil {
		log.Errorf("failed to Unmarshal PluginsConfig: %v", err)
		return nil, nil, nil, fmt.Errorf("failed to Unmarshal PluginsConfig: %v", err)
	}

	return scorePluginMap, filterPlugins, pluginsArgMap, nil
}

func unmarshalPlugins(schedulerConfig *conf.SchedulerConfiguration) (map[string]int, []string, error) {
	var filterPlugins []string
	scorePluginMap := make(map[string]int)

	if len(schedulerConfig.Plugins.Score.Enabled) > 0 {
		for _, plugin := range schedulerConfig.Plugins.Score.Enabled {
			scorePluginMap[plugin.Name] = plugin.Weight
		}
	}

	if len(schedulerConfig.Plugins.Filter.Enabled) > 0 {
		filterPlugins = schedulerConfig.Plugins.Filter.Enabled
	}
	return scorePluginMap, filterPlugins, nil
}

func unmarshalPluginsConfig(schedulerConfig *conf.SchedulerConfiguration) (map[string]runtime.RawExtension, error) {
	pluginsArgMap := make(map[string]runtime.RawExtension)

	if len(schedulerConfig.PluginConfig) > 0 {
		for _, pluginArg := range schedulerConfig.PluginConfig {
			pluginsArgMap[pluginArg.Name] = pluginArg.Args
		}
	}

	return pluginsArgMap, nil
}
