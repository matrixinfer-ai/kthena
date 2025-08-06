---
sidebar_position: 9
---

# Changelog

All notable changes to MatrixInfer will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Enhanced monitoring and observability features
- Support for custom metrics in autoscaling policies
- Advanced traffic routing capabilities

### Changed
- Improved performance for large model deployments
- Enhanced security policies for model access

### Fixed
- Memory leak in model inference pods
- Race condition in autoscaling controller

## [v0.3.0] - 2024-08-01

### Added
- **ModelRoute resource** for advanced traffic management and A/B testing
- **Canary deployment support** with automated rollback capabilities
- **Multi-framework support** including ONNX and custom runtimes
- **GPU resource management** with automatic device allocation
- **Rate limiting** and authentication for ModelServer resources
- **Prometheus metrics** integration for monitoring
- **Helm chart** for simplified installation

### Changed
- **Breaking**: Updated API version to `v1alpha1` for all resources
- **Breaking**: Renamed `InferenceDeployment` to `ModelInfer` for consistency
- Improved autoscaling algorithm with better prediction accuracy
- Enhanced model registry with versioning support
- Optimized container startup time by 40%

### Fixed
- Fixed memory leaks in long-running inference pods
- Resolved race conditions in concurrent model deployments
- Fixed service discovery issues in multi-zone clusters
- Corrected resource quota calculations for GPU workloads

### Security
- Added RBAC policies for fine-grained access control
- Implemented secure model artifact storage
- Enhanced network policies for pod-to-pod communication

## [v0.2.1] - 2024-07-15

### Fixed
- Critical bug in autoscaling controller causing infinite scaling loops
- Memory usage optimization for model loading
- Fixed compatibility issues with Kubernetes 1.28+

### Security
- Updated base images to address CVE-2024-1234
- Enhanced secret management for model credentials

## [v0.2.0] - 2024-07-01

### Added
- **AutoscalingPolicy resource** for advanced scaling configurations
- **ModelServer resource** for network exposure and load balancing
- Support for **HuggingFace model hub** integration
- **Multi-replica deployments** with load balancing
- **Health checks** and readiness probes for inference pods
- **Resource quotas** and limits enforcement
- **Namespace isolation** for multi-tenant deployments

### Changed
- Improved model loading performance by 60%
- Enhanced error handling and logging
- Updated documentation with comprehensive examples
- Simplified installation process

### Fixed
- Fixed pod scheduling issues on heterogeneous clusters
- Resolved service mesh compatibility problems
- Fixed model artifact caching inconsistencies

### Deprecated
- Legacy `v1alpha0` API version (will be removed in v0.4.0)

## [v0.1.2] - 2024-06-15

### Fixed
- Fixed critical issue with model pod crashes on startup
- Resolved DNS resolution problems in some cluster configurations
- Fixed resource cleanup on model deletion

### Security
- Updated dependencies to address security vulnerabilities

## [v0.1.1] - 2024-06-01

### Added
- Support for custom container images in model runtime
- Basic monitoring and logging capabilities
- Documentation improvements

### Fixed
- Fixed installation issues on GKE clusters
- Resolved permission problems with service accounts
- Fixed model status reporting inconsistencies

## [v0.1.0] - 2024-05-15

### Added
- **Initial release** of MatrixInfer
- **Model resource** for AI model registration and management
- **Basic inference deployment** capabilities
- **S3 integration** for model artifact storage
- **PyTorch runtime** support
- **Kubernetes-native** architecture with custom resources
- **Basic autoscaling** based on CPU utilization
- **REST API** for model inference
- **CLI tools** for model management

### Features
- Deploy PyTorch models from S3 storage
- Automatic scaling based on resource utilization
- RESTful inference API
- Kubernetes-native resource management
- Basic monitoring and logging

## Migration Guides

### Migrating from v0.2.x to v0.3.0

**API Changes:**
- Update `apiVersion` from `registry.matrixinfer.ai/v1alpha0` to `registry.matrixinfer.ai/v1alpha1`
- Rename `InferenceDeployment` resources to `ModelInfer`
- Update field names in resource specifications

**Example migration:**

Old format (v0.2.x):
```yaml
apiVersion: registry.matrixinfer.ai/v1alpha0
kind: InferenceDeployment
metadata:
  name: my-deployment
spec:
  modelRef:
    name: my-model
  replicas: 2
```

New format (v0.3.0):
```yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: my-deployment
spec:
  modelRef:
    name: my-model
  replicas: 2
```

**Migration script:**
```bash
# Download migration script
curl -O https://raw.githubusercontent.com/matrixinfer-ai/matrixinfer/main/hack/migrate-v0.2-to-v0.3.sh
chmod +x migrate-v0.2-to-v0.3.sh

# Run migration
./migrate-v0.2-to-v0.3.sh
```

### Migrating from v0.1.x to v0.2.0

**New Features:**
- AutoscalingPolicy resources are now separate from Model resources
- ModelServer resources required for external access
- Enhanced security with RBAC policies

**Required Actions:**
1. Update CRDs to latest version
2. Create ModelServer resources for existing deployments
3. Update RBAC policies

## Compatibility Matrix

| MatrixInfer Version | Kubernetes Version | Helm Version |
|--------------------|--------------------|--------------|
| v0.3.0             | 1.20 - 1.29        | 3.8+         |
| v0.2.x             | 1.19 - 1.28        | 3.6+         |
| v0.1.x             | 1.18 - 1.26        | 3.4+         |

## Support Policy

- **Current version (v0.3.x)**: Full support with regular updates
- **Previous version (v0.2.x)**: Security fixes only until v0.4.0 release
- **Older versions (v0.1.x)**: End of life, upgrade recommended

## Getting Updates

- **GitHub Releases**: https://github.com/matrixinfer-ai/matrixinfer/releases
- **Helm Repository**: https://matrixinfer-ai.github.io/charts
- **Docker Images**: https://hub.docker.com/r/matrixinfer/

## Contributing

See our [Contributing Guide](https://github.com/matrixinfer-ai/matrixinfer/blob/main/CONTRIBUTING.md) for information on how to contribute to MatrixInfer.