# Model Booster

### The rules of generated resource name

- The name of the `ModelServing` is in the format of `<model-name>-<backend-name>`.
- The name of the
  `ModelServer` is in the format of
  `<model-name>-<backend-name>`.
- The `ModelRoute` name is in the format of `<model-name>`.
- The `AutoscalingPolicy` name is in the format of `<model-name>` if in the model level or `<model-name>-<backend-name>`
  if in the backend level.
- The `AutoscalingPolicyBinding` name is same with `AutoscalingPolicy` name.

For example, create a `ModelBooster` named `test-model` with two backends, one is `backend1` and the other is `backend2`, both
backend types are `vLLM`, then
the name of the generated `ModelServing` will be `test-model-backend1` and `test-model-backend2`, the
name of the generated `ModelServer` will be `test-model-backend1` and `test-model-backend2`, and the
name of the generated `ModelRoute` will be `test-model`. If `AutoscalingPolicy` is defined in the model level, the name
will be `test-model`, otherwise the name will be `test-model-backend1` and `test-model-backend2`.

### How Model Booster Controller works

Model Booster Controller watches for changes to `ModelBooster` CR in the Kubernetes cluster. When a `ModelBooster` CR is created or updated,
the controller performs the following steps:

1. Convert the `ModelBooster` CR to `ModelServing` CR, `ModelServer` CR, `ModelRoute` CR. `AutoscalingPolicy` CR and
   `AutoscalingPolicyBinding` CR are optional, only created when the `ModelBooster` CR has `autoscalingPolicy` defined.
2. Use the result of step 1 to create or update the `ModelServing`, `ModelServer`, `ModelRoute` , `AutoscalingPolicy`,
   `AutoscalingPolicyBinding`in the Kubernetes.
3. Set the conditions of `ModelBooster` CR.
    - After creating the related resources, the `Initialized` condition is set to `true`.
    - The controller then monitors the status of the `ModelServing` resources. Once all `ModelServing` resources are
      `Available`, the `Active` condition on the `ModelBooster` is set to `true`.
    - If any error occurs during the process, set the `Failed` condition to true and provide an error message.

The `OwnerReference` is set to the `ModelBooster` CR for all the created resources, so that when the `ModelBooster` CR is deleted, all
the related resources will be deleted as well.

## ModelBooster vs ModelServing Deployment Approaches

Kthena provides two approaches for deploying LLM inference workloads: the **ModelBooster approach** and the **ModelServing approach**. This section compares both approaches to help you choose the right one for your use case.

### Deployment Approach Comparison

| Deployment Method | Manually Created CRDs                 | Automatically Managed Components        | Use Case                                     |
|-------------------|---------------------------------------|-----------------------------------------|----------------------------------------------|
| **ModelBooster**  | ModelBooster                          | ModelServer, ModelRoute, Pod Management | Simplified deployment, automated management  |
| **ModelServing**  | ModelServing, ModelServer, ModelRoute | Pod Management                          | Fine-grained control, complex configurations |

### ModelBooster Approach

**Advantages:**

- Simplified configuration with built-in disaggregation support optimized for NPUs
- Automatic KV cache transfer configuration using NPU-optimized protocols
- Integrated support for Huawei Ascend NPUs with automatic resource allocation
- Streamlined deployment process with NPU-specific optimizations
- Built-in HCCL (Huawei Collective Communication Library) configuration

**Automatically Managed Components:**

- ✅ ModelServer (automatically created and managed with NPU awareness)
- ✅ ModelRoute (automatically created and managed)
- ✅ Inter-service communication configuration (HCCL-optimized)
- ✅ Load balancing and routing for NPU workloads
- ✅ NPU resource scheduling and allocation

**User Only Needs to Create:**

- ModelBooster CRD with NPU resource specifications

### ModelServing Approach

**Advantages:**

- Fine-grained control over NPU container configuration
- Support for init containers and complex volume mounts for NPU drivers
- Detailed environment variable configuration for Ascend NPU settings
- Flexible NPU resource allocation (`huawei.com/ascend-1980`)
- Custom HCCL network interface configuration

**Manually Created Components:**

- ❌ ModelServing CRD with NPU resource specifications
- ❌ ModelServer CRD with NPU-aware workload selection
- ❌ ModelRoute CRD for NPU service routing
- ❌ Manual inter-service communication configuration (HCCL settings)

**NPU-Specific Networking Components:**

- **ModelServer** - Manages inter-service communication and load balancing for NPU workloads
- **ModelRoute** - Provides request routing and traffic distribution to NPU services
- **Supported KV Connector Types** - nixl, mooncake (optimized for NPU communication)
- **HCCL Integration** - Huawei Collective Communication Library for NPU-to-NPU communication

### Selection Guidance

- **Recommended: Use ModelBooster Approach** - Suitable for most NPU deployment scenarios, simple deployment, high automation with NPU optimization
- **Use ModelServing Approach** - Only when fine-grained NPU control or special Ascend-specific configurations are required

## ModelBooster Lifecycle

`ModelBooster` CR has several conditions that indicate the status of the model. These conditions are:

| ConditionType | Description                                                          |
|---------------|----------------------------------------------------------------------|
| `Initialized` | The ModelBooster CR has been validated and is waiting to be processed.      |
| `Active`      | The ModelBooster is ready for inference.                                    |
| `Failed`      | The ModelBooster failed to become active. See the message for more details. |

## Examples of ModelBooster CR

You can find examples of model CR [here](https://github.com/matrixinfer-ai/matrixinfer/tree/main/examples/model-booster)

## Advanced features

### Gang Scheduling

`GangPolicy` is disabled by default, if you want to enable it,
see [here](multi-node-inference.md#gang-scheduling-and-network-topology)
