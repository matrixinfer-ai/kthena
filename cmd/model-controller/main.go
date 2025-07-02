package main

import (
	"context"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	"matrixinfer.ai/matrixinfer/pkg/model-controller/controller"
	"os"
	"os/signal"
	"syscall"
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
	mic := controller.NewModelController(kubeClient, client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Start controller
	go mic.Run(ctx, workers)
	klog.Info("Started Model controller")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	klog.Info("existing")
}
