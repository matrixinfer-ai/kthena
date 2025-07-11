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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	listerv1alpha1 "matrixinfer.ai/matrixinfer/client-go/listers/networking/v1alpha1"
	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type PodController struct {
	podLister         corelisters.PodLister
	modelServerLister listerv1alpha1.ModelServerLister

	podSynced cache.InformerSynced

	workqueue workqueue.TypedRateLimitingInterface[any]
	store     datastore.Store
}

func NewPodController(
	kubeInformerFactory informers.SharedInformerFactory,
	matrixinferInformerFactory informersv1alpha1.SharedInformerFactory,
	store datastore.Store,
) *PodController {
	podInformer := kubeInformerFactory.Core().V1().Pods()
	modelServerInformer := matrixinferInformerFactory.Networking().V1alpha1().ModelServers()

	controller := &PodController{
		podLister:         podInformer.Lister(),
		modelServerLister: modelServerInformer.Lister(),
		podSynced:         podInformer.Informer().HasSynced,
		workqueue:         workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]()),
		store:             store,
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePod,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueuePod(new)
		},
		DeleteFunc: controller.enqueuePod,
	})

	return controller
}

func (c *PodController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.podSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	return nil
}

func (c *PodController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *PodController) processNextWorkItem() bool {
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
			klog.V(2).Infof("error syncing pod %q: %v, requeuing", key, err)
			c.workqueue.AddRateLimited(key)
			return true
		}
		klog.V(2).Infof("giving up on syncing pod %q after %d retries: %v", key, maxRetries, err)
		c.workqueue.Forget(obj)
	}
	return true
}

func (c *PodController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	pod, err := c.podLister.Pods(namespace).Get(name)
	if errors.IsNotFound(err) {
		_ = c.store.DeletePod(types.NamespacedName{Namespace: namespace, Name: name})
		return nil
	}
	if err != nil {
		return err
	}

	if !isPodReady(pod) {
		_ = c.store.DeletePod(types.NamespacedName{Namespace: namespace, Name: name})
		return nil
	}

	modelServers, err := c.modelServerLister.ModelServers(pod.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}

	servers := []*aiv1alpha1.ModelServer{}
	for _, item := range modelServers {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: item.Spec.WorkloadSelector.MatchLabels})
		if err != nil || !selector.Matches(labels.Set(pod.Labels)) {
			continue
		}
		servers = append(servers, item)
	}

	if len(servers) == 0 {
		return nil
	}

	if err := c.store.AddOrUpdatePod(pod, servers); err != nil {
		return fmt.Errorf("failed to add or update pod in data store: %v", name)
	}

	return nil
}

func (c *PodController) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func isPodReady(pod *corev1.Pod) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			if condition.Status == corev1.ConditionTrue {
				return true
			}
			break
		}
	}
	return false
}
