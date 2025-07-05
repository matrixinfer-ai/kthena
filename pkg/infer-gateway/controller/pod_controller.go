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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type PodController struct {
	client.Client

	store datastore.Store
}

func NewPodController(mgr ctrl.Manager, store datastore.Store) *PodController {
	return &PodController{
		Client: mgr.GetClient(),

		store: store,
	}
}

func (p *PodController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := &corev1.Pod{}
	name := req.NamespacedName

	if err := p.Get(ctx, name, pod); err != nil {
		if apierrors.IsNotFound(err) {
			log.Infof("Delete pod: %v", name.String())
			_ = p.store.DeletePod(name)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.Errorf("Unable to get pod %s: %v", name.Namespace+"/"+name.Name, err)
	}

	if !isPodReady(pod) {
		_ = p.store.DeletePod(name)
		return ctrl.Result{}, nil
	}

	ModelServers := &aiv1alpha1.ModelServerList{}
	if err := p.List(ctx, ModelServers, client.InNamespace(pod.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	servers := []*aiv1alpha1.ModelServer{}
	for _, item := range ModelServers.Items {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: item.Spec.WorkloadSelector.MatchLabels})
		if err != nil || !selector.Matches(labels.Set(pod.Labels)) {
			continue
		}
		servers = append(servers, &item)
	}

	if len(servers) == 0 {
		return ctrl.Result{}, nil
	}

	log.Infof("Update pod: %v", name.String())
	if err := p.store.AddOrUpdatePod(pod, servers); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to add or update pod in data store: %v", name.String())
	}

	return ctrl.Result{}, nil
}

func (p *PodController) SetupWithManager(mgr ctrl.Manager) error {
	log.Infof("start pod controller")

	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Pod{}).Complete(p)
}
