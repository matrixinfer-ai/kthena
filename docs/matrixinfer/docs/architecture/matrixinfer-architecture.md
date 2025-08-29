# Architecture Overview

MatrixInfer is a Kubernetes-native AI inference platform designed for scalable, efficient, and intelligent model serving.
The system is logically separated into three planes to decouple management, orchestration, and actual inference execution.

import LightboxImage from '@site/src/components/LightboxImage';

## High-Level Architecture

---

The MatrixInfer architecture consists of three main planes:

- **Management Plane**: Defines models, routes, servers, and autoscaling policies as Kubernetes Custom Resources (CRDs).
- **Control Plane**: Responsible for lifecycle management, orchestration, routing, auth, scheduling, and scaling controllers.
- **Data Plane**: Executes inference workloads using groups of inference pods integrated with KV connectors.

<LightboxImage src="/img/diagrams/architecture/architecture_overview.svg" alt="Architecture Overview" />


## Core Components

---

### 1. Management Plane

The management plane extends Kubernetes with the following **Custom Resources (CRDs):**

- **Model** – Represents metadata and specs for models
- **ModelRoute** – Configures routing rules and traffic management
- **ModelServer** – Defines model serving endpoints and security rules
- **ModelInfer** – Describes inference deployments (replicas, resources)
- **AutoScalingPolicy** – Defines scaling policies
- **AutoScalingPolicyBinding** – Binds models to scaling policies

These CRs provide the declarative configuration, which is reconciled by the Control Plane controllers.


### 2. Control Plane

The control plane manages orchestration and ensures that user requests, CRDs, and system state are consistent.
It consists of three major modules:

#### **Inference Gateway**

Entry for user requests. Components:

- **Auth** – Authentication and authorization for requests
- **Rate Limiting** – Protects pods from overload
- **Fairness Scheduling** – Ensures fair queueing and scheduling of requests
- **Load Balancing** – Balances requests across inference pods
- **Proxy** – Forwards requests into the Data Plane

Request Path (per diagram):
`User → Auth → Rate Limiting → Fairness Scheduling → Load Balancing → Proxy → Inference Pods`

#### **Scheduler**

Implements advanced pod scheduling strategies. Plugins visualized in the diagram:

- **Least Requests** (traffic balance)
- **LoRA Affinity**
- **KV Cache Awareness**
- **Least Latency (TTFT & TPOT)**
- **Prefix Cache**
- **GPU Cache**
- **PD Group Scheduling (Prefill/Decode disaggregation)**

The scheduler integrates tightly with the load balancer and fairness scheduling in the gateway.

#### **Controllers**

Operators or automation modify CRDs, and controllers reconcile that intent:

- **Model Controller** → Manages models
- **ModelRoute Controller** → Syncs routing configs
- **ModelServer Controller** → Manages serving endpoints
- **ModelInfer Controller** → Manages inference replicas and InferGroups
- **Autoscaler Controller** → Monitors pod metrics, enforces AutoScalingPolicies

Each controller reads CRDs from the Management Plane and materializes resources in the Data Plane.


### 3. Data Plane

Executes AI model inference with high efficiency.

#### **Inference Pods**

Organized into **Inference Groups (Group 0, Group 1, …)** with multiple **Replicas**. Each replica may play a specific role:

- **Role A – Prefill** (handles sequence initialization)
- **Role B – Decode** (handles incremental token generation)

Each Pod Replica typically includes:

- **Entry Pod** – handles request ingress into the group
- **Worker Pod(s)** – execute model inference workloads
- **Init Container** – initializes environment and artifacts
- **Sidecar Container** – auxiliary processes for observability/networking
- **Downloader** – pulls model weights/artifacts at startup
- **Runtime Agent** – collects metrics, manages local lifecycle
- **Model Engines** – `vLLM`, `SGLang` integrations

This design enables **PD-disaggregation** (separating Prefill and Decode roles) and supports dynamic scaling.

#### **KV Connector**

Provides fast access to external key-value systems for caching. Supports:

- **Nixl** – High-performance KV store
- **MoonCake** – Distributed caching cluster
- **LMCache** – Large model caching support
- **HTTP-based stores** – Generic KV backend

KV Connector enables **prefix cache re-use** and **GPU cache integration** for acceleration.


## Data Flows

---

### 1. Model & Scaling Flow

```
Operator → Model CRDs → Controllers → AutoScalingPolicy/Binding → ModelServer/Route CRDs
```

### 2. Inference Deployment Flow

```
Autoscaler Controller → ModelInfer CRD → ModelInfer Controller → Inference Pods
```

### 3. Request Serving Flow

```
User → Auth → Rate Limiter → Fairness Scheduling → Load Balancer → Proxy → Inference Pods (Prefill / Decode)
```

### 4. Data Synchronization Flow

```
Controllers → In-Memory Datastores ← Model Router/Scheduler
Autoscaler → Inference Pods (metrics collection)
```


## Key Features

---

- **Three-plane design**: Management Plane (CRDs), Control Plane (controllers/schedulers), Data Plane (pods & caches)
- **Fairness + Advanced Scheduling**: Supports fairness scheduling, cache-aware, latency-aware, lora-affinity filters
- **Scalable Pod Architecture**: Entry vs Worker pods, Prefill/Decode roles, multiple replicas per group
- **Rich KV Cache Integration**: Multi-backend connectivity (LMCache, MoonCake, Nixl, HTTP)
- **Metric-driven autoscaling** with user-defined policies
- **PD-Disaggregation**: Prefill/Decode group separation
- **Extensible with sidecars, init containers, runtime agents for observability & reliability**
