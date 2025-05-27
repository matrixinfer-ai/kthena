/*
Copyright 2025.

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
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	registryv1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

const (
	ModelFinalizer = "matrixinfer.ai/matrixinfer/finalizer"
)

// ModelReconciler reconciles a Model object
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=registry.matrixinfer.ai,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=registry.matrixinfer.ai,resources=models/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=registry.matrixinfer.ai,resources=models/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Model object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Info("Start to process model")

	model := &registryv1.Model{}
	if err := r.Get(ctx, req.NamespacedName, model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if model.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(model, ModelFinalizer) {
			controllerutil.AddFinalizer(model, ModelFinalizer)
			if err := r.Update(ctx, model); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(model, ModelFinalizer) {
			// TODO: Add logic before delete model
			controllerutil.RemoveFinalizer(model, ModelFinalizer)
			if err := r.Update(ctx, model); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	modelInfers := make([]*workload.ModelInfer, len(model.Spec.Backends))
	_, err := r.convertModelToModelInfer(model, modelInfers)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, modelInfer := range modelInfers {
		// modelInfer is owned by model. ModelInfer will be deleted when the model is deleted
		if err := controllerutil.SetControllerReference(model, modelInfer, r.Scheme); err != nil {
			klog.Error(err, "Failed to set controller reference")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, modelInfer); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ModelReconciler) convertModelToModelInfer(model *registryv1.Model, infers []*workload.ModelInfer) (ctrl.Result, error) {
	for _, backend := range model.Spec.Backends {
		roles := make([]workload.Role, len(backend.Workers))
		for _, worker := range backend.Workers {
			args, err := convertJSONToStringSlice(backend.Config.Raw)
			if err != nil {
				klog.Error(err)
				return ctrl.Result{}, err
			}
			roles = append(roles, workload.Role{
				Name:            string(worker.Type),
				Replicas:        &worker.Replicas,
				NetworkTopology: nil,
				EntryTemplate: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{
								Name:  "init-container",
								Image: "matrixinfer/runtime",
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "vllm-leader",
								Image: worker.Image,
								Args:  args,
							},
						},
						Resources: &worker.Resources,
					},
				},
				WorkerReplicas: func() *int32 { result := worker.Pods - 1; return &result }(),
				WorkerTemplate: nil,
			})
		}
		infers = append(infers, &workload.ModelInfer{
			ObjectMeta: v1.ObjectMeta{
				Name:      backend.Name,
				Namespace: model.Namespace,
			},
			Spec: workload.ModelInferSpec{
				Replicas:      &backend.MinReplicas,
				SchedulerName: "volcano", // TODO: how to get scheduler name?
				Template: workload.InferGroup{
					Spec: workload.InferGroupSpec{
						RestartGracePeriodSeconds: nil,
						NetworkTopology:           nil,
						GangSchedule:              workload.GangSchedule{},
						Roles:                     roles,
					},
				},
				RolloutStrategy: workload.RolloutStrategy{
					Type:                       workload.InferGroupRollingUpdate,
					RollingUpdateConfiguration: nil,
				},
				RecoveryPolicy:            workload.InferGroupRestart, // TODO: judge by backend type
				TopologySpreadConstraints: nil,
			},
		})
	}
	return ctrl.Result{}, nil
}

func convertJSONToStringSlice(src []byte) ([]string, error) {
	var strSlice []string
	err := json.Unmarshal(src, &strSlice)
	if err != nil {
		return nil, err
	}
	return strSlice, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&registryv1.Model{}).
		Named("model").
		Complete(r)
}
