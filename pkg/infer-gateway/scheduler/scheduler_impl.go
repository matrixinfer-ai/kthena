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
	prefixCache := plugins.NewPrefixCache(store)
	return &SchedulerImpl{
		store: store,
		filterPlugins: []framework.FilterPlugin{
			// TODO: enable lora affinity when models from metrics are available.
			// plugins.NewLoraAffinity(),
			plugins.NewLeastRequest(),
		},
		scorePlugins: []*scorePlugin{
			// TODO: set the weight of each plugin properly.
			{
				plugin: plugins.NewLeastRequest(),
				weight: 1,
			},
			{
				plugin: plugins.NewGPUCacheUsage(),
				weight: 1,
			},
			{
				plugin: plugins.NewLeastLatency(),
				weight: 1,
			},
			{
				plugin: prefixCache,
				weight: 1,
			},
		},
		ScheduleHooks: []framework.ScheduleHook{
			prefixCache,
		},
	}
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
