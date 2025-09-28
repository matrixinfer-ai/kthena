# Gateway Inference Extension Support

## Overview

Gateway Inference Extension provides a standardized way to expose AI/ML inference services through [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/). This guide demonstrates how to integrate Kthena-deployed models with the upstream Gateway API Inference Extension, enabling intelligent routing and traffic management for inference workloads.

The Gateway API Inference Extension extends the standard Kubernetes Gateway API with inference-specific resources:

- **InferencePool**: Manages collections of model server endpoints with automatic discovery and health monitoring
- **InferenceObjective**: Defines priority and capacity policies for inference requests  
- **Gateway Integration**: Seamless integration with popular gateway implementations including Istio, GKE Gateway, and Kgateway
- **Model-Aware Routing**: Advanced routing capabilities based on model names, adapters, and request characteristics
- **OpenAI API Compatibility**: Full support for OpenAI-compatible endpoints (`/v1/chat/completions`, `/v1/completions`)

## Prerequisites

- Kubernetes cluster with Kthena installed (see [Installation](../getting-started/installation.md))
- Gateway API installed (see [Gateway API](https://gateway-api.sigs.k8s.io/))
- Basic understanding of Gateway API and Gateway Inference Extension

## Getting Started

### Deploy Sample Model Server

First, deploy a model that will serve as the backend for the Gateway Inference Extension. Follow the [Quick Start](../getting-started/quick-start.md) guide to deploy a model in the `default` namespace and ensure it's in `Active` state.

After deployment, identify the labels of your model pods as these will be used to associate the InferencePool with your model instances:

```bash
# Get the model pods and their labels
kubectl get pods -n <your-namespace> -l workload.serving.volcano.sh/managed-by=workload.serving.volcano.sh --show-labels

# Example output shows labels like:
# modelserving.volcano.sh/name=demo-backend1
# modelserving.volcano.sh/group-name=demo-backend1-0
# modelserving.volcano.sh/role=leader
# workload.serving.volcano.sh/model-name=demo
# workload.serving.volcano.sh/backend-name=backend1
# workload.serving.volcano.sh/managed-by=workload.serving.volcano.sh
```

### Install the Inference Extension CRDs

Install the Gateway API Inference Extension CRDs in your cluster:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/latest/download/manifests.yaml
```

### Deploy the InferencePool and Endpoint Picker Extension

Install an InferencePool that selects from Kthena model endpoints with the appropriate labels. The Helm install command automatically installs the endpoint-picker, inferencepool along with provider specific resources.

For Istio deployment:

```bash
export GATEWAY_PROVIDER=istio
export IGW_CHART_VERSION=v1.0.1-rc.1

# Install InferencePool pointing to your Kthena model pods
helm install kthena-demo \
  --set inferencePool.modelServers.matchLabels."workload\.serving\.volcano\.sh/model-name"=demo \
  --set provider.name=$GATEWAY_PROVIDER \
  --version $IGW_CHART_VERSION \
  oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool
```

### Deploy an Inference Gateway

Deploy the Istio-based inference gateway and routing configuration:

1. **Install Istio** (if not already installed):
```bash
TAG=$(curl https://storage.googleapis.com/istio-build/dev/1.28-dev)
# on Linux
wget https://storage.googleapis.com/istio-build/dev/$TAG/istioctl-$TAG-linux-amd64.tar.gz
tar -xvf istioctl-$TAG-linux-amd64.tar.gz

./istioctl install --set tag=$TAG --set hub=gcr.io/istio-testing --set values.pilot.env.ENABLE_GATEWAY_API_INFERENCE_EXTENSION=true
```

2. **Deploy the Gateway**:
```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/gateway.yaml
```

3. **Deploy the HTTPRoute**:

Create and apply the HTTPRoute configuration that connects the gateway to your InferencePool:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: kthena-demo-route
spec:
  parentRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: inference-gateway
  rules:
  - backendRefs:
    - group: inference.networking.k8s.io
      kind: InferencePool
      name: kthena-demo
    matches:
    - path:
        type: PathPrefix
        value: /
    timeouts:
      request: 300s
EOF
```

### Verify Gateway Installation

Confirm that the Gateway was assigned an IP address and reports a `Programmed=True` status:

```bash
kubectl get gateway inference-gateway

# Expected output:
# NAME                CLASS     ADDRESS         PROGRAMMED   AGE
# inference-gateway   istio     <GATEWAY_IP>    True         30s
```

Verify that all components are properly configured:

```bash
# Check Gateway status
kubectl get gateway inference-gateway -o yaml

# Check HTTPRoute status - should show Accepted=True and ResolvedRefs=True
kubectl get httproute kthena-demo-route -o yaml

# Check InferencePool status
kubectl get inferencepool kthena-demo -o yaml
```

## Try it out

Wait until the gateway is ready and test inference through the gateway:

```bash
# Get the gateway IP address
IP=$(kubectl get gateway/inference-gateway -o jsonpath='{.status.addresses[0].value}')
PORT=80

# Test completions endpoint
curl -i ${IP}:${PORT}/v1/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen2.5-0.5B-Instruct",
    "prompt": "Write as if you were a critic: San Francisco",
    "max_tokens": 100,
    "temperature": 0
  }'
```

## Cleanup

To clean up all resources created in this guide:

1. **Uninstall the InferencePool and model resources**:
```bash
helm uninstall kthena-demo
kubectl delete modelbooster demo -n <your-namespace> --ignore-not-found
```

2. **Remove Gateway API Inference Extension CRDs**:
```bash
kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/latest/download/manifests.yaml --ignore-not-found
```

3. **Clean up Istio Gateway resources**:
```bash
kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/gateway.yaml --ignore-not-found
kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/httproute.yaml --ignore-not-found
```

4. **Remove Istio** (if you want to clean up everything):
```bash
istioctl uninstall -y --purge
kubectl delete ns istio-system
```
