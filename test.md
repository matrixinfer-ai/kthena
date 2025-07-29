# Testing the Manual Certificate Setup Solution

To test the manual certificate setup solution for the webhooks, follow these steps:

## 1. Generate the Required Certificates

First, you need to generate a self-signed CA and server certificates:

```bash
# Create a directory for certificates
mkdir -p webhook-certs
cd webhook-certs

# Generate a CA key and certificate
openssl genrsa -out ca.key 2048
openssl req -new -x509 -key ca.key -out ca.crt -subj "/CN=Webhook CA" -days 365

# Generate a server key
openssl genrsa -out tls.key 2048

# Create a certificate signing request (CSR)
# Replace NAMESPACE with your actual namespace
openssl req -new -key tls.key -out server.csr -subj "/CN=matrixinfer-registry-webhook.NAMESPACE.svc" -config <(
cat <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = matrixinfer-registry-webhook.NAMESPACE.svc
DNS.2 = matrixinfer-registry-webhook.NAMESPACE.svc.cluster.local
DNS.3 = matrixinfer-workload-webhook.NAMESPACE.svc
DNS.4 = matrixinfer-workload-webhook.NAMESPACE.svc.cluster.local
EOF
)

# Sign the CSR with the CA
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 365 -extensions v3_req -extfile <(
cat <<EOF
[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = matrixinfer-registry-webhook.NAMESPACE.svc
DNS.2 = matrixinfer-registry-webhook.NAMESPACE.svc.cluster.local
DNS.3 = matrixinfer-workload-webhook.NAMESPACE.svc
DNS.4 = matrixinfer-workload-webhook.NAMESPACE.svc.cluster.local
EOF
)

# Base64 encode the CA bundle for Helm values
CA_BUNDLE=$(cat ca.crt | base64 | tr -d '\n')
echo "CA Bundle for Helm values: $CA_BUNDLE"
```

## 2. Create Kubernetes Secrets for the Certificates

```bash
# Create secrets for both webhooks
kubectl create secret tls matrixinfer-registry-webhook-certs \
  --cert=tls.crt \
  --key=tls.key \
  -n NAMESPACE

kubectl create secret tls matrixinfer-workload-webhook-certs \
  --cert=tls.crt \
  --key=tls.key \
  -n NAMESPACE
```

## 3. Configure Helm Values

Create a `values.yaml` file with the following content:

```yaml
global:
  certManager:
    enabled: false  # Disable cert-manager integration

registry:
  webhook:
    enabled: true
    caBundle: "YOUR_CA_BUNDLE_FROM_STEP_1"  # The base64-encoded CA bundle

workload:
  webhook:
    enabled: true
    caBundle: "YOUR_CA_BUNDLE_FROM_STEP_1"  # The base64-encoded CA bundle
```

## 4. Install or Upgrade the Helm Chart

```bash
helm upgrade --install matrixinfer ./charts/matrixinfer -f values.yaml -n NAMESPACE
```

## 5. Verify the Webhook Configurations

Check that the webhook configurations have the correct CA bundle:

```bash
kubectl get validatingwebhookconfiguration matrixinfer-registry-validating-webhook -o yaml
kubectl get mutatingwebhookconfiguration matrixinfer-registry-mutating-webhook -o yaml
kubectl get validatingwebhookconfiguration matrixinfer-workload-validating-webhook -o yaml
```

## 6. Test the Webhooks

Create a test resource that would trigger the webhooks:

```bash
# Example: Create a Model resource to test the registry webhook
kubectl apply -f test-model.yaml
```

If the webhook is working correctly, it should validate and/or mutate the resource as expected.

## 7. Check Webhook Logs

```bash
kubectl logs -l app.kubernetes.io/component=webhook -n NAMESPACE
```

Look for any errors or successful validation/mutation messages.

## Troubleshooting

If the webhooks are not working:

1. Check that the secrets are correctly mounted:
   ```bash
   kubectl describe pod -l app.kubernetes.io/component=webhook -n NAMESPACE
   ```

2. Verify the webhook server is starting correctly:
   ```bash
   kubectl logs -l app.kubernetes.io/component=webhook -n NAMESPACE
   ```

3. Ensure the CA bundle in the webhook configurations matches the CA that signed the certificates.

4. Check that the DNS names in the certificate match the service names.

By following these steps, you can test the manual certificate setup solution for the webhooks without relying on cert-manager.