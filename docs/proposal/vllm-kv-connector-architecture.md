# Proposal: Enhanced Architecture for vLLM KV Connectors

## 1. Introduction

This document proposes an enhanced architecture for handling KV Connectors in the MatrixInfer gateway. The current implementation for PD (Prefill-Decode) disaggregated routing is based on a simple two-step HTTP request process, which is insufficient to support the diverse KV cache transfer mechanisms available in vLLM.

Based on research of vLLM's actual implementation, the primary KV cache connectors include `LMCacheConnector` for distributed KV cache management with CPU/disk offloading capabilities, and `NIXLConnector` for high-performance distributed in-memory communication. Additionally, vLLM can integrate with `MooncakeStore`, Kimi's (Moonshot AI) KVCache-centric disaggregated architecture as described in their research paper "Mooncake: A KVCache-centric Disaggregated Architecture for LLM Serving".

According to my research, vLLM basically support kv transfer from prefill instance to decode instance by NVLink, RoCE, InfiniBand, etc. Or the kv cache can be first offloaded to a CPU or Disk, then decode instance can read the kv cache from a external cache store as needed. `NixlConnector` is using P2P, while `LMCacheConnector` and `MooncakeStore` actually can act as an external cache store, which can exploit cpu memory or disk.

The proposed architecture introduces a comprehensive, pluggable design that can accommodate these actual connector types while providing proper lifecycle management, error handling, and observability.

[vLLM](./images/vllm-v1-kvconnector.png)

## 2. Current Architecture Analysis

### 2.1. Existing Implementation

The current routing logic in `pkg/infer-gateway/router/router.go` implements PD disaggregation as follows:

```go
// Current simplified flow in proxyModelEndpoint()
1. Build prefill request (remove streaming)
2. Send prefill request to prefill pod (fire-and-forget)
3. Build decode request (with streaming support)  
4. Send decode request to decode pod (return response)
```

### 2.2. Limitations of Current Approach

**Architectural Issues:**
- **Implicit KV Transfer**: Assumes KV cache is transferred automatically between prefill and decode pods
- **Tight Coupling**: Routing logic is hardcoded to HTTP-based sequential operations
- **No Connector Awareness**: No mechanism to beaware of the underlying kv connector

**Connector-Specific Limitations:**
- **LMCache Integration**: No support for vLLM's LMCache system for distributed KV cache management with CPU/disk offloading
- **NIXL Integration**: No support for vLLM's NIXL (NVIDIA Inference Xfer Library) for high-performance distributed in-memory communication
- **MooncakeStore Integration**: No support for integration with Kimi's (Moonshot AI) MooncakeStore KVCache-centric disaggregated architecture

## 3. Implemented Architecture

### 3.1. Core Design Principles

**Separation of Concerns**: Separate KV cache operations from routing logic
**Pluggability**: Support multiple connector implementations with unified interface  
**Configurability**: Allow per-model connector configuration via CRDs
**Resilience**: Comprehensive error handling, retries, and graceful degradation

### 3.2. KVConnector Interface Design

The `KVConnector` interface in `pkg/infer-gateway/connectors/interface.go` is the core of the pluggable design. It defines a single method, `Proxy`, that encapsulates the entire prefill-decode logic for a given connector.

```go
// from: pkg/infer-gateway/connectors/interface.go
package connectors

import (
	"github.com/gin-gonic/gin"
)

// KVConnector is the main interface for KV cache operations
type KVConnector interface {
	// Name returns the connector type name
	Name() string

	// Proxy executes the complete prefill-decode flow with KV cache coordination.
	// It takes the gin context, the parsed request body, and the addresses for the
	// prefill and decode pods.
	Proxy(c *gin.Context, reqBody map[string]interface{}, prefillAddr, decodeAddr string) error
}
```

### 3.3. Connector Implementations

#### 3.3.1 HTTPConnector (Default)
This is the default connector and is used for `http`, `lmcache`, and `mooncake` connector types. It implements a basic two-step prefill-decode flow:
1.  It constructs and sends a "prefill" request to the prefill pod. This is a fire-and-forget operation; the response is not processed. The prefill request is modified to have `max_tokens: 1` and to have streaming disabled.
2.  It then constructs and sends a "decode" request to the decode pod, streaming the response back to the client.

This connector assumes that the KV cache is transferred implicitly between the prefill and decode pods, for example via a shared storage mechanism like a distributed filesystem, which is what `LMCache` and `Mooncake` can be configured to use.

#### 3.3.2 NIXLConnector
This connector is used for the `nixl` connector type and implements a more coordinated KV cache transfer suitable for high-performance, in-memory transfers using vLLM's NIXL library. The flow is as follows:
1.  It constructs a prefill request with NIXL-specific `kv_transfer_params` and sends it to the prefill pod.
2.  It waits for the prefill response and parses it to extract the resulting `kv_transfer_params`. This response contains the necessary information (e.g., remote block IDs) for the decode pod to access the KV cache.
3.  It constructs a decode request, injecting the `kv_transfer_params` received from the prefill pod.
4.  It sends the decode request to the decode pod, which then uses the KV cache from the prefill step.

This implementation actively manages the KV cache transfer between the two steps.

### 3.4. Router Integration

The `Router` integrates with the KV connector system via a `connectorFactory`. When a request requires PD disaggregated routing, the router determines the correct connector type from the `ModelServer` CRD, retrieves the connector from the factory, and calls its `Proxy` method.

```go
// from: pkg/infer-gateway/router/router.go

// proxyToPDDisaggregated handles PD disaggregated routing using KV connectors
func (r *Router) proxyToPDDisaggregated(
	c *gin.Context,
	req *http.Request,
	ctx *framework.Context,
	kvConnector connectors.KVConnector,
	modelRequest ModelRequest,
	port int32,
) error {
	// Try multiple prefill/decode pairs
	maxRetry := len(ctx.DecodePods)
	if len(ctx.PrefillPods) < maxRetry {
		maxRetry = len(ctx.PrefillPods)
	}

	for i := 0; i < maxRetry; i++ {
		if ctx.PrefillPods[i] == nil || ctx.DecodePods[i] == nil {
			continue
		}

		// Build addresses for prefill and decode pods
		prefillAddr := fmt.Sprintf("%s:%d", ctx.PrefillPods[i].Pod.Status.PodIP, port)
		decodeAddr := fmt.Sprintf("%s:%d", ctx.DecodePods[i].Pod.Status.PodIP, port)

		klog.V(4).Infof("Attempting PD disaggregated request: prefill=%s, decode=%s", prefillAddr, decodeAddr)

		// The router calls the Proxy method on the selected connector, passing the gin context,
		// the parsed request body, and the pod addresses.
		if err := kvConnector.Proxy(c, modelRequest, prefillAddr, decodeAddr); err != nil {
			klog.Errorf("proxy failed for prefill pod %s, decode pod %s: %v",
				ctx.PrefillPods[i].Pod.Name, ctx.DecodePods[i].Pod.Name, err)
			continue
		}

		// Record successful operation in cache
		r.scheduler.RunPostHooks(ctx, i)

		klog.V(4).Infof("kv connector run successful for prefill pod %s, decode pod %s",
			ctx.PrefillPods[i].Pod.Name, ctx.DecodePods[i].Pod.Name)

		return nil
	}

	c.AbortWithStatusJSON(http.StatusInternalServerError, "all prefill/decode attempts failed")
	return fmt.Errorf("all prefill/decode attempts failed")
}
```

### 3.5. ModelServer CRD Extensions

We extend the `ModelServer` CRD to support KV connector configuration:

```go
// ModelServerSpec defines the desired state of ModelServer  
type ModelServerSpec struct {
	// ... existing fields
	
	// KVConnector specifies the KV connector configuration for PD disaggregated routing
	// +optional
	KVConnector *KVConnectorSpec `json:"kvConnector,omitempty"`
}

// KVConnectorSpec defines KV connector configuration
type KVConnectorSpec struct {
	// Type specifies the connector type
	// +kubebuilder:validation:Enum=http;lmcache;nixl;mooncake
	Type string `json:"type"`
}
```

## 4. Configuration Examples

### HTTP Connector (Default)
```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer  
metadata:
  name: llama2-7b
spec:
  model: "llama2-7b"
  workloadSelector:
    matchLabels:
      app: llama2-7b
    pdGroup:
      groupKey: "group"
      prefillLabels:
        role: "prefill"
      decodeLabels:  
        role: "decode"
  kvConnector:
    type: "http"
```

### HTTP Connector with MooncakeStore
```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer  
metadata:
  name: llama2-13b-mooncake
spec:
  model: "llama2-13b"
  workloadSelector:
    matchLabels:
      app: llama2-13b
    pdGroup:
      groupKey: "group"
      prefillLabels:
        role: "prefill"
      decodeLabels:  
        role: "decode"
  kvConnector:
    type: "mooncake"
```

### NIXL Connector
```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1  
kind: ModelServer
metadata:
  name: llama2-405b
spec:
  model: "llama2-405b"
  workloadSelector:
    matchLabels:
      app: llama2-405b
    pdGroup:
      groupKey: "group"
      prefillLabels:
        role: "prefill"
      decodeLabels:
        role: "decode"
  kvConnector:
    type: "nixl"
```

### LMCache Connector
```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1  
kind: ModelServer
metadata:
  name: llama2-70b
spec:
  model: "llama2-70b"
  workloadSelector:
    matchLabels:
      app: llama2-70b
    pdGroup:
      groupKey: "group"
      prefillLabels:
        role: "prefill"
      decodeLabels:
        role: "decode"
  kvConnector:
    type: "lmcache"
```


## 5. Testing Strategy

### 5.1. Unit Testing
- Mock implementations for all connector interfaces
- Comprehensive test coverage for error conditions  
- Performance benchmarks for each connector type

### 5.2. Integration Testing  
- End-to-end testing with real vLLM pods
- Multi-connector scenarios with different models
- Failure injection and recovery testing

### 5.3. Performance Testing
- Load testing with different connector types
- Latency and throughput benchmarking
- Resource usage profiling

### 5.4. Chaos Testing
- Network partitions between prefill/decode pods
- Storage failures and recovery scenarios  
- Memory pressure and OOM conditions

## 6. Conclusion

The implemented architecture for vLLM KV Connectors addresses the limitations of a monolithic routing implementation by providing a robust, extensible, and production-ready solution. By introducing a `KVConnector` interface and providing implementations for different KV cache transfer strategies (`HTTP` and `NIXL`), this architecture significantly improves the flexibility and performance of PD disaggregated routing in MatrixInfer.

The design allows operators to select the appropriate connector (`http`, `lmcache`, `mooncake`, or `nixl`) via the `ModelServer` CRD. While `lmcache` and `mooncake` currently default to the `http` connector's behavior, the factory pattern makes it straightforward to add dedicated implementations as needed. This allows operators to choose between simple HTTP-based integration and high-performance in-memory caching with `NIXL`, optimizing for their specific use cases.

This architecture positions MatrixInfer to fully leverage the diverse capabilities of vLLM's KV cache system while providing the operational excellence required for production deployments.
