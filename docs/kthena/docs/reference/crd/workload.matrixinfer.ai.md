# API Reference

## Packages
- [workload.volcano.sh/v1alpha1](#workloadkthenaaiv1alpha1)


## workload.volcano.sh/v1alpha1


### Resource Types
- [ModelInfer](#modelinfer)
- [ModelInferList](#modelinferlist)



#### GangSchedule



GangSchedule defines the gang scheduling configuration.



_Appears in:_
- [InferGroup](#infergroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `networkTopology` _[NetworkTopologySpec](#networktopologyspec)_ | NetworkTopology defines the NetworkTopology config, this field works in conjunction with network topology feature and hyperNode CRD. |  |  |
| `minRoleReplicas` _object (keys:string, values:integer)_ | MinRoleReplicas defines the minimum number of replicas required for each role<br />in gang scheduling. This map allows users to specify different<br />minimum replica requirements for different roles.<br />Key: role name<br />Value: minimum number of replicas required for that role |  |  |


#### InferGroup



InferGroup is the smallest unit to complete the inference task



_Appears in:_
- [ModelInferSpec](#modelinferspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `restartGracePeriodSeconds` _integer_ | RestartGracePeriodSeconds defines the grace time for the controller to rebuild the infergroup when an error occurs<br />Defaults to 0 (infergroup will be rebuilt immediately after an error) | 0 |  |
| `gangSchedule` _[GangSchedule](#gangschedule)_ | GangSchedule defines the GangSchedule config. |  |  |
| `roles` _[Role](#role) array_ |  |  | MaxItems: 4 <br />MinItems: 1 <br /> |


#### Metadata



Metadata is a simplified version of ObjectMeta in Kubernetes.



_Appears in:_
- [PodTemplateSpec](#podtemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ | Map of string keys and values that can be used to organize and categorize<br />(scope and select) objects. May match selectors of replication controllers<br />and services.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations is an unstructured key value map stored with a resource that may be<br />set by external tools to store and retrieve arbitrary metadata. They are not<br />queryable and should be preserved when modifying objects.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations |  |  |


#### ModelInfer



ModelInfer is the Schema for the LLM infer API



_Appears in:_
- [ModelInferList](#modelinferlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `ModelInfer` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ModelInferSpec](#modelinferspec)_ |  |  |  |
| `status` _[ModelInferStatus](#modelinferstatus)_ |  |  |  |




#### ModelInferList



ModelInferList contains a list of ModelInfer





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `workload.volcano.sh/v1alpha1` | | |
| `kind` _string_ | `ModelInferList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ModelInfer](#modelinfer) array_ |  |  |  |


#### ModelInferSpec



ModelInferSpec defines the specification of the ModelInfer resource.



_Appears in:_
- [ModelInfer](#modelinfer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Number of InferGroups. That is the number of instances that run infer tasks<br />Default to 1. | 1 |  |
| `schedulerName` _string_ | SchedulerName defines the name of the scheduler used by ModelInfer |  |  |
| `template` _[InferGroup](#infergroup)_ | Template defines the template for InferGroup |  |  |
| `rolloutStrategy` _[RolloutStrategy](#rolloutstrategy)_ | RolloutStrategy defines the strategy that will be applied to update replicas |  |  |
| `recoveryPolicy` _[RecoveryPolicy](#recoverypolicy)_ | RecoveryPolicy defines the recovery policy for the failed Pod to be rebuilt | RoleRecreate | Enum: [InferGroupRecreate RoleRecreate None] <br /> |
| `topologySpreadConstraints` _[TopologySpreadConstraint](#topologyspreadconstraint) array_ |  |  |  |


#### ModelInferStatus



ModelInferStatus defines the observed state of ModelInfer



_Appears in:_
- [ModelInfer](#modelinfer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | observedGeneration is the most recent generation observed for ModelInfer. It corresponds to the<br />ModelInfer's generation, which is updated on mutation by the API Server. |  |  |
| `replicas` _integer_ | Replicas track the total number of InferGroup that have been created (updated or not, ready or not) |  |  |
| `currentReplicas` _integer_ | CurrentReplicas is the number of InferGroup created by the ModelInfer controller from the ModelInfer version |  |  |
| `updatedReplicas` _integer_ | UpdatedReplicas track the number of InferGroup that have been updated (ready or not). |  |  |
| `availableReplicas` _integer_ | AvailableReplicas track the number of InferGroup that are in ready state (updated or not). |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#condition-v1-meta) array_ | Conditions track the condition of the ModelInfer. |  |  |


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
- [ModelInferSpec](#modelinferspec)

| Field | Description |
| --- | --- |
| `InferGroupRecreate` | InferGroupRecreate will recreate all the pods in the InferGroup if<br />1. Any individual pod in the group is recreated; 2. Any containers/init-containers<br />in a pod is restarted. This is to ensure all pods/containers in the group will be<br />started in the same time.<br /> |
| `RoleRecreate` | RoleRecreate will recreate all pods in one Role if<br />1. Any individual pod in the group is recreated; 2. Any containers/init-containers<br />in a pod is restarted.<br /> |
| `None` | NoneRestartPolicy will follow the same behavior as the default pod or deployment.<br /> |


#### Role



Role defines the specific pod instance role that performs the inference task.



_Appears in:_
- [InferGroup](#infergroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of a role. Name must be unique within an infergroup |  | MaxLength: 12 <br />Pattern: `^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?$` <br /> |
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
| `partition` _integer_ | Partition indicates the ordinal at which the ModelInfer should be partitioned<br />for updates. During a rolling update, all inferGroups from ordinal Replicas-1 to<br />Partition are updated. All inferGroups from ordinal Partition-1 to 0 remain untouched.<br />The default value is 0. |  |  |


#### RolloutStrategy



RolloutStrategy defines the strategy that the ModelInfer controller
will use to perform replica updates.



_Appears in:_
- [ModelInferSpec](#modelinferspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[RolloutStrategyType](#rolloutstrategytype)_ | Type defines the rollout strategy, it can only be “InferGroupRollingUpdate” for now. | InferGroupRollingUpdate | Enum: [InferGroupRollingUpdate] <br /> |
| `rollingUpdateConfiguration` _[RollingUpdateConfiguration](#rollingupdateconfiguration)_ | RollingUpdateConfiguration defines the parameters to be used when type is RollingUpdateStrategyType.<br />optional |  |  |


#### RolloutStrategyType

_Underlying type:_ _string_





_Appears in:_
- [RolloutStrategy](#rolloutstrategy)

| Field | Description |
| --- | --- |
| `InferGroupRollingUpdate` | InferGroupRollingUpdate indicates that InferGroup replicas will be updated one by one.<br /> |


#### TopologySpreadConstraint



TopologySpreadConstraint defines the topology spread constraint.



_Appears in:_
- [ModelInferSpec](#modelinferspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxSkew` _integer_ | MaxSkew describes the degree to which inferGroup may be unevenly distributed. |  |  |
| `topologyKey` _string_ | TopologyKey is the key of node labels. Nodes that have a label with this key<br />and identical values are considered to be in the same topology. |  |  |
| `whenUnsatisfiable` _string_ | WhenUnsatisfiable indicates how to deal with an inferGroup if it doesn't satisfy<br />the spread constraint. |  |  |
| `labelSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta)_ | LabelSelector is used to find matching inferGroups. |  |  |


