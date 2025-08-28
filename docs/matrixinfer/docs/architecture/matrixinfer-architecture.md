# Architecture Overview

MatrixInfer is a Kubernetes-native AI inference platform designed for scalable, efficient, and intelligent model serving. This document provides an overview of the system architecture and core components.

## High-Level Architecture
![architecture_overview.svg](../../static/img/diagrams/architecture/architecture_overview.svg)

## Core Components

### 1. Inference Gateway (`infer-gateway`)

The inference gateway serves as the entry point for all inference requests and provides:

- **Request Routing**: Intelligent routing based on model versions, A/B testing rules, and traffic policies
- **Load Balancing**: Distributes requests across multiple model instances using various algorithms
- **Rate Limiting**: Protects models from overload with configurable rate limiting policies
- **Authentication & Authorization**: Secures access to models with various auth mechanisms
- **Request/Response Transformation**: Handles protocol translation and data format conversion

**Key Features:**
- Protocol support: HTTP/REST, gRPC
- Routing strategies: Round-robin, weighted, least-connections
- Circuit breaker patterns for fault tolerance
- Request/response caching

### 2. Model Controller (`model-controller`)

Manages the lifecycle of AI models in the registry:

- **Model Registration**: Handles model metadata, versioning, and storage references
- **Model Validation**: Validates model specifications and compatibility
- **Lifecycle Management**: Manages model states (registered, validated, deprecated)
- **Dependency Resolution**: Resolves model dependencies and runtime requirements

**Responsibilities:**
- Reconciles `Model` custom resources
- Validates model specifications against schemas
- Manages model storage and retrieval
- Handles model versioning and rollback

### 3. Inference Controller (`infer-controller`)

Orchestrates the deployment and scaling of inference workloads:

- **Workload Management**: Creates and manages inference deployments
- **Resource Allocation**: Optimizes resource usage based on model requirements
- **Health Monitoring**: Monitors inference pod health and performance
- **Scaling Decisions**: Works with autoscaler for intelligent scaling

**Key Functions:**
- Reconciles `ModelInfer` custom resources
- Manages inference pod lifecycle
- Handles rolling updates and rollbacks
- Integrates with Kubernetes HPA/VPA

### 4. Autoscaler (`autoscaler`)

Provides intelligent auto-scaling capabilities:

- **Predictive Scaling**: Uses historical data and ML models for proactive scaling
- **Multi-metric Scaling**: Scales based on CPU, memory, GPU, and custom metrics
- **Cost Optimization**: Balances performance and cost through intelligent scaling
- **Cold Start Mitigation**: Maintains warm pools to reduce cold start latency

**Scaling Strategies:**
- Reactive scaling based on current metrics
- Predictive scaling using time-series forecasting
- Schedule-based scaling for known traffic patterns
- Custom metric scaling (queue depth, response time)

### 5. Registry Webhook (`registry-webhook`)

Provides admission control for model registry operations:

- **Validation**: Validates model specifications before admission
- **Mutation**: Applies default values and transformations
- **Policy Enforcement**: Enforces organizational policies and constraints
- **Security Scanning**: Integrates with security scanners for model validation

### 6. ModelInfer Webhook (`modelinfer-webhook`)

Handles admission control for inference workloads:

- **Resource Validation**: Validates resource requests and limits
- **Scheduling Constraints**: Applies node affinity and scheduling rules
- **Security Policies**: Enforces security contexts and policies
- **Configuration Injection**: Injects runtime configurations and secrets

## Custom Resource Definitions (CRDs)

MatrixInfer extends Kubernetes with several custom resources:

### Registry API Group (`registry.matrixinfer.ai/v1alpha1`)

- **Model**: Represents an AI model with metadata, specifications, and runtime requirements
- **AutoscalingPolicy**: Defines scaling policies and strategies for models

### Workload API Group (`workload.matrixinfer.ai/v1alpha1`)

- **ModelInfer**: Represents an inference deployment with scaling and resource specifications

### Networking API Group (`networking.matrixinfer.ai/v1alpha1`)

- **ModelServer**: Exposes models through network services with routing and security policies
- **ModelRoute**: Defines advanced routing rules for traffic management and A/B testing

## Data Flow

### 1. Model Registration Flow

```
Developer → Model CRD → Registry Webhook → Model Controller → Model Registry
```

1. Developer creates a `Model` resource
2. Registry webhook validates the specification
3. Model controller processes the registration
4. Model metadata is stored in the registry

### 2. Inference Deployment Flow

```
Model CRD → ModelInfer CRD → Infer Controller → Kubernetes Deployment → Pods
```

1. `ModelInfer` resource references a registered model
2. Inference controller creates Kubernetes deployments
3. Pods are scheduled and started with model runtime
4. Health checks ensure pods are ready for traffic

### 3. Request Processing Flow

```
Client → Inference Gateway → Load Balancer → Model Pod → Response
```

1. Client sends inference request to gateway
2. Gateway applies routing, rate limiting, and security policies
3. Request is load-balanced to available model instances
4. Model pod processes request and returns response

## Scalability and Performance

### Horizontal Scaling

- **Controller Scaling**: Multiple controller replicas for high availability
- **Gateway Scaling**: Horizontal scaling of gateway instances
- **Model Scaling**: Independent scaling of each model deployment

### Performance Optimizations

- **Connection Pooling**: Efficient connection management between components
- **Request Batching**: Automatic batching of inference requests
- **Caching**: Multi-level caching for models and responses
- **GPU Optimization**: Efficient GPU resource sharing and scheduling

## Security Architecture

### Authentication & Authorization

- **RBAC Integration**: Kubernetes RBAC for resource access control
- **Service Mesh**: Optional integration with Istio/Linkerd for mTLS
- **API Keys**: Support for API key-based authentication
- **JWT Tokens**: JWT-based authentication for external clients

### Network Security

- **Network Policies**: Kubernetes network policies for traffic isolation
- **TLS Encryption**: End-to-end TLS encryption for all communications
- **Admission Control**: Webhook-based admission control for security policies

## Observability

### Monitoring

- **Metrics**: Prometheus metrics for all components
- **Distributed Tracing**: OpenTelemetry integration for request tracing
- **Logging**: Structured logging with correlation IDs

### Key Metrics

- Request latency and throughput
- Model accuracy and drift detection
- Resource utilization (CPU, memory, GPU)
- Error rates and availability

## High Availability

### Component Redundancy

- Multiple replicas of all control plane components
- Leader election for controllers
- Graceful failover and recovery

### Data Persistence

- Model registry backed by persistent storage
- Configuration stored in Kubernetes etcd
- Stateless design for easy recovery

## Integration Points

### Kubernetes Integration

- Native Kubernetes resources and APIs
- Integration with Kubernetes scheduler
- Support for Kubernetes networking and storage

### External Integrations

- **Model Stores**: S3, GCS, Azure Blob, Hugging Face Hub
- **Monitoring**: Prometheus, Grafana, DataDog
- **Security**: OPA/Gatekeeper, Falco, Twistlock
- **Service Mesh**: Istio, Linkerd, Consul Connect

This architecture provides a robust, scalable, and secure foundation for AI inference workloads in Kubernetes environments.