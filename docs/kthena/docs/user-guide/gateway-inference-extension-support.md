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

A cluster with:
- **Kubernetes cluster** (version 1.20 or later) with support for services of type `LoadBalancer`
- **Support for sidecar containers** (enabled by default since Kubernetes v1.29) to run the model server deployment
- **Kthena installed** on your cluster (see [Installation](../getting-started/installation.md))

Tooling:
- **kubectl** configured to access your cluster
- **Helm** installed

## Steps

### Deploy Sample Model Server

First, deploy a Kthena model that will serve as the backend for the Gateway Inference Extension. Follow the [Quick Start](../getting-started/quick-start.md) guide to deploy a model. For this example, we'll use the Qwen2.5-0.5B-Instruct model:

```bash
# Deploy the example model
kubectl apply -n <your-namespace> -f https://raw.githubusercontent.com/volcano-sh/kthena/refs/heads/main/examples/model-booster/Qwen2.5-0.5B-Instruct.yaml

# Wait for the model to be ready
kubectl get model demo -o jsonpath='{.status.conditions}' -n <your-namespace>
```

Verify that your model is running and has the Active condition set to `true`:

```json
[
  {
    "lastTransitionTime": "2025-09-05T02:14:16Z",
    "message": "Model initialized",
    "reason": "ModelCreating",
    "status": "True",
    "type": "Initialized"
  },
  {
    "lastTransitionTime": "2025-09-05T02:18:46Z",
    "message": "Model is ready",
    "reason": "ModelAvailable",
    "status": "True",
    "type": "Active"
  }
]
```

Identify the labels of your deployed model pods, as these will be used to associate the InferencePool with your model instances:

```bash
# Get the model pods and their labels
kubectl get pods -n <your-namespace> -l app.kubernetes.io/managed-by=kthena --show-labels

# Example output shows labels like:
# app.kubernetes.io/instance=demo
# app.kubernetes.io/managed-by=kthena
# modelbooster.workload.serving.volcano.sh/owner-uid=<uid>
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
  --set inferencePool.modelServers.matchLabels."app\.kubernetes\.io/instance"=demo \
  --set inferencePool.modelServers.matchLabels."app\.kubernetes\.io/managed-by"=kthena \
  --set provider.name=$GATEWAY_PROVIDER \
  --version $IGW_CHART_VERSION \
  oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool
```

### Deploy an Inference Gateway

Deploy the Istio-based inference gateway and routing configuration:

1. **Install Istio** (if not already installed):
   ```bash
   # Download and install Istio
   curl -L https://istio.io/downloadIstio | sh -
   cd istio-*
   export PATH=$PWD/bin:$PATH
   
   # Install Istio with default configuration
   istioctl install --set values.defaultRevision=default -y
   
   # Enable automatic sidecar injection for your namespace
   kubectl label namespace <your-namespace> istio-injection=enabled
   ```

2. **Deploy the Gateway**:
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/gateway.yaml
   ```

3. **Deploy the DestinationRule**:
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/destination-rule.yaml
   ```

4. **Deploy the HTTPRoute**:
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/httproute.yaml
   ```

### Deploy InferenceObjective (Optional)

Deploy the sample InferenceObjective which allows you to specify priority of requests:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/inferenceobjective.yaml
```

### Verify Gateway Installation

Confirm that the Gateway was assigned an IP address and reports a `Programmed=True` status:

```bash
kubectl get gateway inference-gateway

# Expected output:
# NAME                CLASS     ADDRESS         PROGRAMMED   AGE
# inference-gateway   istio     <GATEWAY_IP>    True         30s
```

Get the gateway external IP address:

```bash
IP=$(kubectl get gateway/inference-gateway -o jsonpath='{.status.addresses[0].value}')
PORT=80
echo "Gateway IP: $IP"
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
    "model": "demo",
    "prompt": "Write as if you were a critic: San Francisco",
    "max_tokens": 100,
    "temperature": 0
  }'
```

Expected response format:
```json
{
  "id": "cmpl-xxx",
  "object": "text_completion",
  "created": 1677652288,
  "model": "demo",
  "choices": [
    {
      "index": 0,
      "text": "San Francisco is a vibrant city known for its iconic landmarks...",
      "finish_reason": "length"
    }
  ],
  "usage": {
    "prompt_tokens": 8,
    "completion_tokens": 100,
    "total_tokens": 108
  }
}
```

### Test Chat Completions

You can also test the chat completions endpoint:

```bash
# Test chat completions endpoint
curl -i ${IP}:${PORT}/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "demo",
    "messages": [
      {
        "role": "user",
        "content": "What is the capital of China?"
      }
    ],
    "max_tokens": 50,
    "temperature": 0
  }'
```

## Verification Steps

### 1. Check Gateway Status

Verify that all components are properly configured:

```bash
# Check Gateway status
kubectl get gateway inference-gateway -o yaml

# Check HTTPRoute status - should show Accepted=True and ResolvedRefs=True
kubectl get httproute llm-route -o yaml

# Check InferencePool status
kubectl get inferencepool kthena-demo -o yaml
```

### 2. Monitor Traffic Flow

Monitor that requests are being properly routed to your Kthena model pods:

```bash
# Monitor gateway access logs
kubectl logs -f deployment/istio-ingressgateway -n istio-system

# Check your model pod logs to confirm requests are reaching them
kubectl logs -f <model-pod-name> -n <your-namespace>

# Check InferencePool endpoints discovery
kubectl describe inferencepool kthena-demo
```

### 3. Advanced Verification

For detailed verification, inspect the Istio proxy configuration:

```bash
# Check that the gateway configuration is applied
istioctl proxy-config listeners deployment/istio-ingressgateway -n istio-system

# Verify routing rules
istioctl proxy-config routes deployment/istio-ingressgateway -n istio-system --name 80

# Check discovered endpoints
istioctl proxy-config endpoints deployment/istio-ingressgateway -n istio-system
```

## Cleanup

To clean up all resources created in this guide:

1. **Uninstall the InferencePool and model resources**:
   ```bash
   helm uninstall kthena-demo
   kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/inferenceobjective.yaml --ignore-not-found
   kubectl delete model demo -n <your-namespace> --ignore-not-found
   ```

2. **Remove Gateway API Inference Extension CRDs**:
   ```bash
   kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/latest/download/manifests.yaml --ignore-not-found
   ```

3. **Clean up Istio Gateway resources**:
   ```bash
   kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/gateway.yaml --ignore-not-found
   kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/destination-rule.yaml --ignore-not-found
   kubectl delete -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/istio/httproute.yaml --ignore-not-found
   ```

4. **Optionally remove Istio** (if you want to clean up everything):
   ```bash
   istioctl uninstall -y --purge
   kubectl delete ns istio-system
   ```

