# Simple Tiltfile for Go CLI development

# Build Go CLI binary locally for fast reloads
local_resource(
    'compile-cli',
    cmd='go build -o bin/cli ./cmd/cli',
    deps=['./cmd', './internal', 'go.mod', 'go.sum'],
    ignore=['./bin']
)

# Build and deploy to KIND
docker_build(
    'go-cli:latest',
    context='.',
    dockerfile_contents='''
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY bin/cli /cli
USER 65534
EXPOSE 8080
CMD ["/cli", "server", "start"]
''',
    only=['./bin/cli'],
    live_update=[
        sync('./bin/cli', '/cli')
    ]
)

# Deploy to Kubernetes (simple deployment)
k8s_yaml('''
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-cli
spec:
  replicas: 1
  selector:
    matchLabels:
      app: go-cli
  template:
    metadata:
      labels:
        app: go-cli
    spec:
      containers:
      - name: cli
        image: go-cli:latest
        ports:
        - containerPort: 8080
        env:
        - name: HTTP_PORT
          value: "8080"
        - name: LOG_LEVEL
          value: "info"
---
apiVersion: v1
kind: Service
metadata:
  name: go-cli
spec:
  selector:
    app: go-cli
  ports:
  - port: 80
    targetPort: 8080
''')

# Port forward to access locally
k8s_resource('go-cli', port_forwards='8080:8080')

print("🚀 Go CLI Development Environment")
print("   API: http://localhost:8080")
print("   Health: http://localhost:8080/health")
print("   Build and run: ./bin/cli server start")