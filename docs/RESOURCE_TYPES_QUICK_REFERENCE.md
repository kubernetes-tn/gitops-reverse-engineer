# Resource Types Quick Reference

## GitOps Reversed Admission Controller - Milestone 7

### Default Tracked Resources (37 Types)

---

## 📦 Core Workloads (6)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **Deployment** | Namespaced | Stateless application deployments | Web applications, APIs |
| **StatefulSet** | Namespaced | Stateful applications with persistent identity | Databases, caches |
| **DaemonSet** | Namespaced | Node-level system services | Log collectors, monitoring agents |
| **ReplicaSet** | Namespaced | Replica management (usually via Deployment) | Auto-scaling |
| **Job** | Namespaced | One-time batch processing | Data migrations, backups |
| **CronJob** | Namespaced | Scheduled recurring jobs | Scheduled reports, cleanups |

---

## 🌐 Services & Networking (3)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **Service** | Namespaced | Service discovery and load balancing | Expose applications |
| **Ingress** | Namespaced | HTTP/HTTPS routing (traditional) | External access with TLS |
| **NetworkPolicy** | Namespaced | Network security rules | Restrict pod-to-pod traffic |

---

## 🚪 Gateway API Resources (6)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **Gateway** | Namespaced | Gateway instances | Application gateway |
| **GatewayClass** | Cluster | Gateway types/controllers | Gateway provider config |
| **HTTPRoute** | Namespaced | HTTP routing configuration | Path-based routing |
| **TCPRoute** | Namespaced | TCP routing configuration | Database access |
| **TLSRoute** | Namespaced | TLS routing configuration | SNI-based routing |
| **UDPRoute** | Namespaced | UDP routing configuration | DNS, video streaming |

**Requirements**: Install Gateway API CRDs
```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml
```

---

## 🔴 OpenShift Routes (1)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **Route** | Namespaced | OpenShift HTTP/HTTPS routing | OpenShift ingress alternative |

**Requirements**: OpenShift cluster only

---

## ⚙️ Configuration & Storage (4)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **ConfigMap** | Namespaced | Application configuration | App settings, configs |
| **Secret** | Namespaced | Sensitive data (obfuscated in Git) | Passwords, tokens |
| **PersistentVolumeClaim** | Namespaced | Storage requests | Database volumes |
| **PersistentVolume** | Cluster | Cluster-wide storage resources | NFS, cloud disks |

**Note**: Secrets are obfuscated (`********`) before committing to Git

---

## 🔄 ArgoCD Resources (3)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **Application** | Namespaced | ArgoCD application definitions | GitOps deployments |
| **ApplicationSet** | Namespaced | ArgoCD application generators | Multi-cluster apps |
| **AppProject** | Namespaced | ArgoCD project configurations | Team isolation |

**Requirements**: Install ArgoCD
```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

---

## 🔐 RBAC (5)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **Role** | Namespaced | Namespace-scoped permissions | Developer access |
| **RoleBinding** | Namespaced | Namespace-scoped permission assignments | Bind role to user |
| **ClusterRole** | Cluster | Cluster-wide permissions | Admin access |
| **ClusterRoleBinding** | Cluster | Cluster-wide permission assignments | Cluster admin |
| **ServiceAccount** | Namespaced | Identity for pods | Pod authentication |

---

## 📊 Resource Quotas & Limits (2)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **ResourceQuota** | Namespaced | Namespace resource limits | Prevent resource exhaustion |
| **LimitRange** | Namespaced | Default and limit values for resources | Set default CPU/memory |

---

## 📈 Autoscaling (1)

| Resource | Scope | Description | Example Use Case |
|----------|-------|-------------|------------------|
| **HorizontalPodAutoscaler** | Namespaced | CPU/memory-based pod scaling | Auto-scale based on load |

---

## Configuration

### Enable All Defaults
```yaml
# Deploy without overriding - all 37 resources tracked
helm template gitops-reverse-engineer ./chart \
  --namespace gitops-reverse-engineer-system \
  | kubectl apply -f -
```

### Watch Specific Categories

**Only Workloads**:
```yaml
watch:
  resources:
    include:
      - Deployment
      - StatefulSet
      - DaemonSet
      - ReplicaSet
      - Job
      - CronJob
```

**Only Networking**:
```yaml
watch:
  resources:
    include:
      - Service
      - Ingress
      - NetworkPolicy
      - HTTPRoute
      - Gateway
```

**Only ArgoCD**:
```yaml
watch:
  resources:
    include:
      - Application
      - ApplicationSet
      - AppProject
```

**Only RBAC**:
```yaml
watch:
  resources:
    include:
      - Role
      - RoleBinding
      - ClusterRole
      - ClusterRoleBinding
      - ServiceAccount
```

### Exclude Specific Resources
```yaml
watch:
  resources:
    exclude:
      - Secret        # Don't track secrets
      - Job           # Don't track jobs
      - CronJob       # Don't track cron jobs
```

### Cluster-Scoped Resources

Enable tracking for cluster-scoped resources:
```yaml
watch:
  clusterWideResources: true
```

Cluster-scoped resources include:
- PersistentVolume
- ClusterRole
- ClusterRoleBinding
- GatewayClass

---

## Git Repository Structure

### Namespaced Resources
```
{cluster}/
  {namespace}/
    {resource-kind}/
      {resource-name}.yaml
```

**Example**:
```
production/
  default/
    deployment/nginx.yaml
    service/nginx.yaml
    configmap/app-config.yaml
  argocd/
    application/my-app.yaml
```

### Cluster-Scoped Resources
```
{cluster}/
  _cluster/
    {resource-kind}/
      {resource-name}.yaml
```

**Example**:
```
production/
  _cluster/
    persistentvolume/pv-001.yaml
    clusterrole/admin.yaml
    gatewayclass/istio.yaml
```

---

## Resource Count by Category

| Category | Count | % of Total |
|----------|-------|------------|
| Core Workloads | 6 | 16.2% |
| Services & Networking | 3 | 8.1% |
| Gateway API | 6 | 16.2% |
| OpenShift | 1 | 2.7% |
| Configuration & Storage | 4 | 10.8% |
| ArgoCD | 3 | 8.1% |
| RBAC | 5 | 13.5% |
| Quotas & Limits | 2 | 5.4% |
| Autoscaling | 1 | 2.7% |
| **Total** | **37** | **100%** |

---

## Platform Compatibility

| Platform | Compatible Resources | Notes |
|----------|---------------------|-------|
| **Vanilla Kubernetes** | 36/37 | Exclude `Route` |
| **OpenShift** | 37/37 | All resources supported |
| **With Gateway API CRDs** | 37/37 | Install CRDs separately |
| **With ArgoCD** | 37/37 | Install ArgoCD separately |

---

## Quick Troubleshooting

### Resource Not Appearing in Git?

1. **Check if included**:
   ```bash
   kubectl get configmap gitops-reverse-engineer-config \
     -n gitops-reverse-engineer-system -o yaml | grep -A 50 "resources:"
   ```

2. **Check if CRD exists**:
   ```bash
   kubectl get crd | grep <resource-type>
   ```

3. **Check logs**:
   ```bash
   kubectl logs -n gitops-reverse-engineer-system \
     -l app.kubernetes.io/name=gitops-reverse-engineer --tail=100
   ```

### Missing CRDs?

**Gateway API**:
```bash
kubectl get crd | grep gateway
# If missing, install:
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml
```

**ArgoCD**:
```bash
kubectl get crd | grep argoproj
# If missing, install ArgoCD
```

### OpenShift Route Not Available?

If on vanilla Kubernetes, exclude it:
```yaml
watch:
  resources:
    exclude:
      - Route
```

---

## Documentation

- **Main**: [README.md](../README.md)
- **Implementation**: [MILESTONE_7_IMPLEMENTATION.md](MILESTONE_7_IMPLEMENTATION.md)
- **Testing**: [MILESTONE_7_TESTING.md](MILESTONE_7_TESTING.md)
- **Summary**: [MILESTONE_7_SUMMARY.md](MILESTONE_7_SUMMARY.md)
- **Roadmap**: [ROADMAP.md](ROADMAP.md)

---

**Version**: Milestone 7
**Status**: ✅ Completed
**Date**: November 15, 2025
**Resource Types**: 37 (expanded from 8)
