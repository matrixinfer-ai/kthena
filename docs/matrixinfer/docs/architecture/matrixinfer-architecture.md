import LightboxImage from '@site/src/components/LightboxImage';

# Architecture Overview

MatrixInfer is a Kubernetes-native AI inference platform built on a **two-plane architecture** designed for scalability, observability, and efficiency. The system separates **control plane operations** and **data plane execution** into distinct architectural planes.

## High-Level Architecture

The platform comprises two primary planes:

- **Control Plane**: Manages models, routes, servers, inference configurations, and autoscaling policies through Kubernetes Custom Resource Definitions (CRDs) and Controllers. Controllers continuously reconcile these CRDs into runtime resources.
- **Data Plane**: Executes inference workloads through the Gateway and Scheduler, which orchestrate request routing and scheduling. Inference Pods execute AI model requests using **role-based replica groups** supporting Prefill/Decode disaggregation.

<LightboxImage src="/img/diagrams/architecture/architecture_overview.svg" alt="Architecture Overview"></LightboxImage>

## Core Components

### 1. **Custom Resource Definitions (CRDs)**

MatrixInfer extends Kubernetes with custom resources that provide declarative configuration for AI inference workloads:

- **Model** – Defines model specifications including weights, checkpoints, and metadata
- **ModelRoute** – Configures routing rules and traffic control policies
- **ModelServer** – Manages REST/gRPC endpoints with exposure and authentication rules
- **ModelInfer** – Specifies inference groups, replica configurations, and runtime parameters
- **AutoScalingPolicy** – Defines autoscaling triggers, metrics, and scaling behaviors
- **AutoScalingPolicyBinding** – Associates models with specific autoscaling policies

Platform operators manage these CRDs declaratively, and Control Plane controllers continuously reconcile them into runtime resources.

### 2. **Control Plane**

The Control Plane ensures that declarative configurations are realized into operational resources through continuous reconciliation of CRDs into runtime resources.

#### **Controllers**

- **Model Controller** – Watches `Model` CRDs and manages model lifecycle and state transitions
- **ModelRoute Controller** – Synchronizes routing configurations into the inference gateway
- **ModelServer Controller** – Manages serving endpoints, connectivity, and exposure policies
- **ModelInfer Controller** – Orchestrates inference groups, role assignments, and replica management
- **Autoscaler Controller** – Collects runtime metrics from pods and evaluates scaling decisions based on configured policies

### 3. **Data Plane**

The Data Plane executes inference workloads and handles request processing through the Gateway and Scheduler, using optimized, role-based pod architectures that support both homogeneous and heterogeneous scaling strategies.

#### **Infer Gateway**

The Infer Gateway processes user requests through a comprehensive pipeline that ensures security, fairness, and optimal resource utilization:

**Request Pipeline:**

- **Authentication & Authorization** – Validates user identity and permissions
- **Rate Limiting** – Enforces request throughput limits to prevent system overload
- **Fairness Scheduling** – Implements queuing mechanisms and fair resource allocation
- **Scheduling** – Selects optimal pods using filter and score plugins for intelligent request routing
- **Load Balancing** – Routes requests to optimal backend instances based on health and capacity
- **Proxy** – Dispatches requests to appropriate data plane inference groups

#### **Scheduler**

The Scheduler employs **advanced scheduling plugins** to optimize request routing and resource utilization:

**Filter Plugins:**
- *Least Requests* – Filters pods based on current request load
- *LoRA Affinity* – Ensures requests requiring specific LoRA adapters are routed to compatible pods

**Score Plugins:**
- *KV Cache Aware* – Optimizes routing based on key-value cache availability and utilization
- *Least Latency* – Minimizes Time to First Token (TTFT) and Time Per Output Token (TPOT)
- *Prefix Cache* – Leverages shared prefix caching for improved performance
- *GPU Cache* – Considers GPU memory cache status for optimal routing

The scheduler seamlessly integrates with Load Balancing and Fairness Scheduling components to ensure optimal request distribution.

#### **Inference Pods**

Inference workloads are organized into **Groups** containing **multiple Replicas**. Each replica can assume specialized **roles** to optimize different phases of the inference process:

**Role-Based Architecture:**
- **Prefill Role** – Handles prompt initialization and context processing
- **Decode Role** – Manages incremental token generation and output streaming

**Pod Components:**
Each replica deployment may include the following components:

- **Entry Pod** – Provides ingress endpoints for role-specific requests
- **Worker Pod(s)** – Execute actual model inference computations
- **Init Container** – Handles dependency resolution and artifact setup prior to execution
- **Sidecar Container** – Manages logging, observability, and networking auxiliary processes
- **Downloader** – Fetches model weights and artifacts from storage
- **Runtime Agent** – Collects health metrics and performance telemetry
- **LLM Engines** – Integrates with specialized backends (e.g., **vLLM**, **SGLang**)

This architecture enables **Prefill/Decode Disaggregation (PD mode)**, allowing independent scaling of different inference stages for optimal resource utilization and performance.

## Request Processing Pipeline

1.  **Request Ingress (Client Request Initiation):**
    *   Client applications submit inference requests to the **Infer Gateway** via REST/gRPC endpoints.
2.  **Gateway Request Processing (Authentication & Rate Control):**
    *   The **Infer Gateway** validates client credentials through **Authentication & Authorization** middleware and enforces **Rate Limiting** policies to prevent system overload.
3.  **Fairness Scheduling & Queue Management:**
    *   When request volume exceeds processing capacity, the **Fairness Scheduler** implements queue-based resource allocation mechanisms to ensure equitable request distribution and prevent resource starvation.
4.  **Intelligent Request Routing (Scheduler Optimization):**
    *   The **Scheduler** evaluates available **Inference Pods** using a multi-stage selection algorithm:
    *   **Filter Plugins** eliminate unsuitable pods based on resource constraints and compatibility requirements.
    *   **Score Plugins** rank candidate pods using optimization criteria including current request load (**Least Requests**), cache utilization (**KV Cache Aware**, **GPU Cache**), latency minimization (**Least Latency**), and prefix caching efficiency (**Prefix Cache**).
5.  **Load Balancing & Request Dispatching:**
    *   The **Load Balancer** routes requests to optimal backend instances based on health checks and capacity metrics derived from Control Plane metadata.
    *   The **Proxy** component handles request forwarding and maintains connection pooling to target inference groups.
6.  **Inference Pod Execution:**
    *   **Inference Pods** execute model computations within specialized role-based architectures:
    *   **Prefill Pods:** Process input tokenization, context encoding, and KV cache initialization for prompt understanding.
    *   **Decode Pods:** Perform autoregressive token generation and output streaming for response completion.
    *   Each pod integrates **LLM Engines** (vLLM, SGLang) for optimized inference execution.
    *   Supporting components include **Init Containers** for model artifact retrieval, **Sidecar Containers** with **Runtime Agents** for telemetry collection and health monitoring.
7.  **Control Plane Orchestration (Resource Lifecycle Management):**
    *   **Platform Operators** manage model deployments through CRD manipulation of **Model** resources.
    *   The **Model Controller** propagates configuration changes to downstream controllers: **ModelRoute Controller** (routing rule synchronization), **ModelServer Controller** (endpoint configuration), and **ModelInfer Controller** (replica group orchestration).
    *   **AutoScaling Policies** define scaling triggers and behaviors, bound to models through **AutoScalingPolicyBinding** resources.
    *   The **Autoscaler Controller** continuously monitors pod metrics and executes scaling decisions by instructing the **ModelInfer Controller** to adjust replica counts based on configured policies.

## Key Features

MatrixInfer provides comprehensive capabilities for enterprise-grade AI inference deployment:

- **Declarative Management**: Complete CRD-driven configuration for models, routes, servers, and scaling policies
- **Control Plane Orchestration**: Automated resource management with dedicated controllers and continuous reconciliation
- **Advanced Gateway Pipeline**: Full request processing pipeline with authentication, rate limiting, fairness scheduling, and intelligent routing
- **Intelligent Scheduling**: Pluggable scheduling framework with latency-aware, cache-aware, and LoRA affinity optimizations
- **Prefill/Decode Disaggregation**: Specialized workload separation enabling independent scaling of inference stages
- **Role-Based Architecture**: Flexible inference groups with granular replica management and role assignments
- **Enterprise Observability**: Comprehensive monitoring through Init Containers, Sidecar Containers, and Runtime Agents
- **Dynamic Scaling**: Metric-driven autoscaling with support for both homogeneous and heterogeneous instance types
- **Multi-Engine Support**: Native integration with leading LLM inference engines (vLLM, SGLang)
- **Kubernetes-Native**: Full integration with Kubernetes ecosystem including RBAC, networking, and storage