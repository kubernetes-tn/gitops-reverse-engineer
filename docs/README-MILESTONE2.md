# GitOps Reversed Admission Controller

A Kubernetes admission controller that automatically syncs all resource operations (CREATE/UPDATE/DELETE) to a Git repository, implementing a "reversed GitOps" pattern where the cluster state is continuously backed up to Git.

## 🎯 What It Does

This admission controller:
- ✅ Intercepts CREATE, UPDATE, and DELETE operations for Kubernetes resources
- 📝 Syncs resource definitions to a Git repository (primarily Gitea)
- 🗂️ Organizes resources in a structured path: `{clustername}/{namespace}/{resourcekind}/{resourcename}.yaml`
- 🔄 Implements fault-tolerant Git operations with automatic retry mechanism
- 📊 Maintains an offline queue when Git server is unreachable
- ✅ Never denies requests - operates in audit/sync mode only

## 🏗️ Architecture

```
┌─────────────────┐
│  kubectl/API    │
│     Request     │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│   Kubernetes API        │
│      Server             │
└────────┬────────────────┘
         │
         │ (intercept)
         ▼
┌──────────────────────────────┐
│  ValidatingWebhook           │
│    Configuration             │
└────────┬─────────────────────┘
         │
         ▼
┌──────────────────────────────────────┐
│  GitOps Reversed Admission           │
│      Controller                      │
│                                      │
│  • Intercepts resource operations   │
│  • Syncs to Git repository          │
│  • Fault-tolerant retry logic       │
│  • Returns allowed=true              │
└────────┬─────────────────────────────┘
         │
         ▼
┌──────────────────────────────────────┐
│  Gitea Repository                    │
│                                      │
│  production-cluster/                 │
│    default/                          │
│      deployment/                     │
│        nginx.yaml                    │
│      service/                        │
│        nginx.yaml                    │
│    kube-system/                      │
│      configmap/                      │
│        config.yaml                   │
└──────────────────────────────────────┘
```

## 📋 Prerequisites

- Kubernetes cluster (v1.16+)
- kubectl configured to access your cluster
- Docker (for building the image)
- OpenSSL (for certificate generation)
- Go 1.21+ (for local development)
- Gitea server (or compatible Git server) with API access
- Git repository for storing cluster state

## 🚀 Quick Start

### 1. Create Git Repository

Create a repository in your Gitea instance to store the cluster state:

```bash
# Example: https://git.example.com/sre/cluster-git-reversed.git
```

### 2. Generate Gitea Token

1. Login to your Git server (GitHub, Gitea, GitLab, etc.)
2. Go to Settings > Applications > Generate New Token
3. Name: "gitops-admission-controller"
4. Scopes: `repo` (all)
5. Copy the generated token

### 3. Create Kubernetes Secret

```bash
# Edit k8s/secret.yaml and replace YOUR_GIT_TOKEN_HERE with your token
kubectl apply -f k8s/secret.yaml
```

### 4. Generate TLS Certificates

```bash
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh
```

### 5. Build the Docker Image

```bash
chmod +x scripts/build.sh
./scripts/build.sh --registry ghcr.io/kubernetes-tn --tag latest
```

### 6. Configure Deployment

Edit `k8s/deployment.yaml` to set your environment variables:

```yaml
env:
- name: GIT_REPO_URL
  value: "https://git.example.com/sre/cluster-git-reversed.git"
- name: CLUSTER_NAME
  value: "production-cluster"  # Change to your cluster name
```

### 7. Deploy to Kubernetes

```bash
# Deploy all resources
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# Wait for pod to be ready
kubectl wait --for=condition=ready pod -l app=gitops-reverse-engineer -n verbose-api-log-system --timeout=60s

# Deploy webhook configuration
kubectl apply -f k8s/webhook-configured.yaml
```

### 8. Verify the Installation

```bash
# Check pod status
kubectl get pods -n verbose-api-log-system

# View logs
kubectl logs -n verbose-api-log-system -l app=gitops-reverse-engineer -f

# Check health
kubectl exec -n verbose-api-log-system deployment/gitops-reverse-engineer -- wget -O- https://localhost:8443/health --no-check-certificate
```

## 🧪 Testing

Enable Git sync for a namespace:

```bash
# Label a namespace to enable GitOps sync
kubectl label namespace default gitops-reverse-engineer/enabled="true"

# Create a test deployment
kubectl create deployment test-nginx --image=nginx -n default

# Check Git repository - you should see:
# {clustername}/default/deployment/test-nginx.yaml
```

Update the deployment:

```bash
# Scale the deployment
kubectl scale deployment test-nginx --replicas=3 -n default

# The updated deployment will be synced to Git
```

Delete the deployment:

```bash
# Delete the deployment
kubectl delete deployment test-nginx -n default

# The file will be removed from Git repository
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `GIT_REPO_URL` | Full URL to Git repository | Yes | - |
| `GIT_TOKEN` | Git API token (from secret) | Yes | - |
| `CLUSTER_NAME` | Name of the cluster (for path structure) | No | `default-cluster` |

### Monitored Resources

Edit `k8s/webhook.yaml` to configure which resources are synced:

```yaml
rules:
  - operations: ["CREATE", "UPDATE", "DELETE"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["configmaps", "secrets", "pods", "services"]
  - operations: ["CREATE", "UPDATE", "DELETE"]
    apiGroups: ["apps"]
    apiVersions: ["v1"]
    resources: ["deployments", "statefulsets", "daemonsets"]
```

### Namespace Filtering

Only namespaces with the label `gitops-reverse-engineer/enabled: "true"` are monitored:

```bash
# Enable GitOps sync for a namespace
kubectl label namespace my-namespace gitops-reverse-engineer/enabled="true"

# Disable GitOps sync
kubectl label namespace my-namespace gitops-reverse-engineer/enabled-
```

## 🛡️ Fault Tolerance

The controller implements several fault-tolerance mechanisms:

1. **Offline Queue**: When Git server is unreachable, operations are queued
2. **Automatic Retry**: Failed operations are retried every 30 seconds
3. **Max Retries**: Operations are retried up to 10 times before being discarded
4. **Non-Blocking**: Git failures never block Kubernetes operations
5. **Health Monitoring**: Health endpoint reports pending operation count

### Monitoring Pending Operations

```bash
# Check health endpoint
kubectl exec -n verbose-api-log-system deployment/gitops-reverse-engineer -- \
  wget -O- https://localhost:8443/health --no-check-certificate

# Output: OK - Pending operations: 0
```

## 📁 Repository Structure

The Git repository will have this structure:

```
cluster-state/
├── production-cluster/
│   ├── default/
│   │   ├── deployment/
│   │   │   ├── nginx.yaml
│   │   │   └── app.yaml
│   │   ├── service/
│   │   │   ├── nginx.yaml
│   │   │   └── app.yaml
│   │   └── configmap/
│   │       └── app-config.yaml
│   ├── kube-system/
│   │   └── configmap/
│   │       └── cluster-info.yaml
│   └── app-namespace/
│       ├── deployment/
│       ├── service/
│       └── secret/
└── staging-cluster/
    └── ...
```

## 🔍 How It Works

1. **Interception**: Kubernetes API server sends admission review to webhook
2. **Processing**: Controller extracts resource details and operation type
3. **Git Sync**: 
   - **CREATE/UPDATE**: Serialize resource to YAML and commit to Git
   - **DELETE**: Remove file from Git repository
4. **Commit**: Changes are committed with descriptive message
5. **Push**: Changes are pushed to remote repository
6. **Retry**: On failure, operation is queued for retry
7. **Response**: Controller always returns "allowed" to Kubernetes

## 🛠️ Development

### Local Testing

```bash
# Install dependencies
go mod download

# Run tests (if available)
go test ./...

# Build locally
go build -o admission-controller .
```

### Building Without Docker

```bash
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o admission-controller .
```

## 🐛 Troubleshooting

### Controller not syncing to Git

```bash
# Check logs
kubectl logs -n verbose-api-log-system -l app=gitops-reverse-engineer

# Common issues:
# - Invalid GIT_TOKEN
# - Incorrect GIT_REPO_URL
# - Network connectivity to Gitea server
# - Insufficient permissions on Git repository
```

### High pending operations count

```bash
# Check health
kubectl exec -n verbose-api-log-system deployment/gitops-reverse-engineer -- \
  wget -O- https://localhost:8443/health --no-check-certificate

# If count is high:
# 1. Check Git server availability
# 2. Verify network connectivity
# 3. Check controller logs for errors
```

### Webhook not intercepting requests

```bash
# Verify webhook configuration
kubectl get validatingwebhookconfiguration gitops-reverse-engineer-webhook -o yaml

# Check namespace labels
kubectl get namespace default --show-labels
```

## 🧹 Cleanup

```bash
# Delete webhook first (important!)
kubectl delete validatingwebhookconfiguration gitops-reverse-engineer-webhook

# Delete all resources
kubectl delete -f k8s/deployment.yaml
kubectl delete -f k8s/service.yaml
kubectl delete -f k8s/secret.yaml

# Or delete entire namespace
kubectl delete namespace verbose-api-log-system
```

## 📚 Learn More

- [Kubernetes Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
- [Dynamic Admission Control](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [GitOps Principles](https://www.gitops.tech/)
- [Gitea API Documentation](https://docs.gitea.io/en-us/api-usage/)

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## ⚠️ Important Notes

- This controller **never denies** requests - it only syncs to Git
- `failurePolicy: Ignore` ensures cluster continues working if webhook fails
- Git operations are asynchronous and non-blocking
- Sensitive data in Secrets will be synced to Git - ensure repository is secure
- Consider using sealed-secrets or external-secrets for sensitive data

## 🔒 Security Considerations

1. **Git Repository Access**: Ensure repository has appropriate access controls
2. **Secret Management**: Secrets are synced as-is to Git - use encryption
3. **Token Security**: Store GIT_TOKEN in Kubernetes Secret, never in code
4. **TLS Certificates**: Rotate certificates regularly
5. **Namespace Isolation**: Use namespace labels to control what gets synced

## 📊 Monitoring

Key metrics to monitor:

- Pending operations count (via `/health` endpoint)
- Controller pod health and restarts
- Git push success/failure rate
- Webhook latency and timeout rate

## 🎓 Use Cases

- **Cluster State Backup**: Continuous backup of cluster resources
- **Audit Trail**: Complete history of all resource changes
- **Disaster Recovery**: Restore cluster state from Git
- **Compliance**: Track who changed what and when
- **GitOps Reconciliation**: Compare desired (Git) vs actual (cluster) state
- **Multi-Cluster Management**: Centralized view of all cluster states
