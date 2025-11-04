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

package cert

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	// TLSCertKey is the key for the TLS certificate in the secret
	TLSCertKey = "tls.crt"
	// TLSKeyKey is the key for the TLS private key in the secret
	TLSKeyKey = "tls.key"
	// CAKey is the key for the CA certificate in the secret
	CAKey = "ca.crt"
)

// EnsureCertificate ensures that a certificate exists for the webhook server.
// If the secret doesn't exist, it generates a new certificate and creates the secret.
// If the secret already exists, it returns without error (reusing existing certificate).
// It also writes the certificate files to the specified paths for the webhook server to use.
func EnsureCertificate(ctx context.Context, kubeClient kubernetes.Interface, namespace, secretName string, dnsNames []string, certPath, keyPath string) error {
	klog.Infof("Ensuring certificate exists in secret %s/%s", namespace, secretName)

	// Try to get the existing secret
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		// Secret exists, use it
		klog.Infof("Found existing secret %s/%s, using existing certificate", namespace, secretName)
		return writeCertificateFiles(secret, certPath, keyPath)
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	// Secret doesn't exist, generate new certificate
	klog.Infof("Secret %s/%s not found, generating new certificate", namespace, secretName)
	certBundle, err := GenerateSelfSignedCertificate(dnsNames)
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %w", err)
	}

	// Create the secret
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			TLSCertKey: certBundle.CertPEM,
			TLSKeyKey:  certBundle.KeyPEM,
			CAKey:      certBundle.CAPEM,
		},
	}

	_, err = kubeClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Another pod created the secret concurrently, fetch it and use it
			klog.Infof("Secret %s/%s was created by another pod, fetching it", namespace, secretName)
			secret, err = kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get secret after concurrent creation: %w", err)
			}
			return writeCertificateFiles(secret, certPath, keyPath)
		}
		return fmt.Errorf("failed to create secret %s/%s: %w", namespace, secretName, err)
	}

	klog.Infof("Successfully created secret %s/%s with generated certificate", namespace, secretName)

	// Write certificate files for the webhook server
	return writeCertificateFiles(secret, certPath, keyPath)
}

// writeCertificateFiles writes the certificate and key from the secret to the specified file paths
func writeCertificateFiles(secret *corev1.Secret, certPath, keyPath string) error {
	cert, ok := secret.Data[TLSCertKey]
	if !ok {
		return fmt.Errorf("secret %s/%s does not contain %s", secret.Namespace, secret.Name, TLSCertKey)
	}

	key, ok := secret.Data[TLSKeyKey]
	if !ok {
		return fmt.Errorf("secret %s/%s does not contain %s", secret.Namespace, secret.Name, TLSKeyKey)
	}

	// Write certificate file
	if err := os.WriteFile(certPath, cert, 0600); err != nil {
		return fmt.Errorf("failed to write certificate to %s: %w", certPath, err)
	}
	klog.Infof("Wrote certificate to %s", certPath)

	// Write key file
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return fmt.Errorf("failed to write key to %s: %w", keyPath, err)
	}
	klog.Infof("Wrote key to %s", keyPath)

	return nil
}
