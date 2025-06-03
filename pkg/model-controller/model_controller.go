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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	registryv1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

const (
	ModelFinalizer = "matrixinfer.ai/finalizer"
	// Reason for condition
	ModelInitsReason             = "ModelInits"
	ModelBackendEngineTypeVLLM   = "vllm"
	ModelBackendEngineTypeSlang  = "sglang"
	ModelBackendEngineTypeMindIE = "mindie"
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
	model := &registryv1.Model{}
	if err := r.Get(ctx, req.NamespacedName, model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	klog.InfoS("Start to process model", "namespace", req.Namespace, "model status", model.Status)

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
	// When model condition is null, create model infer according to model.
	if len(model.Status.Conditions) == 0 {
		klog.Info("model status condition is null, create model infer")

		modelInfers, err := buildModelInferCR(model)
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
		meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1.ModelStatusConditionTypeInitialized),
			metav1.ConditionUnknown, ModelInitsReason, "Model inits"))
		if err := r.Status().Update(ctx, model); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func newCondition(conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func getModelBackendEngineType(engineType *registryv1.ModelBackendType) (string, error) {
	switch *engineType {
	case registryv1.ModelBackendTypeVLLM:
		return ModelBackendEngineTypeVLLM, nil
	case registryv1.ModelBackendTypeVLLMDisaggregated:
		return ModelBackendEngineTypeVLLM, nil
	case registryv1.ModelBackendTypeSGLang:
		return ModelBackendEngineTypeSlang, nil
	case registryv1.ModelBackendTypeMindIE:
		return ModelBackendEngineTypeMindIE, nil
	case registryv1.ModelBackendTypeMindIEDisaggregated:
		return ModelBackendEngineTypeMindIE, nil
	default:
		return "", fmt.Errorf("not support model backend type: %s", *engineType)
	}
}

func buildEngineContainer(backend *registryv1.ModelBackend, worker *registryv1.ModelWorker, volume *corev1.Volume) (*corev1.Container, error) {
	workerName := backend.Name + "-" + string(worker.Type)
	volumeMounts := []corev1.VolumeMount{}
	if volume != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: getMountPath(backend),
		})
	}
	switch worker.Type {
	case registryv1.ModelWorkerTypeServer:
		return &corev1.Container{
			Name:         workerName,
			Image:        worker.Image,
			Args:         nil, // TODO: Get args from backend.config
			Resources:    worker.Resources,
			VolumeMounts: volumeMounts,
		}, nil
	default:
		// TODO: support prefill and decode
		return nil, fmt.Errorf("not support worker type: %s", string(worker.Type))
	}
}

func getWorkerName(backend *registryv1.ModelBackend, worker *registryv1.ModelWorker) string {
	return backend.Name + "-" + string(worker.Type)
}

func getMountPath(backend *registryv1.ModelBackend) string {
	return "/" + backend.Name
}

func buildRuntimeContainer(backend *registryv1.ModelBackend, worker *registryv1.ModelWorker) (*corev1.Container, error) {
	modelBackendEngineType, e := getModelBackendEngineType(&backend.Type)
	if e != nil {
		return nil, e
	}
	switch worker.Type {
	case registryv1.ModelWorkerTypeServer, registryv1.ModelWorkerTypePrefill, registryv1.ModelWorkerTypeDecode:
		return &corev1.Container{
			Name:  getWorkerName(backend, worker) + "-runtime",
			Image: "matrixinfer/runtime:latest", //TODO: Get from helm values
			Args: []string{
				"-p", "8100",
				"-u", "http://locahost:8000",
				"-e", modelBackendEngineType,
			},
		}, nil
	default:
		return nil, nil
	}
}

func buildCacheVolume(backend *registryv1.ModelBackend, worker *registryv1.ModelWorker) (*corev1.Volume, error) {
	// TODO: add format check CacheURI
	volumeName := getWorkerName(backend, worker) + "-weights"
	switch {
	case backend.CacheURI == "":
		return nil, nil
	case strings.HasPrefix(backend.CacheURI, "pvc://"):
		return &corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: strings.Split(backend.CacheURI, "://")[1],
				},
			},
		}, nil
	case strings.HasPrefix(backend.CacheURI, "hostPath://"):
		return &corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: strings.Split(backend.CacheURI, ":/")[1],
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("not support prefix in CacheURI: %s", backend.CacheURI)
}

func buildDownloadContainer(backend *registryv1.ModelBackend, worker *registryv1.ModelWorker, volume *corev1.Volume) (*corev1.Container, error) {
	if worker.Type == registryv1.ModelWorkerTypeController || worker.Type == registryv1.ModelWorkerTypeCoordinator {
		return nil, nil
	}
	workerName := getWorkerName(backend, worker)
	downloaderPath := getMountPath(backend)

	volumeMounts := []corev1.VolumeMount{}
	if volume != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: downloaderPath,
		})
	}
	modelBackendEngineType, e := getModelBackendEngineType(&backend.Type)
	if e != nil {
		return nil, e
	}
	return &corev1.Container{
		Name:         workerName + "-downloader",
		Image:        "matrixinfer/downloader:latest", //TODO: Get from helm values
		Args:         []string{"-s", backend.ModelURI, "-o", downloaderPath, "-e", modelBackendEngineType},
		VolumeMounts: volumeMounts,
	}, nil
}

func buildModelInferCR(model *registryv1.Model) ([]*workload.ModelInfer, error) {
	infers := make([]*workload.ModelInfer, len(model.Spec.Backends))
	for backend_idx, backend := range model.Spec.Backends {
		roles := make([]workload.Role, len(backend.Workers))
		for worker_idx, worker := range backend.Workers {
			workerName := getWorkerName(&backend, &worker)
			initContainers := []corev1.Container{}
			volumes := []corev1.Volume{}
			// Set model weights volumes, if cacheURI is not empty.
			cacheVolume, e := buildCacheVolume(&backend, &worker)
			if e != nil {
				return nil, e
			}
			if cacheVolume != nil {
				volumes = append(volumes, *cacheVolume)
			}
			// Set downloader in init containers.
			downloadContainer, e := buildDownloadContainer(&backend, &worker, cacheVolume)
			if e != nil {
				return nil, e
			}
			if downloadContainer != nil {
				initContainers = append(initContainers, *downloadContainer)
			}
			// Set engine container.
			engineContainer, e := buildEngineContainer(&backend, &worker, cacheVolume)
			if e != nil {
				return nil, e
			}
			containers := []corev1.Container{*engineContainer}
			// Set runtime container if it's not controller or coordinator.
			runtimeContainer, e := buildRuntimeContainer(&backend, &worker)
			if e != nil {
				return nil, e
			}
			if runtimeContainer != nil {
				containers = append(containers, *runtimeContainer)
			}

			roles[worker_idx] = workload.Role{
				Name:            workerName,
				Replicas:        &worker.Replicas,
				NetworkTopology: nil,
				EntryTemplate: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: initContainers,
						Containers:     containers,
						Volumes:        volumes,
					},
				},
				WorkerReplicas: func() *int32 { result := worker.Pods - 1; return &result }(),
				WorkerTemplate: nil, // TODO: fix it
			}
		}
		infers[backend_idx] = &workload.ModelInfer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      model.Name + "-instance",
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
		}
	}
	return infers, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&registryv1.Model{}).
		Named("model").
		Complete(r)
}
