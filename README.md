# matrixinfer

## Webhook Certificate Configuration

The matrixinfer project includes webhooks for validating and mutating resources. These webhooks require TLS certificates to function securely. There are two ways to configure certificates for the webhooks:

### Using cert-manager (Default)

By default, the Helm chart is configured to use cert-manager to automatically provision and manage certificates for the webhooks. This requires cert-manager to be installed in your cluster.

To enable cert-manager integration, set the following in your Helm values:

```yaml
global:
  certManager:
    enabled: true
```

### Manual Certificate Configuration (Fallback)

If cert-manager is not available or you prefer to manage certificates manually, you can provide your own certificates using the CLI parameters when starting the webhooks:

For registry-webhook:
```
--tls-cert-file=/path/to/your/cert.crt
--tls-private-key-file=/path/to/your/key.key
```

For modelinfer-webhook:
```
--tls-cert-file=/path/to/your/cert.crt
--tls-private-key-file=/path/to/your/key.key
```

When using manual certificate configuration, make sure to disable cert-manager integration in your Helm values and provide the CA bundle:

```yaml
global:
  certManager:
    enabled: false

registry:
  webhook:
    caBundle: "base64-encoded-ca-bundle"

workload:
  webhook:
    caBundle: "base64-encoded-ca-bundle"
```

The CA bundle should be base64-encoded. You can generate it with:
```
cat /path/to/your/ca.crt | base64 | tr -d '\n'
```

You will need to ensure that the certificates are properly mounted into the webhook pods and that the paths match the CLI parameters. The CA bundle is required for the Kubernetes API server to trust the webhook server's certificate.
