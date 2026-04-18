# Milestone 2 Design Document
## GitOps Reversed Admission Controller

### Overview
This document outlines the design and implementation of Milestone 2 for the GitOps Reversed Admission Controller, which transforms the hello-world admission controller into a production-ready GitOps auditing system.

---

## 1. System Architecture

### 1.1 High-Level Components

```
┌──────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
│                                                              │
│  ┌─────────────┐      ┌──────────────────────────────┐     │
│  │   K8s API   │─────▶│  Validating Webhook Config   │     │
│  │   Server    │      └──────────────────────────────┘     │
│  └─────────────┘                    │                        │
│         │                            │                        │
│         │                            ▼                        │
│         │           ┌────────────────────────────────┐       │
│         │           │  GitOps Reversed Admission     │       │
│         │           │       Controller Pod           │       │
│         │           │                                │       │
│         │           │  ┌──────────────────────────┐  │       │
│         │           │  │   Main Handler           │  │       │
│         │           │  │  - Admission Review      │  │       │
│         │           │  │  - Request Processing    │  │       │
│         │           │  └──────────┬───────────────┘  │       │
│         │           │             │                  │       │
│         │           │  ┌──────────▼───────────────┐  │       │
│         │           │  │   Gitea Client           │  │       │
│         │           │  │  - Git Operations        │  │       │
│         │           │  │  - Fault Tolerance       │  │       │
│         │           │  │  - Retry Mechanism       │  │       │
│         │           │  │  - Pending Queue         │  │       │
│         │           │  └──────────┬───────────────┘  │       │
│         │           └─────────────┼──────────────────┘       │
│         │                         │                           │
└─────────┼─────────────────────────┼───────────────────────────┘
          │                         │
          │                         ▼
          │           ┌─────────────────────────────┐
          │           │    Git Server              │
          │           │  (git.example.com)          │
          │           │                             │
          │           │  Git Repository:            │
          │           │  cluster-state/             │
          │           │    ├── production-cluster/  │
          │           │    │   ├── default/         │
          │           │    │   │   ├── deployment/  │
          │           │    │   │   ├── service/     │
          │           │    │   │   └── configmap/   │
          │           │    │   └── kube-system/     │
          │           │    └── staging-cluster/     │
          │           └─────────────────────────────┘
          │
          ▼
   ┌──────────────┐
   │  Resource    │
   │  Operations  │
   └──────────────┘
```

### 1.2 Data Flow

**CREATE/UPDATE Flow:**
```
1. User creates/updates resource → kubectl
2. API Server receives request
3. API Server calls ValidatingWebhook
4. Admission Controller receives AdmissionReview
5. Controller logs resource info
6. Controller pulls latest from Git (if available)
7. Controller serializes resource to YAML
8. Controller writes to {cluster}/{namespace}/{kind}/{name}.yaml
9. Controller commits with message "{Operation}: {Kind} {Namespace}/{Name}"
10. Controller pushes to Git
    - Success → Operation complete
    - Failure → Add to pending queue
11. Controller returns AdmissionResponse{Allowed: true}
12. API Server applies the resource
```

**DELETE Flow:**
```
1. User deletes resource → kubectl
2. API Server receives delete request
3. API Server calls ValidatingWebhook
4. Admission Controller receives AdmissionReview
5. Controller logs resource info
6. Controller pulls latest from Git (if available)
7. Controller removes {cluster}/{namespace}/{kind}/{name}.yaml
8. Controller commits with message "DELETE: {Kind} {Namespace}/{Name}"
9. Controller pushes to Git
    - Success → Operation complete
    - Failure → Add to pending queue
10. Controller returns AdmissionResponse{Allowed: true}
11. API Server deletes the resource
```

---

## 2. Component Design

### 2.1 Main Controller (`main.go`)

**Responsibilities:**
- Initialize HTTPS server with TLS certificates
- Handle admission review requests
- Coordinate with Git client
- Provide health endpoint
- Manage environment configuration

**Key Functions:**
```go
main()                      // Initialize and start server
handleAdmission()           // Process admission reviews
printResourceInfo()         // Log resource details
healthCheck()              // Health endpoint with pending count
```

**Environment Variables:**
- `GIT_REPO_URL`: Full URL to Git repository
- `GIT_TOKEN`: Authentication token (from K8s Secret)
- `CLUSTER_NAME`: Cluster identifier for path structure

### 2.2 Gitea Client (`gitea.go`)

**Responsibilities:**
- Manage Git repository operations
- Handle CREATE/UPDATE/DELETE operations
- Implement fault tolerance
- Maintain pending operations queue
- Automatic retry mechanism

**Key Structures:**
```go
type gitClient struct {
    repoURL       string              // Git repository URL
    token         string              // Authentication token
    repo          *git.Repository     // Git repository handle
    worktree      *git.Worktree       // Working tree
    repoPath      string              // Local repo path
    clusterName   string              // Cluster identifier
    pendingOps    []PendingOperation  // Queue for failed ops
    mu            sync.Mutex          // Thread safety
    retryInterval time.Duration       // Retry frequency (30s)
    maxRetries    int                 // Max retry attempts (10)
}

type PendingOperation struct {
    Request      *admissionv1.AdmissionRequest
    ResourcePath string
    Operation    admissionv1.Operation
    Timestamp    time.Time
    RetryCount   int
}
```

**Key Functions:**
```go
NewgitClient()              // Initialize client with fault tolerance
initRepo()                    // Clone/initialize Git repository
ProcessRequest()              // Main entry point for operations
handleCreateOrUpdate()        // Handle CREATE/UPDATE operations
handleDelete()                // Handle DELETE operations
buildResourcePath()           // Build {cluster}/{ns}/{kind}/{name}.yaml
pullLatest()                  // Pull latest changes
commitAndPush()               // Commit and push to remote
addToPendingQueue()           // Queue failed operations
retryLoop()                   // Background retry goroutine
processPendingOperations()    // Process pending queue
GetPendingCount()             // Get queue size (for health check)
```

### 2.3 Path Structure

Resources are organized in Git following this convention:

```
{CLUSTER_NAME}/{NAMESPACE}/{RESOURCE_KIND}/{RESOURCE_NAME}.yaml
```

**Examples:**
- `production-cluster/default/deployment/nginx.yaml`
- `production-cluster/kube-system/configmap/cluster-info.yaml`
- `staging-cluster/app-ns/service/api-gateway.yaml`

**Benefits:**
- Easy navigation and discovery
- Clear organization by cluster, namespace, and type
- Supports multi-cluster management
- Git history tracks changes per resource

---

## 3. Fault Tolerance Design

### 3.1 Failure Scenarios

| Scenario | Handling Strategy |
|----------|------------------|
| Git server unreachable | Queue operation, retry every 30s |
| Network timeout | Queue operation, retry with backoff |
| Authentication failure | Log error, queue for retry |
| Repository not found | Log error, attempt re-initialization |
| Concurrent push conflicts | Pull, merge, retry push |
| Disk space issues | Log error, alert via health endpoint |

### 3.2 Retry Mechanism

**Configuration:**
- `retryInterval`: 30 seconds
- `maxRetries`: 10 attempts
- Total retry window: ~5 minutes

**Retry Logic:**
```
1. Operation fails
2. Add to pendingOps queue with timestamp
3. Background goroutine runs every 30s
4. Attempt to re-initialize repo (if needed)
5. Process each pending operation
6. Increment retry count
7. If successful: Remove from queue
8. If failed and count < max: Keep in queue
9. If failed and count >= max: Discard and log
```

### 3.3 Non-Blocking Operation

**Critical Design Principle:**
- Git failures **NEVER** block Kubernetes operations
- Controller always returns `Allowed: true`
- Failed operations are queued asynchronously
- Health endpoint exposes pending operation count

---

## 4. Security Design

### 4.1 Authentication

**Gitea Token:**
- Stored in Kubernetes Secret (`git-token`)
- Mounted as environment variable
- Used for all Git operations via OAuth2

**TLS Certificates:**
- Required for webhook HTTPS communication
- Stored in Kubernetes Secret
- Mounted at `/etc/webhook/certs/`

### 4.2 RBAC Considerations

**Controller Pod:**
- Runs as non-root user (UID 1001)
- Minimal filesystem access
- No special Kubernetes API permissions needed
- Read-only cert mount

**Git Repository:**
- Token requires `repo` scope
- Repository should have appropriate access controls
- Consider private repository for production

### 4.3 Sensitive Data

**Warning:**
- Secrets are synced to Git as-is
- YAML includes base64-encoded data
- Repository must be secured appropriately

**Recommendations:**
- Use private Git repositories
- Implement repository access controls
- Consider excluding Secrets from sync
- Use sealed-secrets or external-secrets for sensitive data

---

## 5. Performance Considerations

### 5.1 Optimization Strategies

**Git Operations:**
- Pull before each push to minimize conflicts
- Use shallow clones if history not needed
- Periodic repository cleanup (garbage collection)

**Memory Management:**
- Pending queue size monitoring
- Limit queue to prevent memory exhaustion
- Discard operations after max retries

**Concurrency:**
- Mutex protection for pending queue
- Single goroutine for retry processing
- Webhook handlers are concurrent-safe

### 5.2 Scalability

**Single Replica:**
- Current design: 1 replica
- Git operations are sequential
- Mutex prevents race conditions

**Multi-Replica (Future):**
- Would require distributed locking
- Consider leader election
- Shared pending queue (external storage)

---

## 6. Monitoring and Observability

### 6.1 Logging

**Log Levels:**
- `INFO`: Normal operations, successful syncs
- `WARNING`: Failures with retry, queue additions
- `ERROR`: Max retries exceeded, critical failures

**Key Log Messages:**
- `✅ Successfully synced X to Git`
- `⚠️ Failed to sync (operation queued for retry)`
- `🔄 Processing N pending operations...`
- `❌ Max retries reached for X, discarding`

### 6.2 Health Endpoint

**Endpoint:** `GET /health`

**Response:**
```
OK - Pending operations: 0
```

**Use Cases:**
- Kubernetes liveness/readiness probes
- External monitoring systems
- Manual health checks

### 6.3 Metrics (Future Enhancement)

Recommended metrics:
- `gitops_sync_total{operation, status}`
- `gitops_pending_operations_count`
- `gitops_retry_attempts_total`
- `gitops_operation_duration_seconds`

---

## 7. Configuration

### 7.1 Environment Variables

| Variable | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `GIT_REPO_URL` | string | Yes | - | Full Git repository URL |
| `GIT_TOKEN` | string | Yes | - | Gitea authentication token |
| `CLUSTER_NAME` | string | No | `default-cluster` | Cluster identifier for paths |

### 7.2 Webhook Configuration

**Monitored Resources:**
- ConfigMaps, Secrets, Pods, Services (core/v1)
- Deployments, StatefulSets, DaemonSets, ReplicaSets (apps/v1)
- Jobs, CronJobs (batch/v1)

**Operations:**
- CREATE, UPDATE, DELETE

**Namespace Filtering:**
- Only namespaces with label `gitops-reverse-engineer/enabled: "true"`

**Failure Policy:**
- `Ignore` - cluster continues if webhook unavailable

---

## 8. Deployment Architecture

### 8.1 Kubernetes Resources

```
namespace: verbose-api-log-system
├── Secret: git-token (GIT_TOKEN)
├── Secret: gitops-reverse-engineer-certs (TLS)
├── Deployment: gitops-reverse-engineer
│   └── Pod
│       └── Container: admission-controller
│           ├── Env: GIT_REPO_URL
│           ├── Env: GIT_TOKEN (from secret)
│           ├── Env: CLUSTER_NAME
│           └── Volume: webhook-certs
├── Service: gitops-reverse-engineer
└── ValidatingWebhookConfiguration: gitops-reverse-engineer-webhook
```

### 8.2 Resource Requirements

**Current Configuration:**
- Requests: 100m CPU, 256Mi memory
- Limits: 500m CPU, 512Mi memory

**Justification:**
- Git operations require memory for cloning
- YAML serialization overhead
- Pending queue storage

---

## 9. Future Enhancements

### 9.1 Short-term (Next Milestones)

1. **Metrics Endpoint**: Prometheus-compatible metrics
2. **Selective Sync**: Configure which resource types to sync
3. **Secret Filtering**: Option to exclude Secrets
4. **Compression**: Reduce Git repository size
5. **Branch Strategy**: Support multiple branches per cluster

### 9.2 Long-term Vision

1. **Multi-Cluster**: Centralized view of all clusters
2. **Reconciliation**: Compare Git vs actual state
3. **Policy Enforcement**: Validate against Git policies
4. **GitOps Integration**: Bi-directional sync with ArgoCD/Flux
5. **AI Analysis**: Detect anomalies in change patterns

---

## 10. Migration from Milestone 1

### 10.1 Breaking Changes

- Module name: `github.com/hello-world-admission` → `github.com/kubernetes-tn/gitops-reverse-engineer`
- Container name: `hello-world-admission` → `gitops-reverse-engineer`
- Kubernetes resources renamed
- New environment variables required

### 10.2 Migration Steps

1. Build new container image
2. Update deployment with new env vars
3. Create Gitea token and secret
4. Update webhook configuration
5. Deploy new version
6. Verify Git sync functionality
7. Clean up old resources

---

## 11. Testing Strategy

### 11.1 Unit Tests (Future)

- Git client initialization
- Path building logic
- Retry mechanism
- Queue management

### 11.2 Integration Tests

**Manual Testing:**
1. Create resource → Verify in Git
2. Update resource → Verify Git commit
3. Delete resource → Verify Git deletion
4. Simulate Git failure → Verify queue
5. Restore Git → Verify retry success

**Automated Testing:**
- End-to-end tests with Kind cluster
- Mock Gitea server for CI/CD

---

## 12. Rollback Plan

### 12.1 Rollback Procedure

If issues arise:
1. Delete ValidatingWebhookConfiguration
2. Revert to Milestone 1 deployment
3. Investigate and fix issues
4. Re-deploy when ready

### 12.2 Data Recovery

Git repository contains complete history:
- Use Git history to audit changes
- Restore deleted resources from Git
- Compare cluster state with Git state

---

## 13. Success Criteria

### 13.1 Functional Requirements

- ✅ All CREATE operations synced to Git
- ✅ All UPDATE operations synced to Git
- ✅ All DELETE operations remove from Git
- ✅ Correct path structure maintained
- ✅ Fault tolerance with retry mechanism
- ✅ Non-blocking operation (never denies)

### 13.2 Non-Functional Requirements

- ✅ Zero downtime during Git failures
- ✅ < 10ms latency impact on K8s operations
- ✅ Memory usage < 512Mi under normal load
- ✅ Pending queue handles 1000+ operations
- ✅ Automatic recovery from failures

---

## Conclusion

Milestone 2 successfully transforms the admission controller from a simple logging tool into a production-ready GitOps auditing system with:

- **Fault-tolerant Git operations**
- **Automatic retry mechanism**
- **Structured resource organization**
- **Non-blocking design**
- **Comprehensive monitoring**

The system provides a complete audit trail of all cluster operations while maintaining zero impact on cluster availability.
