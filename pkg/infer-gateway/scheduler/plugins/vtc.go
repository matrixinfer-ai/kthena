package plugins
package vtc
import (
	"fmt"
	"math"
	"math/rand"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler/framework"
)


const (
	defaultMaxPodLoad        = 100.0
	defaultInputTokenWeight  = 1.0
	defaultOutputTokenWeight = 2.0
	defaultFairnessWeight    = 1.0
	defaultUtilizationWeight = 1.0
)



var (
	maxPodLoad        = defaultMaxPodLoad
	inputTokenWeight  = defaultInputTokenWeight
	outputTokenWeight = defaultOutputTokenWeight
	fairnessWeight    = defaultFairnessWeight
	utilizationWeight = defaultUtilizationWeight
)


var _ framework.ScorePlugin = &BasicVTCRouter{}

const BasicVTCRouterPluginName = "Basic VTC Router"

// TokenTracker tracks token usage per user
type TokenTracker interface {
	GetTokenCount(ctx context.Context, user string) (float64, error)

	UpdateTokenCount(ctx context.Context, user string, inputTokens, outputTokens float64) error

	GetMinTokenCount(ctx context.Context) (float64, error)

	GetMaxTokenCount(ctx context.Context) (float64, error)
}

// TokenEstimator estimates token counts for messages
type TokenEstimator interface {
	EstimateInputTokens(message string) float64

	EstimateOutputTokens(message string) float64
}

// BasicVTCRouter implements the VTC routing algorithm
type BasicVTCRouter struct {
	name string
	tokenTracker   TokenTracker
	tokenEstimator TokenEstimator
}


func NewBasicVTCRouter() *BasicVTCRouter {
	return &BasicVTCRouter{
		name: BasicVTCRouterPluginName,
		tokenEstimator: NewSimpleTokenEstimator(),
		tokenTracker:   NewInMemorySlidingWindowTokenTracker(),
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
	inputTokens := v.tokenEstimator.EstimateInputTokens(ctx.Message)
	outputTokens := v.tokenEstimator.EstimateOutputTokens(ctx.Message)

	userTokens, err := v.tokenTracker.GetTokenCount(*user)
	if err != nil {
		klog.ErrorS(err, "failed to get user token count, falling back to zero", "user", *user)
		userTokens = 0
	}
	var minScore float64 = math.MaxFloat64
	minTokens, err := v.tokenTracker.GetMinTokenCount()
	if err != nil {
		klog.ErrorS(err, "failed to get minimum token count, using default value")
		minTokens = tokenTrackerMinTokens // Use the configured default minimum token count
	}

	maxTokens, err := v.tokenTracker.GetMaxTokenCount()
	if err != nil {
		klog.ErrorS(err, "failed to get maximum token count, using default value")
		maxTokens = tokenTrackerMaxTokens // Use the configured default maximum token count
	}
	for i, info := range pods {

		// 1. Dynamically calculate a reasonable "step size" for mapping user tokens onto pod indices, ensuring the mapping is
		// relevant to the current system load while maintaining a minimum sensitivity
		adaptiveBucketSize := math.Max(tokenTrackerMinTokens, (minTokens+maxTokens)/2)

		/*metrics.SetGaugeMetric(
			metrics.VTCBucketSizeActive,
			metrics.GetMetricHelp(metrics.VTCBucketSizeActive),
			adaptiveBucketSize,
			[]string{"pod", "model"},
			pod.Name, ctx.Model,
		)*/

		// Apply clamped linear mapping: tokens / bucket_size, clamped to [0, npods-1]
		normalizedTokens := math.Min(float64(userTokens)/adaptiveBucketSize, float64(len(readyPods)-1))

		fairnessScore := math.Abs(float64(i) - normalizedTokens)


		// 2. Get pod load for utilization score
		var podLoad float64
		reqCount = info.RequestRunningNum 

		// 3. Calculate utilization score (normalized between 0-1)
		utilizationScore := min(podLoad/maxPodLoad, 1.0)

		// 4. Add a small random factor to break ties and improve distribution
		randomFactor := rand.Float64() * 0.1

		// 5. Calculate combined score (lower is better) - using configurable weights for fairness and utilization
		score := (fairnessWeight * fairnessScore) + (utilizationWeight * utilizationScore) + randomFactor

		scoreResults[info] = int(score)
	}
	return scoreResults
}


func (v *BasicVTCRouter) SubscribedMetrics() []string {
	return []string{
		metrics.NumRequestsRunning,
		metrics.VTCBucketSizeActive,
	}
}


