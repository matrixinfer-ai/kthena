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

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	"github.com/volcano-sh/kthena/pkg/model-booster-webhook/handlers"
	"k8s.io/klog/v2"
)

// WebhookServer contains the server configuration
type WebhookServer struct {
	kthenaClient  clientset.Interface
	server        *http.Server
	tlsCertFile   string
	tlsPrivateKey string
	port          int
	timeout       int
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(
	kthenaClient clientset.Interface,
	tlsCertFile string,
	tlsPrivateKey string,
	port int,
	timeout int,
) *WebhookServer {
	return &WebhookServer{
		kthenaClient:  kthenaClient,
		tlsCertFile:   tlsCertFile,
		tlsPrivateKey: tlsPrivateKey,
		port:          port,
		timeout:       timeout,
	}
}

// Start starts the webhook server
func (ws *WebhookServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	// Create model handlers
	modelValidator := handlers.NewModelValidator()
	modelMutator := handlers.NewModelMutator()
	autoscalingPolicyValidator := handlers.NewAutoscalingPolicyValidator()
	autoscalingPolicyMutator := handlers.NewAutoscalingPolicyMutator()

	// Create mux and register handlers
	mux.HandleFunc("/validate-registry-volcano-sh-v1alpha1-model", modelValidator.Handle)
	mux.HandleFunc("/mutate-registry-volcano-sh-v1alpha1-model", modelMutator.Handle)
	mux.HandleFunc("/validate-registry-volcano-sh-v1alpha1-autoscalingpolicy", autoscalingPolicyValidator.Handle)
	mux.HandleFunc("/mutate-registry-volcano-sh-v1alpha1-autoscalingpolicy", autoscalingPolicyMutator.Handle)

	// Create autoscalingBinding handlers
	autoscalingBindingValidator := handlers.NewAutoscalingBindingValidator(ws.kthenaClient)
	mux.HandleFunc("/validate-registry-volcano-sh-v1alpha1-autoscalingpolicybinding", autoscalingBindingValidator.Handle)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			klog.Errorf("Failed to write health check response: %v", err)
		}
	})

	// Create server
	ws.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", ws.port),
		Handler:      mux,
		ReadTimeout:  time.Duration(ws.timeout) * time.Second,
		WriteTimeout: time.Duration(ws.timeout) * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Start server
	klog.Infof("Starting webhook server on port %d", ws.port)
	go func() {
		if err := ws.server.ListenAndServeTLS(ws.tlsCertFile, ws.tlsPrivateKey); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("Failed to listen and serve: %v", err)
		}
	}()

	// Wait for stop signal
	<-ctx.Done()
	return ws.shutdown()
}

// shutdown gracefully shuts down the server
func (ws *WebhookServer) shutdown() error {
	klog.Info("Shutting down webhook server")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ws.timeout)*time.Second)
	defer cancel()

	if err := ws.server.Shutdown(ctx); err != nil {
		klog.Errorf("Failed to shutdown server: %v", err)
		return err
	}
	return nil
}
