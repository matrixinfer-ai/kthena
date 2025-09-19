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
	"github.com/volcano-sh/kthena/pkg/registry-webhook/server"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type config struct {
	masterURL      string
	tlsCertFile    string
	tlsPrivateKey  string
	port           int
	webhookTimeout int
}

func parseConfig() config {
	var cfg config
	pflag.StringVar(&cfg.masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	pflag.StringVar(&cfg.tlsCertFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS. This can be used as a fallback when cert-manager is not available.")
	pflag.StringVar(&cfg.tlsPrivateKey, "tls-private-key-file", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tls-cert-file. This can be used as a fallback when cert-manager is not available.")
	pflag.IntVar(&cfg.port, "port", 8443, "Secure port that the webhook listens on")
	pflag.IntVar(&cfg.webhookTimeout, "webhook-timeout", 30, "Timeout for webhook operations in seconds")
	return cfg
}

func main() {
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	config := parseConfig()
	pflag.Parse()

	// Set up signals so we handle the first shutdown signal gracefully
	stopCh := setupSignalHandler()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kthenaClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kthena clientset: %s", err.Error())
	}

	// Create webhook server
	webhookServer := server.NewWebhookServer(
		kthenaClient,
		config.tlsCertFile,
		config.tlsPrivateKey,
		config.port,
		config.webhookTimeout,
	)

	// Start webhook server
	go func() {
		if err := webhookServer.Start(stopCh); err != nil {
			klog.Fatalf("Failed to start webhook server: %v", err)
		}
	}()

	<-stopCh
	klog.Info("Shutting down webhook server")
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
