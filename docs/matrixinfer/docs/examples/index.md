---
sidebar_position: 5
---

# Examples

Practical examples and use cases for MatrixInfer deployments.

## Overview

This section provides comprehensive examples demonstrating how to use MatrixInfer for various AI model deployment scenarios. Each example includes complete YAML configurations, step-by-step instructions, and best practices.

## Quick Start Examples

### Basic Model Deployment

Deploy a simple PyTorch model from HuggingFace:

```yaml
# model.yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: gpt2-small
  namespace: default
spec:
  modelSpec:
    modelId: "gpt2-small-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "huggingface://gpt2"
  runtime:
    image: "matrixinfer/pytorch-runtime:latest"
    resources:
      requests:
        memory: "2Gi"
        cpu: "1"
      limits:
        memory: "4Gi"
        cpu: "2"
---
# modelinfer.yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: gpt2-small-infer
  namespace: default
spec:
  modelRef:
    name: gpt2-small
  replicas: 2
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
    targetCPUUtilizationPercentage: 70
---
# modelserver.yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: gpt2-small-server
  namespace: default
spec:
  modelInferRef:
    name: gpt2-small-infer
  service:
    type: ClusterIP
    port: 8080
  routing:
    path: "/v1/models/gpt2-small"
    timeout: "30s"
```

**Deploy:**
```bash
kubectl apply -f model.yaml
kubectl apply -f modelinfer.yaml
kubectl apply -f modelserver.yaml
```

## Production Examples

### High-Availability LLM Deployment

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: llama-7b-production
  namespace: production
spec:
  modelSpec:
    modelId: "llama-7b-chat-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "s3://production-models/llama-7b-chat/"
  runtime:
    image: "matrixinfer/pytorch-runtime:v1.2.0"
    resources:
      requests:
        memory: "16Gi"
        cpu: "4"
        nvidia.com/gpu: "1"
      limits:
        memory: "32Gi"
        cpu: "8"
        nvidia.com/gpu: "1"
---
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: llama-7b-production-infer
  namespace: production
spec:
  modelRef:
    name: llama-7b-production
  replicas: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 60
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: app
            operator: In
            values:
            - llama-7b-production-infer
        topologyKey: kubernetes.io/hostname
  nodeSelector:
    node-type: gpu-inference
  tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
---
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: llama-7b-production-server
  namespace: production
spec:
  modelInferRef:
    name: llama-7b-production-infer
  service:
    type: LoadBalancer
    port: 443
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
      service.beta.kubernetes.io/aws-load-balancer-ssl-cert: "arn:aws:acm:us-west-2:123456789012:certificate/12345678-1234-1234-1234-123456789012"
  routing:
    path: "/v1/models/llama-7b"
    timeout: "60s"
    retries: 3
  rateLimit:
    enabled: true
    requestsPerSecond: 100
    burstSize: 200
  authentication:
    enabled: true
    type: "jwt"
    config:
      issuer: "https://auth.company.com"
      audience: "matrixinfer-api"
  tls:
    enabled: true
    secretName: llama-7b-tls
```

### Multi-Model A/B Testing

```yaml
# Model A (Stable)
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: sentiment-model-v1
  namespace: ml-experiments
spec:
  modelSpec:
    modelId: "sentiment-bert-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "s3://ml-models/sentiment-bert-v1/"
  runtime:
    image: "matrixinfer/pytorch-runtime:latest"
---
# Model B (Experimental)
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: sentiment-model-v2
  namespace: ml-experiments
spec:
  modelSpec:
    modelId: "sentiment-roberta-v2"
    framework: "pytorch"
    version: "2.0.0"
    source:
      uri: "s3://ml-models/sentiment-roberta-v2/"
  runtime:
    image: "matrixinfer/pytorch-runtime:latest"
---
# Inference deployments
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: sentiment-v1-infer
  namespace: ml-experiments
spec:
  modelRef:
    name: sentiment-model-v1
  replicas: 3
---
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: sentiment-v2-infer
  namespace: ml-experiments
spec:
  modelRef:
    name: sentiment-model-v2
  replicas: 1
---
# Model servers
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: sentiment-v1-server
  namespace: ml-experiments
spec:
  modelInferRef:
    name: sentiment-v1-infer
  service:
    type: ClusterIP
    port: 8080
  routing:
    path: "/v1/models/sentiment-v1"
---
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: sentiment-v2-server
  namespace: ml-experiments
spec:
  modelInferRef:
    name: sentiment-v2-infer
  service:
    type: ClusterIP
    port: 8080
  routing:
    path: "/v1/models/sentiment-v2"
---
# A/B Testing Route
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: sentiment-ab-test
  namespace: ml-experiments
spec:
  routes:
  - match:
      headers:
        experiment: "stable"
    destination:
      modelServerRef:
        name: sentiment-v1-server
    weight: 80
  - match:
      headers:
        experiment: "new"
    destination:
      modelServerRef:
        name: sentiment-v2-server
    weight: 20
  fallback:
    modelServerRef:
      name: sentiment-v1-server
```

## Specialized Use Cases

### GPU-Optimized Image Classification

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: resnet50-gpu
  namespace: vision
spec:
  modelSpec:
    modelId: "resnet50-imagenet-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "s3://vision-models/resnet50-optimized/"
  runtime:
    image: "matrixinfer/pytorch-gpu-runtime:latest"
    resources:
      requests:
        memory: "8Gi"
        cpu: "2"
        nvidia.com/gpu: "1"
      limits:
        memory: "16Gi"
        cpu: "4"
        nvidia.com/gpu: "1"
---
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: resnet50-gpu-infer
  namespace: vision
spec:
  modelRef:
    name: resnet50-gpu
  replicas: 2
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 8
    metrics:
    - type: Resource
      resource:
        name: nvidia.com/gpu
        target:
          type: Utilization
          averageUtilization: 75
  nodeSelector:
    accelerator: nvidia-tesla-v100
  tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
```

### Edge Deployment with Resource Constraints

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: mobilenet-edge
  namespace: edge
spec:
  modelSpec:
    modelId: "mobilenet-v2-quantized"
    framework: "onnx"
    version: "1.0.0"
    source:
      uri: "s3://edge-models/mobilenet-v2-quantized.onnx"
  runtime:
    image: "matrixinfer/onnx-runtime:latest"
    resources:
      requests:
        memory: "512Mi"
        cpu: "500m"
      limits:
        memory: "1Gi"
        cpu: "1"
---
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: mobilenet-edge-infer
  namespace: edge
spec:
  modelRef:
    name: mobilenet-edge
  replicas: 1
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 3
    targetCPUUtilizationPercentage: 80
  nodeSelector:
    node-type: edge
  tolerations:
  - key: edge-node
    operator: Equal
    value: "true"
    effect: NoSchedule
```

## Testing Examples

### Load Testing Setup

```bash
# Install hey for load testing
go install github.com/rakyll/hey@latest

# Test basic inference
hey -n 1000 -c 10 -m POST \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"text": "Hello world"}}' \
  http://your-model-server/v1/models/gpt2-small/infer

# Test A/B routing
hey -n 500 -c 5 -m POST \
  -H "Content-Type: application/json" \
  -H "experiment: stable" \
  -d '{"inputs": {"text": "Test sentiment"}}' \
  http://your-gateway/v1/models/sentiment/infer

hey -n 500 -c 5 -m POST \
  -H "Content-Type: application/json" \
  -H "experiment: new" \
  -d '{"inputs": {"text": "Test sentiment"}}' \
  http://your-gateway/v1/models/sentiment/infer
```

### Health Check Examples

```bash
# Check model status
kubectl get models -A

# Check inference deployment status
kubectl get modelinfers -A

# Check service endpoints
kubectl get modelservers -A

# Test health endpoints
curl http://your-model-server/health
curl http://your-model-server/ready
curl http://your-model-server/metrics
```

## Integration Examples

### Prometheus Monitoring

```yaml
apiVersion: v1
kind: ServiceMonitor
metadata:
  name: matrixinfer-metrics
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: matrixinfer
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "MatrixInfer Metrics",
    "panels": [
      {
        "title": "Inference Requests/sec",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(matrixinfer_inference_requests_total[5m])"
          }
        ]
      },
      {
        "title": "Model Loading Time",
        "type": "graph",
        "targets": [
          {
            "expr": "matrixinfer_model_load_duration_seconds"
          }
        ]
      }
    ]
  }
}
```

## Next Steps

- Explore [Best Practices](../best-practices/) for production deployments
- Learn about [Advanced Features](../advanced/) like custom metrics and policies
- Set up [Monitoring](../monitoring/) for your deployments
- Check the [API Reference](../api/overview.md) for detailed specifications

## Contributing Examples

Have a useful example to share? Please contribute by:

1. Creating a pull request with your example
2. Including clear documentation and comments
3. Testing the example in a real environment
4. Following our [contribution guidelines](https://github.com/matrixinfer-ai/matrixinfer/blob/main/CONTRIBUTING.md)