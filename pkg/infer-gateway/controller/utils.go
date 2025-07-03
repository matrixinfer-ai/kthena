package controller

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/conf"
)

const ConfigMapPath = "/etc/config/schedulerConfiguration.yaml"

func isPodReady(pod *corev1.Pod) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			if condition.Status == corev1.ConditionTrue {
				return true
			}
			break
		}
	}
	return false
}

func loadSchedulerConfig() {
	data, err := os.ReadFile(ConfigMapPath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	var kubeSchedulerConfiguration conf.KubeSchedulerConfiguration
	if err := yaml.Unmarshal(data, &kubeSchedulerConfiguration); err != nil {
		log.Error("failed to Unmarshal schedulerConfiguration: %v", err)
		return
	}

	if err = unmarshalPluginsConfig(&kubeSchedulerConfiguration); err != nil {
		log.Error("failed to Unmarshal PluginsConfig: %v", err)
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
			for _, plugin := range profiles.Plugins.PreFilter.Enabled {
				conf.FilterPlugins = append(conf.FilterPlugins, plugin.Name)
			}
		}
	}
	return nil
}
