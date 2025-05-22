package scheduler
package plugins

import (
	"math"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)

var _ framework.ScorePlugin = &LeastLatencyUsage{}

const MaxScore = 100
const LeastLatencyPluginName = "least latency"

type LeastLatencyUsage struct {
	name        string
	maxTTFTTPOT float64
	minTTFTTPOT float64
}

func NewLeastLatencyUsage() *LeastLatencyUsage {
	return &LeastLatencyUsage{
		name:        LeastLatencyPluginName,
		maxTTFTTPOT: 0,
		minTTFTTPOT: math.MaxFloat64,
	}
}

func (l *LeastLatencyUsage) Name() string {
	return l.name
}
func (l *LeastLatencyUsage) Score(pods []*datastore.PodInfo, ctx *framework.Context) map[*datastore.PodInfo]int {
	scoreResults := make(map[*datastore.PodInfo]int)
	l.maxTTFTTPOT = 0
	l.minTTFTTPOT = math.MaxFloat64
	for _, info := range pods {
		sum := info.TTFT + info.TPOT
		if l.maxTTFTTPOT < sum {
			l.maxTTFTTPOT = sum
		}
		if l.minTTFTTPOT > sum {
			l.minTTFTTPOT = sum
		}
	}
	for _, info := range pods {
		sum := info.TTFT + info.TPOT
		score := MaxScore
		if l.maxTTFTTPOT > l.minTTFTTPOT {
			score = int(MaxScore * (l.maxTTFTTPOT - sum) / (l.maxTTFTTPOT - l.minTTFTTPOT))
		}
		scoreResults[info] = score
	}

	return scoreResults
}
