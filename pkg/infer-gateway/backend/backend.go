package backend

import (
	"fmt"

	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend/sglang"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/backend/vllm"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
)

var (
	log = logger.NewLogger("backend")
)

type MetricsProvider interface {
	GetPodMetrics(pod *corev1.Pod) (map[string]*dto.MetricFamily, error)
	GetCountMetricsInfo(allMetrics map[string]*dto.MetricFamily) map[string]float64
	GetHistogramPodMetrics(allMetrics map[string]*dto.MetricFamily, previousHistogram map[string]*dto.Histogram) (map[string]float64, map[string]*dto.Histogram)
}

var engineRegistry = map[string]MetricsProvider{
	"SGLang": sglang.NewSglangEngine(),
	"vLLM":   vllm.NewVllmEngine(),
}

func GetPodMetrics(engine string, pod *corev1.Pod, previousHistogram map[string]*dto.Histogram) (map[string]float64, map[string]*dto.Histogram) {
	provider, err := GetMetricsProvider(engine)
	if err != nil {
		log.Errorf("Failed to get inference engine: %v", err)
		return nil, nil
	}

	allMetrics, err := provider.GetPodMetrics(pod)
	if err != nil {
		log.Errorf("failed to get metrics of pod: %s/%s: %v", pod.GetNamespace(), pod.GetName(), err)
		return nil, nil
	}

	countMetricsInfo := provider.GetCountMetricsInfo(allMetrics)
	histogramMetricsInfo, histogramMetrics := provider.GetHistogramPodMetrics(allMetrics, previousHistogram)

	for name, value := range histogramMetricsInfo {
		// Since the key in countMetricInfo must not be the same as the key in histogramMetricsInfo.
		// You don't have to worry about overriding the value
		countMetricsInfo[name] = value
	}

	return countMetricsInfo, histogramMetrics
}

func GetMetricsProvider(engine string) (MetricsProvider, error) {
	if provider, exists := engineRegistry[engine]; exists {
		return provider, nil
	}
	return nil, fmt.Errorf("unsupported engine: %s", engine)
}
