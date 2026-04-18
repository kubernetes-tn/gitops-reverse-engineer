# Quick Reference Guide

## Installation

```bash
# 1. Generate certificates
./scripts/generate-certs.sh

# 2. Create Git secret
kubectl create namespace gitops-reverse-engineer-system
kubectl create secret generic git-token \
  --from-literal=token=YOUR_GIT_TOKEN \
  -n gitops-reverse-engineer-system

# 3. Deploy
make deploy TAG=v1.0.0
```

## Common Commands

```bash
# View logs
make logs

# Check health
kubectl port-forward -n gitops-reverse-engineer-system svc/gitops-reverse-engineer 8443:443
curl -k https://localhost:8443/health

# View metrics
curl -k https://localhost:8443/metrics

# Run tests
./scripts/test.sh

# Update deployment
make deploy TAG=v1.1.0

# Delete
make delete
```

## Configuration Examples

### Basic Setup

```yaml
# values.yaml
git:
  repoUrl: "https://git.example.com/org/repo.git"
  clusterName: "production"

watch:
  resources:
    include: ["Deployment", "Service", "ConfigMap"]
```

### Production Setup

```yaml
# values.yaml
replicaCount: 3

git:
  repoUrl: "https://git.example.com/org/production-state.git"
  clusterName: "production"

watch:
  clusterWideResources: true
  namespaces:
    exclude: ["kube-system", "kube-public"]
  resources:
    include:
      - Deployment
      - StatefulSet
      - DaemonSet
      - Service
      - ConfigMap
      - Secret
      - PersistentVolumeClaim
      - Namespace
      - PersistentVolume

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    labels:
      prometheus: kube-prometheus

prometheusRule:
  enabled: true
  labels:
    prometheus: kube-prometheus

resources:
  limits:
    cpu: 2000m
    memory: 2Gi
  requests:
    cpu: 500m
    memory: 1Gi

affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app.kubernetes.io/name: gitops-reverse-engineer
        topologyKey: kubernetes.io/hostname
```

## Git Repository Structure

### Namespaced Resources
```
{cluster-name}/
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
```

### Cluster-Wide Resources
```
{cluster-name}/
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
```

## Prometheus Metrics

```
gitops_admission_git_sync_success_total      # Total successful syncs
gitops_admission_git_sync_failures_total     # Total failed syncs
gitops_admission_pending_operations          # Current pending operations
```

## Troubleshooting

### Pod not running
```bash
kubectl describe pod -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer
```

### Git sync failures
```bash
# Check metrics
curl -k https://localhost:8443/metrics | grep failures

# Check logs
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer | grep "Failed"
```

### Webhook not intercepting
```bash
# Check webhook
kubectl get validatingwebhookconfiguration

# Verify service
kubectl get svc -n gitops-reverse-engineer-system
```

## Feature Flags

```yaml
# Enable cluster-wide resources
watch:
  clusterWideResources: true

# Enable metrics
metrics:
  enabled: true

# Enable ServiceMonitor
metrics:
  serviceMonitor:
    enabled: true

# Enable PrometheusRule
prometheusRule:
  enabled: true
```

## Testing

```bash
# Run integration tests
./scripts/test.sh

# Manual test - create
kubectl create configmap test --from-literal=key=value

# Manual test - update
kubectl patch configmap test -p '{"data":{"key":"new-value"}}'

# Manual test - delete
kubectl delete configmap test

# Check Git repository
# Should see commits for create, update, and delete operations
```

## Filtering Examples

### Watch only specific namespaces
```yaml
watch:
  namespaces:
    include: ["production", "staging"]
```

### Exclude system namespaces
```yaml
watch:
  namespaces:
    exclude: ["kube-system", "kube-public", "kube-node-lease"]
```

### Watch specific resources
```yaml
watch:
  resources:
    include:
      - Deployment
      - StatefulSet
      - Service
```

### Exclude secrets
```yaml
watch:
  resources:
    exclude:
      - Secret
```

## Verification Checklist

- [ ] Pod is running
- [ ] Health endpoint responds
- [ ] Metrics endpoint works
- [ ] Webhook is configured
- [ ] Test resource creates Git commit
- [ ] Git commit has correct author
- [ ] YAML is clean (no uid, resourceVersion)
- [ ] Update creates new commit
- [ ] Delete removes file from Git
- [ ] ServiceMonitor exists (if enabled)
- [ ] PrometheusRule exists (if enabled)
- [ ] Prometheus shows metrics

## Documentation

- [README.md](../README.md) - Main documentation
- [MILESTONE3-SUMMARY.md](MILESTONE3-SUMMARY.md) - Implementation details
- [DEPLOYMENT-GUIDE.md](DEPLOYMENT-GUIDE.md) - Detailed deployment guide
- [VALUES-REFERENCE.md](../chart/VALUES-REFERENCE.md) - All configuration options
- [ROADMAP.md](ROADMAP.md) - Project roadmap

## Support

For issues:
1. Check logs: `make logs`
2. Check metrics: `curl -k https://localhost:8443/metrics`
3. Run tests: `./scripts/test.sh`
4. Review documentation
