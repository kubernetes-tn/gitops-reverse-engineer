# Helm Chart Values Documentation

This document provides detailed information about all configuration options available in the GitOps Reversed Admission Helm chart.

## Quick Start Example

```yaml
git:
  repoUrl: "https://git.example.com/sre/cluster-git-reversed.git"
  existingSecret: "git-token"
  clusterName: "production"

watch:
  clusterWideResources: false
  resources:
    include:
      - Deployment
      - Service
      - ConfigMap

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
```

## Global Parameters

### `global.imageRegistry`
- **Type**: string
- **Default**: `""`
- **Description**: Global Docker image registry. Overrides image.registry when set.

### `global.imagePullSecrets`
- **Type**: array
- **Default**: `[]`
- **Description**: Global Docker registry secret names as an array.
- **Example**: 
  ```yaml
  global:
    imagePullSecrets:
      - myRegistryKeySecretName
  ```

## Common Parameters

### `nameOverride`
- **Type**: string
- **Default**: `""`
- **Description**: String to partially override the chart name.

### `fullnameOverride`
- **Type**: string
- **Default**: `""`
- **Description**: String to fully override the chart name.

### `namespaceOverride`
- **Type**: string
- **Default**: `""`
- **Description**: String to override the namespace where resources are deployed.

## Image Parameters

### `image.registry`
- **Type**: string
- **Default**: `"ghcr.io/kubernetes-tn"`
- **Description**: Container image registry.

### `image.repository`
- **Type**: string
- **Default**: `"gitops-reverse-engineer"`
- **Description**: Container image repository.

### `image.tag`
- **Type**: string
- **Default**: `"gitops-1"`
- **Description**: Container image tag.

### `image.pullPolicy`
- **Type**: string
- **Default**: `"IfNotPresent"`
- **Description**: Image pull policy. Options: Always, IfNotPresent, Never.

## Git Configuration

### `git.repoUrl`
- **Type**: string
- **Default**: `"https://git.example.com/sre/cluster-git-reversed.git"`
- **Description**: Git repository URL for syncing resources.
- **Required**: Yes

### `git.token`
- **Type**: string
- **Default**: `""`
- **Description**: Git authentication token. Leave empty if using existingSecret.

### `git.existingSecret`
- **Type**: string
- **Default**: `"git-token"`
- **Description**: Name of existing secret containing git token (key: token).

### `git.clusterName`
- **Type**: string
- **Default**: `"platform"`
- **Description**: Name of the cluster used in git path structure.

## Watch Configuration

### `watch.clusterWideResources`
- **Type**: boolean
- **Default**: `false`
- **Description**: Enable watching cluster-scoped resources (Namespace, PersistentVolume, etc.).

### `watch.namespaces.include`
- **Type**: array
- **Default**: `[]`
- **Description**: List of namespace names to watch. Empty array watches all namespaces.
- **Example**:
  ```yaml
  watch:
    namespaces:
      include:
        - production
        - staging
  ```

### `watch.namespaces.exclude`
- **Type**: array
- **Default**: `[]`
- **Description**: List of namespace names to exclude from watching.
- **Example**:
  ```yaml
  watch:
    namespaces:
      exclude:
        - kube-system
        - kube-public
  ```

### `watch.resources.include`
- **Type**: array
- **Default**: `["Deployment", "StatefulSet", "DaemonSet", "Service", "ConfigMap", "Secret", "PersistentVolumeClaim", "Ingress"]`
- **Description**: List of resource kinds to watch. Empty array watches all resource types.

### `watch.resources.exclude`
- **Type**: array
- **Default**: `[]`
- **Description**: List of resource kinds to exclude from watching.

## Service Configuration

### `service.type`
- **Type**: string
- **Default**: `"ClusterIP"`
- **Description**: Kubernetes service type.

### `service.port`
- **Type**: integer
- **Default**: `443`
- **Description**: Service port.

## Webhook Configuration

### `webhook.failurePolicy`
- **Type**: string
- **Default**: `"Ignore"`
- **Description**: Failure policy for the webhook. Options: Fail, Ignore.

### `webhook.timeoutSeconds`
- **Type**: integer
- **Default**: `10`
- **Description**: Timeout for webhook calls in seconds.

## Metrics Configuration

### `metrics.enabled`
- **Type**: boolean
- **Default**: `true`
- **Description**: Enable Prometheus metrics.

### `metrics.port`
- **Type**: integer
- **Default**: `8443`
- **Description**: Metrics port.

### `metrics.path`
- **Type**: string
- **Default**: `"/metrics"`
- **Description**: Metrics endpoint path.

### `metrics.serviceMonitor.enabled`
- **Type**: boolean
- **Default**: `true`
- **Description**: Create ServiceMonitor resource for Prometheus Operator.

### `metrics.serviceMonitor.interval`
- **Type**: string
- **Default**: `"30s"`
- **Description**: Interval at which metrics should be scraped.

### `prometheusRule.enabled`
- **Type**: boolean
- **Default**: `true`
- **Description**: Create PrometheusRule resource with alerting rules.

## Resource Limits

### `resources.limits.cpu`
- **Type**: string
- **Default**: `"500m"`
- **Description**: CPU limit.

### `resources.limits.memory`
- **Type**: string
- **Default**: `"512Mi"`
- **Description**: Memory limit.

### `resources.requests.cpu`
- **Type**: string
- **Default**: `"100m"`
- **Description**: CPU request.

### `resources.requests.memory`
- **Type**: string
- **Default**: `"256Mi"`
- **Description**: Memory request.

## Complete Example

```yaml
# Global settings
global:
  imageRegistry: "myregistry.com"
  imagePullSecrets:
    - my-secret

# Image configuration
image:
  registry: "ghcr.io/kubernetes-tn"
  repository: "gitops-reverse-engineer"
  tag: "v1.0.0"
  pullPolicy: "Always"

# Git repository settings
git:
  repoUrl: "https://git.example.com/gitops/production-cluster.git"
  existingSecret: "git-token"
  clusterName: "production"

# Watch configuration
watch:
  clusterWideResources: true
  namespaces:
    include: []
    exclude:
      - kube-system
      - kube-public
      - kube-node-lease
  resources:
    include:
      - Deployment
      - StatefulSet
      - DaemonSet
      - Service
      - ConfigMap
      - Secret
      - PersistentVolumeClaim
      - Ingress
      - Namespace  # Only if clusterWideResources: true
      - PersistentVolume  # Only if clusterWideResources: true
    exclude: []

# Metrics and monitoring
metrics:
  enabled: true
  port: 8443
  path: /metrics
  serviceMonitor:
    enabled: true
    interval: 30s
    scrapeTimeout: 10s
    labels:
      prometheus: kube-prometheus

# Prometheus alerting
prometheusRule:
  enabled: true
  labels:
    prometheus: kube-prometheus
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
            description: "The GitOps Admission Controller has failed to sync {{ $value }} operations to Git"

# Resource limits
resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 512Mi

# Security context
podSecurityContext:
  enabled: true
  fsGroup: 1001

containerSecurityContext:
  enabled: true
  runAsUser: 1001
  runAsNonRoot: true
  readOnlyRootFilesystem: false

# Node selection
nodeSelector:
  kubernetes.io/os: linux

tolerations:
  - key: "node-role.kubernetes.io/control-plane"
    operator: "Exists"
    effect: "NoSchedule"

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: gitops-reverse-engineer
          topologyKey: kubernetes.io/hostname
```
