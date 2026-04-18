//go:build e2e

// Package e2e contains end-to-end tests for the gitops-reverse-engineer
// admission controller. Tests require a running Kubernetes cluster with
// Gitea and the webhook controller deployed. See e2e/setup.sh.
package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ---------------------------------------------------------------------------
// Test environment
// ---------------------------------------------------------------------------

var (
	k8s         *kubernetes.Clientset
	giteaURL    string
	giteaToken  string
	giteaOrg    string
	giteaRepo   string
	clusterName string
	testNS      string
	webhookNS   string
)

func TestMain(m *testing.M) {
	giteaURL = envOrDefault("E2E_GITEA_URL", "http://localhost:30080")
	giteaToken = requireEnv("E2E_GITEA_TOKEN")
	giteaOrg = envOrDefault("E2E_GITEA_ORG", "e2e-org")
	giteaRepo = envOrDefault("E2E_GITEA_REPO", "gitops-e2e")
	clusterName = envOrDefault("E2E_CLUSTER_NAME", "e2e-test")
	testNS = envOrDefault("E2E_NAMESPACE", "e2e-test-ns")
	webhookNS = envOrDefault("E2E_WEBHOOK_NAMESPACE", "gitops-reverse-engineer-system")

	// Build k8s client
	kubeconfig := envOrDefault("KUBECONFIG", clientcmd.RecommendedHomeFile)
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load kubeconfig: %v\n", err)
		os.Exit(1)
	}
	k8s, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create k8s client: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Tests — resource sync
// ---------------------------------------------------------------------------

func TestCreateConfigMap(t *testing.T) {
	ctx := context.Background()
	name := "e2e-cm-" + shortID()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Data:       map[string]string{"key1": "value1", "key2": "value2"},
	}

	t.Cleanup(func() {
		_ = k8s.CoreV1().ConfigMaps(testNS).Delete(ctx, name, metav1.DeleteOptions{})
	})

	_, err := k8s.CoreV1().ConfigMaps(testNS).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}

	path := fmt.Sprintf("%s/%s/configmap/%s.yaml", clusterName, testNS, name)
	content := waitForGiteaFile(t, path, 30*time.Second)

	if !strings.Contains(content, "key1: value1") {
		t.Errorf("expected 'key1: value1' in committed YAML, got:\n%s", content)
	}
	if !strings.Contains(content, "key2: value2") {
		t.Errorf("expected 'key2: value2' in committed YAML, got:\n%s", content)
	}
	// Verify runtime fields are cleaned (neat)
	if strings.Contains(content, "resourceVersion") {
		t.Error("committed YAML still contains resourceVersion (should be cleaned)")
	}
	if strings.Contains(content, "uid:") {
		t.Error("committed YAML still contains uid (should be cleaned)")
	}
}

func TestUpdateConfigMap(t *testing.T) {
	ctx := context.Background()
	name := "e2e-cm-upd-" + shortID()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Data:       map[string]string{"version": "v1"},
	}

	t.Cleanup(func() {
		_ = k8s.CoreV1().ConfigMaps(testNS).Delete(ctx, name, metav1.DeleteOptions{})
	})

	created, err := k8s.CoreV1().ConfigMaps(testNS).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}
	path := fmt.Sprintf("%s/%s/configmap/%s.yaml", clusterName, testNS, name)
	waitForGiteaFile(t, path, 30*time.Second)

	// Update
	created.Data["version"] = "v2"
	_, err = k8s.CoreV1().ConfigMaps(testNS).Update(ctx, created, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("update configmap: %v", err)
	}

	waitForGiteaFileContent(t, path, "version: v2", 30*time.Second)
}

func TestDeleteConfigMap(t *testing.T) {
	ctx := context.Background()
	name := "e2e-cm-del-" + shortID()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Data:       map[string]string{"temp": "data"},
	}

	_, err := k8s.CoreV1().ConfigMaps(testNS).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap: %v", err)
	}
	path := fmt.Sprintf("%s/%s/configmap/%s.yaml", clusterName, testNS, name)
	waitForGiteaFile(t, path, 30*time.Second)

	// Delete
	err = k8s.CoreV1().ConfigMaps(testNS).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("delete configmap: %v", err)
	}

	waitForGiteaFileGone(t, path, 30*time.Second)
}

func TestSecretObfuscation(t *testing.T) {
	ctx := context.Background()
	name := "e2e-secret-" + shortID()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"password": "super-secret-value",
			"api-key":  "sk-1234567890",
		},
	}

	t.Cleanup(func() {
		_ = k8s.CoreV1().Secrets(testNS).Delete(ctx, name, metav1.DeleteOptions{})
	})

	_, err := k8s.CoreV1().Secrets(testNS).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	path := fmt.Sprintf("%s/%s/secret/%s.yaml", clusterName, testNS, name)
	content := waitForGiteaFile(t, path, 30*time.Second)

	// Secret values MUST be obfuscated
	if strings.Contains(content, "super-secret-value") {
		t.Error("secret value 'super-secret-value' was NOT obfuscated in git!")
	}
	if strings.Contains(content, "sk-1234567890") {
		t.Error("secret value 'sk-1234567890' was NOT obfuscated in git!")
	}
	// Keys should still be present
	if !strings.Contains(content, "password") {
		t.Error("expected key 'password' in committed YAML")
	}
	if !strings.Contains(content, "api-key") {
		t.Error("expected key 'api-key' in committed YAML")
	}
	// Obfuscated marker
	if !strings.Contains(content, "********") {
		t.Error("expected '********' obfuscation marker in committed YAML")
	}
}

func TestDeploymentSync(t *testing.T) {
	ctx := context.Background()
	name := "e2e-deploy-" + shortID()

	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNS,
			Labels:    map[string]string{"app": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "nginx",
						Image: "nginx:alpine",
					}},
				},
			},
		},
	}

	t.Cleanup(func() {
		_ = k8s.AppsV1().Deployments(testNS).Delete(ctx, name, metav1.DeleteOptions{})
	})

	_, err := k8s.AppsV1().Deployments(testNS).Create(ctx, deploy, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	path := fmt.Sprintf("%s/%s/deployment/%s.yaml", clusterName, testNS, name)
	content := waitForGiteaFile(t, path, 30*time.Second)

	if !strings.Contains(content, "nginx:alpine") {
		t.Errorf("expected 'nginx:alpine' in committed YAML, got:\n%s", content)
	}
	// Verify neat cleaned runtime fields
	if strings.Contains(content, "status:") {
		t.Error("committed YAML still contains status (should be cleaned)")
	}
}

func TestServiceSync(t *testing.T) {
	ctx := context.Background()
	name := "e2e-svc-" + shortID()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "test"},
			Ports: []corev1.ServicePort{{
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
		},
	}

	t.Cleanup(func() {
		_ = k8s.CoreV1().Services(testNS).Delete(ctx, name, metav1.DeleteOptions{})
	})

	_, err := k8s.CoreV1().Services(testNS).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	path := fmt.Sprintf("%s/%s/service/%s.yaml", clusterName, testNS, name)
	content := waitForGiteaFile(t, path, 30*time.Second)

	if !strings.Contains(content, "port: 80") {
		t.Errorf("expected 'port: 80' in committed YAML, got:\n%s", content)
	}
	// clusterIP should be cleaned by neat
	if strings.Contains(content, "clusterIP:") {
		t.Error("committed YAML still contains clusterIP (should be cleaned)")
	}
}

// ---------------------------------------------------------------------------
// Tests — filtering
// ---------------------------------------------------------------------------

func TestExcludedNamespace(t *testing.T) {
	ctx := context.Background()
	name := "e2e-excluded-" + shortID()

	// Create a configmap in kube-system (excluded by default pattern "kube-*")
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "kube-system"},
		Data:       map[string]string{"should": "not-be-synced"},
	}

	t.Cleanup(func() {
		_ = k8s.CoreV1().ConfigMaps("kube-system").Delete(ctx, name, metav1.DeleteOptions{})
	})

	_, err := k8s.CoreV1().ConfigMaps("kube-system").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create configmap in kube-system: %v", err)
	}

	// Wait then verify it was NOT synced
	time.Sleep(10 * time.Second)
	path := fmt.Sprintf("%s/kube-system/configmap/%s.yaml", clusterName, name)
	if giteaFileExists(path) {
		t.Errorf("resource in excluded namespace 'kube-system' was synced to git")
	}
}

// ---------------------------------------------------------------------------
// Tests — webhook endpoints (health & metrics)
// ---------------------------------------------------------------------------

func TestWebhookHealthEndpoint(t *testing.T) {
	ctx := context.Background()

	jobName := "e2e-health-" + shortID()
	backoff := int32(0)
	job := newCurlJob(jobName, webhookNS, fmt.Sprintf(
		"https://gitops-reverse-engineer.%s.svc.cluster.local:443/health", webhookNS))
	job.Spec.BackoffLimit = &backoff

	t.Cleanup(func() {
		bg := metav1.DeletePropagationBackground
		_ = k8s.BatchV1().Jobs(webhookNS).Delete(ctx, jobName, metav1.DeleteOptions{
			PropagationPolicy: &bg,
		})
	})

	_, err := k8s.BatchV1().Jobs(webhookNS).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create health-check job: %v", err)
	}

	if err := waitForJobComplete(ctx, webhookNS, jobName, 30*time.Second); err != nil {
		t.Fatalf("health check job failed: %v", err)
	}

	logs := jobLogs(ctx, t, webhookNS, jobName)
	if !strings.Contains(logs, "OK") {
		t.Errorf("expected 'OK' from /health, got: %s", logs)
	}
}

func TestWebhookMetricsEndpoint(t *testing.T) {
	ctx := context.Background()

	jobName := "e2e-metrics-" + shortID()
	backoff := int32(0)
	job := newCurlJob(jobName, webhookNS, fmt.Sprintf(
		"https://gitops-reverse-engineer.%s.svc.cluster.local:443/metrics", webhookNS))
	job.Spec.BackoffLimit = &backoff

	t.Cleanup(func() {
		bg := metav1.DeletePropagationBackground
		_ = k8s.BatchV1().Jobs(webhookNS).Delete(ctx, jobName, metav1.DeleteOptions{
			PropagationPolicy: &bg,
		})
	})

	_, err := k8s.BatchV1().Jobs(webhookNS).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create metrics-check job: %v", err)
	}

	if err := waitForJobComplete(ctx, webhookNS, jobName, 30*time.Second); err != nil {
		t.Fatalf("metrics check job failed: %v", err)
	}

	logs := jobLogs(ctx, t, webhookNS, jobName)
	if !strings.Contains(logs, "gitops_admission_git_sync_success_total") {
		t.Errorf("expected prometheus metric in output, got:\n%s", logs)
	}
}

// ---------------------------------------------------------------------------
// Gitea API helpers
// ---------------------------------------------------------------------------

func giteaGetFile(path string) (string, int, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s?ref=main",
		giteaURL, giteaOrg, giteaRepo, path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+giteaToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", resp.StatusCode, nil
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", resp.StatusCode, err
	}

	decoded, err := base64.StdEncoding.DecodeString(result.Content)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(decoded), 200, nil
}

func giteaFileExists(path string) bool {
	_, code, err := giteaGetFile(path)
	return err == nil && code == 200
}

func waitForGiteaFile(t *testing.T, path string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, code, err := giteaGetFile(path)
		if err == nil && code == 200 && content != "" {
			return content
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out waiting for file %s in Gitea", path)
	return ""
}

func waitForGiteaFileContent(t *testing.T, path, expected string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		content, code, err := giteaGetFile(path)
		if err == nil && code == 200 && strings.Contains(content, expected) {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out waiting for file %s to contain %q", path, expected)
}

func waitForGiteaFileGone(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, code, _ := giteaGetFile(path)
		if code == 404 {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out waiting for file %s to be deleted from Gitea", path)
}

// ---------------------------------------------------------------------------
// Kubernetes helpers
// ---------------------------------------------------------------------------

// newCurlJob creates a Job that fetches a URL with wget (busybox-based, no TLS verify).
func newCurlJob(name, ns, url string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "curl",
						Image:   "busybox:1.36",
						Command: []string{"wget", "--no-check-certificate", "-qO-", url},
					}},
				},
			},
		},
	}
}

func waitForJobComplete(ctx context.Context, ns, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := k8s.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, c := range job.Status.Conditions {
			if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
				return nil
			}
			if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
				return fmt.Errorf("job %s failed: %s", name, c.Message)
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for job %s", name)
}

func jobLogs(ctx context.Context, t *testing.T, ns, jobName string) string {
	t.Helper()
	pods, err := k8s.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		t.Fatalf("no pods for job %s: %v", jobName, err)
	}
	logStream, err := k8s.CoreV1().Pods(ns).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		t.Fatalf("get logs: %v", err)
	}
	defer logStream.Close()
	data, _ := io.ReadAll(logStream)
	return string(data)
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func shortID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%100000)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required env var %s not set\n", key)
		os.Exit(1)
	}
	return v
}
