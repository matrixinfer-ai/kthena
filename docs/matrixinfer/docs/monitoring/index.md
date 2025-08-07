---
sidebar_position: 8
---

# Monitoring

Comprehensive monitoring and observability setup for MatrixInfer deployments.

## Overview

Effective monitoring is crucial for maintaining reliable AI model inference services. This guide covers setting up comprehensive monitoring, alerting, and observability for MatrixInfer deployments using industry-standard tools and practices.

## Monitoring Stack

### Core Components

**Prometheus** - Metrics collection and storage
**Grafana** - Visualization and dashboards
**Jaeger** - Distributed tracing
**AlertManager** - Alert routing and management
**Loki** - Log aggregation (optional)

### Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   MatrixInfer   │───▶│   Prometheus    │───▶│     Grafana     │
│   Components    │    │                 │    │   Dashboards    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│     Jaeger      │    │  AlertManager   │    │      Loki       │
│    Tracing      │    │    Alerts       │    │      Logs       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Prometheus Setup

### Installation

**Using Helm:**
```bash
# Add Prometheus community helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install Prometheus stack
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false
```

### Configuration

**Prometheus Configuration:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    
    rule_files:
    - "/etc/prometheus/rules/*.yml"
    
    scrape_configs:
    - job_name: 'matrixinfer-models'
      kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ['default', 'production', 'staging']
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__
    
    - job_name: 'matrixinfer-controllers'
      static_configs:
      - targets: ['matrixinfer-controller:8080']
      metrics_path: /metrics
      scrape_interval: 30s
```

### ServiceMonitor for MatrixInfer

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: matrixinfer-metrics
  namespace: monitoring
  labels:
    app: matrixinfer
spec:
  selector:
    matchLabels:
      app: matrixinfer
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
    honorLabels: true
  namespaceSelector:
    matchNames:
    - default
    - production
    - staging
```

## Key Metrics

### Model Performance Metrics

**Inference Metrics:**
```yaml
# Request rate
matrixinfer_inference_requests_total
# Request duration
matrixinfer_inference_duration_seconds
# Request size
matrixinfer_inference_request_size_bytes
# Response size
matrixinfer_inference_response_size_bytes
# Error rate
matrixinfer_inference_errors_total
```

**Model Loading Metrics:**
```yaml
# Model load time
matrixinfer_model_load_duration_seconds
# Model load failures
matrixinfer_model_load_failures_total
# Model memory usage
matrixinfer_model_memory_usage_bytes
# Model status
matrixinfer_model_status
```

**Resource Utilization:**
```yaml
# CPU usage
container_cpu_usage_seconds_total
# Memory usage
container_memory_usage_bytes
# GPU utilization
nvidia_gpu_utilization_percent
# GPU memory usage
nvidia_gpu_memory_usage_bytes
```

### Custom Metrics Configuration

**Model-specific Metrics:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: matrixinfer-metrics-config
data:
  metrics.yaml: |
    custom_metrics:
      - name: model_accuracy_score
        type: gauge
        help: "Current model accuracy score"
        labels: ["model_id", "version", "dataset"]
      
      - name: inference_queue_depth
        type: gauge
        help: "Number of requests waiting in inference queue"
        labels: ["model_id", "priority"]
      
      - name: model_cache_hit_ratio
        type: gauge
        help: "Cache hit ratio for model predictions"
        labels: ["model_id", "cache_type"]
      
      - name: batch_processing_efficiency
        type: histogram
        help: "Efficiency of batch processing"
        labels: ["model_id", "batch_size"]
        buckets: [0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 1.0]
```

## Grafana Dashboards

### Installation and Configuration

**Grafana Configuration:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-config
data:
  grafana.ini: |
    [server]
    root_url = https://grafana.company.com
    
    [security]
    admin_user = admin
    admin_password = $__env{GRAFANA_ADMIN_PASSWORD}
    
    [auth.generic_oauth]
    enabled = true
    name = OAuth
    allow_sign_up = true
    client_id = $__env{OAUTH_CLIENT_ID}
    client_secret = $__env{OAUTH_CLIENT_SECRET}
    scopes = openid profile email
    auth_url = https://auth.company.com/oauth/authorize
    token_url = https://auth.company.com/oauth/token
    api_url = https://auth.company.com/oauth/userinfo
```

### MatrixInfer Dashboard

**Main Dashboard JSON:**
```json
{
  "dashboard": {
    "id": null,
    "title": "MatrixInfer Overview",
    "tags": ["matrixinfer", "ml", "inference"],
    "timezone": "browser",
    "panels": [
      {
        "id": 1,
        "title": "Inference Requests/sec",
        "type": "stat",
        "targets": [
          {
            "expr": "sum(rate(matrixinfer_inference_requests_total[5m]))",
            "legendFormat": "Requests/sec"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "color": {
              "mode": "thresholds"
            },
            "thresholds": {
              "steps": [
                {"color": "green", "value": null},
                {"color": "yellow", "value": 100},
                {"color": "red", "value": 1000}
              ]
            }
          }
        }
      },
      {
        "id": 2,
        "title": "Inference Latency (P95)",
        "type": "stat",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, sum(rate(matrixinfer_inference_duration_seconds_bucket[5m])) by (le))",
            "legendFormat": "P95 Latency"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "s",
            "thresholds": {
              "steps": [
                {"color": "green", "value": null},
                {"color": "yellow", "value": 1},
                {"color": "red", "value": 5}
              ]
            }
          }
        }
      },
      {
        "id": 3,
        "title": "Error Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "sum(rate(matrixinfer_inference_errors_total[5m])) / sum(rate(matrixinfer_inference_requests_total[5m])) * 100",
            "legendFormat": "Error Rate %"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "thresholds": {
              "steps": [
                {"color": "green", "value": null},
                {"color": "yellow", "value": 1},
                {"color": "red", "value": 5}
              ]
            }
          }
        }
      },
      {
        "id": 4,
        "title": "Active Models",
        "type": "stat",
        "targets": [
          {
            "expr": "count(matrixinfer_model_status == 1)",
            "legendFormat": "Active Models"
          }
        ]
      },
      {
        "id": 5,
        "title": "Request Rate by Model",
        "type": "graph",
        "targets": [
          {
            "expr": "sum(rate(matrixinfer_inference_requests_total[5m])) by (model_id)",
            "legendFormat": "{{model_id}}"
          }
        ],
        "xAxis": {
          "show": true
        },
        "yAxes": [
          {
            "label": "Requests/sec",
            "show": true
          }
        ]
      },
      {
        "id": 6,
        "title": "Memory Usage by Model",
        "type": "graph",
        "targets": [
          {
            "expr": "matrixinfer_model_memory_usage_bytes / 1024 / 1024 / 1024",
            "legendFormat": "{{model_id}}"
          }
        ],
        "yAxes": [
          {
            "label": "Memory (GB)",
            "show": true
          }
        ]
      },
      {
        "id": 7,
        "title": "GPU Utilization",
        "type": "graph",
        "targets": [
          {
            "expr": "nvidia_gpu_utilization_percent",
            "legendFormat": "GPU {{gpu_id}} - {{model_id}}"
          }
        ],
        "yAxes": [
          {
            "label": "Utilization %",
            "max": 100,
            "show": true
          }
        ]
      },
      {
        "id": 8,
        "title": "Autoscaling Activity",
        "type": "graph",
        "targets": [
          {
            "expr": "kube_deployment_status_replicas{deployment=~\".*-infer\"}",
            "legendFormat": "{{deployment}} - Replicas"
          }
        ],
        "yAxes": [
          {
            "label": "Replica Count",
            "show": true
          }
        ]
      }
    ],
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "refresh": "30s"
  }
}
```

### Model-specific Dashboard

**Per-Model Dashboard:**
```json
{
  "dashboard": {
    "title": "MatrixInfer Model: $model_id",
    "templating": {
      "list": [
        {
          "name": "model_id",
          "type": "query",
          "query": "label_values(matrixinfer_inference_requests_total, model_id)",
          "refresh": 1
        }
      ]
    },
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "sum(rate(matrixinfer_inference_requests_total{model_id=\"$model_id\"}[5m]))",
            "legendFormat": "Requests/sec"
          }
        ]
      },
      {
        "title": "Latency Distribution",
        "type": "heatmap",
        "targets": [
          {
            "expr": "sum(rate(matrixinfer_inference_duration_seconds_bucket{model_id=\"$model_id\"}[5m])) by (le)",
            "format": "heatmap",
            "legendFormat": "{{le}}"
          }
        ]
      },
      {
        "title": "Model Accuracy Over Time",
        "type": "graph",
        "targets": [
          {
            "expr": "model_accuracy_score{model_id=\"$model_id\"}",
            "legendFormat": "Accuracy"
          }
        ]
      }
    ]
  }
}
```

## Alerting

### AlertManager Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
data:
  alertmanager.yml: |
    global:
      smtp_smarthost: 'smtp.company.com:587'
      smtp_from: 'alerts@company.com'
      slack_api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
    
    route:
      group_by: ['alertname', 'cluster', 'service']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 1h
      receiver: 'default'
      routes:
      - match:
          severity: critical
        receiver: 'critical-alerts'
      - match:
          service: matrixinfer
        receiver: 'matrixinfer-team'
    
    receivers:
    - name: 'default'
      email_configs:
      - to: 'ops-team@company.com'
        subject: 'Alert: {{ .GroupLabels.alertname }}'
        body: |
          {{ range .Alerts }}
          Alert: {{ .Annotations.summary }}
          Description: {{ .Annotations.description }}
          {{ end }}
    
    - name: 'critical-alerts'
      slack_configs:
      - channel: '#critical-alerts'
        title: 'Critical Alert: {{ .GroupLabels.alertname }}'
        text: |
          {{ range .Alerts }}
          *Alert:* {{ .Annotations.summary }}
          *Description:* {{ .Annotations.description }}
          *Severity:* {{ .Labels.severity }}
          {{ end }}
      email_configs:
      - to: 'oncall@company.com'
        subject: 'CRITICAL: {{ .GroupLabels.alertname }}'
    
    - name: 'matrixinfer-team'
      slack_configs:
      - channel: '#ml-ops'
        title: 'MatrixInfer Alert: {{ .GroupLabels.alertname }}'
```

### Prometheus Alert Rules

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: matrixinfer-alerts
  namespace: monitoring
spec:
  groups:
  - name: matrixinfer.rules
    rules:
    
    # High-level service alerts
    - alert: MatrixInferServiceDown
      expr: up{job="matrixinfer-models"} == 0
      for: 1m
      labels:
        severity: critical
        service: matrixinfer
      annotations:
        summary: "MatrixInfer service is down"
        description: "MatrixInfer service {{ $labels.instance }} has been down for more than 1 minute"
    
    - alert: HighInferenceLatency
      expr: histogram_quantile(0.95, sum(rate(matrixinfer_inference_duration_seconds_bucket[5m])) by (le, model_id)) > 2
      for: 5m
      labels:
        severity: warning
        service: matrixinfer
      annotations:
        summary: "High inference latency detected"
        description: "Model {{ $labels.model_id }} has P95 latency of {{ $value }}s for more than 5 minutes"
    
    - alert: HighErrorRate
      expr: sum(rate(matrixinfer_inference_errors_total[5m])) by (model_id) / sum(rate(matrixinfer_inference_requests_total[5m])) by (model_id) > 0.05
      for: 2m
      labels:
        severity: critical
        service: matrixinfer
      annotations:
        summary: "High error rate detected"
        description: "Model {{ $labels.model_id }} has error rate of {{ $value | humanizePercentage }} for more than 2 minutes"
    
    # Resource-based alerts
    - alert: ModelHighMemoryUsage
      expr: matrixinfer_model_memory_usage_bytes / 1024 / 1024 / 1024 > 8
      for: 10m
      labels:
        severity: warning
        service: matrixinfer
      annotations:
        summary: "Model using high memory"
        description: "Model {{ $labels.model_id }} is using {{ $value }}GB of memory"
    
    - alert: GPUHighUtilization
      expr: nvidia_gpu_utilization_percent > 90
      for: 15m
      labels:
        severity: warning
        service: matrixinfer
      annotations:
        summary: "GPU high utilization"
        description: "GPU {{ $labels.gpu_id }} utilization is {{ $value }}% for model {{ $labels.model_id }}"
    
    # Model-specific alerts
    - alert: ModelLoadFailure
      expr: increase(matrixinfer_model_load_failures_total[5m]) > 0
      for: 1m
      labels:
        severity: critical
        service: matrixinfer
      annotations:
        summary: "Model load failure"
        description: "Model {{ $labels.model_id }} failed to load {{ $value }} times in the last 5 minutes"
    
    - alert: ModelAccuracyDrop
      expr: model_accuracy_score < 0.85
      for: 10m
      labels:
        severity: warning
        service: matrixinfer
      annotations:
        summary: "Model accuracy drop detected"
        description: "Model {{ $labels.model_id }} accuracy dropped to {{ $value | humanizePercentage }}"
    
    # Autoscaling alerts
    - alert: AutoscalingNotWorking
      expr: kube_deployment_status_replicas{deployment=~".*-infer"} == kube_deployment_spec_replicas{deployment=~".*-infer"} and on(deployment) rate(matrixinfer_inference_requests_total[5m]) > 10
      for: 10m
      labels:
        severity: warning
        service: matrixinfer
      annotations:
        summary: "Autoscaling may not be working"
        description: "Deployment {{ $labels.deployment }} has high load but is not scaling"
```

## Distributed Tracing

### Jaeger Setup

**Jaeger Installation:**
```bash
# Install Jaeger operator
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/download/v1.42.0/jaeger-operator.yaml -n observability

# Create Jaeger instance
kubectl apply -f - <<EOF
apiVersion: jaegertracing.io/v1
kind: Jaeger
metadata:
  name: matrixinfer-jaeger
  namespace: observability
spec:
  strategy: production
  storage:
    type: elasticsearch
    elasticsearch:
      nodeCount: 3
      storage:
        size: 100Gi
      resources:
        requests:
          memory: "2Gi"
          cpu: "1"
        limits:
          memory: "4Gi"
          cpu: "2"
EOF
```

### Tracing Configuration

**OpenTelemetry Collector:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
data:
  config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
      jaeger:
        protocols:
          grpc:
            endpoint: 0.0.0.0:14250
          thrift_http:
            endpoint: 0.0.0.0:14268
    
    processors:
      batch:
        timeout: 1s
        send_batch_size: 1024
      resource:
        attributes:
        - key: service.name
          value: matrixinfer
          action: upsert
        - key: service.version
          from_attribute: version
          action: upsert
    
    exporters:
      jaeger:
        endpoint: matrixinfer-jaeger-collector:14250
        tls:
          insecure: true
      prometheus:
        endpoint: "0.0.0.0:8889"
        namespace: matrixinfer
        const_labels:
          service: matrixinfer
    
    service:
      pipelines:
        traces:
          receivers: [otlp, jaeger]
          processors: [batch, resource]
          exporters: [jaeger]
        metrics:
          receivers: [otlp]
          processors: [batch, resource]
          exporters: [prometheus]
```

## Log Management

### Loki Setup (Optional)

**Loki Configuration:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: loki-config
data:
  loki.yaml: |
    auth_enabled: false
    
    server:
      http_listen_port: 3100
    
    ingester:
      lifecycler:
        address: 127.0.0.1
        ring:
          kvstore:
            store: inmemory
          replication_factor: 1
        final_sleep: 0s
      chunk_idle_period: 5m
      chunk_retain_period: 30s
    
    schema_config:
      configs:
        - from: 2020-10-24
          store: boltdb
          object_store: filesystem
          schema: v11
          index:
            prefix: index_
            period: 168h
    
    storage_config:
      boltdb:
        directory: /loki/index
      filesystem:
        directory: /loki/chunks
    
    limits_config:
      enforce_metric_name: false
      reject_old_samples: true
      reject_old_samples_max_age: 168h
```

### Promtail Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: promtail-config
data:
  promtail.yaml: |
    server:
      http_listen_port: 9080
      grpc_listen_port: 0
    
    positions:
      filename: /tmp/positions.yaml
    
    clients:
      - url: http://loki:3100/loki/api/v1/push
    
    scrape_configs:
    - job_name: matrixinfer-logs
      kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ['default', 'production', 'staging']
      pipeline_stages:
      - docker: {}
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: matrixinfer.*
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
      - source_labels: [__meta_kubernetes_pod_label_model_id]
        target_label: model_id
```

## Health Checks and SLIs/SLOs

### Service Level Indicators (SLIs)

**Key SLIs for MatrixInfer:**
```yaml
slis:
  availability:
    description: "Percentage of successful inference requests"
    query: "sum(rate(matrixinfer_inference_requests_total{status!~'5..'}[5m])) / sum(rate(matrixinfer_inference_requests_total[5m]))"
    target: 99.9%
  
  latency:
    description: "95th percentile of inference request latency"
    query: "histogram_quantile(0.95, sum(rate(matrixinfer_inference_duration_seconds_bucket[5m])) by (le))"
    target: "< 1s"
  
  throughput:
    description: "Number of inference requests per second"
    query: "sum(rate(matrixinfer_inference_requests_total[5m]))"
    target: "> 100 req/s"
  
  error_rate:
    description: "Percentage of failed inference requests"
    query: "sum(rate(matrixinfer_inference_errors_total[5m])) / sum(rate(matrixinfer_inference_requests_total[5m]))"
    target: "< 0.1%"
```

### Service Level Objectives (SLOs)

**SLO Configuration:**
```yaml
apiVersion: sloth.slok.dev/v1
kind: PrometheusServiceLevel
metadata:
  name: matrixinfer-slo
  namespace: monitoring
spec:
  service: "matrixinfer"
  labels:
    team: "ml-ops"
  slos:
  - name: "inference-availability"
    objective: 99.9
    description: "99.9% of inference requests should be successful"
    sli:
      events:
        error_query: sum(rate(matrixinfer_inference_requests_total{status=~"5.."}[5m]))
        total_query: sum(rate(matrixinfer_inference_requests_total[5m]))
    alerting:
      name: MatrixInferHighErrorRate
      labels:
        severity: critical
      annotations:
        summary: "High error rate on MatrixInfer inference requests"
  
  - name: "inference-latency"
    objective: 95.0
    description: "95% of inference requests should complete within 1 second"
    sli:
      events:
        error_query: sum(rate(matrixinfer_inference_duration_seconds_bucket{le="1.0"}[5m]))
        total_query: sum(rate(matrixinfer_inference_duration_seconds_count[5m]))
    alerting:
      name: MatrixInferHighLatency
      labels:
        severity: warning
      annotations:
        summary: "High latency on MatrixInfer inference requests"
```

## Monitoring Best Practices

### Metric Naming Conventions

**Follow Prometheus naming conventions:**
- Use `_total` suffix for counters
- Use `_seconds` for time measurements
- Use `_bytes` for size measurements
- Include units in metric names
- Use consistent label names across metrics

### Dashboard Organization

**Dashboard Structure:**
1. **Overview Dashboard** - High-level service health
2. **Model-specific Dashboards** - Per-model metrics
3. **Infrastructure Dashboards** - Resource utilization
4. **Troubleshooting Dashboards** - Detailed debugging views

### Alert Fatigue Prevention

**Alert Guidelines:**
- Set appropriate thresholds based on SLOs
- Use different severity levels (critical, warning, info)
- Implement alert suppression during maintenance
- Group related alerts to reduce noise
- Include runbook links in alert annotations

## Troubleshooting Monitoring

### Common Issues

**Metrics Not Appearing:**
```bash
# Check if metrics endpoint is accessible
kubectl port-forward svc/my-model-server 8080:8080
curl http://localhost:8080/metrics

# Verify ServiceMonitor configuration
kubectl get servicemonitor -n monitoring
kubectl describe servicemonitor matrixinfer-metrics -n monitoring

# Check Prometheus targets
kubectl port-forward svc/prometheus-operated 9090:9090
# Visit http://localhost:9090/targets
```

**High Cardinality Issues:**
```bash
# Check metric cardinality
curl -s http://prometheus:9090/api/v1/label/__name__/values | jq '.data[]' | grep matrixinfer | wc -l

# Identify high cardinality metrics
curl -s 'http://prometheus:9090/api/v1/query?query={__name__=~"matrixinfer.*"}' | jq '.data.result | length'
```

## Related Documentation

- [Best Practices](../best-practices/) - Production deployment guidelines
- [Advanced Features](../advanced/) - Advanced monitoring configurations
- [Troubleshooting](../troubleshooting.md) - Common monitoring issues
- [Examples](../examples/) - Monitoring setup examples