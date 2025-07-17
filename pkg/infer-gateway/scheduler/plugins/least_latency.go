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

package plugins

import (
	"math"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

var _ framework.ScorePlugin = &LeastLatency{}

const LeastLatencyPluginName = "least-latency"

// MaxScore is the highest possible score a pod can receive
const MaxScore = 100.0

type LeastLatency struct {
	name                 string
	TTFTTPOTWeightFactor float64
}

type LeastLatencyArgs struct {
	TTFTTPOTWeightFactor float64 `yaml:"TTFTTPOTWeightFactor,omitempty"`
}

func NewLeastLatency(arg map[string]interface{}) *LeastLatency {
	var leastLatencyArgs LeastLatencyArgs
	utils.ParsePluginArgs(LeastLatencyPluginName, arg, &leastLatencyArgs)

	return &LeastLatency{
		name:                 LeastLatencyPluginName,
		TTFTTPOTWeightFactor: leastLatencyArgs.TTFTTPOTWeightFactor,
	}
}

func (l *LeastLatency) Name() string {
	return l.name
}

// Score calculates a score for each pod based on their inference latency:
func (l *LeastLatency) Score(ctx *framework.Context, pods []*datastore.PodInfo) map[*datastore.PodInfo]int {
	// Stores the computed score for each pod
	scoreResults := make(map[*datastore.PodInfo]int)
	// Handle edge case: empty pod list
	if len(pods) == 0 {
		return scoreResults
	}
	// 1. First pass: Determine the minimum and maximum latency values
	// Initialize with extreme values to ensure any valid latency updates them
	// ctx.MaxToken is the max token that the model is allowed to generate in its response.
	// Calculate min/max values for TTFT and TPOT in calculateMinMaxMetrics
	minTTFT, maxTTFT, minTPOT, maxTPOT := calculateMinMaxMetrics(pods)
	// 2. Second pass: Compute scores using linear normalization
	// Note: If all pods have identical latency (max == min), all pods get MaxScore
	for _, info := range pods {
		scoreTTFT := MaxScore
		scoreTPOT := MaxScore
		// Only compute normalized score if there's variance in latency values
		if maxTTFT > minTTFT {
			scoreTTFT = MaxScore * (maxTTFT - info.TTFT) / (maxTTFT - minTTFT)
		}
		if maxTPOT > minTPOT {
			scoreTPOT = MaxScore * (maxTPOT - info.TPOT) / (maxTPOT - minTPOT)
		}
		scoreResults[info] = int(scoreTTFT*l.TTFTTPOTWeightFactor + scoreTPOT*(1-l.TTFTTPOTWeightFactor))
	}

	return scoreResults
}

func calculateMinMaxMetrics(pods []*datastore.PodInfo) (minTTFT, maxTTFT, minTPOT, maxTPOT float64) {
	minTTFT = math.MaxFloat64
	maxTTFT = 0.0
	minTPOT = math.MaxFloat64
	maxTPOT = 0.0

	for _, info := range pods {
		// Skip pods with invalid values
		if info.TTFT < 0 || info.TPOT < 0 {
			continue
		}

		// Update TTFT min/max
		if info.TTFT < minTTFT {
			minTTFT = info.TTFT
		}
		if info.TTFT > maxTTFT {
			maxTTFT = info.TTFT
		}

		// Update TPOT min/max
		if info.TPOT < minTPOT {
			minTPOT = info.TPOT
		}
		if info.TPOT > maxTPOT {
			maxTPOT = info.TPOT
		}
	}

	return minTTFT, maxTTFT, minTPOT, maxTPOT
}
