package controller

import (
	"context"
	"fmt"

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
	ModelInitsReason    = "ModelInits"
	ModelUpdatingReason = "ModelUpdating"
	ModelActiveReason   = "ModelActive"
	ConfigMapName       = "model-config-map"
)

type ModelController struct {
	// Client for k8s. Use it to call K8S API
	kubeClient kubernetes.Interface
	// client for custom resource
	modelClient clientset.Interface

	syncHandler    func(ctx context.Context, miKey string) error
	modelsLister   registryLister.ModelLister
	modelsInformer cache.Controller

	// nolint
	workqueue workqueue.RateLimitingInterface
}

func (mc *ModelController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer mc.workqueue.ShutDown()

	// start informers
	go mc.modelsInformer.RunWithContext(ctx)

	cache.WaitForCacheSync(ctx.Done(),
		mc.modelsInformer.HasSynced,
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
	key, quit := mc.workqueue.Get()
	if quit {
		return false
	}
	defer mc.workqueue.Done(key)

	err := mc.syncHandler(ctx, key.(string))
	if err == nil {
		mc.workqueue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("sync %q failed with %v", key, err))
	mc.workqueue.AddRateLimited(key)

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
		mc.workqueue.Add(key)
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
	// When observed generation not equal to generation, trigger reconciles
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
				// modelInfer is owned by model. ModelInfer will be deleted when the model is deleted
				if _, err := mc.modelClient.WorkloadV1alpha1().ModelInfers(model.Namespace).Create(ctx, modelInfer, metav1.CreateOptions{}); err != nil {
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
		return mc.updateModelInfer(ctx, model)
	}
	return mc.isModelInferActive(ctx, model)
}

// isModelInferActive checks all Model Infers that belong to this model are available
func (mc *ModelController) isModelInferActive(ctx context.Context, model *registryv1alpha1.Model) error {
	modelInferList, err := mc.listModelInferByLabel(ctx, model)
	if err != nil {
		return err
	}
	if len(modelInferList.Items) != len(model.Spec.Backends) {
		return fmt.Errorf("model infer number not equal to backend number")
	}
	for _, modelInfer := range modelInferList.Items {
		if !meta.IsStatusConditionPresentAndEqual(modelInfer.Status.Conditions, string(workload.ModelInferSetAvailable), metav1.ConditionTrue) {
			// requeue until all Model Infers are active
			klog.InfoS("model infer is not active", "model infer", modelInfer.Name, "namespace", modelInfer.Namespace)
			return nil
		}
	}
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
	if _, err := mc.modelClient.RegistryV1alpha1().Models(model.Namespace).UpdateStatus(ctx, model, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	return nil
}

func NewModelController(kubeClient kubernetes.Interface, modelClient clientset.Interface) *ModelController {
	modelInformerFactory := informersv1alpha1.NewSharedInformerFactory(modelClient, 0)
	modelInformer := modelInformerFactory.Registry().V1alpha1().Models()
	mc := &ModelController{
		kubeClient:     kubeClient,
		modelClient:    modelClient,
		modelsLister:   modelInformer.Lister(),
		modelsInformer: modelInformer.Informer(),
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Models"),
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
	mc.syncHandler = mc.reconcile
	mc.loadConfigFromConfigMap()
	return mc
}

// listModelInferByLabel list all model infer which label key is "owner" and label value is model uid
func (mc *ModelController) listModelInferByLabel(ctx context.Context, model *registryv1alpha1.Model) (*workload.ModelInferList, error) {
	if modelInfers, err := mc.modelClient.WorkloadV1alpha1().ModelInfers(model.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", utils.ModelInferOwnerKey, model.UID),
	}); err != nil {
		return nil, err
	} else {
		return modelInfers, nil
	}
}

// updateModelInfer updates model infer when model changed
func (mc *ModelController) updateModelInfer(ctx context.Context, model *registryv1alpha1.Model) error {
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeActive),
		metav1.ConditionFalse, ModelUpdatingReason, "Model is updating, not ready yet"))
	meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeUpdating),
		metav1.ConditionTrue, ModelUpdatingReason, "Model is updating"))
	if err := mc.updateModelStatus(ctx, model); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}
	modelInfers, err := utils.BuildModelInferCR(model)
	if err != nil {
		return err
	}
	for _, modelInfer := range modelInfers {
		oldModelInfer := &workload.ModelInfer{}
		// Get modelInfer resource version to update it
		if oldModelInfer, err = mc.modelClient.WorkloadV1alpha1().ModelInfers(modelInfer.Namespace).Get(ctx, modelInfer.Name, metav1.GetOptions{}); err != nil {
			return err
		}
		modelInfer.ResourceVersion = oldModelInfer.ResourceVersion
		if _, err := mc.modelClient.WorkloadV1alpha1().ModelInfers(model.Namespace).Update(ctx, modelInfer, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (mc *ModelController) loadConfigFromConfigMap() {
	// todo configmap namespace and name is hard-code
	cm, err := mc.kubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), ConfigMapName, metav1.GetOptions{})
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
