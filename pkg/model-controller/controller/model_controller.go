package controller

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	registryLister "matrixinfer.ai/matrixinfer/client-go/listers/registry/v1alpha1"
	workloadLister "matrixinfer.ai/matrixinfer/client-go/listers/workload/v1alpha1"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	workload "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/datastore"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"
	"sync"
)

const (
	ModelInitsReason = "modelInits"
)

type ModelController struct {
	kubeClientSet kubernetes.Interface
	modelClient   clientset.Interface

	syncHandler    func(ctx context.Context, miKey string) error
	modelsLister   registryLister.ModelLister
	modelsInformer cache.Controller

	modelInfersLister workloadLister.ModelInferLister

	// nolint
	workqueue workqueue.RateLimitingInterface
	store     datastore.Store
	graceMap  sync.Map // key: errorPod.namespace/errorPod.name, value:time
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
	if err := mc.store.DeleteModel(types.NamespacedName{
		Namespace: model.Namespace,
		Name:      model.Name,
	}); err != nil {
		klog.Errorf("failed to delete model %s: %v", model.Name, err)
	}
}

func (mc *ModelController) syncModel(ctx context.Context, key string) error {
	klog.V(4).Info("Started syncing Model")
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", err)
	}
	model, err := mc.modelsLister.Models(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(4).Infof("Model %s does not exist anymore", key)
		return nil
	}
	if err != nil {
		return err
	}
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
	return nil
}

func newCondition(conditionType string, status metav1.ConditionStatus, reason string, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func (mc *ModelController) updateModelStatus(ctx context.Context, model *registryv1alpha1.Model) error {
	modelInferList, err := mc.listModelInferByLabel(model)
	if err != nil {
		return err
	}
	var backendStatus []registryv1alpha1.ModelBackendStatus
	for _, infer := range modelInferList {
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

func NewModelController(kubeClientSet kubernetes.Interface, modelClient clientset.Interface) *ModelController {
	modelInformerFactory := informersv1alpha1.NewSharedInformerFactory(modelClient, 0)
	modelInformer := modelInformerFactory.Registry().V1alpha1().Models()
	modelInferInformer := modelInformerFactory.Workload().V1alpha1().ModelInfers()
	store, err := datastore.New()
	if err != nil {
		klog.Fatal("Unable to create data store")
	}
	mc := &ModelController{
		kubeClientSet:     kubeClientSet,
		modelClient:       modelClient,
		modelsLister:      modelInformer.Lister(),
		modelsInformer:    modelInformer.Informer(),
		modelInfersLister: modelInferInformer.Lister(),
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Models"),
		store:             store,
	}
	klog.Info("Set the Model event handler")
	_, err = modelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mc.createModel(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			mc.updateModel(old, new)
		},
		DeleteFunc: func(obj interface{}) {
			mc.deleteModel(obj)
		},
	})
	if err != nil {
		klog.Fatal("Unable to add model event handler")
		return nil
	}
	mc.syncHandler = mc.syncModel
	return mc
}

// listModelInferByLabel list all model infer which label key is "owner" and label value is model uid
func (mc *ModelController) listModelInferByLabel(model *registryv1alpha1.Model) ([]*workload.ModelInfer, error) {
	selector := labels.SelectorFromSet(map[string]string{
		utils.ModelInferOwnerKey: string(model.UID),
	})
	if modelInfers, err := mc.modelInfersLister.ModelInfers(model.Namespace).List(selector); err != nil {
		return nil, err
	} else {
		return modelInfers, nil
	}
}
