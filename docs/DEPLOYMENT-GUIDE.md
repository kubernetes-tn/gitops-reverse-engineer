# Deployment Guide - Milestone 2
## GitOps Reversed Admission Controller

This guide provides step-by-step instructions for deploying the GitOps Reversed Admission Controller in Milestone 2 configuration.

---

## Prerequisites Checklist

- [ ] Kubernetes cluster (v1.16+) with admin access
- [ ] `kubectl` configured and working
- [ ] Docker installed for building images
- [ ] OpenSSL for certificate generation
- [ ] Git server accessible (e.g., GitHub, Gitea, GitLab)
- [ ] Git repository created for cluster state
- [ ] Git API token with `repo` scope

---

## Step 1: Prepare Git Repository

### 1.1 Create Repository in Gitea

1. Login to your Git server (GitHub, Gitea, GitLab, etc.)
2. Click "+" → "New Repository"
3. Repository Name: `cluster-state` (or your preference)
4. Organization/User: `gitops` (or your preference)
5. Visibility: **Private** (recommended for production)
6. Initialize: Yes (with README)
7. Click "Create Repository"

**Result:** Repository URL will be something like:
```
https://git.example.com/sre/cluster-git-reversed.git
```

### 1.2 Generate Gitea Token

1. In Gitea, go to Settings → Applications
2. Under "Generate New Token":
   - Token Name: `gitops-admission-controller`
   - Select Scopes: ✅ **repo** (all)
3. Click "Generate Token"
4. **Copy the token immediately** (you won't see it again)

Example token:
```
1234567890abcdef1234567890abcdef12345678
```

---

## Step 2: Prepare Kubernetes Cluster

### 2.1 Create Namespace

```bash
kubectl apply -f k8s/namespace.yaml
```

Verify:
```bash
kubectl get namespace verbose-api-log-system
```

### 2.2 Create Gitea Token Secret

Edit `k8s/secret.yaml`:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-token
  namespace: verbose-api-log-system
type: Opaque
stringData:
  token: "YOUR_ACTUAL_TOKEN_HERE"  # Replace with token from Step 1.2
```

Apply the secret:
```bash
kubectl apply -f k8s/secret.yaml
```

Verify:
```bash
kubectl get secret git-token -n verbose-api-log-system
```

---

## Step 3: Generate TLS Certificates

### 3.1 Run Certificate Generation Script

```bash
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh
```

This script will:
- Generate CA certificate and key
- Generate server certificate for the webhook
- Create Kubernetes secret `gitops-reverse-engineer-certs`
- Generate `k8s/webhook-configured.yaml` with CA bundle

### 3.2 Verify Certificate Secret

```bash
kubectl get secret gitops-reverse-engineer-certs -n verbose-api-log-system
```

Expected output:
```
NAME                               TYPE     DATA   AGE
gitops-reverse-engineer-certs    Opaque   2      10s
```

---

## Step 4: Build and Push Container Image

### 4.1 Update Go Dependencies

```bash
go mod tidy
```

This will download:
- `github.com/go-git/go-git/v5`
- `gopkg.in/yaml.v3`
- Kubernetes client libraries

### 4.2 Build Docker Image

```bash
chmod +x scripts/build.sh
./scripts/build.sh --registry ghcr.io/kubernetes-tn --tag v2.0.0
```

When prompted to push, select **Yes (y)**.

Alternative (manual):
```bash
docker build -t ghcr.io/kubernetes-tn/gitops-reverse-engineer:v2.0.0 .
docker push ghcr.io/kubernetes-tn/gitops-reverse-engineer:v2.0.0
```

### 4.3 Verify Image

```bash
docker images | grep gitops-reverse-engineer
```

---

## Step 5: Configure Deployment

### 5.1 Edit Deployment Configuration

Edit `k8s/deployment.yaml` and update environment variables:

```yaml
env:
- name: GIT_REPO_URL
  value: "https://git.example.com/sre/cluster-git-reversed.git"  # Your repo URL
- name: GIT_TOKEN
  valueFrom:
    secretKeyRef:
      name: git-token
      key: token
- name: CLUSTER_NAME
  value: "production-cluster"  # Your cluster name (e.g., prod, staging, dev)
```

### 5.2 Update Image Tag (if needed)

If you used a different tag:
```yaml
containers:
- name: admission-controller
  image: ghcr.io/kubernetes-tn/gitops-reverse-engineer:v2.0.0  # Your tag
```

---

## Step 6: Deploy to Kubernetes

### 6.1 Deploy Application Components

```bash
# Deploy the deployment
kubectl apply -f k8s/deployment.yaml

# Deploy the service
kubectl apply -f k8s/service.yaml
```

### 6.2 Wait for Pod to be Ready

```bash
kubectl wait --for=condition=ready pod \
  -l app=gitops-reverse-engineer \
  -n verbose-api-log-system \
  --timeout=120s
```

### 6.3 Check Pod Status

```bash
kubectl get pods -n verbose-api-log-system
```

Expected output:
```
NAME                                         READY   STATUS    RESTARTS   AGE
gitops-reverse-engineer-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
```

### 6.4 View Logs

```bash
kubectl logs -n verbose-api-log-system \
  -l app=gitops-reverse-engineer \
  -f
```

Expected log output:
```
🚀 Starting GitOps Reversed Admission Controller...
🔧 Initializing Git client for repo: https://git.example.com/sre/cluster-git-reversed.git
🔄 Initializing repository https://git.example.com/sre/cluster-git-reversed.git into /tmp/gitops-repo
✅ Successfully initialized Git repository
✅ Git client initialized successfully
✅ Admission controller listening on port :8443
📜 Using TLS cert: /etc/webhook/certs/tls.crt
🔑 Using TLS key: /etc/webhook/certs/tls.key
🏷️  Cluster name: production-cluster
```

---

## Step 7: Deploy Webhook Configuration

### 7.1 Apply Webhook Configuration

```bash
kubectl apply -f k8s/webhook-configured.yaml
```

### 7.2 Verify Webhook

```bash
kubectl get validatingwebhookconfiguration gitops-reverse-engineer-webhook
```

Check webhook details:
```bash
kubectl get validatingwebhookconfiguration gitops-reverse-engineer-webhook -o yaml
```

---

## Step 8: Enable GitOps for Namespaces

### 8.1 Label Target Namespaces

The controller only monitors namespaces with the label `gitops-reverse-engineer/enabled: "true"`.

Enable for `default` namespace:
```bash
kubectl label namespace default gitops-reverse-engineer/enabled="true"
```

Enable for other namespaces:
```bash
kubectl label namespace my-app-namespace gitops-reverse-engineer/enabled="true"
```

### 8.2 Verify Labels

```bash
kubectl get namespace default --show-labels
```

---

## Step 9: Test the Installation

### 9.1 Test CREATE Operation

```bash
# Create a test deployment
kubectl create deployment test-nginx --image=nginx -n default
```

### 9.2 Check Controller Logs

```bash
kubectl logs -n verbose-api-log-system -l app=gitops-reverse-engineer --tail=50
```

Expected output:
```
📝 Handling CREATE for /tmp/gitops-repo/production-cluster/default/deployment/test-nginx.yaml
✅ Successfully synced production-cluster/default/deployment/test-nginx.yaml to Git
✅ Allowed CREATE operation on Deployment/test-nginx in namespace default
```

### 9.3 Verify in Git Repository

1. Go to your Git repository
2. Navigate to `production-cluster/default/deployment/`
3. You should see `test-nginx.yaml`

### 9.4 Test UPDATE Operation

```bash
# Scale the deployment
kubectl scale deployment test-nginx --replicas=3 -n default
```

Check Git for updated file with `replicas: 3`.

### 9.5 Test DELETE Operation

```bash
# Delete the deployment
kubectl delete deployment test-nginx -n default
```

Verify the file is removed from Git repository.

---

## Step 10: Health Check

### 10.1 Check Health Endpoint

```bash
kubectl exec -n verbose-api-log-system \
  deployment/gitops-reverse-engineer -- \
  wget -O- https://localhost:8443/health --no-check-certificate
```

Expected output:
```
OK - Pending operations: 0
```

### 10.2 Monitor Pending Operations

If you see pending operations > 0, check:
- Git server connectivity
- Gitea token validity
- Repository permissions

---

## Step 11: Production Hardening

### 11.1 Resource Limits

Review and adjust based on your cluster size:
```yaml
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

### 11.2 Enable Monitoring

Add Prometheus annotations (optional):
```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8443"
    prometheus.io/path: "/metrics"
```

### 11.3 Backup Strategy

Git repository serves as backup, but also:
- Enable Gitea repository mirroring
- Set up automated Git repository backups
- Document recovery procedures

---

## Step 12: Post-Deployment Verification

### Checklist

- [ ] Pod is running and healthy
- [ ] Webhook configuration is active
- [ ] Test CREATE syncs to Git
- [ ] Test UPDATE syncs to Git
- [ ] Test DELETE removes from Git
- [ ] Health endpoint returns OK
- [ ] Pending operations count is 0
- [ ] No errors in pod logs
- [ ] Correct cluster name in Git paths
- [ ] Namespace filtering works

---

## Troubleshooting

### Issue: Pod not starting

**Check logs:**
```bash
kubectl describe pod -n verbose-api-log-system -l app=gitops-reverse-engineer
kubectl logs -n verbose-api-log-system -l app=gitops-reverse-engineer
```

**Common causes:**
- Missing or invalid GIT_TOKEN
- Invalid GIT_REPO_URL
- TLS certificates missing

### Issue: Not syncing to Git

**Check:**
```bash
# Verify secret exists
kubectl get secret git-token -n verbose-api-log-system

# Verify env vars
kubectl get deployment gitops-reverse-engineer -n verbose-api-log-system -o yaml | grep -A 10 env

# Check logs for Git errors
kubectl logs -n verbose-api-log-system -l app=gitops-reverse-engineer | grep -i "git\|error\|fail"
```

### Issue: High pending operations

**Diagnose:**
```bash
# Check health
kubectl exec -n verbose-api-log-system deployment/gitops-reverse-engineer -- \
  wget -O- https://localhost:8443/health --no-check-certificate

# Check logs for retry attempts
kubectl logs -n verbose-api-log-system -l app=gitops-reverse-engineer | grep "Retry\|pending"
```

**Possible causes:**
- Gitea server down
- Network connectivity issues
- Repository permissions issue
- Invalid credentials

### Issue: Webhook not intercepting

**Verify:**
```bash
# Check webhook config
kubectl get validatingwebhookconfiguration gitops-reverse-engineer-webhook -o yaml

# Check namespace labels
kubectl get namespace default --show-labels | grep verbose-api-log

# Check service
kubectl get svc -n verbose-api-log-system gitops-reverse-engineer
```

---

## Rollback Procedure

If you need to rollback:

```bash
# 1. Delete webhook configuration first (critical!)
kubectl delete validatingwebhookconfiguration gitops-reverse-engineer-webhook

# 2. Delete deployment
kubectl delete deployment gitops-reverse-engineer -n verbose-api-log-system

# 3. Delete service
kubectl delete service gitops-reverse-engineer -n verbose-api-log-system

# 4. Optionally delete secrets (be careful!)
kubectl delete secret git-token -n verbose-api-log-system
kubectl delete secret gitops-reverse-engineer-certs -n verbose-api-log-system

# 5. Remove namespace labels
kubectl label namespace default gitops-reverse-engineer/enabled-
```

---

## Next Steps

After successful deployment:

1. **Monitor Performance**
   - Watch resource usage
   - Monitor Git repository size
   - Track pending operations

2. **Configure Additional Namespaces**
   - Label namespaces to enable sync
   - Document which namespaces are monitored

3. **Set Up Alerting**
   - Alert on high pending operations
   - Alert on pod restarts
   - Alert on Git push failures

4. **Document Your Setup**
   - Record cluster-specific configuration
   - Document recovery procedures
   - Create runbooks for common issues

5. **Plan for Scale**
   - Monitor repository growth
   - Plan for repository cleanup/archival
   - Consider multi-cluster strategy

---

## Support and Maintenance

### Regular Maintenance

- **Weekly**: Check pending operations count
- **Monthly**: Review Git repository size
- **Quarterly**: Rotate Gitea token
- **Yearly**: Review and update resource limits

### Updating the Controller

```bash
# Build new version
./scripts/build.sh --registry ghcr.io/kubernetes-tn --tag v2.1.0

# Update deployment
kubectl set image deployment/gitops-reverse-engineer \
  admission-controller=ghcr.io/kubernetes-tn/gitops-reverse-engineer:v2.1.0 \
  -n verbose-api-log-system

# Watch rollout
kubectl rollout status deployment/gitops-reverse-engineer -n verbose-api-log-system
```

---

## Conclusion

You have successfully deployed the GitOps Reversed Admission Controller in Milestone 2 configuration. The controller is now:

- ✅ Intercepting resource operations
- ✅ Syncing to Git repository
- ✅ Handling failures gracefully
- ✅ Providing audit trail in Git

For questions or issues, refer to:
- `README-MILESTONE2.md` - User guide
- `DESIGN-MILESTONE2.md` - Architecture details
- Pod logs for debugging
