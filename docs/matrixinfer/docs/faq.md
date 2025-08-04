---
sidebar_position: 10
---

# Frequently Asked Questions (FAQ)

Common questions and answers about MatrixInfer.

## General Questions

### What is MatrixInfer?

MatrixInfer is a Kubernetes-native platform for deploying and managing AI model inference workloads. It provides a comprehensive solution for running machine learning models in production with features like autoscaling, traffic routing, and monitoring.

### What types of models does MatrixInfer support?

MatrixInfer supports various ML frameworks including:
- **PyTorch** - Deep learning models
- **TensorFlow** - Machine learning and deep learning models
- **ONNX** - Cross-platform model format
- **HuggingFace** - Pre-trained transformer models
- **Custom** - Your own runtime implementations

### What are the system requirements?

**Minimum Requirements:**
- Kubernetes 1.20 or later
- 4 CPU cores and 8GB RAM per node
- Helm 3.0 or later
- kubectl configured with cluster admin permissions

**Recommended for Production:**
- Kubernetes 1.24 or later
- Multiple nodes with 8+ CPU cores and 16GB+ RAM
- GPU nodes for GPU-accelerated models
- Persistent storage for model artifacts

## Installation and Setup

### How do I install MatrixInfer?

The easiest way is using Helm:

```bash
helm repo add matrixinfer https://matrixinfer-ai.github.io/charts
helm install matrixinfer matrixinfer/matrixinfer \
  --namespace matrixinfer-system \
  --create-namespace
```

For detailed instructions, see the [Installation Guide](./getting-started/installation.md).

### Can I install MatrixInfer on managed Kubernetes services?

Yes, MatrixInfer works on all major managed Kubernetes services:
- **Amazon EKS**
- **Google GKE**
- **Azure AKS**
- **DigitalOcean Kubernetes**
- **Red Hat OpenShift**

### Do I need GPU nodes to use MatrixInfer?

No, GPU nodes are optional. MatrixInfer works perfectly with CPU-only deployments. GPU nodes are recommended for:
- Large language models (LLMs)
- Computer vision models
- Models requiring high throughput

## Model Management

### How do I deploy my first model?

1. Register your model:
```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: Model
metadata:
  name: my-model
spec:
  modelSpec:
    modelId: "my-model-v1"
    framework: "pytorch"
    version: "1.0.0"
    source:
      uri: "s3://my-bucket/model/"
  runtime:
    image: "matrixinfer/pytorch-runtime:latest"
```

2. Deploy for inference:
```yaml
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: my-model-infer
spec:
  modelRef:
    name: my-model
  replicas: 2
```

3. Expose via network:
```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: my-model-server
spec:
  modelInferRef:
    name: my-model-infer
  service:
    type: LoadBalancer
    port: 8080
  routing:
    path: "/v1/models/my-model"
```

### Where can I store my model artifacts?

MatrixInfer supports various storage backends:
- **S3-compatible storage** (AWS S3, MinIO, etc.)
- **Google Cloud Storage**
- **Azure Blob Storage**
- **HTTP/HTTPS URLs**
- **HuggingFace Model Hub**
- **Git repositories** (with Git LFS)

### How do I update a model to a new version?

1. Register the new model version
2. Update the ModelInfer resource to reference the new model
3. Use ModelRoute for gradual rollout (A/B testing)

Example:
```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelRoute
metadata:
  name: model-rollout
spec:
  routes:
  - destination:
      modelServerRef:
        name: model-v1-server
    weight: 80
  - destination:
      modelServerRef:
        name: model-v2-server
    weight: 20
```

## Scaling and Performance

### How does autoscaling work?

MatrixInfer uses Kubernetes Horizontal Pod Autoscaler (HPA) with custom metrics:

```yaml
spec:
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
    metrics:
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

### What's the typical latency I can expect?

Latency depends on several factors:
- **Model size and complexity**
- **Hardware resources** (CPU, GPU, memory)
- **Batch size**
- **Network latency**

Typical ranges:
- **Small models (< 100MB)**: 10-50ms
- **Medium models (100MB-1GB)**: 50-200ms
- **Large models (> 1GB)**: 200ms-2s

### How many requests per second can MatrixInfer handle?

Throughput varies based on:
- Model complexity
- Hardware resources
- Batch processing configuration
- Number of replicas

Example throughput ranges:
- **CPU-based inference**: 10-100 RPS per replica
- **GPU-based inference**: 100-1000 RPS per replica
- **Batch processing**: 10x-100x improvement possible

## Troubleshooting

### My model pods are not starting. What should I check?

1. **Check pod status and events:**
```bash
kubectl get pods -l app=my-model-infer
kubectl describe pod <pod-name>
```

2. **Verify model source accessibility:**
```bash
# Test from within cluster
kubectl run test-pod --image=curlimages/curl --rm -it -- \
  curl -I https://your-model-source.com
```

3. **Check resource requirements:**
```bash
kubectl top nodes
kubectl describe nodes
```

4. **Review logs:**
```bash
kubectl logs <pod-name>
```

### How do I debug high inference latency?

1. **Check resource utilization:**
```bash
kubectl top pods -l app=my-model-infer
```

2. **Monitor metrics:**
- Request latency percentiles
- Queue depth
- Resource utilization

3. **Optimize configuration:**
- Increase resource limits
- Enable batch processing
- Use GPU acceleration
- Optimize model (quantization, pruning)

### Models are failing to load. What could be wrong?

Common causes:
1. **Insufficient memory** - Increase memory limits
2. **Inaccessible model source** - Check credentials and network
3. **Incompatible runtime** - Verify framework and image compatibility
4. **Corrupted model files** - Validate model integrity

### How do I monitor MatrixInfer in production?

Set up comprehensive monitoring:

1. **Metrics collection** with Prometheus
2. **Visualization** with Grafana dashboards
3. **Alerting** with AlertManager
4. **Distributed tracing** with Jaeger
5. **Log aggregation** with Loki

See the [Monitoring Guide](./monitoring/) for detailed setup instructions.

## Security and Compliance

### How secure is MatrixInfer?

MatrixInfer follows Kubernetes security best practices:
- **RBAC** for access control
- **Network policies** for traffic isolation
- **Pod security standards** for container security
- **Secret management** for credentials
- **TLS encryption** for data in transit

### Can I use MatrixInfer in air-gapped environments?

Yes, MatrixInfer supports air-gapped deployments:
1. Use private container registries
2. Store model artifacts in internal storage
3. Configure custom CA certificates
4. Use offline Helm charts

### How do I implement authentication for model endpoints?

Configure authentication in ModelServer:

```yaml
spec:
  authentication:
    enabled: true
    type: "jwt"
    config:
      issuer: "https://your-auth-provider.com"
      audience: "matrixinfer-api"
```

Supported authentication methods:
- **JWT tokens**
- **API keys**
- **mTLS certificates**
- **OAuth 2.0**

## Integration and Development

### Can I integrate MatrixInfer with my CI/CD pipeline?

Yes, MatrixInfer integrates well with:
- **GitOps** (ArgoCD, Flux)
- **CI/CD platforms** (Jenkins, GitHub Actions, GitLab CI)
- **MLOps tools** (MLflow, Kubeflow, DVC)

Example GitHub Actions workflow:
```yaml
- name: Deploy Model
  run: |
    kubectl apply -f model.yaml
    kubectl apply -f modelinfer.yaml
    kubectl apply -f modelserver.yaml
```

### How do I develop custom runtimes?

1. Create a container image with your runtime
2. Implement the inference API endpoints
3. Add health check endpoints
4. Configure metrics collection
5. Reference in Model resource:

```yaml
spec:
  runtime:
    image: "myregistry.com/custom-runtime:v1.0.0"
    command: ["/usr/local/bin/my-server"]
    args: ["--port", "8080"]
```

### Can I use MatrixInfer with service mesh?

Yes, MatrixInfer works with popular service meshes:
- **Istio**
- **Linkerd**
- **Consul Connect**

Benefits include:
- Advanced traffic management
- Enhanced security policies
- Detailed observability
- Circuit breaking and retries

## Cost and Licensing

### Is MatrixInfer free to use?

Yes, MatrixInfer is open source and free to use under the Apache 2.0 license.

### How can I optimize costs?

Cost optimization strategies:
1. **Right-size resources** based on actual usage
2. **Use spot instances** for non-critical workloads
3. **Implement autoscaling** to scale down during low usage
4. **Schedule scaling** for predictable traffic patterns
5. **Use resource quotas** to prevent over-provisioning

### Are there commercial support options?

Community support is available through:
- **GitHub Issues** for bug reports and feature requests
- **GitHub Discussions** for questions and community help
- **Documentation** for comprehensive guides

## Getting Help

### Where can I find more documentation?

- [Getting Started Guide](./getting-started/installation.md)
- [API Reference](./api/overview.md)
- [Examples](./examples/)
- [Best Practices](./best-practices/)
- [Troubleshooting Guide](./troubleshooting.md)

### How do I report bugs or request features?

1. **Search existing issues** on GitHub
2. **Create a new issue** with detailed information
3. **Provide reproduction steps** and environment details
4. **Include logs and configuration** when relevant

### How can I contribute to MatrixInfer?

We welcome contributions! Ways to contribute:
1. **Report bugs** and suggest improvements
2. **Submit pull requests** for fixes and features
3. **Improve documentation**
4. **Share examples** and use cases
5. **Help other users** in discussions

See our [Contributing Guide](https://github.com/matrixinfer-ai/matrixinfer/blob/main/CONTRIBUTING.md) for details.

### Where can I get community support?

- **GitHub Discussions**: https://github.com/matrixinfer-ai/matrixinfer/discussions
- **GitHub Issues**: https://github.com/matrixinfer-ai/matrixinfer/issues
- **Documentation**: https://matrixinfer.ai/docs

For urgent issues, please create a GitHub issue with the "urgent" label.