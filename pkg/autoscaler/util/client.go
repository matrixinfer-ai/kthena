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
	"github.com/volcano-sh/kthena/pkg/apis/registry/v1alpha1"
	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"istio.io/istio/pkg/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

const (
	ModelServingEntryPodLabel = "leader"
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

func UpdateModelInfer(ctx context.Context, client clientset.Interface, modelServing *workload.ModelServing) error {
	modelServingCtx, cancel := context.WithTimeout(ctx, AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	if oldModelServing, err := client.WorkloadV1alpha1().ModelServings(modelServing.Namespace).Get(modelServingCtx, modelServing.Name, metav1.GetOptions{}); err == nil {
		modelServing.ResourceVersion = oldModelServing.ResourceVersion
		if _, updateErr := client.WorkloadV1alpha1().ModelServings(modelServing.Namespace).Update(modelServingCtx, modelServing, metav1.UpdateOptions{}); updateErr != nil {
			klog.Errorf("failed to update modelServing,err: %v", updateErr)
			return updateErr
		}
	} else {
		klog.Errorf("failed to get old modelServing,err: %v", err)
		return err
	}
	return nil
}

func GetTargetLabels(target *v1alpha1.Target) map[string]string {
	if target.TargetRef.Kind == workload.ModelServingKind.Kind {
		lbs := map[string]string{}
		if target.AdditionalMatchLabels != nil {
			lbs = maps.Clone(target.AdditionalMatchLabels)
		}
		lbs[workload.ModelServingNameLabelKey] = target.TargetRef.Name
		lbs[workload.RoleLabelKey] = ModelServingEntryPodLabel
		return lbs
	}
	return nil
}
