# Build stage
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s' \
    -o chatbot \
    ./cmd/chatbot/main.go

# Final stage — distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# CA certificates for HTTPS (Slack API, etc.)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/chatbot /chatbot

# Health check endpoint
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/chatbot"]