#!/bin/bash

set -e

echo "🏗️  Building GitOps Reversed Admission Controller Docker image..."

# Default values
IMAGE_NAME="gitops-reverse-engineer"
IMAGE_TAG="latest"
REGISTRY=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --registry)
      REGISTRY="$2"
      shift 2
      ;;
    --tag)
      IMAGE_TAG="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [--registry REGISTRY] [--tag TAG]"
      exit 1
      ;;
  esac
done

# Construct full image name
if [ -n "$REGISTRY" ]; then
  FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
else
  FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"
fi

echo "🐳 Building image: $FULL_IMAGE"

# Build the Docker image
docker build -t "$FULL_IMAGE" .

echo "✅ Image built successfully: $FULL_IMAGE"

# Optionally push if registry is provided
if [ -n "$REGISTRY" ]; then
  read -p "Push image to registry? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "⬆️  Pushing image to registry..."
    docker push "$FULL_IMAGE"
    echo "✅ Image pushed successfully"
  fi
fi

echo ""
echo "🎉 Build complete!"
echo ""
echo "To use this image in Kubernetes, update k8s/deployment.yaml with:"
echo "  image: $FULL_IMAGE"
