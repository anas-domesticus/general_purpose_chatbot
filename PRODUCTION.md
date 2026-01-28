# Production Deployment Guide

This guide covers production deployment of the General Purpose Chatbot with proper monitoring, error handling, and scalability considerations.

## 🏗️ Architecture Overview

The production setup includes:

- **Health Check Endpoints** (`/health`, `/health/live`, `/health/ready`)
- **Structured Logging** with correlation IDs and request tracing
- **Error Recovery Middleware** with panic recovery and circuit breakers
- **Graceful Shutdown** with proper resource cleanup
- **Configuration Management** with validation
- **Docker Support** with multi-stage builds and security hardening

## 🚀 Quick Start

### 1. Configure Environment

```bash
# Copy configuration template
cp .env.example .env

# Edit with your values
vi .env
```

**Required Configuration:**
```bash
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

### 2. Build and Deploy

```bash
# Build everything (tests, binary, Docker image)
./scripts/build.sh

# Run locally with Docker
docker-compose up -d

# Or deploy to production
docker-compose -f deploy/docker-compose.prod.yml up -d
```

### 3. Verify Deployment

```bash
# Check health
curl http://localhost:8080/health

# Check specific probes
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready

# Test chat functionality
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello, how are you?"}'
```

## 📊 Monitoring

### Health Check Endpoints

| Endpoint | Purpose | Use Case |
|----------|---------|----------|
| `/health` | Combined health status | General monitoring |
| `/health/live` | Liveness probe | Kubernetes liveness checks |
| `/health/ready` | Readiness probe | Load balancer health checks |

### Health Check Response Format

```json
{
  "status": "healthy",
  "timestamp": "2024-01-28T16:30:00Z",
  "uptime": "2h15m30s",
  "version": "v1.0.0",
  "liveness": {
    "status": "healthy",
    "checks": [
      {
        "name": "process",
        "healthy": true,
        "latency": "1ms"
      }
    ]
  },
  "readiness": {
    "status": "ready",
    "checks": [
      {
        "name": "anthropic_api",
        "healthy": true,
        "latency": "150ms"
      }
    ]
  }
}
```

### Metrics and Observability

The application provides structured JSON logging with:

- **Request Tracing**: Every request gets a correlation ID
- **Performance Metrics**: Request duration, response codes, error rates
- **Error Context**: Stack traces, retry attempts, circuit breaker states
- **Resource Usage**: Memory, CPU, connection pools (if applicable)

Example log entry:
```json
{
  "level": "info",
  "timestamp": "2024-01-28T16:30:00Z",
  "service": "general-purpose-chatbot",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "http_method": "POST",
  "http_path": "/api/chat",
  "http_status": 200,
  "duration": "2.5s",
  "client_ip": "192.168.1.100",
  "response_size": 1024,
  "message": "HTTP request completed"
}
```

## 🔧 Configuration

### Environment Variables

See [.env.example](.env.example) for all available configuration options.

### Key Configuration Areas

#### 1. Anthropic/Claude Settings
```bash
CLAUDE_MODEL=claude-3-5-sonnet-20241022
ANTHROPIC_MAX_RETRIES=3
ANTHROPIC_INITIAL_BACKOFF=1s
ANTHROPIC_MAX_BACKOFF=10s
```

#### 2. Performance Tuning
```bash
REQUEST_TIMEOUT=30s
MAX_REQUEST_SIZE=10485760  # 10MB
RATE_LIMIT_RPS=100
```

#### 3. Security
```bash
CORS_ALLOWED_ORIGINS=https://yourdomain.com
RATE_LIMIT_ENABLED=true
```

#### 4. Monitoring
```bash
LOG_LEVEL=info
LOG_FORMAT=json
HEALTH_CHECK_TIMEOUT=10s
METRICS_ENABLED=true
```

## 🐳 Docker Deployment

### Multi-Stage Build

The Dockerfile uses a multi-stage build for:
- **Small image size** (using `scratch` base image)
- **Security hardening** (non-root user, read-only filesystem)
- **Static binary** (no external dependencies)

### Security Features

- Runs as non-root user (UID 1001)
- Read-only root filesystem
- No shell access
- Minimal attack surface

### Resource Limits

Default Docker Compose limits:
```yaml
deploy:
  resources:
    limits:
      cpus: '1.0'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 128M
```

## ☸️ Kubernetes Deployment

### Apply Kubernetes Configuration

```bash
# Create secrets
kubectl create secret generic chatbot-secrets \
  --from-literal=anthropic-api-key=your-key-here

# Deploy application
kubectl apply -f deploy/k8s-deployment.yaml
```

### Kubernetes Features

- **3 replicas** for high availability
- **Health checks** for liveness and readiness probes
- **Resource limits** and requests
- **Security context** with read-only filesystem
- **Service** for internal communication

### Scaling

```bash
# Scale up
kubectl scale deployment general-purpose-chatbot --replicas=5

# Scale down
kubectl scale deployment general-purpose-chatbot --replicas=1
```

## 🔍 Error Handling

### Circuit Breaker Pattern

The application implements circuit breakers for external dependencies:

- **Failure Threshold**: 5 consecutive failures
- **Reset Timeout**: 30 seconds
- **States**: Closed (normal) → Open (failing) → Half-Open (testing)

### Retry Logic

Automatic retries with exponential backoff:
- **Max Retries**: 3 attempts
- **Initial Backoff**: 1 second
- **Max Backoff**: 10 seconds
- **Retry Conditions**: Network errors, 5xx responses, timeouts

### Panic Recovery

All HTTP requests are protected by recovery middleware:
- **Panic Recovery**: Prevents application crashes
- **Stack Trace Logging**: Full context for debugging
- **Graceful Error Responses**: Client-friendly error messages

## 📈 Performance Optimization

### Production Optimizations

1. **Connection Pooling**: HTTP client reuse
2. **Request Timeouts**: Prevent resource exhaustion  
3. **Rate Limiting**: Protect against abuse
4. **Memory Management**: Efficient JSON parsing
5. **Static Binary**: Fast startup times

### Monitoring Performance

Key metrics to monitor:
- **Response Time**: 95th percentile < 3 seconds
- **Error Rate**: < 1% of requests
- **Availability**: > 99.9% uptime
- **Memory Usage**: < 400MB per instance
- **CPU Usage**: < 80% under normal load

## 🚨 Troubleshooting

### Common Issues

#### 1. Health Checks Failing

```bash
# Check logs
kubectl logs deployment/general-purpose-chatbot

# Check specific health endpoints
curl -v http://localhost:8080/health/ready
```

#### 2. Anthropic API Issues

```bash
# Check API connectivity
curl -H "x-api-key: $ANTHROPIC_API_KEY" \
  https://api.anthropic.com/v1/messages

# Review retry logs
grep "anthropic api call failed" /app/logs/app.log
```

#### 3. High Memory Usage

```bash
# Check container stats
docker stats general-purpose-chatbot

# Analyze heap dumps (if available)
go tool pprof http://localhost:8080/debug/pprof/heap
```

### Log Analysis

#### Key Log Patterns

**Successful Request:**
```bash
grep "HTTP request completed" /app/logs/app.log | grep "http_status\":200"
```

**Failed Requests:**
```bash
grep "HTTP request completed" /app/logs/app.log | grep -v "http_status\":200"
```

**Circuit Breaker Events:**
```bash
grep "circuit breaker" /app/logs/app.log
```

**Retry Attempts:**
```bash
grep "anthropic api call failed, retrying" /app/logs/app.log
```

## 🔐 Security Considerations

### Security Checklist

- [ ] **API Key Security**: Store in secrets, not environment variables
- [ ] **Network Security**: Use HTTPS in production
- [ ] **Container Security**: Non-root user, read-only filesystem
- [ ] **CORS Configuration**: Restrict allowed origins
- [ ] **Rate Limiting**: Prevent abuse and DoS
- [ ] **Input Validation**: Sanitize user inputs
- [ ] **Dependency Updates**: Regular security updates

### Security Scanning

Run security scans on Docker images:

```bash
# Using Grype
grype general-purpose-chatbot:latest

# Using Trivy  
trivy image general-purpose-chatbot:latest
```

## 📋 Maintenance

### Regular Tasks

1. **Update Dependencies**: Monthly Go module updates
2. **Security Patches**: Weekly base image updates
3. **Log Rotation**: Daily log cleanup
4. **Performance Review**: Weekly metrics analysis
5. **Backup Configuration**: Version control all configs

### Version Updates

```bash
# Update to new version
export VERSION=v1.1.0
./scripts/build.sh
docker-compose -f deploy/docker-compose.prod.yml up -d
```

## 🆘 Support

### Getting Help

1. **Check Logs**: Application logs contain detailed error information
2. **Health Endpoints**: Use health checks for status verification
3. **Metrics**: Monitor key performance indicators
4. **Documentation**: Refer to code comments and README files

### Reporting Issues

When reporting issues, include:
- Application version
- Environment configuration  
- Error logs with correlation IDs
- Steps to reproduce
- Expected vs actual behavior

---

For development setup and contribution guidelines, see [README.md](README.md).