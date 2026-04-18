# Milestone 1

- Admission controller just print the event details
- main role will be hello world admission controller.
- 
# Milestone 2
- admission controller is refactored as following:
- main role will be gitops-reserved-admission. Accordingly, naming need to be refactored everywhere
- any k8s resource created, the controller will push it to git repo under folder path following naming convention : {clustername}/{namespace}/{resourcekind}/{resourcename}.yaml 
- any k8s resource deleted, the controller will delete its yaml definition from the git repo/path.
- any k8s resources updated, the controller will push the updated one to the git repo/path.
- git server is primarely gitea. but default https://git.example.com ,but it need to be given to admission controller as envvar .. same for GIT_TOKEN for authenticated to api calls.
- if git server is unreachable at time of push, the admission controller must be fault tolerant. but also need to find solution how to not lose auditing this change whether create/update/delete. 

# Milestone 3

**A. Git Enhancements**

1. the admission controller should only commit the desired yaml ( removing uid, timestamp, status,..etc) exactly like kubectl-neat functionality.

2. the git author of the commit should not be "GitOps Reversed Admission" but it should be inherited from the kubernetes user/serviceaccount which is doing the change.
   git author naming can follow this pattern :
       - if serviceaccount - {serviceaccountReplacingColonsByDashs}
       - if user - {userReplacingColonsByDashs} 

**B. Configuration & Parameterization Enhancements**
1. the way of deployment of the controller should be done thru helm chart instead of hard-coded yamls , so the parameters can pass as values during any new rollout. (think about Bitnami helm chart values structure : global, metrics,..etc).

2. the [Makefile](../Makefile) deploy script should be updated with helm. (helm template than kubectl apply... and not helm upgrade)
      
3. the admission controller should be configured optionally to manage also cluster-wide resources like (Namespace, PV,..etc), and to commit it in the right git path. By default, it should not manage cluster wide resources unless .Values.watchClusterWideResources is true
   

3. the admission controller and its helm chart should be able to be configured with include/exclude watchNamespaces. By default it watch all. it can be include/exclude by namespace labels.
4. the admission controller and its helm chart should be able to be configured with include/exclude resources. By default, the common resources is included like (Deployment, Statefulset, PVC,...etc). But enduser can include/exclude more/less.

**C. Monitoring and Alerting**
1. the admission controller and its helm chart should be able to be configured with monitoring: if the controller was not able to push to gitea after specific retry, its prometheus exporter should mention that. the helm chart should create servicemonitor and required alert prometheusrule  with warning severity if .Values.metrics.enabled is true 


# Milestone 4 ✅ COMPLETED

- Optimize the git pushes: if no change in the file (desired yaml of the resource), the admission controller should not push unnecessary git commits.

## Implementation Details

**Status**: ✅ Completed

**Changes Made**:
1. Added `hasFileChanged()` method to compare YAML content before writing
2. Added `normalizeYAML()` helper to handle YAML formatting differences
3. Modified `handleCreateOrUpdate()` to check for changes before committing
4. Added `skippedCommitsTotal` metric to track optimization effectiveness
5. Updated Prometheus metrics endpoint to export skipped commits counter

**Benefits**:
- Eliminates unnecessary Git commits for unchanged resources
- Reduces repository bloat and improves Git history clarity
- Provides observability through metrics
- Maintains fault tolerance (defaults to committing if comparison fails)

**Documentation**:
- [Implementation Guide](MILESTONE_4_IMPLEMENTATION.md)
- [Testing Guide](MILESTONE_4_TESTING.md)

**Files Modified**:
- `gitea.go`: Added comparison logic and normalization
- `metrics.go`: Added skipped commits tracking

# Milestone 4.B ✅ COMPLETED

- Fix admission controller failed to pull non-fast-forward

## Problem

The admission controller was experiencing frequent failures with non-fast-forward pull errors, causing operations to get stuck in the retry queue:

```sh
kubectl logs -n gitops-reverse-engineer-system -l app.kubernetes.io/instance=gitops-reverse-engineer --tail=100
queue (queue size: 473)
2025/11/15 10:25:07 ⚠️ Warning: Failed to sync to 
Git (operation queued for retry): failed to pull: 
non-fast-forward update
2025/11/15 10:25:07 ✅ Allowed UPDATE operation on 
Secret/pull-secret in namespace multicluster-engine
2025/11/15 10:25:07 ⚠️ Failed to pull latest chang
es: failed to pull: non-fast-forward update       
2025/11/15 10:25:07 ⏳ Added operation to pending q
ueue (queue size: 474)
2025/11/15 10:25:07 ⚠️ Warning: Failed to sync to 
Git (operation queued for retry): failed to pull: 
non-fast-forward update
```

## Solution Implemented

**Status**: ✅ Completed

**Changes Made**:
1. Enhanced `pullLatest()` to detect non-fast-forward errors automatically
2. Implemented `forcePull()` method that performs fetch + hard reset to remote
3. Added automatic branch detection for both `main` and `master` branches
4. Added `nonFastForwardTotal` metric to track occurrences
5. Maintained existing fault tolerance and retry mechanisms

**Benefits**:
- Automatic recovery from non-fast-forward errors without manual intervention
- Prevents operations from getting stuck in retry queue indefinitely
- Provides observability through Prometheus metrics
- Works seamlessly with multiple controller replicas
- Remote repository remains the authoritative source of truth

**Documentation**:
- [Implementation Guide](MILESTONE_4B_IMPLEMENTATION.md)
- [Testing Guide](MILESTONE_4B_TESTING.md)

**Files Modified**:
- `gitea.go`: Added `forcePull()` and enhanced `pullLatest()` with error detection
- `metrics.go`: Added non-fast-forward tracking metric 

# Milestone 5 ✅ COMPLETED

1. add Capability to exclude watching some user/serviceaccount . If the event of creation/update came from specific user/serviceaccount, the admission controller can exclude it. 

2. add capability to include/exclude by namespace name pattern : like exclude "kube-*" will exclude events of "kube-system", "kube-dns",...etc

## Implementation Details

**Status**: ✅ Completed

**Changes Made**:
1. Added `excludeUsers` field to `WatchConfig` for filtering specific users/serviceaccounts
2. Added `includePattern` and `excludePattern` fields to `NamespaceFilter` for glob-style matching
3. Implemented `shouldExcludeUser()` method to check user exclusion
4. Enhanced `shouldWatchNamespace()` to support pattern matching using `filepath.Match`
5. Added `matchPattern()` helper function for glob pattern matching
6. Updated Helm chart values.yaml with new configuration options
7. Updated ConfigMap template to pass new configuration to controller
8. Added comprehensive documentation and testing guides

**Benefits**:
- Filter out system controllers (garbage collector, volume binder, etc.)
- Exclude GitOps tools (ArgoCD, Flux) to prevent recursive changes
- Flexible namespace filtering with patterns instead of maintaining long lists
- Reduced noise in Git audit trail
- Better performance by skipping excluded requests early

**Pattern Examples**:
- `"kube-*"` - Excludes kube-system, kube-public, kube-node-lease
- `"*-temp"` - Excludes dev-temp, test-temp, staging-temp
- `"prod-*"` - Includes prod-app, prod-db, prod-api
- `"app-?"` - Includes app-a, app-1 (single character after app-)

**Documentation**:
- [Implementation Guide](MILESTONE_5_IMPLEMENTATION.md)
- [Testing Guide](MILESTONE_5_TESTING.md)

**Files Modified**:
- `config.go`: Added user exclusion and pattern matching logic
- `chart/values.yaml`: Added excludeUsers, includePattern, excludePattern
- `chart/templates/configmap.yaml`: Updated to include new configuration fields
- `README.md`: Updated with examples and documentation


# Milestone 6 - Hardening ✅ COMPLETED

1. obfuscate data of kind secret before pushing to git. something like argocd how to show secret data in UI.
2. Detect changes of data of kind secret even obfuscated.

## Implementation Details

**Status**: ✅ Completed

**Changes Made**:
1. Added `obfuscateSecretData()` function to replace secret values with `********`
2. Added `detectSecretChanges()` function to compare obfuscated secrets
3. Integrated obfuscation into `handleCreateOrUpdate()` for Secret resources
4. Enhanced `hasFileChanged()` to use special logic for secrets
5. Added `obfuscatedSecretsTotal` and `secretChangesDetected` metrics
6. Updated Prometheus metrics endpoint to export secret-specific counters

**Benefits**:
- Secret values never stored in Git (replaced with `********`)
- Keys and metadata visible for audit purposes
- Changes detected when keys are added/removed or metadata changes
- Value-only changes not detected (security trade-off)
- Provides observability through Prometheus metrics

**Change Detection**:
- ✅ New keys added to data/stringData
- ✅ Keys removed from data/stringData
- ✅ Secret type changed
- ✅ Labels or annotations changed
- ❌ Values changed (not detectable when obfuscated - expected limitation)

**Documentation**:
- [Implementation Guide](MILESTONE_6_IMPLEMENTATION.md)
- [Testing Guide](MILESTONE_6_TESTING.md)

**Files Modified**:
- `gitea.go`: Added obfuscation and secret change detection logic
- `metrics.go`: Added secret-specific metrics tracking

**Security Considerations**:
- Secret values are obfuscated (not encrypted)
- Key names remain visible
- Metadata (labels, annotations) remain visible
- Suitable for audit trail, not for secret storage/distribution
- Consider using External Secrets Operator or Sealed Secrets for production secret management

 

# Milestone 7 - onboard more resources ✅ COMPLETED
1. helm chart [values.yaml](../chart/values.yaml) to be defaulted with more resources to be INCLUDED like: Openshift route, Networkpolicy, Gateway API resources (HttpRoute,..etc), ArgoCD resources (app, applicationset, appproject,..etc)

## Implementation Details

**Status**: ✅ Completed

**Changes Made**:
1. Updated `chart/values.yaml` to include comprehensive default resource list
2. Added Core Workloads: ReplicaSet, Job, CronJob
3. Added Gateway API resources: HTTPRoute, Gateway, GatewayClass, TCPRoute, TLSRoute, UDPRoute
4. Added OpenShift resources: Route
5. Added ArgoCD resources: Application, ApplicationSet, AppProject
6. Added RBAC resources: Role, RoleBinding, ClusterRole, ClusterRoleBinding, ServiceAccount
7. Added Resource management: ResourceQuota, LimitRange, HorizontalPodAutoscaler
8. Added NetworkPolicy for network security policies
9. Added PersistentVolume for cluster-wide storage resources
10. Updated README.md with enhanced resource list and documentation

**Default Resources Included** (37 resource types):

**Core Workloads:**
- Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob

**Services & Networking:**
- Service, Ingress, NetworkPolicy

**Gateway API Resources:**
- HTTPRoute, Gateway, GatewayClass, TCPRoute, TLSRoute, UDPRoute

**OpenShift Routes:**
- Route

**Configuration & Storage:**
- ConfigMap, Secret, PersistentVolumeClaim, PersistentVolume

**ArgoCD Resources:**
- Application, ApplicationSet, AppProject

**RBAC:**
- Role, RoleBinding, ClusterRole, ClusterRoleBinding, ServiceAccount

**Resource Quotas & Limits:**
- ResourceQuota, LimitRange

**Autoscaling:**
- HorizontalPodAutoscaler

**Benefits**:
- Comprehensive coverage of common Kubernetes resources out-of-the-box
- Support for modern networking with Gateway API
- OpenShift compatibility with Route resources
- GitOps workflow tracking with ArgoCD resources
- RBAC and security policy tracking
- Resource management and quota tracking
- Users can still customize via include/exclude lists

**Files Modified**:
- `chart/values.yaml`: Expanded default resource list from 8 to 37 resources
- `README.md`: Updated documentation with complete resource list and examples

# Milestone 8 - Multi-Cluster at Scale ✅ COMPLETED

1. the deployment to cluster xyz, should you values/values.xyz.yaml by convention. That help to give every cluster its own values in declarative way overriding values.yaml. 

2. helm chart [../chart](../chart/) should support injecting hostAliases for the deployment of the admission controller

## Implementation Details

**Status**: ✅ Completed

**Changes Made**:
1. Enhanced Makefile to automatically detect and use `values/values.{CLUSTER_NAME}.yaml` files
2. Added `hostAliases` parameter to Helm chart values.yaml with documentation
3. Updated deployment template to conditionally include hostAliases in pod spec
4. Created cluster-specific values files for example clusters:
   - `values.dev.yaml`, `values.staging.yaml`, `values.production.yaml`
5. Updated help text in Makefile to document multi-cluster convention

**Benefits**:
- Declarative per-cluster configuration without script modifications
- Automatic values file detection based on CLUSTER_NAME
- Support for custom DNS resolution via hostAliases
- Easy multi-cluster management at scale
- Version-controlled cluster differences

**Documentation**:
- [Implementation Guide](MILESTONE_8_IMPLEMENTATION.md)
- [Testing Guide](MILESTONE_8_TESTING.md)
- [Values Directory README](../values/README.md)

**Files Modified**:
- `Makefile`: Added automatic cluster-specific values detection
- `chart/values.yaml`: Added hostAliases parameter
- `chart/templates/deployment.yaml`: Added hostAliases support
- `values/values.{cluster}.yaml`: Created 8 cluster-specific values files
- `README.md`: Updated with multi-cluster deployment documentation

## Implementation Details

**Status**: ✅ Completed

**Changes Made**:
1. Enhanced Makefile `deploy` target to automatically detect and use `values/values.$(CLUSTER_NAME).yaml`
2. Added `hostAliases` parameter to `chart/values.yaml` with comprehensive documentation
3. Added `hostAliases` support to `chart/templates/deployment.yaml` pod spec
4. Created example cluster-specific values files:
   - `values/values.production.yaml` - Production cluster configuration
   - `values/values.staging.yaml` - Staging cluster configuration
   - `values/values.dev.yaml` - Development cluster configuration
   - `values/values.platform.yaml` - Platform cluster configuration
5. Updated Makefile help text to document multi-cluster deployment convention
6. Added informative deployment messages showing which values files are being used

**Benefits**:
- Declarative per-cluster configuration in version control
- No deployment script modifications needed for different clusters
- Support for air-gapped and custom DNS environments via hostAliases
- Automatic values file detection during deployment
- Clean separation of cluster-specific concerns
- Easy multi-cluster management at scale

**Documentation**:
- [Implementation Guide](MILESTONE_8_IMPLEMENTATION.md)
- [Testing Guide](MILESTONE_8_TESTING.md)

**Files Modified**:
- `Makefile`: Enhanced deploy target with automatic values file detection
- `chart/values.yaml`: Added hostAliases parameter
- `chart/templates/deployment.yaml`: Added hostAliases to pod spec
- `README.md`: Updated with multi-cluster deployment documentation

**Files Created**:
- `values/values.production.yaml`: Production cluster configuration example
- `values/values.staging.yaml`: Staging cluster configuration example
- `values/values.dev.yaml`: Development cluster configuration example
- `values/values.platform.yaml`: Platform cluster configuration
- `docs/MILESTONE_8_IMPLEMENTATION.md`: Comprehensive implementation guide
- `docs/MILESTONE_8_TESTING.md`: Testing scenarios and validation procedures

**Usage**:
```bash
# Deploy to production using values/values.production.yaml
make deploy CLUSTER_NAME=production TAG=v1.0.0

# Deploy to staging using values/values.staging.yaml
make deploy CLUSTER_NAME=staging TAG=v1.0.0

# Deploy to dev using values/values.dev.yaml
make deploy CLUSTER_NAME=dev TAG=v1.0.0
```

**Host Aliases Example**:
```yaml
# In values/values.{cluster}.yaml
hostAliases:
  - ip: "192.168.x.x"
    hostnames:
      - "gitea.internal.local"
      - "git.internal.local"
```

# Milestone 9 - Multi-Provider Git Support & Open-Source Readiness ✅ COMPLETED

1. Abstract the Gitea-specific Git client into a provider-agnostic interface supporting Gitea, GitHub, and GitLab.
2. Rename the project from `gitops-reversed-admission` to `gitops-reverse-engineer` and publish to `github.com/kubernetes-tn/gitops-reverse-engineer`.
3. Add GitHub Actions CI/CD workflows (build + release).

## Implementation Details

**Status**: ✅ Completed

**Changes Made**:
1. Created `GitClient` interface with `ProcessRequest()` and `GetPendingCount()` methods
2. Introduced `GitProvider` type with constants: `ProviderGitea`, `ProviderGitHub`, `ProviderGitLab`
3. Added `GitClientConfig` struct and `NewGitClient()` factory constructor
4. Renamed `GiteaClient` struct to unexported `gitClient` implementing the `GitClient` interface
5. Added `authCredentials()` method with provider-specific HTTP basic auth:
   - Gitea/GitLab: `username="oauth2"`
   - GitHub: `username="x-access-token"`
6. New `GIT_TOKEN` env var (with `GITEA_TOKEN` backward compatibility)
7. New `GIT_PROVIDER` env var (`"gitea"`, `"github"`, `"gitlab"`)
8. Added `git.provider` field to Helm chart `values.yaml`
9. Updated Helm deployment template with `GIT_TOKEN` and `GIT_PROVIDER` env vars
10. Project renamed: Go module → `github.com/kubernetes-tn/gitops-reverse-engineer`
11. Container registry → `ghcr.io/kubernetes-tn`
12. Added GitHub Actions workflows: `build.yaml` (CI) and `release.yaml` (tag-based release)

**Architecture**:
```
git_client.go     → Interface definition (GitClient, GitProvider, GitClientConfig)
git_ops.go        → Implementation (gitClient struct, all Git operations)
main.go           → Uses GitClient interface, reads GIT_PROVIDER env var
```

**Provider Authentication**:
| Provider | Auth Username     | Token Type             |
|----------|-------------------|------------------------|
| Gitea    | `oauth2`          | Personal access token  |
| GitHub   | `x-access-token`  | PAT or fine-grained    |
| GitLab   | `oauth2`          | Personal/project token |

**Environment Variables**:
| Variable       | Description                                      | Default  |
|----------------|--------------------------------------------------|----------|
| `GIT_PROVIDER` | Git hosting provider (`gitea`, `github`, `gitlab`) | `gitea`  |
| `GIT_TOKEN`    | Authentication token for the Git provider          | -        |
| `GITEA_TOKEN`  | Legacy alias for `GIT_TOKEN` (backward compat)    | -        |
| `GIT_REPO_URL` | Repository URL for syncing resources               | -        |

**Benefits**:
- Support for the three most popular Git hosting platforms
- Single codebase with provider-specific auth handled transparently
- Backward compatible with existing Gitea deployments (`GITEA_TOKEN` still works)
- Clean Go interface design — no OOP, no unnecessary abstractions
- Open-source ready with GitHub container registry and CI/CD

**Files Created**:
- `git_client.go`: Interface, provider type, config struct, factory constructor
- `.github/workflows/build.yaml`: CI workflow (test, build, helm lint)
- `.github/workflows/release.yaml`: Release workflow (test, build+push, helm package, GitHub release)

**Files Renamed**:
- `gitea.go` → `git_ops.go`: Implementation with provider abstraction
- `gitea_test.go` → `git_client_test.go`: Tests

**Files Modified**:
- `main.go`: Uses `GitClient` interface, reads `GIT_PROVIDER` and `GIT_TOKEN`
- `go.mod`: Module path → `github.com/kubernetes-tn/gitops-reverse-engineer`
- `Makefile`: Single `REGISTRY` variable, defaults to `ghcr.io/kubernetes-tn`
- `chart/values.yaml`: Added `git.provider` field
- `chart/templates/deployment.yaml`: `GIT_TOKEN` and `GIT_PROVIDER` env vars
- `chart/Chart.yaml`: Updated URLs to GitHub
- `k8s/deployment.yaml`: Updated env vars
- All documentation files: Updated references

# Milestone 10 - End-to-End Test Framework ✅ COMPLETED

1. Implement a full e2e test suite that bootstraps a Kind cluster with Gitea, deploys the webhook controller, and validates the complete admission → git sync pipeline.
2. Tests run locally (`make e2e-test`) and in CI (GitHub Actions).

## Implementation Details

**Status**: ✅ Completed

**Architecture**:
```
                  ┌──────────────────────────────────────┐
                  │         Kind Cluster                  │
                  │                                      │
                  │  ┌──────────┐    ┌────────────────┐  │
                  │  │  Gitea   │←───│ Webhook Ctrl   │  │
                  │  │ (gitea/) │    │ (system ns)    │  │
                  │  └──────────┘    └────────┬───────┘  │
                  │       ↑                   │          │
                  │       │          ValidatingWebhook   │
                  │       │                   │          │
                  │  ┌────┴───────────────────┴───────┐  │
                  │  │      e2e-test-ns               │  │
                  │  │  ConfigMap, Secret, Deployment  │  │
                  │  └────────────────────────────────┘  │
                  └──────────────────────────────────────┘
                           ↕ (NodePort 30080)
                  ┌──────────────────┐
                  │  Go e2e tests    │
                  │  (Gitea API +    │
                  │   k8s client-go) │
                  └──────────────────┘
```

**Test Scenarios**:
1. **TestCreateConfigMap** — Create a ConfigMap, verify it appears in Gitea with correct content, runtime fields cleaned
2. **TestUpdateConfigMap** — Create then update a ConfigMap, verify the updated content reaches Git
3. **TestDeleteConfigMap** — Create, wait for sync, delete, verify file removed from Git
4. **TestSecretObfuscation** — Create a Secret with sensitive data, verify values are `********` in Git but keys preserved
5. **TestDeploymentSync** — Create a Deployment, verify YAML synced with status removed (kubectl-neat)
6. **TestServiceSync** — Create a Service, verify clusterIP cleaned from committed YAML
7. **TestExcludedNamespace** — Create in `kube-system`, verify it's NOT synced (namespace exclusion)
8. **TestWebhookHealthEndpoint** — In-cluster Job hits `/health`, expects `OK`
9. **TestWebhookMetricsEndpoint** — In-cluster Job hits `/metrics`, expects Prometheus metrics

**Stack**:
- **Kind** v0.23+ — Lightweight local Kubernetes cluster (CI and local)
- **Gitea** 1.21 (rootless) — In-cluster Git server with SQLite backend
- **Go test** with `-tags=e2e` build tag — Isolated from unit tests
- **k8s.io/client-go** — Kubernetes API interaction from tests
- **Gitea REST API** — Verify file contents committed to Git

**Environment Variables** (written to `e2e/.env` by setup):
| Variable | Description | Default |
|----------|-------------|---------|
| `E2E_GITEA_URL` | Gitea HTTP URL (via NodePort) | `http://localhost:30080` |
| `E2E_GITEA_TOKEN` | Gitea API token | (generated) |
| `E2E_GITEA_ORG` | Gitea organisation name | `e2e-org` |
| `E2E_GITEA_REPO` | Gitea repository name | `gitops-e2e` |
| `E2E_CLUSTER_NAME` | Cluster name for git path | `e2e-test` |
| `E2E_NAMESPACE` | Namespace for test resources | `e2e-test-ns` |
| `E2E_SKIP_KIND` | Skip Kind (use existing cluster) | `false` |

**Benefits**:
- Validates the complete pipeline: admission request → resource cleanup → git commit → push
- Catches regressions in secret obfuscation, namespace filtering, and neat cleanup
- Runs identically locally and in CI (no external dependencies)
- Lightweight stack (Kind + Gitea with SQLite) — no external database or heavy infra
- Supports existing clusters via `E2E_SKIP_KIND=true` (Docker Desktop, minikube, etc.)
- Failure log collection in CI (webhook logs, gitea logs, k8s events)

**Files Created**:
- `e2e/e2e_test.go`: All e2e test cases (build tag `e2e`)
- `e2e/setup.sh`: Bootstrap script (Kind + Gitea + certs + webhook deploy)
- `e2e/teardown.sh`: Cleanup script
- `e2e/kind-config.yaml`: Kind cluster configuration (NodePort mapping)
- `e2e/manifests/gitea.yaml`: Gitea Deployment + Service + PVC manifests
- `.github/workflows/e2e.yaml`: GitHub Actions e2e workflow

**Files Modified**:
- `go.mod`: Added `k8s.io/client-go` dependency

# Milestone 11 - CI/CD Multi-Platform Build & Release ✅ COMPLETED

Harden GitHub Actions CI/CD pipelines to produce multi-architecture container images (linux/amd64 + linux/arm64), publish the Helm chart to an OCI registry, and fix correctness/efficiency issues across all workflows.

## Implementation Details

**Status**: ✅ Completed

**Problems Fixed**:

1. **Dockerfile hardcoded `GOOS=linux`** — no `GOARCH`, so Buildx multi-platform builds produced amd64-only binaries for both architectures. Fixed with `TARGETOS`/`TARGETARCH` build args.

2. **build.yaml single-platform only** — used `load: true` (incompatible with multi-platform), no QEMU, no concurrency. Fixed: QEMU + Buildx for `linux/amd64,linux/arm64`, concurrency group.

3. **release.yaml missing QEMU** — `platforms: linux/amd64,linux/arm64` silently broken for arm64. Also: no concurrency, `Chart.yaml` not updated before Helm packaging, `generate_release_notes` duplicated custom body. All fixed.

4. **Helm chart not published** — end users had to clone the repo to install. Added `helm push` to GHCR OCI on every release: `oci://ghcr.io/kubernetes-tn/charts/gitops-reverse-engineer`.

5. **e2e.yaml hardcoded Kind version** — already fixed in Milestone 10 follow-up.

**Files Modified**:
- `Dockerfile`: Added `TARGETOS`/`TARGETARCH` build args
- `.github/workflows/build.yaml`: QEMU, multi-platform build, concurrency, removed `load: true`
- `.github/workflows/release.yaml`: QEMU, concurrency, Chart.yaml sed, removed `generate_release_notes`, added Helm OCI push
- `README.md`: Added OCI-based install as primary Quick Start, moved build-from-source to collapsible section

**Documentation**:
- [Implementation Guide](MILESTONE_11_IMPLEMENTATION.md)

# Milestone 12 - GitHub Community & Repository Maturity ✅ COMPLETED

Make the `.github/` directory follow open-source best practices with issue templates, PR templates, security policy, code ownership, Copilot instructions, and automated issue hygiene.

## Implementation Details

**Status**: ✅ Completed

**Files Created**:
- `.github/ISSUE_TEMPLATE/bug_report.yml`: Structured bug report form with version, K8s version, Git provider, install method, logs, and Helm values fields
- `.github/ISSUE_TEMPLATE/feature_request.yml`: Feature request form with problem/solution/alternatives and area dropdown
- `.github/ISSUE_TEMPLATE/config.yml`: Disables blank issues, redirects questions to GitHub Discussions
- `.github/PULL_REQUEST_TEMPLATE.md`: PR template with What/Why/How sections, verification checklist, and quality gates
- `.github/SECURITY.md`: Security policy with private reporting via GitHub Security Advisories, 72h SLA, scope definition
- `.github/CODEOWNERS`: Auto-assigns reviewers by path (chart/, workflows/, e2e/)
- `.github/copilot-instructions.md`: Project conventions for GitHub Copilot — architecture, code structure, testing, rules
- `.github/workflows/stale.yaml`: Marks issues/PRs stale after 60 days, closes after 14 more, exempts `bug`/`security`/`pinned` labels
- `Makefile`: Added `e2e-setup`, `e2e-test`, `e2e-teardown` targets