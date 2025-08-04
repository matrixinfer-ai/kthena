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

package autoscaler

import (
	"context"
	"sort"

	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	workloadLister "matrixinfer.ai/matrixinfer/client-go/listers/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/algorithm"
	"matrixinfer.ai/matrixinfer/pkg/autoscaler/util"
)

type Optimizer struct {
	Meta       *OptimizerMeta
	Collectors map[string]*MetricCollector
	Status     *Status
}

type OptimizerMeta struct {
	Config        *v1alpha1.OptimizerConfiguration
	MetricTargets map[string]float64
	ScalingOrder  []*ReplicaBlock
	MinReplicas   int32
	MaxReplicas   int32
	Scope         Scope
}

type ReplicaBlock struct {
	name     string
	index    int32
	replicas int32
	cost     int64
}

func (meta *OptimizerMeta) RestoreReplicasOfEachBackend(replicas int32) map[string]int32 {
	replicasMap := make(map[string]int32, len(meta.Config.Params))
	for _, param := range meta.Config.Params {
		replicasMap[param.Target.Name] = param.MinReplicas
	}
	replicas = min(max(replicas, meta.MinReplicas), meta.MaxReplicas)
	replicas -= meta.MinReplicas
	for _, block := range meta.ScalingOrder {
		slot := min(replicas, block.replicas)
		replicasMap[block.name] += slot
		replicas -= slot
		if replicas <= 0 {
			break
		}
	}
	return replicasMap
}

func NewOptimizerMeta(binding *v1alpha1.AutoscalingPolicyBinding) *OptimizerMeta {
	minReplicas := int32(0)
	maxReplicas := int32(0)
	var scalingOrder []*ReplicaBlock
	for index, param := range binding.Spec.OptimizerConfiguration.Params {
		minReplicas += param.MinReplicas
		maxReplicas += param.MaxReplicas
		replicas := param.MaxReplicas - param.MinReplicas
		if replicas <= 0 {
			continue
		}
		if *binding.Spec.OptimizerConfiguration.CostExpansionRatePercent == 100 {
			scalingOrder = append(scalingOrder, &ReplicaBlock{
				index:    int32(index),
				name:     param.Target.Name,
				replicas: replicas,
				cost:     int64(param.Cost),
			})
			continue
		}
		packageLen := 1.0
		for replicas > 0 {
			currentLen := min(replicas, max(int32(packageLen), 1))
			scalingOrder = append(scalingOrder, &ReplicaBlock{
				name:     param.Target.Name,
				index:    int32(index),
				replicas: currentLen,
				cost:     int64(param.Cost) * int64(currentLen),
			})
			replicas -= currentLen
			packageLen = packageLen * float64(*binding.Spec.OptimizerConfiguration.CostExpansionRatePercent) / 100
		}
	}
	sort.Slice(scalingOrder, func(i, j int) bool {
		if scalingOrder[i].cost != scalingOrder[j].cost {
			return scalingOrder[i].cost < scalingOrder[j].cost
		}
		return scalingOrder[i].index < scalingOrder[j].index
	})
	return &OptimizerMeta{
		Config:       binding.Spec.OptimizerConfiguration,
		MinReplicas:  minReplicas,
		MaxReplicas:  maxReplicas,
		ScalingOrder: scalingOrder,
		Scope: Scope{
			OwnedBindingId: binding.UID,
			Namespace:      binding.Namespace,
		},
	}
}

func NewOptimizer(behavior *v1alpha1.AutoscalingPolicyBehavior, binding *v1alpha1.AutoscalingPolicyBinding, metricTargets map[string]float64) *Optimizer {
	collectors := make(map[string]*MetricCollector)
	for _, param := range binding.Spec.OptimizerConfiguration.Params {
		collectors[param.Target.Name] = NewMetricCollector(&param.Target, binding, metricTargets)
	}

	return &Optimizer{
		Meta:       NewOptimizerMeta(binding),
		Collectors: collectors,
		Status:     NewStatus(behavior),
	}
}

func (optimizer *Optimizer) Optimize(ctx context.Context, client clientset.Interface, modelInferLister workloadLister.ModelInferLister, podLister listerv1.PodLister, autoscalePolicy *v1alpha1.AutoscalingPolicy) error {
	size := len(optimizer.Meta.Config.Params)
	unreadyInstancesCount := int32(0)
	readyInstancesMetrics := make([]algorithm.Metrics, 0, size)
	currentInstancesCount := int32(0)
	modelInferList := make([]*workload.ModelInfer, 0, size)
	// Update all model infer instances' metrics
	for _, param := range optimizer.Meta.Config.Params {
		collector, exists := optimizer.Collectors[param.Target.Name]
		if !exists {
			klog.Warningf("collector for target %s not exists", param.Target.Name)
			continue
		}

		// Get autoscaler target(model infer) instance
		modelInfer, err := util.GetModelInferTarget(modelInferLister, optimizer.Meta.Scope.Namespace, param.Target.TargetRef.Name)
		if err != nil {
			klog.Errorf("get model infer error: %v", err)
			return err
		}
		currentInstancesCount += *modelInfer.Spec.Replicas
		klog.Infof("Model infer:%s, current replicas:%d", modelInfer.Name, modelInfer.Spec.Replicas)

		currentUnreadyInstancesCount, currentReadyInstancesMetrics, err := collector.UpdateMetrics(ctx, podLister)
		if err != nil {
			klog.Warningf("update metrics error: %v", err)
			continue
		}
		unreadyInstancesCount += currentUnreadyInstancesCount
		readyInstancesMetrics = append(readyInstancesMetrics, currentReadyInstancesMetrics)
		modelInferList = append(modelInferList, modelInfer)
	}
	// Get recommended replicas of all model infer instances
	instancesAlgorithm := algorithm.RecommendedInstancesAlgorithm{
		MinInstances:          optimizer.Meta.MinReplicas,
		MaxInstances:          optimizer.Meta.MaxReplicas,
		CurrentInstancesCount: currentInstancesCount,
		Tolerance:             float64(autoscalePolicy.Spec.TolerancePercent) * 0.01,
		MetricTargets:         optimizer.Meta.MetricTargets,
		UnreadyInstancesCount: unreadyInstancesCount,
		ReadyInstancesMetrics: readyInstancesMetrics,
		ExternalMetrics:       make(algorithm.Metrics),
	}
	recommendedInstances, skip := instancesAlgorithm.GetRecommendedInstances()
	if skip {
		klog.Warning("skip recommended instances")
		return nil
	}
	if recommendedInstances*100 >= currentInstancesCount*(*autoscalePolicy.Spec.Behavior.ScaleUp.PanicPolicy.PanicThresholdPercent) {
		optimizer.Status.RefreshPanicMode()
	}
	CorrectedInstancesAlgorithm := algorithm.CorrectedInstancesAlgorithm{
		IsPanic:              optimizer.Status.IsPanicMode(),
		History:              optimizer.Status.History,
		Behavior:             &autoscalePolicy.Spec.Behavior,
		MinInstances:         optimizer.Meta.MinReplicas,
		MaxInstances:         optimizer.Meta.MaxReplicas,
		CurrentInstances:     currentInstancesCount,
		RecommendedInstances: recommendedInstances}
	recommendedInstances = CorrectedInstancesAlgorithm.GetCorrectedInstances()

	klog.InfoS("autoscale controller", "recommendedInstances", recommendedInstances, "correctedInstances", recommendedInstances)
	optimizer.Status.AppendRecommendation(recommendedInstances)
	optimizer.Status.AppendCorrected(recommendedInstances)

	replicasMap := optimizer.Meta.RestoreReplicasOfEachBackend(recommendedInstances)

	// Update model infer replicas
	for _, modelInfer := range modelInferList {
		if replicasMap[modelInfer.Name] == *modelInfer.Spec.Replicas {
			klog.Warning("modelInfer replicas no need to update")
			continue
		}
		modelInferCopy := modelInfer.DeepCopy()
		modelInferCopy.Spec.Replicas = ptr.To(replicasMap[modelInfer.Name])
		err := util.UpdateModelInfer(ctx, client, modelInferCopy)
		if err != nil {
			klog.Errorf("failed to update modelInfer replicas for modelInfer.Name: %s, error: %v", modelInfer.Name, err)
			return err
		}
	}
	return nil
}
