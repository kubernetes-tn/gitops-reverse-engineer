# Milestone 11 — CI/CD Multi-Platform Build & Release

**Status**: ✅ Completed

## Overview

Milestone 11 hardens the GitHub Actions CI/CD pipelines to produce multi-architecture container images (linux/amd64 + linux/arm64), publishes the Helm chart to an OCI registry for easy installation, and fixes several correctness and efficiency issues across all three workflows.

## Problems Identified

### Dockerfile — Single Architecture Only

The existing Dockerfile hardcoded `GOOS=linux` with no `GOARCH` argument. When Docker Buildx builds for `linux/arm64`, it still compiled an amd64 binary — producing a broken image for ARM nodes.

**Fix**: Use Buildx `TARGETPLATFORM` / `TARGETOS` / `TARGETARCH` build args (injected automatically by `docker buildx build`) so each platform leg compiles the correct binary.

### build.yaml — No Multi-Platform Validation

| Issue | Impact |
|---|---|
| No `concurrency` group | Back-to-back pushes waste CI minutes running redundant jobs |
| `load: true` on Docker build | Incompatible with multi-platform — only works for single-platform images |
| No QEMU setup | Cannot cross-compile for arm64 on amd64 runners |
| Single-platform build | arm64 regressions go undetected until release time |

**Fix**: Add concurrency, QEMU, and build both platforms (without push). Use `push: false` without `load: true` for multi-platform validation.

### release.yaml — Broken Helm Packaging & Redundant Notes

| Issue | Impact |
|---|---|
| No `concurrency` group | Concurrent tag pushes could produce conflicting releases |
| No QEMU setup | `platforms: linux/amd64,linux/arm64` silently fails or produces broken arm64 images |
| Helm package uses `--version` flag only | `Chart.yaml` version is stale — some tools read it directly |
| `generate_release_notes: true` + custom `body` | Duplicated release notes (GitHub-generated + custom) |
| Helm chart not published to any registry | End users must clone the repo to install |

**Fix**: Add concurrency, QEMU, sed Chart.yaml version before packaging, remove `generate_release_notes`, add `helm push` to GHCR OCI.

## Changes

### 1. Dockerfile — Multi-Architecture Support

```dockerfile
# Before
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o admission-controller .

# After — uses Buildx automatic TARGETOS/TARGETARCH injection
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -installsuffix cgo -o admission-controller .
```

### 2. build.yaml — Multi-Platform CI Validation

- Added `concurrency: group: ci-${{ github.ref }}, cancel-in-progress: true`
- Added `docker/setup-qemu-action@v3` for arm64 emulation
- Changed Docker build to `platforms: linux/amd64,linux/arm64`, removed `load: true`
- Removed `Verify image runs` step (incompatible with multi-platform, and the build itself validates compilation)

### 3. release.yaml — Correct Multi-Platform Release

- Added `concurrency: group: release-${{ github.ref }}, cancel-in-progress: false` (never cancel a release mid-flight)
- Added `docker/setup-qemu-action@v3` for arm64 cross-build
- Helm packaging now updates `Chart.yaml` version/appVersion via `sed` before `helm package`
- Removed `generate_release_notes: true` to avoid duplicating the custom release body
- Added `helm push` to GHCR OCI registry (`oci://ghcr.io/kubernetes-tn/charts`)
- Updated release notes template to reference OCI install instead of `./chart`

### 4. README — OCI-First Install

- New "Install from Helm OCI Registry" section as primary Quick Start
- `helm install ... oci://ghcr.io/kubernetes-tn/charts/gitops-reverse-engineer`
- Moved build-from-source instructions into collapsible `<details>` section

### 5. e2e.yaml — Already Fixed (Milestone 10 follow-up)

- Added `concurrency: group: e2e-${{ github.ref }}, cancel-in-progress: true`
- Replaced hardcoded Kind binary download with `go install sigs.k8s.io/kind@latest`
- Uses `make` targets instead of duplicating shell logic

## Verification

```bash
# Validate Dockerfile multi-arch locally
docker buildx build --platform linux/amd64,linux/arm64 -t test:multi .

# Validate workflow YAML syntax
act -l                          # list detected jobs
yamllint .github/workflows/     # lint YAML structure
```

## Platform Support Matrix

| Platform | CI (build.yaml) | Release (release.yaml) | E2E (e2e.yaml) |
|---|---|---|---|
| linux/amd64 | ✅ Built | ✅ Built & pushed | ✅ Tested (Kind) |
| linux/arm64 | ✅ Built | ✅ Built & pushed | — (no arm64 runners) |
