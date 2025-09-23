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

package util

import (
	"context"
	"time"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	workloadLister "github.com/volcano-sh/kthena/client-go/listers/workload/v1alpha1"
	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"istio.io/istio/pkg/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

const (
	ModelInferEntryPodLabel = "leader"
)

func GetModelInferTarget(lister workloadLister.ModelServingLister, namespace string, name string) (*workload.ModelServing, error) {
	if instance, err := lister.ModelServings(namespace).Get(name); err != nil {
		return nil, err
	} else {
		return instance, nil
	}
}

func GetMetricPods(lister listerv1.PodLister, namespace string, matchLabels map[string]string) ([]*corev1.Pod, error) {
	if podList, err := lister.Pods(namespace).List(labels.SelectorFromSet(matchLabels)); err != nil {
		return nil, err
	} else {
		return podList, nil
	}
}

func UpdateModelInfer(ctx context.Context, client clientset.Interface, modelInfer *workload.ModelServing) error {
	modelInferCtx, cancel := context.WithTimeout(ctx, AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	if oldModelInfer, err := client.WorkloadV1alpha1().ModelServings(modelInfer.Namespace).Get(modelInferCtx, modelInfer.Name, metav1.GetOptions{}); err == nil {
		modelInfer.ResourceVersion = oldModelInfer.ResourceVersion
		if _, updateErr := client.WorkloadV1alpha1().ModelServings(modelInfer.Namespace).Update(modelInferCtx, modelInfer, metav1.UpdateOptions{}); updateErr != nil {
			klog.Errorf("failed to update modelInfer,err: %v", updateErr)
			return updateErr
		}
	} else {
		klog.Errorf("failed to get old modelInfer,err: %v", err)
		return err
	}
	return nil
}

func GetTargetLabels(target *workload.Target) map[string]string {
	if target.TargetRef.Kind == workload.ModelServingKind.Kind {
		lbs := map[string]string{}
		if target.AdditionalMatchLabels != nil {
			lbs = maps.Clone(target.AdditionalMatchLabels)
		}
		lbs[workload.ModelServingNameLabelKey] = target.TargetRef.Name
		lbs[workload.RoleLabelKey] = ModelInferEntryPodLabel
		return lbs
	}
	return nil
}
