# Milestone 5 Implementation Guide

## Overview

Milestone 5 adds advanced filtering capabilities to the GitOps Reversed Admission Controller:

1. **User/ServiceAccount Exclusion** - Filter out changes made by specific users or service accounts
2. **Namespace Pattern Matching** - Use glob-style patterns for namespace include/exclude rules

## Features

### 1. User/ServiceAccount Exclusion

The controller can now exclude changes made by specific users or service accounts. This is useful for:
- Filtering out system controllers (garbage collector, volume binder, etc.)
- Excluding GitOps tools (ArgoCD, Flux) from creating recursive changes
- Preventing automated processes from cluttering the audit trail

#### Configuration

```yaml
watch:
  excludeUsers:
    - "system:serviceaccount:kube-system:generic-garbage-collector"
    - "system:serviceaccount:argocd:argocd-application-controller"
    - "system:admin"
    - "ci-bot@example.com"
```

#### How It Works

When a resource is created, updated, or deleted, the controller checks the `req.UserInfo.Username` field from the admission request. If the username matches any entry in the `excludeUsers` list, the request is skipped and not synced to Git.

**Example Log Output:**
```
⏭️ Skipping request from user system:serviceaccount:kube-system:generic-garbage-collector - user is in exclude list
✅ Allowed UPDATE operation on Secret/pull-secret in namespace default
```

### 2. Namespace Pattern Matching

The controller now supports glob-style pattern matching for namespace filtering, allowing flexible rules without maintaining long lists of namespace names.

#### Configuration

```yaml
watch:
  namespaces:
    # Exact namespace names
    include: ["production", "staging"]
    exclude: ["default"]
    
    # Glob patterns
    includePattern: ["prod-*", "app-*"]
    excludePattern: ["kube-*", "*-temp", "test-*"]
```

#### Pattern Syntax

The implementation uses Go's `filepath.Match` which supports:

- `*` - matches any sequence of characters
- `?` - matches any single character  
- `[abc]` - matches any character in the brackets
- `[a-z]` - matches any character in the range

#### Pattern Examples

| Pattern | Matches | Doesn't Match |
|---------|---------|---------------|
| `kube-*` | `kube-system`, `kube-public`, `kube-node-lease` | `default`, `prod-kube` |
| `*-temp` | `dev-temp`, `test-temp`, `staging-temp` | `temporary`, `temp` |
| `prod-*` | `prod-app`, `prod-db`, `prod-api` | `production`, `preprod` |
| `app-?` | `app-a`, `app-1`, `app-x` | `app-12`, `app-` |
| `[pt]rod-*` | `prod-app`, `trod-db` | `dev-app` |

#### Filter Logic

The namespace filtering follows this priority order:

1. **Exact exclude list** - If namespace is in `exclude`, reject immediately
2. **Exclude patterns** - If namespace matches any `excludePattern`, reject
3. **Include list/patterns** - If empty, accept all (except excluded)
4. **Exact include list** - If namespace is in `include`, accept
5. **Include patterns** - If namespace matches any `includePattern`, accept
6. **Default** - Reject if include list/patterns exist but don't match

**Example Scenarios:**

```yaml
# Scenario 1: Exclude system namespaces only
namespaces:
  excludePattern: ["kube-*"]
  # Result: All namespaces except kube-system, kube-public, etc.

# Scenario 2: Only production environments
namespaces:
  includePattern: ["prod-*"]
  # Result: Only prod-app, prod-db, etc.

# Scenario 3: All except temporary and system
namespaces:
  excludePattern: ["kube-*", "*-temp", "test-*"]
  # Result: Everything except kube-*, *-temp, test-*

# Scenario 4: Specific apps, no temporary
namespaces:
  includePattern: ["app-*"]
  excludePattern: ["*-temp"]
  # Result: app-a, app-b, but NOT app-temp
```

## Implementation Details

### Code Changes

#### 1. Config Structure (`config.go`)

Added new fields to configuration structs:

```go
type WatchConfig struct {
    ClusterWideResources bool
    Namespaces           NamespaceFilter
    Resources            ResourceFilter
    ExcludeUsers         []string  // NEW
}

type NamespaceFilter struct {
    Include        []string
    Exclude        []string
    IncludePattern []string          // NEW
    ExcludePattern []string          // NEW
    LabelSelector  map[string]string
}
```

#### 2. User Exclusion Logic (`config.go`)

```go
func (c *Config) shouldExcludeUser(req *admissionv1.AdmissionRequest) bool {
    username := req.UserInfo.Username
    for _, excludedUser := range c.Watch.ExcludeUsers {
        if excludedUser == username {
            return true
        }
    }
    return false
}
```

#### 3. Pattern Matching Helper (`config.go`)

```go
func matchPattern(pattern, name string) bool {
    matched, err := filepath.Match(pattern, name)
    if err != nil {
        log.Printf("⚠️ Error matching pattern %s: %v", pattern, err)
        return false
    }
    return matched
}
```

#### 4. Enhanced Namespace Filtering (`config.go`)

```go
func (c *Config) shouldWatchNamespace(namespace string) bool {
    // Check exact exclude list
    for _, excluded := range c.Watch.Namespaces.Exclude {
        if excluded == namespace {
            return false
        }
    }

    // Check exclude patterns
    for _, pattern := range c.Watch.Namespaces.ExcludePattern {
        if matchPattern(pattern, namespace) {
            return false
        }
    }

    // If include list/patterns empty, watch all (except excluded)
    if len(c.Watch.Namespaces.Include) == 0 && 
       len(c.Watch.Namespaces.IncludePattern) == 0 {
        return true
    }

    // Check exact include list
    for _, included := range c.Watch.Namespaces.Include {
        if included == namespace {
            return true
        }
    }

    // Check include patterns
    for _, pattern := range c.Watch.Namespaces.IncludePattern {
        if matchPattern(pattern, namespace) {
            return true
        }
    }

    return false
}
```

#### 5. Request Processing (`config.go`)

Updated `ShouldProcessRequest` to check user exclusion:

```go
func (c *Config) ShouldProcessRequest(req *admissionv1.AdmissionRequest) bool {
    // Check user exclusion first
    if c.shouldExcludeUser(req) {
        log.Printf("⏭️ Skipping request from user %s - user is in exclude list", 
                   req.UserInfo.Username)
        return false
    }
    
    // ... rest of filtering logic
}
```

### Helm Chart Updates

#### values.yaml

```yaml
watch:
  namespaces:
    include: []
    exclude: []
    includePattern: []
    excludePattern:
      - "kube-*"  # Default: exclude system namespaces
    labelSelector: {}
  excludeUsers: []
```

#### configmap.yaml Template

```yaml
data:
  config.yaml: |
    watch:
      namespaces:
        include: {{ toJson .Values.watch.namespaces.include }}
        exclude: {{ toJson .Values.watch.namespaces.exclude }}
        includePattern: {{ toJson .Values.watch.namespaces.includePattern }}
        excludePattern: {{ toJson .Values.watch.namespaces.excludePattern }}
        labelSelector: {{ toJson .Values.watch.namespaces.labelSelector }}
      excludeUsers: {{ toJson .Values.watch.excludeUsers }}
```

## Usage Examples

### Example 1: Exclude System Components

```yaml
watch:
  namespaces:
    excludePattern: ["kube-*"]
  excludeUsers:
    - "system:serviceaccount:kube-system:generic-garbage-collector"
    - "system:serviceaccount:kube-system:persistent-volume-binder"
```

### Example 2: Only Track Production

```yaml
watch:
  namespaces:
    includePattern: ["prod-*"]
  excludeUsers:
    - "system:serviceaccount:argocd:argocd-application-controller"
```

### Example 3: Mixed Configuration

```yaml
watch:
  namespaces:
    include: ["default"]  # Exact namespace
    includePattern: ["app-*", "service-*"]  # Pattern-based
    excludePattern: ["*-temp"]  # Exclude temporary
  excludeUsers:
    - "ci-deploy-bot"
    - "system:admin"
```

## Testing

### Test User Exclusion

1. Create a test deployment as a specific user:
```bash
kubectl create deployment test --image=nginx --as=ci-bot
```

2. Configure exclusion:
```yaml
watch:
  excludeUsers:
    - "ci-bot"
```

3. Verify the change is not synced to Git

### Test Namespace Patterns

1. Create test namespaces:
```bash
kubectl create ns kube-test
kubectl create ns prod-app
kubectl create ns dev-temp
```

2. Configure patterns:
```yaml
watch:
  namespaces:
    includePattern: ["prod-*"]
    excludePattern: ["kube-*", "*-temp"]
```

3. Create resources in each namespace and verify:
   - `prod-app`: Should be synced ✅
   - `kube-test`: Should be skipped (matches `kube-*`) ⏭️
   - `dev-temp`: Should be skipped (matches `*-temp`) ⏭️

### Verify Logs

Check the controller logs to see filtering in action:

```bash
kubectl logs -n gitops-reverse-engineer-system \
  -l app.kubernetes.io/name=gitops-reverse-engineer \
  --tail=50
```

Expected log patterns:
```
⏭️ Skipping request from user system:serviceaccount:kube-system:generic-garbage-collector - user is in exclude list
⏭️ Skipping Deployment in namespace kube-test - namespace not in watch list
✅ Allowed CREATE operation on Deployment/nginx in namespace prod-app
```

## Benefits

1. **Reduced noise** - Filter out system controllers and automated tools
2. **Flexible namespace rules** - Use patterns instead of maintaining long lists
3. **Better audit trail** - Only track meaningful human/application changes
4. **Performance** - Skip processing for excluded users early in the pipeline
5. **GitOps compatibility** - Prevent recursive loops with ArgoCD/Flux

## Next Steps

See [ROADMAP.md](ROADMAP.md) for upcoming features in Milestone 6:
- Secret obfuscation
- Additional resource types (OpenShift Route, Gateway API, ArgoCD resources)
