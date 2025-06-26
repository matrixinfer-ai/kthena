package controller

import (
	"context"
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

	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	informersv1alpha1 "matrixinfer.ai/matrixinfer/client-go/informers/externalversions"
	listerv1alpha1 "matrixinfer.ai/matrixinfer/client-go/listers/workload/v1alpha1"
	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

type ModelInferController struct {
	kubeClientSet    kubernetes.Interface
	modelInferClient clientset.Interface

	syncHandler         func(ctx context.Context, miKey string) error
	podsLister          listerv1.PodLister
	podsInformer        cache.Controller
	servicesLister      listerv1.ServiceLister
	servicesInformer    cache.Controller
	modelInfersLister   listerv1alpha1.ModelInferLister
	modelInfersInformer cache.Controller

	// nolint
	workqueue workqueue.RateLimitingInterface
	store     datastore.Store
	graceMap  sync.Map // key: errorPod.namespace/errorPod.name, value:time
}

func NewModelInferController(kubeClientSet kubernetes.Interface, modelInferClient clientset.Interface) *ModelInferController {
	req, err := labels.NewRequirement(workloadv1alpha1.GroupNameLabelKey, selection.Exists, nil)
	if err != nil {
		klog.Errorf("cannot create label selector,err:%v", err)
		return nil
	}
	selector := labels.NewSelector().Add(*req)

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

	store, err := datastore.New()
	if err != nil {
		klog.Fatal("Unable to create data store")
	}

	mic := &ModelInferController{
		kubeClientSet:       kubeClientSet,
		modelInferClient:    modelInferClient,
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
	_, _ = modelInferInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mic.addModelInfer(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			mic.updateModelInfer(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			mic.deleteModelInfer(obj)
		},
	})

	_, _ = podsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mic.addPod(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			mic.updatePod(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			mic.deletePod(obj)
		},
	})

	mic.syncHandler = mic.syncModelInfer

	return mic
}

func (mic *ModelInferController) addModelInfer(obj interface{}) {
	mi, ok := obj.(*workloadv1alpha1.ModelInfer)
	if !ok {
		klog.Error("failed to parse ModelInfer type when addMI")
		return
	}
	klog.V(4).Info("Adding", "modelinfer", klog.KObj(mi))
	mic.enqueueModelInfer(mi)
}

func (mic *ModelInferController) updateModelInfer(old, cur interface{}) {
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

	if *(oldMI.Spec.Replicas) != *(curMI.Spec.Replicas) || !reflect.DeepEqual(oldMI.Spec.Template, curMI.Spec.Template) {
		// Reconciling is only triggered if modelinfer.replicas changes or infergroup.spec changes
		klog.V(4).Info("Updating", "modelinfer", klog.KObj(curMI))
		mic.enqueueModelInfer(curMI)
	}
}

func (mic *ModelInferController) deleteModelInfer(obj interface{}) {
	mi, ok := obj.(*workloadv1alpha1.ModelInfer)
	if !ok {
		klog.Error("failed to parse ModelInfer type when deleteMI")
		return
	}

	inferGroupList, err := mic.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	if err != nil {
		klog.Errorf("get infer group by model infer failed: %v", err)
		return
	}
	for _, group := range inferGroupList {
		mic.DeleteInferGroup(mi, group.Name)
	}
	err = mic.store.DeleteModelInfer(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      mi.Name,
	})
	if err != nil {
		klog.Errorf("delete model infer store failed: %v", err)
	}
}

func (mic *ModelInferController) addPod(obj interface{}) {
}

func (mic *ModelInferController) updatePod(oldObj, newObj interface{}) {
	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		klog.Error("failed to parse newPod type when updatePod")
		return
	}

	mi, inferGroupName, err := mic.getModelInfer(newPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("modelInfer of pod %s has been deleted", newPod.Name)
		} else {
			klog.Errorf("get model infer failed when update pod: %v", err)
		}
		return
	}
	switch {
	case utils.IsPodTerminating(newPod):
		klog.V(4).Infof("pod %s is deleting", newPod.Name)
	case utils.IsPodRunningAndReady(newPod):
		// The pod is available, that is, the state is running, and the container is ready
		err = mic.handleReadyPod(mi, inferGroupName, newPod)
		if err != nil {
			klog.Errorf("handle running pod failed: %v", err)
		}
	case utils.IsPodFailed(newPod) || utils.ContainerRestarted(newPod):
		// Failure occurs in pod and we need to wait for a grace period before making a judgment.
		err = mic.handleErrorPod(mi, inferGroupName, newPod)
		if err != nil {
			klog.Errorf("handle error pod failed: %v", err)
		}
	}
}

func (mic *ModelInferController) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Error("failed to parse pod type when deletePod")
		return
	}

	mi, inferGroupName, err := mic.getModelInfer(pod)
	if err == nil {
		mic.DeleteInferGroup(mi, inferGroupName)
		return
	}
	if apierrors.IsNotFound(err) {
		klog.V(4).Infof("modelInfer of pod %s/%s has been deleted", pod.GetNamespace(), pod.GetName())
	} else {
		klog.Errorf("failed to get modelInfer of pod %s/%s: %v", pod.GetNamespace(), pod.GetName(), err)
	}
}

func (mic *ModelInferController) enqueueModelInfer(mi *workloadv1alpha1.ModelInfer) {
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
	// TODO: add modelinfer rolling upgrade logic
	// TODO: Consider obtaining the pod status during the modelinfer reconcile to process the infergroup status. This can ensure the real-time status.
	klog.V(4).Info("Started syncing ModelInfer")
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", err)
	}

	mi, err := mic.modelInfersLister.ModelInfers(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(4).Infof("%v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	err = mic.manageReplicas(ctx, mi)
	if err != nil {
		return fmt.Errorf("cannot manage inferGroup replicas: %v", err)
	}

	if err := mic.UpdateModelInferStatus(mi); err != nil {
		return fmt.Errorf("failed to update status of mi %s/%s: %v", namespace, name, err)
	}

	//TODO: Add rolling upgrade function
	return nil
}

func (mic *ModelInferController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer mic.workqueue.ShutDown()

	// start informers
	go mic.podsInformer.RunWithContext(ctx)
	go mic.servicesInformer.RunWithContext(ctx)
	go mic.modelInfersInformer.RunWithContext(ctx)

	cache.WaitForCacheSync(ctx.Done(),
		mic.podsInformer.HasSynced,
		mic.servicesInformer.HasSynced,
		mic.modelInfersInformer.HasSynced,
	)

	klog.Info("start modelInfer controller")
	for i := 0; i < workers; i++ {
		go mic.worker(ctx)
	}
	<-ctx.Done()
	klog.Info("shut down modelInfer controller")
}

// UpdateModelInferConditionsStatus update conditions ModelInfer status.
func (mic *ModelInferController) UpdateModelInferConditionsStatus(mi *workloadv1alpha1.ModelInfer, condition metav1.Condition) error {
	if !meta.SetStatusCondition(&mi.Status.Conditions, condition) {
		return fmt.Errorf("failed to update modelInfer %s/%s status conditions", mi.GetNamespace(), mi.GetName())
	}
	return nil
}

// UpdateModelInferReplicasStatus update replicas in modelInfer status.
func (mic *ModelInferController) UpdateModelInferStatus(mi *workloadv1alpha1.ModelInfer) error {
	groups, err := mic.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	if err != nil {
		return err
	}

	available := 0
	progressingGroups := []int{}
	for index := range groups {
		// TODO: Handler of status == updating
		if groups[index].Status != datastore.InferGroupRunning {
			progressingGroups = append(progressingGroups, index)
		} else {
			available = available + 1
		}
	}

	shouldUpdate := utils.SetCondition(mi, progressingGroups)
	miCopy := mi.DeepCopy()
	miCopy.Status.Replicas = int32(len(groups))
	miCopy.Status.AvailableReplicas = int32(available)

	if shouldUpdate {
		_, err := mic.modelInferClient.WorkloadV1alpha1().ModelInfers(mi.GetNamespace()).UpdateStatus(context.TODO(), miCopy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (mic *ModelInferController) manageReplicas(ctx context.Context, mi *workloadv1alpha1.ModelInfer) error {
	inferGroupList, err := mic.store.GetInferGroupByModelInfer(utils.GetNamespaceName(mi))
	if err != nil {
		return fmt.Errorf("cannot get inferGroup from map: %v", err)
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
			err = mic.store.AddInferGroupForModelInfer(utils.GetNamespaceName(mi), idx)
			if err != nil {
				return fmt.Errorf("store infer group failed: %v", err)
			}
			// Create pods for inferGroup
			err = mic.CreatePodsForInferGroup(ctx, mi, idx)
			if err != nil {
				return fmt.Errorf("create infer group failed: %v", err)
			}
		}
	}
	for _, group := range condemned {
		mic.DeleteInferGroup(mi, group.Name)
	}
	return nil
}

func (mic *ModelInferController) CreatePodsForInferGroup(ctx context.Context, mi *workloadv1alpha1.ModelInfer, groupIndex int) error {
	// traverse each role in inferGroup to create entry-worker pod group.
	roleList := mi.Spec.Template.Roles
	for _, role := range roleList {
		// there will be multiple replicas in a role, such as xPyD type
		for roleIndex := range int(*role.Replicas) {
			err := mic.CreatePodByRole(ctx, role, mi, roleIndex, groupIndex)
			if err != nil {
				return fmt.Errorf("create role pod failed: %v, role name: %s, role index: %d", err, role.Name, roleIndex)
			}
		}
	}
	return nil
}

func (mic *ModelInferController) DeleteInferGroup(mi *workloadv1alpha1.ModelInfer, groupname string) {
	miNamedName := utils.GetNamespaceName(mi)
	inferGroupStatus := mic.store.GetInferGroupStatus(miNamedName, groupname)
	if inferGroupStatus == datastore.InferGroupNotFound {
		return
	}

	label := fmt.Sprintf("%s=%s", workloadv1alpha1.GroupNameLabelKey, groupname)
	selector := labels.SelectorFromSet(map[string]string{
		workloadv1alpha1.GroupNameLabelKey: groupname,
	})
	if inferGroupStatus != datastore.InferGroupDeleting {
		err := mic.store.UpdateInferGroupStatus(miNamedName, groupname, datastore.InferGroupDeleting)
		if err != nil {
			klog.Errorf("failed to set inferGroup %s/%s status: %v", miNamedName.Namespace+"/"+mi.Name, groupname, err)
			return
		}
		// Delete all pods in inferGroup
		err = mic.kubeClientSet.CoreV1().Pods(miNamedName.Namespace).DeleteCollection(
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
		services, err := mic.servicesLister.Services(miNamedName.Namespace).List(selector)
		if err != nil {
			klog.Errorf("failed to get service %v", err)
			return
		}
		for _, svc := range services {
			err = mic.kubeClientSet.CoreV1().Services(miNamedName.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
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
	pods, err := mic.podsLister.Pods(mi.GetNamespace()).List(selector)
	if err != nil {
		klog.Errorf("failed to get pod, err:%v", err)
	}
	services, err := mic.servicesLister.Services(miNamedName.Namespace).List(selector)
	if err != nil {
		klog.Errorf("failed to get service, err:%v", err)
	}
	if len(pods) == 0 && len(services) == 0 {
		klog.V(2).Infof("inferGroup %s has been deleted", groupname)
		_ = mic.store.DeleteInferGroupOfRunningPodMap(miNamedName, groupname)
		mic.enqueueModelInfer(mi)
		return
	}
}

func (mic *ModelInferController) CreatePodByRole(ctx context.Context, role workloadv1alpha1.Role, mi *workloadv1alpha1.ModelInfer, roleIndex, groupIndex int) error {
	groupName := utils.GenerateInferGroupName(mi.Name, groupIndex)
	// Create entry pod
	entryPod := utils.GenerateEntryPod(role, mi, groupName, roleIndex)
	_, err := mic.kubeClientSet.CoreV1().Pods(mi.Namespace).Create(ctx, entryPod, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("create entry pod failed: %v", err)
		return err
	}
	// Determine whether to create worker pods and headless service
	if role.WorkerTemplate == nil {
		klog.V(4).Info("workerTemplate is nil, no need to create worker pods and headless service")
		return nil
	}
	// Create headless service
	err = utils.CreateHeadlessService(ctx, mic.kubeClientSet, mi, entryPod.Spec.Subdomain, entryPod.ObjectMeta.Labels, groupName)
	if err != nil {
		klog.Errorf("create headless service failed: %v", err)
		return err
	}
	// Create worker pods
	for podIndex := range int(role.WorkerReplicas) {
		workerPod := utils.GenerateWorkerPod(role, mi, entryPod, groupName, roleIndex, podIndex+1) // worker-pod sequence number starts from 1, so we use index+1 here.
		_, err = mic.kubeClientSet.CoreV1().Pods(mi.Namespace).Create(ctx, workerPod, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("create worker pod failed: %v", err)
			return err
		}
	}
	return nil
}

func (mic *ModelInferController) handleReadyPod(mi *workloadv1alpha1.ModelInfer, inferGroupName string, newPod *corev1.Pod) error {
	// Add the running pod to the global storage and try to update the inferGroup status
	mic.store.AddRunningPodForInferGroup(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      inferGroupName,
	}, newPod.Name)
	ready, err := mic.checkInferGroupReady(mi, inferGroupName)
	if err != nil {
		return fmt.Errorf("failed to check inferGroup status, err: %v", err)
	}
	if ready {
		// All pods in the inferGroup are running, so the inferGroup status also needs to be set to running
		err = mic.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName, datastore.InferGroupRunning)
		if err != nil {
			return fmt.Errorf("failed to set inferGroup %s status: %v", inferGroupName, err)
		}
		klog.V(2).Infof("Update inferGroup %s status to Running", inferGroupName)
		mic.enqueueModelInfer(mi)
	} else {
		klog.V(4).Infof("inferGroup %s still creating", inferGroupName)
	}
	return nil
}

func (mic *ModelInferController) handleErrorPod(mi *workloadv1alpha1.ModelInfer, inferGroupName string, errPod *corev1.Pod) error {
	// pod is already in the grace period and does not need to be processed for the time being.
	_, exists := mic.graceMap.Load(utils.GetNamespaceName(errPod))
	now := time.Now()
	if exists {
		klog.V(4).Infof("Pod %v failed, waiting for grace time", utils.GetNamespaceName(errPod))
		return nil
	}
	// add pod to the grace period map
	mic.graceMap.Store(utils.GetNamespaceName(errPod), now)
	mic.store.DeleteRunningPodForInferGroup(types.NamespacedName{
		Namespace: mi.Namespace,
		Name:      inferGroupName,
	}, errPod.Name)
	// If the inferGroup status is already running, the status needs to be updated
	if groupStatus := mic.store.GetInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName); groupStatus == datastore.InferGroupRunning {
		err := mic.store.UpdateInferGroupStatus(utils.GetNamespaceName(mi), inferGroupName, datastore.InferGroupCreating)
		if err != nil {
			return fmt.Errorf("update infergroup status failed, err:%v", err)
		}
		klog.V(2).Infof("update infergroup %s to processing when pod fails", inferGroupName)
	}
	// Wait for the grace period before processing
	go mic.handlePodAfterGraceTime(mi, errPod)
	// InferGroup status may change, needs reconcile
	mic.enqueueModelInfer(mi)
	return nil
}

func (mic *ModelInferController) handlePodAfterGraceTime(mi *workloadv1alpha1.ModelInfer, errPod *corev1.Pod) {
	if mi.Spec.Template.RestartGracePeriodSeconds != nil && *mi.Spec.Template.RestartGracePeriodSeconds > 0 {
		// Wait for the grace period before making a decision
		time.Sleep(time.Duration(*mi.Spec.Template.RestartGracePeriodSeconds) * time.Second)
		klog.V(4).Infof("%s after grace time", errPod.Name)
		defer mic.graceMap.Delete(utils.GetNamespaceName(errPod))

		newPod, err := mic.podsLister.Pods(mi.Namespace).Get(errPod.Name)
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
			err = mic.kubeClientSet.CoreV1().Pods(mi.Namespace).Delete(context.TODO(), newPod.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("cannot delete pod %s after grace time, err: %v", newPod.Name, err)
				return
			}
			klog.V(2).Infof("%s been deleted after grace time", errPod.Name)
		}
	} else {
		// grace period is not set or the grace period is 0, the deletion will be executed immediately.
		defer mic.graceMap.Delete(utils.GetNamespaceName(errPod))

		err := mic.kubeClientSet.CoreV1().Pods(mi.Namespace).Delete(context.TODO(), errPod.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("cannot delete pod %s when it error, err: %v", errPod.Name, err)
			return
		}
		klog.V(2).Infof("%s been deleted without grace time", errPod.Name)
	}
}

func (mic *ModelInferController) checkInferGroupReady(mi *workloadv1alpha1.ModelInfer, inferGroupName string) (bool, error) {
	// TODO: modify inferGroupReady logic after rolling update functionality is implemented
	runningPodList, err := mic.store.GetRunningPodByInferGroup(utils.GetNamespaceName(mi), inferGroupName)
	if err != nil {
		return false, err
	}
	if len(runningPodList) != utils.ExpectedPodNum(mi) {
		// the number of running pods does not reach the expected number
		return false, nil
	}
	return true, nil
}

func (mic *ModelInferController) getModelInfer(pod *corev1.Pod) (*workloadv1alpha1.ModelInfer, string, error) {
	modelInferName, inferGroupName, ok := utils.GetModelInferAndGroupByLabel(pod.GetLabels())
	if !ok {
		return nil, "", fmt.Errorf("cannot get modelInfer name and inferGroup name from pod %s", pod.Name)
	}
	mi, err := mic.modelInfersLister.ModelInfers(pod.Namespace).Get(modelInferName)
	if err != nil {
		return nil, "", err
	}
	return mi, inferGroupName, nil
}
