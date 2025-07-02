package main

import (
	"context"
	"fmt"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/controller"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultLeaseDuration   = 15 * time.Second
	defaultRenewDeadline   = 10 * time.Second
	defaultRetryPeriod     = 2 * time.Second
	leaderElectionId       = "matrixinfer.model-controller"
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// main starts model controller.
// It will run forever until an error has occurred or the context is cancelled.
func main() {
	var kubeconfig string
	var master string
	var workers int
	var enableLeaderElection bool

	pflag.StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig file path")
	pflag.StringVar(&master, "master", "", "master URL")
	pflag.IntVar(&workers, "workers", 5, "number of workers to run")
	pflag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller. "+
		"Enabling this will ensure there is only one active model controller. Default is false.")
	pflag.Parse()

	// create clientset
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatalf("build client config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create k8s client: %v", err)
	}

	client, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create Model client: %v", err)
	}
	// create Model controller
	mc := controller.NewModelController(kubeClient, client)

	var leaderElector *leaderelection.LeaderElector
	if enableLeaderElection {
		leaderElector, err = initLeaderElector(config, leaderElector, mc, workers)
		if err != nil {
			klog.Error(err)
			return
		}
	}
	// Start the leader election and all required runnable
	{
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if leaderElector != nil {
			// Start the leader elector process
			leaderElector.Run(ctx)
			<-ctx.Done()
		} else {
			// Normal start, not use elector
			go mc.Run(ctx, workers)
			klog.Info("Started Model controller")
		}
	}
	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	klog.Info("existing")
}

func initLeaderElector(config *rest.Config, leaderElector *leaderelection.LeaderElector, mc *controller.ModelController,
	workers int) (*leaderelection.LeaderElector, error) {
	resourceLock, err := newResourceLock(config)
	if err != nil {
		return nil, err
	}
	leaderElector, err = leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// become leader
				go mc.Run(ctx, workers)
			},
			OnStoppedLeading: func() {
				// become follower
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

func newResourceLock(config *rest.Config) (resourcelock.Interface, error) {
	namespace, err := getInClusterNameSpace()
	if err != nil {
		return nil, err
	}
	// Leader id, should be unique
	id, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	id = id + "_" + string(uuid.NewUUID())
	return resourcelock.NewFromKubeconfig(resourcelock.LeasesResourceLock, namespace, leaderElectionId,
		resourcelock.ResourceLockConfig{
			Identity: id,
		}, config, defaultRenewDeadline,
	)
}

func getInClusterNameSpace() (string, error) {
	if _, err := os.Stat(inClusterNamespacePath); errors.IsNotFound(err) {
		return "", fmt.Errorf("not running in-cluster, please specify namespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %v", err)
	}
	// Load the namespace file and return its content
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %v", err)
	}
	return string(namespace), nil
}
