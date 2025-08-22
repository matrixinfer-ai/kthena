# Runtime

MatrixInfer Runtime is a lightweight sidecar service designed to standardize Prometheus metrics from inference engines, provides LoRA adapter download/load/unload capabilities, and supports model downloading.

## Overview

- Metrics standardization: fetch native metrics from the engine's /metrics endpoint, rename them to unified MatrixInfer metrics according to rules.
- LoRA lifecycle management: simple HTTP APIs to download+load and unload LoRA adapters for dynamic enable/disable.
- Model downloading: supports downloading models from S3/OBS/PVC/HuggingFace to a local path.

## Installation

- Runtime does not require separate installation. As part of ModelInfer, it will be automatically deployed in the ModelInfer Pod.
- When deploying via the Model CR (one-stop deployment), no additional configuration is needed; ModelInfer will automatically enable the runtime feature.
- For standalone deployment using ModelInfer YAML, you can add the following configuration to start Runtime:

  ```
  - name: runtime
    ports:
      - containerPort: 8900
    image: matrixinfer/runtime:latest
    args:
      - --port
      - "8900"
      - --engine
      - vllm
      - --engine-base-url
      - http://localhost:8000
      - --engine-metrics-path
      - /metrics
      - --pod
      - $(POD_NAME).$(NAMESPACE)
      - --model
      - test-model
    env:
      - name: ENDPOINT
        value: https://obs.test.com
      - name: RUNTIME_PORT
        value: "8900"
      - name: POD_NAME
        valueFrom:
          fieldRef:
            fieldPath: metadata.name
      - name: NAMESPACE
        valueFrom:
          fieldRef:
            fieldPath: metadata.namespace
      - name: VLLM_USE_V1
        value: "1"
    envFrom:
      - secretRef:
          name: "test-secret"
    readinessProbe:
      httpGet:
        path: /health
        port: 8900
      initialDelaySeconds: 5
      periodSeconds: 10
    resources: { }
  ```

Startup arguments:

- `-E, --engine` (required): engine name, supports `vllm`, `sglang`
- `-H, --host` (default `0.0.0.0`): listen address for Runtime
- `-P, --port` (default `9000`): listen port for Runtime
- `-B, --engine-base-url` (default `http://localhost:8000`): engine base URL
- `-M, --engine-metrics-path` (default `/metrics`): engine metrics path
- `-I, --pod` (required): current instance/Pod identifier, used for events and Redis keys
- `-N, --model` (required): model name

In the Model CR, you can control Runtime startup values via `spec.backends.env`:

```
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  annotations:
    api.kubernetes.io/name: example
  name: qwen25
spec:
  name: qwen25-coder-32b
  owner: example
  backends:
    - name: "qwen25-coder-32b-server"
      type: "vLLM" # --engine
      modelURI: s3://matrixinfer/Qwen/Qwen2.5-Coder-32B-Instruct
      cacheURI: hostpath:///cache/
      envFrom:
        - secretRef:
            name: your-secrets
      env:
        - name: "RUNTIME_PORT"  # default 8100
          value: "8200"
        - name: "RUNTIME_URL"   # default http://localhost:8000/metrics
          value: "http://localhost:8100"
        - name: "RUNTIME_METRICS_PATH" # default /metrics
          value: "/metrics"
      minReplicas: 1
      maxReplicas: 2
      workers:
        - type: server
          image: openeuler/vllm-ascend:latest
          replicase: 1
          pods: 1
          resources:
            limits:
              cpu: "8"
              memory: 96Gi
              huawei.com/ascend-1980: "2"
            requests:
              cpu: "1"
              memory: 96Gi
              huawei.com/ascend-1980: "2"
```

## Metric Standardization

Runtime renames key metrics from different engines to unified names prefixed with `matrixinfer:*` for consistent observability (Prometheus/Grafana):

- `matrixinfer:generation_tokens_total`
- `matrixinfer:num_requests_waiting`
- `matrixinfer:time_to_first_token_seconds`
- `matrixinfer:time_per_output_token_seconds`
- `matrixinfer:e2e_request_latency_seconds`

Notes:

- When `engine=vllm` or `engine=sglang`, key metrics from vLLM/SGLang are renamed to the standard names above.
- Only metrics covered by built-in mappings are standardized, and the original metrics are preserved. You can obtain all raw engine metrics plus the standardized metrics.

## Model Downloading

- `POST /v1/download_model`: download a model from multiple sources to a local directory.
    - body fields:
        - `source` (required): model source, supports `s3://`, `obs://`, `pvc://`, or a Hugging Face repository name in the format `<namespace>/<repo_name>`
        - `output_dir` (required): local output directory
        - `config` (optional, JSON string): download configuration (e.g., `hf_token`, `hf_endpoint`, `hf_revision`, `access_key`, `secret_key`, `endpoint`, etc.). Note: this field must be a JSON string. These values can also be provided via container environment variables (see below); Runtime will read them automatically.
        - `max_workers` (optional, default 8): number of concurrent download workers
        - `async_download` (optional, default false): whether to download in background

### Sources and Formats

- Hugging Face: `<namespace>/<repo_name>`, e.g., `microsoft/phi-2`
- S3: `s3://bucket/path`
- OBS: `obs://bucket/path`
- PVC: `pvc://path`

### Environment Variables and Parameter Configuration

You can provide authentication and download parameters via container environment variables or the `config` JSON string:

- Hugging Face:
  - `HF_AUTH_TOKEN` (optional): token for accessing private models
  - `HF_ENDPOINT` (optional): custom HF API endpoint
  - `HF_REVISION` (optional): model branch/revision (e.g., `main`)
- S3/OBS:
  - `ACCESS_KEY`, `SECRET_KEY`: access credentials (recommended to store in a Secret and load via `envFrom.secretRef.name`)
  - `ENDPOINT`: object storage service endpoint (e.g., `https://s3.us-east-1.amazonaws.com` or `https://obs.test.com`)

Equivalent `config` JSON (as a string) example:

```json
{
  "hf_token": "your_huggingface_token",
  "hf_endpoint": "custom_endpoint",
  "hf_revision": "main",
  "access_key": "your_access_key",
  "secret_key": "your_secret_key",
  "endpoint": "your_endpoint_url"
}
```

> Tip: If the above parameters are already provided via container environment variables, you can omit the `config` field in the request body.

### curl Examples

- Hugging Face model download:

```bash
curl -X POST "http://localhost:8900/v1/download_model" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "microsoft/phi-2",
    "output_dir": "/models/phi-2",
    "config": "{\"hf_token\":\"$HF_AUTH_TOKEN\",\"hf_revision\":\"main\"}",
    "max_workers": 8,
    "async_download": false
  }'
```

- S3 model download (private bucket example):

```bash
curl -X POST "http://localhost:8900/v1/download_model" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "s3://my-bucket/models/llama3",
    "output_dir": "/models/llama3",
    "config": "{\"access_key\":\"YOUR_KEY\",\"secret_key\":\"YOUR_SECRET\",\"endpoint\":\"https://s3.us-east-1.amazonaws.com\"}",
    "max_workers": 8
  }'
```

- OBS model download:

```bash
curl -X POST "http://localhost:8900/v1/download_model" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "obs://my-bucket/qwen",
    "output_dir": "/models/qwen",
    "config": "{\"access_key\":\"YOUR_KEY\",\"secret_key\":\"YOUR_SECRET\",\"endpoint\":\"https://obs.test.com\"}"
  }'
```

- PVC path download (no authentication required):

```bash
curl -X POST "http://localhost:8900/v1/download_model" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "pvc://models",
    "output_dir": "/models/local"
  }'
```

## Model Configuration APIs (LoRA) 

After the base model and Runtime are ready, you can dynamically download, load, and unload LoRA adapters via Runtime APIs.

### curl Request Examples

- Download and load a LoRA (Hugging Face source, synchronous):

```bash
curl -X POST "http://localhost:8900/v1/load_lora_adapter" \
  -H "Content-Type: application/json" \
  -d '{
    "lora_name": "qwen-lora",
    "source": "your-org/your-lora-repo",
    "output_dir": "/models/lora/qwen",
    "config": "{\"hf_token\":\"$HF_AUTH_TOKEN\",\"hf_revision\":\"main\"}",
    "max_workers": 8,
    "async_download": false
  }'
```

- Download and load a LoRA (S3 source, asynchronous/background):

```bash
curl -X POST "http://localhost:8900/v1/load_lora_adapter" \
  -H "Content-Type: application/json" \
  -d '{
    "lora_name": "qwen-lora",
    "source": "s3://my-bucket/lora/qwen",
    "output_dir": "/models/lora/qwen",
    "config": "{\"access_key\":\"YOUR_KEY\",\"secret_key\":\"YOUR_SECRET\",\"endpoint\":\"https://s3.us-east-1.amazonaws.com\"}",
    "async_download": true
  }'
```

- Unload a LoRA (example for vLLM; pass through the fields required by your engine):

```bash
curl -X POST "http://localhost:8900/v1/unload_lora_adapter" \
  -H "Content-Type: application/json" \
  -d '{
    "lora_name": "qwen-lora"
  }'
```
