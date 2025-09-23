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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	"github.com/volcano-sh/kthena/pkg/modelServing-controller/webhook"
)

type modelInferConfig struct {
	kubeconfig  string
	masterURL   string
	tlsCertFile string
	tksKeyFile  string
	port        int
}

func parseConfig() (modelInferConfig, error) {
	var config modelInferConfig
	pflag.StringVar(&config.kubeconfig, "kubeconfig", "", "kubeconfig file path")
	pflag.StringVar(&config.masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	pflag.StringVar(&config.tlsCertFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS. This can be used as a fallback when cert-manager is not available.")
	pflag.StringVar(&config.tksKeyFile, "tls-private-key-file", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tls-cert-file. This can be used as a fallback when cert-manager is not available.")
	pflag.IntVar(&config.port, "port", 8443, "Secure port that the webhook listens on")

	if config.port <= 0 || config.port > 65535 {
		return config, fmt.Errorf("invalid port: %d", config.port)
	}
	return config, nil
}

func main() {
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	config, err := parseConfig()
	if err != nil {
		klog.Fatalf("Config error: %v", err)
	}
	pflag.Parse()

	// Set up signals so we handle the first shutdown signal gracefully
	stopCh := setupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(config.masterURL, config.kubeconfig)
	if err != nil {
		klog.Fatalf("build client config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create k8s client: %v", err)
	}

	modelInferClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create ModelInfer client: %v", err)
	}
	// create modelInfer validator
	validator := webhook.NewModelInferValidator(kubeClient, modelInferClient, config.port)

	klog.Info("Started ModelInfer validator")
	go func() {
		validator.Run(config.tlsCertFile, config.tksKeyFile, stopCh)
	}()

	<-stopCh
	klog.Info("Shutting down webhook server")
}

// TODO: share this function with other components
func setupSignalHandler() <-chan struct{} {
	stopCh := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stopCh)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stopCh
}
