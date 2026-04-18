# Milestone 8 - Multi-Cluster at Scale

**Status**: ✅ Completed

## Overview

Milestone 8 enhances the GitOps Reversed Admission Controller for multi-cluster deployments at scale by:

1. **Cluster-Specific Values Convention**: Supporting `values/values.{cluster}.yaml` files for declarative per-cluster configuration
2. **Host Aliases Support**: Enabling custom DNS resolution for Git servers in air-gapped or custom network environments

## Features Implemented

### 1. Cluster-Specific Values Files

The deployment system now automatically detects and uses cluster-specific values files following the convention:

```
values/values.{CLUSTER_NAME}.yaml
```

**How It Works**:
- When deploying with `make deploy CLUSTER_NAME=production`, the system looks for `values/values.production.yaml`
- If found, it's automatically included in the Helm template command
- Cluster-specific values override the base `chart/values.yaml`
- Multiple values files are layered: base values → certs override → cluster-specific values

**Benefits**:
- Declarative per-cluster configuration
- No need to modify deployment scripts for different clusters
- Git-tracked cluster configurations
- Easy to maintain and audit
- Supports any number of clusters

### 2. Host Aliases Support

The Helm chart now supports `hostAliases` configuration to inject custom host-to-IP mappings into the admission controller pods.

**Why This Matters**:
- Air-gapped environments where DNS is not available
- Custom internal DNS resolution for Git servers
- Bypass external DNS for internal services
- Testing and development scenarios

**Configuration Example**:

```yaml
hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "git.example.com"
      - "git.example.com"
  - ip: "10.x.x.x"
    hostnames:
      - "internal-registry.local"
```

## Implementation Details

### Makefile Changes

**File**: `Makefile`

**Changes**:
1. Enhanced `deploy` target to automatically detect and use `values/values.$(CLUSTER_NAME).yaml`
2. Updated help text to document the multi-cluster convention
3. Added informative messages during deployment about which values files are being used

**Logic Flow**:
```bash
# 1. Check for certs override
if [ -f values-certs-override.yaml ]; then
    VALUES_FILES="-f values-certs-override.yaml"
fi

# 2. Check for cluster-specific values
if [ -f "values/values.$(CLUSTER_NAME).yaml" ]; then
    VALUES_FILES="$VALUES_FILES -f values/values.$(CLUSTER_NAME).yaml"
fi

# 3. Deploy with all values files
helm template ... $VALUES_FILES | kubectl apply -f -
```

### Helm Chart Changes

**File**: `chart/values.yaml`

**Changes**:
- Added `hostAliases` parameter with documentation and examples
- Properly documented as a Bitnami-style parameter with `@param` annotations

**File**: `chart/templates/deployment.yaml`

**Changes**:
- Added conditional `hostAliases` block in pod spec
- Uses Helm's `with` template function to only render when hostAliases is defined

```yaml
{{- with .Values.hostAliases }}
hostAliases:
  {{- toYaml . | nindent 8 }}
{{- end }}
```

### Example Values Files

Created four example cluster-specific values files demonstrating different scenarios:

1. **`values/values.production.yaml`**:
   - Production-grade configuration
   - Higher resource limits (1 CPU, 1Gi memory)
   - 2 replicas for high availability
   - Pod anti-affinity for distribution across nodes
   - Stricter alerting (2m threshold)
   - Critical severity alerts
   - Comprehensive namespace filtering

2. **`values/values.staging.yaml`**:
   - Moderate resource allocation
   - 1 replica
   - Standard monitoring
   - Relaxed alerting (5m threshold)
   - Warning severity alerts

3. **`values/values.dev.yaml`**:
   - Minimal resources (250m CPU, 256Mi memory)
   - 1 replica
   - No cluster-wide resource watching
   - ServiceMonitor disabled (no Prometheus Operator)
   - PrometheusRule disabled
   - Webhook failure policy set to Ignore

4. **`values/values.platform.yaml`**:
   - Current platform cluster configuration
   - Extensive user exclusion list for OpenShift/ACM
   - Standard resource allocation
   - Full monitoring enabled

## Usage Guide

### Basic Multi-Cluster Deployment

**Step 1**: Create cluster-specific values file

```bash
# Create values/values.mycluster.yaml
cat > values/values.mycluster.yaml <<EOF
git:
  repoUrl: "https://gitea.mycluster.example.com/sre/cluster-git-reversed.git"
  existingSecret: "git-token-mycluster"
  clusterName: "mycluster"

replicaCount: 2

resources:
  limits:
    cpu: 1000m
    memory: 1Gi

hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "gitea.mycluster.example.com"
EOF
```

**Step 2**: Generate certificates (one-time)

```bash
make cert
```

**Step 3**: Build and push image

```bash
make build TAG=v1.0.0
make push TAG=v1.0.0
```

**Step 4**: Deploy to cluster

```bash
make deploy CLUSTER_NAME=mycluster TAG=v1.0.0
```

The deployment will automatically:
- Use `chart/values.yaml` as the base
- Apply `values-certs-override.yaml` for TLS configuration
- Apply `values/values.mycluster.yaml` for cluster-specific overrides
- Display which values files are being used

### Deploying to Multiple Clusters

```bash
# Deploy to development
make deploy CLUSTER_NAME=dev TAG=v1.0.0

# Deploy to staging
make deploy CLUSTER_NAME=staging TAG=v1.0.0

# Deploy to production
make deploy CLUSTER_NAME=production TAG=v1.0.0
```

Each cluster automatically gets its own configuration from the corresponding values file.

### Using Host Aliases

For environments where Git servers require custom DNS resolution:

```yaml
# In values/values.{cluster}.yaml
hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "gitea.internal.local"
      - "git.internal.local"
  - ip: "10.x.x.x"
    hostnames:
      - "registry.internal.local"
```

This injects entries into `/etc/hosts` in the admission controller pod:

```
192.168.x.x  gitea.internal.local git.internal.local
10.x.x.x     registry.internal.local
```

## Directory Structure

```
gitops-reverse-engineer/
├── chart/
│   ├── values.yaml              # Base values (all clusters)
│   └── templates/
│       └── deployment.yaml      # Now includes hostAliases support
├── values/
│   ├── values.production.yaml   # Production cluster overrides
│   ├── values.staging.yaml      # Staging cluster overrides
│   ├── values.dev.yaml          # Development cluster overrides
│   └── values.platform.yaml     # Platform cluster overrides
├── Makefile                     # Enhanced with cluster-specific values support
└── values-certs-override.yaml   # Generated by make cert
```

## Configuration Precedence

Values are merged in this order (later overrides earlier):

1. `chart/values.yaml` - Base configuration
2. `values-certs-override.yaml` - TLS certificates (generated by `make cert`)
3. `values/values.{CLUSTER_NAME}.yaml` - Cluster-specific overrides
4. `--set` flags from Makefile - Runtime overrides (image, tag, clusterName)

## Best Practices

### 1. Organize by Environment

```
values/
├── values.prod-us-east.yaml
├── values.prod-eu-west.yaml
├── values.staging-us.yaml
└── values.dev-local.yaml
```

### 2. DRY with YAML Anchors (if needed)

While each file is independent, you can use YAML anchors within a file:

```yaml
# values/values.production.yaml
_resourceDefaults: &resourceDefaults
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 512Mi

resources: *resourceDefaults
```

### 3. Document Cluster-Specific Decisions

Add comments explaining why certain values differ:

```yaml
# Production requires 2 replicas for HA
replicaCount: 2

# Production has stricter SLO - alert after 2m instead of 5m
prometheusRule:
  groups:
    - name: gitops-reverse-engineer-production
      rules:
        - alert: GitOpsAdmissionGitSyncFailure
          for: 2m  # Stricter for production
```

### 4. Version Control All Values Files

Commit all `values/values.*.yaml` files to Git for:
- Audit trail of configuration changes
- Rollback capability
- Collaboration and review
- Documentation of cluster differences

### 5. Use Host Aliases Sparingly

Only use `hostAliases` when necessary:
- ✅ Air-gapped environments without DNS
- ✅ Custom internal networks
- ✅ Testing/development overrides
- ❌ Don't use as workaround for proper DNS configuration in production

## Troubleshooting

### Values File Not Being Used

**Check**:
```bash
# Verify file exists
ls -la values/values.yourcluster.yaml

# Verify CLUSTER_NAME matches filename
make deploy CLUSTER_NAME=yourcluster TAG=v1.0.0
```

**Look for**:
```
📄 Using cluster-specific values from values/values.yourcluster.yaml
```

If you see:
```
ℹ️  No cluster-specific values file found at values/values.yourcluster.yaml
```

The file doesn't exist or the name doesn't match.

### Host Aliases Not Working

**Verify in pod**:
```bash
kubectl exec -n gitops-reverse-engineer-system \
  deployment/gitops-reverse-engineer -- cat /etc/hosts
```

You should see your custom entries.

**Common issues**:
- YAML indentation (hostAliases must be at pod spec level)
- Values not being applied (check which values files are loaded)
- Typo in hostname

### Deployment Using Wrong Values

**Debug**:
```bash
# Dry-run to see generated manifests
helm template gitops-reverse-engineer ./chart \
  --namespace gitops-reverse-engineer-system \
  -f values-certs-override.yaml \
  -f values/values.yourcluster.yaml \
  --set image.tag=v1.0.0 \
  --set git.clusterName=yourcluster
```

Check the generated YAML to verify values are correct.

## Migration from Hardcoded Values

If you have existing deployments with hardcoded values:

**Step 1**: Extract current configuration

```bash
# Get current deployment
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml > current-config.yaml
```

**Step 2**: Convert to values file

Create `values/values.{cluster}.yaml` with the extracted configuration.

**Step 3**: Test with helm template

```bash
helm template gitops-reverse-engineer ./chart \
  -f values/values.{cluster}.yaml \
  --debug
```

**Step 4**: Deploy using new method

```bash
make deploy CLUSTER_NAME={cluster} TAG={version}
```

## Related Documentation

- [README.md](../README.md) - Main documentation
- [Deployment Guide](DEPLOYMENT-GUIDE.md) - Complete deployment guide
- [ROADMAP.md](ROADMAP.md) - Feature roadmap and milestones

## Testing

See [MILESTONE_8_TESTING.md](MILESTONE_8_TESTING.md) for comprehensive testing scenarios.

## Benefits Summary

✅ **Scalability**: Deploy to unlimited clusters with individual configurations
✅ **Maintainability**: Declarative configuration in version control
✅ **Consistency**: Standardized deployment process across all clusters
✅ **Flexibility**: Per-cluster overrides without modifying deployment scripts
✅ **Auditability**: Git history tracks all configuration changes
✅ **Network Flexibility**: Support for air-gapped and custom DNS environments
✅ **Documentation**: Self-documenting through values file comments
