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
	"k8s.io/apimachinery/pkg/types"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
	"sigs.k8s.io/yaml"
)

var (
	log = logger.NewLogger("utils")

	GPUCacheUsage     = "gpu_usage"
	RequestWaitingNum = "request_waiting_num"
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

func LoadSchedulerConfig() {
	data, err := os.ReadFile(ConfigMapPath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	var kubeSchedulerConfiguration conf.KubeSchedulerConfiguration
	if err := yaml.Unmarshal(data, &kubeSchedulerConfiguration); err != nil {
		log.Errorf("failed to Unmarshal schedulerConfiguration: %v", err)
		return
	}

	if err = unmarshalPluginsConfig(&kubeSchedulerConfiguration); err != nil {
		log.Errorf("failed to Unmarshal PluginsConfig: %v", err)
		return
	}
}

func unmarshalPluginsConfig(schedulerConfig *conf.KubeSchedulerConfiguration) error {
	if len(schedulerConfig.Profiles) == 0 {
		return fmt.Errorf("profiles is empty")
	}
	for _, profiles := range schedulerConfig.Profiles {
		if len(profiles.PluginConfig) > 0 {
			for _, pluginConfig := range profiles.PluginConfig {
				conf.PluginsArgs[pluginConfig.Name] = pluginConfig.Args
			}
		}

		if profiles.Plugins == nil {
			continue
		}

		if profiles.Plugins.PreScore != nil && len(profiles.Plugins.PreScore.Enabled) > 0 {
			for _, plugin := range profiles.Plugins.PreScore.Enabled {
				conf.ScorePluginMap[plugin.Name] = plugin.Weight
			}
		}

		if profiles.Plugins.PreFilter != nil && len(profiles.Plugins.PreFilter.Enabled) > 0 {
			conf.FilterPlugins = profiles.Plugins.PreFilter.Enabled
		}
	}
	return nil
}
