---
sidebar_position: 1
---

# Installation

This guide will help you install MatrixInfer on your Kubernetes cluster.

## Prerequisites

Before installing MatrixInfer, ensure you have the following:

-   **Kubernetes cluster** (version 1.20 or later)
-   **kubectl** configured to access your cluster
-   **Helm** (version 3.0 or later)
-   Cluster admin permissions

## Installation Methods

### Method 1: Helm Installation (Recommended)

MatrixInfer Helm charts are published to the GitHub Container Registry (GHCR).

1.  **Install MatrixInfer directly from GHCR:**

    ```bash
    helm install matrixinfer oci://ghcr.io/matrixinfer-ai/charts/matrixinfer \
      --namespace matrixinfer-system \
      --create-namespace
    ```

    You might optionally specify a chart version:

    ```bash
    helm install matrixinfer oci://ghcr.io/matrixinfer-ai/charts/matrixinfer \
      --version <YOUR_CHART_VERSION> \
      --namespace matrixinfer-system \
      --create-namespace
    ```

### Method 2: Manual Installation with GitHub Release Manifests

MatrixInfer provides all necessary components in a single manifest file for easy installation from GitHub Releases.

1.  **Apply the MatrixInfer manifest:**

    ```bash
    kubectl apply -f https://github.com/matrixinfer-ai/matrixinfer/releases/latest/download/matrixinfer.yaml
    ```

    To install a specific version, replace `latest` with the desired release tag (e.g., `v1.2.3`):

    ```bash
    kubectl apply -f https://github.com/matrixinfer-ai/matrixinfer/releases/download/vX.Y.Z/matrixinfer.yaml
    ```

## Configuration Options

### Helm Values

You can customize the installation by providing values:

```bash
helm install matrixinfer oci://ghcr.io/matrixinfer-ai/charts/matrixinfer \
  --namespace matrixinfer-system \
  --create-namespace \
  --set controller.replicas=2 \
  --set gateway.service.type=LoadBalancer
```

### Common Configuration Parameters

| Parameter | Description | Default |
| :------------------ | :---------------------------- | :-------- |
| `controller.replicas` | Number of controller replicas | `1` |
| `gateway.service.type` | Gateway service type | `ClusterIP` |
| `registry.enabled` | Enable model registry | `true` |
| `autoscaler.enabled` | Enable auto-scaling | `true` |

## Verification

After installation, verify that all components are running:

```bash
# Check all pods are running
kubectl get pods -n matrixinfer-system

# Check CRDs are installed (CRDs are included in the main manifest/chart)
kubectl get crd | grep matrixinfer

# Check services
kubectl get svc -n matrixinfer-system
```

## Certificate Management

MatrixInfer components such as webhooks and the gateway require certificates for secure communication. You might need to install a certificate manager to handle certificate provisioning and management automatically.

If you need certificate management capabilities, you can install cert-manager by following the official installation guide of [Cert Manager]((https://cert-manager.io/docs/installation/)).

## Gang Scheduling

For scenarios requiring gang scheduling, particularly for model inference with multiple interdependent instances, MatrixInfer can leverage Volcano.

If you need gang scheduling capabilities, you can install Volcano by following the official installation guide of [Volcano]((https://volcano.sh/en/docs/installation/)).
