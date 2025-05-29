package controller

import (
	"context"
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
)

type ModelInferController struct {
	kubeclientset kubernetes.Interface

	syncHandler         func(ctx context.Context, rsKey string) error
	podsLister          cache.Indexer
	podsInformer        cache.Controller
	modelInfersLister   cache.Indexer
	modelInfersInformer cache.Controller

	workqueue workqueue.RateLimitingInterface
}

func NewModelInferController(kubeclientset kubernetes.Interface, modelInferClient clientset.Interface) *ModelInferController {

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeclientset, 0)
	podsInformer := kubeInformerFactory.Core().V1().Pods()
	modelInferInformerFactory := informersv1alpha1.NewSharedInformerFactory(modelInferClient, 0)
	modelInferInformer := modelInferInformerFactory.Workload().V1alpha1().ModelInfers()

	mic := &ModelInferController{
		kubeclientset:       kubeclientset,
		podsLister:          podsInformer.Informer().GetIndexer(),
		podsInformer:        podsInformer.Informer(),
		modelInfersLister:   modelInferInformer.Informer().GetIndexer(),
		modelInfersInformer: modelInferInformer.Informer(),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ModelInfers"),
	}

	klog.Info("Set the ModelInfer event handler")
	modelInferInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mic.addMI(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			mic.updateMI(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			mic.deleteMI(obj)
		},
	})

	mic.syncHandler = mic.syncModelInfer

	return mic
}

func (mic *ModelInferController) addMI(obj interface{}) {
}

func (mic *ModelInferController) updateMI(old, cur interface{}) {
}

func (mic *ModelInferController) deleteMI(obj interface{}) {
}

func (mic *ModelInferController) enqueueRS(mi *workloadv1alpha1.ModelInfer) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(mi); err != nil {
		utilruntime.HandleError(err)
		return
	}
	mic.workqueue.Add(key)
}

func (mic *ModelInferController) worker(ctx context.Context) {
	for mic.processNextWorkItem(ctx) {
	}
}

func (mic *ModelInferController) processNextWorkItem(ctx context.Context) bool {
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

func (mic *ModelInferController) syncModelInfer(ctx context.Context, key string) error {
	// todo add modelinfer handle logic
	return nil
}

func (mic *ModelInferController) Run() {
	defer utilruntime.HandleCrash()
	defer mic.workqueue.ShutDown()

	klog.Info("start modelInfer controller")
	// todo add controller logic

}

// UpdateStatus update ModelInfer status.
func (mic *ModelInferController) UpdateStatus() {
}
