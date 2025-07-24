/*
Copyright MatrixInfer-AI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"istio.io/istio/pkg/util/sets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	registryLister "matrixinfer.ai/matrixinfer/client-go/listers/registry/v1alpha1"
	workloadLister "matrixinfer.ai/matrixinfer/client-go/listers/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/algorithm"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/histogram"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/util"
	inferControllerUtils "matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
)

type AutoscaleController struct {
	// Client for k8s. Use it to call K8S API
	kubeClient kubernetes.Interface
	// client for custom resource
	client                      clientset.Interface
	namespace                   string
	autoscalingPoliciesLister   registryLister.AutoscalingPolicyLister
	autoscalingPoliciesInformer cache.Controller
	modelsLister                registryLister.ModelLister
	modelsInformer              cache.Controller
	modelInfersLister           workloadLister.ModelInferLister
	modelInfersInformer         cache.Controller
	podsLister                  listerv1.PodLister
	podsInformer                cache.Controller
	autoscalerMap               map[string]*autoscaler.Autoscaler
}

func NewAutoscaleController(kubeClient kubernetes.Interface, client clientset.Interface, namespace string) *AutoscaleController {
	informerFactory := informersv1alpha1.NewSharedInformerFactory(client, 0)
	modelInformer := informerFactory.Registry().V1alpha1().Models()
	modelInferInformer := informerFactory.Workload().V1alpha1().ModelInfers()
	autoscalingPoliciesInformer := informerFactory.Registry().V1alpha1().AutoscalingPolicies()

	req, err := labels.NewRequirement(workload.GroupNameLabelKey, selection.Exists, nil)
	if err != nil {
		klog.Errorf("can not create label selector,err:%v", err)
		return nil
	}
	selector := labels.NewSelector().Add(*req)
	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector.String()
		}),
	)
	podsInformer := kubeInformerFactory.Core().V1().Pods()
	ac := &AutoscaleController{
		kubeClient:                  kubeClient,
		namespace:                   namespace,
		client:                      client,
		autoscalingPoliciesLister:   autoscalingPoliciesInformer.Lister(),
		autoscalingPoliciesInformer: autoscalingPoliciesInformer.Informer(),
		modelsLister:                modelInformer.Lister(),
		modelsInformer:              modelInformer.Informer(),
		modelInfersLister:           modelInferInformer.Lister(),
		modelInfersInformer:         modelInferInformer.Informer(),
		podsLister:                  podsInformer.Lister(),
		podsInformer:                podsInformer.Informer(),
		autoscalerMap:               make(map[string]*autoscaler.Autoscaler),
	}
	return ac
}

func (ac *AutoscaleController) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()

	// start informers
	go ac.autoscalingPoliciesInformer.RunWithContext(ctx)
	go ac.modelsInformer.RunWithContext(ctx)
	go ac.modelInfersInformer.RunWithContext(ctx)
	go ac.podsInformer.RunWithContext(ctx)
	cache.WaitForCacheSync(ctx.Done(),
		ac.autoscalingPoliciesInformer.HasSynced,
		ac.modelsInformer.HasSynced,
		ac.modelInfersInformer.HasSynced,
		ac.podsInformer.HasSynced,
	)

	klog.Info("start autoscale controller")
	go wait.Until(func() {
		ac.Reconcile(ctx)
	}, util.AutoscalingSyncPeriodSeconds*time.Second, nil)

	<-ctx.Done()
	klog.Info("shut down autoscale controller")
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (ac *AutoscaleController) Reconcile(ctx context.Context) {
	klog.InfoS("start to reconcile")
	modelCtx, cancel := context.WithTimeout(ctx, util.AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	modelList, err := ac.client.RegistryV1alpha1().Models(ac.namespace).List(modelCtx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("failed to list model, err:%v", err)
		return
	}

	set := sets.New[string]()
	for _, model := range modelList.Items {
		autoscalingPolicyName := model.Spec.AutoscalingPolicyRef.Name
		klog.InfoS("global", "autoscalingPolicyName", autoscalingPolicyName)
		if autoscalingPolicyName == "" {
			backends := model.Spec.Backends
			for _, backend := range backends {
				autoscalingPolicyName = backend.AutoscalingPolicyRef.Name
				klog.InfoS("backend", "autoscalingPolicyName", autoscalingPolicyName)
				if autoscalingPolicyName == "" {
					continue
				}

				autoscalerMapKey := formatAutoscalerMapKey(model.Name, backend.Name)
				set.Insert(autoscalerMapKey)
			}
		} else {
			set.Insert(formatAutoscalerMapKey(model.Name, ""))
		}
	}

	for key := range ac.autoscalerMap {
		if !set.Contains(key) {
			delete(ac.autoscalerMap, key)
		}
	}

	klog.InfoS("start to process autoscale")
	for _, model := range modelList.Items {
		err := ac.processAutoscale(ctx, model, ac.namespace)
		if err != nil {
			klog.Errorf("failed to process autoscale,err:%v", err)
			continue
		}
	}
}

type AutoscaleScope struct {
	modelId     types.UID
	modelName   string
	backendName string
}

func (ac *AutoscaleController) processAutoscale(ctx context.Context, model v1alpha1.Model, namespace string) error {
	klog.InfoS("processAutoscale enter")
	autoscalingPolicyRef := model.Spec.AutoscalingPolicyRef
	if autoscalingPolicyRef.Name == "" {
		backends := model.Spec.Backends
		for _, backend := range backends {
			autoscalingPolicyName := backend.AutoscalingPolicyRef.Name
			if autoscalingPolicyName == "" {
				continue
			}

			autoscalePolicy, err := ac.getAutoscalePolicy(autoscalingPolicyName, namespace)
			if err != nil {
				klog.Errorf("get autoscale policy error: %v", err)
				continue
			}

			autoscaleScope := AutoscaleScope{model.UID, model.Name, backend.Name}
			backendKey := formatAutoscalerMapKey(model.Name, backend.Name)
			// init autoscalerMap
			backendAutoscaler, ok := ac.autoscalerMap[backendKey]
			if !ok {
				metricTargets := getMetricTargets(autoscalePolicy)
				globalInfo := autoscaler.NewGlobalInfo([]v1alpha1.ModelBackend{backend}, backend.Cost)
				backendAutoscaler = autoscaler.NewAutoscaler(&autoscalePolicy.Spec.Behavior, globalInfo, metricTargets)
				ac.autoscalerMap[backendKey] = backendAutoscaler
			}
			if correctedInstances, skip := ac.doAutoscale(ctx, namespace, autoscaleScope, autoscalePolicy, backendAutoscaler); !skip {
				klog.InfoS("update modelInfer replicas", "correctedInstances", correctedInstances)
				modelInferList, err := ac.listModelInferByLabel(ctx, namespace, backend.Name, model.UID)
				if err != nil {
					klog.Errorf("failed to list modelInfer by backendName: %s, error: %v", backend.Name, err)
					return err
				}
				klog.InfoS("start to update")
				for _, modelInfer := range modelInferList.Items {
					klog.InfoS("finally modelInfer", "modelInfer", modelInfer)
					if modelInfer.Spec.Replicas == nil || *modelInfer.Spec.Replicas == correctedInstances {
						klog.Warning("modelInfer replicas no need to update")
						continue
					}
					*modelInfer.Spec.Replicas = correctedInstances
					err := ac.updateModelInfer(ctx, &modelInfer)
					if err != nil {
						klog.Errorf("failed to update modelInfer replicas for modelInfer.Name: %s, error: %v", modelInfer.Name, err)
						return err
					}
				}
			}
		}
	} else {
		autoscalePolicy, err := ac.getAutoscalePolicy(autoscalingPolicyRef.Name, namespace)
		if err != nil {
			klog.Errorf("get autoscale policy error: %v", err)
			return err
		}

		autoscaleScope := AutoscaleScope{model.UID, model.Name, ""}
		backendKey := formatAutoscalerMapKey(model.Name, "")
		globalAutoscaler, ok := ac.autoscalerMap[backendKey]
		if !ok {
			globalInfo := autoscaler.NewGlobalInfo(model.Spec.Backends, *model.Spec.CostExpansionRatePercent)
			metricTargets := getMetricTargets(autoscalePolicy)
			globalAutoscaler = autoscaler.NewAutoscaler(&autoscalePolicy.Spec.Behavior, globalInfo, metricTargets)
			ac.autoscalerMap[backendKey] = globalAutoscaler
		}
		if correctedInstances, skip := ac.doAutoscale(ctx, namespace, autoscaleScope, autoscalePolicy, globalAutoscaler); !skip {
			// update modelInfer replicas
			replicasMap := globalAutoscaler.GlobalInfo.RestoreReplicasOfEachBackend(correctedInstances)
			for key, value := range replicasMap {
				modelInferList, err := ac.listModelInferByLabel(ctx, namespace, key, model.UID)
				if err != nil {
					klog.Errorf("failed to list modelInfer by backendName: %s, error: %v", key, err)
					return err
				}

				klog.InfoS("start to update")
				for _, modelInfer := range modelInferList.Items {
					modelInferCopy := modelInfer.DeepCopy()
					if *modelInferCopy.Spec.Replicas == value {
						klog.Warning("modelInfer replicas no need to update")
						continue
					}

					modelInferCopy.Spec.Replicas = ptr.To(value)
					err := ac.updateModelInfer(ctx, modelInferCopy)
					if err != nil {
						klog.Errorf("failed to update modelInfer replicas for modelInfer.Name: %s, error: %v", modelInfer.Name, err)
						return err
					}
				}
			}
		}
	}

	klog.InfoS("processAutoscale end")
	return nil
}

func (ac *AutoscaleController) getAutoscalePolicy(autoscalingPolicyName string, namespace string) (*v1alpha1.AutoscalingPolicy, error) {
	autoscalingPolicy, err := ac.autoscalingPoliciesLister.AutoscalingPolicies(namespace).Get(autoscalingPolicyName)
	if err != nil {
		klog.Errorf("can not get autosalingpolicyname: %s, error: %v", autoscalingPolicyName, err)
		return nil, client.IgnoreNotFound(err)
	}
	return autoscalingPolicy, nil
}

func (ac *AutoscaleController) updateModelInfer(ctx context.Context, modelInfer *workload.ModelInfer) error {
	modelInferCtx, cancel := context.WithTimeout(ctx, util.AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	if oldModelInfer, err := ac.client.WorkloadV1alpha1().ModelInfers(modelInfer.Namespace).Get(modelInferCtx, modelInfer.Name, metav1.GetOptions{}); err == nil {
		modelInfer.ResourceVersion = oldModelInfer.ResourceVersion
		if _, updateErr := ac.client.WorkloadV1alpha1().ModelInfers(modelInfer.Namespace).Update(modelInferCtx, modelInfer, metav1.UpdateOptions{}); updateErr != nil {
			klog.Errorf("failed to update modelInfer,err: %v", updateErr)
			return updateErr
		}
	} else {
		klog.Errorf("failed to get old modelInfer,err: %v", err)
		return err
	}

	return nil
}

func formatAutoscalerMapKey(modelName string, backendName string) string {
	if backendName == "" {
		return modelName
	}
	return modelName + "#" + backendName
}

func getMetricTargets(autoscalePolicy *v1alpha1.AutoscalingPolicy) algorithm.MetricsMap {
	metricTargets := algorithm.MetricsMap{}
	if autoscalePolicy == nil {
		klog.Warning("autoscalePolicy is nil,can't get metricTargets")
		return metricTargets
	}

	for _, metric := range autoscalePolicy.Spec.Metrics {
		metricTargets[metric.MetricName] = metric.TargetValue.AsFloat64Slow()
	}
	return metricTargets
}

type ModelInfer struct {
	name     string
	worksize int32
}

func (ac *AutoscaleController) doAutoscale(ctx context.Context, namespace string, autoscaleScope AutoscaleScope, autoscalePolicy *v1alpha1.AutoscalingPolicy, scopeAutoscaler *autoscaler.Autoscaler) (correctedInstances int32, skip bool) {
	klog.InfoS("doAutoscale enter")
	modelInferList, err := ac.listModelInferByLabel(ctx, namespace, autoscaleScope.backendName, autoscaleScope.modelId)
	if err != nil {
		return 0, true
	}

	modelInferMap := make(map[string]ModelInfer)
	var currentInstancesCount int32 = 0
	for _, modelInfer := range modelInferList.Items {
		modelInferMap[modelInfer.Name] = ModelInfer{modelInfer.Name, 0}
		if modelInfer.Spec.Replicas != nil {
			currentInstancesCount += *modelInfer.Spec.Replicas
		}
	}
	klog.InfoS("doAutoscale modelInfer", "currentInstancesCount", currentInstancesCount)
	listPodCtx, cancel := context.WithTimeout(ctx, util.AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	podList, err := ac.kubeClient.CoreV1().Pods(namespace).List(listPodCtx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", utils.ModelInferOwnerKey, autoscaleScope.modelId),
	})

	if err != nil {
		klog.Errorf("failed to get pod list by model.UID: %s", autoscaleScope.modelId)
		return 0, true
	}
	if podList == nil || len(podList.Items) == 0 {
		klog.Errorf("pod list is null")
		return 0, true
	}

	pastHistograms, ok := scopeAutoscaler.PastHistograms.GetLastUnfreshSnapshot()
	if !ok {
		pastHistograms = make(map[string]autoscaler.HistogramInfo)
	}
	currentHistograms := make(map[string]autoscaler.HistogramInfo)

	// classify the podList accoarding to the label,then iterate
	instancePodListMap := make(map[string][]corev1.Pod)
	for _, pod := range podList.Items {
		podLabels := pod.Labels
		inferGroupName, ok := podLabels[workload.GroupNameLabelKey]
		if ok {
			if _, ok := instancePodListMap[inferGroupName]; !ok {
				instancePodListMap[inferGroupName] = make([]corev1.Pod, 0)
			}
			instancePodListMap[inferGroupName] = append(instancePodListMap[inferGroupName], pod)
		}
	}

	unreadyInstancesCount := int32(0)
	readyInstancesMetrics := []algorithm.MetricsMap{}
	for _, instancePodList := range instancePodListMap {
		instanceInfo := ac.processInstance(ctx, instancePodList, pastHistograms, currentHistograms, scopeAutoscaler.MetricTargets)
		klog.InfoS("finish to processInstance", "instanceInfo.isFailed", instanceInfo.isFailed)
		klog.InfoS("finish to processInstance", "instanceInfo.isReady", instanceInfo.isReady)
		klog.InfoS("finish to processInstance", "instanceInfo.metricsMap", instanceInfo.metricsMap)
		if instanceInfo.isFailed {
			continue
		}

		if !instanceInfo.isReady {
			unreadyInstancesCount++
			continue
		}
		readyInstancesMetrics = append(readyInstancesMetrics, instanceInfo.metricsMap)
	}

	scopeAutoscaler.PastHistograms.Append(currentHistograms)
	// minInstance <- AutoscaleScope, currentInstancesCount(replicas) <- workload
	recommendedInstances, skip := algorithm.GetRecommendedInstances(algorithm.GetRecommendedInstancesArgs{
		MinInstances:          scopeAutoscaler.GlobalInfo.MinReplicas,
		MaxInstances:          scopeAutoscaler.GlobalInfo.MaxReplicas,
		CurrentInstancesCount: currentInstancesCount,
		Tolerance:             float64(autoscalePolicy.Spec.TolerancePercent) * 0.01,
		MetricTargets:         scopeAutoscaler.MetricTargets,
		UnreadyInstancesCount: unreadyInstancesCount,
		ReadyInstancesMetrics: readyInstancesMetrics,
		ExternalMetrics:       make(algorithm.MetricsMap),
	})
	if skip {
		klog.Warning("skip recommended instances")
		return correctedInstances, skip
	}

	if autoscalePolicy.Spec.Behavior.ScaleUp.PanicPolicy.PanicThresholdPercent != nil && recommendedInstances*100 >= currentInstancesCount*(*autoscalePolicy.Spec.Behavior.ScaleUp.PanicPolicy.PanicThresholdPercent) {
		scopeAutoscaler.RefreshPanicMode()
	}
	correctedInstances = algorithm.GetCorrectedInstances(algorithm.GetCorrectedInstancesArgs{
		Autoscaler:           scopeAutoscaler,
		Behavior:             &autoscalePolicy.Spec.Behavior,
		MinInstances:         scopeAutoscaler.GlobalInfo.MinReplicas,
		MaxInstances:         scopeAutoscaler.GlobalInfo.MaxReplicas,
		CurrentInstances:     currentInstancesCount,
		RecommendedInstances: recommendedInstances})

	klog.InfoS("autoscale controller", "recommendedInstances", recommendedInstances, "correctedInstances", correctedInstances)
	scopeAutoscaler.AppendRecommendation(recommendedInstances)
	scopeAutoscaler.AppendCorrected(correctedInstances)
	return correctedInstances, skip
}

func (ac *AutoscaleController) listModelInferByLabel(ctx context.Context, namespace string, backendName string, modelUID types.UID) (*workload.ModelInferList, error) {
	var listOptions metav1.ListOptions
	if backendName != "" {
		listOptions = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", utils.ModelInferOwnerKey, modelUID),
		}
	}

	modelInferCtx, cancel := context.WithTimeout(ctx, util.AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	if modelInfers, err := ac.client.WorkloadV1alpha1().ModelInfers(namespace).List(modelInferCtx, listOptions); err != nil {
		klog.Errorf("list modelInfer error: %v", err)
		return nil, err
	} else {
		return modelInfers, nil
	}
}

type InstanceInfo struct {
	isReady    bool
	isFailed   bool
	metricsMap algorithm.MetricsMap
}

func (ac *AutoscaleController) processInstance(ctx context.Context, podList []corev1.Pod, pastHistograms map[string]autoscaler.HistogramInfo, currentHistograms map[string]autoscaler.HistogramInfo, metricTargets algorithm.MetricsMap) InstanceInfo {
	klog.InfoS("processInstance start")
	instanceInfo := InstanceInfo{true, false, make(algorithm.MetricsMap)}
	for i := range podList {
		pod := &podList[i]
		instanceInfo.isReady = instanceInfo.isReady && inferControllerUtils.IsPodRunningAndReady(pod)
		instanceInfo.isFailed = instanceInfo.isFailed || checkPodIsFailed(pod) || inferControllerUtils.ContainerRestarted(pod)

		pastValue, ok := pastHistograms[pod.Name]
		var pastHistogramMap map[string]*histogram.Snapshot
		if !ok || pod.Status.StartTime == nil || pastValue.PodStartTime == nil || !pod.Status.StartTime.Equal(pastValue.PodStartTime) {
			pastHistogramMap = make(map[string]*histogram.Snapshot)
		} else {
			pastHistogramMap = pastValue.HistogramMap
		}

		currentHistogramMap := make(map[string]*histogram.Snapshot)
		for _, container := range pod.Spec.Containers {
			if container.Name == "runtime" {
				ports := container.Ports
				if len(ports) == 0 {
					klog.Errorf("ports is invalid")
					continue
				}

				ip := pod.Status.PodIP
				containerPort := ports[0].ContainerPort

				podCtx, cancel := context.WithTimeout(ctx, util.AutoscaleCtxTimeoutSeconds*time.Second)
				defer cancel()
				url := fmt.Sprintf("http://%s:%d/metrics", ip, containerPort)
				req, _ := http.NewRequestWithContext(podCtx, http.MethodGet, url, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					klog.Errorf("get metric response error: %v", err)
					continue
				}
				if resp == nil || resp.Body == nil {
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
				processMetricString(result, metricTargets, pastHistogramMap, currentHistogramMap, instanceInfo.metricsMap)
				currentHistograms[pod.Name] = autoscaler.HistogramInfo{
					PodStartTime: pod.Status.StartTime,
					HistogramMap: currentHistogramMap,
				}
				return instanceInfo
			}
		}
	}
	return instanceInfo
}

func checkPodIsFailed(pod *corev1.Pod) bool {
	status := pod.Status
	metaData := pod.ObjectMeta
	return status.Phase == corev1.PodFailed || metaData.DeletionTimestamp != nil
}

func processMetricString(metricStr string, metricTargets algorithm.MetricsMap, pastHistograms map[string]*histogram.Snapshot, currentHistograms map[string]*histogram.Snapshot, instanceMetricMap algorithm.MetricsMap) {
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

		if _, ok := metricTargets[mf.GetName()]; !ok {
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

	for key := range metricTargets {
		if _, ok := instanceMetricMap[key]; !ok {
			instanceMetricMap[key] = 0
		}
	}
}

func addMetric(instanceMetricMap algorithm.MetricsMap, metricName string, metricValue float64) {
	if oldValue, ok := instanceMetricMap[metricName]; ok {
		instanceMetricMap[metricName] = oldValue + metricValue
	} else {
		instanceMetricMap[metricName] = metricValue
	}
}
