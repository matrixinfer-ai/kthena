package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/controller"
)

func main() {

	var kubeconfig string
	var master string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig file path")
	flag.StringVar(&master, "master", "", "master URL")
	flag.Parse()

	// create clientset
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatalf("build client config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create k8s client: %v", err)
	}

	modelInferClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to create ModelInfer client: %v", err)
	}

	// create ModelInfer controller
	mic := controller.NewModelInferController(kubeClient, modelInferClient)

	// Start controller
	go mic.Run()
	klog.Info("Started ModelInfer controller")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	klog.Info("Get os signal")
}
