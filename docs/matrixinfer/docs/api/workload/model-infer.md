---
sidebar_position: 1
---

# ModelInfer

The `ModelInfer` resource represents an inference deployment with scaling and resource specifications in the MatrixInfer workload system.

## API Version

`workload.matrixinfer.ai/v1alpha1`

## Resource Definition

```yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: string
  namespace: string
spec:
  modelRef:
    name: string
    namespace: string
  replicas: int32
  autoscaling:
    enabled: boolean
    minReplicas: int32
    maxReplicas: int32
    targetCPUUtilizationPercentage: int32
    metrics: []
  resources:
    requests:
      memory: string
      cpu: string
      nvidia.com/gpu: string
    limits:
      memory: string
      cpu: string
      nvidia.com/gpu: string
  nodeSelector: {}
  tolerations: []
  affinity: {}
status:
  phase: string
  replicas: int32
  readyReplicas: int32
  conditions: []
```

## Specification

### ModelRef

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `name` | string | Name of the Model resource | Yes |
| `namespace` | string | Namespace of the Model resource | No |

### Autoscaling

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `enabled` | boolean | Enable autoscaling | No |
| `minReplicas` | int32 | Minimum number of replicas | No |
| `maxReplicas` | int32 | Maximum number of replicas | No |
| `targetCPUUtilizationPercentage` | int32 | Target CPU utilization percentage | No |

### Resources

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `requests` | object | Resource requests | No |
| `limits` | object | Resource limits | No |

## Examples

### Basic Model Inference Deployment

```yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: llama-7b-infer
  namespace: default
spec:
  modelRef:
    name: llama-7b
  replicas: 2
  resources:
    requests:
      memory: "8Gi"
      cpu: "2"
    limits:
      memory: "16Gi"
      cpu: "4"
```

### GPU-enabled Inference with Autoscaling

```yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: gpu-model-infer
  namespace: ml-models
spec:
  modelRef:
    name: custom-gpu-model
  replicas: 1
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
    targetCPUUtilizationPercentage: 70
  resources:
    requests:
      memory: "16Gi"
      cpu: "4"
      nvidia.com/gpu: "1"
    limits:
      memory: "32Gi"
      cpu: "8"
      nvidia.com/gpu: "1"
  nodeSelector:
    accelerator: nvidia-tesla-v100
```

### Advanced Inference with Node Affinity

```yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: production-infer
  namespace: production
spec:
  modelRef:
    name: production-model
  replicas: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 60
  resources:
    requests:
      memory: "4Gi"
      cpu: "2"
    limits:
      memory: "8Gi"
      cpu: "4"
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node-type
            operator: In
            values:
            - inference
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
            - key: app
              operator: In
              values:
              - production-infer
          topologyKey: kubernetes.io/hostname
  tolerations:
  - key: inference-node
    operator: Equal
    value: "true"
    effect: NoSchedule
```

## Status

The ModelInfer resource status provides information about the deployment state:

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase (Pending, Running, Failed) |
| `replicas` | int32 | Total number of replicas |
| `readyReplicas` | int32 | Number of ready replicas |
| `conditions` | array | Detailed status conditions |

## Related Resources

- [Model](../registry/model.md) - Register models for inference
- [ModelServer](../networking/model-server.md) - Expose inference services
- [AutoscalingPolicy](../registry/autoscaling-policy.md) - Configure advanced scaling policies