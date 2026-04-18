# Milestone 7 Implementation - Onboard More Resources

## Overview

This milestone expands the default resource types that the GitOps Reversed Admission Controller tracks by default, providing comprehensive coverage of Kubernetes, OpenShift, Gateway API, and ArgoCD resources out-of-the-box.

## Implementation Details

### Changes Made

#### 1. Enhanced Default Resource List

Updated `chart/values.yaml` to include 37 resource types by default (expanded from 8):

**Previous Default (8 resources):**
```yaml
include:
  - Deployment
  - StatefulSet
  - DaemonSet
  - Service
  - ConfigMap
  - Secret
  - PersistentVolumeClaim
  - Ingress
```

**New Default (37 resources):**
```yaml
include:
  # Core Workloads (6)
  - Deployment
  - StatefulSet
  - DaemonSet
  - ReplicaSet
  - Job
  - CronJob
  
  # Services & Networking (3)
  - Service
  - Ingress
  - NetworkPolicy
  
  # Gateway API Resources (6)
  - HTTPRoute
  - Gateway
  - GatewayClass
  - TCPRoute
  - TLSRoute
  - UDPRoute
  
  # OpenShift Routes (1)
  - Route
  
  # Configuration & Storage (4)
  - ConfigMap
  - Secret
  - PersistentVolumeClaim
  - PersistentVolume
  
  # ArgoCD Resources (3)
  - Application
  - ApplicationSet
  - AppProject
  
  # RBAC (5)
  - Role
  - RoleBinding
  - ClusterRole
  - ClusterRoleBinding
  - ServiceAccount
  
  # Resource Quotas & Limits (2)
  - ResourceQuota
  - LimitRange
  
  # Autoscaling (1)
  - HorizontalPodAutoscaler
```

#### 2. Documentation Updates

- Updated `README.md` with comprehensive resource list
- Added categorization for better understanding
- Included examples showing subset of resources in quick start
- Added reference to full list in values.yaml

## Resource Categories

### Core Workloads
Track all application workload types:
- **Deployment**: Standard stateless applications
- **StatefulSet**: Stateful applications with persistent identity
- **DaemonSet**: Node-level system services
- **ReplicaSet**: Lower-level replica management (usually managed by Deployments)
- **Job**: One-time batch processing
- **CronJob**: Scheduled recurring jobs

### Services & Networking
Track service discovery and network policies:
- **Service**: Service endpoints and load balancing
- **Ingress**: HTTP/HTTPS routing (traditional)
- **NetworkPolicy**: Network security rules

### Gateway API Resources
Modern, role-oriented networking APIs:
- **HTTPRoute**: HTTP routing configuration
- **Gateway**: Gateway instances
- **GatewayClass**: Gateway types/controllers
- **TCPRoute**: TCP routing configuration
- **TLSRoute**: TLS routing configuration
- **UDPRoute**: UDP routing configuration

### OpenShift Routes
OpenShift-specific routing:
- **Route**: OpenShift HTTP/HTTPS routing (similar to Ingress)

### Configuration & Storage
Track configuration and persistent data:
- **ConfigMap**: Application configuration
- **Secret**: Sensitive data (obfuscated in Git)
- **PersistentVolumeClaim**: Storage requests
- **PersistentVolume**: Cluster-wide storage resources

### ArgoCD Resources
GitOps workflow tracking:
- **Application**: ArgoCD application definitions
- **ApplicationSet**: ArgoCD application generators
- **AppProject**: ArgoCD project configurations

### RBAC
Security and access control:
- **Role**: Namespace-scoped permissions
- **RoleBinding**: Namespace-scoped permission assignments
- **ClusterRole**: Cluster-wide permissions
- **ClusterRoleBinding**: Cluster-wide permission assignments
- **ServiceAccount**: Identity for pods

### Resource Quotas & Limits
Resource management and constraints:
- **ResourceQuota**: Namespace resource limits
- **LimitRange**: Default and limit values for resources

### Autoscaling
Automatic scaling policies:
- **HorizontalPodAutoscaler**: CPU/memory-based pod scaling

## Usage Examples

### Use Default Resources
Simply deploy without overriding the resource list:

```bash
helm template gitops-reverse-engineer ./chart \
  --namespace gitops-reverse-engineer-system \
  | kubectl apply -f -
```

All 37 default resources will be tracked.

### Watch Only Specific Resources
Override with a subset:

```yaml
# values-override.yaml
watch:
  resources:
    include:
      - Deployment
      - Service
      - ConfigMap
```

### Add Custom Resources to Defaults
You cannot merge with defaults in Helm, but you can copy the default list and add to it:

```yaml
# values-override.yaml
watch:
  resources:
    include:
      # Copy all defaults from chart/values.yaml
      - Deployment
      - StatefulSet
      # ... (all other defaults)
      
      # Add your custom resources
      - CustomResourceDefinition
      - MyCRD
```

### Exclude Specific Resources from Defaults
Use the exclude list:

```yaml
# values-override.yaml
watch:
  resources:
    exclude:
      - Secret  # Don't track secrets
      - Job     # Don't track jobs
```

**Note**: Exclude takes precedence over include.

### Track Only Gateway API Resources
```yaml
# values-override.yaml
watch:
  resources:
    include:
      - HTTPRoute
      - Gateway
      - GatewayClass
      - TCPRoute
      - TLSRoute
      - UDPRoute
```

### Track Only ArgoCD Resources
```yaml
# values-override.yaml
watch:
  resources:
    include:
      - Application
      - ApplicationSet
      - AppProject
```

## Important Notes

### CRD Requirements

Some resources require Custom Resource Definitions (CRDs) to be installed:

1. **Gateway API Resources** (HTTPRoute, Gateway, etc.)
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml
   ```

2. **OpenShift Routes**
   - Only available in OpenShift clusters
   - Remove from list if deploying to vanilla Kubernetes

3. **ArgoCD Resources**
   ```bash
   kubectl create namespace argocd
   kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
   ```

### Resource Not Available Handling

The admission controller will gracefully handle resources that don't exist in your cluster. If a CRD is not installed, the controller will simply not receive events for that resource type.

### Performance Considerations

Tracking 37 resource types has minimal performance impact because:
- The controller only receives events for resources that actually exist
- Events are filtered at the API server level
- The controller uses an asynchronous retry queue
- Git operations are batched and optimized

### Cluster-Scoped Resources

Some resources are cluster-scoped and require `watch.clusterWideResources: true`:

- PersistentVolume
- ClusterRole
- ClusterRoleBinding
- GatewayClass

Enable cluster-wide watching:
```yaml
watch:
  clusterWideResources: true
```

## Testing

### 1. Test Core Workloads

```bash
# Test Deployment
kubectl create deployment nginx --image=nginx
# Check: production/default/deployment/nginx.yaml in Git

# Test Job
kubectl create job test-job --image=busybox -- echo "Hello"
# Check: production/default/job/test-job.yaml in Git

# Test CronJob
kubectl create cronjob test-cron --schedule="*/5 * * * *" --image=busybox -- echo "Hello"
# Check: production/default/cronjob/test-cron.yaml in Git
```

### 2. Test Networking

```bash
# Test NetworkPolicy
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: nginx
  policyTypes:
  - Ingress
EOF
# Check: production/default/networkpolicy/test-network-policy.yaml in Git
```

### 3. Test Gateway API (if CRDs installed)

```bash
# Test HTTPRoute
cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: test-http-route
  namespace: default
spec:
  parentRefs:
  - name: test-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /api
    backendRefs:
    - name: api-service
      port: 8080
EOF
# Check: production/default/httproute/test-http-route.yaml in Git
```

### 4. Test ArgoCD (if CRDs installed)

```bash
# Test Application
cat <<EOF | kubectl apply -f -
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/example/repo
    targetRevision: HEAD
    path: manifests
  destination:
    server: https://kubernetes.default.svc
    namespace: default
EOF
# Check: production/argocd/application/test-app.yaml in Git
```

### 5. Test RBAC

```bash
# Test Role
kubectl create role test-role --verb=get --resource=pods -n default
# Check: production/default/role/test-role.yaml in Git

# Test ServiceAccount
kubectl create serviceaccount test-sa -n default
# Check: production/default/serviceaccount/test-sa.yaml in Git
```

### 6. Test Resource Quotas

```bash
# Test ResourceQuota
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ResourceQuota
metadata:
  name: test-quota
  namespace: default
spec:
  hard:
    requests.cpu: "10"
    requests.memory: 10Gi
    pods: "10"
EOF
# Check: production/default/resourcequota/test-quota.yaml in Git
```

### 7. Test HPA

```bash
# Test HorizontalPodAutoscaler
kubectl autoscale deployment nginx --cpu-percent=80 --min=1 --max=10
# Check: production/default/horizontalpodautoscaler/nginx.yaml in Git
```

## Verification

Check metrics to see tracked resources:

```bash
kubectl port-forward -n gitops-reverse-engineer-system \
  svc/gitops-reverse-engineer 8443:443

curl -k https://localhost:8443/metrics | grep gitops_admission_git_sync_success_total
```

Check Git repository structure:
```bash
git clone <your-repo-url>
cd cluster-state
tree production/
```

Expected structure:
```
production/
├── default/
│   ├── deployment/
│   │   └── nginx.yaml
│   ├── job/
│   │   └── test-job.yaml
│   ├── cronjob/
│   │   └── test-cron.yaml
│   ├── networkpolicy/
│   │   └── test-network-policy.yaml
│   ├── role/
│   │   └── test-role.yaml
│   ├── serviceaccount/
│   │   └── test-sa.yaml
│   └── resourcequota/
│       └── test-quota.yaml
├── argocd/
│   └── application/
│       └── test-app.yaml
└── _cluster/
    └── persistentvolume/
        └── pv-001.yaml
```

## Troubleshooting

### Resource Not Being Tracked

1. **Check if resource is in include list:**
   ```bash
   kubectl get configmap gitops-reverse-engineer-config \
     -n gitops-reverse-engineer-system -o yaml
   ```

2. **Check if CRD exists:**
   ```bash
   kubectl get crd | grep <resource>
   ```

3. **Check if namespace is excluded:**
   ```bash
   # Review namespace filters in config
   kubectl get configmap gitops-reverse-engineer-config \
     -n gitops-reverse-engineer-system -o yaml | grep -A 10 "namespaces:"
   ```

4. **Check controller logs:**
   ```bash
   kubectl logs -n gitops-reverse-engineer-system \
     -l app.kubernetes.io/name=gitops-reverse-engineer --tail=100
   ```

### OpenShift Routes Not Available

If deploying to vanilla Kubernetes, exclude Route:

```yaml
watch:
  resources:
    exclude:
      - Route
```

### Gateway API Resources Not Available

Install Gateway API CRDs or exclude:

```yaml
watch:
  resources:
    exclude:
      - HTTPRoute
      - Gateway
      - GatewayClass
      - TCPRoute
      - TLSRoute
      - UDPRoute
```

## Migration from Previous Versions

If upgrading from a deployment with custom resource lists:

### Option 1: Keep Your Custom List
No action needed. Your custom list in values overrides will continue to work.

### Option 2: Adopt New Defaults
Remove custom resource list from your values overrides to use new defaults.

### Option 3: Merge with Defaults
Copy the new default list and add your custom resources.

## Benefits

1. **Comprehensive Coverage**: Track all common Kubernetes resource types out-of-the-box
2. **Modern Networking**: Support for Gateway API (next-gen Ingress)
3. **OpenShift Support**: Track OpenShift Routes for hybrid environments
4. **GitOps Workflow**: Track ArgoCD resources for complete GitOps visibility
5. **Security Tracking**: RBAC resources for security audit trail
6. **Resource Management**: Track quotas and limits for capacity planning
7. **Flexibility**: Users can still customize via include/exclude
8. **Future-Proof**: Covers modern Kubernetes patterns and APIs

## Next Steps

Consider tracking custom resources specific to your environment:

```yaml
watch:
  resources:
    include:
      # Keep all defaults...
      # Add custom resources
      - VirtualMachine  # KubeVirt
      - Certificate     # cert-manager
      - Issuer          # cert-manager
      - ServiceMonitor  # Prometheus Operator
      - PrometheusRule  # Prometheus Operator
      - Kafka           # Strimzi
      - KafkaTopic      # Strimzi
```

## Related Documentation

- [README.md](../README.md) - Main documentation
- [ROADMAP.md](ROADMAP.md) - All milestones
- [values.yaml](../chart/values.yaml) - Complete configuration reference
