package controller

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	listerv1alpha1 "matrixinfer.ai/matrixinfer/client-go/listers/registry/v1alpha1"
	registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/datastore"
	"sync"
)

type ModelController struct {
	kubeClientSet kubernetes.Interface
	modelClient   clientset.Interface

	syncHandler    func(ctx context.Context, miKey string) error
	modelsLister   listerv1alpha1.ModelLister
	modelsInformer cache.Controller

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
	if err := mc.store.DeleteModel(types.NamespacedName{
		Namespace: model.Namespace,
		Name:      model.Name,
	}); err != nil {
		klog.Errorf("failed to delete model %s: %v", model.Name, err)
	}
}

func NewModelController(kubeClientSet kubernetes.Interface, modelClient clientset.Interface) *ModelController {
	modelInformerFactory := informersv1alpha1.NewSharedInformerFactory(modelClient, 0)
	modelInformer := modelInformerFactory.Registry().V1alpha1().Models()
	store, err := datastore.New()
	if err != nil {
		klog.Fatal("Unable to create data store")
	}
	mc := &ModelController{
		kubeClientSet:  kubeClientSet,
		modelClient:    modelClient,
		modelsLister:   modelInformer.Lister(),
		modelsInformer: modelInformer.Informer(),
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Models"),
		store:          store,
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
	return mc
}
