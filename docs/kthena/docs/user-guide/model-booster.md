# Model Booster

### The rules of generated resource name

- The name of the `Model Infer` is in the format of `<model-name>-<backend-name>`.
- The name of the
  `Model Server` is in the format of
  `<model-name>-<backend-name>`.
- The `Model Route` name is in the format of `<model-name>`.
- The `AutoscalingPolicy` name is in the format of `<model-name>` if in the model level or `<model-name>-<backend-name>`
  if in the backend level.
- The `AutoscalingPolicyBinding` name is same with `AutoscalingPolicy` name.

For example, create a `Model` named `test-model` with two backends, one is `backend1` and the other is `backend2`, both
backend types are `vLLM`, then
the name of the generated `Model Infer` will be `test-model-backend1` and `test-model-backend2`, the
name of the generated `Model Server` will be `test-model-backend1` and `test-model-backend2`, and the
name of the generated `Model Route` will be `test-model`. If `AutoscalingPolicy` is defined in the model level, the name
will be `test-model`, otherwise the name will be `test-model-backend1` and `test-model-backend2`.

### How Model Controller works

Model Controller watches for changes to `Model` CR in the Kubernetes cluster. When a `Model` CR is created or updated,
the controller performs the following steps:

1. Convert the `Model` CR to `Model Infer` CR, `Model Server` CR, `Model Route` CR. `AutoscalingPolicy` CR and
   `AutoscalingPolicyBinding` CR are optional, only created when the `Model` CR has `autoscalingPolicy` defined.
2. Use the result of step 1 to create or update the `Model Infer`, `Model Server`, `Model Route` , `AutoscalingPolicy`,
   `AutoscalingPolicyBinding`in the Kubernetes.
3. Set the conditions of `Model` CR.
    - After creating the related resources, the `Initialized` condition is set to `true`.
    - The controller then monitors the status of the `ModelInfer` resources. Once all `ModelInfer` resources are
      `Available`, the `Active` condition on the `Model` is set to `true`.
    - If any error occurs during the process, set the `Failed` condition to true and provide an error message.

The `OwnerReference` is set to the `Model` CR for all the created resources, so that when the `Model` CR is deleted, all
the related resources will be deleted as well.

## Model Lifecycle

`Model` CR has several conditions that indicate the status of the model. These conditions are:

| ConditionType | Description                                                          |
|---------------|----------------------------------------------------------------------|
| `Initialized` | The Model CR has been validated and is waiting to be processed.      |
| `Active`      | The Model is ready for inference.                                    |
| `Failed`      | The Model failed to become active. See the message for more details. |

## Examples of Model CR

You can find examples of model CR [here](/examples/model/examples/model)
You can find examples of model CR [here](/examples/model-booster)

## Advanced features

### Gang Scheduling

`GangSchedule` is disabled by default, if you want to enable it,
see [here](multi-node-inference.md#gang-scheduling-and-network-topology)
