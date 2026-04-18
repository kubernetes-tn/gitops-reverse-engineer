# Changelog

All notable changes to the GitOps Reverse Engineer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.10.0] - 2026-04-17

### Added - Milestone 10: End-to-End Test Framework
- **E2E test suite** with build tag `e2e` covering resource sync, secret obfuscation, namespace exclusion, and webhook endpoints
- **Kind + Gitea stack** for lightweight local and CI test environments
- **Setup/teardown scripts**: `e2e/setup.sh` (bootstrap) and `e2e/teardown.sh` (cleanup)
- **GitHub Actions workflow**: `.github/workflows/e2e.yaml` with failure log collection
- **Makefile targets**: `e2e-setup`, `e2e-test`, `e2e-teardown`
- **k8s.io/client-go** dependency for Kubernetes API interaction in tests

## [0.9.0] - 2026-04-17

### Added - Milestone 9: Multi-Provider Git Support & Open-Source Readiness
- **Multi-provider Git support**: Abstracted `GitClient` interface supporting Gitea, GitHub, and GitLab
- **Provider-aware authentication**: `authCredentials()` method with per-provider auth (`oauth2` for Gitea/GitLab, `x-access-token` for GitHub)
- **New environment variables**: `GIT_TOKEN` (with `GITEA_TOKEN` backward compatibility), `GIT_PROVIDER` (`gitea`, `github`, `gitlab`)
- **Helm chart provider config**: New `git.provider` value for selecting the Git backend
- **GitHub Actions CI/CD**: `build.yaml` (test, build, helm lint) and `release.yaml` (multi-arch build, GHCR push, helm package, GitHub release)
- **Project renamed**: `gitops-reversed-admission` → `gitops-reverse-engineer`
- **Go module**: `github.com/kubernetes-tn/gitops-reverse-engineer`
- **Container registry**: `ghcr.io/kubernetes-tn`

### Changed
- `gitea.go` renamed to `git_ops.go`, `GiteaClient` → unexported `gitClient` implementing `GitClient` interface
- `gitea_test.go` renamed to `git_client_test.go`
- `GITEA_TOKEN` env var replaced by `GIT_TOKEN` (backward compat preserved)
- Makefile unified to single `REGISTRY` variable (was `PUSH_REGISTRY`/`PULL_REGISTRY`)
- Helm deployment template uses `GIT_TOKEN` and `GIT_PROVIDER` env vars

## [0.5.0] - 2025-11-15

### Added - Milestone 5
- **User/ServiceAccount Exclusion**: Added ability to exclude specific users or service accounts from being tracked
  - New `watch.excludeUsers` configuration field
  - Filters requests based on `UserInfo.Username` from admission request
  - Useful for excluding system controllers, GitOps tools, and automation accounts
- **Namespace Pattern Matching**: Added glob-style pattern matching for namespace filtering
  - New `watch.namespaces.includePattern` configuration field
  - New `watch.namespaces.excludePattern` configuration field
  - Supports wildcards: `*` (any sequence), `?` (single char), `[abc]` (character class)
  - Examples: `kube-*`, `*-temp`, `prod-*`, `app-[0-9]`
- Comprehensive documentation for Milestone 5 features
  - Implementation guide
  - Testing guide
  - Configuration examples
  - Pattern matching reference

### Changed
- Enhanced `shouldWatchNamespace()` to support pattern matching
- Updated `ShouldProcessRequest()` to check user exclusion early
- Updated Helm chart values with new configuration options
- Updated ConfigMap template to include new fields
- Updated README with examples and feature documentation

### Technical Details
- Added `filepath` import to config.go for pattern matching
- Implemented `shouldExcludeUser()` method
- Implemented `matchPattern()` helper function
- Modified namespace filtering logic to handle patterns

## [0.4.1] - 2025-11-15

### Added - Milestone 4.B
- **Non-Fast-Forward Recovery**: Automatic handling of Git conflicts
  - Auto-detects non-fast-forward pull errors
  - Implements force pull with fetch + hard reset
  - Works with both `main` and `master` branches
  - Prevents operations from getting stuck in retry queue
- Added `nonFastForwardTotal` metric to track occurrences

### Changed
- Enhanced `pullLatest()` with automatic error detection
- Added `forcePull()` method for conflict resolution

## [0.4.0] - 2025-11-15

### Added - Milestone 4
- **Optimized Git Commits**: Skip unnecessary commits for unchanged resources
  - Added `hasFileChanged()` method to compare YAML content
  - Added `normalizeYAML()` helper for consistent formatting
  - Modified `handleCreateOrUpdate()` to check changes before committing
  - Added `skippedCommitsTotal` metric for observability

### Changed
- Git commits now only occur when resource definitions actually change
- Improved Git history clarity by eliminating redundant commits

## [0.3.0] - 2025-11-14

### Added - Milestone 3
- **kubectl-neat functionality**: Clean YAML commits without runtime metadata
  - Removes uid, timestamp, status, and other runtime fields
  - Commits only desired state to Git
- **Dynamic Git authorship**: Preserves Kubernetes user/serviceaccount information
  - Git author reflects actual user making changes
  - Service account names formatted as `{serviceaccountReplacingColonsByDashs}`
  - User names formatted as `{userReplacingColonsByDashs}`
- **Helm chart deployment**: Complete Helm chart with Bitnami-style structure
  - values.yaml with comprehensive configuration options
  - ConfigMap-based configuration
  - TLS certificate management
  - ServiceMonitor and PrometheusRule support
- **Cluster-wide resource support**: Optional tracking of cluster-scoped resources
  - Configurable via `watch.clusterWideResources`
  - Separate Git path structure: `{cluster}/_cluster/{resource-kind}/`
  - Supports Namespace, PV, StorageClass, ClusterRole, etc.
- **Resource filtering**: Include/exclude specific resource kinds
  - `watch.resources.include` configuration
  - `watch.resources.exclude` configuration
- **Namespace filtering**: Include/exclude specific namespaces
  - `watch.namespaces.include` configuration
  - `watch.namespaces.exclude` configuration
  - `watch.namespaces.labelSelector` support
- **Prometheus metrics**: Observability and monitoring
  - `gitops_admission_git_sync_success_total` counter
  - `gitops_admission_git_sync_failures_total` counter
  - `gitops_admission_pending_operations` gauge
- **ServiceMonitor**: Auto-configured Prometheus scraping
- **PrometheusRule**: Pre-configured alerting rules
  - GitOpsAdmissionGitSyncFailure alert

### Changed
- Deployment method changed from raw YAML to Helm chart
- Configuration moved to ConfigMap from environment variables
- Git commit messages now include actual user information

## [0.2.0] - 2025-11-14

### Added - Milestone 2
- **Git integration**: Automatic sync to Git repository
  - Support for Gitea, GitHub, GitLab
  - Configurable Git repository URL via `GIT_REPO_URL`
  - Authentication via `GIT_TOKEN`
- **Resource lifecycle tracking**: CREATE, UPDATE, DELETE operations
  - Commits created resources to Git
  - Updates existing files in Git
  - Removes deleted resources from Git
- **Git repository structure**: Organized path structure
  - Namespaced resources: `{cluster}/{namespace}/{resource-kind}/{resource-name}.yaml`
  - Pattern-based organization
- **Fault tolerance**: Retry mechanism for Git failures
  - Pending queue for failed operations
  - Automatic retry on temporary failures
  - Metrics for pending operations
- **Cluster name configuration**: Configurable via `CLUSTER_NAME`

### Changed
- Controller now actively syncs to Git instead of just printing events
- Added pending operations queue for reliability

## [0.1.0] - 2025-11-13

### Added - Milestone 1
- **Basic admission controller**: Validating webhook implementation
  - Intercepts CREATE, UPDATE, DELETE operations
  - Prints detailed event information
  - TLS-enabled webhook server
- **Resource information logging**: Detailed admission request logging
  - Operation type (CREATE/UPDATE/DELETE)
  - Resource kind and name
  - Namespace
  - User information
  - Labels and annotations
- **Health check endpoint**: `/health` for readiness/liveness probes
- **TLS certificate support**: Required for webhook communication
- **Kubernetes deployment**: YAML manifests for deployment
  - Deployment
  - Service
  - ValidatingWebhookConfiguration
  - RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)

### Technical Implementation
- Go-based implementation
- Uses Kubernetes admission/v1 API
- Docker containerization
- Certificate generation script

---

## Legend

- **Added**: New features
- **Changed**: Changes to existing functionality
- **Deprecated**: Soon-to-be removed features
- **Removed**: Removed features
- **Fixed**: Bug fixes
- **Security**: Security improvements


## [Unreleased] - 2025-11-15

This release focuses on improving usability, deployment flexibility, and OpenShift compatibility.

### ✨ Features & Enhancements

*   **Makefile Workflow**: All core operations are now managed through the `Makefile`. This provides simple, consistent commands for building, pushing, deploying, and testing (`make build`, `make push`, `make deploy`, `make test`).

*   **On-Demand Namespace Monitoring**: The admission controller no longer watches all namespaces by default. It now only monitors namespaces that are explicitly labeled with `gitops-reverse-engineer/enabled: "true"`. This makes the controller safer and more efficient.

*   **Configurable Registry**: The `Makefile` now supports a configurable `REGISTRY` variable for Docker image builds and deployments.

*   **Targeted Testing**: The `make test` command can now target a specific namespace using the `TEST_NAMESPACE` variable (e.g., `make test TEST_NAMESPACE=my-app-ns`).

*   **Descriptive Naming**: The default namespace for the controller has been renamed from `admission-system` to `verbose-api-log-system` to better describe its purpose.

*   **OpenShift Compatibility**: The `Dockerfile` has been updated to support OpenShift's security model by setting appropriate file permissions and running the container as a non-root user.
