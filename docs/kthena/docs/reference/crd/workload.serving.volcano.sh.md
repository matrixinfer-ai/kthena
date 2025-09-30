# API Reference

## Packages
- [workload.serving.volcano.sh/v1alpha1](#workloadservingvolcanoshv1alpha1)


## workload.serving.volcano.sh/v1alpha1


### Resource Types
- [AutoscalingPolicy](#autoscalingpolicy)
- [AutoscalingPolicyBinding](#autoscalingpolicybinding)
- [AutoscalingPolicyBindingList](#autoscalingpolicybindinglist)
- [AutoscalingPolicyList](#autoscalingpolicylist)
- [ModelBooster](#modelbooster)
- [ModelBoosterList](#modelboosterlist)
- [ModelServing](#modelserving)
- [ModelServingList](#modelservinglist)



#### AutoscalingPolicy



AutoscalingPolicy is the Schema for the autoscalingpolicies API.



_Appears in:_
- [AutoscalingPolicyList](#autoscalingpolicylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `AutoscalingPolicy` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[AutoscalingPolicySpec](#autoscalingpolicyspec)_ |  |  |  |
| `status` _[AutoscalingPolicyStatus](#autoscalingpolicystatus)_ |  |  |  |


#### AutoscalingPolicyBehavior



AutoscalingPolicyBehavior defines the scaling behaviors for up and down actions.



_Appears in:_
- [AutoscalingPolicySpec](#autoscalingpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `scaleUp` _[AutoscalingPolicyScaleUpPolicy](#autoscalingpolicyscaleuppolicy)_ | ScaleUp defines the policy for scaling up (increasing replicas). |  |  |
| `scaleDown` _[AutoscalingPolicyStablePolicy](#autoscalingpolicystablepolicy)_ | ScaleDown defines the policy for scaling down (decreasing replicas). |  |  |


#### AutoscalingPolicyBinding



AutoscalingPolicyBinding is the Schema for the autoscalingpolicybindings API.



_Appears in:_
- [AutoscalingPolicyBindingList](#autoscalingpolicybindinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `AutoscalingPolicyBinding` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[AutoscalingPolicyBindingSpec](#autoscalingpolicybindingspec)_ |  |  |  |
| `status` _[AutoscalingPolicyBindingStatus](#autoscalingpolicybindingstatus)_ |  |  |  |


#### AutoscalingPolicyBindingList



AutoscalingPolicyBindingList contains a list of AutoscalingPolicyBinding.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `AutoscalingPolicyBindingList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[AutoscalingPolicyBinding](#autoscalingpolicybinding) array_ |  |  |  |


#### AutoscalingPolicyBindingSpec



AutoscalingPolicyBindingSpec defines the desired state of AutoscalingPolicyBinding.



_Appears in:_
- [AutoscalingPolicyBinding](#autoscalingpolicybinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `policyRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | PolicyRef references the autoscaling policy to be optimized scaling base on multiple targets. |  |  |
| `optimizerConfiguration` _[OptimizerConfiguration](#optimizerconfiguration)_ | It dynamically adjusts replicas across different ModelServing objects based on overall computing power requirements - referred to as "optimize" behavior in the code.<br />For example:<br />When dealing with two types of ModelServing objects corresponding to heterogeneous hardware resources with different computing capabilities (e.g., H100/A100), the "optimize" behavior aims to:<br />Dynamically adjust the deployment ratio of H100/A100 instances based on real-time computing power demands<br />Use integer programming and similar methods to precisely meet computing requirements<br />Maximize hardware utilization efficiency |  |  |
| `scalingConfiguration` _[ScalingConfiguration](#scalingconfiguration)_ | Adjust the number of related instances based on specified monitoring metrics and their target values. |  |  |


#### AutoscalingPolicyBindingStatus



AutoscalingPolicyBindingStatus defines the status of a autoscaling policy binding.



_Appears in:_
- [AutoscalingPolicyBinding](#autoscalingpolicybinding)



#### AutoscalingPolicyList



AutoscalingPolicyList contains a list of AutoscalingPolicy.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `AutoscalingPolicyList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[AutoscalingPolicy](#autoscalingpolicy) array_ |  |  |  |


#### AutoscalingPolicyMetric



AutoscalingPolicyMetric defines a metric and its target value for scaling.



_Appears in:_
- [AutoscalingPolicySpec](#autoscalingpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metricName` _string_ | MetricName is the name of the metric to monitor. |  |  |
| `targetValue` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api)_ | TargetValue is the target value for the metric to trigger scaling. |  |  |


#### AutoscalingPolicyPanicPolicy



AutoscalingPolicyPanicPolicy defines the policy for panic scaling up.



_Appears in:_
- [AutoscalingPolicyScaleUpPolicy](#autoscalingpolicyscaleuppolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `percent` _integer_ | Percent is the maximum percentage of instances to scale up. | 1000 | Maximum: 1000 <br />Minimum: 0 <br /> |
| `period` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | Period is the duration over which scaling down is evaluated. |  |  |
| `panicThresholdPercent` _integer_ | PanicThresholdPercent is the threshold percent to enter panic mode. | 200 | Maximum: 1000 <br />Minimum: 110 <br /> |
| `panicModeHold` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | PanicModeHold is the duration to hold in panic mode before returning to normal. | 60s |  |


#### AutoscalingPolicyScaleUpPolicy







_Appears in:_
- [AutoscalingPolicyBehavior](#autoscalingpolicybehavior)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `stablePolicy` _[AutoscalingPolicyStablePolicy](#autoscalingpolicystablepolicy)_ | Stable policy usually makes decisions based on the average value of metrics calculated over the past few minutes and introduces a scaling-down cool-down period/delay.<br />This mechanism is relatively stable, as it can smooth out short-term small fluctuations and avoid overly frequent and unnecessary Pod scaling. |  |  |
| `panicPolicy` _[AutoscalingPolicyPanicPolicy](#autoscalingpolicypanicpolicy)_ | When the load surges sharply within a short period (for example, encountering a sudden traffic peak or a rush of sudden computing tasks),<br />using the average value over a long time window to calculate the required number of replicas will cause significant lag.<br />If the system needs to scale out quickly to cope with such peaks, the ordinary scaling logic may fail to respond in time,<br />resulting in delayed Pod startup, slower service response time or timeouts, and may even lead to service paralysis or data backlogs (for workloads such as message queues). |  |  |


#### AutoscalingPolicySpec



AutoscalingPolicySpec defines the desired state of AutoscalingPolicy.



_Appears in:_
- [AutoscalingPolicy](#autoscalingpolicy)
- [ModelBackend](#modelbackend)
- [ModelBoosterSpec](#modelboosterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tolerancePercent` _integer_ | TolerancePercent is the percentage of deviation tolerated before scaling actions are triggered.<br />The current number of instances is current_replicas, and the expected number of instances inferred from monitoring metrics is target_replicas.<br />The scaling operation will only be actually performed when \|current_replicas - target_replicas\| >= current_replicas * TolerancePercent. | 10 | Maximum: 100 <br />Minimum: 0 <br /> |
| `metrics` _[AutoscalingPolicyMetric](#autoscalingpolicymetric) array_ | Metrics is the list of metrics used to evaluate scaling decisions. |  | MinItems: 1 <br /> |
| `behavior` _[AutoscalingPolicyBehavior](#autoscalingpolicybehavior)_ | Behavior defines the scaling behavior for both scale up and scale down. |  |  |


#### AutoscalingPolicyStablePolicy



AutoscalingPolicyStablePolicy defines the policy for stable scaling up or scaling down.



_Appears in:_
- [AutoscalingPolicyBehavior](#autoscalingpolicybehavior)
- [AutoscalingPolicyScaleUpPolicy](#autoscalingpolicyscaleuppolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `instances` _integer_ | Instances is the maximum number of instances to scale. | 1 | Minimum: 0 <br /> |
| `percent` _integer_ | Percent is the maximum percentage of instances to scaling. | 100 | Maximum: 1000 <br />Minimum: 0 <br /> |
| `period` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | Period is the duration over which scaling is evaluated. | 15s |  |
| `selectPolicy` _[SelectPolicyType](#selectpolicytype)_ | SelectPolicy determines the selection strategy for scaling up (e.g., Or, And).<br />'Or' represents the scaling operation will be performed as long as either the Percent requirement or the Instances requirement is met.<br />'And' represents the scaling operation will be performed as long as both the Percent requirement and the Instances requirement is met. | Or | Enum: [Or And] <br /> |
| `stabilizationWindow` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | StabilizationWindow is the time window to stabilize scaling up actions. |  |  |


#### AutoscalingPolicyStatus



AutoscalingPolicyStatus defines the observed state of AutoscalingPolicy.



_Appears in:_
- [AutoscalingPolicy](#autoscalingpolicy)





#### GangPolicy



GangPolicy defines the gang scheduling configuration.



_Appears in:_
- [ServingGroup](#servinggroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minRoleReplicas` _object (keys:string, values:integer)_ | MinRoleReplicas defines the minimum number of replicas required for each role<br />in gang scheduling. This map allows users to specify different<br />minimum replica requirements for different roles.<br />Key: role name<br />Value: minimum number of replicas required for that role |  |  |


#### LoraAdapter



LoraAdapter defines a LoRA (Low-Rank Adaptation) adapter configuration.



_Appears in:_
- [ModelBackend](#modelbackend)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the LoRA adapter. |  | Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `artifactURL` _string_ | ArtifactURL is the URL where the LoRA adapter artifact is stored. |  | Pattern: `^(hf://\|s3://\|pvc://).+` <br /> |


#### Metadata



Metadata is a simplified version of ObjectMeta in Kubernetes.



_Appears in:_
- [PodTemplateSpec](#podtemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ | Map of string keys and values that can be used to organize and categorize<br />(scope and select) objects. May match selectors of replication controllers<br />and services.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations is an unstructured key value map stored with a resource that may be<br />set by external tools to store and retrieve arbitrary metadata. They are not<br />queryable and should be preserved when modifying objects.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations |  |  |


#### MetricEndpoint







_Appears in:_
- [Target](#target)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `uri` _string_ | The metric uri, e.g. /metrics | /metrics |  |
| `port` _integer_ | The port of pods exposing metric endpoints | 8100 |  |


#### ModelBackend



ModelBackend defines the configuration for a model backend.



_Appears in:_
- [ModelBoosterSpec](#modelboosterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the backend. Can't duplicate with other ModelBackend name in the same ModelBooster CR.<br />Note: update name will cause the old modelInfer deletion and a new modelInfer creation. |  | Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `type` _[ModelBackendType](#modelbackendtype)_ | Type is the type of the backend. |  | Enum: [vLLM vLLMDisaggregated SGLang MindIE MindIEDisaggregated] <br /> |
| `modelURI` _string_ | ModelURI is the URI where you download the model. Support hf://, s3://, pvc://. |  | Pattern: `^(hf://\|s3://\|pvc://).+` <br /> |
| `cacheURI` _string_ | CacheURI is the URI where the downloaded model stored. Support hostpath://, pvc://. |  | Pattern: `^(hostpath://\|pvc://).+` <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#envfromsource-v1-core) array_ | List of sources to populate environment variables in the container.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the container is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#envvar-v1-core) array_ | List of environment variables to set in the container.<br />Supported names:<br />"ENDPOINT": When you download model from s3, you have to specify it.<br />"RUNTIME_URL": default is http://localhost:8000<br />"RUNTIME_PORT": default is 8100<br />"RUNTIME_METRICS_PATH": default is /metrics<br />"HF_ENDPOINT":The url of hugging face. Default is https://huggingface.co/<br />Cannot be updated. |  |  |
| `minReplicas` _integer_ | MinReplicas is the minimum number of replicas for the backend. |  | Maximum: 1e+06 <br />Minimum: 0 <br /> |
| `maxReplicas` _integer_ | MaxReplicas is the maximum number of replicas for the backend. |  | Maximum: 1e+06 <br />Minimum: 1 <br /> |
| `scalingCost` _integer_ | ScalingCost is the cost associated with running this backend. |  | Minimum: 0 <br /> |
| `routeWeight` _integer_ | RouteWeight is used to specify the percentage of traffic should be sent to the target backend.<br />It's used to create model route. | 100 | Maximum: 100 <br />Minimum: 0 <br /> |
| `scaleToZeroGracePeriod` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta)_ | ScaleToZeroGracePeriod is the duration to wait before scaling to zero. |  |  |
| `workers` _[ModelWorker](#modelworker) array_ | Workers is the list of workers associated with this backend. |  | MaxItems: 1000 <br />MinItems: 1 <br /> |
| `loraAdapters` _[LoraAdapter](#loraadapter) array_ | LoraAdapter is a list of LoRA adapters. |  |  |
| `autoscalingPolicy` _[AutoscalingPolicySpec](#autoscalingpolicyspec)_ | AutoscalingPolicyRef references the autoscaling policy for this backend. |  |  |


#### ModelBackendStatus



ModelBackendStatus defines the status of a model backend.



_Appears in:_
- [ModelStatus](#modelstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the backend. |  |  |
| `replicas` _integer_ | Replicas is the number of replicas currently running for the backend. |  |  |


#### ModelBackendType

_Underlying type:_ _string_

ModelBackendType defines the type of model backend.

_Validation:_
- Enum: [vLLM vLLMDisaggregated SGLang MindIE MindIEDisaggregated]

_Appears in:_
- [ModelBackend](#modelbackend)

| Field | Description |
| --- | --- |
| `vLLM` | ModelBackendTypeVLLM represents a vLLM backend.<br /> |
| `vLLMDisaggregated` | ModelBackendTypeVLLMDisaggregated represents a disaggregated vLLM backend.<br /> |
| `SGLang` | ModelBackendTypeSGLang represents an SGLang backend.<br /> |
| `MindIE` | ModelBackendTypeMindIE represents a MindIE backend.<br /> |
| `MindIEDisaggregated` | ModelBackendTypeMindIEDisaggregated represents a disaggregated MindIE backend.<br /> |


#### ModelBooster



ModelBooster is the Schema for the models API.



_Appears in:_
- [ModelBoosterList](#modelboosterlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `ModelBooster` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ModelBoosterSpec](#modelboosterspec)_ |  |  |  |
| `status` _[ModelStatus](#modelstatus)_ |  |  |  |


#### ModelBoosterList



ModelBoosterList contains a list of ModelBooster.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `ModelBoosterList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ModelBooster](#modelbooster) array_ |  |  |  |


#### ModelBoosterSpec



ModelBoosterSpec defines the desired state of ModelBooster.



_Appears in:_
- [ModelBooster](#modelbooster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the model. ModelBooster CR name is restricted by kubernetes, for example, can't contain uppercase letters.<br />So we use this field to specify the ModelBooster name. |  | MaxLength: 64 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `owner` _string_ | Owner is the owner of the model. |  |  |
| `backends` _[ModelBackend](#modelbackend) array_ | Backends is the list of model backends associated with this model. A ModelBooster CR at lease has one ModelBackend.<br />ModelBackend is the minimum unit of inference instance. It can be vLLM, SGLang, MindIE or other types. |  | MinItems: 1 <br /> |
| `autoscalingPolicy` _[AutoscalingPolicySpec](#autoscalingpolicyspec)_ | AutoscalingPolicy references the autoscaling policy to be used for this model. |  |  |
| `costExpansionRatePercent` _integer_ | CostExpansionRatePercent is the percentage rate at which the cost expands. |  | Maximum: 1000 <br />Minimum: 0 <br /> |
| `modelMatch` _[ModelMatch](#modelmatch)_ | ModelMatch defines the predicate used to match LLM inference requests to a given<br />TargetModels. Multiple match conditions are ANDed together, i.e. the match will<br />evaluate to true only if all conditions are satisfied. |  |  |


#### ModelServing



ModelServing is the Schema for the LLM Serving API



_Appears in:_
- [ModelServingList](#modelservinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `ModelServing` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ModelServingSpec](#modelservingspec)_ |  |  |  |
| `status` _[ModelServingStatus](#modelservingstatus)_ |  |  |  |




#### ModelServingList



ModelServingList contains a list of ModelServing





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.serving.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `ModelServingList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ModelServing](#modelserving) array_ |  |  |  |


#### ModelServingSpec



ModelServingSpec defines the specification of the ModelServing resource.



_Appears in:_
- [ModelServing](#modelserving)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Number of ServingGroups. That is the number of instances that run serving tasks<br />Default to 1. | 1 |  |
| `schedulerName` _string_ | SchedulerName defines the name of the scheduler used by ModelServing |  |  |
| `template` _[ServingGroup](#servinggroup)_ | Template defines the template for ServingGroup |  |  |
| `rolloutStrategy` _[RolloutStrategy](#rolloutstrategy)_ | RolloutStrategy defines the strategy that will be applied to update replicas |  |  |
| `recoveryPolicy` _[RecoveryPolicy](#recoverypolicy)_ | RecoveryPolicy defines the recovery policy for the failed Pod to be rebuilt | RoleRecreate | Enum: [ServingGroupRecreate RoleRecreate None] <br /> |
| `topologySpreadConstraints` _[TopologySpreadConstraint](#topologyspreadconstraint) array_ |  |  |  |


#### ModelServingStatus



ModelServingStatus defines the observed state of ModelServing



_Appears in:_
- [ModelServing](#modelserving)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | observedGeneration is the most recent generation observed for ModelServing. It corresponds to the<br />ModelServing's generation, which is updated on mutation by the API Server. |  |  |
| `replicas` _integer_ | Replicas track the total number of ServingGroup that have been created (updated or not, ready or not) |  |  |
| `currentReplicas` _integer_ | CurrentReplicas is the number of ServingGroup created by the ModelServing controller from the ModelServing version |  |  |
| `updatedReplicas` _integer_ | UpdatedReplicas track the number of ServingGroup that have been updated (ready or not). |  |  |
| `availableReplicas` _integer_ | AvailableReplicas track the number of ServingGroup that are in ready state (updated or not). |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | Conditions track the condition of the ModelServing. |  |  |


#### ModelStatus



ModelStatus defines the observed state of ModelBooster.



_Appears in:_
- [ModelBooster](#modelbooster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | Conditions represents the latest available observations of the model's state. |  |  |
| `backendStatuses` _[ModelBackendStatus](#modelbackendstatus) array_ | BackendStatuses contains the status of each backend. |  |  |
| `observedGeneration` _integer_ | ObservedGeneration track of generation |  |  |




#### ModelWorker



ModelWorker defines the model worker configuration.



_Appears in:_
- [ModelBackend](#modelbackend)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ModelWorkerType](#modelworkertype)_ | Type is the type of the model worker. | server | Enum: [server prefill decode controller coordinator] <br /> |
| `image` _string_ | Image is the container image for the worker. |  |  |
| `replicas` _integer_ | Replicas is the number of replicas for the worker. |  | Maximum: 1e+06 <br />Minimum: 0 <br /> |
| `pods` _integer_ | Pods is the number of pods for the worker. |  | Maximum: 1e+06 <br />Minimum: 0 <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core)_ | Resources specifies the resource requirements for the worker. |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#affinity-v1-core)_ | Affinity specifies the affinity rules for scheduling the worker pods. |  |  |
| `config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#json-v1-apiextensions-k8s-io)_ | Config contains worker-specific configuration in JSON format.<br />You can find vLLM config here https://docs.vllm.ai/en/stable/configuration/engine_args.html |  |  |


#### ModelWorkerType

_Underlying type:_ _string_

ModelWorkerType defines the type of model worker.

_Validation:_
- Enum: [server prefill decode controller coordinator]

_Appears in:_
- [ModelWorker](#modelworker)

| Field | Description |
| --- | --- |
| `server` | ModelWorkerTypeServer represents a server worker.<br /> |
| `prefill` | ModelWorkerTypePrefill represents a prefill worker.<br /> |
| `decode` | ModelWorkerTypeDecode represents a decode worker.<br /> |
| `controller` | ModelWorkerTypeController represents a controller worker.<br /> |
| `coordinator` | ModelWorkerTypeCoordinator represents a coordinator worker.<br /> |


#### OptimizerConfiguration







_Appears in:_
- [AutoscalingPolicyBindingSpec](#autoscalingpolicybindingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `params` _[OptimizerParam](#optimizerparam) array_ | Parameters of multiple Model Serving Groups to be optimized. |  | MinItems: 1 <br /> |
| `costExpansionRatePercent` _integer_ | CostExpansionRatePercent is the percentage rate at which the cost expands. | 200 | Minimum: 0 <br /> |


#### OptimizerParam







_Appears in:_
- [OptimizerConfiguration](#optimizerconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `target` _[Target](#target)_ | The scaling instance configuration |  |  |
| `cost` _integer_ | Cost is the cost associated with running this backend. |  | Minimum: 0 <br /> |
| `minReplicas` _integer_ | MinReplicas is the minimum number of replicas for the backend. |  | Maximum: 1e+06 <br />Minimum: 0 <br /> |
| `maxReplicas` _integer_ | MaxReplicas is the maximum number of replicas for the backend. |  | Maximum: 1e+06 <br />Minimum: 1 <br /> |


#### PodTemplateSpec



PodTemplateSpec describes the data a pod should have when created from a template



_Appears in:_
- [Role](#role)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PodSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#podspec-v1-core)_ | Specification of the desired behavior of the pod. |  |  |


#### RecoveryPolicy

_Underlying type:_ _string_





_Appears in:_
- [ModelServingSpec](#modelservingspec)

| Field | Description |
| --- | --- |
| `ServingGroupRecreate` | ServingGroupRecreate will recreate all the pods in the ServingGroup if<br />1. Any individual pod in the group is recreated; 2. Any containers/init-containers<br />in a pod is restarted. This is to ensure all pods/containers in the group will be<br />started in the same time.<br /> |
| `RoleRecreate` | RoleRecreate will recreate all pods in one Role if<br />1. Any individual pod in the group is recreated; 2. Any containers/init-containers<br />in a pod is restarted.<br /> |
| `None` | NoneRestartPolicy will follow the same behavior as the default pod or deployment.<br /> |


#### Role



Role defines the specific pod instance role that performs the inference task.



_Appears in:_
- [ServingGroup](#servinggroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of a role. Name must be unique within an ServingGroup |  | MaxLength: 12 <br />Pattern: `^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?$` <br /> |
| `replicas` _integer_ | The number of a certain role.<br />For example, in Disaggregated Prefilling, setting the replica count for both the P and D roles to 1 results in 1P1D deployment configuration.<br />This approach can similarly be applied to configure a xPyD deployment scenario.<br />Default to 1. | 1 |  |
| `entryTemplate` _[PodTemplateSpec](#podtemplatespec)_ | EntryTemplate defines the template for the entry pod of a role.<br />Required: Currently, a role must have only one entry-pod. |  |  |
| `workerReplicas` _integer_ | WorkerReplicas defines the number for the worker pod of a role.<br />Required: Need to set the number of worker-pod replicas. |  |  |
| `workerTemplate` _[PodTemplateSpec](#podtemplatespec)_ | WorkerTemplate defines the template for the worker pod of a role. |  |  |


#### RollingUpdateConfiguration



RollingUpdateConfiguration defines the parameters to be used for RollingUpdateStrategyType.



_Appears in:_
- [RolloutStrategy](#rolloutstrategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#intorstring-intstr-util)_ | The maximum number of replicas that can be unavailable during the update.<br />Value can be an absolute number (ex: 5) or a percentage of total replicas at the start of update (ex: 10%).<br />Absolute number is calculated from percentage by rounding down.<br />This can not be 0 if MaxSurge is 0.<br />By default, a fixed value of 1 is used. | 1 | XIntOrString: \{\} <br /> |
| `maxSurge` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#intorstring-intstr-util)_ | The maximum number of replicas that can be scheduled above the original number of<br />replicas.<br />Value can be an absolute number (ex: 5) or a percentage of total replicas at<br />the start of the update (ex: 10%).<br />Absolute number is calculated from percentage by rounding up.<br />By default, a value of 0 is used. | 0 | XIntOrString: \{\} <br /> |
| `partition` _integer_ | Partition indicates the ordinal at which the ModelServing should be partitioned<br />for updates. During a rolling update, all ServingGroups from ordinal Replicas-1 to<br />Partition are updated. All ServingGroups from ordinal Partition-1 to 0 remain untouched.<br />The default value is 0. |  |  |


#### RolloutStrategy



RolloutStrategy defines the strategy that the ModelServing controller
will use to perform replica updates.



_Appears in:_
- [ModelServingSpec](#modelservingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[RolloutStrategyType](#rolloutstrategytype)_ | Type defines the rollout strategy, it can only be “ServingGroupRollingUpdate” for now. | ServingGroupRollingUpdate | Enum: [ServingGroupRollingUpdate] <br /> |
| `rollingUpdateConfiguration` _[RollingUpdateConfiguration](#rollingupdateconfiguration)_ | RollingUpdateConfiguration defines the parameters to be used when type is RollingUpdateStrategyType.<br />optional |  |  |


#### RolloutStrategyType

_Underlying type:_ _string_





_Appears in:_
- [RolloutStrategy](#rolloutstrategy)

| Field | Description |
| --- | --- |
| `ServingGroupRollingUpdate` | ServingGroupRollingUpdate indicates that ServingGroup replicas will be updated one by one.<br /> |


#### ScalingConfiguration







_Appears in:_
- [AutoscalingPolicyBindingSpec](#autoscalingpolicybindingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `target` _[Target](#target)_ | Target represents the objects be monitored and scaled. |  |  |
| `minReplicas` _integer_ | MinReplicas is the minimum number of replicas. |  | Maximum: 1e+06 <br />Minimum: 0 <br /> |
| `maxReplicas` _integer_ | MaxReplicas is the maximum number of replicas. |  | Maximum: 1e+06 <br />Minimum: 1 <br /> |


#### SelectPolicyType

_Underlying type:_ _string_

SelectPolicyType defines the type of select olicy.

_Validation:_
- Enum: [Or And]

_Appears in:_
- [AutoscalingPolicyStablePolicy](#autoscalingpolicystablepolicy)

| Field | Description |
| --- | --- |
| `Or` |  |
| `And` |  |


#### ServingGroup



ServingGroup is the smallest unit to complete the inference task



_Appears in:_
- [ModelServingSpec](#modelservingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `restartGracePeriodSeconds` _integer_ | RestartGracePeriodSeconds defines the grace time for the controller to rebuild the ServingGroup when an error occurs<br />Defaults to 0 (ServingGroup will be rebuilt immediately after an error) | 0 |  |
| `gangPolicy` _[GangPolicy](#gangpolicy)_ | GangPolicy defines the gang scheduler config. |  |  |
| `networkTopology` _[NetworkTopologySpec](#networktopologyspec)_ | NetworkTopology defines the network topology affinity scheduling policy for the roles of the group, it works only when the scheduler supports network topology feature.	// +optional |  |  |
| `roles` _[Role](#role) array_ |  |  | MaxItems: 4 <br />MinItems: 1 <br /> |


#### Target







_Appears in:_
- [OptimizerParam](#optimizerparam)
- [ScalingConfiguration](#scalingconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core)_ | TargetRef references the target object. |  |  |
| `additionalMatchLabels` _object (keys:string, values:string)_ | AdditionalMatchLabels is the additional labels to match the target object. |  |  |
| `metricEndpoint` _[MetricEndpoint](#metricendpoint)_ | MetricEndpoint is the metric source. |  |  |


#### TopologySpreadConstraint



TopologySpreadConstraint defines the topology spread constraint.



_Appears in:_
- [ModelServingSpec](#modelservingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxSkew` _integer_ | MaxSkew describes the degree to which ServingGroup may be unevenly distributed. |  |  |
| `topologyKey` _string_ | TopologyKey is the key of node labels. Nodes that have a label with this key<br />and identical values are considered to be in the same topology. |  |  |
| `whenUnsatisfiable` _string_ | WhenUnsatisfiable indicates how to deal with an ServingGroup if it doesn't satisfy<br />the spread constraint. |  |  |
| `labelSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta)_ | LabelSelector is used to find matching ServingGroups. |  |  |


