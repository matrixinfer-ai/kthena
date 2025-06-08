/*
Copyright 2024.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// ModelServerController reconciles a ModelServer object
type ModelServerController struct {
	client.Client
	Scheme *runtime.Scheme

	store datastore.Store
}

func NewModelServerController(mgr ctrl.Manager, store datastore.Store) *ModelServerController {
	return &ModelServerController{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		store: store,
	}
}

// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelServers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelServers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelServers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ModelServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *ModelServerController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ms := &aiv1alpha1.ModelServer{}
	name := req.NamespacedName

	if err := r.Get(ctx, name, ms); err != nil {
		log.Infof("Delete ModelServer: %v", name.String())
		_ = r.store.DeleteModelServer(ms)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: ms.Spec.WorkloadSelector.MatchLabels})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("invalid selector: %v", err)
	}

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(req.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, err
	}

	pods := make([]*corev1.Pod, 0, len(podList.Items))
	for i := range podList.Items {
		if isPodReady(&podList.Items[i]) {
			pods = append(pods, &podList.Items[i])
		}
	}

	log.Infof("Update ModelServer: %v", name.String())
	_ = r.store.AddOrUpdateModelServer(name, ms, pods)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelServerController) SetupWithManager(mgr ctrl.Manager) error {
	log.Infof("start modelserver controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&aiv1alpha1.ModelServer{}).
		Named("ModelServer").
		Complete(r)
}
