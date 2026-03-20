# Agent Guidelines

## Project

ACP Chatbot — a Go application that bridges Slack to ACP-compatible coding agents.

**Module**: `github.com/lewisedginton/general_purpose_chatbot`

## Build & Test

```bash
go build ./...              # Build all packages
go test ./...               # Run all tests
golangci-lint run ./...     # Lint (config in .golangci.yml)
task lint                   # Same via Taskfile
task test                   # Same via Taskfile
```

## Key Packages

| Package | Purpose |
|---------|---------|
| `internal/acp` | ACP client core — process management, executor, router, client implementation |
| `internal/config` | Application configuration types and validation |
| `internal/connectors/slack` | Slack Socket Mode connector |
| `internal/server` | Server wiring and health check setup |
| `pkg/config` | Generic config loading (YAML + env vars) |
| `pkg/health` | Health check framework with HTTP handlers |

## Code Patterns

- **Logging**: `*zap.SugaredLogger` everywhere. Use `log.Infow("msg", "key", val)` for structured logging. Tests use `zap.NewNop().Sugar()`.
- **Testing**: Table-driven tests with `testify/assert` and `testify/require`. No mocks — tests use real types.
- **Configuration**: YAML with `yaml` struct tags. Env var override via `env` struct tags. Validation via `Validate()` methods.
- **ACP SDK**: `github.com/coder/acp-go-sdk` imported as `acp`. Internal package is `acpclient` to avoid name collision.

## Architecture

```
Slack (Socket Mode) → Connector → ACP Executor → Agent subprocess
```

The chatbot is pure glue — the ACP agent subprocess owns all LLM calls, tools, and MCP servers. The chatbot just forwards messages and collects responses.
