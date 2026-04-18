package main

import (
	"fmt"
	"net/http"
	"sync"
)

// MetricsCollector collects metrics for the admission controller
type MetricsCollector struct {
	gitSyncSuccessTotal      uint64
	gitSyncFailuresTotal     uint64
	skippedCommitsTotal      uint64
	nonFastForwardTotal      uint64
	obfuscatedSecretsTotal   uint64
	secretChangesDetected    uint64
	pendingOperations        uint64
	mu                       sync.RWMutex
}

var metricsCollector *MetricsCollector

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		gitSyncSuccessTotal:    0,
		gitSyncFailuresTotal:   0,
		skippedCommitsTotal:    0,
		nonFastForwardTotal:    0,
		obfuscatedSecretsTotal: 0,
		secretChangesDetected:  0,
		pendingOperations:      0,
	}
}

// IncrementGitSyncSuccess increments the git sync success counter
func (m *MetricsCollector) IncrementGitSyncSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gitSyncSuccessTotal++
}

// IncrementGitSyncFailure increments the git sync failure counter
func (m *MetricsCollector) IncrementGitSyncFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gitSyncFailuresTotal++
}

// IncrementSkippedCommits increments the skipped commits counter
func (m *MetricsCollector) IncrementSkippedCommits() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skippedCommitsTotal++
}

// IncrementNonFastForward increments the non-fast-forward counter
func (m *MetricsCollector) IncrementNonFastForward() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nonFastForwardTotal++
}

// IncrementObfuscatedSecrets increments the obfuscated secrets counter
func (m *MetricsCollector) IncrementObfuscatedSecrets() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.obfuscatedSecretsTotal++
}

// IncrementSecretChangesDetected increments the secret changes detected counter
func (m *MetricsCollector) IncrementSecretChangesDetected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secretChangesDetected++
}

// SetPendingOperations sets the current pending operations count
func (m *MetricsCollector) SetPendingOperations(count uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingOperations = count
}

// GetMetrics returns the current metrics snapshot
func (m *MetricsCollector) GetMetrics() (success, failures, skipped, nonFastForward, obfuscated, secretChanges, pending uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.gitSyncSuccessTotal, m.gitSyncFailuresTotal, m.skippedCommitsTotal, m.nonFastForwardTotal, m.obfuscatedSecretsTotal, m.secretChangesDetected, m.pendingOperations
}

// metricsHandler handles the /metrics endpoint
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if metricsCollector == nil {
		http.Error(w, "Metrics not enabled", http.StatusNotFound)
		return
	}

	success, failures, skipped, nonFastForward, obfuscated, secretChanges, pending := metricsCollector.GetMetrics()

	// Output in Prometheus format
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP gitops_admission_git_sync_success_total Total number of successful Git syncs\n")
	fmt.Fprintf(w, "# TYPE gitops_admission_git_sync_success_total counter\n")
	fmt.Fprintf(w, "gitops_admission_git_sync_success_total %d\n", success)

	fmt.Fprintf(w, "# HELP gitops_admission_git_sync_failures_total Total number of failed Git syncs\n")
	fmt.Fprintf(w, "# TYPE gitops_admission_git_sync_failures_total counter\n")
	fmt.Fprintf(w, "gitops_admission_git_sync_failures_total %d\n", failures)

	fmt.Fprintf(w, "# HELP gitops_admission_skipped_commits_total Total number of skipped commits (no changes detected)\n")
	fmt.Fprintf(w, "# TYPE gitops_admission_skipped_commits_total counter\n")
	fmt.Fprintf(w, "gitops_admission_skipped_commits_total %d\n", skipped)

	fmt.Fprintf(w, "# HELP gitops_admission_non_fast_forward_total Total number of non-fast-forward pull errors resolved\n")
	fmt.Fprintf(w, "# TYPE gitops_admission_non_fast_forward_total counter\n")
	fmt.Fprintf(w, "gitops_admission_non_fast_forward_total %d\n", nonFastForward)

	fmt.Fprintf(w, "# HELP gitops_admission_obfuscated_secrets_total Total number of secrets obfuscated before commit\n")
	fmt.Fprintf(w, "# TYPE gitops_admission_obfuscated_secrets_total counter\n")
	fmt.Fprintf(w, "gitops_admission_obfuscated_secrets_total %d\n", obfuscated)

	fmt.Fprintf(w, "# HELP gitops_admission_secret_changes_detected_total Total number of secret changes detected (even when obfuscated)\n")
	fmt.Fprintf(w, "# TYPE gitops_admission_secret_changes_detected_total counter\n")
	fmt.Fprintf(w, "gitops_admission_secret_changes_detected_total %d\n", secretChanges)

	fmt.Fprintf(w, "# HELP gitops_admission_pending_operations Current number of pending operations\n")
	fmt.Fprintf(w, "# TYPE gitops_admission_pending_operations gauge\n")
	fmt.Fprintf(w, "gitops_admission_pending_operations %d\n", pending)
}
