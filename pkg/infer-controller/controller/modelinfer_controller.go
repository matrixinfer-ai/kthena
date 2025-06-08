package controller

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	modelInfersLister   listerv1alpha1.ModelInferLister
	modelInfersInformer cache.Controller

	// nolint
	workqueue workqueue.RateLimitingInterface
	store     datastore.Store
}

func NewModelInferController(kubeClientSet kubernetes.Interface, modelInferClient clientset.Interface) *ModelInferController {
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClientSet, 0)
	podsInformer := kubeInformerFactory.Core().V1().Pods()
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
		modelInfersLister:   modelInferInformer.Lister(),
		modelInfersInformer: modelInferInformer.Informer(),
		// nolint
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ModelInfers"),
		store:     store,
	}

	klog.Info("Set the ModelInfer event handler")
	_, _ = modelInferInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

func (mic *ModelInferController) addMI(obj interface{}) {
	mi, ok := obj.(*workloadv1alpha1.ModelInfer)
	if !ok {
		klog.Error("failed to parse ModelInfer type when addMI")
		return
	}
	klog.V(4).Info("Adding", "modelinfer", klog.KObj(mi))
	mic.enqueueMI(mi)
}

func (mic *ModelInferController) updateMI(old, cur interface{}) {
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
		mic.enqueueMI(curMI)
	}
}

func (mic *ModelInferController) deleteMI(obj interface{}) {
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
}

func (mic *ModelInferController) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Error("failed to parse pod type when deletePod")
		return
	}

	podLabels := pod.GetLabels()
	if podLabels == nil {
		return
	}

	inferGroupName, ok := podLabels[workloadv1alpha1.GroupNameLabelKey]
	if !ok {
		klog.Errorf("failed to get infergroupName of pod %s/%s", pod.GetNamespace(), pod.GetName())
		return
	}

	owners := pod.GetOwnerReferences()
	for i := range owners {
		if owners[i].Kind == workloadv1alpha1.ModelInferKind.Kind {
			mi, err := mic.modelInfersLister.ModelInfers(pod.GetNamespace()).Get(owners[i].Name)
			if err == nil {
				mic.DeleteInferGroup(mi, inferGroupName)
			} else {
				klog.Errorf("failed to get modelInfer of pod %s/%s: %v", pod.GetNamespace(), pod.GetName(), err)
			}
		}
	}
}

func (mic *ModelInferController) enqueueMI(mi *workloadv1alpha1.ModelInfer) {
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
	//TODO: Add rolling upgrade function
	return nil
}

func (mic *ModelInferController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer mic.workqueue.ShutDown()

	// start informers
	go mic.podsInformer.RunWithContext(ctx)
	go mic.modelInfersInformer.RunWithContext(ctx)

	cache.WaitForCacheSync(ctx.Done(),
		mic.podsInformer.HasSynced,
		mic.modelInfersInformer.HasSynced,
	)

	klog.Info("start modelInfer controller")
	for i := 0; i < workers; i++ {
		go mic.worker(ctx)
	}
	<-ctx.Done()
	klog.Info("shut down modelInfer controller")
}

// UpdateStatus update ModelInfer status.
func (mic *ModelInferController) UpdateStatus() {
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
	roleList := mi.Spec.Template.Spec.Roles
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
	}

	// check whether the deletion has been completed
	selector := labels.SelectorFromSet(map[string]string{
		workloadv1alpha1.GroupNameLabelKey: groupname,
	})
	pods, err := mic.podsLister.Pods(mi.GetNamespace()).List(selector)
	if err != nil {
		klog.Errorf("failed to get ")
	}
	if len(pods) == 0 {
		_ = mic.store.DeleteInferGroupOfRunningPodMap(miNamedName, groupname)
		mic.enqueueMI(mi)
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
	err = utils.CreateHeadlessService(ctx, mic.kubeClientSet, mi, entryPod.Spec.Subdomain, entryPod.ObjectMeta.Labels)
	if err != nil {
		klog.Errorf("create headless service failed: %v", err)
		return err
	}
	// Create worker pods
	for podIndex := range int(*role.WorkerReplicas) {
		workerPod := utils.GenerateWorkerPod(role, mi, entryPod, groupName, roleIndex, podIndex+1) // worker-pod sequence number starts from 1, so we use index+1 here.
		_, err = mic.kubeClientSet.CoreV1().Pods(mi.Namespace).Create(ctx, workerPod, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("create worker pod failed: %v", err)
			return err
		}
	}
	return nil
}
