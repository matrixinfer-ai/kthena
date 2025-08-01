package autoscaler

import (
	"context"
	"fmt"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"io"
	"istio.io/istio/pkg/util/sets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/algorithm"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/datastructure"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/histogram"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/util"
	inferControllerUtils "matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
	"net/http"
	"strings"
	"time"
)

type MetricCollector struct {
	PastHistograms  *datastructure.SnapshotSlidingWindow[map[string]HistogramInfo]
	Target          *v1alpha1.Target
	Scope           Scope
	WatchMetricList sets.String
}

func NewMetricCollector(target *v1alpha1.Target, binding *v1alpha1.AutoscalingPolicyBinding, metricTargets map[string]float64) *MetricCollector {
	return &MetricCollector{
		PastHistograms: datastructure.NewSnapshotSlidingWindow[map[string]HistogramInfo](util.SecondToTimestamp(util.SloQuantileSlidingWindowSeconds), util.SecondToTimestamp(util.SloQuantileDataKeepSeconds)),
		Target:         target,
		Scope: Scope{
			Namespace:      binding.Namespace,
			OwnedBindingId: binding.UID,
		},
		WatchMetricList: util.ExtractKeysToSet(metricTargets),
	}
}

type HistogramInfo struct {
	PodStartTime *metav1.Time
	HistogramMap map[string]*histogram.Snapshot
}

type Scope struct {
	Namespace      string
	OwnedBindingId types.UID
}

type InstanceInfo struct {
	IsReady    bool
	IsFailed   bool
	MetricsMap algorithm.Metrics
}

func (collector *MetricCollector) UpdateMetrics(ctx context.Context, kubeClient kubernetes.Interface) (unreadyInstancesCount int32, readyInstancesMetric algorithm.Metrics, err error) {
	// Get pod list which will be invoked api to get metrics
	unreadyInstancesCount = int32(0)
	err = nil
	podList, err := util.GetMetricPods(ctx, kubeClient, collector.Scope.Namespace, collector.Target.MetricFrom.MatchLabels)
	if err != nil {
		klog.Errorf("list modelInfer error: %v", err)
		return
	}
	if podList == nil || len(podList.Items) == 0 {
		klog.Errorf("pod list is null")
		return
	}

	currentHistograms := make(map[string]HistogramInfo)
	instanceInfo := collector.fetchMetricsFromPod(ctx, podList, &currentHistograms)
	klog.V(10).InfoS("finish to processInstance", "instanceInfo.isFailed", instanceInfo.IsFailed)
	klog.V(10).InfoS("finish to processInstance", "instanceInfo.isReady", instanceInfo.IsReady)
	klog.V(10).InfoS("finish to processInstance", "instanceInfo.metricsMap", instanceInfo.MetricsMap)
	if instanceInfo.IsFailed {
		klog.Warningf("some pod of %s are failed in namespace: %s.", collector.Scope, collector.Scope.Namespace)
		return
	}

	if !instanceInfo.IsReady {
		unreadyInstancesCount++
		klog.Warningf("some pod of %s are not ready in namespace: %s.", collector.Scope, collector.Scope.Namespace)
		return
	}
	readyInstancesMetric = instanceInfo.MetricsMap
	collector.PastHistograms.Append(currentHistograms)
	return
}

func (collector *MetricCollector) fetchMetricsFromPod(ctx context.Context, podList *corev1.PodList, currentHistograms *map[string]HistogramInfo) InstanceInfo {
	instanceInfo := InstanceInfo{true, false, make(algorithm.Metrics)}
	pastHistograms, ok := collector.PastHistograms.GetLastUnfreshSnapshot()
	if !ok {
		pastHistograms = make(map[string]HistogramInfo)
	}
	klog.InfoS("processInstance start")
	for _, pod := range podList.Items {
		instanceInfo.IsReady = instanceInfo.IsReady && inferControllerUtils.IsPodRunningAndReady(&pod)
		instanceInfo.IsFailed = instanceInfo.IsFailed || util.IsPodFailed(&pod) || inferControllerUtils.ContainerRestarted(&pod)

		pastValue, ok := pastHistograms[pod.Name]
		var pastHistogramMap map[string]*histogram.Snapshot
		if !ok || pod.Status.StartTime == nil || pastValue.PodStartTime == nil || !pod.Status.StartTime.Equal(pastValue.PodStartTime) {
			pastHistogramMap = make(map[string]*histogram.Snapshot)
		} else {
			pastHistogramMap = pastValue.HistogramMap
		}

		currentHistogramMap := make(map[string]*histogram.Snapshot)
		ip := pod.Status.PodIP
		podCtx, cancel := context.WithTimeout(ctx, util.AutoscaleCtxTimeoutSeconds*time.Second)
		defer cancel()
		url := fmt.Sprintf("http://%s:%d%s", ip, collector.Target.MetricFrom.Port, collector.Target.MetricFrom.Uri)

		req, _ := http.NewRequestWithContext(podCtx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			klog.Errorf("get metric response error: %v", err)
			continue
		}
		if resp == nil || util.IsRequestSuccess(resp.StatusCode) || resp.Body == nil {
			klog.Errorf("get metric response is invalid")
			continue
		}
		defer resp.Body.Close()

		bodyStr, err := io.ReadAll(resp.Body)
		if err != nil {
			klog.Errorf("get metrics read response error: %v", err)
			continue
		}
		result := string(bodyStr)
		collector.processPrometheusString(result, pastHistogramMap, currentHistogramMap, instanceInfo.MetricsMap)
		(*currentHistograms)[pod.Name] = HistogramInfo{
			PodStartTime: pod.Status.StartTime,
			HistogramMap: currentHistogramMap,
		}

	}
	return instanceInfo
}

func (collector *MetricCollector) processPrometheusString(metricStr string, pastHistograms map[string]*histogram.Snapshot, currentHistograms map[string]*histogram.Snapshot, instanceMetricMap algorithm.Metrics) {
	reader := strings.NewReader(metricStr)
	decoder := expfmt.NewDecoder(reader, expfmt.NewFormat(expfmt.TypeTextPlain))
	for {
		mf := &io_prometheus_client.MetricFamily{}
		err := decoder.Decode(mf)
		if err == io.EOF {
			break
		}
		if err != nil {
			klog.Errorf("error decoding metric: %v", err)
			continue
		}
		if len(mf.Metric) < 1 {
			klog.Errorf("metric is invalid")
			continue
		}

		if _, ok := collector.WatchMetricList[mf.GetName()]; !ok {
			klog.Errorf("metric name: %s is not matched with metricTargets", mf.GetName())
			continue
		}

		metric := mf.Metric[0]
		switch mf.GetType() {
		case io_prometheus_client.MetricType_COUNTER:
			addMetric(instanceMetricMap, mf.GetName(), metric.GetCounter().GetValue())
		case io_prometheus_client.MetricType_GAUGE:
			addMetric(instanceMetricMap, mf.GetName(), metric.GetGauge().GetValue())
		case io_prometheus_client.MetricType_HISTOGRAM:
			hist := metric.GetHistogram()
			snapshot := histogram.NewSnapshotOfHistogram(hist)
			currentHistograms[mf.GetName()] = snapshot

			if pastHistograms == nil {
				klog.Warning("pastHistograms is nil")
				continue
			}
			past, ok := pastHistograms[mf.GetName()]
			if !ok {
				past = histogram.NewDefaultSnapshot()
			}
			quantileInDiffMetric, err := histogram.QuantileInDiff(util.SloQuantilePercentile, snapshot, past)
			if err == nil {
				addMetric(instanceMetricMap, mf.GetName(), quantileInDiffMetric)
			}
		default:
			klog.InfoS("metric type is out of range", "type", mf.GetType())
		}
	}

	for key := range collector.WatchMetricList {
		if _, ok := instanceMetricMap[key]; !ok {
			instanceMetricMap[key] = 0
		}
	}
}

func addMetric(instanceMetricMap algorithm.Metrics, metricName string, metricValue float64) {
	if oldValue, ok := instanceMetricMap[metricName]; ok {
		instanceMetricMap[metricName] = oldValue + metricValue
	} else {
		instanceMetricMap[metricName] = metricValue
	}
}
