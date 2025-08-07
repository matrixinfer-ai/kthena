---
sidebar_position: 1
---

# Model

The `Model` resource represents an AI model with metadata, specifications, and runtime requirements in the MatrixInfer registry.

## API Version

`registry.matrixinfer.ai/v1alpha1`

## Resource Definition

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: string
  namespace: string
spec:
  modelSpec:
    modelId: string
    framework: string
    version: string
    source:
      uri: string
  runtime:
    image: string
    resources:
      requests:
        memory: string
        cpu: string
      limits:
        memory: string
        cpu: string
status:
  phase: string
  conditions: []
```

## Specification

### ModelSpec

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `modelId` | string | Unique identifier for the model | Yes |
| `framework` | string | ML framework (pytorch, tensorflow, onnx, etc.) | Yes |
| `version` | string | Model version | Yes |
| `source` | object | Model source configuration | Yes |

### Source

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `uri` | string | URI to model artifacts (s3://, huggingface://, etc.) | Yes |

### Runtime

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `image` | string | Container image for model runtime | Yes |
| `resources` | object | Resource requirements | No |

## Examples

### Basic Model Registration

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: llama-7b
  namespace: default
spec:
  modelSpec:
    modelId: "llama-7b-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "huggingface://meta-llama/Llama-2-7b-hf"
  runtime:
    image: "matrixinfer/pytorch-runtime:latest"
    resources:
      requests:
        memory: "8Gi"
        cpu: "2"
      limits:
        memory: "16Gi"
        cpu: "4"
```

### Model with S3 Source

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: custom-model
  namespace: ml-models
spec:
  modelSpec:
    modelId: "custom-model-v2"
    framework: "onnx"
    version: "2.0.0"
    source:
      uri: "s3://my-models/custom-model-v2/"
  runtime:
    image: "matrixinfer/onnx-runtime:latest"
    resources:
      requests:
        memory: "4Gi"
        cpu: "1"
        nvidia.com/gpu: "1"
      limits:
        memory: "8Gi"
        cpu: "2"
        nvidia.com/gpu: "1"
```

## Status

The Model resource status provides information about the model registration state:

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase (Pending, Ready, Failed) |
| `conditions` | array | Detailed status conditions |

## Related Resources

- [ModelInfer](../workload/model-infer.md) - Deploy models for inference
- [AutoscalingPolicy](./autoscaling-policy.md) - Configure scaling policies