package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	"matrixinfer.ai/matrixinfer/pkg/model-webhook/server"
)

var (
	masterURL      string
	kubeconfig     string
	tlsCertFile    string
	tlsPrivateKey  string
	port           int
	webhookTimeout int
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Set up signals so we handle the first shutdown signal gracefully
	stopCh := setupSignalHandler()

	cfg, err := getConfig()
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	matrixinferClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building matrixinfer clientset: %s", err.Error())
	}

	// Create webhook server
	webhookServer := server.NewWebhookServer(
		kubeClient,
		matrixinferClient,
		tlsCertFile,
		tlsPrivateKey,
		port,
		webhookTimeout,
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

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&tlsPrivateKey, "tls-private-key-file", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tls-cert-file.")
	flag.IntVar(&port, "port", 8443, "Secure port that the webhook listens on")
	flag.IntVar(&webhookTimeout, "webhook-timeout", 30, "Timeout for webhook operations in seconds")
}

func getConfig() (*rest.Config, error) {
	if len(kubeconfig) > 0 {
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}
	return rest.InClusterConfig()
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
