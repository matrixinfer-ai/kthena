package sglang

import (
	"fmt"

	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/metrics"
)

const (
	// The address of sglang's query metrics is http://localhost:30000/metrics
	MetricPort = 30000
)

var (
	GPUCacheUsage     = "sglang:token_usage"
	RequestWaitingNum = "sglang:num_queue_reqs"
	TPOT              = "sglang:time_per_output_token_seconds"
	TTFT              = "sglang:time_to_first_token_seconds"

	CounterAndGaugeMetrics = []string{
		"sglang:token_usage",
		"sglang:num_queue_reqs",
	}

	HistogramMetrics = []string{
		"sglang:time_per_output_token_seconds",
		"sglang:time_to_first_token_seconds",
	}
)

func GetSglangPodMetrics(pod *corev1.Pod) (map[string]*dto.MetricFamily, error) {
	url := fmt.Sprintf("http://%s:%d/metrics", pod.Status.PodIP, MetricPort)
	allMetrics, err := metrics.ParseMetricsURL(url)
	if err != nil {
		return nil, err
	}

	return allMetrics, nil
}
