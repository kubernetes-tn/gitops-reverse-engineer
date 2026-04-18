#!/usr/bin/env bash
#
# e2e/teardown.sh — Tear down the e2e test environment
#
# Usage:
#   ./e2e/teardown.sh                     # delete Kind cluster
#   E2E_SKIP_KIND=true ./e2e/teardown.sh  # only remove test resources
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

KIND_CLUSTER_NAME="${E2E_KIND_CLUSTER:-gitops-e2e}"
SKIP_KIND="${E2E_SKIP_KIND:-false}"
WEBHOOK_NAMESPACE="gitops-reverse-engineer-system"
E2E_NS="${E2E_NAMESPACE:-e2e-test-ns}"

info()  { echo "▸ $*"; }
ok()    { echo "✅ $*"; }

# Kill port-forward if running
PORTFWD_PIDFILE="$SCRIPT_DIR/.portforward.pid"
if [[ -f "$PORTFWD_PIDFILE" ]]; then
  info "Stopping Gitea port-forward..."
  kill "$(cat "$PORTFWD_PIDFILE")" 2>/dev/null || true
  rm -f "$PORTFWD_PIDFILE"
  ok "Port-forward stopped"
fi

if [[ "$SKIP_KIND" == "true" ]]; then
  info "Cleaning up resources (keeping cluster)..."
  kubectl delete namespace "$E2E_NS" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete namespace gitea --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete namespace "$WEBHOOK_NAMESPACE" --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete validatingwebhookconfiguration -l app.kubernetes.io/name=gitops-reverse-engineer --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clusterrole -l app.kubernetes.io/name=gitops-reverse-engineer --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete clusterrolebinding -l app.kubernetes.io/name=gitops-reverse-engineer --ignore-not-found >/dev/null 2>&1 || true
  ok "Resources cleaned"
else
  info "Deleting Kind cluster '${KIND_CLUSTER_NAME}'..."
  kind delete cluster --name "$KIND_CLUSTER_NAME" 2>/dev/null || true
  ok "Kind cluster deleted"
fi

# Clean up env file
rm -f "$SCRIPT_DIR/.env"
ok "Environment file removed"

echo ""
echo "✅ E2E teardown complete"
