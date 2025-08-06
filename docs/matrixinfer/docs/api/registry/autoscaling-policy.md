---
sidebar_position: 2
---

# AutoscalingPolicy

The `AutoscalingPolicy` resource defines scaling policies and strategies for models in the MatrixInfer system.

## API Version

`registry.matrixinfer.ai/v1alpha1`

## Resource Definition

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: string
  namespace: string
spec:
  targetRef:
    apiVersion: string
    kind: string
    name: string
  minReplicas: int32
  maxReplicas: int32
  metrics:
  - type: string
    resource:
      name: string
      target:
        type: string
        averageUtilization: int32
  behavior:
    scaleUp:
      stabilizationWindowSeconds: int32
      policies:
      - type: string
        value: int32
        periodSeconds: int32
    scaleDown:
      stabilizationWindowSeconds: int32
      policies:
      - type: string
        value: int32
        periodSeconds: int32
status:
  currentReplicas: int32
  desiredReplicas: int32
  conditions: []
```

## Specification

### TargetRef

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `apiVersion` | string | API version of target resource | Yes |
| `kind` | string | Kind of target resource | Yes |
| `name` | string | Name of target resource | Yes |

### Metrics

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `type` | string | Metric type (Resource, Pods, Object, External) | Yes |
| `resource` | object | Resource metric configuration | Conditional |

### Behavior

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `scaleUp` | object | Scale up behavior configuration | No |
| `scaleDown` | object | Scale down behavior configuration | No |

## Examples

### Basic CPU-based Autoscaling

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: llama-autoscaling
  namespace: default
spec:
  targetRef:
    apiVersion: workload.matrixinfer.ai/v1alpha1
    kind: ModelInfer
    name: llama-7b-infer
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Multi-metric Autoscaling

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: advanced-autoscaling
  namespace: ml-models
spec:
  targetRef:
    apiVersion: workload.matrixinfer.ai/v1alpha1
    kind: ModelInfer
    name: custom-model-infer
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 60
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Resource
    resource:
      name: nvidia.com/gpu
      target:
        type: Utilization
        averageUtilization: 75
```

### Custom Scaling Behavior

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: conservative-autoscaling
  namespace: production
spec:
  targetRef:
    apiVersion: workload.matrixinfer.ai/v1alpha1
    kind: ModelInfer
    name: production-model
  minReplicas: 3
  maxReplicas: 15
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
      - type: Pods
        value: 2
        periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 600
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
```

## Status

The AutoscalingPolicy resource status provides information about the current scaling state:

| Field | Type | Description |
|-------|------|-------------|
| `currentReplicas` | int32 | Current number of replicas |
| `desiredReplicas` | int32 | Desired number of replicas |
| `conditions` | array | Detailed status conditions |

## Related Resources

- [Model](./model.md) - Register models for autoscaling
- [ModelInfer](../workload/model-infer.md) - Deploy models with autoscaling policies