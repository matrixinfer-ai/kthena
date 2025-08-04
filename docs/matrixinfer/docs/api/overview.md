---
sidebar_position: 1
---

# API Reference

MatrixInfer extends Kubernetes with custom resources to manage AI inference workloads. This section provides comprehensive documentation for all MatrixInfer APIs.

## API Groups

MatrixInfer defines three main API groups:

### Registry API Group (`registry.matrixinfer.ai/v1alpha1`)

Manages AI models and their lifecycle in the registry.

| Resource | Description |
|----------|-------------|
| [Model](registry/model.md) | Represents an AI model with metadata, specifications, and runtime requirements |
| [AutoscalingPolicy](registry/autoscaling-policy.md) | Defines scaling policies and strategies for models |

### Workload API Group (`workload.matrixinfer.ai/v1alpha1`)

Manages inference workloads and their deployment.

| Resource | Description |
|----------|-------------|
| [ModelInfer](workload/model-infer.md) | Represents an inference deployment with scaling and resource specifications |

### Networking API Group (`networking.matrixinfer.ai/v1alpha1`)

Manages network access and routing for inference services.

| Resource | Description |
|----------|-------------|
| [ModelServer](networking/model-server.md) | Exposes models through network services with routing and security policies |
| [ModelRoute](networking/model-route.md) | Defines advanced routing rules for traffic management and A/B testing |

## Common Patterns

### Resource References

Many MatrixInfer resources reference other resources using standard Kubernetes patterns:

```yaml
# Reference to another resource in the same namespace
resourceRef:
  name: "resource-name"
```

```yaml
# Reference with explicit namespace
resourceRef:
  name: "resource-name"
  namespace: "target-namespace"
```

### Labels and Selectors

MatrixInfer uses standard Kubernetes label selectors for resource matching:

```yaml
selector:
  matchLabels:
    app: "my-model"
    version: "v1"
  matchExpressions:
  - key: "environment"
    operator: "In"
    values: ["production", "staging"]
```

### Resource Requirements

Resource specifications follow Kubernetes conventions:

```yaml
resources:
  requests:
    memory: "2Gi"
    cpu: "1"
    nvidia.com/gpu: "1"
  limits:
    memory: "4Gi"
    cpu: "2"
    nvidia.com/gpu: "1"
```

## Authentication and Authorization

### RBAC Configuration

MatrixInfer resources are protected by Kubernetes RBAC. Here are common RBAC configurations:

#### Model Registry Access

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: model-registry-user
rules:
- apiGroups: ["registry.matrixinfer.ai"]
  resources: ["models"]
  verbs: ["get", "list", "create", "update", "patch"]
- apiGroups: ["registry.matrixinfer.ai"]
  resources: ["autoscalingpolicies"]
  verbs: ["get", "list"]
```

#### Inference Workload Management

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: inference-operator
rules:
- apiGroups: ["workload.matrixinfer.ai"]
  resources: ["modelinfers"]
  verbs: ["*"]
- apiGroups: ["networking.matrixinfer.ai"]
  resources: ["modelservers", "modelroutes"]
  verbs: ["*"]
```

## Status Conditions

All MatrixInfer resources follow Kubernetes conventions for status reporting:

```yaml
status:
  conditions:
  - type: "Ready"
    status: "True"
    lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: "ModelDeployed"
    message: "Model is successfully deployed and ready for inference"
  - type: "Progressing"
    status: "False"
    lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: "DeploymentComplete"
    message: "Model deployment completed successfully"
```

### Common Condition Types

| Type | Description |
|------|-------------|
| `Ready` | Resource is ready for use |
| `Progressing` | Resource is being processed or updated |
| `Available` | Resource is available for traffic |
| `Degraded` | Resource is partially functional |

## Validation and Admission Control

MatrixInfer uses admission webhooks to validate and mutate resources:

### Validation Rules

- **Resource Names**: Must follow Kubernetes naming conventions
- **Resource Limits**: Must not exceed cluster quotas
- **Model References**: Referenced models must exist and be in Ready state
- **Network Policies**: Must comply with cluster network policies

### Default Values

Admission webhooks apply sensible defaults:

- Default resource requests based on model type
- Default scaling policies for inference workloads
- Default network configurations for model servers

## Error Handling

### Common Error Codes

| Code | Description | Resolution |
|------|-------------|------------|
| `ModelNotFound` | Referenced model does not exist | Ensure model is registered and in Ready state |
| `InsufficientResources` | Not enough cluster resources | Check resource quotas and node capacity |
| `ValidationFailed` | Resource specification is invalid | Review resource specification against schema |
| `NetworkPolicyViolation` | Network configuration violates policies | Adjust network settings or policies |

### Troubleshooting

Use `kubectl describe` to get detailed error information:

```bash
# Check resource status and events
kubectl describe model my-model
kubectl describe modelinfer my-inference
kubectl describe modelserver my-server

# Check admission webhook logs
kubectl logs -n matrixinfer-system -l app=registry-webhook
kubectl logs -n matrixinfer-system -l app=modelinfer-webhook
```

## API Versioning

MatrixInfer follows Kubernetes API versioning conventions:

- **v1alpha1**: Early development, may have breaking changes
- **v1beta1**: Feature complete, API stable, implementation may change
- **v1**: Stable API, backward compatibility guaranteed

### Migration Guide

When upgrading between API versions:

1. Check the [changelog](../changelog) for breaking changes
2. Use `kubectl convert` to migrate resources
3. Test in non-production environments first
4. Update client applications and tooling

## Client Libraries

### Go Client

The MatrixInfer Go client provides programmatic access to all MatrixInfer resources:

```text
// Import the MatrixInfer client libraries
import (
    "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
    registryv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/registry/v1alpha1"
)

// Create a client
client := versioned.NewForConfig(config)

// Create a model
model := &registryv1alpha1.Model{
    ObjectMeta: metav1.ObjectMeta{
        Name: "my-model",
    },
    Spec: registryv1alpha1.ModelSpec{
        // ... model specification
    },
}

// Create the model resource
result, err := client.RegistryV1alpha1().Models("default").Create(ctx, model, metav1.CreateOptions{})
```

### kubectl Plugin

Install the MatrixInfer kubectl plugin for easier resource management:

```bash
# Install plugin
kubectl krew install matrixinfer

# Use plugin
kubectl matrixinfer models list
kubectl matrixinfer deploy my-model
kubectl matrixinfer status my-inference
```

## Examples

For complete examples and tutorials, see:

- [Getting Started Guide](../getting-started/quick-start.md)
- [Example Configurations](../examples/)
- [Best Practices](../best-practices/)

## OpenAPI Specification

The complete OpenAPI specification for all MatrixInfer APIs is available:

- [Download OpenAPI Spec](openapi.yaml)
- [Interactive API Explorer](./swagger-ui/)

## Support

For API-related questions and issues:

- [GitHub Issues](https://github.com/matrixinfer-ai/matrixinfer/issues)
- [Community Discussions](https://github.com/matrixinfer-ai/matrixinfer/discussions)
- [Slack Channel](https://matrixinfer.slack.com)