# Certificate Setup Guide

This guide explains how to generate and configure TLS certificates for the GitOps Reversed Admission Controller.

## Overview

The admission webhook requires TLS certificates to secure communication between the Kubernetes API server and the webhook endpoint. The certificate generation process is fully integrated with Helm deployment.

## Quick Start

```bash
# Generate certificates and create Helm values override
make cert

# Deploy with certificates
make deploy TAG=your-tag
```

## Detailed Setup

### 1. Certificate Generation

The `scripts/generate-certs-helm.sh` script generates:
- CA certificate and private key
- Server certificate and private key
- Kubernetes secret with TLS credentials
- Helm values override file with CA bundle

**Usage:**

```bash
# Use defaults (release: gitops-reverse-engineer, namespace: gitops-reverse-engineer-system)
./scripts/generate-certs-helm.sh

# Custom release name and namespace
./scripts/generate-certs-helm.sh my-release my-namespace
```

### 2. What Gets Created

The script creates:

1. **Kubernetes Secret** (`gitops-reverse-engineer-certs`)
   - Contains `tls.crt` and `tls.key`
   - Created in the target namespace

2. **Helm Values Override** (`values-certs-override.yaml`)
   ```yaml
   tls:
     existingSecret: "gitops-reverse-engineer-certs"
     crt: "<base64-encoded-ca-certificate>"
   ```

3. **CA Certificate** (`ca.crt`)
   - Saved for reference

### 3. Integration with Helm

The generated values are used by the Helm chart:

**Webhook Configuration** (`chart/templates/webhook.yaml`):
```yaml
webhooks:
  - name: validate.gitops-reverse-engineer.com
    clientConfig:
      service:
        name: gitops-reverse-engineer
        namespace: gitops-reverse-engineer-system
        path: "/validate"
      caBundle: {{ .Values.tls.crt }}  # CA bundle from values
```

**Deployment** (`chart/templates/deployment.yaml`):
```yaml
volumes:
  - name: certs
    secret:
      secretName: {{ include "gitops-reverse-engineer.tlsSecretName" . }}
```

### 4. Values Configuration

#### Using Existing Secret (Recommended)

```yaml
tls:
  existingSecret: "gitops-reverse-engineer-certs"
  crt: "<base64-ca-bundle>"
  key: ""  # Not needed when using existingSecret
```

#### Using Inline Certificates

```yaml
tls:
  existingSecret: ""
  crt: "<base64-ca-bundle>"
  key: "<base64-tls-key>"
```

### 5. Deployment Options

#### Option 1: Using Make (Recommended)

```bash
# Generate certificates
make cert

# Deploy (automatically uses values-certs-override.yaml)
make deploy TAG=v1.0.0
```

#### Option 2: Direct Helm Command

```bash
# With values override file
helm template gitops-reverse-engineer ./chart \
  -f values-certs-override.yaml \
  --set image.tag=v1.0.0 \
  --namespace gitops-reverse-engineer-system | kubectl apply -f -

# With inline values
helm template gitops-reverse-engineer ./chart \
  --set tls.existingSecret=gitops-reverse-engineer-certs \
  --set tls.crt=LS0tLS1CRUdJTi... \
  --set image.tag=v1.0.0 \
  --namespace gitops-reverse-engineer-system | kubectl apply -f -
```

#### Option 3: Custom Values File

Create `my-values.yaml`:
```yaml
tls:
  existingSecret: "my-certs"
  crt: "LS0tLS1CRUdJTi..."

git:
  repoUrl: "https://git.example.com/org/repo.git"
  clusterName: "production"
```

Deploy:
```bash
helm template gitops-reverse-engineer ./chart \
  -f my-values.yaml \
  --namespace gitops-reverse-engineer-system | kubectl apply -f -
```

## Certificate Details

### Subject Alternative Names (SANs)

The server certificate includes the following SANs:
- `${SERVICE_NAME}`
- `${SERVICE_NAME}.${NAMESPACE}`
- `${SERVICE_NAME}.${NAMESPACE}.svc`
- `${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local`

### Certificate Validity

- **CA Certificate**: 365 days
- **Server Certificate**: 365 days

### Key Specifications

- **Algorithm**: RSA
- **Key Size**: 2048 bits

## Troubleshooting

### Certificate Validation Errors

If you see errors like:
```
x509: certificate signed by unknown authority
```

**Solution**: Ensure the CA bundle in webhook configuration matches the CA that signed the server certificate.

```bash
# Verify the CA bundle
kubectl get validatingwebhookconfiguration gitops-reverse-engineer-webhook -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | base64 -d

# Compare with your CA certificate
cat ca.crt
```

### Certificate Expired

Regenerate certificates:
```bash
# Delete old secret
kubectl delete secret gitops-reverse-engineer-certs -n gitops-reverse-engineer-system

# Generate new certificates
make cert

# Redeploy
make deploy TAG=current-tag
```

### Wrong Namespace or Service Name

The certificate SANs must match the actual service name and namespace. If you change these:

1. Regenerate certificates with correct values:
   ```bash
   ./scripts/generate-certs-helm.sh new-release-name new-namespace
   ```

2. Update Helm deployment to use the same values:
   ```bash
   helm template new-release-name ./chart \
     -f values-certs-override.yaml \
     --namespace new-namespace | kubectl apply -f -
   ```

## Security Best Practices

1. **Certificate Rotation**: Rotate certificates before expiry (recommend every 6 months)
2. **Secret Management**: Use external secret management (Vault, Sealed Secrets) in production
3. **Access Control**: Restrict access to the certificate secret with RBAC
4. **Backup**: Store CA certificate securely for certificate renewal

## Advanced: Using cert-manager

For production environments, consider using cert-manager:

```yaml
# Create Issuer
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: gitops-admission-issuer
  namespace: gitops-reverse-engineer-system
spec:
  selfSigned: {}

---
# Create Certificate
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gitops-admission-cert
  namespace: gitops-reverse-engineer-system
spec:
  secretName: gitops-reverse-engineer-certs
  duration: 2160h # 90 days
  renewBefore: 360h # 15 days
  subject:
    organizations:
      - gitops-reverse-engineer
  commonName: gitops-reverse-engineer.gitops-reverse-engineer-system.svc
  isCA: false
  privateKey:
    algorithm: RSA
    size: 2048
  dnsNames:
    - gitops-reverse-engineer
    - gitops-reverse-engineer.gitops-reverse-engineer-system
    - gitops-reverse-engineer.gitops-reverse-engineer-system.svc
    - gitops-reverse-engineer.gitops-reverse-engineer-system.svc.cluster.local
  issuerRef:
    name: gitops-admission-issuer
    kind: Issuer
```

Then use the CA bundle from cert-manager:
```bash
CA_BUNDLE=$(kubectl get secret gitops-reverse-engineer-certs -n gitops-reverse-engineer-system -o jsonpath='{.data.ca\.crt}')

helm template gitops-reverse-engineer ./chart \
  --set tls.existingSecret=gitops-reverse-engineer-certs \
  --set tls.crt=${CA_BUNDLE} \
  --namespace gitops-reverse-engineer-system | kubectl apply -f -
```

## Files Reference

- `scripts/generate-certs-helm.sh` - Main certificate generation script (Helm-integrated)
- `scripts/generate-certs.sh` - Legacy certificate generation script
- `values-certs-override.yaml` - Auto-generated Helm values override
- `ca.crt` - CA certificate (for reference)
- `chart/templates/webhook.yaml` - Webhook configuration template
- `chart/templates/secrets.yaml` - TLS secret template
- `chart/values.yaml` - Default values including TLS configuration
