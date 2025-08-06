---
sidebar_position: 2
---

# ModelRoute

The `ModelRoute` resource defines advanced routing rules for traffic management and A/B testing in the MatrixInfer networking system.

## API Version

`networking.matrixinfer.ai/v1alpha1`

## Resource Definition

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: string
  namespace: string
spec:
  routes:
  - match:
      headers: {}
      queryParams: {}
      path: string
    destination:
      modelServerRef:
        name: string
        namespace: string
    weight: int32
    timeout: string
    retries: int32
  fallback:
    modelServerRef:
      name: string
      namespace: string
  canary:
    enabled: boolean
    trafficSplit:
      stable: int32
      canary: int32
    analysis:
      threshold: int32
      interval: string
status:
  phase: string
  activeRoutes: int32
  conditions: []
```

## Specification

### Routes

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `match` | object | Traffic matching criteria | Yes |
| `destination` | object | Target ModelServer reference | Yes |
| `weight` | int32 | Traffic weight percentage (0-100) | No |
| `timeout` | string | Request timeout for this route | No |
| `retries` | int32 | Number of retry attempts | No |

### Match

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `headers` | object | HTTP header matching | No |
| `queryParams` | object | Query parameter matching | No |
| `path` | string | Path matching pattern | No |

### Destination

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `modelServerRef` | object | Reference to ModelServer | Yes |

### Canary

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `enabled` | boolean | Enable canary deployment | No |
| `trafficSplit` | object | Traffic split configuration | No |
| `analysis` | object | Automated analysis configuration | No |

## Examples

### Basic A/B Testing

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: model-ab-test
  namespace: default
spec:
  routes:
  - match:
      headers:
        version: "v1"
    destination:
      modelServerRef:
        name: model-server-v1
    weight: 80
  - match:
      headers:
        version: "v2"
    destination:
      modelServerRef:
        name: model-server-v2
    weight: 20
  fallback:
    modelServerRef:
      name: model-server-v1
```

### Header-based Routing

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: header-routing
  namespace: production
spec:
  routes:
  - match:
      headers:
        user-type: "premium"
    destination:
      modelServerRef:
        name: premium-model-server
    timeout: "60s"
    retries: 3
  - match:
      headers:
        user-type: "standard"
    destination:
      modelServerRef:
        name: standard-model-server
    timeout: "30s"
    retries: 2
  fallback:
    modelServerRef:
      name: default-model-server
```

### Canary Deployment

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: canary-deployment
  namespace: ml-models
spec:
  routes:
  - match:
      path: "/v1/models/production/*"
    destination:
      modelServerRef:
        name: stable-model-server
    weight: 90
  - match:
      path: "/v1/models/production/*"
    destination:
      modelServerRef:
        name: canary-model-server
    weight: 10
  canary:
    enabled: true
    trafficSplit:
      stable: 90
      canary: 10
    analysis:
      threshold: 95
      interval: "5m"
```

### Query Parameter Routing

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: query-param-routing
  namespace: default
spec:
  routes:
  - match:
      queryParams:
        model: "fast"
    destination:
      modelServerRef:
        name: fast-model-server
    timeout: "10s"
  - match:
      queryParams:
        model: "accurate"
    destination:
      modelServerRef:
        name: accurate-model-server
    timeout: "60s"
  - match:
      queryParams:
        beta: "true"
    destination:
      modelServerRef:
        name: beta-model-server
    weight: 100
  fallback:
    modelServerRef:
      name: default-model-server
```

### Multi-condition Routing

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: complex-routing
  namespace: advanced
spec:
  routes:
  - match:
      headers:
        region: "us-west"
        tier: "enterprise"
      queryParams:
        priority: "high"
    destination:
      modelServerRef:
        name: us-west-enterprise-server
    timeout: "45s"
    retries: 3
  - match:
      headers:
        region: "us-east"
      path: "/v1/models/critical/*"
    destination:
      modelServerRef:
        name: us-east-critical-server
    timeout: "30s"
    retries: 2
  - match:
      headers:
        experiment: "new-algorithm"
    destination:
      modelServerRef:
        name: experimental-server
    weight: 5
  fallback:
    modelServerRef:
      name: global-fallback-server
```

## Status

The ModelRoute resource status provides information about the routing state:

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase (Pending, Active, Failed) |
| `activeRoutes` | int32 | Number of active routes |
| `conditions` | array | Detailed status conditions |

## Usage Examples

### Testing Routes

You can test different routes by sending requests with appropriate headers:

```bash
# Route to v1 model
curl -X POST http://model-gateway/v1/models/test/infer \
  -H "Content-Type: application/json" \
  -H "version: v1" \
  -d '{"inputs": {"text": "Hello"}}'

# Route to v2 model
curl -X POST http://model-gateway/v1/models/test/infer \
  -H "Content-Type: application/json" \
  -H "version: v2" \
  -d '{"inputs": {"text": "Hello"}}'

# Route based on query parameters
curl -X POST "http://model-gateway/v1/models/test/infer?model=fast" \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"text": "Hello"}}'
```

### Monitoring Routes

Check route status and traffic distribution:

```bash
# Check route status
kubectl get modelroutes

# Describe route details
kubectl describe modelroute model-ab-test

# View route metrics (if monitoring is enabled)
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/default/modelroutes/model-ab-test/request_rate
```

## Related Resources

- [ModelServer](./model-server.md) - Expose models through network services
- [ModelInfer](../workload/model-infer.md) - Deploy models for routing
- [Model](../registry/model.md) - Register models for deployment