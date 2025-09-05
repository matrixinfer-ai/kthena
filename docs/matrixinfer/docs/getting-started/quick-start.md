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
kubectl get model demo -o jsonpath='{.status.conditions}'
```

And the status section should look like this when the model is ready:

```json
[
  {
    "lastTransitionTime": "2025-09-05T02:14:16Z",
    "message": "Model initialized",
    "reason": "ModelCreating",
    "status": "True",
    "type": "Initialized"
  },
  {
    "lastTransitionTime": "2025-09-05T02:18:46Z",
    "message": "Model is ready",
    "reason": "ModelAvailable",
    "status": "True",
    "type": "Active"
  }
]
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

Use the following command to get the `<model-route-ip>`:

```bash
kubectl get svc networking-infer-gateway -o jsonpath='{.spec.clusterIP}' -n <your-namespace>
```

This IP can only be used inside the cluster. If you want to chat from outside the cluster, you can use the `EXTERNAL-IP`
of `networking-infer-gateway` after you bind it.
