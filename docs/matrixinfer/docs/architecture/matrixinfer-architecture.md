import LightboxImage from '@site/src/components/LightboxImage';

# Architecture Overview

MatrixInfer is a Kubernetes‑native AI inference platform built on a **three‑plane architecture** for scalability, observability, and efficiency.
The system separates **declarative configuration**, **controller orchestration**, and **inference execution** into distinct planes.

## High-Level Architecture

---

The platform is composed of:

- **Control Plane**: Models, Routes, Servers, Inference configs, and AutoScaling policies expressed as Kubernetes CRDs. Controllers reconcile CRDs into runtime resources.
- **Data Plane**: The Gateway/Scheduler orchestrates request routing and scheduling. Inference Pods execute AI model requests with **role-based replica groups** (Prefill/Decode).

<LightboxImage src="/img/diagrams/architecture/architecture_overview.svg" alt="Architecture Overview"></LightboxImage>

---

## Core Components

### 1. **CRDs**

MatrixInfer extends Kubernetes with **custom resources**:

- **Model** – Defines model specification (weights, checkpoints, metadata).
- **ModelRoute** – Routing and traffic control configuration.
- **ModelServer** – Serves REST/gRPC endpoints and specifies exposure/auth rules.
- **ModelInfer** – Defines inference groups, replicas, and runtime configuration.
- **AutoScalingPolicy** – Policy definition for autoscaling triggers/metrics.
- **AutoScalingPolicyBinding** – Connects Models with AutoscalingPolicies.

Operators update these CRDs, which are reconciled by Control Plane controllers.

---

### 2. **Control Plane**

The control plane ensures declarative configs are realized and user requests are orchestrated through the data plane.

#### **Controllers**

- **Model Controller** → Watches `Model` CRDs and configures model state.
- **ModelRoute Controller** → Syncs routes into the inference gateway.
- **ModelServer Controller** → Manages serving surfaces and connectivity.
- **ModelInfer Controller** → Manages inference groups, roles, Replica definitions.
- **Autoscaler Controller** → Collects metrics from Pods, evaluates scaling via attached policies.

#### **Infer Gateway**

Handles user requests with the following pipeline:

`User → Auth → Rate Limiting → Fairness Scheduling → Load Balancing → Proxy → Inference Pods`

- **Auth** → Authentication and authorization.
- **Rate Limiting** → Ensures request throughput safety.
- **Fairness Scheduling** → Queueing and fair allocation.
- **Load Balancing** → Balances to the optimal backend.
- **Proxy** → Dispatches into data plane groups.

#### **Scheduler**

Applies **advanced scheduling plugins** to optimize request placement:

- **Filter Plugins:**
    - *Least Requests*
    - *LoRA Affinity*

- **Score Plugins:**
    - *KV Cache Aware*
    - *Least Latency (TTFT & TPOT)*
    - *Prefix Cache*
    - *GPU Cache*

The scheduler integrates with Load Balancing and Fairness Scheduling.

---

### 3. **Data Plane**

Executes inference with optimized, role-based pods.

#### **Inference Pods**

Organized as **Groups** with **multiple Replicas**. Each replica may assume a **role**:

- **Role A – Prefill**: Handles prompt initialization / warm‑up.
- **Role B – Decode**: Handles incremental token generation.

Each Replica may include:

- **Entry Pod** – Ingress for requests into its role.
- **Worker Pod(s)** – Execute model inference.
- **Init Container** – Dependency/artifact setup before execution.
- **Sidecar Container** – Logging, observability, or networking side processes.
- **Downloader** – Fetches weights and model artifacts.
- **Runtime Agent** – Health / metrics collector.
- **LLM Engines** – Integrations with backends (e.g., **vLLM**, **SGLang**).

This design supports **Prefill/Decode Disaggregation (PD mode)** for efficient scaling across stages of inference.

---

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

- **CRD-driven management** for models, routes, and scaling.
- **Control Plane Orchestration** with dedicated controllers and automation.
- **Infer Gateway** with full request pipeline including fairness scheduling and queue integration.
- **Advanced Scheduling Plugins** (latency-aware, cache-aware, LoRA affinity).
- **PD Disaggregation**: Distinction of Prefill vs Decode workloads.
- **Inference Groups & Role-Based Replicas** enabling scaling granularity.
- **Observability & Reliability via Init, Sidecar, Runtime Agent containers**.
- **Flexible Scaling** via metric-driven autoscaler policies.