<p align="center">
  <img src="docs/logo-ar.svg" alt="GitOps Reverse Engineer Logo" width="400"/>
</p>

<p align="center">
  <a href="https://github.com/kubernetes-tn/gitops-reverse-engineer/stargazers"><img src="https://img.shields.io/github/stars/kubernetes-tn/gitops-reverse-engineer?style=flat-square" alt="Stars"></a>
  <a href="https://github.com/kubernetes-tn/gitops-reverse-engineer/commits/main"><img src="https://img.shields.io/github/last-commit/kubernetes-tn/gitops-reverse-engineer?style=flat-square" alt="Last Commit"></a>
  <a href="https://github.com/kubernetes-tn/gitops-reverse-engineer/issues"><img src="https://img.shields.io/github/issues/kubernetes-tn/gitops-reverse-engineer?style=flat-square" alt="Issues"></a>
  <a href="https://github.com/kubernetes-tn/gitops-reverse-engineer/blob/main/LICENSE"><img src="https://img.shields.io/github/license/kubernetes-tn/gitops-reverse-engineer?style=flat-square" alt="License"></a>
  <a href="https://github.com/kubernetes-tn/gitops-reverse-engineer/actions/workflows/build.yaml"><img src="https://img.shields.io/github/actions/workflow/status/kubernetes-tn/gitops-reverse-engineer/build.yaml?style=flat-square&label=build" alt="Build"></a>
  <a href="https://goreportcard.com/report/github.com/kubernetes-tn/gitops-reverse-engineer"><img src="https://goreportcard.com/badge/github.com/kubernetes-tn/gitops-reverse-engineer" alt="Go Report Card"></a>
</p>

# GitOps Reverse Engineer

A Kubernetes admission controller that automatically syncs resource changes to a Git repository for GitOps auditing and compliance. This controller intercepts CREATE, UPDATE, and DELETE operations and commits them to Git, creating a complete audit trail of cluster state changes.

## 🎯 What It Does

This admission controller:
- ✅ Intercepts CREATE, UPDATE, and DELETE operations for Kubernetes resources
- 📝 Commits clean resource definitions to Git (kubectl-neat functionality)
- � Obfuscates Secret values before committing (keys visible, values hidden)
- �👤 Preserves authorship using the Kubernetes user/serviceaccount
- 🎯 Configurable resource and namespace filtering
- 📊 Prometheus metrics and alerting
- 🔄 Fault-tolerant with automatic retry mechanism
- 🌐 Supports both namespaced and cluster-scoped resources

## 📋 Prerequisites

- Kubernetes cluster (v1.16+)
- Helm 3.x
- kubectl configured to access your cluster
- Docker (for building the image)
- OpenSSL (for certificate generation)
- Go 1.21+ (for local development)
- Git repository (Gitea, GitHub, GitLab, etc.)

## 🚀 Quick Start

### Install from Helm OCI Registry

The chart is published to GitHub Container Registry on every release:

```bash
# 1. Create the namespace and Git token secret
kubectl create namespace gitops-reverse-engineer-system
kubectl create secret generic git-token \
  --from-literal=token=YOUR_GIT_TOKEN \
  -n gitops-reverse-engineer-system

# 2. Install the chart
helm install gitops-reverse-engineer \
  oci://ghcr.io/kubernetes-tn/charts/gitops-reverse-engineer \
  --namespace gitops-reverse-engineer-system \
  --set git.repoUrl=https://git.example.com/org/repo.git \
  --set git.clusterName=my-cluster \
  --set git.provider=gitea          # gitea | github | gitlab

# 3. Verify
kubectl get pods -n gitops-reverse-engineer-system
```

To customize, pull the default values and create an override file:

```bash
helm show values oci://ghcr.io/kubernetes-tn/charts/gitops-reverse-engineer > values-override.yaml
# Edit values-override.yaml, then:
helm install gitops-reverse-engineer \
  oci://ghcr.io/kubernetes-tn/charts/gitops-reverse-engineer \
  --namespace gitops-reverse-engineer-system \
  -f values-override.yaml
```

> **TLS certificates**: The webhook requires TLS. When installing from OCI, generate certs first and pass them via `--set tls.crt=... --set tls.key=...` or use `cert-manager`. See [Certificate Setup](docs/CERTIFICATE-SETUP.md).

### Build from Source (for contributors)

<details>
<summary>Click to expand</summary>

#### 1. Generate TLS Certificates

The admission webhook requires TLS certificates to communicate with the Kubernetes API server:

```bash
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh
```

This script will:
- Generate a CA certificate and key
- Generate server certificates for the webhook
- Create a Kubernetes secret with the certificates

#### 2. Build and Push the Docker Image

```bash
make build
make push
```

For custom registry and tag:
```bash
make build TAG=v1.0.0 REGISTRY=your-registry.com
make push TAG=v1.0.0 REGISTRY=your-registry.com
```

#### 3. Configure Helm Values

#### Option A: Using Cluster-Specific Values Files (Recommended for Multi-Cluster)

Create a cluster-specific values file following the convention `values/values.{cluster-name}.yaml`:

```bash
cat > values/values.production.yaml <<EOF
git:
  repoUrl: "https://git.example.com/sre/cluster-git-reversed.git"
  existingSecret: "git-token"
  clusterName: "production"

watch:
  clusterWideResources: false
  namespaces:
    exclude: ["kube-system", "kube-public"]
  resources:
    include:
      - Deployment
      - StatefulSet
      - Service
      # ... see chart/values.yaml for full default list

metrics:
  enabled: true
  serviceMonitor:
    enabled: true

# For air-gapped or custom DNS environments
hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "git.example.com"
EOF
```

#### Option B: Using Override File

Alternatively, create a `values-override.yaml` file:

```yaml
git:
  repoUrl: "https://git.example.com/sre/cluster-git-reversed.git"
  existingSecret: "git-token"  # Or use token field directly
  clusterName: "production"

watch:
  clusterWideResources: false
  namespaces:
    include: []  # Empty = watch all
    exclude: ["kube-system", "kube-public"]
  resources:
    # By default, includes common workloads, networking, ArgoCD, Gateway API, and OpenShift resources
    # Full list: see chart/values.yaml
    include:
      - Deployment
      - StatefulSet
      - Service
      - ConfigMap
      - Secret
      - NetworkPolicy
      - HTTPRoute
      - Route
      - Application
      - ApplicationSet
    exclude: []

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
  
prometheusRule:
  enabled: true
```

#### 4. Deploy Using Helm

#### Using Cluster-Specific Values (Recommended)

```bash
# The Makefile automatically uses values/values.{CLUSTER_NAME}.yaml if it exists
make deploy CLUSTER_NAME=production TAG=v1.0.0
```

The deployment will automatically use:
1. Base configuration from `chart/values.yaml`
2. TLS certificates from `values-certs-override.yaml` (generated by `make cert`)
3. Cluster-specific overrides from `values/values.production.yaml`

#### Using Override File

```bash
# Using Makefile with custom override file (requires manual modification)
make deploy CLUSTER_NAME=production TAG=v1.0.0

# Or using Helm directly
helm template gitops-reverse-engineer ./chart \
  --namespace gitops-reverse-engineer-system \
  -f values-override.yaml \
  | kubectl apply -f -
```

#### 5. Verify the Installation

```bash
# Check if the pod is running
kubectl get pods -n gitops-reverse-engineer-system

# Check the webhook configuration
kubectl get validatingwebhookconfiguration

# View the logs
make logs
```

</details>

## ⚙️ Configuration

### Multi-Cluster Deployment

The controller supports deploying to multiple clusters with cluster-specific configurations using the convention:

```
values/values.{cluster-name}.yaml
```

**Example structure**:
```
values/
├── values.production.yaml   # Production cluster overrides
├── values.staging.yaml      # Staging cluster overrides
├── values.dev.yaml          # Development cluster overrides
└── values.platform.yaml     # Platform cluster overrides
```

**Usage**:
```bash
# Deploy to production
make deploy CLUSTER_NAME=production TAG=v1.0.0

# Deploy to staging
make deploy CLUSTER_NAME=staging TAG=v1.0.0

# Deploy to dev
make deploy CLUSTER_NAME=dev TAG=v1.0.0
```

The deployment automatically uses the cluster-specific values file if it exists, providing declarative per-cluster configuration without modifying deployment scripts.

**Benefits**:
- ✅ Declarative per-cluster configuration
- ✅ Version-controlled cluster differences
- ✅ No deployment script modifications needed
- ✅ Easy multi-cluster management

See [Milestone 8 Implementation Guide](docs/MILESTONE_8_IMPLEMENTATION.md) for details.

### Git Repository Settings

```yaml
git:
  repoUrl: "https://git.example.com/org/repo.git"
  token: "your-token"  # Or use existingSecret
  existingSecret: "git-token"  # Name of secret containing token
  clusterName: "production"  # Used in Git path structure
```

### Host Aliases (Air-Gapped / Custom DNS)

For environments where Git servers require custom DNS resolution:

```yaml
hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "gitea.internal.local"
      - "git.internal.local"
  - ip: "10.x.x.x"
    hostnames:
      - "registry.internal.local"
```

This injects custom entries into `/etc/hosts` in the admission controller pod, useful for:
- Air-gapped environments without DNS
- Custom internal networks
- Testing with local Git servers

### Resource and Namespace Filtering

```yaml
watch:
  # Enable watching cluster-scoped resources (Namespace, PV, etc.)
  clusterWideResources: true
  
  namespaces:
    # Include specific namespaces (empty = all)
    include: ["production", "staging"]
    # Exclude specific namespaces
    exclude: ["kube-system"]
    # Include namespaces matching patterns (glob-style wildcards)
    includePattern: ["prod-*", "app-*"]
    # Exclude namespaces matching patterns
    excludePattern: ["kube-*", "*-temp"]
    # Label selector for namespaces
    labelSelector:
      environment: "production"
  
  resources:
    # Resource kinds to watch
    # Default includes: workloads, networking, storage, ArgoCD, Gateway API, OpenShift Routes
    # See chart/values.yaml for complete default list
    include:
      # Core Workloads
      - Deployment
      - StatefulSet
      - DaemonSet
      - ReplicaSet
      - Job
      - CronJob
      # Services & Networking
      - Service
      - Ingress
      - NetworkPolicy
      # Gateway API Resources
      - HTTPRoute
      - Gateway
      - GatewayClass
      - TCPRoute
      - TLSRoute
      - UDPRoute
      # OpenShift Routes
      - Route
      # Configuration & Storage
      - ConfigMap
      - Secret
      - PersistentVolumeClaim
      - PersistentVolume
      # ArgoCD Resources
      - Application
      - ApplicationSet
      - AppProject
      # RBAC
      - Role
      - RoleBinding
      - ClusterRole
      - ClusterRoleBinding
      - ServiceAccount
      # Resource Quotas & Limits
      - ResourceQuota
      - LimitRange
      # HorizontalPodAutoscaler
      - HorizontalPodAutoscaler
    # Resource kinds to exclude
    exclude: []
  
  # Exclude specific users or serviceaccounts from being tracked
  excludeUsers:
    - "system:serviceaccount:kube-system:generic-garbage-collector"
    - "system:serviceaccount:argocd:argocd-application-controller"
    - "system:admin"
```

#### Namespace Pattern Matching

The controller supports glob-style pattern matching for namespace filtering:

- **`includePattern`**: Match namespaces by pattern (e.g., `"prod-*"` matches `prod-app`, `prod-db`, etc.)
- **`excludePattern`**: Exclude namespaces by pattern (e.g., `"kube-*"` excludes `kube-system`, `kube-public`, etc.)

Patterns use standard glob syntax:
- `*` matches any sequence of characters
- `?` matches any single character
- `[abc]` matches any character in the brackets

**Example patterns**:
- `"kube-*"` - Excludes `kube-system`, `kube-public`, `kube-node-lease`
- `"*-temp"` - Excludes `test-temp`, `dev-temp`
- `"prod-*"` - Includes `prod-app`, `prod-db`, `prod-cache`
- `"app-?"` - Includes `app-a`, `app-1` (single character after `app-`)

#### User/ServiceAccount Exclusion

You can exclude specific users or serviceaccounts to prevent tracking changes made by:
- System controllers (e.g., garbage collector)
- GitOps tools (e.g., ArgoCD, Flux)
- Automated processes that shouldn't appear in audit trail

**Examples**:
```yaml
watch:
  excludeUsers:
    # Exclude system serviceaccounts
    - "system:serviceaccount:kube-system:generic-garbage-collector"
    - "system:serviceaccount:kube-system:persistent-volume-binder"
    
    # Exclude GitOps controllers
    - "system:serviceaccount:argocd:argocd-application-controller"
    - "system:serviceaccount:flux-system:flux-controller"
    
    # Exclude specific users
    - "system:admin"
    - "ci-bot@example.com"
```

### Metrics and Monitoring

```yaml
metrics:
  enabled: true
  port: 8443
  path: /metrics
  
  # ServiceMonitor for Prometheus Operator
  serviceMonitor:
    enabled: true
    interval: 30s
    scrapeTimeout: 10s

# Prometheus alerting rules
prometheusRule:
  enabled: true
  groups:
    - name: gitops-reverse-engineer
      rules:
        - alert: GitOpsAdmissionGitSyncFailure
          expr: gitops_admission_git_sync_failures_total > 0
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "GitOps Admission Controller failing to sync to Git"
```

### Secret Obfuscation

Secrets are automatically obfuscated before being committed to Git to prevent storing sensitive data in plain text.

**How it works**:
- Secret `data` field values → replaced with `********`
- Secret `stringData` field values → replaced with `********`
- Secret keys, metadata, and type → preserved for audit purposes

**Example**:

Original Secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
data:
  username: YWRtaW4=
  password: cGFzc3dvcmQxMjM=
```

Stored in Git:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
data:
  username: "********"
  password: "********"
```

**Change Detection**:
The controller can detect changes in obfuscated secrets when:
- ✅ Keys are added or removed
- ✅ Secret type changes
- ✅ Labels or annotations change
- ❌ Only values change (limitation - cannot detect without exposing values)

**Security Notes**:
- This provides an audit trail of secret existence and structure
- Secret values are never stored in Git
- For production secret management, consider using:
  - External Secrets Operator
  - Sealed Secrets
  - HashiCorp Vault
  - Cloud provider secret managers

To exclude secrets from tracking entirely:
```yaml
watch:
  resources:
    exclude:
      - Secret
```

## 📊 Metrics

The controller exposes the following Prometheus metrics:

- `gitops_admission_git_sync_success_total` - Total successful Git syncs
- `gitops_admission_git_sync_failures_total` - Total failed Git syncs
- `gitops_admission_pending_operations` - Current pending operations in retry queue
- `gitops_admission_skipped_commits_total` - Total skipped commits (no changes detected)
- `gitops_admission_non_fast_forward_total` - Total non-fast-forward pull errors resolved
- `gitops_admission_obfuscated_secrets_total` - Total secrets obfuscated before commit
- `gitops_admission_secret_changes_detected_total` - Total secret changes detected (even when obfuscated)

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
         │ (intercept)
         ▼
┌──────────────────────────┐
│  ValidatingWebhook       │
│    Configuration         │
└────────┬─────────────────┘
         │
         ▼
┌──────────────────────────┐
│  GitOps Admission        │
│      Controller          │
│                          │
│  • Clean YAML (neat)     │
│  • Extract author        │
│  • Filter resources      │
│  • Commit to Git         │
│  • Retry on failure      │
│  • Export metrics        │
└────────┬─────────────────┘
         │
         ▼
┌──────────────────────────┐
│   Git Repository         │
│   (Gitea/GitHub/etc)     │
│                          │
│  cluster/                │
│    namespace1/           │
│      deployment/         │
│        app.yaml          │
│      service/            │
│    _cluster/             │
│      namespace/          │
└──────────────────────────┘
```

## 📁 Git Repository Structure

Resources are committed to Git using the following structure:

### Namespaced Resources
```
{cluster}/
  {namespace}/
    {resource-kind}/
      {resource-name}.yaml
```

Example:
```
production/
  default/
    deployment/
      nginx.yaml
    service/
      nginx.yaml
  staging/
    configmap/
      app-config.yaml
```

### Cluster-Scoped Resources
```
{cluster}/
  _cluster/
    {resource-kind}/
      {resource-name}.yaml
```

Example:
```
production/
  _cluster/
    namespace/
      production.yaml
    persistentvolume/
      pv-001.yaml
```

## 🧪 Testing

### Test Resource Creation

```bash
kubectl create configmap test-config --from-literal=key=value -n default
```

Check your Git repository - you should see:
```
production/default/configmap/test-config.yaml
```

### Test Resource Update

```bash
kubectl patch configmap test-config -n default -p '{"data":{"key":"new-value"}}'
```

Check Git commit history - the file should be updated with a new commit.

### Test Resource Deletion

```bash
kubectl delete configmap test-config -n default
```

The file should be removed from the Git repository.

### View Metrics

```bash
kubectl port-forward -n gitops-reverse-engineer-system \
  svc/gitops-reverse-engineer 8443:443

curl -k https://localhost:8443/metrics
```

## 🐛 Troubleshooting

### Pod is not starting

```bash
kubectl describe pod -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer

kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer
```

### Git sync failures

Check the metrics:
```bash
curl -k https://localhost:8443/metrics | grep git_sync_failures
```

Check pending operations:
```bash
curl -k https://localhost:8443/health
```

Review logs for error messages:
```bash
make logs
```

### Certificate errors

```bash
# Regenerate certificates
./scripts/generate-certs.sh

# Update the Helm deployment
make deploy
```

## 🧹 Cleanup

```bash
# Using Makefile
make delete

# Or manually
kubectl delete namespace gitops-reverse-engineer-system
kubectl delete validatingwebhookconfiguration gitops-reverse-engineer-webhook
```

## 📚 Features

### Milestone 1 ✅
- Basic admission controller functionality
- Print event details

### Milestone 2 ✅
- Git integration with Gitea
- Create/Update/Delete operations synced to Git
- Fault-tolerant retry mechanism
- Proper path structure in Git

### Milestone 3 ✅
- **kubectl-neat functionality** - Clean YAML commits without runtime metadata
- **Dynamic Git authorship** - Preserves Kubernetes user/serviceaccount
- **Helm chart deployment** - Bitnami-style values structure
- **Cluster-wide resource support** - Optional namespace/PV/etc tracking
- **Resource filtering** - Include/exclude specific resource kinds
- **Namespace filtering** - Include/exclude specific namespaces
- **Prometheus metrics** - Success/failure/pending operations
- **ServiceMonitor & PrometheusRule** - Auto-configured monitoring

### Milestone 4 ✅
- **Optimized Git commits** - Skip unnecessary commits for unchanged resources
- **Non-fast-forward recovery** - Automatic handling of Git conflicts

### Milestone 5 ✅
- **User/ServiceAccount exclusion** - Filter out changes from specific users or service accounts
- **Namespace pattern matching** - Glob-style patterns for include/exclude (e.g., `kube-*`, `prod-*`)

### Milestone 6 ✅
- **Secret data obfuscation** - Secret values replaced with `********` before committing to Git
- **Secret change detection** - Detect changes in obfuscated secrets (keys, type, metadata)
- **Prometheus metrics for secrets** - Track obfuscated secrets and detected changes

### Milestone 7 ✅
- **Enhanced resource coverage** - Default includes 37+ resource types
- **Gateway API resources** - HTTPRoute, Gateway, GatewayClass, TCPRoute, TLSRoute, UDPRoute
- **OpenShift resources** - Route support
- **ArgoCD resources** - Application, ApplicationSet, AppProject
- **RBAC resources** - Role, RoleBinding, ClusterRole, ClusterRoleBinding, ServiceAccount
- **Resource management** - ResourceQuota, LimitRange, HorizontalPodAutoscaler

### Milestone 8 ✅
- **Cluster-specific values files** - Support for `values/values.{cluster}.yaml` convention
- **Declarative multi-cluster deployment** - Per-cluster configuration overrides
- **Host aliases support** - Custom DNS resolution for air-gapped/internal environments
- **Simplified deployment workflow** - Automatic values file detection

### Milestone 9 ✅
- **Multi-provider Git support** - Abstracted `GitClient` interface supporting Gitea, GitHub, and GitLab
- **Provider-aware authentication** - Automatic auth method per provider (`oauth2` for Gitea/GitLab, `x-access-token` for GitHub)
- **New environment variables** - `GIT_TOKEN` (with `GITEA_TOKEN` backward compatibility) and `GIT_PROVIDER`
- **Helm chart provider config** - New `git.provider` value for selecting the Git backend
- **Open-source readiness** - Project renamed to `gitops-reverse-engineer`, GitHub Actions CI/CD workflows

### Milestone 10 ✅
- **End-to-end test framework** - Full e2e tests using Kind + Gitea running in-cluster
- **Automated test environment** - `e2e/setup.sh` bootstraps Kind cluster, Gitea, TLS certs, and webhook deployment
- **Resource lifecycle tests** - CREATE/UPDATE/DELETE for ConfigMaps, Secrets, Deployments, Services
- **Secret obfuscation verification** - Validates secrets are never committed in plaintext to Git
- **Namespace exclusion tests** - Confirms `kube-*` exclusion pattern works
- **Webhook endpoint tests** - Health and Prometheus metrics endpoint checks from inside the cluster
- **CI integration** - Dedicated `e2e.yaml` GitHub Actions workflow with failure log collection
- **Makefile targets** - `make e2e-setup`, `make e2e-test`, `make e2e-teardown`

### Milestone 11 ✅
- **Multi-platform container images** - `linux/amd64` and `linux/arm64` built and pushed on every release
- **Helm OCI registry** - Chart published to `oci://ghcr.io/kubernetes-tn/charts/gitops-reverse-engineer` on every release
- **CI concurrency controls** - Cancel redundant CI runs, never cancel in-flight releases
- **Dockerfile multi-arch support** - `TARGETOS`/`TARGETARCH` build args for correct cross-compilation

### Milestone 12 ✅
- **Issue templates** - Structured bug report and feature request forms with dropdowns and required fields
- **PR template** - What/Why/How sections with verification checklist
- **Security policy** - Private vulnerability reporting via GitHub Security Advisories
- **CODEOWNERS** - Auto-assigned reviewers by path
- **Copilot instructions** - Project conventions for AI-assisted development
- **Stale bot** - Automated issue/PR hygiene (60 days stale, 14 days to close)

## 🔧 Development

### Local Development

```bash
# Download dependencies
go mod download

# Run locally (requires config and certificates)
CONFIG_FILE=./config.yaml \
GIT_REPO_URL=https://git.example.com/repo.git \
GIT_TOKEN=your-token \
GIT_PROVIDER=gitea \
CLUSTER_NAME=dev \
go run .
```

### Building

```bash
# Build Go binary
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o admission-controller .

# Build Docker image
docker build -t gitops-reverse-engineer:latest .
```

## 📄 License

This project is licensed under the [Apache License 2.0](LICENSE). This means you can freely use, modify, and distribute the code, while enjoying explicit patent protection from every contributor. See the [LICENSE](LICENSE) file for details.

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## 📖 Documentation

- [Deployment Guide](docs/DEPLOYMENT-GUIDE.md)
- [Design - Milestone 2](docs/DESIGN-MILESTONE2.md)
- [Roadmap](docs/ROADMAP.md)
- [Changelog](docs/CHANGELOG.md)
- [Contributing](docs/CONTRIBUTING.md)
- [Milestone 8 - Multi-Cluster at Scale](docs/MILESTONE_8_IMPLEMENTATION.md)
- [Milestone 8 Testing Guide](docs/MILESTONE_8_TESTING.md)
