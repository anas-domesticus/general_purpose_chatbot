# ACP Chatbot

A Slack-to-ACP bridge that connects Slack to any [ACP-compatible](https://agentclientprotocol.com/) coding agent — Goose, Claude Code, OpenCode, and others.

> **⚠️ Active Development** — API and configuration are subject to change.

## What It Does

This tool is pure glue: it receives messages from Slack, forwards them to an ACP agent subprocess over JSON-RPC/stdio, and sends the response back. The agent handles all LLM calls, tools, and MCP servers.

```
Slack (Socket Mode) → Connector → ACP Executor → Agent subprocess (Goose, Claude Code, etc.)
```

## Features

- **Channel-to-agent mapping** — different Slack channels can use different agents
- **Multi-agent support** — configure multiple agents (e.g. Goose with Claude, Goose with GPT-4o)
- **Thread-scoped sessions** — each Slack thread gets its own ACP session
- **DM support** — direct messages use the default agent
- **MCP server forwarding** — pass MCP server configs to agents via ACP session setup
- **Auto-approve permissions** — headless mode with `PermissionFunc` hook for future interactive flows
- **Health checks** — HTTP liveness/readiness endpoints for Kubernetes
## Quick Start

### Prerequisites

- Go 1.24+
- An ACP-compatible agent installed (e.g. [Goose](https://github.com/block/goose), [Claude Code](https://docs.anthropic.com/en/docs/claude-code))
- A Slack app with Socket Mode enabled ([setup guide](#slack-app-setup))

### Run

```bash
# Set Slack tokens
export SLACK_BOT_TOKEN=xoxb-your-bot-token
export SLACK_APP_TOKEN=xapp-your-app-token
export ANTHROPIC_API_KEY=your-api-key  # or whatever your agent needs

# Create config.yaml (see docs/examples/config-acp.yaml for full example)
# Run
go run ./cmd/chatbot -config config.yaml
```

### Docker

```bash
docker compose up
```

## Configuration

See [`docs/examples/config-acp.yaml`](docs/examples/config-acp.yaml) for a fully commented example.

```yaml
acp:
  default_agent: goose
  cwd: /home/user/projects
  agents:
    goose:
      command: goose
      args: ["acp"]
      env:
        GOOSE_PROVIDER: "anthropic"
        ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
  channels:
    C0123456789:          # #dev channel → uses goose agent
      agent: goose
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SLACK_BOT_TOKEN` | Slack bot token (`xoxb-...`) |
| `SLACK_APP_TOKEN` | Slack app token (`xapp-...`) |
| `ANTHROPIC_API_KEY` | Passed through to agent subprocess |
| `OPENAI_API_KEY` | Passed through to agent subprocess |

### Slack App Setup

1. Create app at https://api.slack.com/apps
2. Enable **Socket Mode** → generate app token (`xapp-...`)
3. **Bot Token Scopes**: `app_mentions:read`, `chat:write`, `channels:history`, `im:history`
4. **Event Subscriptions**: `app_mention`, `message.im`
5. Install to workspace → copy bot token (`xoxb-...`)

## Health Checks

When `health.enabled: true` (default), an HTTP server runs on port 8080:

- `GET /health/live` — liveness probe (process alive)
- `GET /health/ready` — readiness probe (Slack connected)

## Project Structure

```
cmd/chatbot/main.go              # Entry point
internal/
  acp/                           # ACP client core
    client.go                    #   acp.Client implementation (notifications, permissions)
    executor.go                  #   Sends prompts, collects responses
    process.go                   #   Agent subprocess lifecycle
    router.go                    #   Channel → agent config resolution
    mcp.go                       #   MCP server config conversion
    types.go                     #   Request/Response types
  config/                        # Application configuration
  connectors/slack/              # Slack Socket Mode connector
  server/                        # Server wiring and health checks
pkg/
  config/                        # Generic config loading (YAML + env vars)
  health/                        # Health check framework
```

## Development

```bash
task lint       # Run golangci-lint
task test       # Run tests
task build      # Build the binary
task test:race  # Run tests with race detection
```

## Contributing

Contributions welcome.
