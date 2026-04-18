#!/usr/bin/env bash
#
# e2e/setup.sh — Bootstrap a Kind cluster with Gitea and the webhook controller
# for end-to-end testing.
#
# Usage:
#   ./e2e/setup.sh                      # full setup (kind + gitea + webhook)
#   E2E_SKIP_KIND=true ./e2e/setup.sh   # skip kind, use current kubeconfig
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Configuration (override via environment)
# ---------------------------------------------------------------------------
KIND_CLUSTER_NAME="${E2E_KIND_CLUSTER:-gitops-e2e}"
KIND_NODE_IMAGE="${E2E_KIND_NODE_IMAGE:-kindest/node:v1.35.1}"
SKIP_KIND="${E2E_SKIP_KIND:-false}"
GITEA_IMAGE="${E2E_GITEA_IMAGE:-gitea/gitea:1.21-rootless}"
GITEA_ADMIN_USER="${E2E_GITEA_ADMIN_USER:-e2e-admin}"
GITEA_ADMIN_PASS="${E2E_GITEA_ADMIN_PASS:-e2e-admin-pass}"
GITEA_ADMIN_EMAIL="${E2E_GITEA_ADMIN_EMAIL:-admin@e2e.local}"
GITEA_ORG="${E2E_GITEA_ORG:-e2e-org}"
GITEA_REPO="${E2E_GITEA_REPO:-gitops-e2e}"
CLUSTER_NAME="${E2E_CLUSTER_NAME:-e2e-test}"
WEBHOOK_NAMESPACE="gitops-reverse-engineer-system"
IMAGE_NAME="gitops-reverse-engineer"
IMAGE_TAG="e2e"
CURL_IMAGE="${E2E_CURL_IMAGE:-curlimages/curl:8.5.0}"
GITEA_LOCAL_PORT="${E2E_GITEA_LOCAL_PORT:-3000}"
PORTFWD_PIDFILE="$SCRIPT_DIR/.portforward.pid"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "▸ $*"; }
ok()    { echo "✅ $*"; }
fail()  { echo "❌ $*" >&2; exit 1; }

wait_for_rollout() {
  local ns="$1" deploy="$2" timeout="${3:-120s}"
  kubectl rollout status "deployment/$deploy" -n "$ns" --timeout="$timeout"
}

wait_for_url() {
  local url="$1" timeout="${2:-90}"
  local end=$((SECONDS + timeout))
  while (( SECONDS < end )); do
    if curl -sf "$url" >/dev/null 2>&1; then return 0; fi
    sleep 2
  done
  fail "Timed out waiting for $url"
}

gitea_api() {
  local method="$1" path="$2"; shift 2
  curl -sf -X "$method" \
    -H "Content-Type: application/json" \
    -u "${GITEA_ADMIN_USER}:${GITEA_ADMIN_PASS}" \
    "http://localhost:${GITEA_LOCAL_PORT}/api/v1${path}" \
    "$@"
}

# ---------------------------------------------------------------------------
# Prerequisites check
# ---------------------------------------------------------------------------
info "Checking prerequisites..."
for cmd in docker kubectl helm; do
  command -v "$cmd" >/dev/null || fail "$cmd is required but not found"
done
if [[ "$SKIP_KIND" != "true" ]]; then
  command -v kind >/dev/null || fail "kind is required (or set E2E_SKIP_KIND=true)"
fi
ok "Prerequisites satisfied"

# ---------------------------------------------------------------------------
# 1. Kind cluster
# ---------------------------------------------------------------------------
if [[ "$SKIP_KIND" == "true" ]]; then
  info "Skipping Kind cluster creation (E2E_SKIP_KIND=true)"
else
  if kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
    info "Kind cluster '${KIND_CLUSTER_NAME}' already exists, reusing"
  else
    # Pre-pull the node image only if not already cached locally.
    if docker image inspect "$KIND_NODE_IMAGE" >/dev/null 2>&1; then
      info "Kind node image '$KIND_NODE_IMAGE' found locally — skipping pull"
    else
      info "Pulling Kind node image '$KIND_NODE_IMAGE'..."
      docker pull "$KIND_NODE_IMAGE"
    fi
    info "Creating Kind cluster '${KIND_CLUSTER_NAME}'..."
    kind create cluster --name "$KIND_CLUSTER_NAME" \
      --config "$SCRIPT_DIR/kind-config.yaml" \
      --image "$KIND_NODE_IMAGE" \
      --wait 60s
  fi
  kubectl cluster-info --context "kind-${KIND_CLUSTER_NAME}" >/dev/null 2>&1 \
    || fail "Cannot reach Kind cluster"
  ok "Kind cluster ready"
fi

# ---------------------------------------------------------------------------
# 2. Deploy Gitea
# ---------------------------------------------------------------------------

# Ensure the Gitea image exists locally; pull only if missing.
if docker image inspect "$GITEA_IMAGE" >/dev/null 2>&1; then
  info "Gitea image '$GITEA_IMAGE' found locally — skipping pull"
else
  info "Pulling Gitea image '$GITEA_IMAGE'..."
  # Pull single-platform only; multi-arch manifests break kind load.
  docker pull --platform "linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" "$GITEA_IMAGE"
fi

# Pre-load the image into cluster nodes so kubelet never needs to pull.
# NOTE: We use docker save | ctr import instead of kind load docker-image
# because kind load fails on multi-arch images from registries.
if [[ "$SKIP_KIND" != "true" ]]; then
  for node in $(kind get nodes --name "$KIND_CLUSTER_NAME"); do
    info "Loading Gitea image into node $node..."
    docker save "$GITEA_IMAGE" | docker exec -i "$node" ctr -n k8s.io images import -
  done
else
  for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
    info "Loading Gitea image into node $node..."
    docker save "$GITEA_IMAGE" | docker exec -i "$node" ctr -n k8s.io images import -
  done
fi

info "Deploying Gitea..."
sed "s|image: gitea/gitea:.*|image: ${GITEA_IMAGE}|" "$SCRIPT_DIR/manifests/gitea.yaml" \
  | kubectl apply -f -
wait_for_rollout gitea gitea 120s
ok "Gitea deployment ready"

# Kill any leftover port-forward from a previous run
if [[ -f "$PORTFWD_PIDFILE" ]]; then
  kill "$(cat "$PORTFWD_PIDFILE")" 2>/dev/null || true
  rm -f "$PORTFWD_PIDFILE"
fi

info "Starting port-forward to Gitea (localhost:${GITEA_LOCAL_PORT} → gitea:3000)..."
kubectl port-forward svc/gitea -n gitea "${GITEA_LOCAL_PORT}:3000" >/dev/null 2>&1 &
echo $! > "$PORTFWD_PIDFILE"
sleep 2

info "Waiting for Gitea HTTP to respond..."
wait_for_url "http://localhost:${GITEA_LOCAL_PORT}/api/v1/version" 90
ok "Gitea API reachable"

# ---------------------------------------------------------------------------
# 3. Create Gitea admin user + org + repo
# ---------------------------------------------------------------------------
info "Creating Gitea admin user..."
# Use kubectl exec to create the admin user inside the pod (avoids API chicken-and-egg).
GITEA_POD=$(kubectl get pod -n gitea -l app=gitea -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n gitea "$GITEA_POD" -- \
  gitea admin user create \
    --username "$GITEA_ADMIN_USER" \
    --password "$GITEA_ADMIN_PASS" \
    --email "$GITEA_ADMIN_EMAIL" \
    --admin \
    --must-change-password=false 2>/dev/null || true   # ignore "already exists"
ok "Admin user ready"

info "Creating Gitea organisation '${GITEA_ORG}'..."
gitea_api POST "/orgs" -d "{\"username\":\"${GITEA_ORG}\",\"visibility\":\"public\"}" >/dev/null 2>&1 || true
ok "Organisation ready"

info "Creating Gitea repository '${GITEA_ORG}/${GITEA_REPO}'..."
gitea_api POST "/orgs/${GITEA_ORG}/repos" \
  -d "{\"name\":\"${GITEA_REPO}\",\"auto_init\":true,\"default_branch\":\"main\"}" >/dev/null 2>&1 || true
ok "Repository ready"

info "Creating Gitea access token..."
TOKEN_RESP=$(gitea_api POST "/users/${GITEA_ADMIN_USER}/tokens" \
  -d "{\"name\":\"e2e-$(date +%s)\",\"scopes\":[\"all\"]}" 2>/dev/null || true)
GITEA_TOKEN=$(echo "$TOKEN_RESP" | grep -o '"sha1":"[^"]*"' | head -1 | cut -d'"' -f4)
if [[ -z "$GITEA_TOKEN" ]]; then
  # Gitea 1.21+ uses "token" field instead of sha1
  GITEA_TOKEN=$(echo "$TOKEN_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
fi
[[ -n "$GITEA_TOKEN" ]] || fail "Failed to create Gitea access token"
ok "Access token created"

# ---------------------------------------------------------------------------
# 4. Build & load webhook image
# ---------------------------------------------------------------------------
info "Building webhook image ${IMAGE_NAME}:${IMAGE_TAG}..."
docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" "$ROOT_DIR" --quiet
ok "Image built"

if [[ "$SKIP_KIND" != "true" ]]; then
  info "Loading image into Kind..."
  kind load docker-image "${IMAGE_NAME}:${IMAGE_TAG}" --name "$KIND_CLUSTER_NAME"
  ok "Image loaded"
else
  # Docker Desktop uses containerd nodes that don't share the host Docker daemon.
  # Load the image into each node manually.
  for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
    info "Loading image into node $node..."
    docker save "${IMAGE_NAME}:${IMAGE_TAG}" | docker exec -i "$node" ctr -n k8s.io images import -
  done
  ok "Image loaded into cluster nodes"
fi

# Pre-load the curl helper image used by health/metrics e2e tests.
if docker image inspect "$CURL_IMAGE" >/dev/null 2>&1; then
  info "Curl image '$CURL_IMAGE' found locally — skipping pull"
else
  info "Pulling curl helper image '$CURL_IMAGE'..."
  docker pull --platform "linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" "$CURL_IMAGE"
fi
if [[ "$SKIP_KIND" != "true" ]]; then
  for node in $(kind get nodes --name "$KIND_CLUSTER_NAME"); do
    info "Loading curl image into node $node..."
    docker save "$CURL_IMAGE" | docker exec -i "$node" ctr -n k8s.io images import -
  done
else
  for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
    docker save "$CURL_IMAGE" | docker exec -i "$node" ctr -n k8s.io images import -
  done
fi
ok "Curl helper image loaded"

# ---------------------------------------------------------------------------
# 5. Generate TLS certificates for the webhook
# ---------------------------------------------------------------------------
info "Generating TLS certificates..."
RELEASE_NAME="gitops-reverse-engineer"
kubectl create namespace "$WEBHOOK_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

openssl genrsa -out "$TEMP_DIR/ca.key" 2048 2>/dev/null
openssl req -x509 -new -nodes -key "$TEMP_DIR/ca.key" \
  -subj "/CN=${RELEASE_NAME}-ca" -days 1 -out "$TEMP_DIR/ca.crt" 2>/dev/null

cat > "$TEMP_DIR/csr.conf" <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${RELEASE_NAME}
DNS.2 = ${RELEASE_NAME}.${WEBHOOK_NAMESPACE}
DNS.3 = ${RELEASE_NAME}.${WEBHOOK_NAMESPACE}.svc
DNS.4 = ${RELEASE_NAME}.${WEBHOOK_NAMESPACE}.svc.cluster.local
EOF

openssl genrsa -out "$TEMP_DIR/tls.key" 2048 2>/dev/null
openssl req -new -key "$TEMP_DIR/tls.key" \
  -subj "/CN=${RELEASE_NAME}.${WEBHOOK_NAMESPACE}.svc" \
  -config "$TEMP_DIR/csr.conf" -out "$TEMP_DIR/tls.csr" 2>/dev/null
openssl x509 -req -in "$TEMP_DIR/tls.csr" -CA "$TEMP_DIR/ca.crt" \
  -CAkey "$TEMP_DIR/ca.key" -CAcreateserial -out "$TEMP_DIR/tls.crt" \
  -days 1 -extensions v3_req -extfile "$TEMP_DIR/csr.conf" 2>/dev/null

kubectl create secret generic "${RELEASE_NAME}-certs" \
  --from-file=tls.key="$TEMP_DIR/tls.key" \
  --from-file=tls.crt="$TEMP_DIR/tls.crt" \
  --namespace "$WEBHOOK_NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

CA_BUNDLE=$(base64 < "$TEMP_DIR/ca.crt" | tr -d '\n')
ok "TLS certificates ready"

# ---------------------------------------------------------------------------
# 6. Deploy the webhook via Helm
# ---------------------------------------------------------------------------
GITEA_INTERNAL_URL="http://gitea.gitea.svc.cluster.local:3000/${GITEA_ORG}/${GITEA_REPO}.git"

info "Deploying webhook controller..."
helm template "$RELEASE_NAME" "$ROOT_DIR/chart" \
  --namespace "$WEBHOOK_NAMESPACE" \
  --set image.registry="" \
  --set image.repository="${IMAGE_NAME}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=Never \
  --set git.provider=gitea \
  --set git.repoUrl="$GITEA_INTERNAL_URL" \
  --set git.clusterName="$CLUSTER_NAME" \
  --set tls.existingSecret="${RELEASE_NAME}-certs" \
  --set "tls.crt=${CA_BUNDLE}" \
  --set webhook.failurePolicy=Ignore \
  --set "watch.namespaces.excludePattern[0]=kube-*" \
  --set "watch.namespaces.exclude[0]=gitea" \
  --set "watch.namespaces.exclude[1]=${WEBHOOK_NAMESPACE}" \
  --set metrics.serviceMonitor.enabled=false \
  --set prometheusRule.enabled=false \
  | kubectl apply -f -

# Create the git-token secret
kubectl create secret generic git-token \
  --from-literal=token="$GITEA_TOKEN" \
  --namespace "$WEBHOOK_NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

wait_for_rollout "$WEBHOOK_NAMESPACE" "$RELEASE_NAME" 120s
ok "Webhook controller deployed"

# ---------------------------------------------------------------------------
# 7. Create e2e test namespace
# ---------------------------------------------------------------------------
E2E_NS="${E2E_NAMESPACE:-e2e-test-ns}"
kubectl create namespace "$E2E_NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
ok "Test namespace '$E2E_NS' ready"

# ---------------------------------------------------------------------------
# 8. Write env file for tests
# ---------------------------------------------------------------------------
ENV_FILE="$SCRIPT_DIR/.env"
cat > "$ENV_FILE" <<EOF
E2E_GITEA_URL=http://localhost:${GITEA_LOCAL_PORT}
E2E_GITEA_TOKEN=${GITEA_TOKEN}
E2E_GITEA_ADMIN_USER=${GITEA_ADMIN_USER}
E2E_GITEA_ADMIN_PASS=${GITEA_ADMIN_PASS}
E2E_GITEA_ORG=${GITEA_ORG}
E2E_GITEA_REPO=${GITEA_REPO}
E2E_CLUSTER_NAME=${CLUSTER_NAME}
E2E_NAMESPACE=${E2E_NS}
E2E_WEBHOOK_NAMESPACE=${WEBHOOK_NAMESPACE}
EOF
ok "Environment file written to $ENV_FILE"

echo ""
echo "============================================="
echo "  E2E environment ready!"
echo "============================================="
echo ""
echo "  Run tests with:"
echo "    make e2e-test"
echo ""
echo "  Or manually:"
echo "    cd e2e && source .env && go test -v -tags=e2e -count=1 -timeout=5m ./..."
echo ""
echo "  Tear down with:"
echo "    make e2e-teardown"
echo ""
