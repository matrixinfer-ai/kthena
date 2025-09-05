import LightboxImage from '@site/src/components/LightboxImage';

# Model Controller

## Overview

As an optional component of the MatrixInfer project, the Model Controller primarily provides users with a convenient deployment form - `Model`. Based on the `Model CR` (Model Custom Resource) information provided by users, this component can automatically configure and deploy the components required for inference services, such as gateway routing rules, inference engine instances, and dynamic scaling configurations.

## Feature Description

The Model Controller provides one-stop deployment capabilities, enabling users to only need to be aware of `Model` information while ignoring the complex interaction specifications between various components within ModelInfer, such as the use of some built-in labels and the binding relationships between CRDs. The following diagram illustrates the interaction principle of the Model Controller:

<LightboxImage src="/img/diagrams/architecture/model-controller-architecture.svg" alt="Architecture Overview"></LightboxImage>

The Model Controller subscribes to the creation/modification/deletion events of the `Model` CR and synchronizes them to the CRs of `ModelServer`, `ModelRoute`, `ModelInfer`, `AutoscalingPolicy`, and `AutoscalingPolicyBinding` that have the same semantic content as the corresponding events. All these CRs will carry the label `registry.matrixinfer.ai/managed-by=registry.matrixinfer.ai`.

## Example

Read the [examples](https://github.com/matrixinfer-ai/matrixinfer/blob/main/examples/model/) to learn more.

## Limitations

The `Model` can only cover most inference service scenarios and will be continuously updated. Some scenarios are not yet supported and require manual configuration of relevant CRDs. 

Please note the following limitations when using the Model Controller:
- Each `Model` can create only one `Model Route`.
- Rate limiting for `Model Route` is not supported.
- Topology configuration for `Model Infer` is not supported.
- The `panicPolicy` configuration for `AutoscalingPolicy` is not supported.
- Behavior configuration for `AutoscalingPolicy` is not supported.

In these cases, you can manually create `Model Infer`, `Model Server`, `Model Route`, `AutoscalingPolicy`, `AutoscalingPolicyBinding` resources as needed.