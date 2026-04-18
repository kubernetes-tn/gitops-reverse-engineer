package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tlsCertPath = "/etc/webhook/certs/tls.crt"
	tlsKeyPath  = "/etc/webhook/certs/tls.key"
)

var gitSyncClient GitClient

// handleAdmission processes admission review requests
func handleAdmission(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request from %s", r.Method, r.RemoteAddr)

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the AdmissionReview request
	admissionReview := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		log.Printf("Error unmarshaling admission review: %v", err)
		http.Error(w, "Error unmarshaling admission review", http.StatusBadRequest)
		return
	}

	// Extract request details
	req := admissionReview.Request
	if req == nil {
		log.Println("Admission review request is nil")
		http.Error(w, "Admission review request is nil", http.StatusBadRequest)
		return
	}

	// Print information about the resource
	printResourceInfo(req)

	// Check if we should process this request based on configuration
	if appConfig != nil && !appConfig.ShouldProcessRequest(req) {
		// Skip processing but allow the request
		admissionResponse := &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
			Result: &metav1.Status{
				Message: "Request allowed (not tracked by GitOps)",
			},
		}
		sendResponse(w, admissionResponse)
		return
	}

	// Process the request with Git (sync to Git repository)
	if gitSyncClient != nil {
		if err := gitSyncClient.ProcessRequest(req); err != nil {
			log.Printf("⚠️ Warning: Failed to sync to Git (operation queued for retry): %v", err)
			if metricsCollector != nil {
				metricsCollector.IncrementGitSyncFailure()
			}
			// Don't fail the admission - we queue for retry
		} else {
			if metricsCollector != nil {
				metricsCollector.IncrementGitSyncSuccess()
			}
		}
		// Update pending operations metric
		if metricsCollector != nil {
			metricsCollector.SetPendingOperations(uint64(gitSyncClient.GetPendingCount()))
		}
	} else {
		log.Printf("⚠️ Warning: Git client not initialized, skipping Git sync")
	}

	// Create a response that allows the request (no denial)
	admissionResponse := &admissionv1.AdmissionResponse{
		UID:     req.UID,
		Allowed: true,
		Result: &metav1.Status{
			Message: "Request allowed and synced to GitOps repository",
		},
	}

	sendResponse(w, admissionResponse)

	log.Printf("✅ Allowed %s operation on %s/%s in namespace %s",
		req.Operation,
		req.Kind.Kind,
		req.Name,
		req.Namespace,
	)
}

// sendResponse sends an admission response
func sendResponse(w http.ResponseWriter, admissionResponse *admissionv1.AdmissionResponse) {
	// Create the response AdmissionReview
	responseAdmissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: admissionResponse,
	}

	// Marshal and send the response
	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, "Error marshaling response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// printResourceInfo prints detailed information about the admission request
func printResourceInfo(req *admissionv1.AdmissionRequest) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("📝 Operation: %s\n", req.Operation)
	fmt.Printf("📦 Resource: %s/%s\n", req.Kind.Group, req.Kind.Kind)
	fmt.Printf("🏷️  Name: %s\n", req.Name)
	fmt.Printf("📁 Namespace: %s\n", req.Namespace)
	fmt.Printf("👤 User: %s\n", req.UserInfo.Username)
	fmt.Printf("🆔 UID: %s\n", req.UID)

	if req.Operation == admissionv1.Update {
		fmt.Println("🔄 This is an UPDATE operation")
	} else if req.Operation == admissionv1.Create {
		fmt.Println("✨ This is a CREATE operation")
	} else if req.Operation == admissionv1.Delete {
		fmt.Println("🗑️  This is a DELETE operation")
	}

	// Print a snippet of the object if available
	if len(req.Object.Raw) > 0 {
		var obj map[string]interface{}
		if err := json.Unmarshal(req.Object.Raw, &obj); err == nil {
			if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
				if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) > 0 {
					fmt.Printf("🏷️  Labels: %v\n", labels)
				}
				if annotations, ok := metadata["annotations"].(map[string]interface{}); ok && len(annotations) > 0 {
					fmt.Printf("📋 Annotations: %v\n", annotations)
				}
			}
		}
	}

	fmt.Println(strings.Repeat("=", 80) + "\n")
}

// healthCheck handles health check requests
func healthCheck(w http.ResponseWriter, r *http.Request) {
	pendingCount := 0
	if gitSyncClient != nil {
		pendingCount = gitSyncClient.GetPendingCount()
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK - Pending operations: %d", pendingCount)
}

func main() {
	log.Println("🚀 Starting GitOps Reversed Admission Controller...")

	// Load configuration
	configFile := os.Getenv("CONFIG_FILE")
	var err error
	appConfig, err = LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize metrics collector if enabled
	if appConfig.Metrics.Enabled {
		metricsCollector = NewMetricsCollector()
		log.Println("✅ Metrics collector initialized")
	}

	// Read environment variables
	gitRepoURL := os.Getenv("GIT_REPO_URL")
	gitToken := os.Getenv("GIT_TOKEN")
	if gitToken == "" {
		gitToken = os.Getenv("GITEA_TOKEN") // backward compatibility
	}
	clusterName := os.Getenv("CLUSTER_NAME")
	providerStr := os.Getenv("GIT_PROVIDER") // "gitea" (default), "github", "gitlab"

	if clusterName == "" {
		clusterName = "default-cluster"
		log.Printf("⚠️ CLUSTER_NAME not set, using default: %s", clusterName)
	}

	// Initialize Git client if configured
	if gitRepoURL != "" && gitToken != "" {
		provider, err := ParseGitProvider(providerStr)
		if err != nil {
			log.Fatalf("❌ %v", err)
		}
		log.Printf("🔧 Initializing Git client (provider=%s) for repo: %s", provider, gitRepoURL)
		client, err := NewGitClient(GitClientConfig{
			Provider:    provider,
			RepoURL:     gitRepoURL,
			Token:       gitToken,
			ClusterName: clusterName,
		})
		if err != nil {
			log.Printf("⚠️ Warning: Failed to initialize Git client: %v", err)
			log.Printf("⚠️ Controller will continue without Git sync")
		} else {
			gitSyncClient = client
			log.Printf("✅ Git client initialized successfully (provider=%s)", provider)
		}
	} else {
		log.Printf("⚠️ GIT_REPO_URL or GIT_TOKEN not set, Git sync disabled")
		log.Printf("ℹ️ Controller will run in observation mode only")
	}

	// Check if certificate files exist
	if _, err := os.Stat(tlsCertPath); os.IsNotExist(err) {
		log.Fatalf("TLS certificate not found at %s", tlsCertPath)
	}
	if _, err := os.Stat(tlsKeyPath); os.IsNotExist(err) {
		log.Fatalf("TLS key not found at %s", tlsKeyPath)
	}

	// Set up HTTP handlers
	http.HandleFunc("/validate", handleAdmission)
	http.HandleFunc("/mutate", handleAdmission)
	http.HandleFunc("/health", healthCheck)
	if appConfig.Metrics.Enabled {
		http.HandleFunc(appConfig.Metrics.Path, metricsHandler)
		log.Printf("📊 Metrics endpoint enabled at %s", appConfig.Metrics.Path)
	}

	// Start HTTPS server
	port := ":8443"
	log.Printf("✅ Admission controller listening on port %s", port)
	log.Printf("📜 Using TLS cert: %s", tlsCertPath)
	log.Printf("🔑 Using TLS key: %s", tlsKeyPath)
	log.Printf("🏷️  Cluster name: %s", clusterName)

	if err := http.ListenAndServeTLS(port, tlsCertPath, tlsKeyPath, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
