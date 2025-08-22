# Model Controller

## Overview

![model-controller.png](../../static/img/model-controller-architecture.svg)

Model Controller is a component that help you manage `Model Infer`, `Model Server`, `Model Route`, `AutoscalingPolicy`,
`AutoscalingPolicyBinding` based on the `Model` CR that you defined.

However, there are some limitations when you use Model Controller:

- One `Model` can only create one `Model Route`.
- Don't support configure `Model Route` rate limit.
- Don't support configure `Model Infer` topology.
- Don't support configure `AutoscalingPolicy` panicPolicy.
- Don't support configure `AutoscalingPolicy` behavior.

You can create these resources directly in such condition.