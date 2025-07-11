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
	"fmt"
	"time"

	"istio.io/istio/pkg/util/sets"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	listerv1alpha1 "matrixinfer.ai/matrixinfer/client-go/listers/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/utils"
)

type ModelServerController struct {
	matrixinferclientset versioned.Interface
	kubeclientset        kubernetes.Interface

	modelServerLister listerv1alpha1.ModelServerLister
	podLister         corelisters.PodLister

	modelServerSynced cache.InformerSynced
	podSynced         cache.InformerSynced

	workqueue workqueue.TypedRateLimitingInterface[any]
	store     datastore.Store
}

func NewModelServerController(
	matrixinferInformerFactory informersv1alpha1.SharedInformerFactory,
	kubeInformerFactory informers.SharedInformerFactory,
	store datastore.Store,
) *ModelServerController {
	modelServerInformer := matrixinferInformerFactory.Networking().V1alpha1().ModelServers()
	podInformer := kubeInformerFactory.Core().V1().Pods()

	controller := &ModelServerController{
		modelServerLister: modelServerInformer.Lister(),
		podLister:         podInformer.Lister(),
		modelServerSynced: modelServerInformer.Informer().HasSynced,
		podSynced:         podInformer.Informer().HasSynced,
		workqueue:         workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]()),
		store:             store,
	}

	modelServerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueModelServer,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueModelServer(new)
		},
		DeleteFunc: controller.enqueueModelServer,
	})

	return controller
}

func (c *ModelServerController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.modelServerSynced, c.podSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	return nil
}

func (c *ModelServerController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ModelServerController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	defer c.workqueue.Done(obj)
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		c.workqueue.Forget(obj)
		utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
		return true
	}

	if err := c.syncHandler(key); err != nil {
		if c.workqueue.NumRequeues(key) < maxRetries {
			klog.V(2).Infof("error syncing modelServer %q': %v, requeuing", key, err)
			c.workqueue.AddRateLimited(key)
			return true
		}
		klog.V(2).Infof("giving up on syncing %q after %d retries: %v", key, maxRetries, err)
		c.workqueue.Forget(obj)
	}
	return true
}

func (c *ModelServerController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	ms, err := c.modelServerLister.ModelServers(namespace).Get(name)
	if errors.IsNotFound(err) {
		_ = c.store.DeleteModelServer(ms)
		return nil
	}
	if err != nil {
		return err
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: ms.Spec.WorkloadSelector.MatchLabels})
	if err != nil {
		return fmt.Errorf("invalid selector: %v", err)
	}

	podList, err := c.podLister.Pods(ms.Namespace).List(selector)
	if err != nil {
		return err
	}

	pods := sets.NewWithLength[types.NamespacedName](len(podList))
	for _, pod := range podList {
		if isPodReady(pod) {
			pods.Insert(utils.GetNamespaceName(pod))
		}
	}

	_ = c.store.AddOrUpdateModelServer(ms, pods)

	return nil
}

func (c *ModelServerController) enqueueModelServer(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
