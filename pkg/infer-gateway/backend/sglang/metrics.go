package sglang

import (
	"fmt"

	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend/metrics"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

var (
	GPUCacheUsage     = "sglang:token_usage"
	RequestWaitingNum = "sglang:num_queue_reqs"
	TPOT              = "sglang:time_per_output_token_seconds"
	TTFT              = "sglang:time_to_first_token_seconds"
)

var (
	CounterAndGaugeMetrics = []string{
		GPUCacheUsage,
		RequestWaitingNum,
	}

	HistogramMetrics = []string{
		TPOT,
		TTFT,
	}

	mapOfMetricsName = map[string]string{
		GPUCacheUsage:     utils.GPUCacheUsage,
		RequestWaitingNum: utils.RequestWaitingNum,
		TPOT:              utils.TPOT,
		TTFT:              utils.TTFT,
	}
)

type sglangEngine struct {
	// The address of sglang's query metrics is http://{model server}:MetricPort/metrics
	// Default is 30000
	MetricPort uint32
}

func NewSglangEngine() *sglangEngine {
	// TODO: Get MetricsPort from sglang configuration
	return &sglangEngine{
		MetricPort: 30000,
	}
}

func (engine *sglangEngine) GetPodMetrics(pod *corev1.Pod) (map[string]*dto.MetricFamily, error) {
	url := fmt.Sprintf("http://%s:%d/metrics", pod.Status.PodIP, engine.MetricPort)
	allMetrics, err := metrics.ParseMetricsURL(url)
	if err != nil {
		return nil, err
	}

	return allMetrics, nil
}

func (engine *sglangEngine) GetCountMetricsInfo(allMetrics map[string]*dto.MetricFamily) map[string]float64 {
	wantMetrics := make(map[string]float64)
	for _, metricName := range CounterAndGaugeMetrics {
		metricInfo, exist := allMetrics[metricName]
		if !exist {
			continue
		}
		for _, metric := range metricInfo.Metric {
			metricValue := metric.GetCounter().GetValue()
			wantMetrics[mapOfMetricsName[metricName]] = metricValue
		}
	}

	return wantMetrics
}

func (engine *sglangEngine) GetHistogramPodMetrics(allMetrics map[string]*dto.MetricFamily, previousHistogram map[string]*dto.Histogram) (map[string]float64, map[string]*dto.Histogram) {
	wantMetrics := make(map[string]float64)
	histogramMetrics := make(map[string]*dto.Histogram)
	for _, metricName := range HistogramMetrics {
		metricInfo, exist := allMetrics[metricName]
		if !exist {
			continue
		}
		for _, metric := range metricInfo.Metric {
			metricValue := metric.GetHistogram()
			histogramMetrics[mapOfMetricsName[metricName]] = metricValue
			previousMetric := previousHistogram[mapOfMetricsName[metricName]]
			if previousMetric == nil {
				currentSum := metricValue.GetSampleSum()
				currentCount := metricValue.GetSampleCount()
				wantMetrics[mapOfMetricsName[metricName]] = currentSum / float64(currentCount)
			} else {
				wantMetrics[mapOfMetricsName[metricName]] = metrics.LastPeriodAvg(previousMetric, metricValue)
			}
		}
	}

	return wantMetrics, histogramMetrics
}
