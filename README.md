# Kthena

<p align="center">
  <img src="docs/proposal/images/kthena-arch.svg" alt="Kthena Architecture" width="800"/>
</p>

<p align="center">
  <strong>The Enterprise-Grade LLM Serving Platform That Makes AI Infrastructure Simple, Scalable, and Cost-Efficient</strong>
</p>

<p align="center">
| <a href="#">Documentation</a> | <a href="#">Blog</a> | <a href="#">White Paper</a> | <a href="#">Slack</a> |

</p>

## Overview

**Kthena** is a Kubernetes-native LLM inference platform that provides declarative model lifecycle management and intelligent request routing for production deployments.

The platform extends Kubernetes with Custom Resource Definitions (CRDs) for managing LLM workloads, supporting multiple inference engines (vLLM, SGLang) and advanced serving patterns like prefill-decode disaggregation. Kthena's architecture separates control plane operations (model lifecycle, autoscaling policies) from data plane traffic routing through an intelligent router.

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
-   **Prometheus Metrics**: Built-in metrics collection for router performance and model serving
-   **Request Tracking**: Detailed request routing and performance monitoring
-   **Health Checks**: Comprehensive health checks for all model servers

## Architecture

Kthena implements a Kubernetes-native architecture with separate control plane and data plane components. The platform manages LLM inference workloads through CRD and provides intelligent request routing through a dedicated router.

For more details, please refer to [Kthena Architecture](docs/kthena/docs/architecture/architecture.mdx)


## Performance and Benchmarks

Kthena delivers significant performance improvements and cost savings compared to traditional LLM serving approaches. Here are key performance characteristics and benchmark results:

// TODO: Add some perf stats

> [!Note]
> Benchmark results may vary based on model size, hardware configuration, and workload patterns. Contact us for environment-specific performance testing.

## Getting Started

Get up and running with Kthena in minutes. This [guide](docs/kthena/docs/getting-started/quick-start.md) will walk you through installing the platform and deploying your first LLM model.

## Community

Kthena is an open source project that welcomes contributions from developers, platform engineers, and AI practitioners.

**Get Involved:**
- **Issues**: Report bugs and request features on [GitHub Issues](https://github.com/volcano-sh/kthena/issues)
- **Discussions**: Join conversations on [GitHub Discussions](https://github.com/volcano-sh/kthena/discussions)
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

Kthena is licensed under the [Apache 2.0 License](LICENSE).