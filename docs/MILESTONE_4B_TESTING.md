# Milestone 4.B: Testing Guide

## Overview

This guide provides step-by-step instructions to test the non-fast-forward pull error fix.

## Prerequisites

- Access to a Kubernetes cluster with the admission controller deployed
- Access to the Git repository used by the controller
- `kubectl` configured to access the cluster
- Git client installed

## Test Scenarios

### Test 1: Simulate Non-Fast-Forward Error

This test simulates the exact error condition by creating a divergent Git history.

**Steps:**

1. **Clone the Git repository locally**
   ```bash
   git clone <your-gitea-repo-url> /tmp/test-gitops
   cd /tmp/test-gitops
   ```

2. **Create a conflicting commit**
   ```bash
   # Make a change that will conflict with the controller
   mkdir -p test-cluster/default/configmap
   echo "apiVersion: v1
   kind: ConfigMap
   metadata:
     name: manual-test
     namespace: default
   data:
     test: manual-commit" > test-cluster/default/configmap/manual-test.yaml
   
   git add .
   git commit -m "Manual commit to trigger non-fast-forward"
   git push
   ```

3. **Trigger admission controller activity**
   ```bash
   kubectl create configmap test-cm -n default --from-literal=key=value
   ```

4. **Monitor the logs**
   ```bash
   kubectl logs -n gitops-reverse-engineer-system \
     -l app.kubernetes.io/instance=gitops-reverse-engineer \
     --tail=50 -f
   ```

   **Expected output:**
   ```
   ⚠️ Non-fast-forward update detected, performing force pull...
   ✅ Successfully force-pulled from remote (reset to abc1234)
   ✅ Successfully synced test-cluster/default/configmap/test-cm.yaml to Git
   ```

5. **Verify metrics**
   ```bash
   kubectl port-forward -n gitops-reverse-engineer-system \
     svc/gitops-reverse-engineer-metrics 8080:8080
   
   curl http://localhost:8080/metrics | grep non_fast_forward
   ```

   **Expected output:**
   ```
   # HELP gitops_admission_non_fast_forward_total Total number of non-fast-forward pull errors resolved
   # TYPE gitops_admission_non_fast_forward_total counter
   gitops_admission_non_fast_forward_total 1
   ```

### Test 2: Multiple Controllers with Concurrent Commits

This test validates behavior with multiple admission controller replicas.

**Steps:**

1. **Scale up the admission controller**
   ```bash
   kubectl scale deployment gitops-reverse-engineer \
     -n gitops-reverse-engineer-system --replicas=3
   ```

2. **Create multiple resources rapidly**
   ```bash
   for i in {1..10}; do
     kubectl create configmap test-cm-$i -n default --from-literal=index=$i &
   done
   wait
   ```

3. **Check logs from all pods**
   ```bash
   kubectl logs -n gitops-reverse-engineer-system \
     -l app.kubernetes.io/instance=gitops-reverse-engineer \
     --tail=100 | grep -E "(non-fast-forward|force-pulled)"
   ```

4. **Verify all resources are in Git**
   ```bash
   cd /tmp/test-gitops
   git pull
   ls -la test-cluster/default/configmap/test-cm-*.yaml
   # Should see all 10 ConfigMaps
   ```

### Test 3: Recovery from Extended Conflicts

Test that the retry mechanism works with the new force pull logic.

**Steps:**

1. **Stop the admission controller temporarily**
   ```bash
   kubectl scale deployment gitops-reverse-engineer \
     -n gitops-reverse-engineer-system --replicas=0
   ```

2. **Create resources in Kubernetes**
   ```bash
   kubectl create configmap retry-test-1 -n default --from-literal=test=1
   kubectl create configmap retry-test-2 -n default --from-literal=test=2
   ```

3. **Make conflicting commits to Git**
   ```bash
   cd /tmp/test-gitops
   echo "conflict" > test-cluster/default/configmap/conflict.yaml
   git add .
   git commit -m "Create conflict while controller is down"
   git push
   ```

4. **Restart the controller**
   ```bash
   kubectl scale deployment gitops-reverse-engineer \
     -n gitops-reverse-engineer-system --replicas=1
   ```

5. **Wait for retry loop to process pending operations**
   ```bash
   kubectl logs -n gitops-reverse-engineer-system \
     -l app.kubernetes.io/instance=gitops-reverse-engineer \
     --tail=100 -f
   ```

   **Expected output:**
   ```
   🔄 Processing 2 pending operations...
   ⚠️ Non-fast-forward update detected, performing force pull...
   ✅ Successfully force-pulled from remote
   ✅ Successfully processed pending operation: test-cluster/default/configmap/retry-test-1.yaml
   ✅ Successfully processed pending operation: test-cluster/default/configmap/retry-test-2.yaml
   📊 Pending operations remaining: 0
   ```

### Test 4: Branch Detection (main vs master)

Test that the force pull works with both common default branch names.

**Steps:**

1. **Check your repository's default branch**
   ```bash
   cd /tmp/test-gitops
   git remote show origin | grep "HEAD branch"
   ```

2. **If using master, rename to main**
   ```bash
   git checkout master
   git branch -m main
   git push -u origin main
   git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main
   ```

3. **Trigger a non-fast-forward scenario**
   ```bash
   # Create manual commit
   echo "test" > branch-test.yaml
   git add .
   git commit -m "Test branch detection"
   git push
   
   # Create resource in K8s
   kubectl create configmap branch-test -n default --from-literal=test=branch
   ```

4. **Verify force pull works**
   ```bash
   kubectl logs -n gitops-reverse-engineer-system \
     -l app.kubernetes.io/instance=gitops-reverse-engineer \
     --tail=20 | grep -E "(force-pulled|reset to)"
   ```

## Validation Checklist

- [ ] Non-fast-forward errors are automatically detected
- [ ] Force pull successfully resolves conflicts
- [ ] Metrics counter `non_fast_forward_total` increments correctly
- [ ] Multiple replicas can handle concurrent commits
- [ ] Retry queue processes pending operations after recovery
- [ ] Works with both `main` and `master` branches
- [ ] No manual intervention required for recovery
- [ ] Git repository contains all expected resources after tests

## Metrics to Monitor

After running tests, check these metrics:

```bash
kubectl port-forward -n gitops-reverse-engineer-system \
  svc/gitops-reverse-engineer-metrics 8080:8080

curl http://localhost:8080/metrics
```

Key metrics:
- `gitops_admission_non_fast_forward_total`: Should increment when conflicts occur
- `gitops_admission_git_sync_success_total`: Should increase after resolution
- `gitops_admission_git_sync_failures_total`: Should be minimal
- `gitops_admission_pending_operations`: Should return to 0 after processing

## Troubleshooting

### Issue: Force pull not triggered

**Check:**
```bash
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/instance=gitops-reverse-engineer | grep "non-fast-forward"
```

**Resolution:** Ensure the error message contains "non-fast-forward" string

### Issue: Reference not found error

**Check:**
```bash
cd /tmp/test-gitops
git branch -a
```

**Resolution:** Verify your default branch is either `main` or `master`

### Issue: Authentication failures

**Check:**
```bash
kubectl get secret -n gitops-reverse-engineer-system | grep gitea
kubectl describe secret <gitea-secret-name> -n gitops-reverse-engineer-system
```

**Resolution:** Verify GIT_TOKEN environment variable is set correctly

## Cleanup

After testing, clean up test resources:

```bash
# Delete test ConfigMaps
kubectl delete configmap -n default test-cm test-cm-{1..10} retry-test-{1,2} branch-test manual-test --ignore-not-found=true

# Scale down replicas (optional)
kubectl scale deployment gitops-reverse-engineer \
  -n gitops-reverse-engineer-system --replicas=1

# Remove local test repository
rm -rf /tmp/test-gitops
```

## Success Criteria

The implementation is successful if:

1. ✅ Non-fast-forward errors are detected and resolved automatically
2. ✅ No operations remain stuck in the pending queue
3. ✅ All Kubernetes resources are successfully committed to Git
4. ✅ Metrics accurately reflect non-fast-forward occurrences
5. ✅ Controller remains stable with multiple replicas
6. ✅ No manual intervention required during normal operations
