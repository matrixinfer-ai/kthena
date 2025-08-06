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

## Troubleshooting

### Common Issues

1. **CRD Installation Fails:**
   - Ensure you have cluster admin permissions
   - Check if CRDs already exist: `kubectl get crd | grep matrixinfer`

2. **Pods Not Starting:**
   - Check pod logs: `kubectl logs -n matrixinfer-system <pod-name>`
   - Verify resource requirements are met

3. **Service Not Accessible:**
   - Check service type and configuration
   - Verify network policies and firewall rules

### Getting Help

If you encounter issues:
- Check the [troubleshooting guide](../troubleshooting)
- Review logs: `kubectl logs -n matrixinfer-system -l app=matrixinfer`
- Open an issue on [GitHub](https://github.com/matrixinfer-ai/matrixinfer/issues)