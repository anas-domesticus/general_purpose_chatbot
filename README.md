# General Purpose Chatbot Framework

A modular, extensible agent framework built in Go for connecting chat platforms to LLM-powered conversational agents.

> **⚠️ Active Development Notice**
> This project is under rapid active development and is not yet stable. The API, configuration format, and features are subject to change without notice. **Do not rely on this project for production use in its current state.**

## What It Is

This framework bridges chat platforms (Slack, Telegram) with LLMs using Google's Agent Development Kit (ADK). Unlike traditional command-based ChatOps tools, it enables natural language conversations with AI agents that can integrate with external tools via MCP (Model Context Protocol) servers.

### Architecture

```
Chat Platform (Slack/Telegram)
        ↓
    Connector (handles platform-specific messaging)
        ↓
    Executor (routes messages to agents)
        ↓
    Agent Factory (creates LLM-powered agents)
        ↓
    MCP Tools (filesystem, database, HTTP, custom)
```

## Key Features

- **Multi-LLM Support** - Support for Claude (Anthropic), GPT-4 (OpenAI), and Gemini (Google) with custom LLM implementations
- **Multi-Platform Support** - Slack (Socket Mode) and Telegram connectors with extensible architecture
- **MCP Tool Ecosystem** - Connect to any Model Context Protocol server for extended capabilities
- **Session Management** - Persistent conversations with local or S3 storage backends
- **Customizable Agents** - Configure agent behavior via `system.md` prompt files
- **Production Ready** - Structured logging, health checks, graceful shutdown

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

Configuration can be provided via environment variables, a YAML config file, or both. When both are used, environment variables override values from the config file.

### Option 1: Environment Variables Only

```bash
# Set required credentials
export ANTHROPIC_API_KEY="sk-ant-your-api-key"
export SLACK_BOT_TOKEN="xoxb-your-bot-token"
export SLACK_APP_TOKEN="xapp-your-app-token"

# Create a system prompt (optional)
cat > system.md << 'EOF'
You are a helpful DevOps assistant. You help engineers query logs,
check system status, and troubleshoot issues.
EOF

# Run the chatbot
./chatbot
```

### Option 2: Config File with Environment Variable Overrides

```bash
# Set sensitive credentials as environment variables (these override config file values)
export ANTHROPIC_API_KEY="sk-ant-your-api-key"
export SLACK_BOT_TOKEN="xoxb-your-bot-token"
export SLACK_APP_TOKEN="xapp-your-app-token"

# Create a config file for non-sensitive settings
cat > config.yaml << 'EOF'
llm:
  provider: claude  # or openai, or gemini

anthropic:
  model: claude-sonnet-4-5-20250929

storage:
  backend: local
  local_dir: ./data
EOF

# Create a system prompt (optional)
cat > system.md << 'EOF'
You are a helpful DevOps assistant. You help engineers query logs,
check system status, and troubleshoot issues.
EOF

# Run the chatbot with config file
./chatbot --config config.yaml
```

See [Configuration Examples](#configuration-examples) for complete config files for each LLM provider.

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

Configuration is loaded from YAML file first, then environment variables override any matching values. Use environment variables for sensitive values like API keys.

Minimal configuration example (using Anthropic):

```yaml
llm:
  provider: claude  # claude, gemini, or openai

anthropic:
  model: claude-sonnet-4-5-20250929
  # api_key loaded from ANTHROPIC_API_KEY env var

# slack credentials loaded from SLACK_BOT_TOKEN and SLACK_APP_TOKEN env vars

storage:
  backend: local  # local or s3

mcp:
  enabled: true
  servers:
    filesystem:
      enabled: true
      transport: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
```

For complete configuration examples including OpenAI and Gemini, see [Configuration Examples](#configuration-examples).

### MCP Server Configuration

Connect to any MCP-compliant server for extended capabilities:

```yaml
mcp:
  enabled: true
  timeout: 30s
  servers:
    # Filesystem operations
    filesystem:
      name: filesystem
      enabled: true
      transport: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
      environment:
        PATH: /usr/local/bin:/usr/bin
      tool_filter:  # Optional: limit which tools are exposed
        - list_directory
        - read_file

    # Database queries via WebSocket
    database:
      name: database
      enabled: false
      transport: websocket
      url: ws://db-server:8080/mcp
      auth:
        type: bearer  # bearer, basic, or api_key
        token: your-bearer-token  # or set via environment variable

    # Custom internal tools
    internal-api:
      name: internal-api
      enabled: true
      transport: stdio
      command: ./mcp-internal-tools
```

**Transport Support:**
- `stdio` - Executes command as subprocess
- `http` - HTTP transport (2025-03-26 MCP spec)
- `sse` - Server-Sent Events transport (2024-11-05 MCP spec)
- `websocket` - WebSocket transport with authentication support

**Authentication Types:**
- `bearer` - Bearer token authentication
- `basic` - Basic auth (username/password)
- `api_key` - API key in custom header

### Environment Variables

#### LLM Providers

| Variable | Description | Default |
|----------|-------------|---------|
| `LLM_PROVIDER` | LLM provider to use | `claude` |
| `ANTHROPIC_API_KEY` | Anthropic Claude API key | - |
| `CLAUDE_MODEL` | Claude model name | `claude-sonnet-4-5-20250929` |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `OPENAI_MODEL` | OpenAI model name | `gpt-4` |
| `GEMINI_API_KEY` | Google Gemini API key | - |
| `GEMINI_MODEL` | Gemini model name | `gemini-2.5-flash` |

#### Chat Platforms

| Variable | Description | Required |
|----------|-------------|----------|
| `SLACK_BOT_TOKEN` | Slack bot token (xoxb-*) | For Slack |
| `SLACK_APP_TOKEN` | Slack app token (xapp-*) | For Slack |
| `SLACK_DEBUG` | Enable Slack debug logging | No |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token | For Telegram |
| `TELEGRAM_DEBUG` | Enable Telegram debug logging | No |

#### Session Storage

| Variable | Description | Default |
|----------|-------------|---------|
| `STORAGE_BACKEND` | Storage backend (local/s3) | `local` |
| `STORAGE_LOCAL_DIR` | Directory for local file storage | `./data` |
| `STORAGE_S3_BUCKET` | S3 bucket name | - |
| `STORAGE_S3_PREFIX` | S3 key prefix | `sessions` |
| `STORAGE_S3_REGION` | AWS region | - |
| `STORAGE_S3_PROFILE` | AWS profile name (optional) | - |

#### Monitoring & Logging

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `LOG_FORMAT` | Log format (json/text) | `json` |
| `HEALTH_CHECK_TIMEOUT` | Health check timeout | `10s` |

#### MCP Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `MCP_ENABLED` | Enable MCP servers | `false` |
| `MCP_TIMEOUT` | MCP operation timeout | `30s` |

#### Service Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVICE_NAME` | Service name | `general-purpose-chatbot` |
| `ENVIRONMENT` | Environment (development/production) | `development` |
| `PORT` | HTTP server port | `8080` |
| `REQUEST_TIMEOUT` | Request timeout | `30s` |

For complete configuration options, see the [example configs](docs/examples/).

### Session Storage

Choose where conversation history is stored:

- **local** - JSON files on disk (default, good for development)
- **s3** - AWS S3 bucket (production, scalable)

```yaml
storage:
  backend: s3
  s3_bucket: my-chatbot-sessions
  s3_prefix: sessions/
  s3_region: us-west-2
  s3_profile: default  # optional AWS profile
```

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.24 |
| LLM Providers | Anthropic Claude, OpenAI GPT-4, Google Gemini |
| Agent Framework | Google ADK v0.3.0 |
| Tool Protocol | MCP (Model Context Protocol) v0.7.0 |
| Chat Platforms | Slack Socket Mode, Telegram Bot API |
| Session Storage | Local filesystem, AWS S3 |
| Observability | Logrus |

## Configuration Examples

Complete configuration examples for each LLM provider:

- [Claude (Anthropic)](docs/examples/config-claude.yaml) - Full configuration with all options
- [OpenAI](docs/examples/config-openai.yaml) - GPT-4 configuration example
- [Gemini (Google)](docs/examples/config-gemini.yaml) - Gemini configuration example

## Health Checks

**Health Check Endpoints:**
- `/health` - Combined liveness and readiness status
- `/health/live` - Kubernetes liveness probe
- `/health/ready` - Kubernetes readiness probe

## Contributing

Contributions welcome.
