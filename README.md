# General Purpose Chatbot Framework

A modular, extensible agent framework built in Go for connecting chat platforms to LLM-powered conversational agents.

## Vision

Build a general-purpose, modular agent framework that bridges chat platforms (Telegram, Slack, Discord) with intelligent LLM-powered agents. Unlike traditional command-based ChatOps tools, this framework enables natural language conversations with AI agents that can spawn specialized sub-agents, integrate with external tools via MCP servers, and operate natively in Kubernetes environments.

## Key Features

- **Google ADK Integration** - Built on Google's Agent Development Kit for robust agent orchestration
- **Multi-Platform Connectors** - Telegram, Slack, Discord support with pluggable architecture
- **MCP Server Integration** - Connect to any Model Context Protocol server for tool ecosystem
- **Subagent Spawning** - Dynamic creation of specialized agents for complex workflows
- **File-based Configuration** - Version-controlled prompts and configuration management
- **Kubernetes Native** - Operators, custom resources, and cloud-native deployment patterns

## First Use Case

Slack bot for Kubernetes operations - allowing developers to query logs, check pod status, and troubleshoot issues through natural conversation rather than remembering complex kubectl commands.

## Project Status

**Research Phase Complete** ✅  
**Ready for Implementation** 🚀

## Documentation

- **[Project Overview](docs/project-overview.md)** - Complete project vision and market validation
- **[Architecture](docs/architecture/)** - Technical architecture proposals and detailed designs
- **[Research](docs/research/)** - Market analysis and requirements gathering

## Configuration

### System Prompt

The agent's behavior is configured via a `system.md` file placed in the directory where the binary is executed:

```bash
# Create your system prompt
cat > system.md << EOF
# Your Custom Agent

You are a specialized AI assistant for...
EOF

# Run the agent (will load system.md automatically)
./chatbot
```

**Container-friendly**: Simply mount your `system.md` as a volume or include it in your container image.

**Fallback**: If no `system.md` exists, the agent uses sensible defaults and logs a warning.

### MCP Server Configuration

The agent supports **Model Context Protocol (MCP)** servers for extended capabilities:

```yaml
# config.yaml or environment variables
mcp:
  enabled: true
  servers:
    filesystem:
      enabled: true
      transport: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
      
    database:
      enabled: true  
      transport: websocket
      url: "ws://db-server:8080/mcp"
      auth:
        type: bearer
        token: "${DB_MCP_TOKEN}"
```

**Available MCP Server Types:**
- **Filesystem** - File operations (read, write, list)
- **Google Maps** - Directions and place searches  
- **Database Toolbox** - Query PostgreSQL, MySQL, BigQuery, etc.
- **HTTP** - REST API calls
- **Custom servers** - Any MCP-compliant server

### Environment Variables

- `SLACK_BOT_TOKEN` - Slack bot token (xoxb-*)
- `SLACK_APP_TOKEN` - Slack app token (xapp-*)
- `ADK_BASE_URL` - ADK server URL (default: http://localhost:8000)
- `AGENT_NAME` - ADK agent name (default: slack_assistant)
- `ANTHROPIC_API_KEY` - Claude API key (required)
- `GOOGLE_MAPS_API_KEY` - Google Maps API key (if using Maps MCP server)

## Quick Start with MCP

Test the bot with filesystem operations:

```bash
# 1. Set your Claude API key
export ANTHROPIC_API_KEY="sk-ant-your-api-key"

# 2. Run with MCP filesystem server  
./chatbot web --config mcp.example.yaml

# 3. Open http://localhost:8080 and try:
#    "List the files in the directory" 
#    "Read the contents of filename.txt"
#    "Create a file called test.txt with hello world"
```

## Getting Started

This project is in early development. See the [architecture documentation](docs/architecture/) for detailed technical specifications and implementation roadmap.

## Technology Stack

- **Language**: Go 1.23+
- **Core Framework**: Google ADK (Agent Development Kit)
- **Chat Platforms**: go-telegram-bot-api, slack-go/slack
- **Container Platform**: Kubernetes with custom operators
- **Configuration**: YAML-based with hot-reload support
- **Protocols**: MCP (Model Context Protocol) for tool integration

## Contributing

Project is in initial development phase. Design documents and architecture are being finalized before implementation begins.