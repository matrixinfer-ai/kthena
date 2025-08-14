---
sidebar_position: 2
---

# Quick Start

Get up and running with MatrixInfer in minutes! This guide will walk you through deploying your first AI model.

## Prerequisites

- MatrixInfer installed on your Kubernetes cluster (see [Installation](./installation.md))
- A model to deploy (we'll use a sample model in this guide)

## Step 1: Create a Model Registry Entry

First, register your model in the MatrixInfer registry:

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: sample-llm
  namespace: default
spec:
  modelSpec:
    modelId: "sample-llm-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "s3://my-models/sample-llm/"
      # or use: "huggingface://microsoft/DialoGPT-medium"
  runtime:
    image: "matrixinfer/pytorch-runtime:latest"
    resources:
      requests:
        memory: "2Gi"
        cpu: "1"
      limits:
        memory: "4Gi"
        cpu: "2"
```

Apply the model configuration:

```bash
kubectl apply -f model.yaml
```

## Step 2: Deploy the Model for Inference

Create a ModelInfer resource to deploy your model:

```yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: sample-llm-infer
  namespace: default
spec:
  modelRef:
    name: sample-llm
  replicas: 2
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
    targetCPUUtilizationPercentage: 70
  resources:
    requests:
      memory: "2Gi"
      cpu: "1"
    limits:
      memory: "4Gi"
      cpu: "2"
```

Deploy the inference service:

```bash
kubectl apply -f modelinfer.yaml
```

## Step 3: Create a Model Server for External Access

Set up a ModelServer to expose your model:

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: sample-llm-server
  namespace: default
spec:
  modelInferRef:
    name: sample-llm-infer
  service:
    type: LoadBalancer
    port: 8080
  routing:
    path: "/v1/models/sample-llm"
    timeout: "30s"
  rateLimit:
    enabled: true
    requestsPerSecond: 100
```

Create the model server:

```bash
kubectl apply -f modelserver.yaml
```

## Step 4: Verify the Deployment

Check that all resources are running:

```bash
# Check the model registration
kubectl get models

# Check the inference deployment
kubectl get modelinfers

# Check the model server
kubectl get modelservers

# Check pods are running
kubectl get pods -l app=sample-llm-infer
```

## Step 5: Test Your Model

Get the service endpoint:

```bash
# For LoadBalancer service
kubectl get svc sample-llm-server

# For port-forward (if using ClusterIP)
kubectl port-forward svc/sample-llm-server 8080:8080
```

Send a test request:

```bash
curl -X POST http://<SERVICE_IP>:8080/v1/models/sample-llm/infer \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": {
      "text": "Hello, how are you?"
    }
  }'
```

## Step 6: Monitor Your Deployment

View logs and metrics:

```bash
# Check inference logs
kubectl logs -l app=sample-llm-infer

# Check model server logs
kubectl logs -l app=sample-llm-server

# View resource usage
kubectl top pods -l app=sample-llm-infer
```

## Advanced Configuration

### Auto-scaling Configuration

Configure advanced auto-scaling based on custom metrics:

```yaml
spec:
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 10
    metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

### Model Routing

Set up intelligent routing for A/B testing:

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: sample-llm-route
spec:
  routes:
  - match:
      headers:
        version: "v1"
    destination:
      modelServerRef:
        name: sample-llm-server-v1
    weight: 90
  - match:
      headers:
        version: "v2"
    destination:
      modelServerRef:
        name: sample-llm-server-v2
    weight: 10
```

## Troubleshooting

### Common Issues

**Model not starting:**
```bash
# Check model logs
kubectl describe model sample-llm
kubectl logs -l app=sample-llm-infer
```

**Service not accessible:**
```bash
# Check service status
kubectl get svc sample-llm-server
kubectl describe svc sample-llm-server
```

**Auto-scaling not working:**
```bash
# Check HPA status
kubectl get hpa
kubectl describe hpa sample-llm-infer
```
