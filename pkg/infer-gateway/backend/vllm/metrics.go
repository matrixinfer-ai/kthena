package vllm

import (
	"fmt"

	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend/metrics"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

var (
	GPUCacheUsage     = "vllm:gpu_cache_usage_perc"
	RequestWaitingNum = "vllm:num_requests_waiting"
	TPOT              = "vllm:time_per_output_token_seconds"
	TTFT              = "vllm:time_to_first_token_seconds"
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

type vllmEngine struct {
	// The address of vllm's query metrics is http://{model server}:MetricPort/metrics
	// Default is 8000
	MetricPort uint32
}

func NewVllmEngine() *vllmEngine {
	// TODO: Get MetricsPort from vllm configuration
	return &vllmEngine{
		MetricPort: 8000,
	}
}

func (engine *vllmEngine) GetPodMetrics(pod *corev1.Pod) (map[string]*dto.MetricFamily, error) {
	url := fmt.Sprintf("http://%s:%d/metrics", pod.Status.PodIP, engine.MetricPort)
	allMetrics, err := metrics.ParseMetricsURL(url)
	if err != nil {
		return nil, err
	}

	return allMetrics, nil
}

func (engine *vllmEngine) GetCountMetricsInfo(allMetrics map[string]*dto.MetricFamily) map[string]float64 {
	wantMetrics := make(map[string]float64)
	for _, metricName := range CounterAndGaugeMetrics {
		metricInfo, exist := allMetrics[metricName]
		if !exist {
			continue
		}
		for _, metric := range metricInfo.Metric {
			metricValue := metric.GetGauge().GetValue()
			wantMetrics[mapOfMetricsName[metricName]] = metricValue
		}
	}

	return wantMetrics
}

func (engine *vllmEngine) GetHistogramPodMetrics(allMetrics map[string]*dto.MetricFamily, previousHistogram map[string]*dto.Histogram) (map[string]float64, map[string]*dto.Histogram) {
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
