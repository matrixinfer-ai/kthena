---
title: Two-Level Gang Scheduling for ModelInfer Workloads
authors:
- "@VanderChen"
reviewers:
- TBD
approvers:
- TBD

creation-date: 2025-08-06
status: draft

---

## Two-Level Gang Scheduling for ModelInfer Workloads

<!--
This is the title of your KEP. Keep it short, simple, and descriptive. A good
title can help communicate what the KEP is and should be considered as part of
any review.
-->

### Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap.

A good summary is probably at least a paragraph in length.
-->

This proposal describes the design for implementing two-level gang scheduling for ModelInfer workloads using Volcano's PodGroup mechanism.
The design enables gang scheduling at two levels: 
1. the entire ModelInfer instance (InferGroups) level, ensuring all InferGroups are scheduled together.
2. the individual role level within each InferGroup, ensuring all pods of a specific role are scheduled together.

### Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->

#### Goals

<!--
List the specific goals of the KEP. What is it trying to achieve? How will we
know that this has succeeded?
-->

**Two-level gang scheduling**: Implement gang scheduling at both ModelInfer instance level and role level

#### Non-Goals

<!--
What is out of scope for this KEP? Listing non-goals helps to focus discussion
and make progress.
-->

1. **Custom scheduler**: We will not implement a custom scheduler, but use Volcano's existing capabilities
2. **Complex scheduling policies**: Focus on basic gang scheduling, not advanced scheduling policies
3. **Multi-cluster scheduling**: This design is focused on single-cluster scheduling

### Proposal

<!--
This is where we get down to the specifics of what the proposal actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real
nitty-gritty.
-->

#### User Stories (Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

##### Story 1: ModelInfer instance level gang scheduling

InferGroups are the top-level scheduling units in ModelInfer. Each InferGroup represents a single ModelInfer instance.

###### Scenario 1: Fix ratio prefill decode disaggregation

In this scenario, we want to ensure that all pods of a infergroup are scheduled togethergit.
For example, a ModelInfer instance has 2 roles: `prefiller` and `decoder`.
The `prefiller` role has 1 pod, and the `decoder` role has 3 pods.
```yaml
kind: ModelInfer
metadata:
  name: sample
spec:
  replicas: 1  # inferGroup replicas
  template:
    roles:
      - name: prefill
        replicas: 1
        ...       # role replicas, for example, 1P3D
      - name: decode
        replicas: 3  # role replicas, for example, 1P3D
        ...
```
we want to ensure the `prefiller` pod and the 3 `decoder` pods are scheduled together.
When scale up, another `prefiller` pod and 3 `decoder` pods will be scheduled together.
```yaml
kind: ModelInfer
metadata:
  name: sample
spec:
  replicas: 2  # inferGroup replicas
  template:
    roles:
      - name: prefill
        replicas: 1
        ...       # role replicas, for example, 1P3D
      - name: decode
        replicas: 3  # role replicas, for example, 1P3D
        ...
```

##### Story 2: Dynamic ratio prefill decode disaggregation

In this scenario, we want to ensure that all pods of a specific role are scheduled together, but different InferGroups can be scheduled independently.

#### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go in to as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

#### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate?

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->

### Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

#### API Changes

The design uses an enum-based approach for gang scheduling levels instead of separate boolean fields, providing better clarity and preventing invalid combinations:

- **`group`**: Use when you want all InferGroups of a ModelInfer to be scheduled together. This is suitable for workloads where all instances need to be coordinated, such as distributed training or inference workloads that require global coordination.

- **`role`**: Use when you want all pods of the same role within each InferGroup to be scheduled together, but different InferGroups can be scheduled independently. This is suitable for workloads where role-level coordination is sufficient, such as when each InferGroup operates independently.

It only works when `GangSchedule.Enable` is true.

```go
// GangScheduleLevel defines the level at which gang scheduling should be applied
type GangScheduleLevel string

const (
    // GangScheduleLevelRole enables gang scheduling at the role level within each InferGroup
    GangScheduleLevelRole GangScheduleLevel = "role"
    // GangScheduleLevelGroup enables gang scheduling at the ModelInfer group level, creating podgroup for each modelinfer replica
    GangScheduleLevelGroup GangScheduleLevel = "group"
)

// GangSchedule defines the gang scheduling configuration.
type GangSchedule struct {
    // Enable indicates whether users want to enforce gang-scheduling,
    // default true
    // +kubebuilder:default=true
    Enable *bool `json:"enable,omitempty"`
    
    // Level defines the level at which gang scheduling should be applied
    // +optional
    // +kubebuilder:default="group"
    // +kubebuilder:validation:Enum={role,group}
    Level GangScheduleLevel `json:"level,omitempty"`

    // MinRoleReplicas defines the minimum number of replicas required for each role
    // in role-level gang scheduling. This map allows users to specify different
    // minimum replica requirements for different roles.
    // Key: role name, Value: minimum number of replicas required for that role
    // Only used when Level is "role"
    // +optional
    MinRoleReplicas map[string]int32 `json:"minRoleReplicas,omitempty"`
}
```

Use [`MinTaskMember`](https://github.com/volcano-sh/volcano/blob/master/docs/design/task-minavailable.md) to control the gang-scheduling scope.

MinTaskMember is a map where:
- **Key**: `{role name + role replica index}` (e.g., "prefill-0", "decode-1")
- **Value**: Number of pods required for that specific role replica

This allows fine-grained control over gang scheduling at both instance and role levels by defining the minimum number of pods required for each role replica within a PodGroup.

#### Controller Architecture

The gang scheduling implementation is integrated into the existing ModelInfer controller architecture with the following components:

##### Gang Scheduling Manager

A dedicated gang scheduling manager within the ModelInfer controller handles PodGroup lifecycle management. The manager is responsible for:

- Creating and managing PodGroups based on the configured gang scheduling level
- Cleaning up PodGroups when gang scheduling is disabled
- Handling PodGroup updates during scaling operations

##### PodGroup Creation Strategy

**Group-Level Gang Scheduling:**
- Creates one PodGroup per ModelInfer replica (each InferGroup)
- PodGroup name: `{modelinfer-name}-{infergroup-index}`
- Each role instance becomes a task in the PodGroup
- MinTaskMember includes all role instances within the InferGroup

**Role-Level Gang Scheduling:**
- Creates one PodGroup per ModelInfer replica (each InferGroup)
- PodGroup name: `{modelinfer-name}-{infergroup-index}`
- Each role instance becomes a task in the PodGroup
- MinTaskMember includes only specific role instances based on MinRoleReplicas configuration

**Important Note:** For both levels, PodGroups are created for each ModelInfer replica (each InferGroup). Each role instance becomes a task in the PodGroup. The difference is in the MinTaskMember configuration:
- Group-level: MinTaskMember includes all role instances, enforcing all roles to be scheduled together
- Role-level: MinTaskMember includes only role instances up to the MinRoleReplicas limit for each role, allowing flexible role-level coordination

##### Pod Annotation Strategy

All pods created by the ModelInfer controller are annotated with the appropriate PodGroup information using the `volcano.sh/group-name` annotation. The annotation value corresponds to the PodGroup name that the pod should belong to, enabling Volcano scheduler to enforce gang scheduling constraints.

#### Scaling and Lifecycle Management

##### Scale-Up Operations

When scaling up ModelInfer replicas, the controller creates new PodGroups for the newly added InferGroups:

- **Group-Level**: Creates new PodGroups for each new InferGroup with MinTaskMember including all role instances
- **Role-Level**: Creates new PodGroups for each new InferGroup with MinTaskMember including only specific role instances for coordination

##### Scale-Down Operations

When scaling down, the controller removes PodGroups associated with the scaled-down InferGroups:

- **Group-Level**: Removes PodGroups for scaled-down InferGroups
- **Role-Level**: Removes PodGroups for scaled-down InferGroups

#### Instance-Level Gang Scheduling

##### Creation Process
When creating a ModelInfer instance with `gangSchedule.level: instance`, a single PodGroup is created for the entire ModelInfer instance.

**PodGroup Naming Convention:**
- Name: `{modelinfer-name}-instance`
- Labels and annotations include ModelInfer metadata

**PodGroup Configuration:**
```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: {modelinfer-name}-instance
  namespace: {modelinfer-namespace}
  labels:
    modelinfer.matrixinfer.ai/name: {modelinfer-name}
    modelinfer.matrixinfer.ai/gang-level: instance
  annotations:
    volcano.sh/group-name: {modelinfer-name}-instance
    modelinfer.matrixinfer.ai/created-by: modelinfer-controller
spec:
  # Total pods across all InferGroups and roles
  minMember: {total-pod-count}
  # MinTaskMember defines minimum pods required for each role replica
  minTaskMember:
    "prefill-0": 4    # 1 entry + 3 workers for prefill role replica 0
    "decode-0": 4     # 1 entry + 3 workers for decode role replica 0
    # ... additional role replicas
  minResources:
    # Aggregated resource requirements
  ttlSecondsAfterFinished: {timeout-seconds}
```

**Pod Count Calculation:**
For instance-level gang scheduling, `minMember` is calculated as:
```
minMember = replicas × Σ(role.replicas × (1 + role.workerReplicas))
```

Where:
- `replicas`: Number of InferGroup instances
- `role.replicas`: Number of role instances within each InferGroup
- `1 + role.workerReplicas`: Entry pod + worker pods per role instance

#### Role-Level Gang Scheduling

##### Creation Process
When creating a ModelInfer instance with `gangSchedule.level: role`, PodGroups are created for each ModelInfer replica (each InferGroup), same as group-level, but with different MinTaskMember configuration.

**PodGroup Naming Convention:**
- Name: `{modelinfer-name}-{infergroup-index}`
- Same naming as group-level scheduling

**PodGroup Configuration:**
```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: {modelinfer-name}-{infergroup-index}
  namespace: {modelinfer-namespace}
  labels:
    modelinfer.matrixinfer.ai/name: {modelinfer-name}
    modelinfer.matrixinfer.ai/group-name: {modelinfer-name}-{infergroup-index}
    modelinfer.matrixinfer.ai/gang-level: role
  annotations:
    volcano.sh/group-name: {modelinfer-name}-{infergroup-index}
    modelinfer.matrixinfer.ai/created-by: modelinfer-controller
spec:
  # Total pods across all roles in this InferGroup
  minMember: {total-pod-count}
  # MinTaskMember defines minimum pods required for each role replica
  # For role-level: references MinRoleReplicas configuration
  minTaskMember:
    "prefill-0": 4    # 1 entry + 3 workers for prefill role replica 0
    "prefill-1": 4    # 1 entry + 3 workers for prefill role replica 1
    # Only includes role replicas up to MinRoleReplicas["prefill"] limit
    # If MinRoleReplicas["prefill"] = 2, only prefill-0 and prefill-1 are included
```

**MinTaskMember Generation Logic:**
For role-level gang scheduling, MinTaskMember is populated based on the MinRoleReplicas configuration:
- For each role specified in MinRoleReplicas map
- Include role replicas from index 0 up to MinRoleReplicas[roleName] - 1
- Each role replica entry has value = (1 + role.workerReplicas)

**Example Configuration:**
```yaml
gangSchedule:
  enable: true
  level: role
  minRoleReplicas:
    prefill: 2    # Require at least 2 prefill role replicas
    decode: 1     # Require at least 1 decode role replica
```

**Pod Count Calculation:**
For role-level gang scheduling, `minMember` for each PodGroup is calculated as:
```
minMember = Σ(role.replicas × (1 + role.workerReplicas))
```

#### PodGroup Examples

For detailed YAML examples of PodGroups created from ModelInfer instances, see [podgroup-examples.yaml](./podgroup-examples.yaml).

The examples demonstrate:
1. **Instance-level gang scheduling**: Single PodGroup for entire ModelInfer
2. **Role-level gang scheduling**: Multiple PodGroups, one per role per InferGroup
3. **Gang scheduling disabled**: No PodGroups created

#### Test Plan

<!--
**Note:** *Not required until targeted at a release.*

Consider the following in developing a test plan for this enhancement:
- Will there be e2e and integration tests, in addition to unit tests?
- How will it be tested in isolation vs with other components?

No need to outline all test cases, just the general strategy. Anything
that would count as tricky in the implementation, and anything particularly
challenging to test, should be called out.

-->

### Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->
