# MatrixInfer

<p align="center">
  <img src="docs/proposal/images/matrixInfer-arch.svg" alt="MatrixInfer Architecture" width="800"/>
</p>

<p align="center">
  <strong>The Enterprise-Grade LLM Serving Platform That Makes AI Infrastructure Simple, Scalable, and Cost-Efficient</strong>
</p>

<p align="center">
| <a href="#">Documentation</a> | <a href="#">Blog</a> | <a href="#">White Paper</a> | <a href="#">Slack</a> |

</p>

## Overview

**MatrixInfer** is a Kubernetes-native LLM inference platform that provides declarative model lifecycle management and intelligent request routing for production deployments.

The platform extends Kubernetes with CRD for managing LLM workloads, supporting multiple inference engines (vLLM, SGLang) and advanced serving patterns like prefill-decode disaggregation. MatrixInfer's architecture separates control plane operations (model lifecycle, autoscaling policies) from data plane traffic routing through an intelligent gateway.

## Key Features

### **Multi-Backend Inference Engine**
-   **Engine Support**: Native support for vLLM, SGLang, Triton, TorchServe inference engines with consistent Kubernetes-native APIs
-   **Serving Patterns**: Support for both standard and disaggregated serving patterns across heterogeneous hardware accelerators
-   **Advanced Load Balancing**: Pluggable scheduling algorithms including least request, least latency, random, LoRA affinity, prefix-cache, KV-cache and PD Groups aware routing
-   **Traffic Management**: Supports canary releases, weighted traffic distribution, token-based rate limiting, and automated failover policies
-   **LoRA Adapter Management**: Dynamic LoRA adapter routing and management without service interruption
-   **Rolling Updates**: Zero-downtime model updates with configurable rollout strategies

### **Prefill-Decode Disaggregation**
-   **Workload Separation**: Optimize large model serving by separating prefill and decode workloads for enhanced performance
-   **KV Cache Coordination**: Seamless coordination through LMCache, MoonCake, or NIXL connectors for optimized resource utilization
-   **Intelligent Routing**: Model-aware request distribution with PD Groups awareness for disaggregated serving patterns

### **Cost-Driven Autoscaling**
-   **Multi-Metric Scaling**: Autoscaling based on custom metrics, CPU, memory, GPU utilization, and budget constraints
-   **Flexible Policies**: Flexible scaling behaviors with panic mode, stable scaling policies, and configurable scaling behaviors
-   **Policy Binding**: Granular autoscaling policy assignment to specific model deployments not limited to `ModelInfer`

### **Observability & Monitoring**
-   **Prometheus Metrics**: Built-in metrics collection for gateway performance and model serving
-   **Request Tracking**: Detailed request routing and performance monitoring
-   **Health Checks**: Comprehensive health checks for all model servers

## Architecture

MatrixInfer implements a Kubernetes-native architecture with separate control plane and data plane components. The platform manages LLM inference workloads through CRD and provides intelligent request routing through a dedicated gateway.

### Control Plane Components

**Model Controller** (`model-controller`)
- Manages `Model` CRDs and orchestrates them into `ModelInfer`, `ModelServer`, and `ModelRoute` resources
- Handles model lifecycle operations including creation, updates, and deletion
- Supports multiple inference engine types: vLLM, SGLang
- Dynamic LoRA adapter management capabilities
- Integrates with autoscaling policies for dynamic resource management

**Inference Controller** (`infer-controller`)
- Manages `ModelInfer` workloads and instantialize them into Kubernetes Pods and Services
- Handles pod lifecycle management with support for different recovery policies (InferGroupRecreate, RoleRecreate, None)
- Implements rolling update strategies for zero-downtime model updates
- Manages multi-role deployments for prefill-decode disaggregation patterns
- Provides workload scheduling and resource allocation

**Autoscaler** (`autoscaler`)
- Implements autoscaling based on `AutoscalingPolicy` and `AutoscalingPolicyBinding` resources
- Supports multiple metrics including custom metrics for scaling decisions
- Provides configurable scaling behaviors with panic mode and stable scaling policies
- Manages scale-to-zero capabilities with grace periods
- Integrates with Prometheus for metrics collection

**Admission Webhooks**
- **Registry Webhook** (`registry-webhook`): Validates and mutates `Model`, `AutoscalingPolicy`, and `AutoscalingPolicyBinding` resources
- **ModelInfer Webhook** (`modelinfer-webhook`): Validates `ModelInfer` resource specifications
- **Infer Gateway Webhook** (`infer-gateway-webhook`): Validates `ModelServer` and `ModelRoute` configurations

### Data Plane Component
**Inference Gateway** (`infer-gateway`)
- HTTP gateway that routes inference requests to appropriate model servers efficiently
- Implements pluggable scheduling algorithms (least request, least latency, random, LoRA affinity, prefix-based)
- Supports multiple KV connector types for prefill-decode disaggregation: HTTP, NIXL, LMCache, MooncakeStore
- Implements token-based rate limiting and failover traffic policies
- Supports multiple inference engines: vLLM and SGLang

### Supported Serving Patterns

**Standard Serving**
- Single-instance model serving with standard vLLM or SGLang backends
- Automatic load balancing across multiple replicas
- Support for LoRA adapter management

**Prefill-Decode Disaggregation**
- Separate prefill and decode workloads for enhanced performance
- KV cache coordination through configurable connectors (NIXL, LMCache, MooncakeStore)
- Optimized resource allocation for different workload types

**Multi-Backend Deployments**
- Support for multiple backends within a single `model` deployment
- Weighted traffic distribution across different backend versions
- Canary deployments and A/B testing capabilities

## Getting Started

// TODO add a link

## Community

MatrixInfer is an open source project that welcomes contributions from developers, platform engineers, and AI practitioners.

**Get Involved:**
- **Issues**: Report bugs and request features on [GitHub Issues](https://github.com/matrixinfer-ai/matrixinfer/issues)
- **Discussions**: Join conversations on [GitHub Discussions](https://github.com/matrixinfer-ai/matrixinfer/discussions)
- **Documentation**: Help improve guides and examples

## Contributing

Contributions are welcome! Here's how to get started:

### Contribution Guidelines

- **Code**: Follow Go conventions and include tests for new features
- **Documentation**: Update relevant docs and examples
- **Issues**: Use GitHub Issues for bug reports and feature requests
- **Pull Requests**: Ensure CI passes and include clear descriptions

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed guidelines.

## License

MatrixInfer is licensed under the [Apache 2.0 License](LICENSE).