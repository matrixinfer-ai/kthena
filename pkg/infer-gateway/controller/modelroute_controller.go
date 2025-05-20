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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

// ModelRouteController reconciles a ModelRoute object
type ModelRouteController struct {
	client.Client
	Scheme *runtime.Scheme
	store  datastore.Store
}

func NewModelRouteController(mgr ctrl.Manager, store datastore.Store) *ModelRouteController {
	return &ModelRouteController{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		store:  store,
	}
}

// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelRoutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelRoutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelRoutes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ModelRoute object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (m *ModelRouteController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mr aiv1alpha1.ModelRoute
	if err := m.Get(ctx, req.NamespacedName, &mr); err != nil {
		if apierrors.IsNotFound(err) {
			if err := m.store.DeleteModelRoute(req.NamespacedName.String()); err != nil {
				log.Errorf("failed to delete route: %v", err)
				return ctrl.Result{}, nil
			}
		}

		log.Errorf("unable to fetch ModelRoute: %v", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Infof("Get ModelRoute model: %s", mr.Spec.ModelName)

	// Update route in datastore
	if err := m.store.UpdateModelRoute(req.NamespacedName.String(), &mr); err != nil {
		log.Error(err, "failed to update ModelRouter")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (m *ModelRouteController) SetupWithManager(mgr ctrl.Manager) error {
	log.Infof("start modelroutes controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&aiv1alpha1.ModelRoute{}).
		Named("ModelRoute").
		Complete(m)
}
