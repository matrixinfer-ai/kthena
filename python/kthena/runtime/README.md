# Kthena Runtime

This FastAPI application acts as a sidecar-style metrics proxy and utility service for inference engines. It fetches metrics from an engine endpoint, standardizes them into a Prometheus-compatible format, and exposes additional utility endpoints for LoRA adapter lifecycle and model downloading.

## Features

- Metric standardization for vLLM and SGLang
- Sidecar pattern to transparently proxy and normalize engine metrics
- LoRA utilities: download + load, and unload via simple HTTP APIs
- Standalone model downloader supporting multiple sources (S3/OBS/PVC/HuggingFace)

## Build

Build the Docker image (runtime stage):

```bash
cd python
# Build only the runtime target
docker build -t kthena-runtime:latest -f Dockerfile --target runtime .
```

## Run

CLI arguments:

- `-E, --engine` (required): inference engine name. Supported: `vllm`, `sglang`
- `-H, --host` (default: `0.0.0.0`): bind address
- `-P, --port` (default: `9000`): listening port
- `-B, --engine-base-url` (default: `http://localhost:8000`): engine base URL
- `-M, --engine-metrics-path` (default: `/metrics`): engine metrics endpoint path

Example (Docker):

```bash
docker run --rm -p 9000:9000 \
  kthena-runtime:latest \
  --engine vllm \
  --engine-base-url http://host.docker.internal:8000 \
  --engine-metrics-path /metrics
```

## HTTP Endpoints

### Health

- `GET /health`
- Returns service health status.

Response:
```json
{"status":"healthy","service":"runtime"}
```

### Metrics

- `GET /metrics`
- Proxies the engine metrics at `{engine_base_url}{engine_metrics_path}` and standardizes them based on the selected engine.
- Response content type: `text/plain; charset=utf-8`

Example:
```bash
curl -s http://localhost:9000/metrics
```

### LoRA: Download + Load

- `POST /v1/load_lora_adapter`
- Download a LoRA adapter and then request the engine to load it.

Request body:
- `lora_name` (string, required): LoRA adapter name
- `source` (string, optional): adapter source (supports `s3://`, `obs://`, `pvc://`, or HuggingFace model name, e.g., `hf://my-org/my-model`)
- `output_dir` (string, optional): local directory to store the adapter; also used as `lora_path` when loading to the engine
- `config` (object, optional): downloader config (e.g. `access_key`, `secret_key`, `endpoint`, `hf_token`, etc.)
- `max_workers` (int, optional, default: 8): parallel download workers
- `async_download` (bool, optional, default: false): run download in background


Examples:
```bash
# Synchronous download and load
curl -X POST http://localhost:9000/v1/load_lora_adapter \
  -H 'Content-Type: application/json' \
  -d '{
        "lora_name": "my-lora",
        "source": "hf://myorg/my-lora-repo",
        "output_dir": "/models/my-lora",
        "config": {"hf_token": "<token>"},
        "max_workers": 8,
        "async_download": false
      }'

# Async download and load
curl -X POST http://localhost:9000/v1/load_lora_adapter \
  -H 'Content-Type: application/json' \
  -d '{
        "lora_name": "my-lora",
        "source": "s3://bucket/path/to/lora",
        "output_dir": "/models/my-lora",
        "config": {"access_key": "...", "secret_key": "...", "endpoint": "..."},
        "async_download": true
      }'
```

### LoRA: Unload

- `POST /v1/unload_lora_adapter`
- Proxy to engine `POST {engine_base_url}/v1/unload_lora_adapter` with the request body as-is.

Example (body depends on engine contract):
```bash
curl -X POST http://localhost:9000/v1/unload_lora_adapter \
  -H 'Content-Type: application/json' \
  -d '{"lora_name": "my-lora"}'
```

### Model Downloader

- `POST /v1/download_model`
- Download a model from multiple sources.

Request body:
- `source` (string, required): model source (supports `s3://`, `obs://`, `pvc://`, or HuggingFace model name, e.g., `hf://my-org/my-model`)
- `output_dir` (string, required): local directory to store the model
- `config` (object, optional): downloader config (e.g. `access_key`, `secret_key`, `endpoint`, `hf_token`, etc.)
- `max_workers` (int, optional, default: 8): parallel download workers
- `async_download` (bool, optional, default: false): run download in background

Examples:
```bash
# Synchronous download
curl -X POST http://localhost:9000/v1/download_model \
  -H 'Content-Type: application/json' \
  -d '{
        "source": "hf://mistralai/Mistral-7B-Instruct-v0.3",
        "output_dir": "/models/mistral",
        "config": {"hf_token": "<token>"},
        "max_workers": 8
      }'

# Async download
curl -X POST http://localhost:9000/v1/download_model \
  -H 'Content-Type: application/json' \
  -d '{
        "source": "s3://bucket/path/to/model",
        "output_dir": "/models/modelA",
        "config": {"access_key": "...", "secret_key": "...", "endpoint": "..."},
        "async_download": true
      }'
```

## Environment Variables

- `REQUEST_TIMEOUT` (float, default: `30.0`): timeout (seconds) for outbound requests to the engine.

## Supported Engines

- `vllm`
- `sglang`

If an unsupported engine is specified, the service will fail to start with a clear error.

## License

This project is open-sourced under the Apache License 2.0. See the [LICENSE](../../../LICENSE) file for details.