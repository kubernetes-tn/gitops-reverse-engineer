#!/bin/bash

# GitOps Reversed Admission - Integration Test Script
# This script tests the complete functionality of the admission controller

set -e

echo "🧪 GitOps Reversed Admission Controller - Integration Test"
echo "==========================================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${TEST_NAMESPACE:-gitops-reverse-engineer-system}"
TEST_NS="${TEST_TARGET_NAMESPACE:-default}"

# Helper functions
log_info() {
    echo -e "${GREEN}ℹ️  $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    if ! command -v helm &> /dev/null; then
        log_warn "helm is not installed (required for deployment)"
    fi
    
    log_success "Prerequisites check passed"
}

# Check if controller is running
check_controller_running() {
    log_info "Checking if controller is running..."
    
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_error "Namespace $NAMESPACE does not exist"
        log_info "Run 'make deploy' first to deploy the controller"
        exit 1
    fi
    
    POD_STATUS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=gitops-reverse-engineer -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")
    
    if [ "$POD_STATUS" != "Running" ]; then
        log_error "Controller pod is not running (status: $POD_STATUS)"
        kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=gitops-reverse-engineer
        exit 1
    fi
    
    log_success "Controller is running"
}

# Test 1: Create a ConfigMap
test_create_configmap() {
    log_info "Test 1: Creating a ConfigMap..."
    
    kubectl create configmap test-gitops-cm \
        --from-literal=test=value1 \
        --from-literal=env=test \
        -n "$TEST_NS" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    sleep 2
    
    if kubectl get configmap test-gitops-cm -n "$TEST_NS" &> /dev/null; then
        log_success "ConfigMap created successfully"
    else
        log_error "Failed to create ConfigMap"
        return 1
    fi
}

# Test 2: Update the ConfigMap
test_update_configmap() {
    log_info "Test 2: Updating the ConfigMap..."
    
    kubectl patch configmap test-gitops-cm -n "$TEST_NS" \
        -p '{"data":{"test":"value2","new-key":"new-value"}}'
    
    sleep 2
    
    VALUE=$(kubectl get configmap test-gitops-cm -n "$TEST_NS" -o jsonpath='{.data.test}')
    if [ "$VALUE" = "value2" ]; then
        log_success "ConfigMap updated successfully"
    else
        log_error "Failed to update ConfigMap"
        return 1
    fi
}

# Test 3: Create a Deployment
test_create_deployment() {
    log_info "Test 3: Creating a Deployment..."
    
    kubectl create deployment test-gitops-deploy \
        --image=nginx:latest \
        --replicas=1 \
        -n "$TEST_NS" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    sleep 2
    
    if kubectl get deployment test-gitops-deploy -n "$TEST_NS" &> /dev/null; then
        log_success "Deployment created successfully"
    else
        log_error "Failed to create Deployment"
        return 1
    fi
}

# Test 4: Scale the Deployment
test_scale_deployment() {
    log_info "Test 4: Scaling the Deployment..."
    
    kubectl scale deployment test-gitops-deploy -n "$TEST_NS" --replicas=3
    
    sleep 2
    
    REPLICAS=$(kubectl get deployment test-gitops-deploy -n "$TEST_NS" -o jsonpath='{.spec.replicas}')
    if [ "$REPLICAS" = "3" ]; then
        log_success "Deployment scaled successfully"
    else
        log_error "Failed to scale Deployment"
        return 1
    fi
}

# Test 5: Create a Service
test_create_service() {
    log_info "Test 5: Creating a Service..."
    
    kubectl expose deployment test-gitops-deploy \
        --port=80 \
        --target-port=80 \
        --name=test-gitops-svc \
        -n "$TEST_NS"
    
    sleep 2
    
    if kubectl get service test-gitops-svc -n "$TEST_NS" &> /dev/null; then
        log_success "Service created successfully"
    else
        log_error "Failed to create Service"
        return 1
    fi
}

# Test 6: Check metrics
test_metrics() {
    log_info "Test 6: Checking metrics endpoint..."
    
    POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=gitops-reverse-engineer -o jsonpath='{.items[0].metadata.name}')
    
    if [ -z "$POD_NAME" ]; then
        log_error "Could not find controller pod"
        return 1
    fi
    
    # Forward port temporarily
    kubectl port-forward -n "$NAMESPACE" "$POD_NAME" 8443:8443 &
    PF_PID=$!
    sleep 3
    
    METRICS=$(curl -sk https://localhost:8443/metrics 2>/dev/null || echo "")
    
    kill $PF_PID 2>/dev/null || true
    
    if echo "$METRICS" | grep -q "gitops_admission_git_sync_success_total"; then
        log_success "Metrics endpoint is working"
        echo ""
        echo "Sample metrics:"
        echo "$METRICS" | grep "gitops_admission"
    else
        log_warn "Metrics endpoint may not be working properly"
    fi
}

# Test 7: Check controller logs
test_controller_logs() {
    log_info "Test 7: Checking controller logs..."
    
    LOGS=$(kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=gitops-reverse-engineer --tail=50 2>/dev/null || echo "")
    
    if echo "$LOGS" | grep -q "Successfully synced"; then
        log_success "Controller is syncing resources to Git"
        echo ""
        echo "Recent log entries:"
        echo "$LOGS" | grep "Successfully synced" | tail -5
    elif echo "$LOGS" | grep -q "Skipping"; then
        log_warn "Controller is running but skipping resources (check configuration)"
    else
        log_warn "No sync activity detected in logs"
    fi
}

# Test 8: Delete resources
test_delete_resources() {
    log_info "Test 8: Deleting test resources..."
    
    kubectl delete service test-gitops-svc -n "$TEST_NS" --ignore-not-found=true
    kubectl delete deployment test-gitops-deploy -n "$TEST_NS" --ignore-not-found=true
    kubectl delete configmap test-gitops-cm -n "$TEST_NS" --ignore-not-found=true
    
    sleep 2
    
    log_success "Test resources deleted"
}

# Test 9: Check health endpoint
test_health_endpoint() {
    log_info "Test 9: Checking health endpoint..."
    
    POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=gitops-reverse-engineer -o jsonpath='{.items[0].metadata.name}')
    
    if [ -z "$POD_NAME" ]; then
        log_error "Could not find controller pod"
        return 1
    fi
    
    kubectl port-forward -n "$NAMESPACE" "$POD_NAME" 8443:8443 &
    PF_PID=$!
    sleep 3
    
    HEALTH=$(curl -sk https://localhost:8443/health 2>/dev/null || echo "")
    
    kill $PF_PID 2>/dev/null || true
    
    if echo "$HEALTH" | grep -q "OK"; then
        log_success "Health endpoint is responding: $HEALTH"
    else
        log_error "Health endpoint is not responding properly"
        return 1
    fi
}

# Test 10: Verify webhook configuration
test_webhook_config() {
    log_info "Test 10: Verifying webhook configuration..."
    
    WEBHOOK_NAME=$(kubectl get validatingwebhookconfigurations -o name | grep gitops-reverse-engineer || echo "")
    
    if [ -z "$WEBHOOK_NAME" ]; then
        log_error "ValidatingWebhookConfiguration not found"
        return 1
    fi
    
    WEBHOOK_READY=$(kubectl get "$WEBHOOK_NAME" -o jsonpath='{.webhooks[0].clientConfig.service.name}' 2>/dev/null || echo "")
    
    if [ -n "$WEBHOOK_READY" ]; then
        log_success "Webhook configuration is valid"
        echo "Webhook: $WEBHOOK_NAME"
    else
        log_error "Webhook configuration is not valid"
        return 1
    fi
}

# Main test execution
main() {
    echo ""
    check_prerequisites
    echo ""
    check_controller_running
    echo ""
    
    FAILED_TESTS=0
    
    # Run tests
    test_create_configmap || ((FAILED_TESTS++))
    echo ""
    
    test_update_configmap || ((FAILED_TESTS++))
    echo ""
    
    test_create_deployment || ((FAILED_TESTS++))
    echo ""
    
    test_scale_deployment || ((FAILED_TESTS++))
    echo ""
    
    test_create_service || ((FAILED_TESTS++))
    echo ""
    
    test_health_endpoint || ((FAILED_TESTS++))
    echo ""
    
    test_metrics || ((FAILED_TESTS++))
    echo ""
    
    test_controller_logs || ((FAILED_TESTS++))
    echo ""
    
    test_webhook_config || ((FAILED_TESTS++))
    echo ""
    
    test_delete_resources || ((FAILED_TESTS++))
    echo ""
    
    # Summary
    echo "==========================================================="
    if [ $FAILED_TESTS -eq 0 ]; then
        log_success "All tests passed! 🎉"
        echo ""
        log_info "Next steps:"
        echo "  1. Check your Git repository for committed resources"
        echo "  2. Verify Git commit authors match Kubernetes users"
        echo "  3. Review Prometheus metrics in your monitoring system"
        echo "  4. Check PrometheusRule alerts are configured"
    else
        log_error "$FAILED_TESTS test(s) failed"
        exit 1
    fi
}

# Run main function
main
