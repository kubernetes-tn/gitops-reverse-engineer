# Milestone 5 Testing Guide

This guide provides step-by-step instructions for testing the Milestone 5 features: User/ServiceAccount exclusion and Namespace pattern matching.

## Prerequisites

- Kubernetes cluster with GitOps Reversed Admission Controller deployed
- `kubectl` configured to access the cluster
- Access to the Git repository for verification
- Permissions to create namespaces and resources

## Test Suite 1: User/ServiceAccount Exclusion

### Test 1.1: Exclude Specific User

**Objective:** Verify that changes from excluded users are not synced to Git.

**Steps:**

1. Configure the controller to exclude a test user:

```yaml
# values-override.yaml
watch:
  excludeUsers:
    - "test-user"
```

2. Deploy the updated configuration:

```bash
helm template gitops-reverse-engineer ./chart \
  --namespace gitops-reverse-engineer-system \
  -f values-override.yaml \
  | kubectl apply -f -
```

3. Wait for the pod to restart and load the new config:

```bash
kubectl rollout status deployment/gitops-reverse-engineer \
  -n gitops-reverse-engineer-system
```

4. Create a resource as the excluded user:

```bash
kubectl create configmap test-excluded \
  --from-literal=key=value \
  -n default \
  --as=test-user
```

5. Verify the logs show the request was skipped:

```bash
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer \
  --tail=20 | grep "test-user"
```

**Expected Output:**
```
⏭️ Skipping request from user test-user - user is in exclude list
✅ Allowed CREATE operation on ConfigMap/test-excluded in namespace default
```

6. Check Git repository - the file should NOT exist:

```bash
# The following path should not exist in Git
git ls-files | grep "default/configmap/test-excluded.yaml"
# Should return nothing
```

7. Clean up:

```bash
kubectl delete configmap test-excluded -n default
```

### Test 1.2: Exclude ServiceAccount

**Objective:** Verify that changes from excluded service accounts are not synced.

**Steps:**

1. Create a test service account:

```bash
kubectl create serviceaccount test-sa -n default
kubectl create clusterrolebinding test-sa-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=default:test-sa
```

2. Configure exclusion for this service account:

```yaml
# values-override.yaml
watch:
  excludeUsers:
    - "system:serviceaccount:default:test-sa"
```

3. Apply the configuration and restart the controller.

4. Create a resource using the service account:

```bash
kubectl create configmap test-sa-config \
  --from-literal=data=test \
  -n default \
  --as=system:serviceaccount:default:test-sa
```

5. Verify logs and Git repository (should not be synced).

6. Clean up:

```bash
kubectl delete configmap test-sa-config -n default
kubectl delete clusterrolebinding test-sa-admin
kubectl delete serviceaccount test-sa -n default
```

### Test 1.3: Non-Excluded User

**Objective:** Verify that non-excluded users' changes are still synced.

**Steps:**

1. Configure to exclude only specific users:

```yaml
watch:
  excludeUsers:
    - "blocked-user"
```

2. Create a resource as a non-excluded user:

```bash
kubectl create configmap test-allowed \
  --from-literal=key=value \
  -n default
```

3. Verify it IS synced to Git:

```bash
# Check Git repository
git pull
git log --oneline -1 default/configmap/test-allowed.yaml
```

4. Clean up:

```bash
kubectl delete configmap test-allowed -n default
```

## Test Suite 2: Namespace Pattern Matching

### Test 2.1: Exclude Pattern

**Objective:** Verify that namespaces matching exclude patterns are not synced.

**Steps:**

1. Configure to exclude `kube-*` pattern:

```yaml
watch:
  namespaces:
    excludePattern:
      - "kube-*"
```

2. Create test namespaces:

```bash
kubectl create ns kube-test
kubectl create ns kubetesting  # Should NOT match kube-*
kubectl create ns test-app
```

3. Create resources in each namespace:

```bash
kubectl create configmap test1 --from-literal=a=1 -n kube-test
kubectl create configmap test2 --from-literal=a=2 -n kubetesting
kubectl create configmap test3 --from-literal=a=3 -n test-app
```

4. Check logs:

```bash
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer \
  --tail=50 | grep "kube"
```

**Expected Output:**
```
⏭️ Skipping ConfigMap in namespace kube-test - namespace not in watch list
✅ Allowed CREATE operation on ConfigMap/test2 in namespace kubetesting
```

5. Verify Git repository:

```bash
git pull
git ls-files | grep configmap/test
```

**Expected:**
- `kube-test/configmap/test1.yaml` - Should NOT exist
- `kubetesting/configmap/test2.yaml` - Should exist
- `test-app/configmap/test3.yaml` - Should exist

6. Clean up:

```bash
kubectl delete ns kube-test kubetesting test-app
```

### Test 2.2: Include Pattern

**Objective:** Verify that only namespaces matching include patterns are synced.

**Steps:**

1. Configure to include only `prod-*` pattern:

```yaml
watch:
  namespaces:
    includePattern:
      - "prod-*"
```

2. Create test namespaces:

```bash
kubectl create ns prod-app
kubectl create ns prod-db
kubectl create ns dev-app
```

3. Create resources:

```bash
kubectl create deployment nginx --image=nginx -n prod-app
kubectl create deployment nginx --image=nginx -n prod-db
kubectl create deployment nginx --image=nginx -n dev-app
```

4. Verify Git repository:

```bash
git pull
git ls-files | grep deployment/nginx.yaml
```

**Expected:**
- `prod-app/deployment/nginx.yaml` - Should exist ✅
- `prod-db/deployment/nginx.yaml` - Should exist ✅
- `dev-app/deployment/nginx.yaml` - Should NOT exist ❌

5. Clean up:

```bash
kubectl delete ns prod-app prod-db dev-app
```

### Test 2.3: Multiple Patterns

**Objective:** Test complex pattern combinations.

**Steps:**

1. Configure multiple patterns:

```yaml
watch:
  namespaces:
    includePattern:
      - "app-*"
      - "service-*"
    excludePattern:
      - "*-temp"
```

2. Create test namespaces:

```bash
kubectl create ns app-frontend
kubectl create ns app-temp
kubectl create ns service-api
kubectl create ns service-temp
kubectl create ns database
```

3. Create resources in each:

```bash
for ns in app-frontend app-temp service-api service-temp database; do
  kubectl create configmap test --from-literal=ns=$ns -n $ns
done
```

4. Check which are synced:

```bash
git pull
git ls-files | grep configmap/test.yaml
```

**Expected:**
- `app-frontend/configmap/test.yaml` - ✅ (matches `app-*`)
- `app-temp/configmap/test.yaml` - ❌ (excluded by `*-temp`)
- `service-api/configmap/test.yaml` - ✅ (matches `service-*`)
- `service-temp/configmap/test.yaml` - ❌ (excluded by `*-temp`)
- `database/configmap/test.yaml` - ❌ (no include pattern matches)

5. Clean up:

```bash
kubectl delete ns app-frontend app-temp service-api service-temp database
```

### Test 2.4: Exact Name vs Pattern

**Objective:** Verify exact names take precedence over patterns.

**Steps:**

1. Configure with both exact and pattern:

```yaml
watch:
  namespaces:
    include:
      - "important"
    includePattern:
      - "prod-*"
    excludePattern:
      - "*-temp"
```

2. Create namespaces:

```bash
kubectl create ns important
kubectl create ns prod-app
kubectl create ns prod-temp
```

3. Create resources and verify:

```bash
kubectl create configmap test --from-literal=a=1 -n important
kubectl create configmap test --from-literal=a=2 -n prod-app
kubectl create configmap test --from-literal=a=3 -n prod-temp
```

4. Expected results:
- `important` - ✅ (exact match in include)
- `prod-app` - ✅ (matches `prod-*`)
- `prod-temp` - ❌ (excluded by `*-temp`)

5. Clean up:

```bash
kubectl delete ns important prod-app prod-temp
```

## Test Suite 3: Combined Filtering

### Test 3.1: User Exclusion + Namespace Pattern

**Objective:** Test both filters working together.

**Steps:**

1. Configure both filters:

```yaml
watch:
  namespaces:
    includePattern:
      - "prod-*"
  excludeUsers:
    - "deploy-bot"
```

2. Create a namespace:

```bash
kubectl create ns prod-app
```

3. Create resources from different users:

```bash
# As excluded user
kubectl create configmap bot-config \
  --from-literal=a=1 \
  -n prod-app \
  --as=deploy-bot

# As normal user
kubectl create configmap user-config \
  --from-literal=a=2 \
  -n prod-app
```

4. Verify Git repository:

**Expected:**
- `prod-app/configmap/bot-config.yaml` - ❌ (user excluded)
- `prod-app/configmap/user-config.yaml` - ✅ (user allowed, namespace matches)

5. Clean up:

```bash
kubectl delete ns prod-app
```

## Test Suite 4: Edge Cases

### Test 4.1: Special Characters in Patterns

**Steps:**

1. Test pattern with brackets:

```yaml
watch:
  namespaces:
    includePattern:
      - "app-[0-9]"  # Match app-0 through app-9
```

2. Create namespaces:

```bash
kubectl create ns app-1
kubectl create ns app-a
kubectl create ns app-10
```

3. Expected:
- `app-1` - ✅ (matches single digit)
- `app-a` - ❌ (letter not in range)
- `app-10` - ❌ (two digits, pattern expects one)

### Test 4.2: Empty Configuration

**Steps:**

1. Configure with empty patterns:

```yaml
watch:
  namespaces:
    includePattern: []
    excludePattern: []
  excludeUsers: []
```

2. Create resources - all should be synced (default behavior).

### Test 4.3: Invalid Pattern

**Steps:**

1. Configure with invalid pattern:

```yaml
watch:
  namespaces:
    includePattern:
      - "bad-[pattern"  # Unclosed bracket
```

2. Check controller logs for warnings:

```bash
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer \
  | grep "Error matching pattern"
```

**Expected:**
```
⚠️ Error matching pattern bad-[pattern: syntax error in pattern
```

## Verification Checklist

After running tests, verify:

- [ ] Excluded users' changes are not in Git
- [ ] Non-excluded users' changes are in Git
- [ ] Namespaces matching exclude patterns are skipped
- [ ] Namespaces matching include patterns are synced
- [ ] Exact namespace names work correctly
- [ ] Multiple patterns work together
- [ ] Combined user + namespace filtering works
- [ ] Logs show correct skip messages
- [ ] Invalid patterns generate warnings
- [ ] Default behavior (empty config) works

## Troubleshooting

### Changes are synced despite exclusion

1. Check the configuration is loaded:

```bash
kubectl get configmap gitops-reverse-engineer-config \
  -n gitops-reverse-engineer-system \
  -o yaml
```

2. Verify pod has restarted since config change:

```bash
kubectl get pods -n gitops-reverse-engineer-system -o wide
```

3. Check logs for the exclusion message.

### Patterns not matching as expected

1. Test the pattern locally:

```bash
# Use Go to test pattern matching
go run -e 'package main; import ("fmt"; "path/filepath"); func main() { matched, _ := filepath.Match("kube-*", "kube-system"); fmt.Println(matched) }'
```

2. Remember patterns use filepath.Match rules, not regex.

### Configuration not updating

1. Force pod restart:

```bash
kubectl rollout restart deployment/gitops-reverse-engineer \
  -n gitops-reverse-engineer-system
```

2. Check configmap checksum annotation changed on pod.

## Performance Testing

Test with many namespaces:

```bash
# Create 100 namespaces
for i in {1..100}; do
  kubectl create ns test-$i
  kubectl create configmap test --from-literal=i=$i -n test-$i
done

# Configure to exclude test-*
# Verify performance is acceptable
```

Check logs for timing information and ensure no performance degradation.
