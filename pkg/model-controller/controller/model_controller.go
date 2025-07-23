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

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"

	workloadLister "matrixinfer.ai/matrixinfer/client-go/listers/workload/v1alpha1"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	registryLister "matrixinfer.ai/matrixinfer/client-go/listers/registry/v1alpha1"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/config"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ModelInitsReason              = "ModelInits"
	ModelUpdatingReason           = "ModelUpdating"
	ModelActiveReason             = "ModelActive"
	CreateModelServerFailedReason = "CreateModelServerFailed"
	CreateModelRouteFailedReason  = "CreateModelRouteFailed"
	ConfigMapName                 = "model-config-map"
)

type ModelController struct {
	// Client for k8s. Use it to call K8S API
	kubeClient kubernetes.Interface
	// client for custom resource
	client clientset.Interface

	syncHandler         func(ctx context.Context, miKey string) error
	modelsLister        registryLister.ModelLister
	modelsInformer      cache.Controller
	modelInfersLister   workloadLister.ModelInferLister
	modelInfersInformer cache.Controller
	workQueue           workqueue.TypedRateLimitingInterface[any]
}

func (mc *ModelController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer mc.workQueue.ShutDown()

	// start informers
	go mc.modelsInformer.RunWithContext(ctx)
	go mc.modelInfersInformer.RunWithContext(ctx)

	cache.WaitForCacheSync(ctx.Done(),
		mc.modelsInformer.HasSynced,
		mc.modelInfersInformer.HasSynced,
	)

	klog.Info("start model controller")
	for i := 0; i < workers; i++ {
		go mc.worker(ctx)
	}
	<-ctx.Done()
	klog.Info("shut down model controller")
}

func (mc *ModelController) worker(ctx context.Context) {
	for mc.processNextWorkItem(ctx) {
	}
}

func (mc *ModelController) processNextWorkItem(ctx context.Context) bool {
	key, quit := mc.workQueue.Get()
	if quit {
		return false
	}
	defer mc.workQueue.Done(key)

	err := mc.syncHandler(ctx, key.(string))
	if err == nil {
		mc.workQueue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("sync %q failed with %v", key, err))
	mc.workQueue.AddRateLimited(key)

	return true
}

func (mc *ModelController) createModel(obj interface{}) {
	model, ok := obj.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse Model when createModel")
		return
	}
	klog.V(4).Info("Creating", "model", klog.KObj(model))
	mc.enqueueModel(model)
}

func (mc *ModelController) enqueueModel(model *registryv1alpha1.Model) {
	if key, err := cache.MetaNamespaceKeyFunc(model); err != nil {
		utilruntime.HandleError(err)
	} else {
		mc.workQueue.Add(key)
	}
}

func (mc *ModelController) updateModel(old interface{}, new interface{}) {
	newModel, ok := new.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse new Model type when updateModel")
		return
	}
	oldModel, ok := old.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse old Model when updateModel")
		return
	}
	// When observed generation not equal to generation, reconcile model
	if oldModel.Status.ObservedGeneration != newModel.Generation {
		mc.enqueueModel(newModel)
	}
}

func (mc *ModelController) deleteModel(obj interface{}) {
	model, ok := obj.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse Model when deleteModel")
		return
	}
	klog.Infof("Delete model: %s", model.Name)
}

// reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (mc *ModelController) reconcile(ctx context.Context, namespaceAndName string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(namespaceAndName)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", err)
	}
	model, err := mc.modelsLister.Models(namespace).Get(name)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	klog.InfoS("Start to process model", "namespace", namespace, "model name", model.Name, "model status", model.Status)
	if len(model.Status.Conditions) == 0 {
		klog.Info("model status condition is null, create model infer")
		if modelInfers, err := utils.BuildModelInferCR(model); err != nil {
			klog.Errorf("failed to build model infer for model %s: %v", model.Name, err)
			return err
		} else {
			for _, modelInfer := range modelInfers {
				// modelInfer is owned by the model. ModelInfer will be deleted when the model is deleted
				if _, err := mc.client.WorkloadV1alpha1().ModelInfers(model.Namespace).Create(ctx, modelInfer, metav1.CreateOptions{}); err != nil {
					klog.Errorf("create modelInfer failed: %v", err)
					return err
				}
			}
			meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeInitializing),
				metav1.ConditionTrue, ModelInitsReason, "Model is initializing"))
			if err := mc.updateModelStatus(ctx, model); err != nil {
				klog.Errorf("update model status failed: %v", err)
				return err
			}
		}
	}
	if model.Generation != model.Status.ObservedGeneration {
		klog.Info("model generation is not equal to observed generation, update model infer")
		if err := mc.setModelUpdateCondition(ctx, model); err != nil {
			return err
		}
		if err := mc.updateModelInfer(ctx, model); err != nil {
			return err
		}
		if err := mc.updateModelServer(ctx, model); err != nil {
			return err
		}
	}
	modelInferActive, err := mc.isModelInferActive(ctx, model)
	if err != nil || !modelInferActive {
		return err
	}
	if err := mc.setModelActiveCondition(ctx, model); err != nil {
		return err
	}
	if err := mc.createModelServer(ctx, model); err != nil {
		updateError := mc.setModelServerFailedCondition(ctx, model)
		if updateError != nil {
			return updateError
		}
		return err
	}
	return nil
}

// setModelServerFailedCondition sets model server conditions when creating model server failed.
func (mc *ModelController) setModelServerFailedCondition(ctx context.Context, model *registryv1alpha1.Model) error {
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeFailed),
		metav1.ConditionTrue, CreateModelServerFailedReason, "Creating model server failed"))
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeActive),
		metav1.ConditionFalse, CreateModelServerFailedReason, "Model is not active due to failed create model server"))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

// isModelInferActive returns true if all Model Infers are available.
func (mc *ModelController) isModelInferActive(ctx context.Context, model *registryv1alpha1.Model) (bool, error) {
	// List all Model Infers associated with the model
	modelInferList, err := mc.listModelInferByLabel(ctx, model)
	if err != nil {
		return false, err
	}
	// Ensure the number of Model Infers matches the number of backends
	if len(modelInferList.Items) != len(model.Spec.Backends) {
		return false, fmt.Errorf("model infer number not equal to backend number")
	}
	// Check if all Model Infers are available
	for _, modelInfer := range modelInferList.Items {
		if !meta.IsStatusConditionPresentAndEqual(modelInfer.Status.Conditions, string(workload.ModelInferAvailable), metav1.ConditionTrue) {
			// requeue until all Model Infers are active
			klog.InfoS("model infer is not available", "model infer", modelInfer.Name, "namespace", modelInfer.Namespace)
			return false, nil
		}
	}
	return true, nil
}

// setModelActiveCondition sets model conditions when all Model Infers are active.
func (mc *ModelController) setModelActiveCondition(ctx context.Context, model *registryv1alpha1.Model) error {
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeActive),
		metav1.ConditionTrue, ModelActiveReason, "Model is active"))
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeInitializing),
		metav1.ConditionFalse, ModelActiveReason, "Model is active, so initializing is false"))
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeUpdating),
		metav1.ConditionFalse, ModelActiveReason, "Model not updating"))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

// newCondition returns a condition
func newCondition(conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// updateModelStatus updates model status.
func (mc *ModelController) updateModelStatus(ctx context.Context, model *registryv1alpha1.Model) error {
	modelInferList, err := mc.listModelInferByLabel(ctx, model)
	if err != nil {
		return err
	}
	var backendStatus []registryv1alpha1.ModelBackendStatus
	for _, infer := range modelInferList.Items {
		backendStatus = append(backendStatus, registryv1alpha1.ModelBackendStatus{
			Name:     infer.Name,
			Hash:     "", // todo: get hash
			Replicas: infer.Status.Replicas,
		})
	}
	model.Status.BackendStatuses = backendStatus
	model.Status.ObservedGeneration = model.Generation
	if _, err := mc.client.RegistryV1alpha1().Models(model.Namespace).UpdateStatus(ctx, model, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

func NewModelController(kubeClient kubernetes.Interface, client clientset.Interface) *ModelController {
	informerFactory := informersv1alpha1.NewSharedInformerFactory(client, 0)
	modelInformer := informerFactory.Registry().V1alpha1().Models()
	modelInferInformer := informerFactory.Workload().V1alpha1().ModelInfers()
	mc := &ModelController{
		kubeClient:          kubeClient,
		client:              client,
		modelsLister:        modelInformer.Lister(),
		modelsInformer:      modelInformer.Informer(),
		modelInfersLister:   modelInferInformer.Lister(),
		modelInfersInformer: modelInferInformer.Informer(),
		workQueue: workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{}),
	}
	klog.Info("Set the Model event handler")
	_, err := modelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    mc.createModel,
		UpdateFunc: mc.updateModel,
		DeleteFunc: mc.deleteModel,
	})
	if err != nil {
		klog.Fatal("Unable to add model event handler")
		return nil
	}
	_, err = modelInferInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: mc.triggerModel,
	})
	if err != nil {
		klog.Fatal("Unable to add model infer event handler")
		return nil
	}
	mc.syncHandler = mc.reconcile
	mc.loadConfigFromConfigMap()
	return mc
}

// listModelInferByLabel list all model infer which label key is "owner" and label value is model uid
func (mc *ModelController) listModelInferByLabel(ctx context.Context, model *registryv1alpha1.Model) (*workload.ModelInferList, error) {
	if modelInfers, err := mc.client.WorkloadV1alpha1().ModelInfers(model.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", utils.ModelInferOwnerKey, model.UID),
	}); err != nil {
		return nil, err
	} else {
		return modelInfers, nil
	}
}

// updateModelInfer updates model infer when model changed
func (mc *ModelController) updateModelInfer(ctx context.Context, model *registryv1alpha1.Model) error {
	modelInfers, err := utils.BuildModelInferCR(model)
	if err != nil {
		return err
	}
	for _, modelInfer := range modelInfers {
		oldModelInfer, err := mc.client.WorkloadV1alpha1().ModelInfers(modelInfer.Namespace).Get(ctx, modelInfer.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				if _, err := mc.client.WorkloadV1alpha1().ModelInfers(model.Namespace).Create(ctx, modelInfer, metav1.CreateOptions{}); err != nil {
					klog.Errorf("failed to create ModelInfer %s: %v", klog.KObj(modelInfer), err)
					return err
				}
				continue
			}
			klog.Errorf("failed to get ModelInfer %s: %v", klog.KObj(modelInfer), err)
			return err
		}
		if equality.Semantic.DeepEqual(oldModelInfer.Spec, modelInfer.Spec) {
			continue
		}
		modelInfer.ResourceVersion = oldModelInfer.ResourceVersion
		if _, err := mc.client.WorkloadV1alpha1().ModelInfers(model.Namespace).Update(ctx, modelInfer, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// setModelUpdateCondition sets model condition to updating
func (mc *ModelController) setModelUpdateCondition(ctx context.Context, model *registryv1alpha1.Model) error {
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeActive),
		metav1.ConditionFalse, ModelUpdatingReason, "Model is updating, not ready yet"))
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeUpdating),
		metav1.ConditionTrue, ModelUpdatingReason, "Model is updating"))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

func (mc *ModelController) loadConfigFromConfigMap() {
	namespace, err := utils.GetInClusterNameSpace()
	// when not running in cluster, namespace is default
	if err != nil {
		klog.Error(err)
		namespace = "default"
	}
	cm, err := mc.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), ConfigMapName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("ConfigMap does not exist. Error: %v", err)
		return
	}
	if modelInferDownloaderImage, ok := cm.Data["model_infer_downloader_image"]; !ok {
		klog.Errorf("failed to load modelInferDownloaderImage: %v", err)
	} else {
		config.Config.SetModelInferDownloaderImage(modelInferDownloaderImage)
	}
	if modelInferRuntimeImage, ok := cm.Data["model_infer_runtime_image"]; !ok {
		klog.Errorf("failed to load model_infer_runtime_image: %v", err)
	} else {
		config.Config.SetModelInferRuntimeImage(modelInferRuntimeImage)
	}
}

// When model infer status changed, model reconciles
func (mc *ModelController) triggerModel(old interface{}, new interface{}) {
	newModelInfer, ok := new.(*workload.ModelInfer)
	if !ok {
		klog.Error("failed to parse new ModelInfer")
		return
	}
	_, ok = old.(*workload.ModelInfer)
	if !ok {
		klog.Error("failed to parse old ModelInfer")
		return
	}
	if newModelInfer.OwnerReferences != nil {
		// Find the owner of modelInfer and reconcile the owner to change its status
		if model, err := mc.modelsLister.Models(newModelInfer.Namespace).Get(newModelInfer.OwnerReferences[0].Name); err == nil {
			mc.enqueueModel(model)
		}
	}
}

// createModelServer creates model server when model infer is available
func (mc *ModelController) createModelServer(ctx context.Context, model *registryv1alpha1.Model) error {
	klog.Info("Model Infer is active, start to create model server")
	modelServers := utils.BuildModelServer(model)
	for _, modelServer := range modelServers {
		if _, err := mc.client.NetworkingV1alpha1().ModelServers(model.Namespace).Create(ctx, modelServer, metav1.CreateOptions{}); err != nil {
			if errors.IsAlreadyExists(err) {
				klog.V(4).InfoS("ModelServer already exists, skipping creation", "modelServer", klog.KObj(modelServer))
				continue
			}
			klog.Errorf("create model server failed: %v", err)
			return err
		}
	}
	return nil
}

// updateModelServer updates model server
func (mc *ModelController) updateModelServer(ctx context.Context, model *registryv1alpha1.Model) error {
	modelServers := utils.BuildModelServer(model)
	for _, modelServer := range modelServers {
		oldModelServer, err := mc.client.NetworkingV1alpha1().ModelServers(modelServer.Namespace).Get(ctx, modelServer.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// ModelServer doesn't exist, create it.
				if _, err := mc.client.NetworkingV1alpha1().ModelServers(model.Namespace).Create(ctx, modelServer, metav1.CreateOptions{}); err != nil {
					klog.Errorf("failed to create ModelServer %s: %v", klog.KObj(modelServer), err)
					return err
				}
				continue
			}
			klog.Errorf("failed to get ModelServer %s: %v", klog.KObj(modelServer), err)
			return err
		}
		if equality.Semantic.DeepEqual(oldModelServer.Spec, modelServer.Spec) {
			continue
		}
		modelServer.ResourceVersion = oldModelServer.ResourceVersion
		if _, err := mc.client.NetworkingV1alpha1().ModelServers(model.Namespace).Update(ctx, modelServer, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("failed to update ModelServer %s: %v", klog.KObj(modelServer), err)
			return err
		}
	}
	return nil
}
