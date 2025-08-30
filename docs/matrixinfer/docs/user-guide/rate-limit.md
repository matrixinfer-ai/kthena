# Gateway Rate Limiting

Unlike traditional microservices that use request count or connection-based rate limiting, AI inference scenarios require **token-based rate limiting**. This is because AI requests can vary dramatically in computational cost - a single request with 10,000 tokens consumes far more GPU resources than 100 requests with 10 tokens each. Token-based limits ensure fair resource allocation based on actual computational consumption rather than simple request counts.

## Overview

MatrixInfer Gateway provides powerful rate-limiting capabilities to control the traffic flow to your backend models. This is essential for preventing service overload, managing costs, and ensuring fair usage. Rate limiting is configured directly within the **ModelRoute** Custom Resource (CR).

The gateway supports two main types of rate limiting:
- **Local Rate Limiting**: Enforces limits on a per-gateway-instance basis. It\'s simple to configure and effective for basic load protection.
- **Global Rate Limiting**: Enforces a shared limit across all gateway instances, using a central store like Redis. This is ideal for providing consistent limits in a scaled-out environment.

Limits are based on the number of input/output tokens over a specific time window (second, minute, hour, day, or month).

## Preparation

Before diving into the rate-limiting configurations, let's set up the environment. All the configuration examples in this document can be found in the [examples/infer-gateway](https://github.com/matrixinfer-ai/matrixinfer/tree/main/examples/infer-gateway) directory of the MatrixInfer repository.

### Prerequisites

- A running Kubernetes cluster with MatrixInfer installed.
- For global rate limiting, a running Redis instance is required. You can deploy one using the [redis-standalone.yaml](../../../../examples/redis/redis-standalone.yaml) example.
- Basic understanding of gateway CRDs (ModelServer and ModelRoute).

### Getting Started

1.  Deploy a mock LLM inference engine, such as [LLM-Mock-ds1.5b.yaml](../../../../examples/infer-gateway/LLM-Mock-ds1.5b.yaml), if you don't have a real GPU/NPU environment.
2.  Deploy the corresponding ModelServer, [ModelServer-ds1.5b.yaml](../../../../examples/infer-gateway/ModelServer-ds1.5b.yaml).
3.  All rate-limiting examples in this guide use this mock service.

## Rate Limiting Scenarios

### 1. Local Rate Limiting

**Scenario**: Protect a model from being overwhelmed by limiting the number of tokens it can process per minute from each gateway pod.

**Traffic Processing**: The gateway inspects the number of tokens in each request. If the cumulative number of input or output tokens within a minute exceeds the defined limits, the gateway will reject the request with an `HTTP 429 Too Many Requests` error. This limit is tracked independently by each gateway pod.

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: deepseek-rate-limit
  namespace: default
spec:
  modelName: "deepseek-r1-with-rate-limit"
  rules:
  - name: "default"
    targetModels:
    - modelServerName: "deepseek-r1-1-5b"
  # Rate limit configuration:
  # - 10000 input tokens per minute
  # - 5000 output tokens per minute
  # This configuration applies to all rules in this ModelRoute
  rateLimit:
    inputTokensPerUnit: 10000
    outputTokensPerUnit: 5000
    unit: minute
```

**Flow Description**:
1.  A request for the model `deepseek-r1-with-rate-limit` arrives at the gateway.
2.  The gateway checks its local counters for the token limits defined in the `rateLimit` policy.
3.  If the `inputTokensPerUnit` or `outputTokensPerUnit` limit is not exceeded, the request is forwarded to the `deepseek-r1-1-5b` ModelServer.
4.  If either limit is exceeded, the gateway immediately returns an `HTTP 429` status code.

**Try it out**:
To test this, you can set a low limit (e.g., `inputTokensPerUnit: 10`) and send multiple requests.

```bash
export MODEL="deepseek-r1-with-rate-limit"

# This request should succeed
curl http://$GATEWAY_IP/v1/completions \
    -H "Content-Type: application/json" \
    -d "{
        \"model\": \"$MODEL\",
        \"prompt\": \"San Francisco is a\",
        \"temperature\": 0
    }"

# Subsequent requests within the same minute may fail if the token limit is breached
curl http://$GATEWAY_IP/v1/completions \
    -H "Content-Type: application/json" \
    -d "{
        \"model\": \"$MODEL\",
        \"prompt\": \"Another request to test the limit\",
        \"temperature\": 0
    }"
# Expected output for rejected request:
# {
#   "error": "rate limit exceeded"
# }
```

### 2. Global Rate Limiting

**Scenario**: Enforce a consistent, cluster-wide rate limit for a specific model route, ensuring that the total traffic from all users and all gateway pods does not exceed a global threshold.

**Traffic Processing**: This works similarly to local rate limiting, but instead of using local in-memory counters, the gateway uses a Redis instance to share and synchronize the token counts across all gateway pods. This ensures the rate limit is applied globally and accurately, regardless of how many gateway replicas are running.

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: deepseek-global-rate-limit
  namespace: default
spec:
  modelName: "deepseek-r1-with-global-rate-limit"
  rules:
  - name: "default"
    targetModels:
    - modelServerName: "deepseek-r1-1-5b"
  # Global rate limit configuration:
  # - 10 input tokens per minute
  # - 5000 output tokens per minute
  rateLimit:
    inputTokensPerUnit: 10
    outputTokensPerUnit: 5000
    unit: minute
    global:
      redis:
        address: "redis-server.matrixinfer-system.svc.cluster.local:6379"
```

**Flow Description**:
1.  A request for the model `deepseek-r1-with-global-rate-limit` arrives at any gateway pod.
2.  The gateway connects to the Redis server specified in the `global.redis.address` field.
3.  It atomically checks and increments the token counters stored in Redis for this route.
4.  If the global limit is not exceeded, the request is forwarded.
5.  If the limit is exceeded, the gateway returns an `HTTP 429` error. All other gateway pods will now also enforce this limit until the time window resets.

**NOTE**: Before applying this configuration, ensure you have a Redis service running and accessible at the specified address (`redis-server.matrixinfer-system.svc.cluster.local:6379`). You can use the provided [redis-standalone.yaml](../../../../examples/redis/redis-standalone.yaml) to deploy one. And make sure you have deployed multiple gateway pods.

**Try it out**:
The test process is similar to local rate limiting. Even if your requests are handled by different gateway pods, the global limit will be consistently enforced.

```bash
export MODEL="deepseek-r1-with-global-rate-limit"

# Send requests from multiple terminals or in a loop
for i in $(seq 1 5);
do
    curl http://$GATEWAY_IP/v1/completions \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$MODEL\",
            \"prompt\": \"This is a test prompt\",
            \"temperature\": 0
        }" &
done
# You will observe that only a certain number of requests succeed
# before others start failing with a 429 error, regardless of
# which gateway pod they hit.
```

By leveraging local and global rate limiting, MatrixInfer gives you fine-grained control over your AI service traffic, enabling robust, scalable, and cost-effective model deployments.
