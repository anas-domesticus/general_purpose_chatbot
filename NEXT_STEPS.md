# Implementation Next Steps

**Ready to spawn agents for immediate implementation.**

## Phase 1: Foundation (Week 1)

### Agent 1: Claude Model Implementation
**Task:** Build custom `model.LLM` implementation for Claude Sonnet 4.5
**Files to create:**
- `internal/models/anthropic/claude.go`
- `internal/models/anthropic/transform.go` 
- `go.mod` dependency: `github.com/anthropics/anthropic-sdk-go`

**Success criteria:** Claude model responds through ADK web UI

### Agent 2: ADK Integration  
**Task:** Wire Claude model into ADK launcher and create basic agent
**Files to create:**
- `cmd/chatbot/main.go`
- `internal/agents/slack.go`
- `internal/agents/registry.go`

**Success criteria:** `go run cmd/chatbot/main.go web` shows working Claude agent

### Agent 3: Slack Connector
**Task:** Build Slack Socket Mode connector with message routing
**Files to create:**
- `internal/connectors/slack/connector.go`
- `internal/connectors/bridge/adk_bridge.go`
- `go.mod` dependency: `github.com/slack-go/slack`

**Success criteria:** Slack messages reach Claude agent and responses return

## Phase 2: Production Ready (Week 2)

### Agent 4: Kubernetes Tools
**Task:** Implement K8s troubleshooting tools as ADK function tools
**Files to create:**
- `internal/tools/k8s/pods.go`
- `internal/tools/k8s/logs.go`
- `internal/tools/k8s/describe.go`

### Agent 5: Error Handling & Monitoring
**Task:** Production-grade error handling, logging, and health checks
**Files to create:**
- `internal/monitoring/health.go`
- `internal/middleware/recovery.go`
- Enhanced error handling in all connectors

## Ready to Launch

Each agent should be given:
1. **The specific task above**
2. **Reference to `IMPLEMENTATION_PLAN.md` for detailed code examples**  
3. **Reference to `CLAUDE_STRATEGY.md` for Claude integration approach**
4. **Access to current repo structure and existing boilerplate**

## Environment Setup

```bash
# Get Claude API key from Anthropic Console
export ANTHROPIC_API_KEY="sk-ant-..."

# Slack app setup (create at api.slack.com)
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_APP_TOKEN="xapp-..."

# Dependencies
cd ./code/general_purpose_chatbot
go mod tidy
```

## First Verification

After Agent 1 + Agent 2 complete:
```bash
cd ./code/general_purpose_chatbot
go run cmd/chatbot/main.go web
# Should start ADK web UI with Claude-powered agent
# Visit http://localhost:8080 and test conversation
```

**Ready to go?** Spawn the first two agents and let's get Claude talking through ADK.