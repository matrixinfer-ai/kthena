# Registry Webhook with cert-manager Integration

This directory contains the Kubernetes manifests for deploying the registry-webhook with cert-manager integration for TLS certificate management.

## Prerequisites

- Kubernetes cluster with cert-manager installed
- kubectl configured to communicate with your cluster

## Components

### cert-manager Resources

- **Issuer**: A self-signed certificate issuer in the `matrixinfer-system` namespace
- **Certificate**: Generates a TLS certificate for the webhook server and stores it in the `registry-webhook-certs` secret

### Webhook Resources

- **Deployment**: Runs the webhook server
- **Service**: Exposes the webhook server
- **ValidatingWebhookConfiguration**: Configures the validating webhook
- **MutatingWebhookConfiguration**: Configures the mutating webhook

## How It Works

1. cert-manager creates a self-signed CA and issues a certificate for the webhook server
2. The certificate is stored in the `registry-webhook-certs` secret
3. The webhook deployment mounts this secret
4. cert-manager injects the CA bundle into the webhook configurations

## Test Workflow
```bash
make install-cert-manager # optional
make install-crd # optional

make deploy-registry-webhook
kubectl apply -f ./examples/model/model_example.yaml
make undeploy-registry-webhook

make uninstall-crd # optional
make uninstall-cert-manager # optional
```

## Troubleshooting

If the webhook is not working, check:

1. cert-manager pods are running:
```bash
kubectl get pods -n cert-manager
```

2. Certificate is ready:
```bash
kubectl get certificate -n matrixinfer-system registry-webhook-cert
```

3. Secret exists:
```bash
kubectl get secret -n matrixinfer-system registry-webhook-certs
```

4. Webhook configurations have the CA bundle injected:
```bash
kubectl get validatingwebhookconfiguration registry-validating-webhook -o yaml
kubectl get mutatingwebhookconfiguration registry-mutating-webhook -o yaml
```
