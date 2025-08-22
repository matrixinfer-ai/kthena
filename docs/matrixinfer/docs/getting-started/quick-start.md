---
sidebar_position: 2
---

# Quick Start

Get up and running with MatrixInfer in minutes! This guide will walk you through deploying your first AI model.

## Prerequisites

- MatrixInfer installed on your Kubernetes cluster (see [Installation](./installation.md))
- Access to a Kubernetes cluster with `kubectl` configured

## Step 1: Create a Model Resource

```bash
kubectl apply -f /examples/Deepseek-R1-Distill-llama-8B.yaml
```

## Step 2: Wait for Model to be Ready

Wait model status to become `Active`. You can check the status using:

```bash
kubectl get model Deepseek-R1-Distill-llama-8B -o yaml
```

## Step 3: Perform Inference

You can now perform inference using the model. Here’s an example of how to send a request:

```bash
curl -X POST http://<model-route-ip>/v1/chat/completions \
-H "Content-Type: application/json" \
-d '{
  "model": "deepseek-r1-distill-llama-8b", // the name of your model CR
  "messages": [
    {
      "role": "user",
      "content": "Where is the capital of France?"
    }
  ],
  "stream": false,
}'
```
