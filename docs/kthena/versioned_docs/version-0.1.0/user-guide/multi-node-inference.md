# Multi-Node Inference

This page describes the multi-node inference capabilities in Kthena, base on real-world examples and configurations.

## Overview

With the development of LLM, the scale of model parameters has grown exponentially, and the resource limits of a single conventional virtual machine or physical server can no longer meet the computational demands of these LLM.

The industry has proposed various innovative optimization strategies, such as PD-disaggregation and hybrid deployment of large and small models. These strategies have significantly changed the execution pattern of inference tasks, making inference instances no longer limited to the level of a single pod, but rather evolving into scenarios where multiple pods collaboratively complete a single inference prediction.

To address this issue, kthena provides a new `ModelServing` CR to describe specific inference deployment, enabling flexible and diverse deployment methods for inference task pods.

For a detailed definition of the `ModelServing`, please refer to the [ModelServing Reference](../reference/crd/workload.serving.volcano.sh.md) pages.

## Preparation

### Prerequisites

- Kubernetes cluster with Kthena installed and [volcano](https://volcano.sh/en/docs/installation/) installed
- Access to the Kthena examples repository
- Basic understanding of ModelServing CRD

### Getting Started

Deploy [llama LLM inference engine](../assets/examples/model-serving/multi-node.yaml). Set the tensor parallel size is 8 and the pipeline parallel size is 2.

You can run the following command to check the ModelServing status and pod status in the cluster.

```sh
kubectl get modelServing -oyaml | grep status -A 10

status:
  availableReplicas: 1
  conditions:
  - lastTransitionTime: "2025-09-05T08:53:25Z"
    message: All Serving groups are ready
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

kubectl get pod -owide -l modelserving.volcano.sh/name=llama-multinode

NAMESPACE   NAME                          READY   STATUS    RESTARTS   AGE     IP            NODE           NOMINATED NODE   READINESS GATES
default     llama-multinode-0-405b-0-0    1/1     Running   0          15m     10.244.0.56   192.168.5.12   <none>           <none>
default     llama-multinode-0-405b-0-1    1/1     Running   0          15m     10.244.0.58   192.168.5.43   <none>           <none>
default     llama-multinode-0-405b-1-0    1/1     Running   0          15m     10.244.0.57   192.168.5.58   <none>           <none>
default     llama-multinode-0-405b-1-1    1/1     Running   0          15m     10.244.0.53   192.168.5.36   <none>           <none>
```

**Note:** The first number in the pod name indicates which `ServingGroup` this pod belongs to. The second number indicates which `Role` it belongs to. The third number indicates the pod's sequence number within `Role`.

## Scaling

ModelServing supports scale strategies at two levels: `ServingGroup` and `Role`.

You can modify `modelServing.Spec.Replicas` to trigger scaling at the group level.
Additionally, modifying `modelServing.Spec.Template.Role.Replicas` triggers role-level scaling.

### Role Level Scale Down

Reduce the `modelServing.Spec.Template.Role.Replicas` from 2 to 1. To trigger a `Role Level` scale down.

You can see the result:

```sh
kubectl get pod -l modelserving.volcano.sh/name=llama-multinode

NAMESPACE            NAME                                          READY   STATUS    RESTARTS   AGE
default              llama-multinode-0-405b-0-0                    1/1     Running   0          28m
default              llama-multinode-0-405b-0-1                    1/1     Running   0          28m
```

You can see that all pods in `Role1` have been deleted.

### ServingGroup Level Scale Up

Add the `modelServing.Spec.Replicas` from 1 to 2. To trigger a `ServingGroup Level` scale up.

You can see the result:

```sh
kubectl get pod -l modelserving.volcano.sh/name=llama-multinode

NAMESPACE            NAME                                          READY   STATUS    RESTARTS   AGE
default              llama-multinode-0-405b-0-0                    1/1     Running   0          35m
default              llama-multinode-0-405b-0-1                    1/1     Running   0          35m
default              llama-multinode-1-405b-0-0                    1/1     Running   0          2m
default              llama-multinode-1-405b-0-1                    1/1     Running   0          2m
```

You can see that all roles in `ServingGroup1` are created.

You can also scale both the `ServingGroup` and `Role Level`.

## Rolling Update

Currently, `ModelServing` supports rolling upgrades at the `ServingGroup` level, enabling users to configure `Partitions` to control the rolling process.

- Partition: Indicates the ordinal at which the `ModelServing` should be partitioned for updates. During a rolling update, replicas with an ordinal greater than or equal to `Partition` will be updated. Replicas with an ordinal less than `Partition` will not be updated.
  
### ServingGroup Rolling Update

We configure the corresponding rolling update strategy for llama-multinode.

```yaml
spec:
  rolloutStrategy:
    type: ServingGroupRollingUpdate
    rollingUpdateConfiguration:
      partition: 1
```

Modifying the parameters of entryTemplate or workerTemplate triggers a rolling update. For example, changing the image.

You can see the result:

```sh
kubectl get pods -l modelserving.volcano.sh/name=llama-multinode -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].image}{"\n"}{end}'

llama-multinode-0-405b-0-0        vllm/vllm-openai:latest    
llama-multinode-0-405b-0-1        vllm/vllm-openai:latest        
llama-multinode-1-405b-0-0        vllm/vllm-openai:v0.10.1                  
llama-multinode-1-405b-0-1        vllm/vllm-openai:v0.10.1                 
```

From the pod runtime, it can be seen that only group 1 has been updated. Because we have set rolloutStrategy.partition = 1.

## Gang Scheduling and Network Topology

Gang scheduling is a feature that allows pods to be scheduled together. This is useful when you have a set of pods that need to be scheduled together. For example, you may have a set of pods that need to be scheduled together because they are pods of the same model.

In kthena, we use [Volcano gang scheduling](https://volcano.sh/en/docs/v1-10-0/plugins/#gang) to ensure that required pods are scheduled concurrently.

Use the following command to install Volcano:

```sh
helm repo add volcano-sh https://volcano-sh.github.io/helm-charts

helm repo update

helm install volcano volcano-sh/volcano -n volcano-system --create-namespace
```

We will create podGroups based on modelServing. Among these, the important one is `MinRoleReplicas`. Defines the minimum number of replicas required for each role in gang scheduling. This map allows users to specify different minimum replica requirements for different roles.

- **Key:** role name
- **Value:** minimum number of replicas required for that role

Additionally, it supports using `network topology` to reduce network latency among pods within the same podGroup.

Before using the network topology, you need to create a [hyper node](https://volcano.sh/en/docs/network_topology_aware_scheduling/) for Volcano.

PodGroup can set the topology constraints of the job through the networkTopology field, supporting the following configurations:

- **mode:** Supports hard and soft modes.
  - hard: Hard constraint, tasks within the job must be deployed within the same HyperNode.
  - soft: Soft constraint, tasks are deployed within the same HyperNode as much as possible.
- **highestTierAllowed:** Used with hard mode, indicating the highest tier of HyperNode allowed for job deployment. This field is not required when mode is soft.

You can run the follow command to see the `PodGroup` created by the kthena base ob the `llama-multinode modelServing`.

```sh
kubectl get podgroup-0 -oyaml

apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  annotations:
    scheduling.k8s.io/group-name: llama-multinode-0
  creationTimestamp: "2025-09-05T08:43:40Z"
  generation: 9
  labels:
    modelserving.volcano.sh/group-name: llama-multinode-0
    modelserving.volcano.sh/name: llama-multinode
  name: llama-multinode-0
  namespace: default
  ownerReferences:
  - apiVersion: workload.volcano.sh/v1alpha1
    controller: true
    kind: ModelServing
    name: llama-multinode
    uid: a08cd31a-9f39-450e-a3dc-bc868e08ce0a
  resourceVersion: "2621200"
  uid: 3abd9759-1fd7-48d7-be6b-ac55e17b36a0
spec:
  minMember: 2
  minResources: {}
  minTaskMember:
    405b: 2
  queue: default
status:
  conditions:
  - lastTransitionTime: "2025-09-05T09:10:47Z"
    reason: tasks in gang are ready to be scheduled
    status: "True"
    transitionID: fea87f6f-c172-4091-b55d-bd7160a7a801
    type: Scheduled
  phase: Running
  running: 2
```

## Clean up

```sh
kubectl delete modelserving llama-multinode

helm uninstall matrixinfe -n kthena-system

helm uninstall volcano -n volcano-system
```
