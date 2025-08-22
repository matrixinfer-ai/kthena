# Model Controller

## Overview

![model-controller.png](../../static/img/model-controller-architecture.svg)

Model Controller is a component that manages the lifecycle of `Model` CR(Custom Resource) in kubernetes. It is
responsible for creating,
updating, and deleting `Model`, `Model Infer`, `Model Server`, `Model Route`, `AutoscalingPolicy`,
`AutoscalingPolicyBinding` based on the `Model` CR that user defined. Resources created by the Model Controller have
some
limits:

- One `Model` can only create one `Model Route`.
- Does not support `Model Route` rate limit.
- Does not support `Model Infer` topology.

You can create these resources directly in such condition.