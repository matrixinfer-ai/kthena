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

## 3. Proposed Architecture

### 3.1. Core Design Principles

**Separation of Concerns**: Separate KV cache operations from routing logic
**Pluggability**: Support multiple connector implementations with unified interface  
**Configurability**: Allow per-model connector configuration via CRDs
**Resilience**: Comprehensive error handling, retries, and graceful degradation

### 3.2. KVConnector Interface Design

We propose a comprehensive `KVConnector` interface in `pkg/infer-gateway/connector/interface.go`:

```go
package connector

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)


// KVConnector is the main interface for KV cache operations
type KVConnector interface {
	// Name returns the connector type name
	Name() string

		// Prefill executes prefill and parse the prefill response necessarily
	Proxy(ctx context.Context, req *http.Request, prefillAddr, decodeAddr string) error
	
	// Prefill executes prefill and parse the prefill response necessarily
	Prefill(ctx context.Context, req *http.Request, prefillAddr string) error
	
	// Decode executes decode using stored KV cache  
	Decode(ctx context.Context, c *gin.Context, req *http.Request, decodeAddr string) error
}
```

### 3.3. Connector Implementations

#### 3.3.1 HTTPConnector (Default)
Maintains backward compatibility with current implementation and supports HTTP-based connectors including MooncakeStoreConnector:

```go
// HTTPConnector implements simple HTTP-based KV transfer and MooncakeStore integration
type HTTPConnector struct {
	client *http.Client
}
```

#### 3.3.2 LMCacheConnector  
Distributed KV cache management using vLLM's LMCache system with support for CPU/disk offloading:

```go
type LMCacheConnector struct {
	client *http.Client
}
```

#### 3.3.3 NIXLConnector
High-performance distributed in-memory KV cache using NIXL (NVIDIA Inference Xfer Library):

```go
// NIXLConnectorConfig specifies NIXL configuration for high-performance distributed KV cache
type NIXLConnectorConfig struct {
	client *http.Client
}
```

### 3.4. Router Integration

The enhanced `Router` will integrate with the KV connector system:

```go
type Router struct {
	scheduler       scheduler.Scheduler
	store           datastore.Store  
	loadRateLimiter *ratelimit.TokenRateLimiter
	
	// KV Connector management
	connectorFactory  *connector.Factory
	connectorRegistry map[string]connector.KVConnector
}

func (r *Router) proxyModelEndpoint(
	c *gin.Context,
	req *http.Request, 
	ctx *framework.Context,
	modelRequest ModelRequest,
	port int32,
) error {
	// Get appropriate connector for this model server
	kvConnector, err := r.getKVConnector(ctx.ModelServerName)
	if err != nil {
		return fmt.Errorf("failed to get KV connector: %w", err)
	}
	
	
	// Handle different routing modes
	if ctx.BestPods != nil {
		// PD aggregated mode - direct proxy
		return r.proxyToPDAggregated(c, req, ctx, port)
	}
	
	// PD disaggregated mode - prefill then decode
	return r.proxyToPDDisaggregated(c, req, ctx, kvConnector, port)
}

func (r *Router) proxyToPDDisaggregated(
	c *gin.Context,
	req *http.Request,
	ctx *framework.Context, 
	kvConnector connector.KVConnector,
	port int32,
) error {
	// Try multiple prefill/decode pairs
	maxRetry := min(len(ctx.PrefillPods), len(ctx.DecodePods))
	for i := 0; i < maxRetry; i++ {
		if ctx.PrefillPods[i] == nil || ctx.DecodePods[i] == nil {
			continue
		}
		
		// Build addresses for prefill and decode pods
		prefillAddr := fmt.Sprintf("%s:%d", ctx.PrefillPods[i].Pod.Status.PodIP, port)
		decodeAddr := fmt.Sprintf("%s:%d", ctx.DecodePods[i].Pod.Status.PodIP, port)
		
		// Execute prefill-decode flow with KV connector
		err := kvConnector.Proxy(req.Context(), req, prefillAddr, decodeAddr)
		if err != nil {
			klog.Errorf("KV connector proxy failed for prefill pod %s, decode pod %s: %v", 
				ctx.PrefillPods[i].Pod.Name, ctx.DecodePods[i].Pod.Name, err)
			continue
		}
		
		// Record successful operation in cache
		r.scheduler.RunPostHooks(ctx, i)
		
		klog.V(4).Infof("KV transfer successful for prefill pod %s, decode pod %s", 
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
	// +kubebuilder:validation:Enum=http;lmcache;nixl
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

The proposed enhanced architecture for vLLM KV Connectors addresses the fundamental limitations of the current implementation while providing a robust, extensible, and production-ready solution. By focusing on the actual vLLM connector types (HTTP, LMCache, and NIXL) and introducing proper abstractions, comprehensive error handling, and rich observability, this architecture will significantly improve the reliability and performance of PD disaggregated routing in MatrixInfer.

The streamlined design ensures that new connector types can be easily added as vLLM evolves, while the backward-compatible migration strategy minimizes disruption to existing users. The focused configuration options allow operators to choose between HTTP compatibility and high-performance in-memory caching solutions, optimizing for their specific use cases whether prioritizing compatibility, latency, or throughput.

This architecture positions MatrixInfer to fully leverage the capabilities of vLLM's KV cache system while providing the operational excellence required for production deployments.
