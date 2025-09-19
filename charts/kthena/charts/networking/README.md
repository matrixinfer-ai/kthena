# Kthena Networking Chart

This chart deploys the Kthena networking components, including the infer-router and webhook.

## Configuration

### Infer Router

The infer-router is the main component that handles inference requests and provides fairness scheduling.

#### Basic Configuration

```yaml
inferRouter:
  enabled: true
  replicas: 1
  image:
    repository: ghcr.io/volcano-sh/infer-router
    tag: latest
    pullPolicy: IfNotPresent
```

#### Fairness Scheduling Configuration

The fairness scheduling system ensures equitable resource allocation among users based on their token usage history.

```yaml
inferRouter:
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
| `inferRouter.fairness.enabled` | boolean | `false` | Enable fairness scheduling |
| `inferRouter.fairness.windowSize` | string | `"5m"` | Sliding window duration (1m-1h) |
| `inferRouter.fairness.inputTokenWeight` | float | `1.0` | Weight for input tokens (≥0) |
| `inferRouter.fairness.outputTokenWeight` | float | `2.0` | Weight for output tokens (≥0) |

#### Configuration Scenarios

##### Development Environment
```yaml
inferRouter:
  fairness:
    enabled: true
    windowSize: "2m"          # Short window for quick feedback
    inputTokenWeight: 1.0     # Equal weights for simplicity
    outputTokenWeight: 1.0
```

##### Production Environment
```yaml
inferRouter:
  fairness:
    enabled: true
    windowSize: "10m"         # Balanced window size
    inputTokenWeight: 1.0     # Realistic cost ratios
    outputTokenWeight: 2.5
```

##### Cost-Sensitive Environment
```yaml
inferRouter:
  fairness:
    enabled: true
    windowSize: "30m"         # Longer window for stability
    inputTokenWeight: 1.0     # High output weight for cost control
    outputTokenWeight: 4.0
```

### TLS Configuration

```yaml
inferRouter:
  tls:
    enabled: true
    dnsName: "your-domain.com"
    secretName: "infer-router-tls"
```

### Resource Configuration

```yaml
inferRouter:
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
helm install kthena ./charts/kthena
```

### With Fairness Scheduling
```bash
helm install kthena ./charts/kthena \
  --set networking.inferRouter.fairness.enabled=true \
  --set networking.inferRouter.fairness.windowSize=10m \
  --set networking.inferRouter.fairness.outputTokenWeight=3.0
```
