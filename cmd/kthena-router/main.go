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
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	"github.com/volcano-sh/kthena/cmd/kthena-router/app"
	routerwebhook "github.com/volcano-sh/kthena/pkg/kthena-router/webhook"
	webhookcert "github.com/volcano-sh/kthena/pkg/webhook/cert"
)

func main() {
	var (
		routerPort       string
		tlsCert          string
		tlsKey           string
		enableWebhook    bool
		webhookPort      int
		webhookCert      string
		webhookKey       string
		autoGenerateCert bool
		certSecretName   string
		serviceName      string
	)

	// Initialize klog flags
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.StringVar(&routerPort, "port", "8080", "Server listen port")
	pflag.StringVar(&tlsCert, "tls-cert", "", "TLS certificate file path")
	pflag.StringVar(&tlsKey, "tls-key", "", "TLS key file path")
	pflag.BoolVar(&enableWebhook, "enable-webhook", true, "Enable built-in admission webhook server")
	pflag.IntVar(&webhookPort, "webhook-port", 8443, "The port for the webhook server")
	pflag.StringVar(&webhookCert, "webhook-tls-cert-file", "/etc/tls/tls.crt", "Path to the webhook TLS certificate file")
	pflag.StringVar(&webhookKey, "webhook-tls-private-key-file", "/etc/tls/tls.key", "Path to the webhook TLS private key file")
	pflag.BoolVar(&autoGenerateCert, "auto-generate-cert", true, "If true, automatically generate self-signed certificate for webhook if not exists")
	pflag.StringVar(&certSecretName, "cert-secret-name", "kthena-router-webhook-certs", "Name of the secret to store auto-generated webhook certificates")
	pflag.StringVar(&serviceName, "webhook-service-name", "kthena-router-webhook", "Service name for the webhook server")
	defer klog.Flush()
	pflag.Parse()

	if (tlsCert != "" && tlsKey == "") || (tlsCert == "" && tlsKey != "") {
		klog.Fatal("tls-cert and tls-key must be specified together")
	}

	if enableWebhook {
		if !autoGenerateCert && (webhookCert == "" || webhookKey == "") {
			klog.Fatal("webhook TLS cert and key must be specified when webhook is enabled and auto-generate-cert is disabled")
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
		go runWebhook(ctx, webhookPort, webhookCert, webhookKey, autoGenerateCert, certSecretName, serviceName)
	} else {
		klog.Info("Webhook server is disabled")
	}

	app.NewServer(routerPort, tlsCert != "" && tlsKey != "", tlsCert, tlsKey).Run(ctx)
}

// ensureWebhookCertificate ensures that a certificate exists for the webhook server
func ensureWebhookCertificate(ctx context.Context, kubeClient kubernetes.Interface, secretName, serviceName string) error {
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	dnsNames := []string{
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
	}

	klog.Infof("Auto-generating certificate for webhook server")
	return webhookcert.EnsureCertificate(ctx, kubeClient, namespace, secretName, dnsNames)
}

func runWebhook(ctx context.Context, port int, certFile, keyFile string, autoGenerate bool, secretName, serviceName string) {
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

	// Auto-generate certificate if enabled
	if autoGenerate {
		if err := ensureWebhookCertificate(ctx, kubeClient, secretName, serviceName); err != nil {
			klog.Fatalf("Failed to ensure certificate: %v", err)
		}
	}

	validator := routerwebhook.NewKthenaRouterValidator(kubeClient, kthenaClient, port)

	go validator.Run(ctx, certFile, keyFile)

	klog.Infof("Webhook server running on port %d", port)
}
