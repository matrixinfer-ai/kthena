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
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	"github.com/volcano-sh/kthena/pkg/controller"
	"github.com/volcano-sh/kthena/pkg/model-booster-webhook/handlers"
	"github.com/volcano-sh/kthena/pkg/model-serving-controller/webhook"
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
	var wc webhookConfig
	var cc controller.Config
	// Initialize klog flags
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.StringVar(&cc.Kubeconfig, "kubeconfig", "", "kubeconfig file path")
	pflag.BoolVar(&enableWebhook, "enable-webhook", true, "If true, webhook will be used. Default is true")
	pflag.StringVar(&cc.MasterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	pflag.StringVar(&wc.tlsCertFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS. This can be used as a fallback when cert-manager is not available.")
	pflag.StringVar(&wc.tlsPrivateKey, "tls-private-key-file", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tls-cert-file. This can be used as a fallback when cert-manager is not available.")
	pflag.IntVar(&wc.port, "port", 8443, "Secure port that the webhook listens on")
	pflag.IntVar(&wc.webhookTimeout, "webhook-timeout", 30, "Timeout for webhook operations in seconds")
	pflag.BoolVar(&cc.EnableLeaderElection, "leader-elect", false, "Enable leader election for controller. "+
		"Enabling this will ensure there is only one active controller. Default is false.")
	pflag.IntVar(&cc.Workers, "workers", 5, "number of workers to run. Default is 5")
	pflag.Parse()
	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		klog.Infof("Flag: %s, Value: %s", f.Name, f.Value.String())
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()
	if enableWebhook {
		if err := setupWebhook(ctx, wc); err != nil {
			os.Exit(1)
		}
	}
	controller.SetupController(ctx, cc)
}

func setupWebhook(ctx context.Context, wc webhookConfig) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("build client config: %v", err)
		return err
	}
	kthenaClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to create kthenaClient: %v", err)
		return err
	}

	mux := http.NewServeMux()

	modelServingValidator := webhook.NewModelServingValidator()
	mux.HandleFunc("/validate-workload-ai-v1alpha1-modelServing", modelServingValidator.Handle)

	modelValidator := handlers.NewModelValidator()
	modelMutator := handlers.NewModelMutator()
	autoscalingPolicyValidator := handlers.NewAutoscalingPolicyValidator()
	autoscalingPolicyMutator := handlers.NewAutoscalingPolicyMutator()
	autoscalingBindingValidator := handlers.NewAutoscalingBindingValidator(kthenaClient)
	mux.HandleFunc("/validate-registry-volcano-sh-v1alpha1-model", modelValidator.Handle)
	mux.HandleFunc("/mutate-registry-volcano-sh-v1alpha1-model", modelMutator.Handle)
	mux.HandleFunc("/validate-registry-volcano-sh-v1alpha1-autoscalingpolicy", autoscalingPolicyValidator.Handle)
	mux.HandleFunc("/mutate-registry-volcano-sh-v1alpha1-autoscalingpolicy", autoscalingPolicyMutator.Handle)
	mux.HandleFunc("/validate-registry-volcano-sh-v1alpha1-autoscalingpolicybinding", autoscalingBindingValidator.Handle)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			klog.Errorf("failed to write health check response: %v", err)
		}
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", wc.port),
		Handler:      mux,
		ReadTimeout:  time.Duration(wc.webhookTimeout) * time.Second,
		WriteTimeout: time.Duration(wc.webhookTimeout) * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	go func() {
		klog.Infof("Starting webhook server on %s", server.Addr)
		if err := server.ListenAndServeTLS(wc.tlsCertFile, wc.tlsPrivateKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.Fatalf("failed to start unified webhook server: %v", err)
		}
	}()

	<-ctx.Done()
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctxTimeout)
	return nil
}
