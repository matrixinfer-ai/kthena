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
	"os"
	"time"

	"github.com/volcano-sh/kthena/pkg/autoscaler/autoscaler"
	"github.com/volcano-sh/kthena/pkg/controller"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/utils"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	informersv1alpha1 "github.com/volcano-sh/kthena/client-go/informers/externalversions"
	workloadLister "github.com/volcano-sh/kthena/client-go/listers/workload/v1alpha1"
	workload "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/autoscaler/algorithm"
	"github.com/volcano-sh/kthena/pkg/autoscaler/util"
	"istio.io/istio/pkg/util/sets"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
	leaderElectionId     = "kthena.autoscaler"
	leaseName            = "lease.kthena.autoscaler"
)

type AutoscaleController struct {
	// Client for k8s. Use it to call K8S API
	kubeClient kubernetes.Interface
	// client for custom resource
	client                             clientset.Interface
	namespace                          string
	autoscalingPoliciesLister          workloadLister.AutoscalingPolicyLister
	autoscalingPoliciesInformer        cache.Controller
	autoscalingPoliciesBindingLister   workloadLister.AutoscalingPolicyBindingLister
	autoscalingPoliciesBindingInformer cache.Controller
	modelServingLister                 workloadLister.ModelServingLister
	modelServingInformer               cache.Controller
	podsLister                         listerv1.PodLister
	podsInformer                       cache.Controller
	scalerMap                          map[string]*autoscaler.Autoscaler
	optimizerMap                       map[string]*autoscaler.Optimizer
}

func SetupAutoscaleController(ctx context.Context, cc controller.Config, kubeClient *kubernetes.Clientset, client *clientset.Clientset) {
	namespace, err := utils.GetInClusterNameSpace()
	if err != nil {
		klog.Fatalf("create Autoscaler client: %v", err)
	}

	asc := NewAutoscaleController(kubeClient, client, namespace)

	if cc.EnableLeaderElection {
		leaderElector, err := initLeaderElector(kubeClient, asc, namespace)
		if err != nil {
			klog.Fatalf("failed to init leader elector: %v", err)
			panic(err)
		}
		// Start the leader elector process
		leaderElector.Run(ctx)
	} else {
		go asc.Run(ctx)
		klog.Info("Started autoscaler without leader election")
	}
	<-ctx.Done()
}

func initLeaderElector(kubeClient *kubernetes.Clientset, asc *AutoscaleController, namespace string) (*leaderelection.LeaderElector, error) {
	resourceLock, err := newResourceLock(kubeClient, namespace)
	if err != nil {
		return nil, err
	}
	leaderElector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				go asc.Run(ctx)
				klog.Info("Started autoscaler as leader")
			},
			OnStoppedLeading: func() {
				klog.Fatalf("leader election lost")
			},
		},
		ReleaseOnCancel: false,
		Name:            leaderElectionId,
	})
	if err != nil {
		return nil, err
	}
	return leaderElector, nil
}

// newResourceLock returns a lease lock which is used to elect leader
func newResourceLock(client kubernetes.Interface, namespace string) (*resourcelock.LeaseLock, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: hostname + "_" + string(uuid.NewUUID()),
		},
	}, nil
}

func NewAutoscaleController(kubeClient kubernetes.Interface, client clientset.Interface, namespace string) *AutoscaleController {
	informerFactory := informersv1alpha1.NewSharedInformerFactory(client, 0)
	modelInferInformer := informerFactory.Workload().V1alpha1().ModelServings()
	autoscalingPoliciesInformer := informerFactory.Workload().V1alpha1().AutoscalingPolicies()
	autoscalingPoliciesBindingInformer := informerFactory.Workload().V1alpha1().AutoscalingPolicyBindings()

	selector, err := labels.NewRequirement(workload.GroupNameLabelKey, selection.Exists, nil)
	if err != nil {
		klog.Errorf("can not create label selector,err:%v", err)
		return nil
	}
	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector.String()
		}),
	)
	podsInformer := kubeInformerFactory.Core().V1().Pods()
	ac := &AutoscaleController{
		kubeClient:                         kubeClient,
		client:                             client,
		namespace:                          namespace,
		autoscalingPoliciesLister:          autoscalingPoliciesInformer.Lister(),
		autoscalingPoliciesInformer:        autoscalingPoliciesInformer.Informer(),
		autoscalingPoliciesBindingLister:   autoscalingPoliciesBindingInformer.Lister(),
		autoscalingPoliciesBindingInformer: autoscalingPoliciesBindingInformer.Informer(),
		modelServingLister:                 modelInferInformer.Lister(),
		modelServingInformer:               modelInferInformer.Informer(),
		podsLister:                         podsInformer.Lister(),
		podsInformer:                       podsInformer.Informer(),
		scalerMap:                          make(map[string]*autoscaler.Autoscaler),
		optimizerMap:                       make(map[string]*autoscaler.Optimizer),
	}
	return ac
}

func (ac *AutoscaleController) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()

	// start informers
	go ac.autoscalingPoliciesInformer.RunWithContext(ctx)
	go ac.autoscalingPoliciesBindingInformer.RunWithContext(ctx)
	go ac.modelServingInformer.RunWithContext(ctx)
	go ac.podsInformer.RunWithContext(ctx)
	cache.WaitForCacheSync(ctx.Done(),
		ac.autoscalingPoliciesInformer.HasSynced,
		ac.autoscalingPoliciesBindingInformer.HasSynced,
		ac.modelServingInformer.HasSynced,
		ac.podsInformer.HasSynced,
	)

	klog.Info("start autoscale controller")
	go wait.Until(func() {
		ac.Reconcile(ctx)
	}, util.AutoscalingSyncPeriodSeconds*time.Second, nil)

	<-ctx.Done()
	klog.Info("shut down autoscale controller")
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (ac *AutoscaleController) Reconcile(ctx context.Context) {
	klog.InfoS("start to reconcile")
	ctx, cancel := context.WithTimeout(ctx, util.AutoscaleCtxTimeoutSeconds*time.Second)
	defer cancel()
	bindingList, err := ac.client.WorkloadV1alpha1().AutoscalingPolicyBindings(ac.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("failed to list autoscaling policy bindings, err:%v", err)
		return
	}

	scalerSet := sets.New[string]()
	optimizerSet := sets.New[string]()

	for _, binding := range bindingList.Items {
		policyName := binding.Spec.PolicyRef.Name
		klog.InfoS("global", "autoscalingPolicyName", policyName)
		if policyName == "" {
			continue
		}
		if binding.Spec.ScalingConfiguration != nil {
			target := binding.Spec.ScalingConfiguration.Target
			instanceKey := formatAutoscalerMapKey(binding.Name, target.TargetRef.Name)
			scalerSet.Insert(instanceKey)
		} else if binding.Spec.OptimizerConfiguration != nil {
			autoscalerMapKey := formatAutoscalerMapKey(binding.ObjectMeta.Name, "")
			optimizerSet.Insert(autoscalerMapKey)
		}
	}

	for key := range ac.scalerMap {
		if !scalerSet.Contains(key) {
			delete(ac.scalerMap, key)
		}
	}

	for key := range ac.optimizerMap {
		if !optimizerSet.Contains(key) {
			delete(ac.optimizerMap, key)
		}
	}

	klog.InfoS("start to process autoscale")
	for _, binding := range bindingList.Items {
		err := ac.schedule(ctx, &binding)
		if err != nil {
			klog.Errorf("failed to process autoscale,err:%v", err)
			continue
		}
	}
}

func (ac *AutoscaleController) schedule(ctx context.Context, binding *workload.AutoscalingPolicyBinding) error {
	klog.InfoS("start to process autoscale", "namespace", binding.Namespace, "model name", binding.Name)
	autoscalePolicy, err := ac.getAutoscalePolicy(binding.Spec.PolicyRef.Name, binding.Namespace)
	if err != nil {
		klog.Errorf("get autoscale policy error: %v", err)
		return err
	}
	metricTargets := getMetricTargets(autoscalePolicy)
	if binding.Spec.OptimizerConfiguration != nil {
		optimizerKey := formatAutoscalerMapKey(binding.Name, "")
		optimizer, ok := ac.optimizerMap[optimizerKey]
		if !ok {
			optimizer = autoscaler.NewOptimizer(&autoscalePolicy.Spec.Behavior, binding, metricTargets)
			ac.optimizerMap[optimizerKey] = optimizer
		}
		if err := optimizer.Optimize(ctx, ac.client, ac.modelServingLister, ac.podsLister, autoscalePolicy); err != nil {
			klog.Errorf("failed to do optimize, err: %v", err)
			return err
		}
	} else if binding.Spec.ScalingConfiguration != nil {
		target := binding.Spec.ScalingConfiguration.Target
		instanceKey := formatAutoscalerMapKey(binding.Name, target.TargetRef.Name)
		scalingAutoscaler, ok := ac.scalerMap[instanceKey]
		if !ok {
			scalingAutoscaler = autoscaler.NewAutoscaler(&autoscalePolicy.Spec.Behavior, binding, metricTargets)
			ac.scalerMap[instanceKey] = scalingAutoscaler
		}
		if err := scalingAutoscaler.Scale(ctx, ac.client, ac.modelServingLister, ac.podsLister, autoscalePolicy); err != nil {
			klog.Errorf("failed to do scaling, err: %v", err)
			return err
		}
	} else {
		klog.Warningf("binding %s has no scalingConfiguration and optimizerConfiguration", binding.Name)
	}

	klog.InfoS("schedule end")
	return nil
}

func (ac *AutoscaleController) getAutoscalePolicy(autoscalingPolicyName string, namespace string) (*workload.AutoscalingPolicy, error) {
	autoscalingPolicy, err := ac.autoscalingPoliciesLister.AutoscalingPolicies(namespace).Get(autoscalingPolicyName)
	if err != nil {
		klog.Errorf("can not get autosalingpolicyname: %s, error: %v", autoscalingPolicyName, err)
		return nil, client.IgnoreNotFound(err)
	}
	return autoscalingPolicy, nil
}

func formatAutoscalerMapKey(bindingName string, instanceName string) string {
	if instanceName == "" {
		return bindingName
	}
	return bindingName + "#" + instanceName
}

func getMetricTargets(autoscalePolicy *workload.AutoscalingPolicy) algorithm.Metrics {
	metricTargets := algorithm.Metrics{}
	if autoscalePolicy == nil {
		klog.Warning("autoscalePolicy is nil, can't get metricTargets")
		return metricTargets
	}

	for _, metric := range autoscalePolicy.Spec.Metrics {
		metricTargets[metric.MetricName] = metric.TargetValue.AsFloat64Slow()
	}
	return metricTargets
}
