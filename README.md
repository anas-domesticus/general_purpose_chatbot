# General Purpose Chatbot Framework

A modular, extensible agent framework built in Go for connecting chat platforms to Claude-powered conversational agents.

## What It Is

This framework bridges chat platforms (Slack, Telegram) with Claude AI using Google's Agent Development Kit (ADK). Unlike traditional command-based ChatOps tools, it enables natural language conversations with AI agents that can integrate with external tools via MCP (Model Context Protocol) servers.

### Architecture

```
Chat Platform (Slack/Telegram)
        ↓
    Connector (handles platform-specific messaging)
        ↓
    Executor (routes messages to agents)
        ↓
    Agent Factory (creates Claude-powered agents)
        ↓
    MCP Tools (filesystem, database, HTTP, custom)
```

## Key Features

- **Claude AI Integration** - Powered by Anthropic's Claude with custom LLM implementation
- **Multi-Platform Support** - Slack (Socket Mode) and Telegram connectors with pluggable architecture
- **MCP Tool Ecosystem** - Connect to any Model Context Protocol server for extended capabilities
- **Session Management** - Persistent conversations with local, S3, or in-memory storage backends
- **Customizable Agents** - Configure agent behavior via `system.md` prompt files
- **Production Ready** - Structured logging, Prometheus metrics, health checks, graceful shutdown

## Use Cases

### DevOps & Kubernetes Operations
Query logs, check pod status, and troubleshoot issues through natural conversation instead of remembering complex kubectl commands.

> "Show me the logs from the payment service in the last hour"
> "Which pods are in CrashLoopBackOff?"
> "Scale the frontend deployment to 5 replicas"

### Internal Tooling Assistant
Connect your internal APIs and databases to give teams a conversational interface to company systems.

> "What's the status of order #12345?"
> "Create a new support ticket for customer Acme Corp"
> "Show me sales metrics for last quarter"

### Document & Knowledge Base Search
Integrate with filesystem or database MCP servers to search and retrieve information.

> "Find the architecture docs for the auth service"
> "What's our policy on PTO requests?"

### Multi-Tool Workflows
Chain multiple MCP tools together for complex operations.

> "Check if the API is healthy, and if not, show me the recent error logs"

## Quick Start

```bash
# 1. Set your API keys
export ANTHROPIC_API_KEY="sk-ant-your-api-key"
export SLACK_BOT_TOKEN="xoxb-your-bot-token"
export SLACK_APP_TOKEN="xapp-your-app-token"

# 2. Create a system prompt
cat > system.md << 'EOF'
You are a helpful DevOps assistant. You help engineers query logs,
check system status, and troubleshoot issues.
EOF

# 3. Run the chatbot
./chatbot --config config.yaml
```

## Configuration

### Agent Behavior (`system.md`)

The agent's personality and capabilities are defined in a `system.md` file:

```markdown
# Your Custom Agent

You are a specialized AI assistant for [your use case].

## Capabilities
- You can access the filesystem via MCP tools
- You can query the database for customer information

## Guidelines
- Always confirm before making changes
- Provide concise, actionable responses
```

Place this file in the working directory or mount it as a volume in containers.

### Application Config (`config.yaml`)

```yaml
anthropic:
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-20250514

slack:
  bot_token: ${SLACK_BOT_TOKEN}
  app_token: ${SLACK_APP_TOKEN}

telegram:
  bot_token: ${TELEGRAM_BOT_TOKEN}

session:
  backend: local  # local, s3, or memory
  local_dir: ./sessions

mcp:
  enabled: true
  servers:
    filesystem:
      enabled: true
      transport: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
```

### MCP Server Configuration

Connect to any MCP-compliant server for extended capabilities:

```yaml
mcp:
  servers:
    # Filesystem operations
    filesystem:
      transport: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/data"]

    # Database queries
    database:
      transport: websocket
      url: ws://db-server:8080/mcp
      auth:
        type: bearer
        token: ${DB_MCP_TOKEN}

    # Custom internal tools
    internal-api:
      transport: stdio
      command: ./mcp-internal-tools
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `ANTHROPIC_API_KEY` | Claude API key | Yes |
| `SLACK_BOT_TOKEN` | Slack bot token (xoxb-*) | For Slack |
| `SLACK_APP_TOKEN` | Slack app token (xapp-*) | For Slack |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token | For Telegram |

### Session Storage

Choose where conversation history is stored:

- **local** - JSON files on disk (default, good for development)
- **s3** - AWS S3 bucket (production, scalable)
- **memory** - In-memory only (testing, no persistence)

```yaml
session:
  backend: s3
  s3_bucket: my-chatbot-sessions
  s3_prefix: sessions/
  s3_region: us-west-2
```

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.24 |
| LLM | Anthropic Claude |
| Agent Framework | Google ADK v0.3.0 |
| Tool Protocol | MCP (Model Context Protocol) |
| Chat Platforms | Slack Socket Mode, Telegram Bot API |
| Session Storage | Local filesystem, AWS S3 |
| Observability | Logrus, Prometheus metrics |
| Database | PostgreSQL (via sqlc) |

## Documentation

- [Project Overview](docs/project-overview.md) - Vision and roadmap
- [Architecture](docs/architecture/) - Technical design documents
- [Research](docs/research/) - Market analysis and requirements

## Contributing

Contributions welcome. See the architecture documentation for technical specifications.
