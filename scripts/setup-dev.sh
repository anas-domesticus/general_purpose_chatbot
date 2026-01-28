#!/bin/bash

# Setup script for Go development environment
# Installs golangci-lint for linting

set -e

echo "🔧 Setting up Go linter..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi

echo "✅ Go version: $(go version)"

# Install golangci-lint
echo "📦 Installing golangci-lint..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify installation
echo ""
echo "🔍 Verifying installation..."
command -v golangci-lint &> /dev/null && echo "✅ golangci-lint: $(golangci-lint version --format short)" || echo "❌ golangci-lint not found"

echo ""
echo "🎉 Linter setup complete!"
echo ""
echo "Usage:"
echo "  task lint      - Run linter"
echo "  task lint:fix  - Run linter with fixes"
echo ""