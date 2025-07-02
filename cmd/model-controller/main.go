package main

import (
	"context"
	"fmt"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/controller"
	"os"
	"os/signal"
	"sync/atomic"
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
	var startedLeading atomic.Bool

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

	kubeClient := kubernetes.NewForConfigOrDie(config)
	client := clientset.NewForConfigOrDie(config)
	// create Model controller
	mc := controller.NewModelController(kubeClient, client)

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
		lock, err := newResourceLock(kubeClient)
		if err != nil {
			panic(err)
		}
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:          lock,
			LeaseDuration: defaultLeaseDuration,
			RenewDeadline: defaultRenewDeadline,
			RetryPeriod:   defaultRetryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					// become leader, start controller
					startedLeading.Store(true)
					go mc.Run(ctx, workers)
					klog.Info("Started Model controller")
				},
				OnStoppedLeading: func() {
					// become follower, do cleanup if lead before
					if startedLeading.Load() {
						klog.Info("Performing cleanup")
					} else {
						klog.Info("No need to cleanup")
					}
					os.Exit(0)
				},
			},
			ReleaseOnCancel: false,
			Name:            leaderElectionId,
		})
	} else {
		go mc.Run(ctx, workers)
	}
}

func newResourceLock(client kubernetes.Interface) (*resourcelock.LeaseLock, error) {
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
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "lease-lock-name",
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}, nil
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
