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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/volcano-sh/kthena/pkg/model-controller/utils"

	"github.com/spf13/pflag"
	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	"github.com/volcano-sh/kthena/pkg/autoscaler/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

const (
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
	leaderElectionId     = "kthena.autoscaler"
	leaseName            = "lease.kthena.autoscaler"
)

func main() {
	var kubeconfig string
	var master string
	var enableLeaderElection bool

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig file path")
	pflag.StringVar(&master, "master", "", "master URL")
	pflag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller. "+
		"Enabling this will ensure there is only one active model controller. Default is false.")

	pflag.Parse()

	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		// print all flags for debugging
		klog.Infof("Flag: %s, Value: %s", f.Name, f.Value.String())
	})

	// create clientset
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatalf("build client config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create k8s client: %v", err)
	}

	autoscalingClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create Autoscaler client: %v", err)
	}

	namespace, err := utils.GetInClusterNameSpace()
	if err != nil {
		klog.Fatalf("create Autoscaler client: %v", err)
	}
	// create Autoscale controller
	asc := controller.NewAutoscaleController(kubeClient, autoscalingClient, namespace)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()
	if enableLeaderElection {
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

// initLeaderElector inits a leader elector for leader election
func initLeaderElector(kubeClient kubernetes.Interface, mc *controller.AutoscaleController, namespace string) (*leaderelection.LeaderElector, error) {
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
				go mc.Run(ctx)
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
