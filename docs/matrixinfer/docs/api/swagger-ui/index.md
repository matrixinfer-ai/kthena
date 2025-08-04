---
sidebar_position: 10
---

# Swagger UI

Interactive API documentation for MatrixInfer resources.

## Overview

This page provides an interactive Swagger UI interface for exploring the MatrixInfer API. You can view detailed specifications, try out API calls, and understand the request/response formats for all MatrixInfer custom resources.

## API Specification

The complete OpenAPI specification is available at: [openapi.yaml](../openapi.yaml)

## Interactive Documentation

## Interactive API Explorer

You can explore the MatrixInfer API interactively using Swagger UI. The complete OpenAPI specification includes all resource definitions, endpoints, and examples.

To view the interactive documentation:

1. Open the [OpenAPI specification](../openapi.yaml) in your browser
2. Use an online Swagger UI viewer like [Swagger Editor](https://editor.swagger.io/)
3. Copy and paste the OpenAPI specification content

## API Resources

The MatrixInfer API includes the following main resource types:

### Registry API Group (`registry.matrixinfer.ai/v1alpha1`)

- **Model** - Register and manage AI models
- **AutoscalingPolicy** - Define scaling policies for models

### Workload API Group (`workload.matrixinfer.ai/v1alpha1`)

- **ModelInfer** - Deploy models for inference with scaling specifications

### Networking API Group (`networking.matrixinfer.ai/v1alpha1`)

- **ModelServer** - Expose models through network services
- **ModelRoute** - Define advanced routing rules for traffic management

## Usage Examples

### Authentication

Most MatrixInfer API calls require proper Kubernetes authentication:

```bash
# Using kubectl proxy
kubectl proxy --port=8080

# API calls through proxy
curl http://localhost:8080/apis/registry.matrixinfer.ai/v1alpha1/models
```

### Common Operations

#### List Models

```bash
curl -X GET \
  "https://api.matrixinfer.ai/v1alpha1/models" \
  -H "Authorization: Bearer $TOKEN"
```

#### Create a Model

```bash
curl -X POST \
  "https://api.matrixinfer.ai/v1alpha1/models" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "apiVersion": "registry.matrixinfer.ai/v1alpha1",
    "kind": "Model",
    "metadata": {
      "name": "my-model",
      "namespace": "default"
    },
    "spec": {
      "modelSpec": {
        "modelId": "my-model-v1",
        "framework": "pytorch",
        "version": "1.0.0",
        "source": {
          "uri": "s3://my-models/my-model-v1/"
        }
      },
      "runtime": {
        "image": "matrixinfer/pytorch-runtime:latest"
      }
    }
  }'
```

#### Get Model Status

```bash
curl -X GET \
  "https://api.matrixinfer.ai/v1alpha1/models/my-model?namespace=default" \
  -H "Authorization: Bearer $TOKEN"
```

## Response Formats

All API responses follow standard Kubernetes API conventions:

### Success Response

```json
{
  "apiVersion": "registry.matrixinfer.ai/v1alpha1",
  "kind": "Model",
  "metadata": {
    "name": "my-model",
    "namespace": "default",
    "uid": "12345678-1234-1234-1234-123456789012",
    "creationTimestamp": "2024-08-04T15:18:00Z"
  },
  "spec": {
    "modelSpec": {
      "modelId": "my-model-v1",
      "framework": "pytorch",
      "version": "1.0.0"
    }
  },
  "status": {
    "phase": "Ready",
    "conditions": []
  }
}
```

### Error Response

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

## Related Documentation

- [API Overview](../overview.md) - High-level API concepts and patterns
- [Model Resource](../registry/model.md) - Detailed Model resource documentation
- [ModelInfer Resource](../workload/model-infer.md) - Detailed ModelInfer resource documentation
- [ModelServer Resource](../networking/model-server.md) - Detailed ModelServer resource documentation

## Support

For API-related questions or issues:

1. Check the [troubleshooting guide](../../troubleshooting.md)
2. Review the resource-specific documentation
3. Open an issue on [GitHub](https://github.com/matrixinfer-ai/matrixinfer/issues)