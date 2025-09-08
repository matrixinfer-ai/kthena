# Proposal: Gateway Observability

## Goals

- **Metrics definition**: Define all the metrics about the running status of the gateway, LLM requests processing, and performance optimization.
- **Access log format**: Define the format of the access log to intuitively display the key results of the request processing.
- **Debug interface**: Define the debug interface to observe the gateway's internal status.

## 1. Introduction

The infer-gateway serves as a critical component in the AI inference system, managing model routing, request scheduling, and resource allocation. Effective observability is essential for:

1. **Performance Monitoring**: Track request latencies, token processing rates, and resource utilization to ensure optimal system performance.

2. **Troubleshooting**: Quickly identify and diagnose issues through detailed metrics, logs, and debug information.

3. **Capacity Planning**: Analyze usage patterns and resource consumption to make informed scaling decisions.

4. **Cost Optimization**: Monitor token usage and model utilization to optimize resource allocation and costs.

This proposal outlines a comprehensive observability framework that combines:

- **Prometheus Metrics**: Detailed metrics covering HTTP requests, token processing, rate limiting, and errors
- **Structured Access Logs**: Request lifecycle logging with timing breakdowns and routing decisions
- **Debug Endpoints**: Internal state exposure for troubleshooting and diagnostics

The framework is designed to integrate with standard observability tools while providing AI-specific insights needed for managing inference workloads at scale.


## 2. Technical Implementation

### 2.1 Metrics definition

The gateway exposes the following metrics to monitor request processing:

#### Design Principles

**Essential Labels**: Include key dimensions for effective monitoring and debugging:
- `method`: HTTP method (GET, POST) - useful for differentiating request types
- `endpoint`: API path (/v1/chat/completions, /v1/completions) - track different API usage
- `status_code`: HTTP response status code (200, 400, 500) - monitor success/failure patterns
- `model`: AI model name - essential for AI workload monitoring
- `error_type`: Specific error categories for detailed troubleshooting

**Label Cardinality Management**: Keep label values bounded to avoid high cardinality issues:
- Limited set of endpoints and methods
- Standard HTTP status codes
- Controlled model catalog
- Predefined error types

#### Request Processing Metrics

**HTTP Request Metrics**
- `infer_gateway_requests_total{method="<method>",endpoint="<path>",status_code="<code>",model="<model_name>"}` (Counter)
  - Total number of HTTP requests processed by the gateway
  - Labels: 
    - `method`: HTTP method (GET, POST, etc.)
    - `endpoint`: Request path (/v1/chat/completions, /v1/completions, etc.)
    - `status_code`: HTTP response status code (200, 400, 500, etc.)
    - `model`: AI model name

- `infer_gateway_request_duration_seconds{method="<method>",endpoint="<path>",status_code="<code>",model="<model_name>"}` (Histogram)
  - Request processing latency distribution
  - Labels:
    - `method`: HTTP method (GET, POST, etc.)
    - `endpoint`: Request path (/v1/chat/completions, /v1/completions, etc.)
    - `status_code`: HTTP response status code (200, 400, 500, etc.)
    - `model`: AI model name
  - Buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60]

**AI-Specific Token Metrics**
- `infer_gateway_tokens_total{model="<model_name>",endpoint="<path>",token_type="input|output"}` (Counter)
  - Total tokens processed/generated
  - Labels:
    - `model`: AI model name
    - `endpoint`: Request path (/v1/chat/completions, /v1/completions, etc.)
    - `token_type`: Token type ("input" for processed tokens, "output" for generated tokens)

- `infer_gateway_token_duration_seconds{model="<model_name>",token_type="input|output"}` (Histogram)
  - Token processing time distribution
  - Labels:
    - `model`: AI model name
    - `token_type`: Token type ("input" for processed tokens, "output" for generated tokens)
  - Buckets: [10, 50, 100, 500, 1000, 2000, 5000, 10000]

**Filter Processing Metrics**
- `infer_gateway_filter_duration_seconds{filter="<filter_name>"}` (Histogram)
  - Processing time per filter
  - Labels:
    - `filter`: Filter name
  - Buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5]

**Rate Limiting Metrics**  
- `infer_gateway_rate_limit_exceeded_total{model="<model_name>",limit_type="input_tokens|output_tokens|requests",endpoint="<path>"}` (Counter)
  - Number of requests rejected due to rate limiting
  - Labels:
    - `model`: AI model name
    - `limit_type`: Type of rate limit (input_tokens, output_tokens, requests)
    - `endpoint`: Request path (/v1/chat/completions, /v1/completions, etc.)
  
- `infer_gateway_rate_limit_utilization{model="<model_name>",limit_type="input_tokens|output_tokens|requests"}` (Gauge)
  - Current rate limit utilization percentage (0-1)
  - Labels:
    - `model`: AI model name
    - `limit_type`: Type of rate limit (input_tokens, output_tokens, requests)

**Resource Access Metrics**
- `infer_gateway_model_route_requests_total{route="<route_name>",model="<model_name>"}` (Counter)
  - Number of ModelRoute accesses
  - Labels:
    - `route`: ModelRoute name
    - `model`: AI model name
  
- `infer_gateway_model_server_requests_total{server="<server_name>",model="<model_name>",status_code="<code>"}` (Counter)
  - Number of ModelServer backend accesses
  - Labels:
    - `server`: ModelServer name
    - `model`: AI model name
    - `status_code`: HTTP response status code (200, 400, 500, etc.)

**Connection and Scheduling Metrics**
- `infer_gateway_active_connections{model="<model_name>"}` (Gauge)
  - Current number of active connections per model
  - Labels:
    - `model`: AI model name
  
- `infer_gateway_queue_size{model="<model_name>"}` (Gauge)
  - Current queue size for pending requests
  - Labels:
    - `model`: AI model name
  
- `infer_gateway_queue_duration_seconds{model="<model_name>"}` (Histogram)
  - Time requests spend in queue before processing
  - Labels:
    - `model`: AI model name
  - Buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5]

**Error Metrics**
- `infer_gateway_request_errors_total{model="<model_name>",error_type="<type>",status_code="<code>",endpoint="<path>"}` (Counter)
  - Request errors by type and status code
  - Labels:
    - `model`: AI model name
    - `error_type`: Type of error (validation, timeout, internal, rate_limit, etc.)
    - `status_code`: HTTP response status code (200, 400, 500, etc.)
    - `endpoint`: Request path (/v1/chat/completions, /v1/completions, etc.)

All metrics are exposed at the `/metrics` endpoint in Prometheus format. The metrics provide comprehensive visibility into:

**Key Observability Dimensions**
- **Request Processing**: HTTP request patterns, latency, and success rates
- **AI Workload Specific**: Token consumption, model performance, and inference costs  
- **Resource Utilization**: Connection pools, queuing, and rate limiting
- **Error Tracking**: Detailed error categorization and failure patterns


### 2.2 Access log format

The gateway generates structured access logs for each request, following Envoy's access log format with AI-specific extensions to track model routing and processing stages.

#### Log Format

**Default Format (JSON)**
```json
{
  "timestamp": "2024-01-15T10:30:45.123Z",
  "method": "POST",
  "path": "/v1/chat/completions",
  "protocol": "HTTP/1.1",
  "status_code": 200,
  
  // AI-specific routing information
  "model_name": "llama2-7b",
  "model_route": "llama2-route-v1",
  "model_server": "llama2-server",
  "selected_pod": "llama2-deployment-5f7b8c9d-xk2p4",
  
  // Token information
  "input_tokens": 150,
  "output_tokens": 75,
  
  // Timing breakdown (in milliseconds)
  "duration": {
    "total": 2350,                // Total request processing time
    "request_processing": 45,     // Gateway request processing overhead
    "upstream_processing": 2180,     // Model inference time on backend pod
    "response_processing": 5      // Response processing time
  },
  
  // Error information (if applicable)
  "error": {
    "type": "timeout",
    "message": "Model inference timeout after 30s"
  }
}
```

#### Text Format (Alternative)
For environments preferring text logs, a structured text format is also supported:
```
[2024-01-15T10:30:45.123Z] "POST /v1/chat/completions HTTP/1.1" 200 2350ms
model=llama2-7b route=llama2-route-v1 server=llama2-server pod=llama2-deployment-5f7b8c9d-xk2p4 
tokens=150/75 timings=45+2180+5ms
```

#### Key Fields Explanation

**Standard HTTP Fields** (Following Envoy format):
- `timestamp`: Request start time in ISO 8601 format
- `method`, `path`, `protocol`: Standard HTTP request information
- `status_code`: HTTP response status code

**AI-Specific Routing Fields**:
- `model_name`: The AI model requested in the request body
- `model_route`: Which ModelRoute CR was matched for this request
- `model_server`: Which ModelServer CR was selected for routing
- `selected_pod`: The specific pod that processed the inference request

**Token Tracking**:
- `input_tokens`: Number of tokens in the input prompt
- `output_tokens`: Number of tokens in the response

**Detailed Timing Breakdown** (all times in milliseconds):
- `total`: End-to-end request processing time
- `request_processing`: Time spent in gateway request processing (parsing, routing, etc.)
- `upstream_processing`: Actual model inference time on the backend pod
- `response_processing`: Time spent processing and formatting the response

**Error Information**:
- `error`: Detailed error information for failed requests (type and message)

Logs are written to stdout by default and can be configured to write to files or external log collectors.

### 2.3 Debug interface

The gateway exposes a debug interface at `/debug` to help operators inspect internal state and troubleshoot issues. This interface provides access to the gateway's datastore information, allowing examination of ModelRoutes, ModelServers, and Pod details.

#### Debug Endpoints

The following debug endpoints are available:

**List Resources**
- `/debug/modelroutes` - List all ModelRoute configurations
- `/debug/modelservers` - List all ModelServer configurations 
- `/debug/pods` - List all Pod information

**Get Specific Resource**
- `/debug/modelroute/{name}` - Get details of a specific ModelRoute
- `/debug/modelserver/{namespace}/{name}` - Get details of a specific ModelServer
- `/debug/pod/{namespace}/{name}` - Get details of a specific Pod

#### Example Responses

**GET /debug/modelroutes**
```json
{
  "modelroutes": [
    {
      "name": "llama2-route",
      "namespace": "default",
      "spec": {
        "modelName": "llama2-7b",
        "loraAdapters": ["lora-adapter-1", "lora-adapter-2"],
        "rules": [
          {
            "name": "default-rule",
            "modelMatch": {
              "body": {
                "model": "llama2-7b"
              }
            },
            "targetModels": [
              {
                "modelServer": {
                  "name": "llama2-server",
                  "namespace": "default"
                },
                "weight": 100
              }
            ]
          }
        ],
        "rateLimit": {
          "local": {
            "inputTokensPerSecond": 1000,
            "outputTokensPerSecond": 500
          }
        }
      }
    }
  ]
}
```

**GET /debug/modelroute/llama2-route**
```json
{
  "name": "llama2-route",
  "namespace": "default",
  "spec": {
    "modelName": "llama2-7b",
    "loraAdapters": ["lora-adapter-1", "lora-adapter-2"],
    "rules": [
      {
        "name": "default-rule",
        "modelMatch": {
          "headers": {
            "authorization": {
              "prefix": "Bearer "
            }
          },
          "uri": {
            "prefix": "/v1/chat/completions"
          },
          "body": {
            "model": "llama2-7b"
          }
        },
        "targetModels": [
          {
            "modelServer": {
              "name": "llama2-server",
              "namespace": "default"
            },
            "weight": 100
          }
        ]
      }
    ],
    "rateLimit": {
      "local": {
        "inputTokensPerSecond": 1000,
        "outputTokensPerSecond": 500
      }
    }
  },
  "routeInfo": {
    "model": "llama2-7b",
    "loras": ["lora-adapter-1", "lora-adapter-2"]
  }
}
```

**GET /debug/modelservers**
```json
{
  "modelservers": [
    {
      "name": "llama2-server",
      "namespace": "default",
      "spec": {
        "model": "llama2-7b-chat",
        "inferenceEngine": "vLLM",
        "workloadSelector": {
          "matchLabels": {
            "app": "llama2",
            "version": "v1"
          }
        },
        "workloadPort": {
          "port": 8000,
          "protocol": "http"
        },
        "trafficPolicy": {
          "loadBalancer": {
            "simple": "LEAST_REQUEST"
          }
        }
      }
    }
  ]
}
```

**GET /debug/modelserver/default/llama2-server**
```json
{
  "name": "llama2-server",
  "namespace": "default",
  "spec": {
    "model": "llama2-7b-chat",
    "inferenceEngine": "vLLM",
    "workloadSelector": {
      "matchLabels": {
        "app": "llama2",
        "version": "v1"
      },
      "pdGroup": {
        "groupKey": "group-id",
        "prefillLabels": {
          "role": "prefill"
        },
        "decodeLabels": {
          "role": "decode"
        }
      }
    },
    "workloadPort": {
      "port": 8000,
      "protocol": "http"
    },
    "trafficPolicy": {
      "loadBalancer": {
        "simple": "LEAST_REQUEST"
      }
    },
    "kvConnector": {
      "type": "redis",
      "redis": {
        "address": "redis.default.svc.cluster.local:6379",
        "db": 0
      }
    }
  },
  "associatedPods": [
    "default/llama2-deployment-5f7b8c9d-xk2p4",
    "default/llama2-deployment-5f7b8c9d-mn8q7"
  ]
}
```

**GET /debug/pods**
```json
{
  "pods": [
    {
      "name": "llama2-deployment-5f7b8c9d-xk2p4",
      "namespace": "default",
      "podIP": "10.244.2.20",
      "nodeName": "worker-node-1",
      "phase": "Running",
      "engine": "vLLM",
      "metrics": {
        "gpuCacheUsage": 0.75,
        "requestWaitingNum": 3,
        "requestRunningNum": 2,
        "tpot": 0.045,
        "ttft": 1.2
      },
      "models": ["llama2-7b", "lora-adapter-1", "lora-adapter-2"],
      "modelServers": ["default/llama2-server"]
    }
  ]
}
```

**GET /debug/pod/default/llama2-deployment-5f7b8c9d-xk2p4**
```json
{
  "name": "llama2-deployment-5f7b8c9d-xk2p4",
  "namespace": "default",
  "podInfo": {
    "podIP": "10.244.2.20",
    "nodeName": "worker-node-1",
    "phase": "Running",
    "startTime": "2024-01-15T10:00:00Z",
    "labels": {
      "app": "llama2",
      "version": "v1",
      "role": "inference"
    }
  },
  "engine": "vLLM",
  "metrics": {
    "gpuCacheUsage": 0.75,
    "requestWaitingNum": 3,
    "requestRunningNum": 2,
    "tpot": 0.045,
    "ttft": 1.2,
    "timeToFirstTokenHistogram": {
      "buckets": [
        {"upperBound": 0.5, "cumulativeCount": 10},
        {"upperBound": 1.0, "cumulativeCount": 25},
        {"upperBound": 2.0, "cumulativeCount": 45}
      ],
      "sampleCount": 50,
      "sampleSum": 62.5
    },
    "timePerOutputTokenHistogram": {
      "buckets": [
        {"upperBound": 0.01, "cumulativeCount": 5},
        {"upperBound": 0.05, "cumulativeCount": 35},
        {"upperBound": 0.1, "cumulativeCount": 48}
      ],
      "sampleCount": 50,
      "sampleSum": 2.25
    }
  },
  "models": ["llama2-7b", "lora-adapter-1", "lora-adapter-2"],
  "modelServers": ["default/llama2-server"],
  "containers": [
    {
      "name": "inference-server",
      "image": "vllm/vllm-openai:latest",
      "ports": [
        {
          "containerPort": 8000,
          "protocol": "TCP"
        }
      ],
      "resources": {
        "requests": {
          "nvidia.com/gpu": "1",
          "memory": "16Gi",
          "cpu": "4"
        }
      }
    }
  ]
}
```

## 3. Conclusion

This proposal defines a comprehensive observability framework for the infer-gateway that provides:

1. **Metrics**: Prometheus-compatible metrics covering HTTP requests, AI-specific token processing, rate limiting, and error tracking with carefully chosen labels to avoid high cardinality issues.

2. **Access Logs**: Structured logging format that captures the complete AI inference request lifecycle, including model routing decisions, pod selection, and detailed timing breakdowns for performance analysis.

3. **Debug Interface**: RESTful debug endpoints that expose internal datastore state for troubleshooting, including ModelRoute configurations, ModelServer details, and Pod metrics.

The framework enables operators to:
- Monitor system performance and health in real-time
- Troubleshoot routing and scheduling issues effectively  
- Analyze token consumption patterns for cost optimization
- Track rate limiting effectiveness and resource utilization
- Correlate metrics, logs, and debug information for comprehensive observability

All components are designed to integrate seamlessly with standard observability tools (Prometheus, Grafana, log collectors) while providing AI-specific insights essential for managing inference workloads.
