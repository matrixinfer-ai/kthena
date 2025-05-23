package scheduler
package plugins

import (
	"math"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

var _ framework.ScorePlugin = &LeastLatencyUsage{}
// MaxScore is the highest possible score a pod can receive
const MaxScore = 100
const LeastLatencyPluginName = "least latency"

type LeastLatencyUsage struct {
	name        string
}

func NewLeastLatencyUsage() *LeastLatencyUsage {
	return &LeastLatencyUsage{
		name:        LeastLatencyPluginName,
	}
}

func (l *LeastLatencyUsage) Name() string {
	return l.name
}
// Score calculates a score for each pod based on their inference latency:
func (l *LeastLatencyUsage) Score(pods []*datastore.PodInfo, ctx *framework.Context) map[*datastore.PodInfo]int {
	// Stores the computed score for each pod
	scoreResults := make(map[*datastore.PodInfo]int)
	// Handle edge case: empty pod list
	if len(pods) == 0 {
		return scoreResults
	}
	// 1. First pass: Determine the minimum and maximum latency values
	// Initialize with extreme values to ensure any valid latency updates them
	// ctx.MaxToken is the max token that the model is allowed to generate in its response. 
	var maxTotalLatency = 0.0
	var minTotalLatency = math.MaxFloat64
	for _, info := range pods {
		sum := info.TTFT + info.TPOT * float64(ctx.MaxToken)
		if maxTotalLatency < sum {
			maxTotalLatency = sum
		}
		if minTotalLatency > sum {
			minTotalLatency = sum
		}
	}
	// 2. Second pass: Compute scores using linear normalization
	// Note: If all pods have identical latency (max == min), all pods get MaxScore
	for _, info := range pods {
		sum := info.TTFT + info.TPOT * float64(ctx.MaxToken)
		score := MaxScore
		// Only compute normalized score if there's variance in latency values
		if maxTotalLatency > minTotalLatency {
			score = int(MaxScore * (maxTotalLatency - sum) / (maxTotalLatency - minTotalLatency))
		}
		scoreResults[info] = score
	}

	return scoreResults
}
