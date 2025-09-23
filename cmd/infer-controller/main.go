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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/spf13/pflag"
	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	volcanoClientSet "volcano.sh/apis/pkg/client/clientset/versioned"

	"github.com/volcano-sh/kthena/pkg/modelServing-controller/controller"
)

type modelInferConfig struct {
	kubeconfig string
	masterURL  string
	workers    int
}

func parseConfig() (modelInferConfig, error) {
	var config modelInferConfig
	pflag.StringVar(&config.kubeconfig, "kubeconfig", "", "kubeconfig file path")
	pflag.StringVar(&config.masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	pflag.IntVar(&config.workers, "workers", 5, "number of workers to run")
	pflag.Parse()
	return config, nil
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	config, err := parseConfig()
	if err != nil {
		klog.Fatalf("Config error: %v", err)
	}

	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		// print all flags for debugging
		klog.Infof("Flag: %s, Value: %s", f.Name, f.Value.String())
	})

	// create clientset
	restConfig, err := clientcmd.BuildConfigFromFlags(config.masterURL, config.kubeconfig)
	if err != nil {
		klog.Fatalf("build client config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("failed to create k8s client: %v", err)
	}

	volcanoClient, err := volcanoClientSet.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("failed to create volcano client: %v", err)
	}

	modelInferClient, err := clientset.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("failed to create ModelInfer client: %v", err)
	}
	// create ModelInfer controller
	mic, err := controller.NewModelInferController(kubeClient, modelInferClient, volcanoClient)
	if err != nil {
		klog.Fatalf("failed to create ModelInfer controller: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Start controller
	go mic.Run(ctx, config.workers)
	klog.Info("Started ModelInfer controller")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	klog.Info("existing")
}
