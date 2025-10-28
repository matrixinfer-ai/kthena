# Autoscaler Features

This page describes the autoscaling features and capabilities in Kthena.

## Overview

Facing real-time changing inference requests, the required hardware resources also fluctuate dynamically. Kthena Autoscaler, as an optional component of the Kthena project running in a Kubernetes environment, dynamically adjusts the number of deployed serving instances based on their real-time load. It ensures healthy business metrics (such as SLO indicators) while reducing the consumption of computational resources.

Kthena Autoscaler periodically collects runtime metrics from the Pods of managed serving instances. Based on one or more monitoring metrics specified by the user and their corresponding target values, it estimates the required number of serving instances. Finally, it performs scaling operations according to the configured scaling policies.

## Scaling Strategies

Kthena Autoscaler provides two granularities of scaling methods: **Homogeneous Instances Autoscale** and **Heterogeneous Instances Autoscale**.

### Homogeneous Instances Autoscale

Homogeneous instances autoscaling is used to manage a group of serving instances with identical configurations. It works through the following steps:

1. **Metric Collection**: Periodically collect monitoring metrics from all ready Pods
2. **Instance Count Calculation**: Calculate the required number of instances based on collected metrics and target values
3. **Scaling Execution**: Adjust the number of instances according to the calculation results and configured scaling policies

The core algorithm considers current instance count, minimum/maximum instance limits, tolerance percentage, and the impact of unready instances.

### Heterogeneous Instances Autoscale

For the same model, serving instances can be deployed in multiple different ways: such as heterogeneous resource types (GPU/NPU), types of inference engines (vLLM/Sglang), or even different runtime parameters (e.g., TP/DP configuration parameters). While these differently deployed serving instances can all provide normal inference services and expose consistent business functionality externally, they differ in required hardware resources and provided business capabilities.

The functionality of `Heterogeneous Instances Autoscale` can be divided into two parts: predicting instance count and scheduling instances.

**Predicting instances** follows the same logic as Homogeneous Instances Autoscale: dynamically calculating the total desired number of instances for the group of serving instances in each scheduling cycle based on runtime metrics.

Heterogeneous Instances Autoscale adopts a greedy algorithm with a doubling strategy:

1. Treat the sum of the differences between the maximum instance count `maxReplicas` and the minimum instance count `minReplicas` for each type of instance as the available `capacity`
2. Based on the idea of multiplication, divide the `capacity` into multiple batches according to the power of `costExpansionRate`
3. At the batch level, mix and sort batches of various serving instances by cost in ascending order
4. Expand the sorted batches into corresponding multiple instance combinations
5. Finally, obtain a sequence `seq` with a length of capacity as the scaling order

This method ensures that in resource-limited situations, instance types with lower costs are prioritized, thereby achieving cost optimization.

## Configuration

Kthena Autoscaler is configured through two custom resources: `AutoscalingPolicy` and `AutoscalingPolicyBinding`.

### Main Configuration Parameters

#### AutoscalingPolicy Configuration

- **Metrics**: Define monitoring metrics and target values for scaling
- **TolerancePercent**: Tolerance percentage to determine whether scaling is needed
- **Behavior**: Define scaling behavior, including:
  - **ScaleUp**: Configuration related to scaling up
    - **PanicPolicy**: Panic mode configuration
      - **PanicThresholdPercent**: Threshold to trigger panic mode
      - **PanicModeHold**: Duration for panic mode
    - **StablePolicy**: Stable mode configuration
      - **StabilizationWindow**: Stabilization window time
      - **Period**: Stable mode period
  - **ScaleDown**: Configuration related to scaling down
    - **StabilizationWindow**: Scale down stabilization window time
    - **Period**: Scale down period

#### AutoscalingPolicyBinding Configuration

- **ScalingConfiguration**: Define scaling configuration
  - **Target**: Target serving instance
  - **MinReplicas**: Minimum number of instances
  - **MaxReplicas**: Maximum number of instances
- **OptimizerConfiguration**: Optimizer configuration (for heterogeneous instance autoscaling)
  - **CostExpansionRatePercent**: Cost expansion rate
  - **Params**: Parameters for each instance type
    - **Target**: Target serving instance
    - **MinReplicas**: Minimum number of instances
    - **MaxReplicas**: Maximum number of instances
    - **Cost**: Instance cost

### Configuration Example

Here is a simple configuration example for homogeneous instance autoscaling:

```yaml
apiVersion: workload.serving.volcano.sh/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: example-policy
spec:
  metrics:
  - metricName: requests_per_second
    targetValue: 10.0
  tolerancePercent: 10
  behavior:
    scaleUp:
      panicPolicy:
        panicThresholdPercent: 150
        panicModeHold: 5m
      stablePolicy:
        stabilizationWindow: 1m
        period: 30s
    scaleDown:
      stabilizationWindow: 5m
      period: 1m
---
apiVersion: workload.serving.volcano.sh/v1alpha1
kind: AutoscalingPolicyBinding
metadata:
  name: example-binding
spec:
  scalingConfiguration:
    target:
      targetRef:
        name: example-model-serving
    minReplicas: 2
    maxReplicas: 10
```

## Monitoring

Kthena Autoscaler provides various monitoring capabilities to ensure the observability and controllability of scaling operations.

### Metric Collection Mechanism

- **Real-time Metric Collection**: Collect runtime metrics from each Pod, including request count, latency, resource usage, etc.
- **Sliding Window Storage**: Use sliding window data structures to store historical metric data, supporting statistics for different time windows
- **Histogram Statistics**: Support for statistical analysis of distributional metrics such as latency using histograms

### Key Monitoring Metrics

Kthena Autoscaler supports scaling decisions based on multiple metrics:

- **Request-related Metrics**: Such as requests per second (RPS), request latency, etc.
- **Resource Usage Metrics**: Such as CPU usage, memory usage, GPU/NPU utilization, etc.
- **Custom Metrics**: Support for user-defined specific business metrics

### State Management

Autoscaler maintains detailed state information, including:

- **Recommended Instance Count History**: Records the history of recommended instance counts
- **Corrected Instance Count History**: Records the history of instance counts after policy correction
- **Panic Mode State**: Tracks whether in panic mode and its end time

This state information is used to implement stable scaling decisions, avoid frequent scaling operations, and quickly respond to sudden load increases.

### Logs and Events

Autoscaler provides detailed logging, including:

- Scaling decision process
- Metric collection and processing
- Policy application and correction
- Error and warning information

By reviewing these logs, users can understand the reasons and processes behind scaling operations, facilitating debugging and configuration optimization.
