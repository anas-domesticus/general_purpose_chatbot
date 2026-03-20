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

# Final stage — needs a real OS to spawn agent subprocesses
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# Install Goose (Block's AI agent with ACP support)
# https://github.com/block/goose/releases
RUN GOOSE_VERSION="v1.0.22" && \
    curl -fsSL "https://github.com/block/goose/releases/download/${GOOSE_VERSION}/goose-x86_64-unknown-linux-gnu.tar.bz2" \
    | tar -xj -C /usr/local/bin goose && \
    chmod +x /usr/local/bin/goose

# Create non-root user with a home directory (agents may need ~/.config)
RUN useradd --create-home --shell /bin/bash chatbot
USER chatbot
WORKDIR /home/chatbot

# Copy chatbot binary
COPY --from=builder /app/chatbot /usr/local/bin/chatbot

# Health check endpoint
EXPOSE 8080

ENTRYPOINT ["chatbot"]