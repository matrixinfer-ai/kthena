---
sidebar_position: 2
---

# Quick Start

Get up and running with MatrixInfer in minutes! This guide will walk you through deploying your first AI model.
We'll install a model from Hugging Face and perform inference using a simple curl command.

## Prerequisites

- MatrixInfer installed on your Kubernetes cluster (see [Installation](./installation.md))
- Access to a Kubernetes cluster with `kubectl` configured

## Step 1: Create a Model Resource

Create the example model in your namespace (replace `<your-namespace>` with your actual namespace):

```bash
kubectl apply -f /examples/model/Qwen2.5-0.5B-Instruct.yaml -n <your-namespace>
```

```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: demo
spec:
  backends:
    - name: "backend1"
      type: "vLLM"
      modelURI: "hf://Qwen/Qwen2.5-0.5B-Instruct"
      cacheURI: "hostpath:///tmp/cache"
      minReplicas: 1
      maxReplicas: 1
      env:
        - name: "HF_ENDPOINT"
          value: "https://hf-mirror.com/" # Optional: Use a Hugging Face mirror if you have some network issues
      workers:
        - type: "server"
          image: "vllm/vllm-openai:v0.7.1"
          replicas: 1
          pods: 1
          config:
            served-model-name: "Qwen2.5-0.5B-Instruct"
            max-model-len: 32768
            max-num-batched-tokens: 65536
            block-size: 128
            trust-remote-code: ""
            enable-prefix-caching: ""
          resources:
            limits:
              cpu: "2"
              memory: "8Gi"
            requests:
              cpu: "2"
              memory: "8Gi"
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
