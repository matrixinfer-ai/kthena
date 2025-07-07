package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	"matrixinfer.ai/matrixinfer/pkg/registry-webhook/handlers"
)

// WebhookServer contains the server configuration
type WebhookServer struct {
	kubeClient        kubernetes.Interface
	matrixinferClient clientset.Interface
	server            *http.Server
	tlsCertFile       string
	tlsPrivateKey     string
	port              int
	timeout           int
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(
	kubeClient kubernetes.Interface,
	matrixinferClient clientset.Interface,
	tlsCertFile string,
	tlsPrivateKey string,
	port int,
	timeout int,
) *WebhookServer {
	return &WebhookServer{
		kubeClient:        kubeClient,
		matrixinferClient: matrixinferClient,
		tlsCertFile:       tlsCertFile,
		tlsPrivateKey:     tlsPrivateKey,
		port:              port,
		timeout:           timeout,
	}
}

// Start starts the webhook server
func (ws *WebhookServer) Start(stopCh <-chan struct{}) error {
	// Create handlers
	modelValidator := handlers.NewModelValidator(ws.kubeClient, ws.matrixinferClient)
	modelMutator := handlers.NewModelMutator(ws.kubeClient, ws.matrixinferClient)

	// Create mux and register handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/validate-registry-matrixinfer-ai-v1alpha1-model", modelValidator.Handle)
	mux.HandleFunc("/mutate-registry-matrixinfer-ai-v1alpha1-model", modelMutator.Handle)
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
	<-stopCh
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
