package scheduler

import (
	"fmt"
	"math"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins"
)

type SchedulerImpl struct {
	store datastore.Store

	filterPlugins []framework.FilterPlugin
	scorePlugins  []*scorePlugin
}

type scorePlugin struct {
	plugin framework.ScorePlugin
	weight int
}

func NewScheduler(store datastore.Store) Scheduler {
	return &SchedulerImpl{
		store: store,
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
				plugin: plugins.NewLeastLatencyUsage(),
				weight: 1,
			},
		},
	}
}

func (s *SchedulerImpl) Schedule(req map[string]interface{}, pods []*datastore.PodInfo) (*datastore.PodInfo, error) {
	if len(pods) == 0 {
		return nil, fmt.Errorf("pods shouldn't be empty")
	}

	ctx := &framework.Context{
		Model: req["model"].(string),
	}

	pods, err := s.RunFilterPlugins(pods, ctx)
	if err != nil {
		return nil, err
	}

	scores, err := s.RunScorePlugins(pods, ctx)
	if err != nil {
		return nil, err
	}

	maxScore := math.MinInt
	best := pods[0]
	for pod, score := range scores {
		if score > maxScore {
			maxScore = score
			best = pod
		}
	}

	// TODO: return several best scorred pods to do fallback in case failure.

	return best, nil
}

func (s *SchedulerImpl) RunFilterPlugins(pods []*datastore.PodInfo, ctx *framework.Context) ([]*datastore.PodInfo, error) {
	for _, filterPlugin := range s.filterPlugins {
		pods = filterPlugin.Filter(pods, ctx)
		if len(pods) == 0 {
			return nil, fmt.Errorf("pods have all been filtered out by %q", filterPlugin.Name())
		}
	}

	return pods, nil
}

func (s *SchedulerImpl) RunScorePlugins(pods []*datastore.PodInfo, ctx *framework.Context) (map[*datastore.PodInfo]int, error) {
	res := make(map[*datastore.PodInfo]int)
	for _, scorePlugin := range s.scorePlugins {
		scores := scorePlugin.plugin.Score(pods, ctx)
		for k, v := range scores {
			if _, ok := res[k]; !ok {
				res[k] = v * scorePlugin.weight
			} else {
				res[k] += v * scorePlugin.weight
			}
		}
	}

	return res, nil
}
