package main

import (
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
)

// GitProvider identifies the Git hosting provider.
type GitProvider string

const (
	ProviderGitea  GitProvider = "gitea"
	ProviderGitHub GitProvider = "github"
	ProviderGitLab GitProvider = "gitlab"
)

// ParseGitProvider converts a string to a GitProvider.
// Returns ProviderGitea as the default for backward compatibility.
func ParseGitProvider(s string) (GitProvider, error) {
	switch GitProvider(s) {
	case ProviderGitea:
		return ProviderGitea, nil
	case ProviderGitHub:
		return ProviderGitHub, nil
	case ProviderGitLab:
		return ProviderGitLab, nil
	case "":
		return ProviderGitea, nil
	default:
		return "", fmt.Errorf("unsupported git provider: %q (supported: gitea, github, gitlab)", s)
	}
}

// GitClientConfig holds the configuration for creating a GitClient.
type GitClientConfig struct {
	Provider    GitProvider
	RepoURL     string
	Token       string
	ClusterName string
}

// GitClient is the interface for syncing Kubernetes resources to a Git repository.
type GitClient interface {
	// ProcessRequest handles an admission request and syncs the resource to Git.
	ProcessRequest(req *admissionv1.AdmissionRequest) error

	// GetPendingCount returns the number of operations waiting in the retry queue.
	GetPendingCount() int
}

// NewGitClient creates a GitClient for the given provider configuration.
func NewGitClient(cfg GitClientConfig) (GitClient, error) {
	return newGitClient(cfg)
}
