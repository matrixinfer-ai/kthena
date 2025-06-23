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

func main() {
	var kubeconfig string
	var master string
	var workers int

	pflag.StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig file path")
	pflag.StringVar(&master, "master", "", "master URL")
	pflag.IntVar(&workers, "workers", 5, "number of workers to run")
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

	modelClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create Model client: %v", err)
	}
	// create Model controller
	mic := controller.NewModelController(kubeClient, modelClient)

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
