# Go Modular Agent System Project

## Request Date: 2026-01-28
**Deadline**: Lewis wants proposal at 8:30 AM tomorrow (2026-01-29)

## Project Vision
A modular, pluggable system written in Go for connecting chat platforms (Telegram/Slack) to LLM sessions, similar to how Clawdbot works but for other purposes.

## Core Requirements

### Agent System
- File-based system prompts
- Subagent spawning capabilities
- Configurable via files (no hardcoded config)

### Connectivity
- Pluggable chat connectors (Telegram, Slack)
- MCP (Model Context Protocol) server integration
- Connect any MCP server
- **Google ADK integration** (Application Development Kit)

### Plugin Architecture
- Tools as plugins
- Modular design with proper separation of concerns

## First Use Case
**K8s DevOps Agent**:
- Lives in Kubernetes cluster
- Connected to Slack
- Allows developers to:
  - Query logs
  - Check pod state
  - General cluster operations

## Research Tasks for Tomorrow
1. **Go Plugin Patterns**: Research Go approaches to plugin systems (pkg/plugin, interfaces, etc.)
2. **Chat Bot Frameworks**: Existing Go frameworks for chat platforms
3. **Agent Architectures**: Study distributed agent patterns, session management
4. **MCP Integration**: Understanding MCP server connectivity patterns
5. **Google ADK Integration**: Research Google Application Development Kit usage in Go
6. **K8s Integration**: Go patterns for in-cluster operations

## Architecture Considerations
- Clean separation of concerns
- Testable components
- Configuration-driven behavior
- Hot-reloadable plugins?
- Resource management (agent lifecycle)
- Session persistence/recovery

## TODO
- [ ] Set up proper reminder for 8:30 AM
- [ ] Research Go plugin architectures
- [ ] Study existing agent frameworks
- [ ] Propose clean module structure
- [ ] Consider K8s deployment patterns