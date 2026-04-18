# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
# TARGETOS and TARGETARCH are injected automatically by Docker Buildx
# when building with --platform (e.g., linux/amd64, linux/arm64).
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -installsuffix cgo -o admission-controller .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Set permissions for OpenShift compatibility
# Allow group to have read/write access, as OpenShift runs with a random user in the root group
RUN chmod -R g+rwx /root

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/admission-controller .

# Make the binary executable
RUN chmod +x ./admission-controller

# Create directory for certificates
RUN mkdir -p /etc/webhook/certs

# Expose the webhook port
EXPOSE 8443

# Run as a non-root user for security
USER 1001

# Run the admission controller
CMD ["./admission-controller"]
