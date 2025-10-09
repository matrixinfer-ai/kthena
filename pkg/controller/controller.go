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

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	autoscaler "github.com/volcano-sh/kthena/pkg/autoscaler/controller"
	modelbooster "github.com/volcano-sh/kthena/pkg/model-booster-controller/controller"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/utils"
	modelserving "github.com/volcano-sh/kthena/pkg/model-serving-controller/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	volcanoClientSet "volcano.sh/apis/pkg/client/clientset/versioned"
)

const (
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
	leaderElectionId     = "kthena.controller-manager"
	leaseName            = "lease.kthena.controller-manager"
)

func SetupController(ctx context.Context, cc Config) {
	config, err := clientcmd.BuildConfigFromFlags(cc.MasterURL, cc.Kubeconfig)
	if err != nil {
		klog.Fatalf("build client config: %v", err)
	}
	kubeClient := kubernetes.NewForConfigOrDie(config)
	client := clientset.NewForConfigOrDie(config)
	volcanoClient, err := volcanoClientSet.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create volcano client: %v", err)
	}
	mc := modelbooster.NewModelBoosterController(kubeClient, client)
	msc, err := modelserving.NewModelServingController(kubeClient, client, volcanoClient)
	if err != nil {
		klog.Fatalf("failed to create ModelServing controller: %v", err)
	}
	namespace, err := utils.GetInClusterNameSpace()
	if err != nil {
		klog.Fatalf("create Autoscaler client: %v", err)
	}
	ac := autoscaler.NewAutoscaleController(kubeClient, client, namespace)
	if cc.EnableLeaderElection {
		startedLeading := func(ctx context.Context) {
			go mc.Run(ctx, cc.Workers)
			go msc.Run(ctx, cc.Workers)
			go ac.Run(ctx)
			klog.Info("Start as leader")
		}
		leaderElector, err := initLeaderElector(kubeClient, startedLeading)
		if err != nil {
			panic(err)
		}
		leaderElector.Run(ctx)
	} else {
		go mc.Run(ctx, cc.Workers)
		go msc.Run(ctx, cc.Workers)
		go ac.Run(ctx)
		klog.Info("Started controller without leader election")
	}
	<-ctx.Done()
}

// initLeaderElector inits a leader elector for leader election
func initLeaderElector(kubeClient kubernetes.Interface, startedLeading func(ctx context.Context)) (*leaderelection.LeaderElector, error) {
	resourceLock, err := newResourceLock(kubeClient)
	if err != nil {
		return nil, err
	}
	leaderElector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: startedLeading,
			OnStoppedLeading: func() {
				klog.Error("leader election lost")
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
func newResourceLock(client kubernetes.Interface) (*resourcelock.LeaseLock, error) {
	namespace, err := utils.GetInClusterNameSpace()
	if err != nil {
		return nil, err
	}
	// Leader id, should be unique
	id, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	id = id + "_" + string(uuid.NewUUID())
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}, nil
}
