# Model Controller

## Introduction

Model Controller is a component that manages the lifecycle of `Model` CR(Custom Resource) in kubernetes. It is
responsible for creating,
updating, and deleting `Model`, `Model Infer`, `Model Server`, `Model Route`, `AutoscalingPolicy`,
`AutoscalingPolicyBinding` based on the `Model` CR that user defined. The namespace of these resources is same with
`Model`.

There may be multiple `backends` in `Model` CR, each `backend` stands for a model that can be used for inference. Each
`backend` has its own `Model Infer`. And `backend` has several `workers` that are used to run the inference.

### The rules of generated resource name

- The name of the `Model Infer` is in the format of `<model-name>-<index>-<backend-type>-instance`.
- The name of the
  `Model Server` is in the format of
  `<model-name>-<index>-<backend-type>-server`.
- The `Model Route` name is in the format of `<model-name>`.
- The `AutoscalingPolicy` name is in the format of `<model-name>` if in the model level or `<model-name>-<backend-name>`
  if in the backend level.
- The `AutoscalingPolicyBinding` name is same with `AutoscalingPolicy` name.

For example, create a `Model` named `test-model` with two backends, one is `backend1` and the other is `backend2`, both
backend types are `vLLM`, then
the name of the generated `Model Infer` will be `test-model-0-vllm-instance` and `test-model-1-vllm-instance`, the
name of the generated `Model Server` will be `test-model-0-vllm-server` and `test-model-1-vllm-server`, and the
name of the generated `Model Route` will be `test-model`. If `AutoscalingPolicy` is defined in the model level, the name
will be `test-model`, otherwise the name will be `test-model-backend1` and `test-model-backend2`.

### Conditions of Model CR

`Model` CR has several conditions that indicate the status of the model. These conditions are:

| ConditionType | Explain                                                                     |
|---------------|-----------------------------------------------------------------------------|
| `Initialized` | Model CR has passed webhook check, just wait for starting                   |
| `Active`      | Model is ready for use                                                      |
| `Failed`      | Model runs failed due to some reasons, we can get more details from message |

### How Model Controller works

Model Controller watches for changes to `Model` CR in the Kubernetes cluster. When a `Model` CR is created or updated,
the controller performs the following steps:

1. Convert the `Model` CR to `Model Infer` CR, `Model Server` CR, `Model Route` CR. `AutoscalingPolicy` CR and
   `AutoscalingPolicyBinding` CR are optional, only created when the `Model` CR has `autoscalingPolicy` defined.
2. Use the result of step 1 to create or update the `Model Infer`, `Model Server`, `Model Route` , `AutoscalingPolicy`,
   `AutoscalingPolicyBinding`in the Kubernetes.
3. Set the conditions of `Model` CR.
    - After creating the related resources, the `Initialized` condition is set to `true`.
    - The controller then monitors the status of the `ModelInfer` resources. Once all `ModelInfer` resources are `Available`, the `Active` condition on the `Model` is set to `true`.
    - If any error occurs during the process, set the `Failed` condition to true and provide an error message.

The `OwnerReference` is set to the `Model` CR for all the created resources, so that when the `Model` CR is deleted, all
the related resources will be deleted as well.

## Examples of Model CR

You can find examples of model CR [here](https://github.com/matrixinfer-ai/matrixinfer/tree/main/examples/model)