---
sidebar_position: 1
---

# ModelServer

The `ModelServer` resource exposes models through network services with routing and security policies in the MatrixInfer networking system.

## API Version

`networking.matrixinfer.ai/v1alpha1`

## Resource Definition

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: string
  namespace: string
spec:
  modelInferRef:
    name: string
    namespace: string
  service:
    type: string
    port: int32
    targetPort: int32
    annotations: {}
  routing:
    path: string
    timeout: string
    retries: int32
  rateLimit:
    enabled: boolean
    requestsPerSecond: int32
    burstSize: int32
  authentication:
    enabled: boolean
    type: string
    config: {}
  tls:
    enabled: boolean
    secretName: string
status:
  phase: string
  serviceEndpoint: string
  conditions: []
```

## Specification

### ModelInferRef

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `name` | string | Name of the ModelInfer resource | Yes |
| `namespace` | string | Namespace of the ModelInfer resource | No |

### Service

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `type` | string | Service type (ClusterIP, NodePort, LoadBalancer) | No |
| `port` | int32 | Service port | Yes |
| `targetPort` | int32 | Target port on pods | No |
| `annotations` | object | Service annotations | No |

### Routing

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `path` | string | URL path for the model endpoint | Yes |
| `timeout` | string | Request timeout duration | No |
| `retries` | int32 | Number of retry attempts | No |

### RateLimit

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `enabled` | boolean | Enable rate limiting | No |
| `requestsPerSecond` | int32 | Requests per second limit | No |
| `burstSize` | int32 | Burst size for rate limiting | No |

## Examples

### Basic Model Server

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: llama-server
  namespace: default
spec:
  modelInferRef:
    name: llama-7b-infer
  service:
    type: ClusterIP
    port: 8080
  routing:
    path: "/v1/models/llama-7b"
    timeout: "30s"
```

### LoadBalancer with Rate Limiting

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: public-model-server
  namespace: production
spec:
  modelInferRef:
    name: production-model-infer
  service:
    type: LoadBalancer
    port: 443
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
  routing:
    path: "/v1/models/production"
    timeout: "60s"
    retries: 3
  rateLimit:
    enabled: true
    requestsPerSecond: 100
    burstSize: 200
  tls:
    enabled: true
    secretName: model-server-tls
```

### Authenticated Model Server

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: secure-model-server
  namespace: ml-models
spec:
  modelInferRef:
    name: sensitive-model-infer
  service:
    type: ClusterIP
    port: 8080
  routing:
    path: "/v1/models/secure"
    timeout: "45s"
  authentication:
    enabled: true
    type: "jwt"
    config:
      issuer: "https://auth.company.com"
      audience: "matrixinfer-api"
  rateLimit:
    enabled: true
    requestsPerSecond: 50
    burstSize: 100
```

### Multi-Port Model Server

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: multi-port-server
  namespace: default
spec:
  modelInferRef:
    name: multi-model-infer
  service:
    type: NodePort
    port: 8080
    targetPort: 8080
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "9090"
  routing:
    path: "/v1/models/multi"
    timeout: "30s"
    retries: 2
  rateLimit:
    enabled: true
    requestsPerSecond: 200
    burstSize: 400
```

## Status

The ModelServer resource status provides information about the service state:

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase (Pending, Ready, Failed) |
| `serviceEndpoint` | string | Service endpoint URL |
| `conditions` | array | Detailed status conditions |

## Usage Examples

### Making Requests

Once a ModelServer is deployed, you can make inference requests:

```bash
# For ClusterIP service (from within cluster)
curl -X POST http://llama-server:8080/v1/models/llama-7b/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"text": "Hello world"}}'

# For LoadBalancer service
curl -X POST https://external-ip/v1/models/production/infer \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"inputs": {"text": "Hello world"}}'
```

### Health Checks

ModelServer automatically provides health check endpoints:

```bash
# Health check
curl http://model-server:8080/health

# Readiness check
curl http://model-server:8080/ready

# Metrics (if enabled)
curl http://model-server:8080/metrics
```

## Related Resources

- [ModelInfer](../workload/model-infer.md) - Deploy models for serving
- [ModelRoute](./model-route.md) - Advanced routing and traffic management
- [Model](../registry/model.md) - Register models for serving