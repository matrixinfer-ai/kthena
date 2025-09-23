/*
Copyright The Volcano Authors.

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
	"net/http"
	"time"

	networkingv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/networking/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	informersv1alpha1 "github.com/volcano-sh/kthena/client-go/informers/externalversions"
	networkingLister "github.com/volcano-sh/kthena/client-go/listers/networking/v1alpha1"
	registryLister "github.com/volcano-sh/kthena/client-go/listers/registry/v1alpha1"
	workloadLister "github.com/volcano-sh/kthena/client-go/listers/workload/v1alpha1"
	registryv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/registry/v1alpha1"
	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/model-controller/config"
	"github.com/volcano-sh/kthena/pkg/model-controller/utils"
)

const (
	ConfigMapName = "model-controller-config"
)

type ModelController struct {
	// Client for k8s. Use it to call K8S API
	kubeClient kubernetes.Interface
	// client for custom resource
	client clientset.Interface
	// httpClient for HTTP requests to LoRA adapter APIs
	httpClient *http.Client

	syncHandler                       func(ctx context.Context, miKey string) error
	modelsLister                      registryLister.ModelLister
	modelsInformer                    cache.Controller
	modelInfersLister                 workloadLister.ModelServingLister
	modelInfersInformer               cache.SharedIndexInformer
	modelServersLister                networkingLister.ModelServerLister
	modelServersInformer              cache.SharedIndexInformer
	modelRoutesLister                 networkingLister.ModelRouteLister
	modelRoutesInformer               cache.SharedIndexInformer
	autoscalingPoliciesLister         registryLister.AutoscalingPolicyLister
	autoscalingPoliciesInformer       cache.SharedIndexInformer
	autoscalingPolicyBindingsLister   registryLister.AutoscalingPolicyBindingLister
	autoscalingPolicyBindingsInformer cache.SharedIndexInformer
	podsLister                        listerv1.PodLister
	podsInformer                      cache.SharedIndexInformer
	kubeInformerFactory               informers.SharedInformerFactory
	workQueue                         workqueue.TypedRateLimitingInterface[any]
	// loraUpdateCache stores the previous model version for LoRA adapter comparison
	// Key format: "namespace/name:generation" to avoid version conflicts
	loraUpdateCache map[string]*registryv1alpha1.Model
}

func (mc *ModelController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer mc.workQueue.ShutDown()

	// start informers
	go mc.modelsInformer.RunWithContext(ctx)
	go mc.modelInfersInformer.RunWithContext(ctx)
	go mc.autoscalingPoliciesInformer.RunWithContext(ctx)
	go mc.autoscalingPolicyBindingsInformer.RunWithContext(ctx)
	go mc.podsInformer.RunWithContext(ctx)
	go mc.modelServersInformer.RunWithContext(ctx)
	go mc.modelRoutesInformer.RunWithContext(ctx)

	// start Kubernetes informer factory
	go mc.kubeInformerFactory.Start(ctx.Done())

	cache.WaitForCacheSync(ctx.Done(),
		mc.modelsInformer.HasSynced,
		mc.modelInfersInformer.HasSynced,
		mc.autoscalingPoliciesInformer.HasSynced,
		mc.autoscalingPolicyBindingsInformer.HasSynced,
		mc.podsInformer.HasSynced,
		mc.modelServersInformer.HasSynced,
		mc.modelRoutesInformer.HasSynced,
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

func (mc *ModelController) createModel(obj any) {
	model, ok := obj.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse Model when createModel")
		return
	}
	klog.V(4).Infof("Create model: %s", klog.KObj(model))
	mc.enqueueModel(model)
}

func (mc *ModelController) enqueueModel(model *registryv1alpha1.Model) {
	if key, err := cache.MetaNamespaceKeyFunc(model); err != nil {
		utilruntime.HandleError(err)
	} else {
		mc.workQueue.Add(key)
	}
}

func (mc *ModelController) updateModel(old any, new any) {
	newModel, ok := new.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse new Model when updateModel")
		return
	}
	oldModel, ok := old.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse old Model when updateModel")
		return
	}

	// When observed generation not equal to generation, reconcile model
	if oldModel.Status.ObservedGeneration != newModel.Generation {
		// Store the old model in cache with generation-specific key to avoid conflicts
		cacheKey := fmt.Sprintf("%s/%s:%d", newModel.Namespace, newModel.Name, newModel.Generation)
		mc.loraUpdateCache[cacheKey] = oldModel.DeepCopy()

		mc.enqueueModel(newModel)
	}
}

func (mc *ModelController) deleteModel(obj any) {
	model, ok := obj.(*registryv1alpha1.Model)
	if !ok {
		klog.Error("failed to parse Model when deleteModel")
		return
	}
	klog.V(4).Infof("Delete model: %s", klog.KObj(model))
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
		if err := mc.setModelInitCondition(ctx, model); err != nil {
			return err
		}
	}

	// Track backends that have been dynamically updated
	var dynamicUpdatedBackends []string
	// check if only LoRA adapters have changed for runtime update
	if oldModel, err := mc.getPreviousModelVersion(model); err == nil && oldModel != nil && mc.hasOnlyLoraAdaptersChanged(oldModel, model) {
		klog.Info("model generation is not equal to observed generation, checking for LoRA adapter changes")

		if oldModel, err := mc.getPreviousModelVersion(model); err == nil && oldModel != nil {
			dynamicBackends := mc.getDynamicLoraUpdateBackends(oldModel, model)
			if len(dynamicBackends) > 0 {
				klog.Infof("Detected LoRA adapter changes for backends: %v, attempting runtime update", dynamicBackends)
				successUpdatedBackends := mc.handleDynamicLoraUpdates(oldModel, model, dynamicBackends)
				klog.Infof("Dynamic LoRA updates completed successfully for backends: %v", successUpdatedBackends)
				dynamicUpdatedBackends = successUpdatedBackends
			}
		}
	}
	if err := mc.setModelProcessingCondition(ctx, model); err != nil {
		return err
	}
	if err := mc.createOrUpdateModelInfer(ctx, model, dynamicUpdatedBackends); err != nil {
		mc.setModelFailedCondition(ctx, model, err)
		return err
	}
	if err := mc.createOrUpdateModelServer(ctx, model); err != nil {
		mc.setModelFailedCondition(ctx, model, err)
		return err
	}
	if err := mc.createOrUpdateModelRoute(ctx, model); err != nil {
		mc.setModelFailedCondition(ctx, model, err)
		return err
	}
	if err := mc.createOrUpdateAutoscalingPolicyAndBinding(ctx, model); err != nil {
		mc.setModelFailedCondition(ctx, model, err)
		return err
	}
	modelInferActive, err := mc.isModelInferActive(model)
	if err != nil || !modelInferActive {
		return err
	}
	if err := mc.setModelActiveCondition(ctx, model); err != nil {
		return err
	}

	return nil
}

// isModelInferActive returns true if all Model Infers are available.
func (mc *ModelController) isModelInferActive(model *registryv1alpha1.Model) (bool, error) {
	// List all Model Infers associated with the model
	modelInfers, err := mc.listModelInferByLabel(model)
	if err != nil {
		return false, err
	}
	// Ensure the number of Model Infers matches the number of backends
	if len(modelInfers) != len(model.Spec.Backends) {
		klog.Infof("Number of Model Infer: %d, number of backends: %d", len(modelInfers), len(model.Spec.Backends))
		return false, fmt.Errorf("model infer number not equal to backend number")
	}
	// Check if all Model Infers are available
	for _, modelInfer := range modelInfers {
		if !meta.IsStatusConditionPresentAndEqual(modelInfer.Status.Conditions, string(workload.ModelServingAvailable), metav1.ConditionTrue) {
			// requeue until all Model Infers are active
			klog.InfoS("model infer is not available", "model infer", klog.KObj(modelInfer))
			return false, nil
		}
	}
	return true, nil
}

// updateModelStatus updates model status.
func (mc *ModelController) updateModelStatus(ctx context.Context, model *registryv1alpha1.Model) error {
	modelInfers, err := mc.listModelInferByLabel(model)
	if err != nil {
		return err
	}
	var backendStatus []registryv1alpha1.ModelBackendStatus
	for _, infer := range modelInfers {
		backendStatus = append(backendStatus, registryv1alpha1.ModelBackendStatus{
			Name:     infer.Name,
			Replicas: infer.Status.Replicas,
		})
	}
	model.Status.BackendStatuses = backendStatus
	model.Status.ObservedGeneration = model.Generation
	if _, err := mc.client.RegistryV1alpha1().Models(model.Namespace).UpdateStatus(ctx, model, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("update model status failed: %v", err)
		return err
	}

	// Clean up outdated cache entries for this model
	mc.cleanupOutdatedLoraUpdateCache(model)
	return nil
}

func NewModelController(kubeClient kubernetes.Interface, client clientset.Interface) *ModelController {
	selector, err := labels.NewRequirement(utils.ManageBy, selection.Equals, []string{registryv1alpha1.GroupName})
	if err != nil {
		klog.Errorf("cannot create label selector, err: %v", err)
		return nil
	}

	filterInformerFactory := informersv1alpha1.NewSharedInformerFactoryWithOptions(
		client,
		0,
		informersv1alpha1.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector.String()
		}),
	)

	informerFactory := informersv1alpha1.NewSharedInformerFactory(client, 0)
	modelInformer := informerFactory.Registry().V1alpha1().Models()
	modelInferInformer := filterInformerFactory.Workload().V1alpha1().ModelServings()
	modelServerInformer := filterInformerFactory.Networking().V1alpha1().ModelServers()
	modelRouteInformer := filterInformerFactory.Networking().V1alpha1().ModelRoutes()
	autoscalingPoliciesInformer := filterInformerFactory.Registry().V1alpha1().AutoscalingPolicies()
	autoscalingPolicyBindingsInformer := filterInformerFactory.Registry().V1alpha1().AutoscalingPolicyBindings()

	// Initialize Kubernetes informer factory for pods
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	podsInformer := kubeInformerFactory.Core().V1().Pods().Informer()
	podsLister := kubeInformerFactory.Core().V1().Pods().Lister()

	// Create a shared HTTP client for LoRA adapter API calls
	// This client will be reused across all HTTP requests, enabling connection pooling
	httpClient := &http.Client{
		Timeout: 5 * time.Minute, // Default timeout for LoRA adapter operations
		Transport: &http.Transport{
			MaxIdleConns:        100,              // Maximum number of idle connections
			MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
			IdleConnTimeout:     90 * time.Second, // How long an idle connection is kept
		},
	}

	mc := &ModelController{
		kubeClient:                        kubeClient,
		client:                            client,
		httpClient:                        httpClient,
		modelsLister:                      modelInformer.Lister(),
		modelsInformer:                    modelInformer.Informer(),
		modelInfersLister:                 modelInferInformer.Lister(),
		modelInfersInformer:               modelInferInformer.Informer(),
		modelServersLister:                modelServerInformer.Lister(),
		modelServersInformer:              modelServerInformer.Informer(),
		modelRoutesLister:                 modelRouteInformer.Lister(),
		modelRoutesInformer:               modelRouteInformer.Informer(),
		autoscalingPoliciesLister:         autoscalingPoliciesInformer.Lister(),
		autoscalingPoliciesInformer:       autoscalingPoliciesInformer.Informer(),
		autoscalingPolicyBindingsLister:   autoscalingPolicyBindingsInformer.Lister(),
		autoscalingPolicyBindingsInformer: autoscalingPolicyBindingsInformer.Informer(),
		podsLister:                        podsLister,
		podsInformer:                      podsInformer,
		kubeInformerFactory:               kubeInformerFactory,
		loraUpdateCache:                   make(map[string]*registryv1alpha1.Model),

		workQueue: workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{}),
	}
	klog.Info("Set the Model event handler")
	_, err = modelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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
		DeleteFunc: mc.deleteModelInfer,
	})
	if err != nil {
		klog.Fatal("Unable to add model infer event handler")
		return nil
	}
	_, err = modelRouteInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: mc.deleteModelRoute,
	})
	if err != nil {
		klog.Fatal("Unable to add model route event handler")
		return nil
	}
	_, err = modelServerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: mc.deleteModelServer,
	})
	if err != nil {
		klog.Fatal("Unable to add model server event handler")
		return nil
	}
	mc.syncHandler = mc.reconcile
	mc.loadConfigFromConfigMap()
	return mc
}

func (mc *ModelController) loadConfigFromConfigMap() {
	namespace, err := utils.GetInClusterNameSpace()
	// When run locally, namespace will be empty, default value of downloader image and runtime image will be used.
	// So we don't need to read ConfigMap in this case.
	if len(namespace) == 0 {
		klog.Warning(err)
		return
	}
	cm, err := mc.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), ConfigMapName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("ConfigMap does not exist. Error: %v", err)
		return
	}
	if modelInferDownloaderImage, ok := cm.Data["model_infer_downloader_image"]; ok {
		config.Config.SetModelInferDownloaderImage(modelInferDownloaderImage)
	} else {
		klog.Warning("Failed to load model infer Downloader Image. Use Default Value.")
	}
	if modelInferRuntimeImage, ok := cm.Data["model_infer_runtime_image"]; ok {
		config.Config.SetModelInferRuntimeImage(modelInferRuntimeImage)
	} else {
		klog.Warning("Failed to load model infer Runtime Image. Use Default Value.")
	}
}

// When model infer status changed, model reconciles
func (mc *ModelController) triggerModel(old any, new any) {
	newModelInfer, ok := new.(*workload.ModelServing)
	if !ok {
		klog.Error("failed to parse new ModelInfer")
		return
	}
	_, ok = old.(*workload.ModelServing)
	if !ok {
		klog.Error("failed to parse old ModelInfer")
		return
	}
	if len(newModelInfer.OwnerReferences) > 0 {
		// Find the owner of modelInfer and reconcile the owner to change its status
		if model, err := mc.modelsLister.Models(newModelInfer.Namespace).Get(newModelInfer.OwnerReferences[0].Name); err == nil {
			mc.enqueueModel(model)
		}
	}
}

// deleteModelInfer is called when a ModelInfer is deleted. It will reconcile the Model. Recreate model infer.
func (mc *ModelController) deleteModelInfer(obj any) {
	modelInfer, ok := obj.(*workload.ModelServing)
	if !ok {
		klog.Error("failed to parse ModelInfer when deleteModelInfer")
		return
	}
	klog.V(4).Infof("model infer: %s is deleted", klog.KObj(modelInfer))
	if len(modelInfer.OwnerReferences) > 0 {
		if model, err := mc.modelsLister.Models(modelInfer.Namespace).Get(modelInfer.OwnerReferences[0].Name); err == nil {
			mc.enqueueModel(model)
		}
	}
}

// deleteModelRoute is called when a ModelRoute is deleted. It will reconcile the Model. Recreate model route.
func (mc *ModelController) deleteModelRoute(obj any) {
	modelRoute, ok := obj.(*networkingv1alpha1.ModelRoute)
	if !ok {
		klog.Error("failed to parse ModelRoute when deleteModelRoute")
		return
	}
	klog.V(4).Infof("model route: %s is deleted", klog.KObj(modelRoute))
	if len(modelRoute.OwnerReferences) > 0 {
		if model, err := mc.modelsLister.Models(modelRoute.Namespace).Get(modelRoute.OwnerReferences[0].Name); err == nil {
			mc.enqueueModel(model)
		}
	}
}

// deleteModelServer is called when a ModelServer is deleted. It will reconcile the Model. Recreate model server.
func (mc *ModelController) deleteModelServer(obj any) {
	modelServer, ok := obj.(*networkingv1alpha1.ModelServer)
	if !ok {
		klog.Error("failed to parse ModelServer when deleteModelServer")
		return
	}
	klog.V(4).Infof("model server: %s is deleted", klog.KObj(modelServer))
	if len(modelServer.OwnerReferences) > 0 {
		if model, err := mc.modelsLister.Models(modelServer.Namespace).Get(modelServer.OwnerReferences[0].Name); err == nil {
			mc.enqueueModel(model)
		}
	}
}
