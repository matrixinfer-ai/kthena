# Multi-Node Inference

This page describes the multi-node inference capabilities in MatrixInfer, base on real-world examples and configurations.

## Overview

With the development of LLM, the scale of model parameters has grown exponentially, and the resource limits of a single conventional virtual machine or physical server can no longer meet the computational demands of these LLM.

The industry has proposed various innovative optimization strategies, such as PD-disaggregates and hybrid deployment of large and small models. These strategies have significantly changed the execution pattern of inference tasks, making inference instances no longer limited to the level of a single Pod, but rather evolving into scenarios where multiple Pods collaboratively complete a single inference prediction.

To address this issue, matrixinfer provides a new `ModelInfer` CR to describe specific inference depolyment, enabling flexible and diverse deployment methods for inference task pods.

For a detailed definition of the `ModelInfer`, please refer to the [ModelInfer Reference](../reference/crd/workload.matrixinfer.ai.md) pages.

## Preparation

To simplify deployment and reduce the requirements for demonstration environments, we use a **mock LLM server** instead of deploying real models. This mock server implements the vLLM standard interface and returns mock data, making it perfect for testing and learning purposes.

### Prerequisites

- Kubernetes cluster with MatrixInfer installed
- Access to the MatrixInfer examples repository
- Basic understanding of ModelInfer CRD

### Geting Started

Deploy [mock LLM inference engine](../../../../examples/model-infer/mock.yaml) if you donot have a real GPU/NPU environment at the moment.

You can run the following command to check the ModelInfer status and pod status in the cluster.

```sh
kubectl get modelinfer -oyaml | grep status -A 10

status:
  availableReplicas: 1
  conditions:
  - lastTransitionTime: "2025-09-05T08:53:25Z"
    message: All infer groups are ready
    reason: AllGroupsReady
    status: "True"
    type: Available
  - lastTransitionTime: "2025-09-05T08:53:23Z"
    message: 'Some groups is progressing: [0]'
    reason: GroupProgressing
    status: "False"
    type: Progerssing
  currentReplicas: 1
  observedGeneration: 4
  replicas: 1
  updatedReplicas: 1

kubectl get pod -A | grep sample
NAMESPACE            NAME                                          READY   STATUS    RESTARTS   AGE
default              sample-0-decode-0-0                           1/1     Running   0          15m
default              sample-0-decode-0-1                           1/1     Running   0          15m
default              sample-0-decode-0-2                           1/1     Running   0          15m
default              sample-0-decode-1-0                           1/1     Running   0          5m59s
default              sample-0-decode-1-1                           1/1     Running   0          5m59s
default              sample-0-decode-1-2                           1/1     Running   0          5m59s
default              sample-0-prefill-0-0                          1/1     Running   0          15m
default              sample-0-prefill-0-1                          1/1     Running   0          15m
default              sample-0-prefill-0-2                          1/1     Running   0          15m
default              sample-0-prefill-1-0                          1/1     Running   0          5m59s
default              sample-0-prefill-1-1                          1/1     Running   0          5m59s
default              sample-0-prefill-1-2                          1/1     Running   0          5m59s
```

**Note:** The first number in the pod name indicates which `InferGroup` this pod belongs to. The second number indicates which `Role` it belongs to. The third number indicates the pod's sequence number within `Role`.

## Scaling

ModelInfer supports scale strategies at two levels: `InferGroup` and `Role`.

You can modify `modelInfer.Spec.Replicas` to trigger scaling at the group level.
Additionally, modifying `modelInfer.Spec.Template.Role.Replicas` triggers role-level scaling.

### Role Level Scale Down

Reduce the `modelInfer.Spec.Template.Role.Replicas` from 2 to 1. To trigger a `Role Level` scale down.

You can see the result:

```sh
kubectl get pod -A | grep sample

NAMESPACE            NAME                                          READY   STATUS    RESTARTS   AGE
default              sample-0-decode-0-0                           1/1     Running   0          28m
default              sample-0-decode-0-1                           1/1     Running   0          28m
default              sample-0-decode-0-2                           1/1     Running   0          28m
default              sample-0-prefill-0-0                          1/1     Running   0          28m
default              sample-0-prefill-0-1                          1/1     Running   0          28m
default              sample-0-prefill-0-2                          1/1     Running   0          28m
```

You can see that all pods in `Role1` have been deleted.

### InferGroup Level Scale Up

Add the `modelInfer.Spec.Replicas` from 1 to 2. To trigger a `InferGroup Level` scale up.

You can see the result:

```sh
kubectl get pod -A | grep sample 

NAMESPACE            NAME                                          READY   STATUS    RESTARTS   AGE
default              sample-0-decode-0-0                           1/1     Running   0          35m
default              sample-0-decode-0-1                           1/1     Running   0          35m
default              sample-0-decode-0-2                           1/1     Running   0          35m
default              sample-0-prefill-0-0                          1/1     Running   0          35m
default              sample-0-prefill-0-1                          1/1     Running   0          35m
default              sample-0-prefill-0-2                          1/1     Running   0          35m
default              sample-1-decode-0-0                           1/1     Running   0          5s
default              sample-1-decode-0-1                           1/1     Running   0          5s
default              sample-1-decode-0-2                           1/1     Running   0          5s
default              sample-1-prefill-0-0                          1/1     Running   0          5s
default              sample-1-prefill-0-1                          1/1     Running   0          5s
default              sample-1-prefill-0-2                          1/1     Running   0          5s
```

You can see that all roles in `InferGroup1` are created.

You can also scale both the `InferGroup` and `Role Level`.

## Rolling Update

Currently, `ModelInfer` supports rolling upgrades at the `InferGroup` level, enabling users to configure `Partitions` to control the rolling process.

- Partition: Indicates the ordinal at which the `ModelInfer` should be partitioned for updates. During a rolling update, replicas with an ordinal greater than or equal to `Partition` will be updated. Replicas with an ordinal less than `Partition` will not be updated.
  
### InferGroup Rolling Update

We configure the corresponding rolling update strategy for sample.

```yaml
spec:
  rolloutStrategy:
    type: InferGroupRollingUpdate
    rollingUpdateConfiguration:
      partition: 1
```

Modifying the parameters of entryTemplate or workerTemplate triggers a rolling update.

You can see the result:

```sh
kubectl get pod | grep sample

NAMESPACE            NAME                                          READY   STATUS    RESTARTS   AGE
default              sample-0-decode-0-0                           1/1     Running   0          2d18h
default              sample-0-decode-0-1                           1/1     Running   0          2d18h
default              sample-0-decode-0-2                           1/1     Running   0          2d18h
default              sample-0-prefill-0-0                          1/1     Running   0          2d18h
default              sample-0-prefill-0-1                          1/1     Running   0          2d18h
default              sample-0-prefill-0-2                          1/1     Running   0          2d18h
default              sample-1-decode-0-0                           1/1     Running   0          2m13s
default              sample-1-decode-0-1                           1/1     Running   0          2m13s
default              sample-1-decode-0-2                           1/1     Running   0          2m13s
default              sample-1-prefill-0-0                          1/1     Running   0          2m13s
default              sample-1-prefill-0-1                          1/1     Running   0          2m13s
default              sample-1-prefill-0-2                          1/1     Running   0          2m13s
```

From the pod runtime, it can be seen that only group 1 has been updated. Because we have set rolloutStrategy.partition = 1.

## Gang Scheduling and Network Topology

Gang scheduling is a feature that allows pods to be scheduled together. This is useful when you have a set of pods that need to be scheduled together. For example, you may have a set of pods that need to be scheduled together because they are pods of the same model.

In matrixinfer, we use [Volcano gang scheduling](https://volcano.sh/en/docs/v1-10-0/plugins/#gang) to ensure that required pods are scheduled concurrently.

Use the following command to install Volcano:

```sh
helm repo add volcano-sh https://volcano-sh.github.io/helm-charts

helm repo update

helm install volcano volcano-sh/volcano -n volcano-system --create-namespace
```

We will create podGroups based on modelinfer. Among these, the important one is `MinRoleReplicas`. Defines the minimum number of replicas required for each role in gang scheduling. This map allows users to specify different minimum replica requirements for different roles.

- **Key:** role name
- **Value:** minimum number of replicas required for that role

Additionally, it supports using `network topology` to reduce network latency among pods within the same podGroup.

Before using the network topology, you need to create a [hyper node](https://volcano.sh/en/docs/network_topology_aware_scheduling/) for Volcano.

PodGroup can set the topology constraints of the job through the networkTopology field, supporting the following configurations:

- **mode:** Supports hard and soft modes.
  - hard: Hard constraint, tasks within the job must be deployed within the same HyperNode.
  - soft: Soft constraint, tasks are deployed within the same HyperNode as much as possible.
- **highestTierAllowed:** Used with hard mode, indicating the highest tier of HyperNode allowed for job deployment. This field is not required when mode is soft.

You can run the follow command to see the `PodGroup` created by the matrixinfer base ob the `sample modelinfer`.

```sh
kubectl get podgroup-0 -oyaml

apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  annotations:
    scheduling.k8s.io/group-name: sample-0
  creationTimestamp: "2025-09-05T08:43:40Z"
  generation: 9
  labels:
    modelinfer.matrixinfer.ai/group-name: sample-0
    modelinfer.matrixinfer.ai/name: sample
  name: sample-0
  namespace: default
  ownerReferences:
  - apiVersion: workload.matrixinfer.ai/v1alpha1
    controller: true
    kind: ModelInfer
    name: sample
    uid: a08cd31a-9f39-450e-a3dc-bc868e08ce0a
  resourceVersion: "2621200"
  uid: 3abd9759-1fd7-48d7-be6b-ac55e17b36a0
spec:
  minMember: 6
  minResources: {}
  minTaskMember:
    decode-0: 3
    prefill-0: 3
  queue: default
status:
  conditions:
  - lastTransitionTime: "2025-09-05T09:10:47Z"
    reason: tasks in gang are ready to be scheduled
    status: "True"
    transitionID: fea87f6f-c172-4091-b55d-bd7160a7a801
    type: Scheduled
  phase: Running
  running: 6
```

## Clean up

```sh
kubectl delete sample

helm uninstall matrixinfe -n matrixinfer-system

helm uninstall volcano -n volcano-system
```
