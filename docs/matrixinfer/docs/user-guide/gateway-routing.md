# Gateway Routing

This page describes the gateway routing features and capabilities in MatrixInfer, based on real-world examples and configurations.

## Overview

MatrixInfer Gateway provides sophisticated traffic routing capabilities that enable intelligent forwarding of inference requests to appropriate backend models. The routing system is built around two core Custom Resources (CRs):

- **ModelServer**: Defines backend inference service instances with their associated pods, models, and traffic policies
- **ModelRoute**: Defines routing rules based on request characteristics such as model name, LoRA adapters, HTTP headers, and weight distribution

The gateway supports various routing strategies, from simple model-based forwarding to complex weighted distribution and header-based routing. This flexibility allows for advanced deployment patterns including canary releases, A/B testing, and load balancing across heterogeneous model deployments.

## Routing Rules

### ModelRoute Configuration

ModelRoute defines the routing logic that determines which ModelServer should handle incoming requests:

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: deepseek-simple
  namespace: default
spec:
  modelName: "deepseek-r1"
  rules:
  - name: "default"
    targetModels:
    - modelServerName: "deepseek-r1-1-5b"
```

### ModelServer Configuration

ModelServer defines the backend inference service and specifies how traffic should be handled:

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: deepseek-r1-1-5b
  namespace: default
spec:
  workloadSelector:
    matchLabels:
      app: deepseek-r1-1-5b
  workloadPort:
    port: 8000
  model: "deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B"
  inferenceEngine: "vLLM"
  trafficPolicy:
    timeout: 10s
```

## Configuration

### ModelRoute Fields

- **modelName**: The logical model name that clients will request
- **loraAdapters**: Optional list of supported LoRA adapters
- **rules**: List of routing rules with match criteria and target models
- **targetModels**: Backend ModelServers with optional weights

### ModelServer Fields

- **workloadSelector**: Selects pods that will serve the model using Kubernetes label selectors
- **workloadPort**: Specifies the port where the inference service is listening
- **model**: The actual model identifier or path
- **inferenceEngine**: The inference framework (vLLM, SGLang, etc.)
- **trafficPolicy**: Defines timeout and other traffic management policies

## ModelServer Definitions

First, let's define the ModelServers that will be reused across different routing scenarios:

```yaml
# 1.5B Parameter Model using vLLM
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: deepseek-r1-1-5b
  namespace: default
spec:
  workloadSelector:
    matchLabels:
      app: deepseek-r1-1-5b
  workloadPort:
    port: 8000
  model: "deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B"
  inferenceEngine: "vLLM"
  trafficPolicy:
    timeout: 10s
---
# 7B Parameter Model using SGLang
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: deepseek-r1-7b
  namespace: default
spec:
  workloadSelector:
    matchLabels:
      app: deepseek-r1-7b
  workloadPort:
    port: 8000
  model: "deepseek-r1:7b"
  inferenceEngine: "SGLang"
  trafficPolicy:
    timeout: 10s
```

## Routing Examples

### 1. Simple Model-Based Routing

**Scenario**: Direct all requests for a specific model to a single backend service.

**Traffic Processing**: When a request comes in for model "deepseek-r1", the gateway matches this criterion and forwards all traffic to the 1.5B ModelServer. This is the most straightforward routing pattern.

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: deepseek-simple
  namespace: default
spec:
  modelName: "deepseek-r1"
  rules:
  - name: "default"
    targetModels:
    - modelServerName: "deepseek-r1-1-5b"
```

**Flow Description**: 
1. Request arrives for model name "deepseek-r1"
2. Gateway matches the modelName field in the ModelRoute
3. 100% of traffic is directed to `deepseek-r1-1-5b`
4. The ModelServer serves requests using vLLM inference engine with 10s timeout

### 2. LoRA-Aware Routing

**Scenario**: Route requests requiring specific LoRA adapters to specialized ModelServers optimized for LoRA workloads.

**Traffic Processing**: When a request specifies LoRA adapters (lora-A or lora-B), the gateway routes it to ModelServers configured to handle these specific adapters.

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: deepseek-lora
  namespace: default
spec:
  loraAdapters:
  - "lora-A"
  - "lora-B"
  rules:
  - name: "lora-route"
    targetModels:
    - modelServerName: "deepseek-r1-1-5b"
```

**Flow Description**:
1. Request arrives with LoRA adapter requirement (`lora-A` or `lora-B`)
2. Gateway matches the LoRA adapter against the supported list
3. Routes to `deepseek-r1-1-5b` ModelServer configured for LoRA workloads
4. ModelServer efficiently handles LoRA adapter loading and inference

### 3. Weight-Based Traffic Distribution

**Scenario**: Gradually roll out new model versions by splitting traffic between different versions using weighted distribution.

**Traffic Processing**: The gateway uses weighted round-robin to distribute requests. For every 100 requests, approximately 70 will go to version 1 and 30 to version 2. This allows safe validation of new model versions with controlled risk.

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: deepseek-subset
  namespace: default
spec:
  modelName: "deepseek-r1"
  rules:
  - name: "deepseek-r1-route"
    targetModels:
    - modelServerName: "deepseek-r1-1-5b-v1"
      weight: 70
    - modelServerName: "deepseek-r1-1-5b-v2"
      weight: 30
```

**Flow Description**:
1. Request arrives for model "deepseek-r1"
2. Gateway applies weighted distribution algorithm
3. 70% of requests → `deepseek-r1-1-5b-v1` (stable version)
4. 30% of requests → `deepseek-r1-1-5b-v2` (new version being tested)
5. This enables controlled testing of new model versions

### 4. Header-Based Multi-Model Routing

**Scenario**: Route traffic to different model sizes based on user tier, enabling premium users to access more powerful models.

**Traffic Processing**: The gateway evaluates incoming requests in the order rules are defined. Premium users (identified by `user-type: premium` header) are routed to the 7B model, while regular users fall back to the 1.5B model.

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: deepseek-multi-models
  namespace: default
spec:
  modelName: "deepseek-r1"
  rules:
  - name: "premium"
    modelMatch:
      headers:
        user-type:
          exact: premium
    targetModels:
    - modelServerName: "deepseek-r1-7b"
  - name: "default"
    targetModels:
    - modelServerName: "deepseek-r1-1-5b"
```

**Flow Description**:
1. Request arrives for model "deepseek-r1" with headers
2. Gateway first checks if `user-type: premium` header exists with exact match
3. If premium header found → Routes to `deepseek-r1-7b` (7B model using SGLang)
4. If no premium header → Falls back to `deepseek-r1-1-5b` (1.5B model using vLLM)
5. Premium users get access to the more powerful 7B model for better performance

This comprehensive routing system enables flexible, scalable, and maintainable model serving infrastructure that can adapt to various deployment patterns and user requirements.
