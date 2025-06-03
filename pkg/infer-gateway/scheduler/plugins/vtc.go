package plugins
import (
	"math"
	"math/rand"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/plugins/vtc"
)


const (
	defaultMaxPodLoad        = 100.0
	defaultFairnessWeight    = 1.0
	defaultUtilizationWeight = 1.0
)



var (
	maxPodLoad        = defaultMaxPodLoad
	fairnessWeight    = defaultFairnessWeight
	utilizationWeight = defaultUtilizationWeight
)


var _ framework.ScorePlugin = &BasicVTCRouter{}

const BasicVTCRouterPluginName = "Basic VTC Router"





// BasicVTCRouter implements the VTC routing algorithm
type BasicVTCRouter struct {
	name string
	tokenTracker   vtc.TokenTracker
	tokenEstimator vtc.TokenEstimator
}


func NewBasicVTCRouter() *BasicVTCRouter {
	return &BasicVTCRouter{
		name: BasicVTCRouterPluginName,
		tokenEstimator: vtc.NewSimpleTokenEstimator(),
		tokenTracker:   vtc.NewInMemorySlidingWindowTokenTracker(),
	}
}

func (v *BasicVTCRouter) Name() string {
	return v.name
}
func (v *BasicVTCRouter) Score(pods []*datastore.PodInfo, ctx *framework.Context) map[*datastore.PodInfo]int {
	// Stores the computed score for each pod
	scoreResults := make(map[*datastore.PodInfo]int)
	// Handle edge case: empty pod list
	if len(pods) == 0 {
		return scoreResults
	}
	user := ctx.User
	if user == nil {
		return scoreResults
	}
	inputTokens := v.tokenEstimator.EstimateInputTokens(ctx.Message)
	outputTokens := v.tokenEstimator.EstimateOutputTokens(ctx.Message)

	userTokens, err := v.tokenTracker.GetTokenCount(*user)
	minTokens, err := v.tokenTracker.GetMinTokenCount()
	if err != nil {
		minTokens = vtc.TokenTrackerMinTokens // Use the configured default minimum token count
	}

	maxTokens, err := v.tokenTracker.GetMaxTokenCount()
	if err != nil {
		maxTokens = vtc.TokenTrackerMaxTokens // Use the configured default maximum token count
	}
	for i, info := range pods {

		// 1. Dynamically calculate a reasonable "step size" for mapping user tokens onto pod indices, ensuring the mapping is
		// relevant to the current system load while maintaining a minimum sensitivity
		adaptiveBucketSize := math.Max(vtc.TokenTrackerMinTokens, (minTokens+maxTokens)/2)

		// Apply clamped linear mapping: tokens / bucket_size, clamped to [0, npods-1]
		normalizedTokens := math.Min(float64(userTokens)/adaptiveBucketSize, float64(len(pods)-1))

		fairnessScore := math.Abs(float64(i) - normalizedTokens)


		// 2. Get pod load for utilization score

		var podLoad float64 = float64(info.RequestRunningNum)

		// 3. Calculate utilization score (normalized between 0-1)
		utilizationScore := min(podLoad/maxPodLoad, 1.0)

		// 4. Add a small random factor to break ties and improve distribution
		randomFactor := rand.Float64() * 0.1

		// 5. Calculate combined score (lower is better) - using configurable weights for fairness and utilization
		score := (fairnessWeight * fairnessScore) + (utilizationWeight * utilizationScore) + randomFactor
		scoreResults[info] = int(score)
		if *user != "" {
		v.tokenTracker.UpdateTokenCount( *user, inputTokens, outputTokens)
	}
	}
	return scoreResults
}


