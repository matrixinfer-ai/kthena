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

func GetCouterAndGaugePodMetrics(allMetrics map[string]*dto.MetricFamily, counterAndGaugeMetrics []string) map[string]float64 {
	wantMetrics := make(map[string]float64)
	for _, metricName := range counterAndGaugeMetrics {
		metricInfo, exist := allMetrics[metricName]
		if !exist {
			continue
		}
		for _, metric := range metricInfo.Metric {
			metricValue := metric.GetCounter().GetValue()
			wantMetrics[metricName] = metricValue
		}
	}

	return wantMetrics
}

func GetHistogramPodMetrics(allMetrics map[string]*dto.MetricFamily, HistogramMetrics []string) map[string]float64 {
	wantMetrics := make(map[string]float64)
	for _, metricName := range HistogramMetrics {
		metricInfo, exist := allMetrics[metricName]
		if !exist {
			continue
		}
		for _, metric := range metricInfo.Metric {
			metricValue := metric.GetHistogram()
			aveValue := aveHistogram(metricValue)
			wantMetrics[metricName] = aveValue
		}
	}

	return wantMetrics
}

func aveHistogram(metricValue *dto.Histogram) float64 {
	sum := metricValue.GetSampleSum()
	count := metricValue.GetSampleCount()
	return sum / float64(count)
}
