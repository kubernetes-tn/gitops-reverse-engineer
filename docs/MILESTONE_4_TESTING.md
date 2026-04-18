# Milestone 4 Testing Guide

## Prerequisites
- Admission controller deployed and running
- Metrics enabled in configuration
- Access to Kubernetes cluster
- Git repository configured

## Test 1: Verify New Resource Creation

### Steps
```bash
# Create a test deployment
kubectl create deployment test-app --image=nginx:1.20 -n default

# Check logs
kubectl logs -n gitops-reverse-engineer deployment/gitops-reverse-engineer

# Expected log output:
# ✅ Successfully synced default-cluster/default/deployment/test-app.yaml to Git
```

### Verification
1. Check Git repository for commit
2. Verify file exists at: `default-cluster/default/deployment/test-app.yaml`
3. Check metrics:
   ```bash
   curl http://localhost:8443/metrics | grep gitops_admission_git_sync_success_total
   # Should increment by 1
   ```

## Test 2: Verify No-Change Update Optimization

### Steps
```bash
# Apply the same deployment again (no changes)
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
EOF

# Check logs
kubectl logs -n gitops-reverse-engineer deployment/gitops-reverse-engineer
```

### Expected Results
```
⏭️  Skipping commit for default-cluster/default/deployment/test-app.yaml - no changes detected
```

### Verification
1. **NO new commit in Git repository**
2. Git history shows only 1 commit for this deployment
3. Check metrics:
   ```bash
   curl http://localhost:8443/metrics | grep gitops_admission_skipped_commits_total
   # Should increment by 1
   ```

## Test 3: Verify Actual Change Detection

### Steps
```bash
# Update deployment with real change (different image version)
kubectl set image deployment/test-app nginx=nginx:1.21 -n default

# Check logs
kubectl logs -n gitops-reverse-engineer deployment/gitops-reverse-engineer
```

### Expected Results
```
📝 Handling UPDATE for /tmp/gitops-repo/default-cluster/default/deployment/test-app.yaml
✅ Successfully synced default-cluster/default/deployment/test-app.yaml to Git
```

### Verification
1. **New commit created in Git repository**
2. Git history shows 2 commits for this deployment
3. File content shows `image: nginx:1.21`
4. Check metrics:
   ```bash
   curl http://localhost:8443/metrics | grep gitops_admission_git_sync_success_total
   # Should increment by 1 (total 2)
   ```

## Test 4: Multiple Rapid No-Change Updates

### Steps
```bash
# Apply same deployment multiple times rapidly
for i in {1..5}; do
  kubectl annotate deployment test-app test-annotation-$i=value --overwrite -n default
  kubectl annotate deployment test-app test-annotation-$i- -n default
done

# Check logs and metrics
kubectl logs -n gitops-reverse-engineer deployment/gitops-reverse-engineer --tail=20
curl http://localhost:8443/metrics | grep skipped_commits_total
```

### Expected Results
- Multiple "Skipping commit" log entries
- `gitops_admission_skipped_commits_total` increments by 5
- No new commits in Git repository

## Test 5: Metrics Verification

### Steps
```bash
# Get all metrics
curl http://localhost:8443/metrics
```

### Expected Output
```
# HELP gitops_admission_git_sync_success_total Total number of successful Git syncs
# TYPE gitops_admission_git_sync_success_total counter
gitops_admission_git_sync_success_total 2

# HELP gitops_admission_git_sync_failures_total Total number of failed Git syncs
# TYPE gitops_admission_git_sync_failures_total counter
gitops_admission_git_sync_failures_total 0

# HELP gitops_admission_skipped_commits_total Total number of skipped commits (no changes detected)
# TYPE gitops_admission_skipped_commits_total counter
gitops_admission_skipped_commits_total 6

# HELP gitops_admission_pending_operations Current number of pending operations
# TYPE gitops_admission_pending_operations gauge
gitops_admission_pending_operations 0
```

## Test 6: YAML Formatting Differences

### Purpose
Verify that minor YAML formatting differences don't trigger false changes

### Steps
```bash
# Create deployment with specific formatting
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: format-test
  namespace: default
  labels:
    app: test
    version: "1.0"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: nginx:alpine
EOF

# Apply again with different formatting (extra spaces, different order)
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: format-test
  namespace: default
  labels:
    version: "1.0"
    app: test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: nginx:alpine
EOF
```

### Expected Results
- Second apply should be skipped (no commit)
- Log shows "Skipping commit - no changes detected"
- YAML normalization handles formatting differences

## Cleanup

```bash
# Delete test resources
kubectl delete deployment test-app -n default
kubectl delete deployment format-test -n default

# These deletions will create DELETE commits in Git
```

## Success Criteria

✅ All tests pass
✅ Skipped commits metric tracks optimization correctly
✅ Git repository only contains commits for actual changes
✅ No unnecessary commits for identical resources
✅ YAML formatting differences are normalized correctly
✅ Metrics endpoint exports all four metrics

## Troubleshooting

### Issue: All updates create commits
**Cause**: YAML normalization might be failing
**Debug**: Check logs for warnings about normalization
**Fix**: Review `normalizeYAML()` function

### Issue: Skipped commits metric not incrementing
**Cause**: Metrics not enabled or collector not initialized
**Debug**: Check config file and startup logs
**Fix**: Ensure `.Values.metrics.enabled` is true

### Issue: File comparison errors
**Cause**: Permission issues or file system problems
**Debug**: Check logs for "Error checking file changes"
**Fix**: Verify file permissions in `/tmp/gitops-repo`
