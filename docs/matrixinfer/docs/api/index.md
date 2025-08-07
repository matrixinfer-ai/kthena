---
sidebar_position: 4
---

# API Reference

Complete API documentation for MatrixInfer custom resources.

## Overview

MatrixInfer extends Kubernetes with custom resources that provide a declarative API for managing AI model inference workloads. This section provides comprehensive documentation for all MatrixInfer APIs.

## API Groups

MatrixInfer organizes its APIs into three main groups:

### Registry API Group (`registry.matrixinfer.ai/v1alpha1`)

Manages AI models and their associated policies.

| Resource | Description |
|----------|-------------|
| [Model](./registry/model.md) | Represents an AI model with metadata, specifications, and runtime requirements |
| [AutoscalingPolicy](./registry/autoscaling-policy.md) | Defines scaling policies and strategies for models |

### Workload API Group (`workload.matrixinfer.ai/v1alpha1`)

Manages inference workloads and their deployment.

| Resource | Description |
|----------|-------------|
| [ModelInfer](./workload/model-infer.md) | Represents an inference deployment with scaling and resource specifications |

### Networking API Group (`networking.matrixinfer.ai/v1alpha1`)

Manages network access and routing for inference services.

| Resource | Description |
|----------|-------------|
| [ModelServer](./networking/model-server.md) | Exposes models through network services with routing and security policies |
| [ModelRoute](./networking/model-route.md) | Defines advanced routing rules for traffic management and A/B testing |

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

## API Specifications

### OpenAPI Specification

The complete OpenAPI specification is available at: [openapi.yaml](./openapi.yaml)

### Interactive Documentation

Explore the API interactively using our Swagger UI: [Swagger UI](./swagger-ui/)

## Quick Start

### Basic Model Deployment

Here's a complete example of deploying a model using MatrixInfer APIs:

```yaml
# 1. Register the model
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: example-model
  namespace: default
spec:
  modelSpec:
    modelId: "example-model-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "s3://my-models/example-model/"
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
# 2. Deploy for inference
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: example-model-infer
  namespace: default
spec:
  modelRef:
    name: example-model
  replicas: 2
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
    targetCPUUtilizationPercentage: 70
---
# 3. Expose via network
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: example-model-server
  namespace: default
spec:
  modelInferRef:
    name: example-model-infer
  service:
    type: LoadBalancer
    port: 8080
  routing:
    path: "/v1/models/example"
    timeout: "30s"
  rateLimit:
    enabled: true
    requestsPerSecond: 100
```

### Applying the Configuration

```bash
# Apply all resources
kubectl apply -f model-deployment.yaml

# Check status
kubectl get models,modelinfers,modelservers

# Test the endpoint
curl -X POST http://<service-ip>:8080/v1/models/example/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"text": "Hello world"}}'
```

## API Versioning

MatrixInfer follows Kubernetes API versioning conventions:

- **v1alpha1**: Early development version, may have breaking changes
- **v1beta1**: API is stable, but may have minor changes before v1
- **v1**: Stable API with backward compatibility guarantees

Current API versions:
- `registry.matrixinfer.ai/v1alpha1`
- `workload.matrixinfer.ai/v1alpha1`
- `networking.matrixinfer.ai/v1alpha1`

## Authentication and Authorization

### RBAC Configuration

MatrixInfer integrates with Kubernetes RBAC for access control:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: model-manager
rules:
- apiGroups: ["registry.matrixinfer.ai"]
  resources: ["models"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["workload.matrixinfer.ai"]
  resources: ["modelinfers"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["networking.matrixinfer.ai"]
  resources: ["modelservers", "modelroutes"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### Service Account

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: model-manager
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: model-manager-binding
  namespace: default
subjects:
- kind: ServiceAccount
  name: model-manager
  namespace: default
roleRef:
  kind: Role
  name: model-manager
  apiGroup: rbac.authorization.k8s.io
```

## Error Handling

### Common Error Responses

MatrixInfer follows Kubernetes error response conventions:

```json
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {},
  "status": "Failure",
  "message": "models.registry.matrixinfer.ai \"my-model\" not found",
  "reason": "NotFound",
  "details": {
    "name": "my-model",
    "group": "registry.matrixinfer.ai",
    "kind": "models"
  },
  "code": 404
}
```

### Validation Errors

```json
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {},
  "status": "Failure",
  "message": "Model.registry.matrixinfer.ai \"my-model\" is invalid: spec.modelSpec.framework: Required value",
  "reason": "Invalid",
  "details": {
    "name": "my-model",
    "group": "registry.matrixinfer.ai",
    "kind": "Model",
    "causes": [
      {
        "reason": "FieldValueRequired",
        "message": "Required value",
        "field": "spec.modelSpec.framework"
      }
    ]
  },
  "code": 422
}
```

## Client Libraries

### kubectl

Use kubectl to interact with MatrixInfer resources:

```bash
# List all models
kubectl get models

# Describe a specific model
kubectl describe model my-model

# Get model in YAML format
kubectl get model my-model -o yaml

# Apply configuration
kubectl apply -f model.yaml

# Delete a model
kubectl delete model my-model
```

### Kubernetes Client Libraries

MatrixInfer resources work with standard Kubernetes client libraries:

**Go:**
```go
package main

import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

func main() {
    config, err := rest.InClusterConfig()
    clientset, err := kubernetes.NewForConfig(config)
}
```

**Python:**
```python
from kubernetes import client, config

config.load_incluster_config()
v1 = client.CustomObjectsApi()

# List models
models = v1.list_namespaced_custom_object(
    group="registry.matrixinfer.ai",
    version="v1alpha1",
    namespace="default",
    plural="models"
)
```

## Migration Guide

### Upgrading API Versions

When upgrading between API versions:

1. **Check compatibility**: Review breaking changes in release notes
2. **Update manifests**: Modify apiVersion fields in your YAML files
3. **Test in staging**: Validate changes in a non-production environment
4. **Gradual rollout**: Update resources incrementally

### Backward Compatibility

MatrixInfer maintains backward compatibility within major versions:

- **v1alpha1 → v1alpha2**: May have breaking changes
- **v1beta1 → v1beta2**: Minor changes only
- **v1 → v1.1**: Full backward compatibility

## Related Documentation

- [Getting Started](../getting-started/installation.md) - Installation and setup
- [Examples](../examples/) - Practical usage examples
- [Best Practices](../best-practices/) - Production deployment guidelines
- [Troubleshooting](../troubleshooting.md) - Common issues and solutions

## Support

For API-related questions:

1. Check the resource-specific documentation
2. Review the [FAQ](../faq.md)
3. Search [GitHub Issues](https://github.com/matrixinfer-ai/matrixinfer/issues)
4. Create a new issue with the "api" label