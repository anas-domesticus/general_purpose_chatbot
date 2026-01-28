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