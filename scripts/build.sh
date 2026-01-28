#!/bin/bash

# Build script for General Purpose Chatbot
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
VERSION="${VERSION:-dev}"
BUILD_TIME="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
DOCKER_TAG="${DOCKER_TAG:-general-purpose-chatbot:${VERSION}}"

echo -e "${GREEN}Building General Purpose Chatbot${NC}"
echo -e "${YELLOW}Version: ${VERSION}${NC}"
echo -e "${YELLOW}Build Time: ${BUILD_TIME}${NC}"
echo -e "${YELLOW}Git Commit: ${GIT_COMMIT}${NC}"
echo ""

# Function to print section headers
print_section() {
    echo -e "${GREEN}=== $1 ===${NC}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
print_section "Checking Prerequisites"

if ! command_exists go; then
    echo -e "${RED}Error: Go is not installed${NC}"
    exit 1
fi

if ! command_exists docker; then
    echo -e "${RED}Error: Docker is not installed${NC}"
    exit 1
fi

GO_VERSION=$(go version | cut -d' ' -f3)
echo "Go version: $GO_VERSION"

DOCKER_VERSION=$(docker --version | cut -d' ' -f3 | cut -d',' -f1)
echo "Docker version: $DOCKER_VERSION"
echo ""

# Verify go.mod exists
if [ ! -f "go.mod" ]; then
    echo -e "${RED}Error: go.mod not found. Please run from project root.${NC}"
    exit 1
fi

# Run tests
print_section "Running Tests"
if go test ./... -v; then
    echo -e "${GREEN}All tests passed${NC}"
else
    echo -e "${RED}Tests failed${NC}"
    exit 1
fi
echo ""

# Run linting (if golangci-lint is available)
if command_exists golangci-lint; then
    print_section "Running Linter"
    if golangci-lint run; then
        echo -e "${GREEN}Linting passed${NC}"
    else
        echo -e "${YELLOW}Linting issues found, but continuing with build${NC}"
    fi
    echo ""
fi

# Build binary locally for testing
print_section "Building Binary"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o bin/chatbot \
    ./cmd/chatbot/main.go

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Binary built successfully${NC}"
    ls -lh bin/chatbot
else
    echo -e "${RED}Binary build failed${NC}"
    exit 1
fi
echo ""

# Build Docker image
print_section "Building Docker Image"
docker build \
    --build-arg VERSION="${VERSION}" \
    --build-arg BUILD_TIME="${BUILD_TIME}" \
    --build-arg GIT_COMMIT="${GIT_COMMIT}" \
    -t "${DOCKER_TAG}" \
    .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Docker image built successfully${NC}"
    docker images | grep general-purpose-chatbot | head -5
else
    echo -e "${RED}Docker build failed${NC}"
    exit 1
fi
echo ""

# Run security scan (if available)
if command_exists docker && docker images | grep -q "general-purpose-chatbot"; then
    print_section "Running Security Scan"
    
    # Try different security scanners
    if command_exists grype; then
        echo "Running Grype security scan..."
        grype "${DOCKER_TAG}" || echo -e "${YELLOW}Grype scan completed with issues${NC}"
    elif command_exists trivy; then
        echo "Running Trivy security scan..."
        trivy image "${DOCKER_TAG}" || echo -e "${YELLOW}Trivy scan completed with issues${NC}"
    else
        echo -e "${YELLOW}No security scanner available (grype or trivy recommended)${NC}"
    fi
    echo ""
fi

# Test Docker image
print_section "Testing Docker Image"
if docker run --rm "${DOCKER_TAG}" health >/dev/null 2>&1; then
    echo -e "${GREEN}Docker image health check passed${NC}"
else
    echo -e "${YELLOW}Docker image health check failed (this is expected if health endpoint needs external dependencies)${NC}"
fi
echo ""

# Generate deployment artifacts
print_section "Generating Deployment Artifacts"

# Create deployment directory
mkdir -p deploy/

# Generate docker-compose.prod.yml
cat > deploy/docker-compose.prod.yml << EOF
version: '3.8'

services:
  chatbot:
    image: ${DOCKER_TAG}
    restart: unless-stopped
    ports:
      - "\${PORT:-8080}:8080"
    environment:
      - ANTHROPIC_API_KEY=\${ANTHROPIC_API_KEY}
      - CLAUDE_MODEL=\${CLAUDE_MODEL:-claude-3-5-sonnet-20241022}
      - LOG_LEVEL=\${LOG_LEVEL:-info}
      - LOG_FORMAT=json
      - ENVIRONMENT=production
      - SERVICE_NAME=general-purpose-chatbot
    healthcheck:
      test: ["/chatbot", "health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 128M
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=100m

networks:
  default:
    driver: bridge
EOF

# Generate Kubernetes deployment
cat > deploy/k8s-deployment.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: general-purpose-chatbot
  labels:
    app: general-purpose-chatbot
    version: "${VERSION}"
spec:
  replicas: 3
  selector:
    matchLabels:
      app: general-purpose-chatbot
  template:
    metadata:
      labels:
        app: general-purpose-chatbot
        version: "${VERSION}"
    spec:
      containers:
      - name: chatbot
        image: ${DOCKER_TAG}
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: chatbot-secrets
              key: anthropic-api-key
        - name: LOG_LEVEL
          value: "info"
        - name: LOG_FORMAT
          value: "json"
        - name: ENVIRONMENT
          value: "production"
        resources:
          requests:
            memory: "128Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1001
---
apiVersion: v1
kind: Service
metadata:
  name: general-purpose-chatbot-service
spec:
  selector:
    app: general-purpose-chatbot
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
EOF

echo -e "${GREEN}Deployment artifacts generated in deploy/ directory${NC}"
echo ""

print_section "Build Summary"
echo -e "Version: ${GREEN}${VERSION}${NC}"
echo -e "Docker Tag: ${GREEN}${DOCKER_TAG}${NC}"
echo -e "Binary Size: ${GREEN}$(du -h bin/chatbot | cut -f1)${NC}"
echo -e "Docker Image Size: ${GREEN}$(docker images --format "table {{.Size}}" ${DOCKER_TAG} | tail -1)${NC}"
echo ""

echo -e "${GREEN}Build completed successfully!${NC}"
echo ""
echo "Next steps:"
echo "1. Test locally: docker run --rm -p 8080:8080 -e ANTHROPIC_API_KEY=your_key ${DOCKER_TAG} web"
echo "2. Push to registry: docker push ${DOCKER_TAG}"
echo "3. Deploy with: docker-compose -f deploy/docker-compose.prod.yml up -d"
echo "4. Or deploy to Kubernetes: kubectl apply -f deploy/k8s-deployment.yaml"