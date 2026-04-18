# Milestone 8 Testing Guide

This guide provides comprehensive testing procedures for Milestone 8 features: cluster-specific values files and host aliases support.

## Prerequisites

- Kubernetes cluster access
- `kubectl` configured
- `helm` installed
- `make` utility
- Git repository access (Gitea or similar)
- Multiple cluster contexts (optional, for multi-cluster testing)

## Test Suite Overview

1. **Cluster-Specific Values Files**: Test automatic detection and application
2. **Host Aliases**: Test custom DNS resolution
3. **Multi-Cluster Deployment**: Test deploying to multiple clusters
4. **Values Precedence**: Test configuration override behavior
5. **Edge Cases**: Test missing files, invalid configurations

## Test 1: Cluster-Specific Values Detection

### Objective
Verify that the deployment system correctly detects and uses cluster-specific values files.

### Steps

**1.1 Verify values directory structure**

```bash
cd gitops-reverse-engineer
ls -la values/
```

**Expected output**:
```
values.dev.yaml
values.platform.yaml
values.production.yaml
values.staging.yaml
```

**1.2 Test deployment with existing cluster-specific file**

```bash
make deploy CLUSTER_NAME=dev TAG=v1.0.0
```

**Expected output**:
```
🚀 Deploying to Kubernetes using Helm...
📝 Cluster: dev
📝 Updating deployment image to ghcr.io/kubernetes-tn/gitops-reverse-engineer:v1.0.0...
🔧 Using helm template to generate manifests...
📄 Using values-certs-override.yaml for TLS configuration
📄 Using cluster-specific values from values/values.dev.yaml
```

**1.3 Test deployment with non-existent cluster-specific file**

```bash
make deploy CLUSTER_NAME=nonexistent TAG=v1.0.0
```

**Expected output**:
```
🚀 Deploying to Kubernetes using Helm...
📝 Cluster: nonexistent
📝 Updating deployment image to ghcr.io/kubernetes-tn/gitops-reverse-engineer:v1.0.0...
🔧 Using helm template to generate manifests...
📄 Using values-certs-override.yaml for TLS configuration
ℹ️  No cluster-specific values file found at values/values.nonexistent.yaml
```

**1.4 Verify cluster name is set correctly**

```bash
kubectl get configmap -n gitops-reverse-engineer-system \
  gitops-reverse-engineer-config -o yaml | grep clusterName
```

**Expected**: Should show `clusterName: dev` (or whatever CLUSTER_NAME you used)

**✅ Pass Criteria**:
- Deployment succeeds with existing cluster-specific file
- Deployment succeeds without cluster-specific file (using defaults)
- Correct informational messages displayed
- Cluster name correctly configured

## Test 2: Host Aliases Configuration

### Objective
Verify that hostAliases are correctly injected into pods.

### Steps

**2.1 Create test values file with hostAliases**

```bash
cat > values/values.test-hostalias.yaml <<'EOF'
git:
  repoUrl: "https://test-git.example.com/repo.git"
  existingSecret: "git-token"
  clusterName: "test-hostalias"

hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "test-gitea.internal"
      - "git.test.local"
  - ip: "10.x.x.x"
    hostnames:
      - "registry.internal"

replicaCount: 1
EOF
```

**2.2 Deploy with host aliases**

```bash
make deploy CLUSTER_NAME=test-hostalias TAG=v1.0.0
```

**2.3 Verify hostAliases in pod spec**

```bash
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml | grep -A 10 hostAliases
```

**Expected output**:
```yaml
hostAliases:
- hostnames:
  - test-gitea.internal
  - git.test.local
  ip: 192.168.x.x
- hostnames:
  - registry.internal
  ip: 10.x.x.x
```

**2.4 Verify /etc/hosts in running pod**

```bash
kubectl exec -n gitops-reverse-engineer-system \
  deployment/gitops-reverse-engineer -- cat /etc/hosts
```

**Expected output** (should include):
```
192.168.x.x   test-gitea.internal git.test.local
10.x.x.x       registry.internal
```

**2.5 Test DNS resolution from pod**

```bash
# Test that custom hostname resolves to custom IP
kubectl exec -n gitops-reverse-engineer-system \
  deployment/gitops-reverse-engineer -- getent hosts test-gitea.internal
```

**Expected output**:
```
192.168.x.x   test-gitea.internal
```

**✅ Pass Criteria**:
- hostAliases present in deployment spec
- Custom entries visible in /etc/hosts
- Custom hostnames resolve to specified IPs

## Test 3: Values Precedence and Overrides

### Objective
Verify that values are merged correctly with proper precedence.

### Steps

**3.1 Create test values with specific overrides**

```bash
cat > values/values.test-precedence.yaml <<'EOF'
git:
  clusterName: "from-values-file"

replicaCount: 5

resources:
  limits:
    cpu: 999m
    memory: 999Mi
EOF
```

**3.2 Deploy and check precedence**

```bash
make deploy CLUSTER_NAME=test-precedence TAG=v1.0.0
```

**3.3 Verify final configuration**

```bash
# Check replica count (should be 5 from values file)
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system \
  -o jsonpath='{.spec.replicas}'

# Check resource limits (should be 999m/999Mi from values file)
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system \
  -o jsonpath='{.spec.template.spec.containers[0].resources.limits}'

# Check cluster name (should be "test-precedence" from --set, NOT from values file)
kubectl get configmap -n gitops-reverse-engineer-system \
  gitops-reverse-engineer-config -o yaml | grep clusterName
```

**Expected**:
- Replicas: `5` (from values file)
- CPU limit: `999m` (from values file)
- Memory limit: `999Mi` (from values file)
- Cluster name: `test-precedence` (from --set flag, highest priority)

**✅ Pass Criteria**:
- Values file overrides base values.yaml
- --set flags override values files
- No unexpected configurations

## Test 4: Multi-Cluster Deployment Simulation

### Objective
Simulate deploying to multiple clusters with different configurations.

### Steps

**4.1 Review cluster-specific configurations**

```bash
# Compare dev vs production configurations
echo "=== DEV ==="
grep -E "(replicaCount|cpu:|memory:)" values/values.dev.yaml

echo "=== PRODUCTION ==="
grep -E "(replicaCount|cpu:|memory:)" values/values.production.yaml
```

**Expected differences**:
- Dev: 1 replica, 250m CPU, 256Mi memory
- Production: 2 replicas, 1000m CPU, 1Gi memory

**4.2 Deploy to "dev" cluster**

```bash
make deploy CLUSTER_NAME=dev TAG=v1.0.0
```

**4.3 Check dev deployment configuration**

```bash
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml | grep -E "(replicas:|cpu:|memory:)"
```

**4.4 Deploy to "production" cluster (simulated)**

```bash
make deploy CLUSTER_NAME=production TAG=v1.0.0
```

**4.5 Check production deployment configuration**

```bash
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml | grep -E "(replicas:|cpu:|memory:)"
```

**4.6 Verify configurations match expectations**

Compare the outputs from steps 4.3 and 4.5.

**✅ Pass Criteria**:
- Dev deployment has dev-specific resources
- Production deployment has production-specific resources
- Easy switching between cluster configurations

## Test 5: Deployment Without Cluster-Specific Values

### Objective
Verify graceful fallback when no cluster-specific values file exists.

### Steps

**5.1 Deploy with cluster name that has no values file**

```bash
make deploy CLUSTER_NAME=minimal-cluster TAG=v1.0.0
```

**Expected output**:
```
ℹ️  No cluster-specific values file found at values/values.minimal-cluster.yaml
```

**5.2 Verify deployment uses base values**

```bash
# Should use defaults from chart/values.yaml
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml | grep -E "(replicas:|cpu:|memory:)"
```

**Expected**: Default values from `chart/values.yaml`:
- Replicas: 1
- CPU limits: 500m
- Memory limits: 512Mi

**✅ Pass Criteria**:
- Deployment succeeds without cluster-specific file
- Base values.yaml is used as fallback
- Informative message displayed

## Test 6: Host Aliases with Real Git Server

### Objective
Test that host aliases work for Git operations (if you have a test Git server).

### Steps

**6.1 Set up test Git server with custom IP**

Assume you have a Git server at `192.168.x.x` that should be accessed via hostname `gitea.test.local`.

**6.2 Create values with hostAliases**

```bash
cat > values/values.test-git.yaml <<'EOF'
git:
  repoUrl: "https://gitea.test.local/gitops/test-cluster.git"
  token: "your-test-token"
  clusterName: "test-git"

hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "gitea.test.local"

replicaCount: 1
EOF
```

**6.3 Deploy**

```bash
make deploy CLUSTER_NAME=test-git TAG=v1.0.0
```

**6.4 Create test resource to trigger Git sync**

```bash
kubectl create configmap test-git-sync -n default \
  --from-literal=test=value
```

**6.5 Check admission controller logs**

```bash
make logs
```

**Expected**: Logs should show successful Git operations using `gitea.test.local` hostname.

**✅ Pass Criteria**:
- Git operations succeed using custom hostname
- No DNS resolution errors
- Repository synced correctly

## Test 7: Helm Template Dry-Run Validation

### Objective
Validate generated manifests without deploying.

### Steps

**7.1 Test template generation with dev values**

```bash
helm template gitops-reverse-engineer ./chart \
  --namespace gitops-reverse-engineer-system \
  -f values-certs-override.yaml \
  -f values/values.dev.yaml \
  --set image.tag=v1.0.0 \
  --set git.clusterName=dev \
  --debug > /tmp/dev-manifest.yaml
```

**7.2 Inspect generated manifest**

```bash
# Check hostAliases (should be empty for dev)
grep -A 5 hostAliases /tmp/dev-manifest.yaml

# Check resources
grep -A 5 "resources:" /tmp/dev-manifest.yaml

# Check replicas
grep replicas /tmp/dev-manifest.yaml
```

**7.3 Test template with production values**

```bash
helm template gitops-reverse-engineer ./chart \
  --namespace gitops-reverse-engineer-system \
  -f values-certs-override.yaml \
  -f values/values.production.yaml \
  --set image.tag=v1.0.0 \
  --set git.clusterName=production \
  --debug > /tmp/prod-manifest.yaml
```

**7.4 Compare dev vs production manifests**

```bash
diff /tmp/dev-manifest.yaml /tmp/prod-manifest.yaml
```

**Expected**: Differences in replicas, resources, alert severity, etc.

**✅ Pass Criteria**:
- Template generation succeeds for all cluster types
- Generated manifests have expected differences
- No YAML syntax errors

## Test 8: Edge Cases

### 8.1 Invalid YAML in Values File

**Test**: Create invalid YAML

```bash
cat > values/values.invalid.yaml <<'EOF'
this is not: valid: yaml::
  - broken
EOF
```

```bash
make deploy CLUSTER_NAME=invalid TAG=v1.0.0
```

**Expected**: Helm template should fail with clear error message.

### 8.2 Missing Required Secret

**Test**: Deploy without creating required secret

```bash
# Delete git-token secret if it exists
kubectl delete secret git-token -n gitops-reverse-engineer-system --ignore-not-found

# Deploy
make deploy CLUSTER_NAME=dev TAG=v1.0.0
```

**Expected**: Deployment succeeds (secret is referenced but pod will fail to start).

**Verify pod status**:
```bash
kubectl get pods -n gitops-reverse-engineer-system
```

**Expected**: Pod in `CreateContainerConfigError` state due to missing secret.

### 8.3 Conflicting Host Aliases

**Test**: Multiple IPs for same hostname

```bash
cat > values/values.conflict.yaml <<'EOF'
hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "test.local"
  - ip: "192.168.x.x"
    hostnames:
      - "test.local"
EOF
```

```bash
make deploy CLUSTER_NAME=conflict TAG=v1.0.0
```

**Expected**: Deployment succeeds (Kubernetes allows this, last entry wins).

**Verify**:
```bash
kubectl exec -n gitops-reverse-engineer-system \
  deployment/gitops-reverse-engineer -- cat /etc/hosts | grep test.local
```

## Integration Testing

### Full End-to-End Test

**Scenario**: Deploy to production-like environment with all features.

```bash
# 1. Generate certificates
make cert

# 2. Create production-like values
cat > values/values.e2e-test.yaml <<'EOF'
git:
  repoUrl: "https://gitea.your-domain.com/gitops/e2e-test.git"
  existingSecret: "git-token"
  clusterName: "e2e-test"

replicaCount: 2

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 512Mi

hostAliases:
  - ip: "YOUR_GITEA_IP"
    hostnames:
      - "gitea.your-domain.com"

watch:
  clusterWideResources: true
  namespaces:
    exclude:
      - "kube-system"
    excludePattern:
      - "kube-*"

metrics:
  enabled: true
  serviceMonitor:
    enabled: true

prometheusRule:
  enabled: true
EOF
```

**3. Build and deploy**

```bash
make build TAG=v1.0.0-e2e
make push TAG=v1.0.0-e2e
make deploy CLUSTER_NAME=e2e-test TAG=v1.0.0-e2e
```

**4. Wait for deployment**

```bash
kubectl rollout status deployment/gitops-reverse-engineer \
  -n gitops-reverse-engineer-system
```

**5. Test resource creation**

```bash
kubectl create namespace test-e2e
kubectl create configmap test-config -n test-e2e --from-literal=key=value
```

**6. Verify Git sync**

Check your Git repository for:
```
e2e-test/
  test-e2e/
    configmap/
      test-config.yaml
  _cluster/
    namespace/
      test-e2e.yaml
```

**7. Check metrics**

```bash
kubectl port-forward -n gitops-reverse-engineer-system \
  svc/gitops-reverse-engineer 8443:443

curl -k https://localhost:8443/metrics | grep gitops_admission
```

**8. Cleanup**

```bash
kubectl delete namespace test-e2e
make delete
```

## Troubleshooting Guide

### Issue: Values file not being used

**Check**:
```bash
# Verify file exists
ls -la values/values.yourcluster.yaml

# Check file permissions
ls -l values/values.yourcluster.yaml

# Verify CLUSTER_NAME matches exactly
echo "CLUSTER_NAME should match: values.${CLUSTER_NAME}.yaml"
```

### Issue: hostAliases not in pod

**Check**:
```bash
# Verify hostAliases in values file
cat values/values.yourcluster.yaml | grep -A 10 hostAliases

# Check deployment spec
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml | grep -A 20 hostAliases

# If not present, check Helm template output
helm template gitops-reverse-engineer ./chart \
  -f values/values.yourcluster.yaml --debug | grep -A 20 hostAliases
```

### Issue: Wrong configuration applied

**Debug**:
```bash
# Generate manifest without applying
helm template gitops-reverse-engineer ./chart \
  -f values-certs-override.yaml \
  -f values/values.yourcluster.yaml \
  --set image.tag=v1.0.0 \
  --set git.clusterName=yourcluster \
  --debug > debug-manifest.yaml

# Inspect specific sections
grep -A 10 "kind: Deployment" debug-manifest.yaml
```

## Success Criteria Summary

All tests should meet these criteria:

✅ **Cluster-Specific Values**:
- Automatic detection works
- Values correctly override base configuration
- Graceful fallback when file missing

✅ **Host Aliases**:
- Correctly injected into pod spec
- Visible in /etc/hosts
- DNS resolution works

✅ **Multi-Cluster**:
- Can deploy to different "clusters" with different configs
- Easy switching between configurations
- No manual script modifications needed

✅ **Values Precedence**:
- Base → Certs → Cluster-specific → CLI flags
- Predictable override behavior

✅ **Edge Cases**:
- Invalid YAML handled gracefully
- Missing files don't break deployment
- Clear error messages

## Cleanup

After testing:

```bash
# Remove test values files
rm -f values/values.test-*.yaml
rm -f values/values.e2e-test.yaml
rm -f values/values.invalid.yaml
rm -f values/values.conflict.yaml

# Delete test deployment
make delete

# Clean generated files
make clean
```

## Automation

For CI/CD integration, create a test script:

```bash
#!/bin/bash
# test-milestone-8.sh

set -e

echo "Testing Milestone 8 features..."

# Test 1: Deployment with cluster-specific values
make deploy CLUSTER_NAME=dev TAG=test

# Test 2: Verify hostAliases
kubectl get deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system -o yaml | grep -q hostAliases && \
  echo "✅ hostAliases test passed" || echo "❌ hostAliases test failed"

# Test 3: Verify cluster name
CLUSTER_NAME=$(kubectl get configmap -n gitops-reverse-engineer-system \
  gitops-reverse-engineer-config -o yaml | grep clusterName | awk '{print $2}')
[ "$CLUSTER_NAME" == "dev" ] && \
  echo "✅ Cluster name test passed" || echo "❌ Cluster name test failed"

# Cleanup
make delete

echo "All tests completed!"
```

Make it executable and run:
```bash
chmod +x test-milestone-8.sh
./test-milestone-8.sh
```
