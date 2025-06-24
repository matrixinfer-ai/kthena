# MatrixInfer Runtime

This FastAPI application acts as a metrics collection proxy for inference engines. It retrieves metrics from a specified inference engine endpoint, processes them according to the engine's metric standard, and exposes them in a standardized Prometheus-compatible format.

## Features

- **Metric Standardization:** Unify inference runtime metrics for both SGLang and vLLM frameworks to reduce operational complexity and costs.
- **Sidecar Pattern:** Deploy a Runtime container within Kubernetes Pods to transparently relay requests to the Engine container, enabling metric standardization.
- Extensibility: Support integration with other inference engines.

## Installation

Build the Docker image:

```bash
cd python
docker build -t matrixinfer-runtime:latest -f Dockerfile --target runtime .
```

## Usage

### Docker Command Options

```bash
docker run -E vllm
```

Parameters:

- `-E, --enginc`: Inference engine name (e.g., vllm, sglang)(required)
- `-H, --host`: Host address to bind (default: 0.0.0.0)
- `-P, --port`: Port to listen on (default: 9000)
- `-U, --url`: Engine metrics endpoint URL (default: http://localhost:8000/metrics)

### Examples


```bash
docker run --port 8100 --engine vllm --url "http://localhost:8000/metrics"
```

## Configuration

### Environment Variables

| Variable        | Default | Description                           |
|:----------------|:--------|:--------------------------------------|
| REQUEST_TIMEOUT | 30.0    | Timeout (seconds) for engine requests |


## License

This project is open-sourced under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.