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
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

var (
	log = logger.NewLogger("scheduler")
)

const (
	// Get the top five scoring podinfo
	topN = 5
)

type SchedulerImpl struct {
	store datastore.Store

	filterPlugins []framework.FilterPlugin
	scorePlugins  []*scorePlugin

	ScheduleHooks []framework.ScheduleHook
}

type scorePlugin struct {
	plugin framework.ScorePlugin
	weight int
}

type podInfoWithValue struct {
	pod   *datastore.PodInfo
	score int
}

func NewScheduler(store datastore.Store) Scheduler {
	scorePluginMap, filterPluginMap, pluginsArgsMap, err := utils.LoadSchedulerConfig()
	if err != nil {
		log.Errorf("failed to Load Scheduler: %v", err)
	}
	prefixCache := plugins.NewPrefixCache(store, pluginsArgsMap)
	return &SchedulerImpl{
		store:         store,
		filterPlugins: ParseFilterPlugin(filterPluginMap, pluginsArgsMap),
		scorePlugins:  GetScorePlugin(prefixCache, scorePluginMap, pluginsArgsMap),
		ScheduleHooks: []framework.ScheduleHook{
			prefixCache,
		},
	}
}

func ParseFilterPlugin(filterPluginMap []string, pluginsArgsMap map[string]interface{}) []framework.FilterPlugin {
	var list []framework.FilterPlugin
	// TODO: enable lora affinity when models from metrics are available.
	for _, pluginName := range filterPluginMap {
		if factory, exist := framework.GetFilterPluginBuilder(pluginName); !exist {
			log.Errorf("Failed to get plugin %s.", pluginName)
			continue
		} else {
			plugin := factory(pluginsArgsMap)
			list = append(list, plugin)
		}
	}
	return list
}

// TODO: set the weight of each plugin properly.
func GetScorePlugin(prefixCache *plugins.PrefixCache, scorePluginMap map[string]int, pluginsArgsMap map[string]interface{}) []*scorePlugin {
	var list []*scorePlugin
	for key, value := range scorePluginMap {
		if key == plugins.PrefixCachePluginName {
			list = append(list, &scorePlugin{
				plugin: prefixCache,
				weight: value,
			})
		} else {
			if pb, exist := framework.GetScorePluginBuilder(key); !exist {
				log.Errorf("Failed to get plugin %s.", key)
			} else {
				list = append(list, &scorePlugin{
					plugin: pb(pluginsArgsMap),
					weight: value,
				})
			}
		}
	}
	return list
}

func (s *SchedulerImpl) Schedule(req map[string]interface{}, pods []*datastore.PodInfo, pdGroup *aiv1alpha1.PDGroup) (*framework.Context, error) {
	if len(pods) == 0 {
		return nil, fmt.Errorf("pods shouldn't be empty")
	}

	prompt, err := utils.GetPrompt(req)
	if err != nil {
		return nil, err
	}

	ctx := &framework.Context{
		Model:  req["model"].(string),
		Prompt: prompt,
	}

	// Since the elements in the ctxSlice have the same model and prompt, it is straightforward to take the first element
	pods, err = s.RunFilterPlugins(pods, ctx)
	if err != nil {
		return nil, err
	}

	originalPods := make([]*datastore.PodInfo, len(pods))
	copy(originalPods, pods)

	var pdFilter framework.FilterPlugin
	if pdGroup != nil {
		// Initialize PDFilter plugin if PD disaggregation is enabled.

		// First filter out decode pods.
		// NOTE: Further optimization can be done on whether to filter out decode pod or prefill pod first,
		// or even how to select the best PD group.
		pdFilter = plugins.NewPDFilter(pdGroup.DecodeLabels, pdGroup.PrefillLabels, pdGroup.GroupKey)
		pods = pdFilter.Filter(ctx, pods)

		if len(pods) == 0 {
			return nil, fmt.Errorf("no decode pod found")
		}
	}

	log.Debugf("Running score plugins for decode pod")
	scores, err := s.RunScorePlugins(pods, ctx)
	if err != nil {
		return nil, err
	}

	topNDecodePods := TopNPodInfos(scores, topN)
	ctx.DecodePods = topNDecodePods

	if pdGroup != nil {
		prefillPods := make([]*datastore.PodInfo, len(topNDecodePods))
		for i := range ctx.DecodePods {
			ctx.PDIndex = i
			// Filter prefill pods if PD disaggregation is enabled.
			// Also make sure the prefill pod is in the same infer group of decode pod we get before.
			selectedPods := pdFilter.Filter(ctx, originalPods)

			if len(selectedPods) == 0 {
				return nil, fmt.Errorf("no prefill pod found")
			}

			log.Debugf("Running score plugins for prefill pod")
			scores, err = s.RunScorePlugins(selectedPods, ctx)
			if err != nil {
				return nil, err
			}

			bestPrefillPod := TopNPodInfos(scores, 1)
			prefillPods[i] = bestPrefillPod[0]
		}
		ctx.PrefillPods = prefillPods
	}

	return ctx, nil
}

func (s *SchedulerImpl) RunFilterPlugins(pods []*datastore.PodInfo, ctx *framework.Context) ([]*datastore.PodInfo, error) {
	for _, filterPlugin := range s.filterPlugins {
		pods = filterPlugin.Filter(ctx, pods)
		if len(pods) == 0 {
			return nil, fmt.Errorf("pods have all been filtered out by %q", filterPlugin.Name())
		}
	}

	return pods, nil
}

func (s *SchedulerImpl) RunScorePlugins(pods []*datastore.PodInfo, ctx *framework.Context) (map[*datastore.PodInfo]int, error) {
	res := make(map[*datastore.PodInfo]int)
	for _, scorePlugin := range s.scorePlugins {
		scores := scorePlugin.plugin.Score(ctx, pods)
		log.Debugf("ScorePlugin: %s", scorePlugin.plugin.Name())
		for k, v := range scores {
			if k.Pod != nil {
				log.Debugf("Pod: %s/%s, Score: %d", k.Pod.Namespace, k.Pod.Name, v)
			}
			if _, ok := res[k]; !ok {
				res[k] = v * scorePlugin.weight
			} else {
				res[k] += v * scorePlugin.weight
			}
		}
	}

	if log.Logger != nil && log.Logger.IsLevelEnabled(logrus.DebugLevel) {
		log.Debugf("Final Pod Scores:")
		for k, v := range res {
			if k.Pod != nil {
				log.Debugf("  Pod: %s/%s, Final Score: %d", k.Pod.Namespace, k.Pod.Name, v)
			}
		}
	}

	return res, nil
}

func (s *SchedulerImpl) RunPostHooks(ctx *framework.Context, index int) {
	for _, hook := range s.ScheduleHooks {
		hook.PostSchedule(ctx, index)
	}
}

func TopNPodInfos(m map[*datastore.PodInfo]int, n int) []*datastore.PodInfo {
	var list []podInfoWithValue
	for k, v := range m {
		list = append(list, podInfoWithValue{pod: k, score: v})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})

	res := []*datastore.PodInfo{}
	for i := range list {
		if i >= n {
			break
		}
		res = append(res, list[i].pod)
	}

	return res
}
