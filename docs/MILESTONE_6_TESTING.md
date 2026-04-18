# Milestone 6 Testing Guide

## Overview

This guide provides comprehensive testing scenarios for Milestone 6: Secret Data Obfuscation functionality.

## Prerequisites

1. GitOps Reversed Admission Controller deployed
2. Git repository configured and accessible
3. kubectl access to the cluster
4. Prometheus/metrics endpoint access

## Test Scenarios

### Test 1: Basic Secret Obfuscation

**Objective**: Verify that secret values are obfuscated when committed to Git

**Steps**:

1. Create a test secret with data:
```bash
kubectl create secret generic test-secret \
  --from-literal=username=admin \
  --from-literal=password=secret123 \
  -n default
```

2. Wait for the admission controller to process the request (a few seconds)

3. Check the Git repository:
```bash
cd /tmp/gitops-repo
git pull
cat <cluster-name>/default/secret/test-secret.yaml
```

**Expected Result**:
```yaml
apiVersion: v1
data:
  password: "********"
  username: "********"
kind: Secret
metadata:
  name: test-secret
  namespace: default
type: Opaque
```

**Verification**:
- ✅ Secret file exists in Git
- ✅ Keys (username, password) are visible
- ✅ Values are replaced with `********`
- ✅ Metadata is preserved
- ✅ Secret type is preserved

---

### Test 2: Secret with StringData

**Objective**: Verify obfuscation works with stringData field

**Steps**:

1. Create a secret manifest with stringData:
```yaml
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: test-stringdata
  namespace: default
type: Opaque
stringData:
  api-key: "sk-1234567890abcdef"
  token: "ghp_abc123xyz789"
EOF
```

2. Check the Git repository:
```bash
cat <cluster-name>/default/secret/test-stringdata.yaml
```

**Expected Result**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: test-stringdata
  namespace: default
stringData:
  api-key: "********"
  token: "********"
type: Opaque
```

**Verification**:
- ✅ stringData keys are visible
- ✅ stringData values are obfuscated

---

### Test 3: TLS Secret Obfuscation

**Objective**: Verify TLS secrets are properly obfuscated

**Steps**:

1. Create a TLS secret:
```bash
# Generate test certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /tmp/tls.key -out /tmp/tls.crt \
  -subj "/CN=test.example.com"

# Create TLS secret
kubectl create secret tls test-tls-secret \
  --cert=/tmp/tls.crt \
  --key=/tmp/tls.key \
  -n default
```

2. Check Git repository:
```bash
cat <cluster-name>/default/secret/test-tls-secret.yaml
```

**Expected Result**:
```yaml
apiVersion: v1
data:
  tls.crt: "********"
  tls.key: "********"
kind: Secret
metadata:
  name: test-tls-secret
  namespace: default
type: kubernetes.io/tls
```

**Verification**:
- ✅ Certificate and key are obfuscated
- ✅ Secret type is `kubernetes.io/tls`

---

### Test 4: Secret Update - New Key Added

**Objective**: Verify change detection when new keys are added

**Steps**:

1. Update the secret to add a new key:
```bash
kubectl patch secret test-secret -n default \
  --type='json' \
  -p='[{"op": "add", "path": "/data/email", "value": "YWRtaW5AZXhhbXBsZS5jb20="}]'
```

2. Check Git commit history:
```bash
cd /tmp/gitops-repo
git log --oneline -5
git show HEAD:<cluster-name>/default/secret/test-secret.yaml
```

**Expected Result**:
- New commit created
- Secret file now contains three keys: username, password, email
- All values are `********`

**Verification**:
- ✅ New commit exists
- ✅ Change was detected
- ✅ New key is visible
- ✅ All values remain obfuscated

---

### Test 5: Secret Update - Key Removed

**Objective**: Verify change detection when keys are removed

**Steps**:

1. Remove a key from the secret:
```bash
kubectl patch secret test-secret -n default \
  --type='json' \
  -p='[{"op": "remove", "path": "/data/email"}]'
```

2. Check Git commit:
```bash
git pull
git log --oneline -5
cat <cluster-name>/default/secret/test-secret.yaml
```

**Expected Result**:
- New commit created
- Email key removed from the file
- Remaining keys still obfuscated

**Verification**:
- ✅ Change detected and committed
- ✅ Removed key no longer in file

---

### Test 6: Secret Update - Value Changed (Same Keys)

**Objective**: Verify behavior when only values change but keys remain the same

**Steps**:

1. Update secret value:
```bash
kubectl patch secret test-secret -n default \
  --type='json' \
  -p='[{"op": "replace", "path": "/data/password", "value": "bmV3cGFzc3dvcmQxMjM="}]'
```

2. Check Git:
```bash
git pull
git log --oneline -5
```

**Expected Result**:
- **No new commit** (because keys are the same, and we can't detect value changes)
- File remains unchanged in Git

**Verification**:
- ✅ No new commit created
- ⚠️ Value change not detected (expected limitation)

---

### Test 7: Secret Update - Metadata Changed

**Objective**: Verify change detection for metadata changes

**Steps**:

1. Add a label to the secret:
```bash
kubectl label secret test-secret -n default env=production
```

2. Check Git:
```bash
git pull
git log --oneline -5
cat <cluster-name>/default/secret/test-secret.yaml
```

**Expected Result**:
- New commit created
- Label visible in metadata

**Verification**:
- ✅ Metadata change detected
- ✅ New commit with label

---

### Test 8: Secret Deletion

**Objective**: Verify secret deletion is tracked in Git

**Steps**:

1. Delete the secret:
```bash
kubectl delete secret test-secret -n default
```

2. Check Git:
```bash
git pull
git log --oneline -5
```

**Expected Result**:
- File removed from Git
- Commit message: `DELETE: Secret default/test-secret`

**Verification**:
- ✅ File deleted
- ✅ Deletion commit exists

---

### Test 9: Metrics Verification

**Objective**: Verify Prometheus metrics are correctly tracking secret operations

**Steps**:

1. Create multiple secrets:
```bash
for i in {1..3}; do
  kubectl create secret generic metric-test-$i \
    --from-literal=key=value \
    -n default
done
```

2. Query metrics endpoint:
```bash
kubectl port-forward -n gitops-reverse-engineer-system \
  svc/gitops-reverse-engineer 8443:443 &

curl -k https://localhost:8443/metrics | grep secret
```

**Expected Metrics**:
```
gitops_admission_obfuscated_secrets_total 3
gitops_admission_secret_changes_detected_total 0
```

3. Update one secret to add a key:
```bash
kubectl patch secret metric-test-1 -n default \
  --type='json' \
  -p='[{"op": "add", "path": "/data/newkey", "value": "bmV3dmFsdWU="}]'
```

4. Check metrics again:
```bash
curl -k https://localhost:8443/metrics | grep secret
```

**Expected Metrics**:
```
gitops_admission_obfuscated_secrets_total 4
gitops_admission_secret_changes_detected_total 1
```

**Verification**:
- ✅ `obfuscated_secrets_total` increments on create/update
- ✅ `secret_changes_detected_total` increments when changes detected

---

### Test 10: DockerRegistry Secret

**Objective**: Verify Docker registry secrets are obfuscated

**Steps**:

1. Create a Docker registry secret:
```bash
kubectl create secret docker-registry regcred \
  --docker-server=https://index.docker.io/v1/ \
  --docker-username=myuser \
  --docker-password=mypassword \
  --docker-email=myemail@example.com \
  -n default
```

2. Check Git:
```bash
cat <cluster-name>/default/secret/regcred.yaml
```

**Expected Result**:
```yaml
apiVersion: v1
data:
  .dockerconfigjson: "********"
kind: Secret
metadata:
  name: regcred
  namespace: default
type: kubernetes.io/dockerconfigjson
```

**Verification**:
- ✅ Docker config is obfuscated
- ✅ Type is preserved

---

### Test 11: Secret with Annotations

**Objective**: Verify annotations are preserved during obfuscation

**Steps**:

1. Create secret with annotations:
```yaml
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: annotated-secret
  namespace: default
  annotations:
    owner: "team-a"
    purpose: "database-credentials"
type: Opaque
data:
  db-password: cGFzc3dvcmQ=
EOF
```

2. Check Git:
```bash
cat <cluster-name>/default/secret/annotated-secret.yaml
```

**Expected Result**:
```yaml
apiVersion: v1
data:
  db-password: "********"
kind: Secret
metadata:
  annotations:
    owner: team-a
    purpose: database-credentials
  name: annotated-secret
  namespace: default
type: Opaque
```

**Verification**:
- ✅ Annotations preserved
- ✅ Values obfuscated

---

### Test 12: Compare with Non-Secret Resources

**Objective**: Verify non-secret resources are NOT obfuscated

**Steps**:

1. Create a ConfigMap:
```bash
kubectl create configmap test-config \
  --from-literal=app-name=myapp \
  --from-literal=log-level=debug \
  -n default
```

2. Check Git:
```bash
cat <cluster-name>/default/configmap/test-config.yaml
```

**Expected Result**:
```yaml
apiVersion: v1
data:
  app-name: myapp
  log-level: debug
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
```

**Verification**:
- ✅ ConfigMap values are NOT obfuscated
- ✅ Only Secrets are obfuscated

---

## Performance Testing

### Test 13: High Volume Secret Creation

**Objective**: Test performance with many secrets

**Steps**:

1. Create 50 secrets:
```bash
for i in {1..50}; do
  kubectl create secret generic perf-test-$i \
    --from-literal=key1=value1 \
    --from-literal=key2=value2 \
    -n default &
done
wait
```

2. Monitor logs:
```bash
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer \
  --tail=100 -f
```

3. Check metrics:
```bash
curl -k https://localhost:8443/metrics | grep obfuscated
```

**Expected Result**:
- All secrets processed successfully
- Counter shows 50+ obfuscated secrets
- No errors in logs

---

## Troubleshooting

### Secret Not Obfuscated in Git

**Symptoms**: Secret values visible in Git

**Checks**:
1. Verify the resource is of kind `Secret`
2. Check controller logs for errors
3. Verify the admission webhook is functioning

### Changes Not Detected

**Symptoms**: Updates don't create new commits

**Checks**:
1. If only values changed (not keys), this is expected behavior
2. Check if metadata/keys actually changed
3. Review logs for "no changes detected" messages

### Metrics Not Updating

**Symptoms**: Prometheus metrics stuck at 0

**Checks**:
1. Verify metrics are enabled in deployment
2. Check if metrics endpoint is accessible
3. Ensure secrets are being processed (check logs)

---

## Cleanup

Remove all test resources:
```bash
# Delete secrets
kubectl delete secret --all -n default

# Clean up test files
rm -f /tmp/tls.key /tmp/tls.crt
```

---

## Success Criteria

All tests should pass with the following results:

- ✅ All secret values obfuscated with `********`
- ✅ Secret keys visible in Git
- ✅ Metadata preserved (labels, annotations, type)
- ✅ New keys detected and committed
- ✅ Removed keys detected and committed
- ✅ Value-only changes NOT detected (expected limitation)
- ✅ Non-secret resources not affected
- ✅ Metrics accurately track operations
- ✅ No errors in controller logs
- ✅ Performance acceptable under load

---

## Related Documentation

- [Milestone 6 Implementation Guide](MILESTONE_6_IMPLEMENTATION.md)
- [ROADMAP.md](ROADMAP.md)
- [README.md](../README.md)
