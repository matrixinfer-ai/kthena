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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	icUtils "matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/convert"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/utils"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ModelInitsReason    = "ModelInits"
	ModelUpdatingReason = "ModelUpdating"
	ModelActiveReason   = "ModelActive"
	ConfigMapName       = "model-controller-config"
)

type ModelController struct {
	// Client for k8s. Use it to call K8S API
	kubeClient kubernetes.Interface
	// client for custom resource
	client clientset.Interface

	syncHandler                       func(ctx context.Context, miKey string) error
	modelsLister                      registryLister.ModelLister
	modelsInformer                    cache.Controller
	modelInfersLister                 workloadLister.ModelInferLister
	modelInfersInformer               cache.SharedIndexInformer
	autoscalingPoliciesLister         registryLister.AutoscalingPolicyLister
	autoscalingPoliciesInformer       cache.SharedIndexInformer
	autoscalingPolicyBindingsLister   registryLister.AutoscalingPolicyBindingLister
	autoscalingPolicyBindingsInformer cache.SharedIndexInformer
	workQueue                         workqueue.TypedRateLimitingInterface[any]
}

func (mc *ModelController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer mc.workQueue.ShutDown()

	// start informers
	go mc.modelsInformer.RunWithContext(ctx)
	go mc.modelInfersInformer.RunWithContext(ctx)
	go mc.autoscalingPoliciesInformer.RunWithContext(ctx)
	go mc.autoscalingPolicyBindingsInformer.RunWithContext(ctx)

	cache.WaitForCacheSync(ctx.Done(),
		mc.modelsInformer.HasSynced,
		mc.modelInfersInformer.HasSynced,
		mc.autoscalingPoliciesInformer.HasSynced,
		mc.autoscalingPolicyBindingsInformer.HasSynced,
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

func (mc *ModelController) updateModel(old any, new any) {
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

func (mc *ModelController) deleteModel(obj any) {
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
	// TODO: Expect no distinction between create phase and update phase
	klog.InfoS("Start to process model", "namespace", namespace, "model name", model.Name, "model status", model.Status)
	if len(model.Status.Conditions) == 0 {
		if err := mc.createModelInfer(ctx, model); err != nil {
			return err
		}
		if err := mc.createModelServer(ctx, model); err != nil {
			return err
		}
		if err := mc.createModelRoute(ctx, model); err != nil {
			return err
		}
		if err := mc.createAutoScalingPolicyAndBinding(ctx, model); err != nil {
			return err
		}
		meta.SetStatusCondition(&model.Status.Conditions, newCondition(string(registryv1alpha1.ModelStatusConditionTypeInitializing),
			metav1.ConditionTrue, ModelInitsReason, "Model is initializing"))
		if err := mc.updateModelStatus(ctx, model); err != nil {
			klog.Errorf("update model status failed: %v", err)
			return err
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
		if err := mc.updateModelRoute(ctx, model); err != nil {
			return err
		}
		if err := mc.updateAutoscalingPolicyAndBinding(ctx, model); err != nil {
			return err
		}
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
		return false, fmt.Errorf("model infer number not equal to backend number")
	}
	// Check if all Model Infers are available
	for _, modelInfer := range modelInfers {
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
	modelInfers, err := mc.listModelInferByLabel(model)
	if err != nil {
		return err
	}
	var backendStatus []registryv1alpha1.ModelBackendStatus
	for _, infer := range modelInfers {
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
	selector, err := labels.NewRequirement(registryv1alpha1.ManageBy, selection.Equals, []string{registryv1alpha1.GroupName})
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
	modelInferInformer := filterInformerFactory.Workload().V1alpha1().ModelInfers()
	autoscalingPoliciesInformer := filterInformerFactory.Registry().V1alpha1().AutoscalingPolicies()
	autoscalingPolicyBindingsInformer := filterInformerFactory.Registry().V1alpha1().AutoscalingPolicyBindings()
	mc := &ModelController{
		kubeClient:                        kubeClient,
		client:                            client,
		modelsLister:                      modelInformer.Lister(),
		modelsInformer:                    modelInformer.Informer(),
		modelInfersLister:                 modelInferInformer.Lister(),
		modelInfersInformer:               modelInferInformer.Informer(),
		autoscalingPoliciesLister:         autoscalingPoliciesInformer.Lister(),
		autoscalingPoliciesInformer:       autoscalingPoliciesInformer.Informer(),
		autoscalingPolicyBindingsLister:   autoscalingPolicyBindingsInformer.Lister(),
		autoscalingPolicyBindingsInformer: autoscalingPolicyBindingsInformer.Informer(),

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
func (mc *ModelController) listModelInferByLabel(model *registryv1alpha1.Model) ([]*workload.ModelInfer, error) {
	if modelInfers, err := mc.modelInfersLister.ModelInfers(model.Namespace).List(labels.SelectorFromSet(map[string]string{
		convert.ModelInferOwnerKey: string(model.UID),
	})); err != nil {
		return nil, err
	} else {
		return modelInfers, nil
	}
}

// updateModelInfer updates model infer when model changed
func (mc *ModelController) updateModelInfer(ctx context.Context, model *registryv1alpha1.Model) error {
	modelInfers, err := convert.CreateModelInferResources(model)
	if err != nil {
		return err
	}
	for _, modelInfer := range modelInfers {
		oldModelInfer, err := mc.modelInfersLister.ModelInfers(modelInfer.Namespace).Get(modelInfer.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
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
func (mc *ModelController) triggerModel(old any, new any) {
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

// createModelServer creates model server
func (mc *ModelController) createModelServer(ctx context.Context, model *registryv1alpha1.Model) error {
	klog.V(4).Info("Start to create model server")
	modelServers, err := convert.BuildModelServer(model)
	if err != nil {
		return err
	}
	for _, modelServer := range modelServers {
		if _, err := mc.client.NetworkingV1alpha1().ModelServers(model.Namespace).Create(ctx, modelServer, metav1.CreateOptions{}); err != nil {
			if apierrors.IsAlreadyExists(err) {
				klog.V(4).InfoS("ModelServer already exists, skipping creation", "modelServer", klog.KObj(modelServer))
				continue
			}
			klog.Errorf("Create model server failed: %v", err)
			return err
		}
	}
	return nil
}

// updateModelServer updates model server
func (mc *ModelController) updateModelServer(ctx context.Context, model *registryv1alpha1.Model) error {
	modelServers, err := convert.BuildModelServer(model)
	if err != nil {
		return err
	}
	for _, modelServer := range modelServers {
		oldModelServer, err := mc.client.NetworkingV1alpha1().ModelServers(modelServer.Namespace).Get(ctx, modelServer.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
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

func (mc *ModelController) createAutoscalingPolicyAndBinding(ctx context.Context, model *registryv1alpha1.Model) error {
	if model.Spec.AutoscalingPolicy != nil {
		// Create autoscaling policy and optimize policy binding
		modelAutoscalePolicy := convert.BuildAutoscalingPolicy(model.Spec.AutoscalingPolicy, model, "")
		if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicies(model.Namespace).Create(ctx, modelAutoscalePolicy, metav1.CreateOptions{}); err != nil {
			klog.Errorf("Create autoscaling policy of model: [%s] failed: %v", model.Name, err)
			return err
		}
		modelPolicyBinding := convert.BuildOptimizePolicyBinding(model, utils.GetBackendResourceName(model.Name, ""))
		if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicyBindings(model.Namespace).Create(ctx, modelPolicyBinding, metav1.CreateOptions{}); err != nil {
			klog.Errorf("Create autoscaling policy binding of model: [%s] failed: %v", model.Name, err)
			return err
		}
	} else {
		// Create autoscaling policy and scaling policy binding
		for _, backend := range model.Spec.Backends {
			if backend.AutoscalingPolicy == nil {
				continue
			}
			backendAutoscalePolicy := convert.BuildAutoscalingPolicy(backend.AutoscalingPolicy, model, backend.Name)
			if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicies(model.Namespace).Create(ctx, backendAutoscalePolicy, metav1.CreateOptions{}); err != nil {
				klog.Errorf("Create autoscaling policy of backend: [%s] in model: [%s] failed: %v", backend.Name, model.Name, err)
				return err
			}
			backendPolicyBinding := convert.BuildScalingPolicyBinding(model, &backend, utils.GetBackendResourceName(model.Name, backend.Name))
			if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicyBindings(model.Namespace).Create(ctx, backendPolicyBinding, metav1.CreateOptions{}); err != nil {
				klog.Errorf("Create autoscaling policy binding of backend: [%s] in model: [%s] failed: %v", backend.Name, model.Name, err)
				return err
			}
		}
	}
	return nil
}

func (mc *ModelController) updateAutoscalingPolicyAndBinding(ctx context.Context, model *registryv1alpha1.Model) error {
	if model.Spec.AutoscalingPolicy != nil {
		name := utils.GetBackendResourceName(model.Name, "")
		err := mc.tryUpdateAutoscalingPolicy(ctx, model, model.Spec.AutoscalingPolicy, name)
		if err != nil {
			return err
		}
		err = mc.tryUpdateAutoscalingPolicyBinding(ctx, model, nil)
		if err != nil {
			return err
		}
	} else {
		for _, backend := range model.Spec.Backends {
			if backend.AutoscalingPolicy == nil {
				continue
			}
			err := mc.tryUpdateAutoscalingPolicy(ctx, model, backend.AutoscalingPolicy, utils.GetBackendResourceName(model.Name, backend.Name))
			if err != nil {
				return err
			}
			err = mc.tryUpdateAutoscalingPolicyBinding(ctx, model, &backend)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (mc *ModelController) tryUpdateAutoscalingPolicyBinding(ctx context.Context, model *registryv1alpha1.Model, backend *registryv1alpha1.ModelBackend) error {
	var targetAutoscalePolicyBinding *registryv1alpha1.AutoscalingPolicyBinding
	if backend == nil {
		targetAutoscalePolicyBinding = convert.BuildOptimizePolicyBinding(model, utils.GetBackendResourceName(model.Name, ""))
	} else {
		targetAutoscalePolicyBinding = convert.BuildOptimizePolicyBinding(model, utils.GetBackendResourceName(model.Name, backend.Name))
	}
	currentAutoscalePolicyBinding, err := mc.autoscalingPolicyBindingsLister.AutoscalingPolicyBindings(model.Namespace).Get(targetAutoscalePolicyBinding.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicyBindings(model.Namespace).Create(ctx, targetAutoscalePolicyBinding, metav1.CreateOptions{}); err != nil {
				klog.Errorf("Create autoscaling policy binding of model: [%s] failed: %v", model.Name, err)
				return err
			}
			return nil
		}
		klog.Errorf("Failed to get autoscaling policy binding of model: [%s] failed: %v", model.Name, err)
		return err
	}
	if utils.GetAutoscalingPolicyBindingRevision(currentAutoscalePolicyBinding) == icUtils.Revision(targetAutoscalePolicyBinding.Spec) {
		klog.InfoS("Autoscaling policy binding [%s] of model: [%s] need not to update", currentAutoscalePolicyBinding.Name, model.Name)
		return nil
	}

	_, err = mc.client.RegistryV1alpha1().AutoscalingPolicyBindings(model.Namespace).Update(ctx, targetAutoscalePolicyBinding, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update autoscaling policy binding of model: [%s] failed: %v", model.Name, err)
		return err
	}
	return nil
}

func (mc *ModelController) tryUpdateAutoscalingPolicy(ctx context.Context, model *registryv1alpha1.Model, policy *registryv1alpha1.AutoscalingPolicyConfig, policyName string) error {
	targetAutoscalePolicy := convert.BuildAutoscalingPolicy(policy, model, policyName)
	currentAutoscalePolicy, err := mc.autoscalingPoliciesLister.AutoscalingPolicies(model.Namespace).Get(policyName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := mc.client.RegistryV1alpha1().AutoscalingPolicies(model.Namespace).Create(ctx, targetAutoscalePolicy, metav1.CreateOptions{}); err != nil {
				klog.Errorf("Create autoscaling policy of model: [%s] failed: %v", model.Name, err)
				return err
			}
			return nil
		}
		klog.Errorf("Failed to get autoscaling policy of model: [%s] failed: %v", model.Name, err)
		return err
	}
	if utils.GetAutoscalingPolicyRevision(currentAutoscalePolicy) == icUtils.Revision(targetAutoscalePolicy.Spec) {
		klog.InfoS("Autoscaling policy [%s] of model: [%s] need not to update", currentAutoscalePolicy.Name, model.Name)
		return nil
	}

	_, err = mc.client.RegistryV1alpha1().AutoscalingPolicies(model.Namespace).Update(ctx, targetAutoscalePolicy, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update autoscaling policy of model: [%s] failed: %v", model.Name, err)
		return err
	}
	return nil
}

// createModelRoute creates model route
func (mc *ModelController) createModelRoute(ctx context.Context, model *registryv1alpha1.Model) error {
	klog.V(4).Info("Start to create model route")
	modelRoute := utils.BuildModelRoute(model)
	if _, err := mc.client.NetworkingV1alpha1().ModelRoutes(model.Namespace).Create(ctx, modelRoute, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			klog.V(4).InfoS("ModelRoute already exists, skipping creation", "modelRoute", klog.KObj(modelRoute))
			return nil
		}
		klog.Errorf("create model route failed: %v", err)
		return err
	}
	return nil
}

// updateModelRoute updates model route
func (mc *ModelController) updateModelRoute(ctx context.Context, model *registryv1alpha1.Model) error {
	modelRoute := utils.BuildModelRoute(model)
	oldModelRoute, err := mc.client.NetworkingV1alpha1().ModelRoutes(modelRoute.Namespace).Get(ctx, modelRoute.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ModelRoute doesn't exist, create it.
			if _, err := mc.client.NetworkingV1alpha1().ModelRoutes(model.Namespace).Create(ctx, modelRoute, metav1.CreateOptions{}); err != nil {
				klog.Errorf("failed to create ModelRoute %s: %v", klog.KObj(modelRoute), err)
				return err
			}
			return nil
		}
		klog.Errorf("failed to get ModelRoute %s: %v", klog.KObj(modelRoute), err)
		return err
	}
	if equality.Semantic.DeepEqual(oldModelRoute.Spec, modelRoute.Spec) {
		return nil
	}
	modelRoute.ResourceVersion = oldModelRoute.ResourceVersion
	if _, err := mc.client.NetworkingV1alpha1().ModelRoutes(model.Namespace).Update(ctx, modelRoute, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("failed to update ModelRoute %s: %v", klog.KObj(modelRoute), err)
		return err
	}
	return nil
}

// createModelInfer creates model infer
func (mc *ModelController) createModelInfer(ctx context.Context, model *registryv1alpha1.Model) error {
	klog.V(4).Info("start to create model infer")
	modelInfers, err := utils.BuildModelInferCR(model)
	if err != nil {
		klog.Errorf("failed to build model infer for model %s: %v", model.Name, err)
		return err
	}
	for _, modelInfer := range modelInfers {
		if _, err := mc.client.WorkloadV1alpha1().ModelInfers(model.Namespace).Create(ctx, modelInfer, metav1.CreateOptions{}); err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			return err
		}
	}
	return nil
}
