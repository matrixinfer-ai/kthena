/*
Copyright MatrixInfer-AI Authors.

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

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/webhook"
)

func main() {
	// Initialize klog flags
	klog.InitFlags(nil)
	webhookPort := pflag.Int("webhook-port", 8443, "The port for the webhook server.")
	webhookCertFile := pflag.String("webhook-cert-file", "", "The path to the webhook TLS certificate file.")
	webhookKeyFile := pflag.String("webhook-key-file", "", "The path to the webhook TLS private key file.")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	defer klog.Flush()

	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to get kube config: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to get kube client: %v", err)
	}
	matrixInferClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to get matrix infer client: %v", err)
	}

	validator := webhook.NewInferGatewayValidator(kubeClient, matrixInferClient, *webhookPort)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	// Wait for a signal
	go func() {
		<-signalCh
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()

	validator.Run(*webhookCertFile, *webhookKeyFile, ctx.Done())
}
