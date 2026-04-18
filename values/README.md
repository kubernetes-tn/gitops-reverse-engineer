# Cluster-Specific Values Files

This directory contains cluster-specific configuration overrides for the GitOps Reversed Admission Controller.

## Convention

Files follow the naming convention: `values.{cluster-name}.yaml`

The Makefile automatically uses the appropriate values file when deploying:
```bash
make deploy CLUSTER_NAME={cluster-name} TAG={version}
```

## Example Clusters

### Development
- **dev** - Development cluster with minimal resources, relaxed policies

### Staging
- **staging** - Staging cluster with moderate resources, label-based namespace filtering

### Production
- **production** - Production cluster with HA (2 replicas), higher resource limits, critical alerting

## Common Configuration

All clusters share these common settings from `chart/values.yaml`:

```yaml
git:
  repoUrl: "https://git.example.com/sre/cluster-git-reversed.git"
  existingSecret: "git-token"

watch:
  clusterWideResources: false
  namespaces:
    excludePattern:
      - "kube-*"

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 256Mi

replicaCount: 1

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
```

## Customization

To customize a cluster's configuration:

1. Copy an existing values file: `cp values.dev.yaml values.mycluster.yaml`
2. Edit `values/values.mycluster.yaml`
3. Common customizations:
   - Change `replicaCount` for HA
   - Adjust `resources` limits/requests
   - Modify `excludeUsers` list
   - Add `hostAliases` for custom DNS
   - Change alert `severity` or `for` duration

## Deployment Examples

```bash
# Deploy to dev cluster
make deploy CLUSTER_NAME=dev TAG=v1.0.0

# Deploy to staging
make deploy CLUSTER_NAME=staging TAG=v1.0.0

# Deploy to production
make deploy CLUSTER_NAME=production TAG=v1.0.0
```

## Host Aliases

For environments where the Git server requires custom DNS resolution (air-gapped, internal networks):

```yaml
hostAliases:
  - ip: "10.x.x.x"
    hostnames:
      - "git.example.com"
  - ip: "10.x.x.x"
    hostnames:
      - "registry.internal.local"
```

This injects custom entries into `/etc/hosts` in the admission controller pods.

## User Exclusions

Clusters can exclude specific system service accounts to prevent tracking changes from:
- Kubernetes system controllers
- GitOps tools (ArgoCD, Flux)
- Operators and automated processes

See any values file for examples of excluded users.

## Documentation

For more information:
- [Main README](../README.md)
- [Helm Chart Values Reference](../chart/VALUES-REFERENCE.md)
