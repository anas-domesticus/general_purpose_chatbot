# Build stage
FROM golang:1.22-alpine AS builder

# Install necessary packages for building
RUN apk add --no-cache \
    ca-certificates \
    git \
    gcc \
    musl-dev

# Create a non-root user for the build process
RUN adduser -D -u 1001 appuser

# Set the working directory
WORKDIR /app

# Copy go module files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o chatbot \
    ./cmd/chatbot/main.go

# Verify the binary was created and is statically linked
RUN file chatbot
RUN ldd chatbot || true

# Final stage - minimal runtime image
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the non-root user from builder
COPY --from=builder /etc/passwd /etc/passwd

# Copy the built binary
COPY --from=builder /app/chatbot /chatbot

# Create necessary directories
COPY --from=builder --chown=1001:1001 /tmp /tmp

# Use non-root user
USER 1001

# Expose the default port (8080 for ADK)
EXPOSE 8080

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/chatbot", "health"] || exit 1

# Set the entrypoint
ENTRYPOINT ["/chatbot"]

# Default to web mode
CMD ["web"]

# Build arguments for version information (can be overridden at build time)
ARG VERSION=dev
ARG BUILD_TIME=""
ARG GIT_COMMIT=""

# Add labels for better container metadata
LABEL \
    org.opencontainers.image.title="General Purpose Chatbot" \
    org.opencontainers.image.description="Production-ready chatbot built with ADK-Go and Claude" \
    org.opencontainers.image.version="${VERSION}" \
    org.opencontainers.image.created="${BUILD_TIME}" \
    org.opencontainers.image.revision="${GIT_COMMIT}" \
    org.opencontainers.image.vendor="General Purpose Chatbot" \
    org.opencontainers.image.source="https://github.com/lewisedginton/general_purpose_chatbot"