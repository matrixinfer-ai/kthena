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

**Kthena** is a Kubernetes-native LLM inference platform that transforms how organizations deploy and manage Large Language Models in production. Built with declarative model lifecycle management and intelligent request routing, it provides high performance and enterprise-grade scalability for LLM inference workloads.

The platform extends Kubernetes with purpose-built Custom Resource Definitions (CRDs) for managing LLM workloads, supporting multiple inference engines (vLLM, SGLang, Triton) and advanced serving patterns like prefill-decode disaggregation. Kthena's architecture separates control plane operations (model lifecycle, autoscaling policies) from data plane traffic routing through an intelligent router, enabling teams to manage complex LLM deployments with familiar cloud-native patterns while delivering cost-driven autoscaling, heterogeneous accelerators support, and multi-backend inference engines.

## Key Features

### **Production-Ready LLM Serving**
Deploy and scale Large Language Models with enterprise-grade reliability, supporting vLLM, SGLang, Triton, and TorchServe inference engines through consistent Kubernetes-native APIs.

### **Simplified LLM Management**
- **Prefill-Decode Disaggregation**: Separate compute-intensive prefill operations from token generation decode processes to optimize hardware utilization and meet latency-based SLOs.
- **Cost-Driven Autoscaling**: Intelligent scaling based on multiple metrics (CPU, GPU, memory, custom) with configurable budget constraints and cost optimization policies
- **Zero-Downtime Updates**: Rolling model updates with configurable strategies
- **Dynamic LoRA Management**: Hot-swap adapters without service interruption  

### **Built-in Network Topology-Aware Scheduling**
Network topology-aware scheduling places inference instances within the same network domain to maximize inter-instance communication bandwidth and enhance inference performance.

### **Built-in Gang Scheduling**
Gang scheduling ensures atomic scheduling of distributed inference groups like xPyD, preventing resource waste from partial deployments.

### Intelligent Routing & Traffic Control
- Multi-model routing with pluggable load-balancing algorithms, including model load aware and KV-cache aware strategies.
- PD group aware request distribution for xPyD (x-prefill/y-decode) deployment patterns.
- Rich traffic policies, including canary releases, weighted traffic distribution, token-based rate limiting, and automated failover.
- LoRA adapter aware routing without inference outage

## Architecture

Kthena implements a Kubernetes-native architecture with separate control plane and data plane components, each can be deployed and used alone. The platform manages LLM inference workloads through CRD and provides intelligent request routing through a dedicated router.

For more details, please refer to [Kthena Architecture](docs/kthena/docs/architecture/architecture.mdx)

> [!Note]
> The router component is a reference implementation, because Gateway Inference Extension does not natively support prefill-decode distribution. kthena router is still under active iteration, and it can be deployed behind a standard api gateway.


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