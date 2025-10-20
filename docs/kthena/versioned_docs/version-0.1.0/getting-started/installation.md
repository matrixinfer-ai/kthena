---
sidebar_position: 1
---

# Installation

This guide will help you install Kthena on your Kubernetes cluster.

## Prerequisites

Before installing Kthena, ensure you have the following:

-   **Kubernetes cluster** (version 1.20 or later)
-   **kubectl** configured to access your cluster
-   **Helm** (version 3.0 or later)
-   Cluster admin permissions

## Installation Methods

### Method 1: Helm Installation (Recommended)

Kthena Helm charts are published to the GitHub Container Registry (GHCR).

1.  **Install Kthena directly from GHCR:**

    ```bash
    helm install kthena oci://ghcr.io/volcano-sh/charts/kthena \
      --namespace kthena-system \
      --create-namespace
    ```

    You might optionally specify a chart version:

    ```bash
    helm install kthena oci://ghcr.io/volcano-sh/charts/kthena \
      --version <YOUR_CHART_VERSION> \
      --namespace kthena-system \
      --create-namespace
    ```

### Method 2: Manual Installation with GitHub Release Manifests

Kthena provides all necessary components in a single manifest file for easy installation from GitHub Releases.

1.  **Apply the Kthena manifest:**

    ```bash
    kubectl apply -f https://github.com/volcano-sh/kthena/releases/latest/download/kthena.yaml
    ```

    To install a specific version, replace `latest` with the desired release tag (e.g., `v1.2.3`):

    ```bash
    kubectl apply -f https://github.com/volcano-sh/kthena/releases/download/vX.Y.Z/kthena.yaml
    ```

## Configuration Options

### Helm Values

You can customize the installation by providing values:

```bash
helm install kthena oci://ghcr.io/volcano-sh/charts/kthena \
  --namespace kthena-system \
  --create-namespace \
  --set controller.replicas=2 \
  --set router.service.type=LoadBalancer
```

### Common Configuration Parameters

| Parameter | Description | Default |
| :------------------ | :---------------------------- | :-------- |
| `controller.replicas` | Number of controller replicas | `1` |
| `router.service.type` | Router service type | `ClusterIP` |
| `registry.enabled` | Enable model registry | `true` |
| `autoscaler.enabled` | Enable auto-scaling | `true` |

## Verification

After installation, verify that all components are running:

```bash
# Check all pods are running
kubectl get pods -n kthena-system

# Check CRDs are installed (CRDs are included in the main manifest/chart)
kubectl get crd | grep kthena

# Check services
kubectl get svc -n kthena-system
```

## Certificate Management

Kthena components such as webhooks and the router require certificates for secure communication. You might need to install a certificate manager to handle certificate provisioning and management automatically.

If you need certificate management capabilities, you can install cert-manager by following the official installation guide of [Cert Manager](https://cert-manager.io/docs/installation/).

## Gang Scheduling

Kthena leverages **Volcano** (a high-performance batch system for Kubernetes) to provide gang scheduling capabilities.

If you need gang scheduling capabilities, you can install Volcano by following the official installation guide of [Volcano](https://volcano.sh/en/docs/installation/).

# Minfer CLI
Kthena provides a CLI tool called `minfer` to manage your Kthena deployments. You can download CLI from the [GitHub release page](https://github.com/volcano-sh/kthena/releases/). Please refer to the [CLI documentation](../reference/cli/minfer.md) for more information.