---
title: Your short, descriptive title
authors:
- "@LiZhencheng9527" # Authors' GitHub accounts here.
reviewers:
- "@robot"
- TBD
approvers:
- "@robot"
- TBD

creation-date: 2025-11-06

---

## Your short, descriptive title

<!--
This is the title of your proposal. Keep it short, simple, and descriptive. A good
title can help communicate what the proposal is and should be considered as part of
any review.
-->

### Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap.

A good summary is probably at least a paragraph in length.
-->

When scaling down a `ServingGroup` or `Role`, the binpack score determines which pods should be evicted.
This change will disrupt the existing logic that processes changes to `ServingGroup` and `Role` replicas in descending order by replica ID. This article will also explain how to minimize the impact on the original logic.

### Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this proposal.  Describe why the change is important and the benefits to users.
-->

#### Goals

<!--
List the specific goals of the proposal. What is it trying to achieve? How will we
know that this has succeeded?
-->

#### Non-Goals

<!--
What is out of scope for this proposal? Listing non-goals helps to focus discussion
and make progress.
-->

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
Detail the things that people will be able to do if this proposal is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

##### Story 1

##### Story 2

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

Within modelServing, there are two granularity levels for scaling operations: ServingGroup and Role. But in fact, both involve evicting a group of pods. However, both methods ultimately evict a group of pods. Therefore, this proposal outlines how to calculate the Pod Eviction Cost for a group of pods.

In Kubernetes, the `PodDeletionCost` annotation specifies the cost associated with deleting a pod. We utilize Volcano's binpack plugin to update this annotation.

#### Cost Calculation for Deleting Roles and Service Groups

A role contains one entryPod and multiple workerPods. Each Pod annotates its deletion cost via `PodDeletionCost`. Thus, the cost of deleting the role can be obtained by simply summing these values. When `Role` scaling down, calculate the deletion cost for all `Roles` under the `ServingGroup`. Then sort them by score and select the `Roles` to be deleted.

Similar to scaling down at the `Role`, when scaling down a `ServingGroup`, the `PodDeletionCost` values of all Pods within the `ServingGroup` are summed. The `ServingGroup` to be deleted is then selected based on this sum.

```math
roleScore = \sum_{i=1}^{n} PodDeletionCost_{i}

servingGroupScore = \sum_{i=1}^{m} roleScore_{i}
```

#### Pod Sequence Number Handling

The current `modelServing` pods operate similarly to `statefulSet`. During scaling, they are processed in ascending order by sequence number. However, in the binpack scale-down process, this processing logic is disrupted. However, during binpack scale-down operations, or when selecting `ServingGroups` or `Roles` for deletion based on scores, the target may not necessarily be the object with the highest serial number. This can disrupt the previously established processing logic.

To ensure maximum compatibility with existing logic, we have implemented this approach.

The logic behind scaling down is as described above. During the scaling up process, the ModelServing Controller will first fill any vacancies before scaling out further.

For example:

|        | R-0 | R-1 | R-2 | R-3 | Note                                                                          |
|--------|-----|-----|-----|-----|-------------------------------------------------------------------------------|
| Stage1 | ✅   | ✅   | ✅   || Before Scaling update |
| Stage2 | ✅   | ⏳   | ✅   || Scaling down started, The replica with the lowest score(R-1) is deleting |
| Stage3 | ✅   || ✅   || After Scaling down |
| Stage4 | ✅   | ⏳ | ✅ | ⏳ | Scale up 2 replicas. First create R-1. Then create R-3 |
| Stage5 | ✅   | ✅ | ✅   | ✅   | After Scaling up |

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