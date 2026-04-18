# Milestone 6 Implementation Guide

## Overview

Milestone 6 implements **Secret Data Obfuscation** functionality to enhance security when syncing Kubernetes Secrets to Git repositories. This prevents sensitive data from being stored in plain text in Git history while maintaining the ability to track changes.

## Features Implemented

### 1. Secret Data Obfuscation

Secret values are obfuscated before being committed to Git, similar to how ArgoCD displays secrets in its UI.

**Behavior:**
- Secret `data` field (base64-encoded values) → Keys preserved, values replaced with `********`
- Secret `stringData` field (plain text values) → Keys preserved, values replaced with `********`
- Metadata (labels, annotations) → Preserved as-is
- Secret type → Preserved as-is

**Example:**

Original Secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
type: Opaque
data:
  username: YWRtaW4=
  password: cGFzc3dvcmQxMjM=
```

Obfuscated in Git:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
type: Opaque
data:
  username: "********"
  password: "********"
```

### 2. Secret Change Detection

The controller can detect actual changes in secrets even when the data is obfuscated.

**Changes Detected:**
- ✅ New keys added to `data` or `stringData`
- ✅ Keys removed from `data` or `stringData`
- ✅ Secret type changed
- ✅ Labels or annotations changed
- ❌ Values changed (not detectable when obfuscated)

**Note:** Since values are obfuscated, the controller cannot detect if a secret value has changed but the keys remain the same. This is a security trade-off to prevent storing sensitive data in Git.

### 3. Prometheus Metrics

Two new metrics track secret operations:

1. **`gitops_admission_obfuscated_secrets_total`** (counter)
   - Total number of secrets obfuscated before committing to Git
   - Incremented every time a Secret resource is processed

2. **`gitops_admission_secret_changes_detected_total`** (counter)
   - Total number of secret changes detected (even when obfuscated)
   - Incremented when the change detection logic identifies differences

## Implementation Details

### Code Changes

#### 1. `gitea.go` - Secret Obfuscation Function

```go
func obfuscateSecretData(secretObj map[string]interface{}) {
    // Obfuscate the 'data' field
    if data, ok := secretObj["data"].(map[string]interface{}); ok {
        obfuscatedData := make(map[string]interface{})
        for key := range data {
            obfuscatedData[key] = "********"
        }
        secretObj["data"] = obfuscatedData
    }
    
    // Obfuscate the 'stringData' field
    if stringData, ok := secretObj["stringData"].(map[string]interface{}); ok {
        obfuscatedStringData := make(map[string]interface{})
        for key := range stringData {
            obfuscatedStringData[key] = "********"
        }
        secretObj["stringData"] = obfuscatedStringData
    }
}
```

#### 2. `gitea.go` - Change Detection Function

```go
func detectSecretChanges(existing, new map[string]interface{}) bool {
    // Compare secret type
    if getStringField(existing, "type") != getStringField(new, "type") {
        return true
    }
    
    // Compare data keys
    existingDataKeys := getSecretDataKeys(existing, "data")
    newDataKeys := getSecretDataKeys(new, "data")
    if !stringSlicesEqual(existingDataKeys, newDataKeys) {
        return true
    }
    
    // Compare stringData keys
    existingStringDataKeys := getSecretDataKeys(existing, "stringData")
    newStringDataKeys := getSecretDataKeys(new, "stringData")
    if !stringSlicesEqual(existingStringDataKeys, newStringDataKeys) {
        return true
    }
    
    // Compare metadata (labels, annotations)
    if hasMetadataChanged(existing, new) {
        return true
    }
    
    return false
}
```

#### 3. `gitea.go` - Integration in handleCreateOrUpdate

```go
// Detect if this is a Secret resource
isSecret := req.Kind.Kind == "Secret"

// Obfuscate secret data before committing
if isSecret {
    if objMap, ok := obj.(map[string]interface{}); ok {
        obfuscateSecretData(objMap)
        if metricsCollector != nil {
            metricsCollector.IncrementObfuscatedSecrets()
        }
    }
}
```

#### 4. `gitea.go` - Enhanced hasFileChanged

```go
func (c *gitClient) hasFileChanged(filePath string, newContent []byte, isSecret bool) (bool, error) {
    // ... file reading logic ...
    
    // For secrets, use special comparison logic
    if isSecret {
        return c.hasSecretChanged(existingContent, newContent)
    }
    
    // For non-secrets, use normal YAML comparison
    // ... normal comparison logic ...
}
```

#### 5. `metrics.go` - New Metrics

```go
type MetricsCollector struct {
    // ... existing fields ...
    obfuscatedSecretsTotal   uint64
    secretChangesDetected    uint64
    // ...
}

func (m *MetricsCollector) IncrementObfuscatedSecrets() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.obfuscatedSecretsTotal++
}

func (m *MetricsCollector) IncrementSecretChangesDetected() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.secretChangesDetected++
}
```

## Architecture Flow

```
┌─────────────────────────────┐
│   kubectl create secret     │
│   kubectl patch secret      │
└──────────┬──────────────────┘
           │
           ▼
┌──────────────────────────────┐
│  Kubernetes API Server       │
└──────────┬───────────────────┘
           │
           ▼
┌──────────────────────────────┐
│  Admission Webhook           │
│  (intercept request)         │
└──────────┬───────────────────┘
           │
           ▼
┌──────────────────────────────┐
│  GitOps Admission Controller │
│                              │
│  1. Clean resource (neat)    │
│  2. Detect if Secret         │
│  3. Obfuscate data values    │
│  4. Compare with existing    │
│  5. Detect changes (keys)    │
│  6. Commit to Git            │
│  7. Update metrics           │
└──────────┬───────────────────┘
           │
           ▼
┌──────────────────────────────┐
│   Git Repository             │
│                              │
│   cluster/                   │
│     namespace/               │
│       secret/                │
│         my-secret.yaml       │
│         (obfuscated)         │
└──────────────────────────────┘
```

## Security Considerations

### What is Protected
- ✅ Secret values are never stored in Git in plain text
- ✅ Secret values are never stored in Git in base64 (which is easily decoded)
- ✅ Git history does not contain sensitive data
- ✅ Keys are preserved for audit purposes

### What is Not Protected
- ⚠️ Secret metadata (names, labels, annotations) are still visible
- ⚠️ The existence of a secret is tracked
- ⚠️ The number and names of keys in the secret are visible
- ⚠️ Value changes cannot be detected if keys remain the same

### Best Practices
1. **Use External Secret Management**: For highly sensitive environments, consider using:
   - External Secrets Operator
   - Sealed Secrets
   - HashiCorp Vault
   - Cloud provider secret managers (AWS Secrets Manager, Azure Key Vault, GCP Secret Manager)

2. **Restrict Git Repository Access**: Ensure the Git repository has appropriate access controls

3. **Audit Git Access**: Monitor who accesses the Git repository

4. **Rotate Secrets Regularly**: Even obfuscated, change detection may reveal patterns

## Comparison with Other Tools

### ArgoCD
- **ArgoCD UI**: Shows secret keys but hides values with `********`
- **GitOps Reversed Admission**: Stores secrets in Git with keys visible but values obfuscated

### Sealed Secrets
- **Sealed Secrets**: Encrypts secrets, stores encrypted values in Git
- **GitOps Reversed Admission**: Obfuscates secrets, stores `********` in Git
- **Trade-off**: Sealed Secrets allows GitOps workflows, our approach prioritizes audit trail

### kubectl-neat
- **kubectl-neat**: Removes runtime metadata
- **GitOps Reversed Admission**: Uses kubectl-neat functionality + secret obfuscation

## Configuration

No additional configuration is required. Secret obfuscation is automatically applied to all Secret resources.

To disable secret tracking entirely, exclude Secrets from the watch configuration:

```yaml
watch:
  resources:
    exclude:
      - Secret
```

## Monitoring

### Prometheus Metrics

Query examples:
```promql
# Total secrets obfuscated
gitops_admission_obfuscated_secrets_total

# Rate of secret obfuscation (per minute)
rate(gitops_admission_obfuscated_secrets_total[1m])

# Total secret changes detected
gitops_admission_secret_changes_detected_total

# Percentage of secret operations with detected changes
gitops_admission_secret_changes_detected_total / gitops_admission_obfuscated_secrets_total * 100
```

### Grafana Dashboard

Create a panel to track secret operations:
```
Panel: Secret Operations
Query: sum(rate(gitops_admission_obfuscated_secrets_total[5m]))
Legend: Secrets per second
```

## Limitations

1. **Value Changes Not Detected**: If secret keys remain the same but values change, this is not detected due to obfuscation
2. **No Encryption**: Values are obfuscated (replaced with `********`), not encrypted
3. **Key Names Visible**: The names of secret keys are visible in Git
4. **Type Visibility**: The secret type (Opaque, TLS, etc.) is visible

## Future Enhancements

Potential improvements for future milestones:

1. **Optional Encryption**: Instead of `********`, use encrypted values
2. **Key Hashing**: Optionally hash key names for additional privacy
3. **Value Change Detection**: Use hash comparison to detect value changes without exposing values
4. **Configurable Obfuscation**: Allow different obfuscation strategies per namespace
5. **Secret Rotation Tracking**: Track when secret values were last changed based on update timestamps

## Related Documentation

- [Milestone 6 Testing Guide](MILESTONE_6_TESTING.md)
- [ROADMAP.md](ROADMAP.md)
- [README.md](../README.md)
