package metrics

import (
	"fmt"
	"net/http"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// This function refer to aibrix(https://github.com/vllm-project/aibrix/blob/main/pkg/metrics/utils.go)
func ParseMetricsURL(url string) (map[string]*dto.MetricFamily, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch metrics from %s: %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v", err)
		}
	}()

	var parser expfmt.TextParser
	allMetrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error parsing metric families: %v\n", err)
	}
	return allMetrics, nil
}

func LastPeriodAvg(previous, current *dto.Histogram) float64 {
	previousSum := previous.GetSampleSum()
	previousCount := previous.GetSampleCount()

	currentSum := current.GetSampleSum()
	currentCount := current.GetSampleCount()

	deltaSum := currentSum - previousSum
	deltaCount := currentCount - previousCount

	if deltaCount == 0 {
		// If no new access records have been generated in a period of time, directly return zero.
		// When updating MetricsInfo, the last value is preserved for values of 0.
		return previousSum / float64(previousCount)
	}

	return deltaSum / float64(deltaCount)
}
