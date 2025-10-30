# CertManager Integration

This page describes how to integrate Kthena with cert-manager for automated SSL/TLS certificate management.

## Overview

Kthena supports integration with [cert-manager](https://cert-manager.io/) to automatically provision and manage TLS certificates for various components. When enabled, cert-manager handles certificate lifecycle management including issuance, renewal, and rotation.

The integration covers:
- **Admission Webhooks**: TLS certificates for registry, workload, and networking webhook servers
- **Kthena Router**: TLS certificates for external API access
- **Internal Communication**: Service-to-service encrypted communication

## Prerequisites

Before enabling cert-manager integration, ensure that:

1. **cert-manager is installed** in your Kubernetes cluster. Follow the [official installation guide](https://cert-manager.io/docs/installation/).
2. **Sufficient RBAC permissions** are available for cert-manager to create and manage certificates in your namespace.

## Configuration

### Enabling cert-manager Integration

cert-manager integration is controlled through the `global.certManager.enabled` flag in your Helm values:

```yaml
global:
  certManager:
    # Enable cert-manager integration for all components
    enabled: true
```

When enabled, Kthena will automatically create:
- Self-signed `Issuer` resources for each component
- `Certificate` resources with appropriate DNS names
- TLS secrets for storing certificates and private keys

### Component-Specific Configuration

#### Kthena Router TLS

For external API access, configure the kthena router TLS settings:

```yaml
networking:
  kthenaRouter:
    enabled: true
    tls:
      enabled: true
      # The DNS name for your external domain
      dnsName: "your-domain.com"
      # Secret name to store the TLS certificate
      secretName: "kthena-router-tls"
```

#### Webhook Certificates

Webhook certificates are automatically configured when cert-manager is enabled. Each subchart (registry, workload, networking) creates its own webhook certificates with internal DNS names:

- `<subchart-name>-webhook.<namespace>.svc`
- `<subchart-name>-webhook.<namespace>.svc.cluster.local`

### Manual Certificate Management

If cert-manager is not available or you prefer manual certificate management, disable cert-manager and provide your own CA bundle:

```yaml
global:
  certManager:
    enabled: false
  webhook:
    # Base64-encoded CA bundle for webhook certificates
    caBundle: "LS0tLS1CRUdJTi..."
```

To generate a CA bundle:
```bash
cat /path/to/your/ca.crt | base64 | tr -d '\n'
```

## Certificate Resources

When cert-manager integration is enabled, the following resources are created:

### Issuers
- `<subchart-name>-webhook-issuer` (per subchart: registry, workload, networking)
- `<subchart-name>-router-issuer` (for kthena router)

### Certificates
- `<subchart-name>-webhook-cert` (per subchart: registry, workload, networking)
- `<subchart-name>-router-cert` (for kthena router)

### Secrets
- `<subchart-name>-webhook-certs` (per subchart: registry, workload, networking)
- `<secretName>` (configurable for kthena router)

## Troubleshooting

### Common Issues

#### cert-manager Not Found
```
Error: failed to create certificate: cert-manager.io/v1, Kind=Certificate not found
```
**Solution**: Install cert-manager in your cluster before enabling the integration.

#### Certificate Not Ready
```
Certificate is not ready: certificate request has not been approved
```
**Solution**: Check cert-manager logs and ensure the issuer is properly configured:
```bash
kubectl describe certificate <certificate-name> -n <namespace>
kubectl logs -n cert-manager deployment/cert-manager
```

#### Webhook Connection Refused
```
Error: connection refused when calling webhook
```
**Solution**: Verify webhook certificates are properly mounted and the service is accessible:
```bash
kubectl get certificates -n <namespace>
kubectl describe secret <webhook-secret-name> -n <namespace>
```

#### DNS Resolution Issues
```
Error: certificate validation failed for DNS name
```
**Solution**: Ensure DNS names in your configuration are resolvable and match your actual service endpoints.

### Debugging Commands

Check certificate status:
```bash
kubectl get certificates -n <namespace>
kubectl describe certificate <certificate-name> -n <namespace>
```

Verify issuer status:
```bash
kubectl get issuers -n <namespace>
kubectl describe issuer <issuer-name> -n <namespace>
```

Check cert-manager logs:
```bash
kubectl logs -n cert-manager deployment/cert-manager
kubectl logs -n cert-manager deployment/cert-manager-webhook
```

### Getting Help

If you encounter issues not covered here:
1. Check the [cert-manager documentation](https://cert-manager.io/docs/)
2. Review Kthena logs for certificate-related errors
3. Consult the [cert-manager troubleshooting guide](https://cert-manager.io/docs/troubleshooting/)