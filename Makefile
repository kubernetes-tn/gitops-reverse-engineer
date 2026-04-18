MAKEFLAGS += --silent

.PHONY: help build push deploy delete logs test clean cert e2e-setup e2e-test e2e-teardown

help:
	@echo "GitOps Reversed Admission Controller"
	@echo ""
	@echo "Available targets:"
	@echo "  make cert        - Generate TLS certificates"
	@echo "  make build       - Build Docker image"
	@echo "  make push        - Push Docker image to registry"
	@echo "  make deploy      - Deploy to Kubernetes"
	@echo "  make delete      - Remove from Kubernetes"
	@echo "  make logs        - View admission controller logs"
	@echo "  make test        - Test the admission controller"
	@echo "  make clean       - Clean up generated files"
	@echo "  make e2e-setup   - Set up e2e test environment (Kind + Gitea + webhook)"
	@echo "  make e2e-test    - Run e2e tests"
	@echo "  make e2e-teardown - Tear down e2e environment"
	@echo ""
	@echo "Variables:"
	@echo "  REGISTRY         - Docker registry (default: ghcr.io/kubernetes-tn)"
	@echo "  TAG              - Image tag (default: latest)"
	@echo "  CLUSTER_NAME     - Cluster name for Git path structure (required for deploy)"
	@echo ""
	@echo "Multi-Cluster Support:"
	@echo "  The deploy command automatically uses values/values.CLUSTER_NAME.yaml if it exists."
	@echo "  This allows per-cluster configuration overrides in a declarative way."
	@echo ""
	@echo "  Example structure:"
	@echo "    values/values.production.yaml  - Production cluster overrides"
	@echo "    values/values.staging.yaml     - Staging cluster overrides"
	@echo "    values/values.dev.yaml         - Development cluster overrides"
	@echo ""
	@echo "Example:"
	@echo "  make build TAG=v1.0.0"
	@echo "  make deploy CLUSTER_NAME=production TAG=v1.0.0"
	@echo "  make deploy CLUSTER_NAME=staging TAG=v1.0.0"

IMAGE_NAME := gitops-reverse-engineer
TAG ?= latest
REGISTRY ?= ghcr.io/kubernetes-tn
NAMESPACE := gitops-reverse-engineer-system
RELEASE_NAME := gitops-reverse-engineer
CLUSTER_NAME ?= $(error CLUSTER_NAME is required. Usage: make deploy CLUSTER_NAME=your-cluster TAG=your-tag)

# Full image reference
IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(TAG)

cert:
	@echo "🔐 Generating certificates with Helm integration..."
	chmod +x scripts/generate-certs-helm.sh
	./scripts/generate-certs-helm.sh $(RELEASE_NAME) $(NAMESPACE)
	@echo ""
	@echo "✅ Certificates ready for Helm deployment"

build:
	@echo "🏗️  Building Docker image: $(IMAGE)"
	docker build -t $(IMAGE) .
	@echo "✅ Build complete"

push: build
	@echo "⬆️  Pushing image: $(IMAGE)"
	docker push $(IMAGE)
	@echo "✅ Push complete"

deploy:
	@echo "🚀 Deploying to Kubernetes using Helm..."
	@echo "📝 Cluster: $(CLUSTER_NAME)"
	@echo "📝 Updating deployment image to $(IMAGE)..."
	@echo "🔧 Using helm template to generate manifests..."
	@# Build Helm values file arguments
	@VALUES_FILES=""; \
	if [ -f values-certs-override.yaml ]; then \
		echo "📄 Using values-certs-override.yaml for TLS configuration"; \
		VALUES_FILES="$$VALUES_FILES -f values-certs-override.yaml"; \
	else \
		echo "⚠️  Warning: values-certs-override.yaml not found. Run 'make cert' first!"; \
	fi; \
	if [ -f "values/values.$(CLUSTER_NAME).yaml" ]; then \
		echo "📄 Using cluster-specific values from values/values.$(CLUSTER_NAME).yaml"; \
		VALUES_FILES="$$VALUES_FILES -f values/values.$(CLUSTER_NAME).yaml"; \
	else \
		echo "ℹ️  No cluster-specific values file found at values/values.$(CLUSTER_NAME).yaml"; \
	fi; \
	helm template $(RELEASE_NAME) ./chart \
		--namespace $(NAMESPACE) \
		--set image.registry=$(REGISTRY) \
		--set image.repository=$(IMAGE_NAME) \
		--set image.tag=$(TAG) \
		--set git.clusterName=$(CLUSTER_NAME) \
		$$VALUES_FILES \
		| kubectl apply -f -
	@echo "⏳ Waiting for deployment to be ready..."
	kubectl rollout status deployment/$(RELEASE_NAME) -n $(NAMESPACE) || true
	@echo "✅ Deployment complete"
	@echo ""
	@$(MAKE) logs

delete:
	@echo "🗑️  Deleting admission controller..."
	-helm template $(RELEASE_NAME) ./chart --namespace $(NAMESPACE) | kubectl delete -f -
	-kubectl delete namespace $(NAMESPACE)
	@echo "✅ Deleted"

LOG_NAMESPACE := gitops-reverse-engineer-system
TEST_NAMESPACE ?= default

logs:
	@echo "📋 Viewing logs in namespace '$(LOG_NAMESPACE)' (Ctrl+C to exit)..."
	kubectl logs -n $(LOG_NAMESPACE) -l app.kubernetes.io/name=gitops-reverse-engineer -f

test:
	@echo "🧪 Testing admission controller in namespace: $(TEST_NAMESPACE)..."
	@echo ""
	@echo "Creating test configmap..."
	kubectl create configmap test-admission-cm -n $(TEST_NAMESPACE) --from-literal=test=hello --dry-run=client -o yaml | kubectl apply -f -
	@echo ""
	@echo "Creating test deployment..."
	kubectl create deployment test-admission-deploy -n $(TEST_NAMESPACE) --image=nginx --dry-run=client -o yaml | kubectl apply -f -
	@echo ""
	@echo "Scaling test deployment..."
	kubectl scale deployment test-admission-deploy -n $(TEST_NAMESPACE) --replicas=2
	@echo ""
	@echo "✅ Test resources created. Check logs with 'make logs'"
	@echo ""
	@echo "To clean up test resources:"
	@echo "  kubectl delete configmap test-admission-cm -n $(TEST_NAMESPACE)"
	@echo "  kubectl delete deployment test-admission-deploy -n $(TEST_NAMESPACE)"

clean:
	@echo "🧹 Cleaning up..."
	rm -f gitops-reverse-engineer
	rm -f values-certs-override.yaml
	rm -f ca.crt
	rm -f k8s/webhook-configured.yaml
	@echo "✅ Clean complete"

e2e-setup:
	@echo "🧪 Setting up e2e test environment..."
	chmod +x e2e/setup.sh
	./e2e/setup.sh
	@echo "✅ E2E environment ready. Run 'make e2e-test' to execute tests."

e2e-test:
	@echo "🧪 Running e2e tests..."
	@if [ ! -f e2e/.env ]; then echo "❌ Run 'make e2e-setup' first"; exit 1; fi
	set -a && . e2e/.env && set +a && \
		go test -v -tags=e2e -count=1 -timeout=5m ./e2e/...

e2e-teardown:
	@echo "🧹 Tearing down e2e environment..."
	chmod +x e2e/teardown.sh
	./e2e/teardown.sh

.DEFAULT_GOAL := help
