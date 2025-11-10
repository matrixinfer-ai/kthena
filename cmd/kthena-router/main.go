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

	"github.com/volcano-sh/kthena/cmd/kthena-router/app"
	routerwebhook "github.com/volcano-sh/kthena/pkg/kthena-router/webhook"
	webhookcert "github.com/volcano-sh/kthena/pkg/webhook/cert"
)

func main() {
	var (
		routerPort     string
		tlsCert        string
		tlsKey         string
		enableWebhook  bool
		webhookPort    int
		webhookCert    string
		webhookKey     string
		certSecretName string
		serviceName    string
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
	// auto-generate-cert flag removed: behavior now automatic based on cert file presence
	pflag.StringVar(&certSecretName, "cert-secret-name", "kthena-router-webhook-certs", "Name of the secret to store auto-generated webhook certificates")
	pflag.StringVar(&serviceName, "webhook-service-name", "kthena-router-webhook", "Service name for the webhook server")
	defer klog.Flush()
	pflag.Parse()

	if (tlsCert != "" && tlsKey == "") || (tlsCert == "" && tlsKey != "") {
		klog.Fatal("tls-cert and tls-key must be specified together")
	}

	// If webhook cert/key files do not exist they will be auto-generated into a secret and mounted.

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
		go runWebhook(ctx, webhookPort, webhookCert, webhookKey, certSecretName, serviceName)
	} else {
		klog.Info("Webhook server is disabled")
	}

	app.NewServer(routerPort, tlsCert != "" && tlsKey != "", tlsCert, tlsKey).Run(ctx)
}

const validatingWebhookConfigurationName = "kthena-router-validating-webhook"

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
	caBundle, err := webhookcert.EnsureCertificate(ctx, kubeClient, namespace, secretName, dnsNames)
	if err != nil {
		return err
	}

	// Update ValidatingWebhookConfiguration with CA bundle
	if err := webhookcert.UpdateValidatingWebhookCABundle(ctx, kubeClient, validatingWebhookConfigurationName, caBundle); err != nil {
		klog.Warningf("Failed to update ValidatingWebhookConfiguration CA bundle: %v", err)
	}

	return nil
}

func runWebhook(ctx context.Context, port int, certFile, keyFile, secretName, serviceName string) {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to get kube config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to get kube client: %v", err)
	}
	// Auto-generate certificate if cert/key files are missing.
	if _, err := os.Stat(certFile); err != nil || certFile == "" {
		klog.Infof("Webhook cert file '%s' not found, will attempt auto-generation", certFile)
		if err := ensureWebhookCertificate(ctx, kubeClient, secretName, serviceName); err != nil {
			klog.Fatalf("Failed to auto-generate certificate: %v", err)
		}
	} else {
		if _, err := os.Stat(keyFile); err != nil || keyFile == "" {
			klog.Infof("Webhook key file '%s' not found, will attempt auto-generation", keyFile)
			if err := ensureWebhookCertificate(ctx, kubeClient, secretName, serviceName); err != nil {
				klog.Fatalf("Failed to auto-generate certificate: %v", err)
			}
		} else {
			klog.Infof("Using existing webhook TLS cert/key files")
		}
	}

	validator := routerwebhook.NewKthenaRouterValidator(kubeClient, port)

	go validator.Run(ctx, certFile, keyFile)

	klog.Infof("Webhook server running on port %d", port)
}
