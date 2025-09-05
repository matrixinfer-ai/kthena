# Proposal: Gateway Observability

## Goals

- **Metrics definition**: Define all the metrics about the running status of the gateway, LLM requests processing, and performance optimization.
- **Access log format**: Define the format of the access log to intuitively display the key results of the request processing.
- **Debug interface**: Define the debug interface to observe the gateway's internal status.

## 1. Introduction


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
  - Labels: Same as above
  - Buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60]

**AI-Specific Token Metrics**
- `infer_gateway_input_tokens_total{model="<model_name>",endpoint="<path>"}` (Counter)
  - Total input tokens processed
  - Labels: model name, endpoint path
  
- `infer_gateway_output_tokens_total{model="<model_name>",endpoint="<path>"}` (Counter)
  - Total output tokens generated
  - Labels: model name, endpoint path

- `infer_gateway_token_duration_seconds{model="<model_name>",token_type="input|output"}` (Histogram)
  - Token processing time distribution
  - Labels: model name, token type (input/output)
  - Buckets: [10, 50, 100, 500, 1000, 2000, 5000, 10000]

**Filter Processing Metrics**
- `infer_gateway_filter_duration_seconds{filter="<filter_name>"}` (Histogram)
  - Processing time per filter
  - Labels: filter name
  - Buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5]

**Rate Limiting Metrics**  
- `infer_gateway_rate_limit_exceeded_total{model="<model_name>",limit_type="input_tokens|output_tokens|requests",endpoint="<path>"}` (Counter)
  - Number of requests rejected due to rate limiting
  - Labels: model name, limit type, endpoint path
  
- `infer_gateway_rate_limit_utilization{model="<model_name>",limit_type="input_tokens|output_tokens|requests"}` (Gauge)
  - Current rate limit utilization percentage (0-1)
  - Labels: model name, limit type

**Resource Access Metrics**
- `infer_gateway_model_route_requests_total{route="<route_name>",model="<model_name>"}` (Counter)
  - Number of ModelRoute accesses
  - Labels: route name, model name
  
- `infer_gateway_model_server_requests_total{server="<server_name>",model="<model_name>",status_code="<code>"}` (Counter)
  - Number of ModelServer backend accesses
  - Labels: server name, model name, status code

**Connection and Scheduling Metrics**
- `infer_gateway_active_connections{model="<model_name>"}` (Gauge)
  - Current number of active connections per model
  - Labels: model name
  
- `infer_gateway_queue_size{model="<model_name>"}` (Gauge)
  - Current queue size for pending requests
  - Labels: model name
  
- `infer_gateway_queue_duration_seconds{model="<model_name>"}` (Histogram)
  - Time requests spend in queue before processing
  - Labels: model name
  - Buckets: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5]

**Error Metrics**
- `infer_gateway_request_errors_total{model="<model_name>",error_type="<type>",status_code="<code>",endpoint="<path>"}` (Counter)
  - Request errors by type and status code
  - Labels: model name, error type (validation, timeout, internal, rate_limit, etc.), status code, endpoint path

All metrics are exposed at the `/metrics` endpoint in Prometheus format. The metrics provide comprehensive visibility into:

**Key Observability Dimensions**
- **Request Processing**: HTTP request patterns, latency, and success rates
- **AI Workload Specific**: Token consumption, model performance, and inference costs  
- **Resource Utilization**: Connection pools, queuing, and rate limiting
- **Error Tracking**: Detailed error categorization and failure patterns

**Operational Use Cases**
1. **Performance Monitoring**: Track request latency, throughput, and model response times
2. **Capacity Planning**: Monitor token consumption, queue depths, and resource utilization
3. **Cost Analysis**: Analyze token usage patterns and model efficiency
4. **Troubleshooting**: Correlate errors with request patterns, models, and endpoints
5. **Rate Limiting**: Monitor rate limit effectiveness and token consumption patterns
6. **Service Health**: Track success rates and identify performance bottlenecks

**Integration with Monitoring Stack**
- **Prometheus**: Metrics collection and storage
- **Grafana**: Visualization and dashboards
- **Alert Manager**: Alerting based on metrics thresholds



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


## 3. Conclusion


