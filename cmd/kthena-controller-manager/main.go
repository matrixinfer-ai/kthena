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
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	"github.com/volcano-sh/kthena/pkg/model-booster-webhook/server"
	"github.com/volcano-sh/kthena/pkg/model-serving-controller/webhook"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type webhookConfig struct {
	tlsCertFile    string
	tlsPrivateKey  string
	port           int
	webhookTimeout int
}

func main() {
	var enableWebhook bool
	var masterURL string
	var wc webhookConfig
	// Initialize klog flags
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.BoolVar(&enableWebhook, "enable-Webhook", true, "If true, webhook will be used. Default is true")
	pflag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	pflag.StringVar(&wc.tlsCertFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS. This can be used as a fallback when cert-manager is not available.")
	pflag.StringVar(&wc.tlsPrivateKey, "tls-private-key-file", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tls-cert-file. This can be used as a fallback when cert-manager is not available.")
	pflag.IntVar(&wc.port, "port", 8443, "Secure port that the webhook listens on")
	pflag.IntVar(&wc.webhookTimeout, "webhook-timeout", 30, "Timeout for webhook operations in seconds")

	pflag.Parse()
	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		klog.Infof("Flag: %s, Value: %s", f.Name, f.Value.String())
	})

	if enableWebhook {
		if err := setupWebhook(wc); err != nil {
			os.Exit(1)
		}
	}
}

func setupWebhook(wc webhookConfig) error {
	// Set up signals so we handle the first shutdown signal gracefully
	stopCh := setupSignalHandler()
	if err := setupModelBoosterWebhook(wc, stopCh); err != nil {
		return err
	}
	if err := setupModelServingWebhook(wc, stopCh); err != nil {
		return err
	}
	return nil
}

func setupModelServingWebhook(wc webhookConfig, stopCh <-chan struct{}) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("build client config: %v", err)
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create k8s client: %v", err)
		return err
	}
	kthenaClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create kthenaClient: %v", err)
		return err
	}
	modelServingWebhook := webhook.NewModelServingValidator(kubeClient, kthenaClient, wc.port)
	go func() {
		modelServingWebhook.Run(wc.tlsCertFile, wc.tlsPrivateKey, stopCh)
	}()
	return nil
}

func setupModelBoosterWebhook(wc webhookConfig, stopCh <-chan struct{}) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
		return err
	}
	kthenaClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kthena clientset: %s", err.Error())
		return err
	}
	modelBoosterWebhook := server.NewWebhookServer(
		kthenaClient,
		wc.tlsCertFile,
		wc.tlsPrivateKey,
		wc.port,
		wc.webhookTimeout,
	)
	go func() {
		if err := modelBoosterWebhook.Start(stopCh); err != nil {
			klog.Fatalf("Failed to start webhook server: %v", err)
		}
	}()
	return nil
}

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
