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
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	volcano "volcano.sh/apis/pkg/client/clientset/versioned"

	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	listerv1alpha1 "matrixinfer.ai/matrixinfer/client-go/listers/workload/v1alpha1"
	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/gangscheduling"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

const (
	GroupNameKey = "GroupName"
	RoleIDKey    = "RoleID"
)

type ModelInferController struct {
	kubeClientSet    kubernetes.Interface
	modelInferClient clientset.Interface

	syncHandler         func(ctx context.Context, miKey string) error
	gangManager         gangscheduling.Manager
	podsLister          listerv1.PodLister
	podsInformer        cache.SharedIndexInformer
	servicesLister      listerv1.ServiceLister
	servicesInformer    cache.SharedIndexInformer
	modelInfersLister   listerv1alpha1.ModelInferLister
	modelInfersInformer cache.SharedIndexInformer

	// nolint
	workqueue   workqueue.RateLimitingInterface
	store       datastore.Store
	graceMap    sync.Map // key: errorPod.namespace/errorPod.name, value:time
	initialSync bool     // indicates whether the initial sync has been completed
}

func NewModelInferController(kubeClientSet kubernetes.Interface, modelInferClient clientset.Interface, volcanoClient volcano.Interface) (*ModelInferController, error) {
	selector, err := labels.NewRequirement(workloadv1alpha1.GroupNameLabelKey, selection.Exists, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create label selector, err: %v", err)
	}

	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClientSet,
		0,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector.String()
		}),
	)
	podsInformer := kubeInformerFactory.Core().V1().Pods()
	servicesInformer := kubeInformerFactory.Core().V1().Services()
	modelInferInformerFactory := informersv1alpha1.NewSharedInformerFactory(modelInferClient, 0)
	modelInferInformer := modelInferInformerFactory.Workload().V1alpha1().ModelInfers()

	err = podsInformer.Informer().AddIndexers(cache.Indexers{
		GroupNameKey: utils.GroupNameIndexFunc,
		RoleIDKey:    utils.RoleIDIndexFunc,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create pod Informer Index, err: %v", err)
	}

	err = servicesInformer.Informer().AddIndexers(cache.Indexers{
		GroupNameKey: utils.GroupNameIndexFunc,
		RoleIDKey:    utils.RoleIDIndexFunc,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create service Informer Index, err: %v", err)
	}

	store := datastore.New()

	c := &ModelInferController{
		kubeClientSet:       kubeClientSet,
		modelInferClient:    modelInferClient,
		gangManager:         gangscheduling.NewManager(kubeClientSet, volcanoClient),
		podsLister:          podsInformer.Lister(),
		podsInformer:        podsInformer.Informer(),
		servicesLister:      servicesInformer.Lister(),
		servicesInformer:    servicesInformer.Informer(),
		modelInfersLister:   modelInferInformer.Lister(),
		modelInfersInformer: modelInferInformer.Informer(),
		// nolint
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ModelInfers"),
		store:     store,
	}

	klog.Info("Set the ModelInfer event handler")
	_, _ = c.modelInfersInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.addModelInfer(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.updateModelInfer(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.deleteModelInfer(obj)
		},
	})

	_, _ = c.podsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.addPod(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.updatePod(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.deletePod(obj)
		},
	})

	c.syncHandler = c.syncModelInfer

	return c, nil
}

func (c *ModelInferController) addModelInfer(obj interface{}) {
	mi, ok := obj.(*workloadv1alpha1.ModelInfer)
	if !ok {
		klog.Error("failed to parse ModelInfer type when addMI")
		return
	}
	klog.V(4).InfoS("Adding", "modelinfer", klog.KObj(mi))
	c.enqueueModelInfer(mi)
}

func (c *ModelInferController) updateModelInfer(old, cur interface{}) {
	curMI, ok := cur.(*workloadv1alpha1.ModelInfer)
	if !ok {
		klog.Error("failed to parse ModelInfer type when updateMI")
		return
	}
	oldMI, ok := old.(*workloadv1alpha1.ModelInfer)
	if !ok {
		klog.Error("failed to parse ModelInfer type when updateMI")
		return
	}

	if reflect.DeepEqual(oldMI.Spec, curMI.Spec) {
		// If the spec has not changed, we do not need to reconcile.
		klog.V(4).InfoS("Spec has not changed, skipping update", "modelinfer", klog.KObj(curMI))
		return
	}

	c.enqueueModelInfer(curMI)
}

func (c *ModelInferController) deleteModelInfer(obj interface{}) {
	mi, ok := obj.(*workloadv1alpha1.ModelInfer)
	if !ok {
		// If the object is not a ModelInfer, it might be a tombstone object.
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("failed to parse ModelInfer type when deleteMI %#v", obj)
			return
		}
		mi, ok = tombstone.Obj.(*workloadv1alpha1.ModelInfer)
		if !ok {
			klog.Errorf("failed to parse ModelInfer from tombstone %#v", tombstone.Obj)
			return
		}
	}

	c.store.DeleteModelInfer(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      mi.Name,
	})
}

func (c *ModelInferController) addPod(obj interface{}) {
	c.updatePod(nil, obj)
}

func (c *ModelInferController) updatePod(oldObj, newObj interface{}) {
	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		klog.Error("failed to parse newPod type when updatePod")
		return
	}

	if newPod.DeletionTimestamp != nil {
		// If the pod is being deleted, we do not need to handle it.
		// After deleted，following work will be done in deletePod.
		return
	}

	mi, inferGroupName, err := c.getModelInfer(newPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("modelInfer of pod %s has been deleted", newPod.Name)
		} else {
			klog.Errorf("get model infer failed when update pod: %v", err)
		}
		return
	}

	if c.shouldSkipPodHandling(mi, inferGroupName, newPod) {
		// Pod revision mismatch inferGroup, this can rarely happen
		return
	}

	switch {
	case utils.IsPodRunningAndReady(newPod):
		// The pod is available, that is, the state is running, and the container is ready
		err = c.handleReadyPod(mi, inferGroupName, newPod)
		if err != nil {
			klog.Errorf("handle running pod failed: %v", err)
		}
	case utils.IsPodFailed(newPod) || utils.ContainerRestarted(newPod):
		// handleErrorPod is not called until modelInfer has been called.
		if !c.initialSync {
			return
		}
		// Failure occurs in pod and we need to wait for a grace period before making a judgment.
		err = c.handleErrorPod(mi, inferGroupName, newPod)
		if err != nil {
			klog.Errorf("handle error pod failed: %v", err)
		}
	}
}

func (c *ModelInferController) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		// If the object is not a Pod, it might be a tombstone object.
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Error("failed to parse pod type when deletePod")
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			klog.Errorf("failed to parse Pod from tombstone %#v", tombstone.Obj)
			return
		}
	}

	mi, inferGroupName, err := c.getModelInfer(pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("modelInfer of pod %s/%s has been deleted", pod.GetNamespace(), pod.GetName())
		} else {
			klog.Errorf("failed to get modelInfer of pod %s/%s: %v", pod.GetNamespace(), pod.GetName(), err)
		}
		return
	}

	roleName, roleID := utils.PodRoleName(pod), utils.PodRoleID(pod)
	// check inferGroup status
	if c.store.GetInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName) == datastore.InferGroupDeleting {
		// inferGroup is already in the deletion process, only checking whether the deletion is completed
		if c.isInferGroupDeleted(mi, inferGroupName) {
			// inferGroup has been deleted, so the storage needs to be updated and need to reconcile.
			klog.V(2).Infof("inferGroup %s has been deleted", inferGroupName)
			c.store.DeleteInferGroup(utils.GetNamespaceName(mi), inferGroupName)
			c.enqueueModelInfer(mi)
		}
		return
	}

	c.store.DeleteRunningPodFromInferGroup(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      mi.Name,
	}, inferGroupName, pod.Name)

	// check role status
	if c.store.GetRoleStatus(utils.GetNamespaceName(mi), inferGroupName, roleName, roleID) == datastore.RoleDeleting {
		// role is already in the deletion process, only checking whether the deletion is completed
		if c.isRoleDeleted(mi, inferGroupName, utils.PodRoleName(pod), utils.PodRoleID(pod)) {
			// role has been deleted, so the storage needs to be updated and need to reconcile.
			klog.V(2).Infof("role %s of inferGroup %s has been deleted", utils.PodRoleID(pod), inferGroupName)
			c.store.DeleteRole(utils.GetNamespaceName(mi), inferGroupName, roleName, roleID)
			c.enqueueModelInfer(mi)
		}
		return
	}

	if c.shouldSkipPodHandling(mi, inferGroupName, pod) {
		return
	}

	err = c.handleDeletedPod(mi, inferGroupName, pod)
	if err != nil {
		klog.Errorf("handle deleted pod failed: %v", err)
	}
}

func (c *ModelInferController) enqueueModelInfer(mi *workloadv1alpha1.ModelInfer) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(mi); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *ModelInferController) worker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *ModelInferController) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.workqueue.Get()
	if quit {
		return false
	}
	defer c.workqueue.Done(key)

	err := c.syncHandler(ctx, key.(string))
	if err == nil {
		c.workqueue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("sync %q failed with %v", key, err))
	c.workqueue.AddRateLimited(key)

	return true
}

func (c *ModelInferController) syncModelInfer(ctx context.Context, key string) error {
	// TODO: Consider obtaining the pod status during the modelinfer reconcile to process the infergroup status. This can ensure the real-time status.
	klog.V(4).Info("Started syncing ModelInfer")
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", err)
	}

	mi, err := c.modelInfersLister.ModelInfers(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(4).Infof("%v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}
	// only fields in roles can be modified in rolling updates.
	// and only modifying the role.replicas field will not affect the revision.
	copy := utils.RemoveRoleReplicasForRevision(mi)
	revision := utils.Revision(copy.Spec.Template.Roles)

	// PodGroup Manager
	if err := c.gangManager.ManagePodGroups(ctx, mi); err != nil {
		return fmt.Errorf("Failed to manage PodGroups for ModelInfer %s/%s: %v", mi.Namespace, mi.Name, err)
	}

	err = c.manageInferGroupReplicas(ctx, mi, revision)
	if err != nil {
		return fmt.Errorf("cannot manage inferGroup replicas: %v", err)
	}

	err = c.manageRole(ctx, mi, revision)
	if err != nil {
		return fmt.Errorf("cannot manage role replicas: %v", err)
	}

	err = c.manageInferGroupRollingUpdate(mi, revision)
	if err != nil {
		return fmt.Errorf("cannot manage inferGroup rollingUpdate: %v", err)
	}

	if err := c.UpdateModelInferStatus(mi, revision); err != nil {
		return fmt.Errorf("failed to update status of mi %s/%s: %v", namespace, name, err)
	}

	return nil
}

func (c *ModelInferController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// start informers
	go c.podsInformer.RunWithContext(ctx)
	go c.servicesInformer.RunWithContext(ctx)
	go c.modelInfersInformer.RunWithContext(ctx)

	cache.WaitForCacheSync(ctx.Done(),
		c.podsInformer.HasSynced,
		c.servicesInformer.HasSynced,
		c.modelInfersInformer.HasSynced,
	)

	// sync pods first
	c.syncAll()
	klog.Info("initial sync has been done")

	klog.Info("start modelInfer controller")
	for i := 0; i < workers; i++ {
		go c.worker(ctx)
	}
	<-ctx.Done()
	klog.Info("shut down modelInfer controller")
}

// sync all pods before starting the worker
// we donot need to sync ModelInfer here, because the ModelInfer controller will sync all ModelInfers after the initial sync.
// Related inferGroups will be created when syncing pods.
func (c *ModelInferController) syncAll() {
	pods, _ := c.podsLister.List(labels.Everything())
	for _, pod := range pods {
		c.addPod(pod)
	}

	c.initialSync = true
}

// UpdateModelInferConditionsStatus update conditions ModelInfer status.
func (c *ModelInferController) UpdateModelInferConditionsStatus(mi *workloadv1alpha1.ModelInfer, condition metav1.Condition) error {
	if !meta.SetStatusCondition(&mi.Status.Conditions, condition) {
		return fmt.Errorf("failed to update modelInfer %s/%s status conditions", mi.GetNamespace(), mi.GetName())
	}
	return nil
}

// UpdateModelInferStatus update replicas in modelInfer status.
func (c *ModelInferController) UpdateModelInferStatus(mi *workloadv1alpha1.ModelInfer, revision string) error {
	groups, err := c.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	if err != nil {
		return err
	}

	available, updated, current := 0, 0, 0
	progressingGroups, updatedGroups, currentGroups := []int{}, []int{}, []int{}
	for index := range groups {
		if groups[index].Status == datastore.InferGroupRunning {
			available = available + 1
		} else if ok, err := c.checkInferGroupReady(mi, groups[index].Name); ok && err == nil {
			// some scenarios, pod events may not trigger group status updates, such as role scaling down.
			err = c.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), groups[index].Name, datastore.InferGroupRunning)
			if err != nil {
				return fmt.Errorf("failed to set inferGroup %s status: %v", groups[index].Name, err)
			}
			available = available + 1
			klog.V(2).Infof("Update inferGroup %s status to Running", groups[index].Name)
		} else {
			progressingGroups = append(progressingGroups, index)
		}

		if groups[index].Revision == revision {
			updated = updated + 1
			updatedGroups = append(updatedGroups, index)
		} else {
			current = current + 1
			currentGroups = append(currentGroups, index)
		}
	}

	copy := mi.DeepCopy()
	shouldUpdate := utils.SetCondition(copy, progressingGroups, updatedGroups, currentGroups)
	if copy.Status.Replicas != int32(len(groups)) || copy.Status.AvailableReplicas != int32(available) || copy.Status.UpdatedReplicas != int32(updated) || copy.Status.CurrentReplicas != int32(current) {
		shouldUpdate = true
		copy.Status.Replicas = int32(len(groups))
		copy.Status.AvailableReplicas = int32(available)
		copy.Status.UpdatedReplicas = int32(updated)
		copy.Status.CurrentReplicas = int32(current)
	}

	if copy.Spec.RolloutStrategy == nil || copy.Spec.RolloutStrategy.RollingUpdateConfiguration == nil || copy.Spec.RolloutStrategy.RollingUpdateConfiguration.Partition == nil {
		// if not set spec.RolloutStrategy.RollingUpdateConfiguration.Partition,
		// should set currentReplicas = updatedReplicas when rolling update is over.
		if copy.Status.UpdatedReplicas == *copy.Spec.Replicas &&
			copy.Status.AvailableReplicas == *copy.Spec.Replicas &&
			copy.Status.Replicas == *copy.Spec.Replicas {
			shouldUpdate = true
			copy.Status.CurrentReplicas = copy.Status.UpdatedReplicas
		}
	}

	if copy.Status.ObservedGeneration != mi.Generation {
		shouldUpdate = true
		copy.Status.ObservedGeneration = mi.Generation
	}

	if shouldUpdate {
		_, err := c.modelInferClient.WorkloadV1alpha1().ModelInfers(copy.GetNamespace()).UpdateStatus(context.TODO(), copy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ModelInferController) manageInferGroupReplicas(ctx context.Context, mi *workloadv1alpha1.ModelInfer, newRevision string) error {
	inferGroupList, err := c.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	if err != nil && !errors.Is(err, datastore.ErrInferGroupNotFound) {
		return fmt.Errorf("cannot get inferGroup of modelInfer: %s from map: %v", mi.GetName(), err)
	}
	expectedCount := int(*mi.Spec.Replicas)
	curReplicas := len(inferGroupList)
	if curReplicas == expectedCount {
		klog.V(4).Info("The number of replicas is consistent, no need to scale up or down")
		return nil
	}
	// slice that will contain all InferGroups as excepted
	replicas := make([]*datastore.InferGroup, expectedCount)
	// slice that will contain all InferGroups Out of except or fails to parse ordinal
	condemned := make([]datastore.InferGroup, 0)
	// First we partition inferGroups into two lists valid replicas and condemned inferGroups
	for _, group := range inferGroupList {
		_, inferGroupOrdinal := utils.GetParentNameAndOrdinal(group.Name)
		if inferGroupOrdinal >= 0 && inferGroupOrdinal < expectedCount {
			copyInferGroup := group
			replicas[inferGroupOrdinal] = &copyInferGroup
		} else {
			// Whether the inferGroup sequence number fails to parse or out of except, a rebuild should be performed
			condemned = append(condemned, group)
		}
	}
	for idx := 0; idx < expectedCount; idx++ {
		if replicas[idx] == nil {
			// Insert new InferGroup to global storage
			c.store.AddInferGroup(utils.GetNamespaceName(mi), idx, newRevision)
			// Create pods for inferGroup
			err = c.CreatePodsForInferGroup(ctx, mi, idx, newRevision)
			if err != nil {
				// I think that after create a pod failed, a period of time should pass before joining the coordination queue.
				return fmt.Errorf("create infer group failed: %v", err)
			}
		}
	}
	for _, group := range condemned {
		c.DeleteInferGroup(mi, group.Name)
	}
	return nil
}

func (c *ModelInferController) CreatePodsForInferGroup(ctx context.Context, mi *workloadv1alpha1.ModelInfer, groupIndex int, newHash string) error {
	// traverse each role in inferGroup to create entry-worker pod group.
	roleList := mi.Spec.Template.Roles
	for _, role := range roleList {
		// there will be multiple replicas in a role, such as xPyD type
		for roleIndex := range int(*role.Replicas) {
			err := c.CreatePodByRole(ctx, *role.DeepCopy(), mi, roleIndex, groupIndex, newHash)
			if err != nil {
				return fmt.Errorf("create role pod failed: %v, role name: %s, role index: %d", err, role.Name, roleIndex)
			}
		}
	}
	return nil
}

func (c *ModelInferController) DeleteInferGroup(mi *workloadv1alpha1.ModelInfer, groupname string) {
	miNamedName := utils.GetNamespaceName(mi)
	inferGroupStatus := c.store.GetInferGroupStatus(miNamedName, groupname)
	if inferGroupStatus == datastore.InferGroupNotFound {
		return
	}

	groupNameValue := fmt.Sprintf("%s/%s", miNamedName.Namespace, groupname)
	label := fmt.Sprintf("%s=%s", workloadv1alpha1.GroupNameLabelKey, groupname)
	if inferGroupStatus != datastore.InferGroupDeleting {
		err := c.store.UpdateInferGroupStatus(miNamedName, groupname, datastore.InferGroupDeleting)
		if err != nil {
			klog.Errorf("failed to set inferGroup %s/%s status: %v", miNamedName.Namespace+"/"+mi.Name, groupname, err)
			return
		}
		// Delete all pods in inferGroup
		err = c.kubeClientSet.CoreV1().Pods(miNamedName.Namespace).DeleteCollection(
			context.TODO(),
			metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: label,
			},
		)
		if err != nil {
			klog.Errorf("failed to delete inferGroup %s/%s: %v", miNamedName.Namespace+"/"+mi.Name, groupname, err)
			return
		}
		// There is no DeleteCollection operation in the service of client-go. We need to list and delete them one by one.
		services, err := c.getServicesByIndex(GroupNameKey, groupNameValue)
		if err != nil {
			klog.Errorf("failed to get service %v", err)
			return
		}
		for _, svc := range services {
			err = c.kubeClientSet.CoreV1().Services(miNamedName.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(4).Infof("service %s/%s has been deleted", miNamedName.Namespace, svc.Name)
				} else {
					klog.Errorf("failed to delete service %s/%s: %v", miNamedName.Namespace, svc.Name, err)
					return
				}
			}
		}
	}

	// check whether the deletion has been completed
	pods, err := c.getPodsByIndex(GroupNameKey, groupNameValue)
	if err != nil {
		klog.Errorf("failed to get pod, err:%v", err)
	}
	services, err := c.getServicesByIndex(GroupNameKey, groupNameValue)
	if err != nil {
		klog.Errorf("failed to get service, err:%v", err)
	}
	if len(pods) == 0 && len(services) == 0 {
		klog.V(2).Infof("inferGroup %s has been deleted", groupname)
		c.store.DeleteInferGroup(miNamedName, groupname)
		c.enqueueModelInfer(mi)
		return
	}
}

func (c *ModelInferController) CreatePodByRole(ctx context.Context, role workloadv1alpha1.Role, mi *workloadv1alpha1.ModelInfer, roleIndex, groupIndex int, newHash string) error {
	groupName := utils.GenerateInferGroupName(mi.Name, groupIndex)
	taskName := c.gangManager.GenerateTaskName(role.Name, roleIndex)
	// Create entry pod
	entryPod := utils.GenerateEntryPod(role, mi, groupName, roleIndex, newHash)

	c.gangManager.AnnotatePodWithPodGroup(entryPod, mi, 1+int(role.WorkerReplicas), groupName, taskName)

	_, err := c.kubeClientSet.CoreV1().Pods(mi.Namespace).Create(ctx, entryPod, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			klog.Errorf("create entry pod failed: %v", err)
			return err
		}
	}

	// Determine whether to create worker pods and headless service
	if role.WorkerTemplate == nil {
		klog.V(4).Info("workerTemplate is nil, no need to create worker pods and headless service")
		return nil
	}
	// Create headless service
	err = utils.CreateHeadlessService(ctx, c.kubeClientSet, mi, entryPod.ObjectMeta.Labels, groupName, role.Name, roleIndex)
	if err != nil {
		klog.Errorf("create headless service failed: %v", err)
		return err
	}
	// Create worker pods
	for podIndex := range int(role.WorkerReplicas) {
		workerPod := utils.GenerateWorkerPod(role, mi, entryPod, groupName, roleIndex, podIndex+1, newHash) // worker-pod sequence number starts from 1, so we use index+1 here.
		c.gangManager.AnnotatePodWithPodGroup(workerPod, mi, 1+int(role.WorkerReplicas), groupName, taskName)
		_, err = c.kubeClientSet.CoreV1().Pods(mi.Namespace).Create(ctx, workerPod, metav1.CreateOptions{})
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				klog.Errorf("create worker pod failed: %v", err)
				return err
			}
		}
	}
	return nil
}

func (c *ModelInferController) manageRole(ctx context.Context, mi *workloadv1alpha1.ModelInfer, newRevision string) error {
	inferGroupList, err := c.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	if err != nil && !errors.Is(err, datastore.ErrInferGroupNotFound) {
		return fmt.Errorf("cannot get inferGroup of modelInfer: %s from map: %v", mi.GetName(), err)
	}
	for _, inferGroup := range inferGroupList {
		if c.store.GetInferGroupStatus(utils.GetNamespaceName(mi), inferGroup.Name) == datastore.InferGroupDeleting {
			// Deleting inferGroup will be recreated after the deletion is complete, so there is no need to scale the roles
			continue
		}
		_, inferGroupOrdinal := utils.GetParentNameAndOrdinal(inferGroup.Name)
		for _, targetRole := range mi.Spec.Template.Roles {
			c.manageRoleReplicas(ctx, mi, inferGroup.Name, targetRole, inferGroupOrdinal, newRevision)
		}
	}
	return nil
}

// manageRoleReplicas manages the replicas of a specific role within an infer group
// It handles both scale up and scale down operations for the role
func (c *ModelInferController) manageRoleReplicas(ctx context.Context, mi *workloadv1alpha1.ModelInfer, groupName string, targetRole workloadv1alpha1.Role, inferGroupOrdinal int, newRevision string) {
	// TODO: add podGroup update after gang scheduler finished
	// Get all replicas of a role from storage, for example, prefill-0, prefill-1...
	roleList, err := c.store.GetRoleList(utils.GetNamespaceName(mi), groupName, targetRole.Name)
	if err != nil {
		klog.Errorf("cannot get role %s in inferGroup %s, err:%v", targetRole.Name, groupName, err)
		return
	}

	expectedCount := int(*targetRole.Replicas)
	if len(roleList) == expectedCount {
		klog.V(4).Infof("The replicas of role %s in inferGroup %s is consistent, no need to scale up or down", targetRole.Name, groupName)
		return
	}

	// slice that will contain all Roles as expected
	replicas := make([]*datastore.Role, expectedCount)
	// slice that will contain all Roles out of expected range or fails to parse ordinal
	condemned := make([]datastore.Role, 0)

	// Partition roles into valid replicas and condemned roles
	for _, role := range roleList {
		_, roleOrdinal := utils.GetParentNameAndOrdinal(role.Name)
		if roleOrdinal >= 0 && roleOrdinal < expectedCount {
			copy := role
			replicas[roleOrdinal] = &copy
		} else {
			// Whether the role sequence number fails to parse or out of expected range, a rebuild should be performed
			condemned = append(condemned, role)
		}
	}

	// Handle scale up
	for idx := 0; idx < expectedCount; idx++ {
		if replicas[idx] == nil {
			// Role needs to scale up, and the inferGroup status needs to be set to Scaling
			if c.store.GetInferGroupStatus(utils.GetNamespaceName(mi), groupName) != datastore.InferGroupScaling {
				err := c.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), groupName, datastore.InferGroupScaling)
				if err != nil {
					klog.Errorf("failed to set inferGroup %s/%s status: %v", mi.Namespace+"/"+mi.Name, groupName, err)
					return
				}
			}
			// Insert new Role to global storage
			c.store.AddRole(utils.GetNamespaceName(mi), groupName, targetRole.Name, utils.GenerateRoleID(targetRole.Name, idx), newRevision)
			// Create pods for role
			err = c.CreatePodByRole(ctx, *targetRole.DeepCopy(), mi, idx, inferGroupOrdinal, newRevision)
			if err != nil {
				klog.Errorf("create role %s for inferGroup %s failed: %v", utils.GenerateRoleID(targetRole.Name, idx), groupName, err)
			}
		}
	}

	// Handle scale down
	for _, role := range condemned {
		// Role needs to scale down, and the inferGroup status needs to be set to Scaling
		if c.store.GetInferGroupStatus(utils.GetNamespaceName(mi), groupName) != datastore.InferGroupScaling {
			err := c.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), groupName, datastore.InferGroupScaling)
			if err != nil {
				klog.Errorf("failed to set inferGroup %s/%s status: %v", mi.Namespace+"/"+mi.Name, groupName, err)
				return
			}
		}
		c.DeleteRole(ctx, mi, groupName, targetRole.Name, role.Name)
	}
}

func (c *ModelInferController) DeleteRole(ctx context.Context, mi *workloadv1alpha1.ModelInfer, groupName, roleName, roleID string) {
	selector := labels.SelectorFromSet(map[string]string{
		workloadv1alpha1.GroupNameLabelKey: groupName,
		workloadv1alpha1.RoleLabelKey:      roleName,
		workloadv1alpha1.RoleIDKey:         roleID,
	})
	// If the role is already in the deletion process, no further processing will be done.
	roleStatus := c.store.GetRoleStatus(utils.GetNamespaceName(mi), groupName, roleName, roleID)
	if roleStatus == datastore.RoleDeleting {
		return
	}
	err := c.store.UpdateRoleStatus(utils.GetNamespaceName(mi), groupName, roleName, roleID, datastore.RoleDeleting)
	if err != nil {
		klog.Errorf("failed to set role %s/%s status: %v", groupName, roleID, err)
		return
	}
	// Delete all pods in role
	err = c.kubeClientSet.CoreV1().Pods(mi.Namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)
	if err != nil {
		klog.Errorf("failed to delete pods of role %s/%s: %v", groupName, roleID, err)
	}
	// There is no DeleteCollection operation in the service of client-go. We need to list and delete them one by one.
	roleIDValue := fmt.Sprintf("%s/%s/%s/%s", mi.Namespace, groupName, roleName, roleID)
	services, err := c.getServicesByIndex(RoleIDKey, roleIDValue)
	if err != nil {
		klog.Errorf("failed to get service %v", err)
		return
	}
	for _, svc := range services {
		err = c.kubeClientSet.CoreV1().Services(mi.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(4).Infof("service %s/%s has been deleted", mi.Namespace, svc.Name)
			}
			klog.Errorf("failed to delete service %s/%s: %v", mi.Namespace, svc.Name, err)
		}
	}
}

func (c *ModelInferController) manageInferGroupRollingUpdate(mi *workloadv1alpha1.ModelInfer, revision string) error {
	// we compute the minimum ordinal of the target sequence for a destructive update based on the strategy.
	updateMin := 0
	if mi.Spec.RolloutStrategy != nil && mi.Spec.RolloutStrategy.RollingUpdateConfiguration != nil && mi.Spec.RolloutStrategy.RollingUpdateConfiguration.Partition != nil {
		updateMin = int(*mi.Spec.RolloutStrategy.RollingUpdateConfiguration.Partition)
	}
	inferGroupList, err := c.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	if err != nil {
		return fmt.Errorf("cannot get inferGroupList from store, err:%v", err)
	}
	// we terminate the inferGroup with the largest ordinal that does not match the update revision.
	for i := len(inferGroupList) - 1; i >= updateMin; i-- {
		if c.isInferGroupOutdated(inferGroupList[i], mi.Namespace, revision) {
			// target inferGroup is not the latest version, needs to be updated
			klog.V(2).Infof("inferGroup %s will be terminating for update", inferGroupList[i].Name)
			c.DeleteInferGroup(mi, inferGroupList[i].Name)
			return nil
		}
		if inferGroupList[i].Status != datastore.InferGroupRunning {
			// target inferGroup is the latest version, but not running. We need to wait for the status to change to running.
			// If the group fails after rolling, it will automatically be deleted and rebuilt when detecting the pod failure.
			// If the group still pending due to reasons such as being unable to be scheduled, rolling update process will stop
			// to avoid affecting other groups that are running normally.
			klog.V(4).Infof("waiting for the infergroup %s status become running", inferGroupList[i].Name)
			return nil
		}
		// target inferGroup is already the latest version and running, processing the rolling update of the next group.
	}
	klog.V(2).Infof("all target groups of modelInfer %s have been updated", mi.Name)
	return nil
}

func (c *ModelInferController) handleReadyPod(mi *workloadv1alpha1.ModelInfer, inferGroupName string, newPod *corev1.Pod) error {
	// Add the running pod to the global storage and try to update the inferGroup status
	c.store.AddRunningPodToInferGroup(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      mi.Name,
	}, inferGroupName, newPod.Name, utils.PodRevision(newPod), utils.PodRoleName(newPod), utils.PodRoleID(newPod))
	ready, err := c.checkInferGroupReady(mi, inferGroupName)
	if err != nil {
		return fmt.Errorf("failed to check inferGroup status, err: %v", err)
	}
	if ready {
		// All pods in the inferGroup are running, so the inferGroup status also needs to be set to running
		err = c.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName, datastore.InferGroupRunning)
		if err != nil {
			return fmt.Errorf("failed to set inferGroup %s status: %v", inferGroupName, err)
		}
		klog.V(2).Infof("Update inferGroup %s status to Running", inferGroupName)
		c.enqueueModelInfer(mi)
	} else {
		klog.V(4).Infof("inferGroup %s still creating", inferGroupName)
	}
	return nil
}

func (c *ModelInferController) handleErrorPod(mi *workloadv1alpha1.ModelInfer, inferGroupName string, errPod *corev1.Pod) error {
	// pod is already in the grace period and does not need to be processed for the time being.
	_, exists := c.graceMap.Load(utils.GetNamespaceName(errPod))
	now := time.Now()
	if exists {
		klog.V(4).Infof("Pod %v failed, waiting for grace time", utils.GetNamespaceName(errPod))
		return nil
	}
	// add pod to the grace period map
	c.graceMap.Store(utils.GetNamespaceName(errPod), now)
	c.store.DeleteRunningPodFromInferGroup(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      mi.Name,
	}, inferGroupName, errPod.Name)
	// If the inferGroup status is already running, the status needs to be updated
	if groupStatus := c.store.GetInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName); groupStatus == datastore.InferGroupRunning {
		err := c.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName, datastore.InferGroupCreating)
		if err != nil {
			return fmt.Errorf("update infergroup status failed, err:%v", err)
		}
		klog.V(2).Infof("update infergroup %s to processing when pod fails", inferGroupName)
	}
	// Wait for the grace period before processing
	go c.handlePodAfterGraceTime(mi, errPod)
	// InferGroup status may change, needs reconcile
	c.enqueueModelInfer(mi)
	return nil
}

func (c *ModelInferController) handlePodAfterGraceTime(mi *workloadv1alpha1.ModelInfer, errPod *corev1.Pod) {
	if mi.Spec.Template.RestartGracePeriodSeconds != nil && *mi.Spec.Template.RestartGracePeriodSeconds > 0 {
		// Wait for the grace period before making a decision
		time.Sleep(time.Duration(*mi.Spec.Template.RestartGracePeriodSeconds) * time.Second)
		klog.V(4).Infof("%s after grace time", errPod.Name)
		defer c.graceMap.Delete(utils.GetNamespaceName(errPod))

		newPod, err := c.podsLister.Pods(mi.Namespace).Get(errPod.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(4).Infof("pod %s has been deleted after grace time", errPod.Name)
			} else {
				klog.Errorf("cannot get pod %s after grace time, err: %v", errPod.Name, err)
			}
			return
		}

		if !utils.IsPodRunningAndReady(newPod) {
			// pod has not recovered after the grace period, needs to be rebuilt
			// After this pod has been deleted, we will rebuild the inferGroup in deletePod function
			err = c.kubeClientSet.CoreV1().Pods(mi.Namespace).Delete(context.TODO(), newPod.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("cannot delete pod %s after grace time, err: %v", newPod.Name, err)
				return
			}
			klog.V(2).Infof("%s been deleted after grace time", errPod.Name)
		}
	} else {
		// grace period is not set or the grace period is 0, the deletion will be executed immediately.
		defer c.graceMap.Delete(utils.GetNamespaceName(errPod))

		err := c.kubeClientSet.CoreV1().Pods(mi.Namespace).Delete(context.TODO(), errPod.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("cannot delete pod %s when it error, err: %v", errPod.Name, err)
			return
		}
		klog.V(2).Infof("%s been deleted without grace time", errPod.Name)
	}
}

func (c *ModelInferController) handleDeletedPod(mi *workloadv1alpha1.ModelInfer, inferGroupName string, pod *corev1.Pod) error {
	// pod is deleted due to failure or other reasons and needs to be rebuilt according to the RecoveryPolicy
	switch mi.Spec.RecoveryPolicy {
	case workloadv1alpha1.InferGroupRecreate:
		// Rebuild the entire inferGroup directly
		c.DeleteInferGroup(mi, inferGroupName)
	case workloadv1alpha1.RoleRecreate:
		if c.store.GetInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName) == datastore.InferGroupRunning {
			// If the inferGroup status is running when the pod fails, we need to set it to creating
			err := c.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName, datastore.InferGroupCreating)
			if err != nil {
				return fmt.Errorf("failed to set inferGroup %s status: %v", inferGroupName, err)
			}
		}
		c.DeleteRole(context.Background(), mi, inferGroupName, utils.PodRoleName(pod), utils.PodRoleID(pod))
	}
	return nil
}

func (c *ModelInferController) checkInferGroupReady(mi *workloadv1alpha1.ModelInfer, inferGroupName string) (bool, error) {
	// TODO: modify inferGroupReady logic after rolling update functionality is implemented
	runningPodsNum, err := c.store.GetRunningPodNumByInferGroup(utils.GetNamespaceName(mi), inferGroupName)
	if err != nil {
		return false, err
	}
	if runningPodsNum != utils.ExpectedPodNum(mi) {
		// the number of running pods does not reach the expected number
		return false, nil
	}
	return true, nil
}

func (c *ModelInferController) isInferGroupOutdated(group datastore.InferGroup, namespace, newRevision string) bool {
	// Find the pods corresponding to inferGroup
	groupNameValue := fmt.Sprintf("%s/%s", namespace, group.Name)
	pods, err := c.getPodsByIndex(GroupNameKey, groupNameValue)
	if err != nil {
		klog.Errorf("cannot list pod when check group updated,err: %v", err)
		return true
	}
	// Check all pods match the newHash
	for _, pod := range pods {
		if utils.PodRevision(pod) != newRevision {
			return true
		}
	}
	return false
}

func (c *ModelInferController) getModelInfer(pod *corev1.Pod) (*workloadv1alpha1.ModelInfer, string, error) {
	modelInferName, inferGroupName, ok := utils.GetModelInferAndGroupByLabel(pod.GetLabels())
	if !ok {
		return nil, "", fmt.Errorf("cannot get modelInfer name and inferGroup name from pod %s", pod.Name)
	}
	mi, err := c.modelInfersLister.ModelInfers(pod.Namespace).Get(modelInferName)
	if err != nil {
		return nil, "", err
	}
	return mi, inferGroupName, nil
}

// shouldSkipPodHandling checks if a pod should be skipped based on revision mismatch
func (c *ModelInferController) shouldSkipPodHandling(mi *workloadv1alpha1.ModelInfer, inferGroupName string, pod *corev1.Pod) bool {
	podRevision := utils.PodRevision(pod)
	inferGroup := c.store.GetInferGroup(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      mi.Name,
	}, inferGroupName)
	if inferGroup != nil && inferGroup.Revision != podRevision {
		// If the pod revision is not equal to the inferGroup revision, we do not need to handle it.
		klog.V(4).Infof("pod %s/%s revision %s is not equal to inferGroup %s revision %s, skip handling",
			pod.Namespace, pod.Name, podRevision, inferGroupName, inferGroup.Revision)
		return true
	}
	return false
}

func (c *ModelInferController) isInferGroupDeleted(mi *workloadv1alpha1.ModelInfer, inferGroupName string) bool {
	status := c.store.GetInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName)
	if status != datastore.InferGroupDeleting {
		// It will be determined whether all resource have been deleted only when the group status is deleting.
		return false
	}
	// check whether the inferGroup deletion has been completed
	groupNameValue := fmt.Sprintf("%s/%s", mi.Namespace, inferGroupName)
	pods, err := c.getPodsByIndex(GroupNameKey, groupNameValue)
	if err != nil {
		klog.Errorf("failed to get pod, err: %v", err)
		return false
	}
	services, err := c.getServicesByIndex(GroupNameKey, groupNameValue)
	if err != nil {
		klog.Errorf("failed to get service, err:%v", err)
		return false
	}
	return len(pods) == 0 && len(services) == 0
}

func (c *ModelInferController) isRoleDeleted(mi *workloadv1alpha1.ModelInfer, inferGroupName, roleName, roleID string) bool {
	if c.store.GetRoleStatus(utils.GetNamespaceName(mi), inferGroupName, roleName, roleID) != datastore.RoleDeleting {
		// It will be determined whether all resource have been deleted only when the role status is deleting.
		return false
	}
	roleIDValue := fmt.Sprintf("%s/%s/%s/%s", mi.Namespace, inferGroupName, roleName, roleID)
	// check whether the role deletion has been completed
	pods, err := c.getPodsByIndex(RoleIDKey, roleIDValue)
	if err != nil {
		klog.Errorf("failed to get pod, err: %v", err)
		return false
	}
	services, err := c.getServicesByIndex(RoleIDKey, roleIDValue)
	if err != nil {
		klog.Errorf("failed to get service, err:%v", err)
		return false
	}
	return len(pods) == 0 && len(services) == 0
}

// getPodsByIndex filter pods using the informer indexer.
func (c *ModelInferController) getPodsByIndex(indexName, indexValue string) ([]*corev1.Pod, error) {
	indexer := c.podsInformer.GetIndexer()
	if _, exists := indexer.GetIndexers()[indexName]; !exists {
		return nil, fmt.Errorf("pod indexer %s not found", indexName)
	}
	objs, err := indexer.ByIndex(indexName, indexValue)
	if err != nil {
		return nil, err
	}

	var pods []*corev1.Pod
	for _, obj := range objs {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			klog.Errorf("unexpected object type in pod indexer: %T", obj)
			continue
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// getServicesByIndex filter services using the informer indexer.
func (c *ModelInferController) getServicesByIndex(indexName, indexValue string) ([]*corev1.Service, error) {
	indexer := c.servicesInformer.GetIndexer()
	if _, exists := indexer.GetIndexers()[indexName]; !exists {
		return nil, fmt.Errorf("service indexer %s not found", indexName)
	}
	objs, err := indexer.ByIndex(indexName, indexValue)
	if err != nil {
		return nil, err
	}

	var services []*corev1.Service
	for _, obj := range objs {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			klog.Errorf("unexpected object type in service indexer: %T", obj)
			continue
		}
		services = append(services, svc)
	}
	return services, nil
}
