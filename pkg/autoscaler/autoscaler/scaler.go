/*
Copyright The Volcano Authors.

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

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	workloadLister "github.com/volcano-sh/kthena/client-go/listers/workload/v1alpha1"
	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/autoscaler/algorithm"
	"github.com/volcano-sh/kthena/pkg/autoscaler/util"
	"k8s.io/apimachinery/pkg/types"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type Autoscaler struct {
	Collector *MetricCollector
	Status    *Status
	Meta      *ScalingMeta
}
type ScalingMeta struct {
	Config        *workload.Homogeneous
	MetricTargets map[string]float64
	BindingId     types.UID
	Namespace     string
}

func NewAutoscaler(behavior *workload.AutoscalingPolicyBehavior, binding *workload.AutoscalingPolicyBinding, metricTargets map[string]float64) *Autoscaler {
	return &Autoscaler{
		Status:    NewStatus(behavior),
		Collector: NewMetricCollector(&binding.Spec.Homogeneous.Target, binding, metricTargets),
		Meta: &ScalingMeta{
			Config:        binding.Spec.Homogeneous,
			BindingId:     binding.UID,
			Namespace:     binding.Namespace,
			MetricTargets: metricTargets,
		},
	}
}

func (autoscaler *Autoscaler) Scale(ctx context.Context, client clientset.Interface, modelServingLister workloadLister.ModelServingLister, podLister listerv1.PodLister, autoscalePolicy *workload.AutoscalingPolicy) error {
	// Get autoscaler target(model infer) instance
	modelInfer, err := util.GetModelInferTarget(modelServingLister, autoscaler.Meta.Namespace, autoscaler.Meta.Config.Target.TargetRef.Name)
	if err != nil {
		klog.Errorf("get model infer error: %v", err)
		return err
	}
	currentInstancesCount := *modelInfer.Spec.Replicas
	klog.InfoS("doAutoscale modelInfer", "currentInstancesCount", currentInstancesCount)

	unreadyInstancesCount, readyInstancesMetrics, err := autoscaler.Collector.UpdateMetrics(ctx, podLister)
	if err != nil {
		klog.Errorf("update metrics error: %v", err)
		return err
	}
	// minInstance <- AutoscaleScope, currentInstancesCount(replicas) <- workload
	instancesAlgorithm := algorithm.RecommendedInstancesAlgorithm{
		MinInstances:          autoscaler.Meta.Config.MinReplicas,
		MaxInstances:          autoscaler.Meta.Config.MaxReplicas,
		CurrentInstancesCount: currentInstancesCount,
		Tolerance:             float64(autoscalePolicy.Spec.TolerancePercent) * 0.01,
		MetricTargets:         autoscaler.Meta.MetricTargets,
		UnreadyInstancesCount: unreadyInstancesCount,
		ReadyInstancesMetrics: []algorithm.Metrics{readyInstancesMetrics},
		ExternalMetrics:       make(algorithm.Metrics),
	}
	recommendedInstances, skip := instancesAlgorithm.GetRecommendedInstances()
	if skip {
		klog.Warning("skip recommended instances")
		return nil
	}
	if autoscalePolicy.Spec.Behavior.ScaleUp.PanicPolicy.PanicThresholdPercent != nil && recommendedInstances*100 >= currentInstancesCount*(*autoscalePolicy.Spec.Behavior.ScaleUp.PanicPolicy.PanicThresholdPercent) {
		autoscaler.Status.RefreshPanicMode()
	}
	CorrectedInstancesAlgorithm := algorithm.CorrectedInstancesAlgorithm{
		IsPanic:              autoscaler.Status.IsPanicMode(),
		History:              autoscaler.Status.History,
		Behavior:             &autoscalePolicy.Spec.Behavior,
		MinInstances:         autoscaler.Meta.Config.MinReplicas,
		MaxInstances:         autoscaler.Meta.Config.MaxReplicas,
		CurrentInstances:     currentInstancesCount,
		RecommendedInstances: recommendedInstances,
	}
	recommendedInstances = CorrectedInstancesAlgorithm.GetCorrectedInstances()

	klog.InfoS("autoscale controller", "recommendedInstances", recommendedInstances, "correctedInstances", recommendedInstances)
	autoscaler.Status.AppendRecommendation(recommendedInstances)
	autoscaler.Status.AppendCorrected(recommendedInstances)

	if modelInfer.Spec.Replicas == nil || *modelInfer.Spec.Replicas == recommendedInstances {
		klog.InfoS("modelInfer replicas no need to update")
		return nil
	}
	*modelInfer.Spec.Replicas = recommendedInstances
	if err = util.UpdateModelInfer(ctx, client, modelInfer); err != nil {
		klog.Errorf("failed to update modelInfer replicas for modelInfer.Name: %s, error: %v", modelInfer.Name, err)
		return err
	}
	return nil
}
