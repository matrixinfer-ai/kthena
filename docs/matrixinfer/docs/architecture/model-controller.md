# Model Controller

## Overview

![model-controller.png](../../static/img/model-controller-architecture.svg)

The Model Controller is a component designed to help you manage `Model Infer`, `Model Server`, `Model Route`,
`AutoscalingPolicy`,
`AutoscalingPolicyBinding` resources based on the `Model` Custom Resource (CR) you define.

## Limitations

Please note the following limitations when using the Model Controller:

- Each `Model` can create only one `Model Route`.
- Rate limiting for `Model Route` is not supported.
- Topology configuration for `Model Infer` is not supported.
- The `panicPolicy` configuration for `AutoscalingPolicy` is not supported.
- Behavior configuration for `AutoscalingPolicy` is not supported.

In these cases, you can manually create `Model Infer`, `Model Server`, `Model Route`, `AutoscalingPolicy`,
`AutoscalingPolicyBinding` resources as needed.