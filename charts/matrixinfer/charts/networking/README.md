# MatrixInfer Networking Chart

This chart deploys the MatrixInfer networking components, including the infer-gateway and webhook.

## Configuration

### Infer Gateway

The infer-gateway is the main component that handles inference requests and provides fairness scheduling.

#### Basic Configuration

```yaml
inferGateway:
  enabled: true
  replicas: 1
  image:
    repository: ghcr.io/matrixinfer-ai/infer-gateway
    tag: latest
    pullPolicy: IfNotPresent
```

#### Fairness Scheduling Configuration

The fairness scheduling system ensures equitable resource allocation among users based on their token usage history.

```yaml
inferGateway:
  fairness:
    # Enable fairness scheduling (default: false)
    enabled: true
    
    # Sliding window duration for token tracking (default: "1h")
    # Valid formats: 1m, 5m, 10m, 30m, 1h
    windowSize: "10m"
    
    # Token weights for priority calculation
    # Input token weight (default: 1.0)
    inputTokenWeight: 1.0
    
    # Output token weight (default: 2.0)
    # Typically higher than input weight due to generation cost
    outputTokenWeight: 2.0
```

#### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `inferGateway.fairness.enabled` | boolean | `false` | Enable fairness scheduling |
| `inferGateway.fairness.windowSize` | string | `"5m"` | Sliding window duration (1m-1h) |
| `inferGateway.fairness.inputTokenWeight` | float | `1.0` | Weight for input tokens (≥0) |
| `inferGateway.fairness.outputTokenWeight` | float | `2.0` | Weight for output tokens (≥0) |
| `inferGateway.fairness.queueQPS` | integer | `100` | Queue processing rate (>0) |

#### Configuration Scenarios

##### Development Environment
```yaml
inferGateway:
  fairness:
    enabled: true
    windowSize: "2m"          # Short window for quick feedback
    inputTokenWeight: 1.0     # Equal weights for simplicity
    outputTokenWeight: 1.0
    queueQPS: 50             # Lower rate for resource conservation
```

##### Production Environment
```yaml
inferGateway:
  fairness:
    enabled: true
    windowSize: "10m"         # Balanced window size
    inputTokenWeight: 1.0     # Realistic cost ratios
    outputTokenWeight: 2.5
    queueQPS: 150            # Higher rate for performance
```

##### Cost-Sensitive Environment
```yaml
inferGateway:
  fairness:
    enabled: true
    windowSize: "30m"         # Longer window for stability
    inputTokenWeight: 1.0     # High output weight for cost control
    outputTokenWeight: 4.0
    queueQPS: 75             # Conservative processing rate
```

### TLS Configuration

```yaml
inferGateway:
  tls:
    enabled: true
    dnsName: "your-domain.com"
    secretName: "infer-gateway-tls"
```

### Resource Configuration

```yaml
inferGateway:
  resource:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi
```

## Installation

### Basic Installation
```bash
helm install matrixinfer ./charts/matrixinfer
```

### With Fairness Scheduling
```bash
helm install matrixinfer ./charts/matrixinfer \
  --set networking.inferGateway.fairness.enabled=true \
  --set networking.inferGateway.fairness.windowSize=10m \
  --set networking.inferGateway.fairness.outputTokenWeight=3.0
```
