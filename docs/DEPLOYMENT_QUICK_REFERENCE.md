# Quick Deployment Reference

## Multi-Cluster Deployment Commands

### Example Clusters

```bash
# Development cluster
make deploy CLUSTER_NAME=dev TAG=v1.0.0

# Staging cluster
make deploy CLUSTER_NAME=staging TAG=v1.0.0

# Production cluster
make deploy CLUSTER_NAME=production TAG=v1.0.0
```

## What Happens During Deployment

When you run `make deploy CLUSTER_NAME=production TAG=v1.0.0`:

1. Checks for `values-certs-override.yaml` (TLS certs)
2. Checks for `values/values.production.yaml` (cluster-specific config)
3. Runs helm template with all values files
4. Applies generated manifests to Kubernetes
5. Waits for rollout to complete
6. Shows logs

## Configuration Hierarchy

Values are merged in this order (later overrides earlier):

1. `chart/values.yaml` - Base defaults
2. `values-certs-override.yaml` - TLS certificates
3. `values/values.{CLUSTER_NAME}.yaml` - Cluster-specific
4. `--set` flags - Runtime overrides (image, tag, clusterName)

## Verify Deployment

```bash
# Check pod status
kubectl get pods -n gitops-reverse-engineer-system

# Check which values were applied
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml

# Check hostAliases (if configured)
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml | grep -A 10 hostAliases

# View logs
make logs

# Check metrics
kubectl port-forward -n gitops-reverse-engineer-system \
  svc/gitops-reverse-engineer 8443:443
curl -k https://localhost:8443/metrics
```

## Troubleshooting

### Check which values file is being used

```bash
# The deployment will show:
# "Using cluster-specific values from values/values.XXX.yaml"
# or
# "No cluster-specific values file found at values/values.XXX.yaml"
```

### Verify hostAliases in running pod

```bash
# For clusters with hostAliases, verify custom DNS
kubectl exec -n gitops-reverse-engineer-system \
  deployment/gitops-reverse-engineer -- cat /etc/hosts
```

### Test Git connectivity

```bash
# Check logs for Git sync operations
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer --tail=50 | grep -i git
```

## Complete Deployment Workflow

```bash
# 1. Generate certificates (one-time per cluster context)
make cert

# 2. Build and push image (if needed)
make build TAG=v1.0.0
make push TAG=v1.0.0

# 3. Deploy to specific cluster
make deploy CLUSTER_NAME=production TAG=v1.0.0

# 4. Verify deployment
kubectl get pods -n gitops-reverse-engineer-system

# 5. Test with a resource
kubectl create configmap test-config -n default --from-literal=test=value

# 6. Check Git repository for the committed resource
# Should see: production/default/configmap/test-config.yaml

# 7. Cleanup test resource
kubectl delete configmap test-config -n default
```

## Update Existing Deployment

```bash
# Re-run deploy with the new tag
make deploy CLUSTER_NAME=production TAG=v2.0.0

# Or update values file and redeploy
vim values/values.production.yaml
make deploy CLUSTER_NAME=production TAG=v1.0.0
```

## See Also

- [values/README.md](values/README.md) - Cluster values documentation
- [README.md](README.md) - Main documentation
