---
sidebar_position: 1
---

# Installation

This guide will help you install MatrixInfer on your Kubernetes cluster.

## Prerequisites

Before installing MatrixInfer, ensure you have the following:

- **Kubernetes cluster** (version 1.20 or later)
- **kubectl** configured to access your cluster
- **Helm** (version 3.0 or later)
- Cluster admin permissions

## Installation Methods

### Method 1: Helm Installation (Recommended)

1. **Add the MatrixInfer Helm repository:**

```bash
helm repo add matrixinfer https://matrixinfer-ai.github.io/charts
helm repo update
```

2. **Install MatrixInfer:**

```bash
helm install matrixinfer matrixinfer/matrixinfer \
  --namespace matrixinfer-system \
  --create-namespace
```

3. **Verify the installation:**

```bash
kubectl get pods -n matrixinfer-system
```

### Method 2: Manual Installation with Manifests

1. **Clone the repository:**

```bash
git clone https://github.com/matrixinfer-ai/matrixinfer.git
cd matrixinfer
```

2. **Apply the CRDs:**

```bash
kubectl apply -f charts/matrixinfer/charts/networking/crds/
kubectl apply -f charts/matrixinfer/charts/registry/crds/
kubectl apply -f charts/matrixinfer/charts/workload/crds/
```

3. **Install the components:**

```bash
make build-installer
kubectl apply -f dist/install.yaml
```

## Configuration Options

### Helm Values

You can customize the installation by providing values:

```bash
helm install matrixinfer matrixinfer/matrixinfer \
  --namespace matrixinfer-system \
  --create-namespace \
  --set controller.replicas=2 \
  --set gateway.service.type=LoadBalancer
```

### Common Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.replicas` | Number of controller replicas | `1` |
| `gateway.service.type` | Gateway service type | `ClusterIP` |
| `registry.enabled` | Enable model registry | `true` |
| `autoscaler.enabled` | Enable auto-scaling | `true` |

## Verification

After installation, verify that all components are running:

```bash
# Check all pods are running
kubectl get pods -n matrixinfer-system

# Check CRDs are installed
kubectl get crd | grep matrixinfer

# Check services
kubectl get svc -n matrixinfer-system
```

## Certificate Management

MatrixInfer components such as webhooks and the gateway require certificates for secure communication. You might need to install a certificate manager to handle certificate provisioning and management automatically.

If you need certificate management capabilities, you can install cert-manager by following the [official installation guide](https://cert-manager.io/docs/installation/) of Cert Manager.
