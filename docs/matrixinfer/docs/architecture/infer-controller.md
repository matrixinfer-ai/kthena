# Infer Controller

Infer Controller is the controller for the inference workload `ModelInfer` in MatrixInfer, which is used to reconcile `ModelInfer` resources and manage the lifecycle of inference pods.

## ModelInfer Overview

`ModelInfer` represents the optimal deployment paradigm for distributed inference scenarios involving large models, offering flexible and user-friendly workload configurations with
Prefilling Decoding Disaggregation and parallel inference services like Pipeline Parallelism (PP) and Tensor Parallelism (TP).

## ModelInfer Architecture

![modelinfer.svg](../../static/img/modelinfer.svg)

The Custom Resource Definition (CRD) of `ModelInfer` is primarily divided into three layers, namely:

1. ModelInfer

`ModelInfer` is a novel type of workload designed to define specific inference services. It manages a set of `InferGroup` inference instances with consistent configurations

- Supports topology aware and gang scheduling: It enables the simultaneous scheduling of inference pods within an `InferGroup` to the same **HyperNode**, and allows for the configuration of gang scheduling parameters.
  Scheduling is only permitted when the HyperNode meets the **minimum number** of pods required for the inference tasks.
- Supports scaling and rolling upgrades: It provides scaling capabilities at both the `InferGroup` level and `Role` level, along with fault recovery capabilities. The current controller supports sequential rolling upgrades for InferGroups.

2. InferGroup

`InferGroup` is a group of inference instances, representing the smallest unit capable of independently completing a single inference service.

- Supports defining multiple inference roles: Based on `Role` to represent inference roles such as **Prefill** and **Decode**, enabling the management of complex inference scenarios like xPyD configurations.
- Supports graceful reconstruction: During the execution of inference tasks, if a failure occurs, the system allows a configurable grace period for pods recovery before triggering rebuilding, minimizing service interruption.

3. Role

`Role` represents the smallest functional unit, which can correspond to specific instance types such as Prefill, Decode, Aggregated.

- Supports double-Pod templates: Within a single `Role` instance, two distinct Pod templates can be defined as `Entry` Pod and `Worker` Pod. 
`Entry` Pod serves as the entry point to receive inference requests and distribute tasks, while the `Worker` Pod is responsible for executing the actual inference computations.
- Supports network topology aware scheduling: Enables the co-location scheduling of `Entry` Pods and `Worker` Pods within the same `Role` to the same **HyperNode**.

## Example

Read the [examples](https://github.com/matrixinfer-ai/matrixinfer/blob/main/examples/model-infer/sample.yaml) to learn more.

## Labels and Environment Variables

### Labels

| Key                                  | Description                              | Example    | Applies to  |
|--------------------------------------|------------------------------------------|------------|-------------|
| modelinfer.matrixinfer.ai/name       | The label key for the ModelInfer name    | sample     | pod         |
| modelinfer.matrixinfer.ai/group-name | The label key for the InferGroup name    | sample-0   | pod,service |
| modelinfer.matrixinfer.ai/role       | The label key for the Role name          | decode     | pod,service |
| modelinfer.matrixinfer.ai/role-id    | The label key for the role serial number | decode-0   | pod,service |
| modelinfer.matrixinfer.ai/revision   | The revision label for the model infer   | 67b8d4b8c7 | pod         |
| modelinfer.matrixinfer.ai/entry      | The entry pod label key                  | true       | pod         |


### Environment Variables

| Key           | Description                                                         | Example                                         | Applies to |
|---------------|---------------------------------------------------------------------|-------------------------------------------------|------------|
| GROUP_SIZE    | The environment variable for the inference Role instance size       | 4                                               | pod        |
| ENTRY_ADDRESS | The address of the Entry via the headless service                   | sample-0-decode-0-0.sample-0-decode-0-0.default | pod        |
| WORKER_INDEX  | The index or identity of the pod within the inference Role instance | 0                                               | pod        |
