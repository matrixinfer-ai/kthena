---
sidebar_position: 8
---

# Troubleshooting

Common issues and solutions when working with MatrixInfer.

## Installation Issues

### CRD Installation Fails

**Problem**: Custom Resource Definitions (CRDs) fail to install.

**Symptoms**:
```bash
error validating data: ValidationError(CustomResourceDefinition.spec): unknown field "preserveUnknownFields"
```

**Solutions**:
1. **Check Kubernetes version**: Ensure you're running Kubernetes 1.20 or later
   ```bash
   kubectl version --short
   ```

2. **Verify cluster admin permissions**:
   ```bash
   kubectl auth can-i create customresourcedefinitions
   ```

3. **Clean up existing CRDs** (if upgrading):
   ```bash
   kubectl get crd | grep matrixinfer
   kubectl delete crd <crd-name>
   ```

### Controller Pods Not Starting

**Problem**: MatrixInfer controller pods remain in Pending or CrashLoopBackOff state.

**Symptoms**:
```bash
kubectl get pods -n matrixinfer-system
NAME                                     READY   STATUS             RESTARTS   AGE
matrixinfer-controller-7d4b8c9f8-xyz12   0/1     CrashLoopBackOff   5          5m
```

**Solutions**:
1. **Check pod logs**:
   ```bash
   kubectl logs -n matrixinfer-system matrixinfer-controller-7d4b8c9f8-xyz12
   ```

2. **Verify resource requirements**:
   ```bash
   kubectl describe pod -n matrixinfer-system matrixinfer-controller-7d4b8c9f8-xyz12
   ```

3. **Check node resources**:
   ```bash
   kubectl top nodes
   kubectl describe nodes
   ```

## Model Registration Issues

### Model Source Not Accessible

**Problem**: Model fails to register due to inaccessible source URI.

**Symptoms**:
```bash
kubectl describe model my-model
# Events show: Failed to download model from source
```

**Solutions**:
1. **Verify source URI format**:
   - S3: `s3://bucket-name/path/to/model/`
   - HuggingFace: `huggingface://model-name`
   - HTTP: `https://example.com/model.tar.gz`

2. **Check credentials** (for private sources):
   ```bash
   # For S3
   kubectl create secret generic s3-credentials \
     --from-literal=access-key=YOUR_ACCESS_KEY \
     --from-literal=secret-key=YOUR_SECRET_KEY
   
   # For HuggingFace
   kubectl create secret generic hf-token \
     --from-literal=token=YOUR_HF_TOKEN
   ```

3. **Test connectivity**:
   ```bash
   # From a pod in the cluster
   kubectl run test-pod --image=curlimages/curl --rm -it -- \
     curl -I https://your-model-source.com
   ```

### Model Status Stuck in Pending

**Problem**: Model resource remains in Pending state.

**Solutions**:
1. **Check model events**:
   ```bash
   kubectl describe model my-model
   ```

2. **Verify runtime image**:
   ```bash
   docker pull matrixinfer/pytorch-runtime:latest
   ```

3. **Check controller logs**:
   ```bash
   kubectl logs -n matrixinfer-system -l app=matrixinfer-controller
   ```

## Inference Deployment Issues

### ModelInfer Pods Not Starting

**Problem**: ModelInfer pods fail to start or remain in ImagePullBackOff.

**Solutions**:
1. **Check image availability**:
   ```bash
   kubectl describe pod -l app=my-model-infer
   ```

2. **Verify model reference**:
   ```bash
   kubectl get model my-model -o yaml
   kubectl get modelinfer my-model-infer -o yaml
   ```

3. **Check resource limits**:
   ```bash
   kubectl top pods -l app=my-model-infer
   ```

### Autoscaling Not Working

**Problem**: ModelInfer doesn't scale based on CPU/memory usage.

**Solutions**:
1. **Check HPA status**:
   ```bash
   kubectl get hpa
   kubectl describe hpa my-model-infer
   ```

2. **Verify metrics server**:
   ```bash
   kubectl get pods -n kube-system | grep metrics-server
   kubectl top pods
   ```

3. **Check resource requests** (required for HPA):
   ```yaml
   spec:
     resources:
       requests:  # Must be specified
         cpu: "100m"
         memory: "256Mi"
   ```

## Networking Issues

### ModelServer Service Not Accessible

**Problem**: Cannot access model inference endpoints.

**Solutions**:
1. **Check service status**:
   ```bash
   kubectl get svc my-model-server
   kubectl describe svc my-model-server
   ```

2. **Verify endpoints**:
   ```bash
   kubectl get endpoints my-model-server
   ```

3. **Test internal connectivity**:
   ```bash
   kubectl run test-pod --image=curlimages/curl --rm -it -- \
     curl http://my-model-server:8080/health
   ```

4. **Check ingress/load balancer** (for external access):
   ```bash
   kubectl get ingress
   kubectl describe ingress my-model-ingress
   ```

### ModelRoute Traffic Not Routing Correctly

**Problem**: Traffic routing rules not working as expected.

**Solutions**:
1. **Check route status**:
   ```bash
   kubectl get modelroute my-route
   kubectl describe modelroute my-route
   ```

2. **Verify header/query parameter matching**:
   ```bash
   # Test with specific headers
   curl -H "version: v1" http://my-gateway/api/infer
   curl -H "version: v2" http://my-gateway/api/infer
   ```

3. **Check gateway logs**:
   ```bash
   kubectl logs -l app=matrixinfer-gateway
   ```

## Performance Issues

### High Inference Latency

**Problem**: Model inference requests are slow.

**Solutions**:
1. **Check resource utilization**:
   ```bash
   kubectl top pods -l app=my-model-infer
   ```

2. **Verify GPU allocation** (if using GPUs):
   ```bash
   kubectl describe pod my-model-infer-xyz
   # Look for nvidia.com/gpu in resources
   ```

3. **Check model loading time**:
   ```bash
   kubectl logs my-model-infer-xyz | grep "Model loaded"
   ```

4. **Optimize resource requests/limits**:
   ```yaml
   resources:
     requests:
       memory: "4Gi"      # Increase if model is large
       cpu: "2"           # Increase for CPU-intensive models
     limits:
       memory: "8Gi"
       cpu: "4"
   ```

### Memory Issues

**Problem**: Pods getting OOMKilled or running out of memory.

**Solutions**:
1. **Check memory usage**:
   ```bash
   kubectl top pods -l app=my-model-infer
   ```

2. **Increase memory limits**:
   ```yaml
   resources:
     limits:
       memory: "16Gi"  # Adjust based on model size
   ```

3. **Enable memory profiling**:
   ```yaml
   spec:
     env:
     - name: PYTORCH_PROFILER_ENABLED
       value: "true"
   ```

## Monitoring and Debugging

### Enable Debug Logging

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: matrixinfer-config
data:
  log-level: "debug"
```

### Collect Diagnostic Information

```bash
# Collect all MatrixInfer resources
kubectl get models,modelinfers,modelservers,modelroutes -A -o yaml > matrixinfer-resources.yaml

# Collect pod logs
kubectl logs -n matrixinfer-system -l app=matrixinfer-controller > controller.log

# Collect events
kubectl get events --sort-by=.metadata.creationTimestamp > events.log
```

### Common Log Messages

| Log Message | Meaning | Action |
|-------------|---------|--------|
| `Model source not found` | Source URI is inaccessible | Check URI and credentials |
| `Image pull failed` | Container image not available | Verify image name and registry access |
| `Resource quota exceeded` | Not enough cluster resources | Check resource limits and node capacity |
| `CRD not found` | Custom resources not installed | Reinstall MatrixInfer CRDs |

## Getting Help

If you're still experiencing issues:

1. **Check the [FAQ](faq.md)** for common questions
2. **Search existing [GitHub issues](https://github.com/matrixinfer-ai/matrixinfer/issues)**
3. **Create a new issue** with:
   - MatrixInfer version
   - Kubernetes version
   - Resource YAML files
   - Relevant logs
   - Steps to reproduce

## Related Documentation

- [Installation Guide](./getting-started/installation.md)
- [Quick Start](./getting-started/quick-start.md)
- [API Reference](./api/overview.md)
- [Best Practices](./best-practices/)