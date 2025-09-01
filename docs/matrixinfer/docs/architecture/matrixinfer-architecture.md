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

**Request Pipeline:** `User → Auth → Rate Limiting → Fairness Scheduling → Load Balancing → Proxy → Inference Pods`

- **Authentication & Authorization** – Validates user identity and permissions
- **Rate Limiting** – Enforces request throughput limits to prevent system overload
- **Fairness Scheduling** – Implements queuing mechanisms and fair resource allocation
- **Load Balancing** – Routes requests to optimal backend instances based on health and capacity
- **Proxy** – Dispatches requests to appropriate data plane inference groups

#### **Scheduler**

The Scheduler employs **advanced scheduling plugins** to optimize request placement and resource utilization:

**Filter Plugins:**
- *Least Requests* – Filters nodes based on current request load
- *LoRA Affinity* – Ensures requests requiring specific LoRA adapters are routed to compatible nodes

**Score Plugins:**
- *KV Cache Aware* – Optimizes placement based on key-value cache availability and utilization
- *Least Latency* – Minimizes Time to First Token (TTFT) and Time Per Output Token (TPOT)
- *Prefix Cache* – Leverages shared prefix caching for improved performance
- *GPU Cache* – Considers GPU memory cache status for optimal placement

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

## How it Works (The Flow)

1.  **Someone wants an AI to do something (User Request):**
    *   A **User** sends a request to the **Infer Gateway**.
2.  **The Gateway checks things (Infer Gateway):**
    *   It first checks who you are (**Auth**) and if you're sending too many requests (**Rate Limiting**).
3.  **Making sure everyone gets a turn (Fairness Scheduling & Queue):**
    *   If many requests come in at once, a **Fairness Scheduler** makes sure everyone gets a fair chance, sometimes putting requests in a **Queue** to wait.
4.  **Sending the request to the right AI worker (Load Balancing & Proxy):**
    *   A **Load Balancer** then directs the request to a healthy "worker" based on model information (from the Control Plane).
    *   A **Proxy** acts as the delivery service, taking the request to the specific AI "worker."
5.  **The AI Workers do the thinking (Inference Pods):**
    *   These are called **Inference Pods**. Each pod is like a small AI expert.
    *   For LLMs, there might be two types of experts:
    *   **Prefill Pods (Role A):** Handle the initial understanding of your request.
    *   **Decode Pods (Role B):** Generate the actual answer, token by token.
    *   Each pod has special **LLM engines** (like vLLM or SGLang) that are good at LLM tasks.
    *   They also have helpers: an **Init Container** to download the model (like getting the right tools) and a **Sidecar Container** with a **Runtime Agent** to keep an eye on how well the worker is doing.
6.  **Managing the Workers (Control Plane in Action):**
    *   **Operators** can tell the **Model Controller** to add new AI models or change existing ones.
    *   The **Model Controller** then tells other parts, like the **Model Route Controller** (how to find the new model), the **Model Server Controller** (how to set up the new model's home), and the **Model Infer Controller** (how to manage its workers).
    *   It also sets **Autoscaling Policies** (how many workers to have based on demand) and binds them to the model.
    *   The **Autoscaler Controller** watches the **Inference Pods** for performance (using "get metrics") and tells the **Model Infer Controller** to create more or fewer workers as needed.
7.  **Smartly Placing Workers (Scheduler):**
    *   A **Scheduler** decides *where* new **Inference Pods** should run (e.g., on which powerful computer).
    *   It uses **Filter Plugins** to rule out unsuitable places and **Score Plugins** to pick the best spot, considering things like available memory (**KV Cache Aware**, **GPU Cache**) and how quickly it can start responding (**Least Latency**).

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