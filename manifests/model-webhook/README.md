# Model Webhook with cert-manager Integration

This directory contains the Kubernetes manifests for deploying the model-webhook with cert-manager integration for TLS certificate management.

## Prerequisites

- Kubernetes cluster with cert-manager installed
- kubectl configured to communicate with your cluster

## Components

### cert-manager Resources

- **Issuer**: A self-signed certificate issuer in the `matrixinfer-system` namespace
- **Certificate**: Generates a TLS certificate for the webhook server and stores it in the `model-webhook-certs` secret

### Webhook Resources

- **Deployment**: Runs the webhook server
- **Service**: Exposes the webhook server
- **ValidatingWebhookConfiguration**: Configures the validating webhook
- **MutatingWebhookConfiguration**: Configures the mutating webhook

## Deployment

If the cert-manager is not installed yet, install it first. 

```bash
make install-cert-manager
```

Deploy webhook
```bash
make deploy-model-webhook
```

Undeploy webhook
```bash
make undeploy-model-webhook
```

## How It Works

1. cert-manager creates a self-signed CA and issues a certificate for the webhook server
2. The certificate is stored in the `model-webhook-certs` secret
3. The webhook deployment mounts this secret
4. cert-manager injects the CA bundle into the webhook configurations

## Test
```bash
kubectl apply -f ./examples/model/model_example.yaml
```

## Troubleshooting

If the webhook is not working, check:

1. cert-manager pods are running:
```bash
kubectl get pods -n cert-manager
```

2. Certificate is ready:
```bash
kubectl get certificate -n matrixinfer-system model-webhook-cert
```

3. Secret exists:
```bash
kubectl get secret -n matrixinfer-system model-webhook-certs
```

4. Webhook configurations have the CA bundle injected:
```bash
kubectl get validatingwebhookconfiguration model-validating-webhook -o yaml
kubectl get mutatingwebhookconfiguration model-mutating-webhook -o yaml
```
