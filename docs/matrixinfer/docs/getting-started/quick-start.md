---
sidebar_position: 2
---

# Quick Start

Get up and running with MatrixInfer in minutes! This guide will walk you through deploying your first AI model.
We'll install a model from Hugging Face and perform inference using a simple curl command.

## Prerequisites

- MatrixInfer installed on your Kubernetes cluster (see [Installation](./installation.md))
- Access to a Kubernetes cluster with `kubectl` configured
- Pod in Kubernetes can access the internet

## Step 1: Create a Model Resource

Create the example model in your namespace (replace `<your-namespace>` with your actual namespace):

You can find the example YAML
file [here](https://github.com/matrixinfer-ai/matrixinfer/tree/main/examples/model/Qwen2.5-0.5B-Instruct.yaml).

```bash
kubectl apply -f https://raw.githubusercontent.com/matrixinfer-ai/matrixinfer/refs/heads/main/examples/model/Qwen2.5-0.5B-Instruct.yaml -n <your-namespace>
```

## Step 2: Wait for Model to be Ready

Wait model condition `Active` to become `true`. You can check the status using:

```bash
kubectl get model demo -o yaml
```

And the status section should look like this when the model is ready:

```bash
status:
  backendStatuses:
  - name: demo-backend1
    replicas: 1
  conditions:
  - lastTransitionTime: "2025-09-03T07:10:55Z"
    message: Model initialized
    reason: ModelCreating
    status: "True"
    type: Initialized
  - lastTransitionTime: "2025-09-03T07:12:35Z"
    message: Model is ready
    reason: ModelAvailable
    status: "True"
    type: Active
```

## Step 3: Perform Inference

You can now perform inference using the model. Here’s an example of how to send a request:

```bash
curl -X POST http://<model-route-ip>/v1/chat/completions \
-H "Content-Type: application/json" \
-d '{
  "model": "demo",
  "messages": [
    {
      "role": "user",
      "content": "Where is the capital of China?"
    }
  ],
  "stream": false
}'
```

Replace `<model-route-ip>` with the `CLUSTER-IP` of `networking-infer-gateway`

```bash
kubectl get svc networking-infer-gateway
```

or if you want to chat from outside the cluster, you can use the `EXTERNAL-IP` of `networking-infer-gateway` if it's
available.
