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
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	"github.com/volcano-sh/kthena/cmd/infer-router/app"
	routerwebhook "github.com/volcano-sh/kthena/pkg/infer-router/webhook"
)

func main() {
	var (
		routerPort    string
		tlsCert       string
		tlsKey        string
		enableWebhook bool
		webhookPort   int
		webhookCert   string
		webhookKey    string
	)

	// Initialize klog flags
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.StringVar(&routerPort, "port", "8080", "Server listen port")
	pflag.StringVar(&tlsCert, "tls-cert", "", "TLS certificate file path")
	pflag.StringVar(&tlsKey, "tls-key", "", "TLS key file path")
	pflag.BoolVar(&enableWebhook, "enable-webhook", true, "Enable built-in admission webhook server")
	pflag.IntVar(&webhookPort, "webhook-port", 8443, "The port for the webhook server")
	pflag.StringVar(&webhookCert, "webhook-tls-cert-file", "/etc/webhook/certs/tls.crt", "Path to the webhook TLS certificate file")
	pflag.StringVar(&webhookKey, "webhook-tls-private-key-file", "/etc/webhook/certs/tls.key", "Path to the webhook TLS private key file")
	defer klog.Flush()
	pflag.Parse()

	if (tlsCert != "" && tlsKey == "") || (tlsCert == "" && tlsKey != "") {
		klog.Fatal("tls-cert and tls-key must be specified together")
	}

	if enableWebhook {
		if webhookCert == "" || webhookKey == "" {
			klog.Fatal("webhook TLS cert and key must be specified when webhook is enabled")
		}
	}

	if webhookPort <= 0 || webhookPort > 65535 {
		klog.Fatalf("invalid webhook port: %d", webhookPort)
	}

	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		// print all flags for debugging
		klog.Infof("Flag: %s, Value: %s", f.Name, f.Value.String())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalCh
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()

	if enableWebhook {
		go runWebhook(ctx, webhookPort, webhookCert, webhookKey)
	} else {
		klog.Info("Webhook server is disabled")
	}

	app.NewServer(routerPort, tlsCert != "" && tlsKey != "", tlsCert, tlsKey).Run(ctx)
}

func runWebhook(ctx context.Context, port int, certFile, keyFile string) {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to get kube config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to get kube client: %v", err)
	}

	kthenaClient, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to get kthena client: %v", err)
	}

	validator := routerwebhook.NewInferRouterValidator(kubeClient, kthenaClient, port)

	stopCh := make(chan struct{})

	go func() {
		<-ctx.Done()
		close(stopCh)
	}()

	if err := waitForTLSFiles(ctx, certFile, keyFile); err != nil {
		if errors.Is(err, context.Canceled) {
			klog.Info("Context cancelled before TLS certificates became available")
			return
		}
		klog.Fatalf("Failed while waiting for webhook TLS certificates: %v", err)
	}

	go validator.Run(certFile, keyFile, stopCh)

	klog.Infof("Webhook server running on port %d", port)
}

func waitForTLSFiles(ctx context.Context, files ...string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		allReady := true
		for _, file := range files {
			if _, err := os.Stat(file); err != nil {
				if os.IsNotExist(err) {
					allReady = false
					break
				}
				return err
			}
		}

		if allReady {
			return nil
		}

		klog.Infof("Waiting for webhook TLS certificates to become available at %v", files)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
