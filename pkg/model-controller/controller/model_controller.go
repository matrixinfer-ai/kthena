package controller

import (
	"context"
	"fmt"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	listerv1alpha1 "matrixinfer.ai/matrixinfer/client-go/listers/registry/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/datastore"
	"sync"
)

type ModelController struct {
	kubeClientSet    kubernetes.Interface
	modelInferClient clientset.Interface

	syncHandler      func(ctx context.Context, miKey string) error
	podsLister       listerv1.PodLister
	podsInformer     cache.Controller
	servicesLister   listerv1.ServiceLister
	servicesInformer cache.Controller
	modelsLister     listerv1alpha1.ModelLister
	modelsInformer   cache.Controller

	// nolint
	workqueue workqueue.RateLimitingInterface
	store     datastore.Store
	graceMap  sync.Map // key: errorPod.namespace/errorPod.name, value:time
}

func (mic *ModelController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer mic.workqueue.ShutDown()

	// start informers
	go mic.podsInformer.RunWithContext(ctx)
	go mic.servicesInformer.RunWithContext(ctx)
	go mic.modelsInformer.RunWithContext(ctx)

	cache.WaitForCacheSync(ctx.Done(),
		mic.podsInformer.HasSynced,
		mic.servicesInformer.HasSynced,
		mic.modelsInformer.HasSynced,
	)

	klog.Info("start model controller")
	for i := 0; i < workers; i++ {
		go mic.worker(ctx)
	}
	<-ctx.Done()
	klog.Info("shut down model controller")
}

func (mic *ModelController) worker(ctx context.Context) {
	for mic.processNextWorkItem(ctx) {
	}
}

func (mic *ModelController) processNextWorkItem(ctx context.Context) bool {
	key, quit := mic.workqueue.Get()
	if quit {
		return false
	}
	defer mic.workqueue.Done(key)

	err := mic.syncHandler(ctx, key.(string))
	if err == nil {
		mic.workqueue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("sync %q failed with %v", key, err))
	mic.workqueue.AddRateLimited(key)

	return true
}

func NewModelController(kubeClientSet kubernetes.Interface, modelClient clientset.Interface) *ModelController {
	// todo
	return nil
}
