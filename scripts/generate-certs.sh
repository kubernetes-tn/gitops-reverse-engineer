#!/bin/bash

set -e

# Determine the script's directory to reliably locate other files
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
VALUES_FILE="${SCRIPT_DIR}/chart/values.yaml"

# Configuration - these should match your Helm release name and namespace
# You can override these with command-line arguments
RELEASE_NAME="${1:-gitops-reverse-engineer}"
NAMESPACE="${2:-gitops-reverse-engineer-system}"

# Derive service name from Helm template (fullname pattern)
SERVICE_NAME="${RELEASE_NAME}"

# Extract secret name from values.yaml (default: gitops-reverse-engineer-certs)
SECRET_NAME=$(grep -A1 "tls:" "$VALUES_FILE" | grep "existingSecret:" | awk '{print $2}' | tr -d '"' || echo "${RELEASE_NAME}-certs")

echo "🔐 Generating TLS certificates for admission webhook..."
echo "📋 Configuration:"
echo "   Release Name: $RELEASE_NAME"
echo "   Namespace: $NAMESPACE"
echo "   Service Name: $SERVICE_NAME"
echo "   Secret Name: $SECRET_NAME"
echo ""

# Create a temporary directory for certificates
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

echo "📁 Working directory: $TEMP_DIR"

# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate
openssl req -x509 -new -nodes -key ca.key -subj "/CN=gitops-reverse-engineer-ca" -days 365 -out ca.crt

# Generate server private key
openssl genrsa -out tls.key 2048

# Create certificate signing request (CSR)
cat > csr.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

# Generate CSR
openssl req -new -key tls.key -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" -config csr.conf -out tls.csr

# Generate server certificate
openssl x509 -req -in tls.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 365 -extensions v3_req -extfile csr.conf

echo "✅ Certificates generated successfully"

# Create namespace if it doesn't exist
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# Create Kubernetes secret with the certificates
kubectl create secret generic "$SECRET_NAME" \
  --from-file=tls.key=tls.key \
  --from-file=tls.crt=tls.crt \
  --namespace="$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "✅ Secret '$SECRET_NAME' created in namespace '$NAMESPACE'"

# Get the CA bundle and encode it
CA_BUNDLE=$(cat ca.crt | base64 | tr -d '\n')

echo "✅ CA Bundle generated"
echo ""
echo "🎉 Certificate generation complete!"
echo ""
echo "📝 Next steps:"
echo ""
echo "1. Update your Helm values file with the CA bundle:"
echo "   Add the following to your values.yaml or values-override.yaml:"
echo ""
echo "   tls:"
echo "     existingSecret: \"${SECRET_NAME}\""
echo "     crt: |"
cat ca.crt | sed 's/^/       /'
echo ""
echo "2. Or set it via command line during deployment:"
echo "   helm template ${RELEASE_NAME} ./chart \\"
echo "     --set tls.existingSecret=${SECRET_NAME} \\"
echo "     --set-file tls.crt=${TEMP_DIR}/ca.crt \\"
echo "     --namespace ${NAMESPACE} | kubectl apply -f -"
echo ""
echo "3. Or use the CA bundle value directly:"
echo "   helm template ${RELEASE_NAME} ./chart \\"
echo "     --set tls.existingSecret=${SECRET_NAME} \\"
echo "     --set tls.crt=${CA_BUNDLE} \\"
echo "     --namespace ${NAMESPACE} | kubectl apply -f -"
echo ""
echo "4. Build and push your Docker image"
echo "5. Deploy using make:"
echo "   make deploy TAG=your-tag"
echo ""

# Cleanup
cd -
rm -rf "$TEMP_DIR"
