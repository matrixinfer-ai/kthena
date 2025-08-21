# Autoscaler
## Overview:
In the face of real-time changing inference requests, the required hardware resources also change in real-time. As an optional component in the Matrix Infer project running in a Kubernetes environment, Matrix Infer Autoscaler can dynamically adjust the number of deployed inference instances according to the real-time load of the inference instances. It achieves the effect of saving the usage cost of computing resources while ensuring the health of business indicators (such as SLO indicators).

## Function Introduction:
Matrix Infer Autoscaler regularly collects running metrics from the Pods to which the managed inference instances belong. It estimates the required number of inference instances based on one or more user-specified monitoring indicators and their corresponding expected values. Finally, it executes scaling operations in combination with the set scaling policies. Matrix Infer Autoscaler provides two granularities of scaling methods: Scaling Autoscale and Optimize Autoscale.

### Scaling Autoscale
Scaling Autoscale for a single type of inference instance: This method is similar to the behavior of KPA, supporting two modes: Stable and Panic. It performs scaling management for a single type of deployment form (engine deployed through the same Deployment or Model Infer CR) according to business indicators.

### Optimize Autoscale
For the same model, there are multiple different deployment methods for inference instances: such as heterogeneous resource types (GPU/NPU), types of inference engines (vLLM / Sglang), and even different running parameters (different TP/DP configuration parameters). The inference instances with the above different deployment methods can all provide normal inference services, presenting consistent business functions externally, but they require different hardware resources and provide different business capabilities.
Optimize Autoscale dynamically calculates the expected total number of this group of inference instances in each scheduling cycle based on running metrics, and dynamically adjusts the proportion of these inference instances with the same function but different deployment methods, so as to achieve the optimization goal of maximizing the utilization of hardware resources.
The details of how Optimize Autoscale regulates the proportion of various parameters will be introduced below, which can be understood as a greedy + 
doubling bin-packing algorithm.

#TODO Introduce algorithm details, which can be accompanied by a diagram.

## Constraints:
When using Matrix Infer Autoscale, the following points should be noted:

The same type of inference instance cannot have both Scaling Autoscale and Optimize Autoscale enabled. In other words, from an operational perspective, do not bind a Deployment or Matrix Infer CR to different Binding CRs at the same time.
Before using Optimize Autoscale, it is necessary to configure the cost for each type of inference instance. It is recommended that the cost parameter be filled with the computing power of each instance, and operators can obtain it by running the script in the Matrix Infer project.

## Typical Usage Scenarios