package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"gopkg.in/yaml.v3"
	admissionv1 "k8s.io/api/admission/v1"
)

// PendingOperation represents a Git operation that failed and needs retry
type PendingOperation struct {
	Request      *admissionv1.AdmissionRequest
	ResourcePath string
	Operation    admissionv1.Operation
	Timestamp    time.Time
	RetryCount   int
}

// gitClient handles Git operations with fault tolerance.
// It implements the GitClient interface and works with any Git provider.
type gitClient struct {
	provider      GitProvider
	repoURL       string
	token         string
	repo          *git.Repository
	worktree      *git.Worktree
	repoPath      string
	clusterName   string
	pendingOps    []PendingOperation
	mu            sync.Mutex
	retryInterval time.Duration
	maxRetries    int
}

// newGitClient creates a gitClient from the given config.
func newGitClient(cfg GitClientConfig) (*gitClient, error) {
	repoPath := "/tmp/gitops-repo"

	client := &gitClient{
		provider:      cfg.Provider,
		repoURL:       cfg.RepoURL,
		token:         cfg.Token,
		repoPath:      repoPath,
		clusterName:   cfg.ClusterName,
		pendingOps:    make([]PendingOperation, 0),
		retryInterval: 30 * time.Second,
		maxRetries:    10,
	}

	if err := client.initRepo(); err != nil {
		log.Printf("⚠️ Failed to initialize repo (will retry): %v", err)
		// Don't fail - we'll retry later
		go client.retryLoop()
		return client, nil
	}

	// Start retry loop for pending operations
	go client.retryLoop()

	return client, nil
}

// authCredentials returns the HTTP basic auth for the configured provider.
//
// Each provider uses a different username convention:
//   - Gitea:  "oauth2"
//   - GitHub: "x-access-token"
//   - GitLab: "oauth2"
func (c *gitClient) authCredentials() *githttp.BasicAuth {
	username := "oauth2"
	switch c.provider {
	case ProviderGitHub:
		username = "x-access-token"
	case ProviderGitLab:
		username = "oauth2"
	}
	return &githttp.BasicAuth{
		Username: username,
		Password: c.token,
	}
}

// initRepo initializes the Git repository
func (c *gitClient) initRepo() error {
	log.Printf("🔄 Initializing repository %s into %s", c.repoURL, c.repoPath)

	// Clean up previous repo if it exists
	if _, err := os.Stat(c.repoPath); !os.IsNotExist(err) {
		os.RemoveAll(c.repoPath)
	}

	repo, err := git.PlainClone(c.repoPath, false, &git.CloneOptions{
		URL:      c.repoURL,
		Progress: os.Stdout,
		Auth:     c.authCredentials(),
	})
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	c.repo = repo
	c.worktree = worktree

	log.Printf("✅ Successfully initialized Git repository")
	return nil
}

// ProcessRequest processes an admission request and syncs to Git
func (c *gitClient) ProcessRequest(req *admissionv1.AdmissionRequest) error {
	resourcePath := c.buildResourcePath(req)

	switch req.Operation {
	case admissionv1.Create, admissionv1.Update:
		return c.handleCreateOrUpdate(req, resourcePath)
	case admissionv1.Delete:
		return c.handleDelete(req, resourcePath)
	}
	return nil
}

// buildResourcePath creates the path: {clustername}/{namespace}/{resourcekind}/{resourcename}.yaml
// For cluster-scoped resources: {clustername}/_cluster/{resourcekind}/{resourcename}.yaml
func (c *gitClient) buildResourcePath(req *admissionv1.AdmissionRequest) string {
	// For cluster-scoped resources (no namespace)
	if req.Namespace == "" {
		return filepath.Join(
			c.repoPath,
			c.clusterName,
			"_cluster",
			strings.ToLower(req.Kind.Kind),
			req.Name+".yaml",
		)
	}

	// For namespaced resources
	return filepath.Join(
		c.repoPath,
		c.clusterName,
		req.Namespace,
		strings.ToLower(req.Kind.Kind),
		req.Name+".yaml",
	)
}

// handleCreateOrUpdate handles CREATE and UPDATE operations
func (c *gitClient) handleCreateOrUpdate(req *admissionv1.AdmissionRequest, resourcePath string) error {
	log.Printf("📝 Handling %s for %s", req.Operation, resourcePath)

	// Check if repo is initialized
	if c.repo == nil {
		c.addToPendingQueue(req, resourcePath)
		return fmt.Errorf("Git repository not initialized, operation queued")
	}

	// Pull latest changes first
	if err := c.pullLatest(); err != nil {
		log.Printf("⚠️ Failed to pull latest changes: %v", err)
		c.addToPendingQueue(req, resourcePath)
		return err
	}

	// Clean the resource object (kubectl-neat functionality)
	cleanedBytes, err := cleanResource(req.Object.Raw)
	if err != nil {
		return fmt.Errorf("failed to clean resource object: %w", err)
	}

	var obj interface{}
	if err := json.Unmarshal(cleanedBytes, &obj); err != nil {
		return fmt.Errorf("failed to unmarshal cleaned resource object: %w", err)
	}

	// Milestone 6: Obfuscate secret data before committing to Git
	isSecret := req.Kind.Kind == "Secret"
	if isSecret {
		if objMap, ok := obj.(map[string]interface{}); ok {
			obfuscateSecretData(objMap)
			if metricsCollector != nil {
				metricsCollector.IncrementObfuscatedSecrets()
			}
		}
	}

	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal resource to YAML: %w", err)
	}

	// Milestone 4: Check if content has actually changed before writing
	relPath := strings.TrimPrefix(resourcePath, c.repoPath+"/")
	hasChanged, err := c.hasFileChanged(resourcePath, yamlBytes, isSecret)
	if err != nil {
		log.Printf("⚠️ Error checking file changes: %v, proceeding with write", err)
		hasChanged = true // If we can't check, assume changed to be safe
	}

	if !hasChanged {
		log.Printf("⏭️  Skipping commit for %s - no changes detected", relPath)
		if metricsCollector != nil {
			metricsCollector.IncrementSkippedCommits()
		}
		return nil
	}

	// Create directory structure
	if err := os.MkdirAll(filepath.Dir(resourcePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write resource file
	if err := os.WriteFile(resourcePath, yamlBytes, 0644); err != nil {
		return fmt.Errorf("failed to write resource file: %w", err)
	}

	// Add to Git
	if _, err := c.worktree.Add(relPath); err != nil {
		return fmt.Errorf("failed to add file to worktree: %w", err)
	}

	// Commit and push with dynamic author
	commitMsg := fmt.Sprintf("%s: %s %s/%s", req.Operation, req.Kind.Kind, req.Namespace, req.Name)
	author := c.buildGitAuthor(req)
	if err := c.commitAndPush(commitMsg, author); err != nil {
		c.addToPendingQueue(req, resourcePath)
		return err
	}

	log.Printf("✅ Successfully synced %s to Git", relPath)
	return nil
}

// handleDelete handles DELETE operations
func (c *gitClient) handleDelete(req *admissionv1.AdmissionRequest, resourcePath string) error {
	log.Printf("🗑️  Handling DELETE for %s", resourcePath)

	// Check if repo is initialized
	if c.repo == nil {
		c.addToPendingQueue(req, resourcePath)
		return fmt.Errorf("Git repository not initialized, operation queued")
	}

	// Pull latest changes first
	if err := c.pullLatest(); err != nil {
		log.Printf("⚠️ Failed to pull latest changes: %v", err)
		c.addToPendingQueue(req, resourcePath)
		return err
	}

	// Check if file exists
	if _, err := os.Stat(resourcePath); os.IsNotExist(err) {
		log.Printf("ℹ️ File %s does not exist, nothing to delete", resourcePath)
		return nil
	}

	// Remove from Git
	relPath := strings.TrimPrefix(resourcePath, c.repoPath+"/")
	if _, err := c.worktree.Remove(relPath); err != nil {
		return fmt.Errorf("failed to remove file from worktree: %w", err)
	}

	// Commit and push with dynamic author
	commitMsg := fmt.Sprintf("DELETE: %s %s/%s", req.Kind.Kind, req.Namespace, req.Name)
	author := c.buildGitAuthor(req)
	if err := c.commitAndPush(commitMsg, author); err != nil {
		c.addToPendingQueue(req, resourcePath)
		return err
	}

	log.Printf("✅ Successfully deleted %s from Git", relPath)
	return nil
}

// pullLatest pulls the latest changes from remote
// If a regular pull fails due to non-fast-forward update, it performs a force pull
func (c *gitClient) pullLatest() error {
	err := c.worktree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       c.authCredentials(),
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		// Check if it's a non-fast-forward error
		if strings.Contains(err.Error(), "non-fast-forward") {
			log.Printf("⚠️ Non-fast-forward update detected, performing force pull...")
			if metricsCollector != nil {
				metricsCollector.IncrementNonFastForward()
			}
			return c.forcePull()
		}
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}

// forcePull performs a hard reset to match the remote branch
// This is used when regular pull fails due to non-fast-forward updates
func (c *gitClient) forcePull() error {
	// Fetch the latest changes from remote
	err := c.repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		Auth:       c.authCredentials(),
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	// Get the reference to origin/main (or origin/master)
	// Try main first, then master
	var ref *plumbing.Reference
	ref, err = c.repo.Reference("refs/remotes/origin/main", true)
	if err != nil {
		// Try master if main doesn't exist
		ref, err = c.repo.Reference("refs/remotes/origin/master", true)
		if err != nil {
			return fmt.Errorf("failed to get remote reference: %w", err)
		}
	}

	// Hard reset to the remote branch
	err = c.worktree.Reset(&git.ResetOptions{
		Commit: ref.Hash(),
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("failed to reset to remote: %w", err)
	}

	log.Printf("✅ Successfully force-pulled from remote (reset to %s)", ref.Hash().String()[:7])
	return nil
}

// buildGitAuthor creates a Git author signature from the admission request user info
func (c *gitClient) buildGitAuthor(req *admissionv1.AdmissionRequest) *object.Signature {
	username := req.UserInfo.Username
	email := username + "@cluster.local"

	// Replace colons with dashes for serviceaccounts and users
	name := strings.ReplaceAll(username, ":", "-")
	email = strings.ReplaceAll(email, ":", "-")

	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}
}

// commitAndPush commits and pushes changes to remote
func (c *gitClient) commitAndPush(commitMessage string, author *object.Signature) error {
	status, err := c.worktree.Status()
	if err != nil {
		return fmt.Errorf("failed to get worktree status: %w", err)
	}

	if status.IsClean() {
		log.Println("ℹ️ No changes to commit")
		return nil
	}

	commit, err := c.worktree.Commit(commitMessage, &git.CommitOptions{
		Author: author,
	})
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	log.Printf("📦 Committed %s", commit.String())

	err = c.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       c.authCredentials(),
	})
	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	log.Println("🚀 Successfully pushed changes to Git")
	return nil
}

// addToPendingQueue adds a failed operation to the retry queue
func (c *gitClient) addToPendingQueue(req *admissionv1.AdmissionRequest, resourcePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	op := PendingOperation{
		Request:      req,
		ResourcePath: resourcePath,
		Operation:    req.Operation,
		Timestamp:    time.Now(),
		RetryCount:   0,
	}

	c.pendingOps = append(c.pendingOps, op)
	log.Printf("⏳ Added operation to pending queue (queue size: %d)", len(c.pendingOps))
}

// retryLoop periodically retries pending operations
func (c *gitClient) retryLoop() {
	ticker := time.NewTicker(c.retryInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.processPendingOperations()
	}
}

// processPendingOperations attempts to process all pending operations
func (c *gitClient) processPendingOperations() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pendingOps) == 0 {
		return
	}

	log.Printf("🔄 Processing %d pending operations...", len(c.pendingOps))

	// Try to initialize repo if not done yet
	if c.repo == nil {
		if err := c.initRepo(); err != nil {
			log.Printf("⚠️ Still unable to initialize repo: %v", err)
			return
		}
	}

	// Process each pending operation
	remaining := make([]PendingOperation, 0)
	for _, op := range c.pendingOps {
		op.RetryCount++

		var err error
		// Unlock mutex during processing to avoid blocking
		c.mu.Unlock()
		switch op.Operation {
		case admissionv1.Create, admissionv1.Update:
			err = c.handleCreateOrUpdate(op.Request, op.ResourcePath)
		case admissionv1.Delete:
			err = c.handleDelete(op.Request, op.ResourcePath)
		}
		c.mu.Lock()

		if err != nil {
			if op.RetryCount < c.maxRetries {
				log.Printf("⚠️ Retry %d/%d failed for %s: %v", op.RetryCount, c.maxRetries, op.ResourcePath, err)
				remaining = append(remaining, op)
			} else {
				log.Printf("❌ Max retries reached for %s, discarding operation", op.ResourcePath)
			}
		} else {
			log.Printf("✅ Successfully processed pending operation: %s", op.ResourcePath)
		}
	}

	c.pendingOps = remaining
	log.Printf("📊 Pending operations remaining: %d", len(c.pendingOps))
}

// GetPendingCount returns the number of pending operations
func (c *gitClient) GetPendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pendingOps)
}

// hasFileChanged compares the new YAML content with the existing file
// Returns true if the file is new or has changed, false if identical
// For secrets, uses special comparison logic that detects changes even with obfuscated data
func (c *gitClient) hasFileChanged(filePath string, newContent []byte, isSecret bool) (bool, error) {
	// Check if file exists
	existingContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, so this is a new file
			return true, nil
		}
		// Error reading file
		return true, err
	}

	// For secrets, use special comparison logic
	if isSecret {
		return c.hasSecretChanged(existingContent, newContent)
	}

	// For non-secrets, use normal YAML comparison
	// Normalize both contents for comparison by parsing and re-marshaling
	// This ensures that YAML formatting differences don't trigger false changes
	existingNormalized, err := normalizeYAML(existingContent)
	if err != nil {
		log.Printf("⚠️ Failed to normalize existing YAML: %v", err)
		return true, nil // Assume changed if we can't normalize
	}

	newNormalized, err := normalizeYAML(newContent)
	if err != nil {
		log.Printf("⚠️ Failed to normalize new YAML: %v", err)
		return true, nil // Assume changed if we can't normalize
	}

	// Compare the normalized content
	return string(existingNormalized) != string(newNormalized), nil
}

// hasSecretChanged compares two obfuscated secrets and detects if there are actual changes
func (c *gitClient) hasSecretChanged(existingContent, newContent []byte) (bool, error) {
	var existingObj, newObj map[string]interface{}

	// Parse existing secret
	if err := yaml.Unmarshal(existingContent, &existingObj); err != nil {
		log.Printf("⚠️ Failed to parse existing secret YAML: %v", err)
		return true, nil // Assume changed if we can't parse
	}

	// Parse new secret
	if err := yaml.Unmarshal(newContent, &newObj); err != nil {
		log.Printf("⚠️ Failed to parse new secret YAML: %v", err)
		return true, nil // Assume changed if we can't parse
	}

	// Use the detectSecretChanges function to compare
	hasChanged := detectSecretChanges(existingObj, newObj)

	if hasChanged && metricsCollector != nil {
		metricsCollector.IncrementSecretChangesDetected()
	}

	return hasChanged, nil
}

// normalizeYAML parses and re-marshals YAML to ensure consistent formatting
func normalizeYAML(content []byte) ([]byte, error) {
	var obj interface{}
	if err := yaml.Unmarshal(content, &obj); err != nil {
		return nil, err
	}
	return yaml.Marshal(obj)
}

// obfuscateSecretData obfuscates the data and stringData fields in a Secret object
// Similar to how ArgoCD displays secrets in the UI - showing keys but hiding values
func obfuscateSecretData(secretObj map[string]interface{}) {
	// Obfuscate the 'data' field (base64-encoded values)
	if data, ok := secretObj["data"].(map[string]interface{}); ok {
		obfuscatedData := make(map[string]interface{})
		for key := range data {
			obfuscatedData[key] = "********"
		}
		secretObj["data"] = obfuscatedData
	}

	// Obfuscate the 'stringData' field (plain text values)
	if stringData, ok := secretObj["stringData"].(map[string]interface{}); ok {
		obfuscatedStringData := make(map[string]interface{})
		for key := range stringData {
			obfuscatedStringData[key] = "********"
		}
		secretObj["stringData"] = obfuscatedStringData
	}
}

// detectSecretChanges compares two secret objects and determines if the actual data has changed
// even though the values are obfuscated. It compares:
// 1. The set of keys in data/stringData
// 2. The type of the secret
// 3. Annotations and labels
func detectSecretChanges(existing, new map[string]interface{}) bool {
	// Compare secret type
	if getStringField(existing, "type") != getStringField(new, "type") {
		log.Printf("🔍 Secret change detected: type changed")
		return true
	}

	// Compare data keys
	existingDataKeys := getSecretDataKeys(existing, "data")
	newDataKeys := getSecretDataKeys(new, "data")
	if !stringSlicesEqual(existingDataKeys, newDataKeys) {
		log.Printf("🔍 Secret change detected: data keys changed")
		return true
	}

	// Compare stringData keys
	existingStringDataKeys := getSecretDataKeys(existing, "stringData")
	newStringDataKeys := getSecretDataKeys(new, "stringData")
	if !stringSlicesEqual(existingStringDataKeys, newStringDataKeys) {
		log.Printf("🔍 Secret change detected: stringData keys changed")
		return true
	}

	// Compare metadata (labels, annotations)
	if hasMetadataChanged(existing, new) {
		log.Printf("🔍 Secret change detected: metadata changed")
		return true
	}

	// No changes detected in obfuscated secret
	return false
}

// getStringField safely retrieves a string field from a nested map structure
func getStringField(obj map[string]interface{}, field string) string {
	if val, ok := obj[field].(string); ok {
		return val
	}
	return ""
}

// getSecretDataKeys extracts the keys from the data or stringData field of a secret
func getSecretDataKeys(secretObj map[string]interface{}, fieldName string) []string {
	keys := []string{}
	if dataField, ok := secretObj[fieldName].(map[string]interface{}); ok {
		for key := range dataField {
			keys = append(keys, key)
		}
	}
	return keys
}

// stringSlicesEqual compares two string slices for equality (order-independent)
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create a map for quick lookup
	aMap := make(map[string]bool)
	for _, item := range a {
		aMap[item] = true
	}

	// Check if all items in b exist in a
	for _, item := range b {
		if !aMap[item] {
			return false
		}
	}

	return true
}

// hasMetadataChanged checks if metadata (labels, annotations) has changed
func hasMetadataChanged(existing, new map[string]interface{}) bool {
	existingMeta, existingOk := existing["metadata"].(map[string]interface{})
	newMeta, newOk := new["metadata"].(map[string]interface{})

	if existingOk != newOk {
		return true
	}

	if !existingOk {
		return false
	}

	// Compare labels
	existingLabels, _ := existingMeta["labels"].(map[string]interface{})
	newLabels, _ := newMeta["labels"].(map[string]interface{})
	if !mapsEqual(existingLabels, newLabels) {
		return true
	}

	// Compare annotations
	existingAnnotations, _ := existingMeta["annotations"].(map[string]interface{})
	newAnnotations, _ := newMeta["annotations"].(map[string]interface{})
	if !mapsEqual(existingAnnotations, newAnnotations) {
		return true
	}

	return false
}

// mapsEqual compares two string maps for equality
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}
		if aVal != bVal {
			return false
		}
	}

	return true
}
