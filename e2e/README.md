# End-to-End Tests

This directory contains the e2e test suite for **gitops-reverse-engineer**. The tests spin up a [Kind](https://kind.sigs.k8s.io/) cluster with a local Gitea instance and verify the full admission-webhook flow.

## Prerequisites

| Tool | Minimum version | Install |
|------|----------------|---------|
| **Docker** | 20+ | [docs.docker.com/get-docker](https://docs.docker.com/get-docker/) |
| **kubectl** | 1.25+ | [kubernetes.io/docs/tasks/tools](https://kubernetes.io/docs/tasks/tools/) |
| **Helm** | 3.x | [helm.sh/docs/intro/install](https://helm.sh/docs/intro/install/) |
| **Kind** | 0.20+ | See below |
| **Go** | 1.21+ | [go.dev/dl](https://go.dev/dl/) |

### Installing Kind

```bash
# macOS (Homebrew)
brew install kind

# macOS / Linux (Go install)
go install sigs.k8s.io/kind@latest

# Linux (binary)
curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

Verify the installation:

```bash
kind version
```

## Quick Start

```bash
# 1. Set up the environment (creates Kind cluster + Gitea + webhook)
make e2e-setup

# 2. Run the tests
make e2e-test

# 3. Tear down
make e2e-teardown
```

## What `e2e-setup` Does

1. Creates a Kind cluster (`gitops-e2e`) using `e2e/kind-config.yaml`
2. Deploys Gitea (rootless, SQLite) into the `gitea` namespace
3. Creates an admin user, organisation, and repository in Gitea
4. Builds the webhook Docker image and loads it into the Kind nodes
5. Generates TLS certificates and deploys the webhook via Helm
6. Creates a test namespace and writes an `.env` file for the test runner

## Configuration

All settings can be overridden with environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `E2E_KIND_CLUSTER` | `gitops-e2e` | Kind cluster name |
| `E2E_KIND_NODE_IMAGE` | `kindest/node:v1.35.1` | Kind node image (pinned for caching) |
| `E2E_SKIP_KIND` | `false` | Skip Kind cluster creation (use existing kubeconfig) |
| `E2E_GITEA_IMAGE` | `gitea/gitea:1.21-rootless` | Gitea container image |
| `E2E_GITEA_ADMIN_USER` | `e2e-admin` | Gitea admin username |
| `E2E_GITEA_ADMIN_PASS` | `e2e-admin-pass` | Gitea admin password |
| `E2E_GITEA_ORG` | `e2e-org` | Gitea organisation name |
| `E2E_GITEA_REPO` | `gitops-e2e` | Gitea repository name |
| `E2E_CLUSTER_NAME` | `e2e-test` | Cluster name written to Git commits |
| `E2E_GITEA_LOCAL_PORT` | `3000` | Local port for Gitea port-forward |

### Image Pull Sizes

First run pulls ~425 MB (compressed). Subsequent runs reuse cached layers.

| Image | Compressed | Purpose |
|-------|-----------|---------|
| `kindest/node:v1.35.1` | ~361 MB | Kubernetes node for Kind |
| `gitea/gitea:1.21-rootless` | ~62 MB | Git server |

> **Tip:** Pre-pull images to speed up setup:
> ```bash
> docker pull kindest/node:v1.35.1
> docker pull gitea/gitea:1.21-rootless
> ```

## Troubleshooting

**`kind is required (or set E2E_SKIP_KIND=true)`** — Install Kind (see above). The `E2E_SKIP_KIND=true` escape hatch is for CI environments that provide their own cluster, but Kind is the recommended path.

**`ErrImageNeverPull`** — The image wasn't loaded into the cluster nodes. Re-run `make e2e-setup`; the script loads the image via `kind load docker-image`.

**Port 3000 already in use** — Another process is using port 3000. Either stop it or set `E2E_GITEA_LOCAL_PORT=3001`.
