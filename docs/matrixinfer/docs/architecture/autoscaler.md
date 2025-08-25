# Autoscaler

Facing real-time changing inference requests, the required hardware resources also fluctuate dynamically. Matrix Infer Autoscaler, as an optional component of the Matrix Infer project running in a Kubernetes environment, dynamically adjusts the number of deployed inference instances based on their real-time load. It ensures healthy business metrics (such as SLO indicators) while reducing the consumption of computational resources.

## Feature Description

Matrix Infer Autoscaler periodically collects runtime metrics from the Pods of managed inference instances. Based on one or more monitoring metrics specified by the user and their corresponding target values, it estimates the required number of inference instances. Finally, it performs scaling operations according to the configured scaling policies.

![alt text](../../static/img/architecture-autoscaler.svg)

Matrix Infer Autoscaler provides two granularities of scaling methods: **Scaling Autoscale** and **Optimize Autoscale**.

### Scaling Autoscale

Scaling Autoscale targets scaling a single type of inference instance: This method is similar to the behavior of [KPA](https://knative.dev/docs/serving/autoscaling/kpa-specific/), supporting both Stable and Panic modes. It scales a single type of deployment (engines deployed via the same Deployment or Model Infer CR) based on business metrics.

### Optimize Autoscale

For the same model, inference instances can be deployed in multiple different ways: such as heterogeneous resource types (GPU/NPU), types of inference engines (vLLM / Sglang), or even different runtime parameters (e.g., TP/DP configuration parameters). While these differently deployed inference instances can all provide normal inference services and expose consistent business functionality externally, they differ in required hardware resources and provided business capabilities.

The functionality of Optimize Autoscale can be divided into two parts: predicting instance count and scheduling instances.
**Predicting instances** follows the same logic as Scaling Autoscale: dynamically calculating the total desired number of instances for the group of inference instances in each scheduling cycle based on runtime metrics.
**The scheduling phase** primarily dynamically adjusts the proportion of these functionally identical but differently deployed inference instances to achieve the optimization goal of maximizing hardware resource utilization. From a problem modeling perspective, the scheduling phase can be viewed as an integer programming problem. Considering the significant state transitions between scheduling cycles (due to the overhead of model cold starts).
Optimize Autoscale adopts a greedy algorithm with a doubling strategy. It first treats the sum of the differences between the maximum instance count `maxReplicas` and the minimum instance count `minReplicas` for each type of instance as the available capacity capacity, meaning there are capacity manageable instances. Each time, it selects to scale up a portion of them.
`costExpensionRate` reflects the aggressiveness of the scaling strategy. Optimize Autoscale lays out all capacity instances into a sequence seq. When the predicted result requires K instances, it retains the first K instances in the seq order. This strategy ensures, to some extent, that previously started instances can be reused during scaling to reduce cold start overhead.

• When `costExpensionRate` is 1.0, it is equivalent to sorting the capacity instances by cost in ascending order.
  
• When `costExpensionRate` is 2.0, it is equivalent to further grouping and batching the capacity instances of each type. Specifically, assuming the cost for the current type of instance is `cost_i`, and there are `capacity_i` instances, then `capacity_i` is split into log batches in the form of powers of 2. The number of instances in these batches is $$1, 2, 4, ..., capacity_i - 1 - 2 - 4 - 8 - ...$$, and the costs of these batches are $$cost_i, 2*cost_i, 4*cost_i, ..., (capacity_i - 1 - 2 - 4 - 8 - ...) * cost_i$$In summary, the algorithm mixes the batches of various inference instance types together at the batch granularity, sorts them by cost in ascending order, and then expands the sorted batches into corresponding combinations of multiple instances. Finally, a sequence seq of length capacity is obtained as the scaling order.

• More generally, when `costExpensionRate` is P, the number of instances in each batch is $$P^0 ,P^1,P^2+...,capacity - P^0 - P^1 - P^2$$ Thus, `costExpensionRate` can be considered as the "cost expansion ratio for the next batch" within each type of inference instance.

## Constraints

When using Matrix Infer Autoscaler, note the following:

- The same type of inference instance cannot have both Scaling Autoscale and Optimize Autoscale enabled simultaneously. In other words, from an operational perspective, do not bind the same Deployment or Matrix Infer CR to different Binding CRs at the same time.

- Before using Optimize Autoscale, you need to configure the cost for each type of inference instance. It is recommended to set the cost parameter to the computational power of each instance. Operators can run the script in the Matrix Infer project to obtain this value.