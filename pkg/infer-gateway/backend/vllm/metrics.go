package vllm

import (
	"fmt"

	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/metrics"
)

const (
	// The address of vllm's query metrics is http://localhost:8000/metrics
	MetricPort = 8000
)

var (
	GPUCacheUsage     = "vllm:vllm:gpu_cache_usage_perc"
	RequestWaitingNum = "vllm:num_request_waiting"
	TPOT              = "vllm:time_per_output_token_seconds"
	TTFT              = "vllm:time_to_first_token_seconds"

	CounterAndGaugeMetrics = []string{
		"vllm:vllm:gpu_cache_usage_perc",
		"vllm:num_request_waiting",
	}

	HistogramMetrics = []string{
		"vllm:time_per_output_token_seconds",
		"vllm:time_to_first_token_seconds",
	}
)

func GetVllmPodMetrics(pod *corev1.Pod) (map[string]*dto.MetricFamily, error) {
	url := fmt.Sprintf("http://%s:%d/metrics", pod.Status.PodIP, MetricPort)
	allMetrics, err := metrics.ParseMetricsURL(url)
	if err != nil {
		return nil, err
	}

	return allMetrics, nil
}
