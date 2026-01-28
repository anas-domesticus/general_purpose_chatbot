# General Purpose Chatbot Implementation Plan

**Date:** January 28, 2026  
**Status:** Final - Ready for Implementation  
**Based on:** ADK Go v0.3.0 + Custom Claude Integration  

## Executive Summary

Build a modular chatbot framework using Google ADK Go with **Claude Sonnet 4.5 from day 1**. Use custom `model.LLM` implementation to avoid dependency on Google's Claude support roadmap.

## Strategy: Claude-First Approach

**Decision:** Custom Claude implementation via `model.LLM` interface
- ✅ **Claude Sonnet 4.5 from day 1** - No waiting for Google support
- ✅ **Full ADK benefits** - Agents, tools, sessions, web UI
- ✅ **Zero Google dependency** - Can never be blocked by their roadmap
- ✅ **Future-proof** - Easy migration if official support comes

## Detailed Implementation Plan

### Module Structure (Based on ADK v0.3.0 patterns)
```
general_purpose_chatbot/
├── cmd/
│   └── chatbot/
│       └── main.go              # ADK launcher integration
├── internal/
│   ├── agents/                  # ADK agent definitions
│   │   ├── slack.go            # Slack workspace agent
│   │   └── registry.go         # Agent loading
│   ├── connectors/             # Chat platform integrations
│   │   ├── slack/              # Slack Socket Mode/Events API
│   │   └── bridge/             # Connector <-> ADK bridge
│   └── models/                 # Custom model implementations
│       └── anthropic/          # Claude Sonnet integration
└── config/
    ├── agents/                 # Agent instructions
    └── chatbot.yml            # Main configuration
```

### Week 1: Claude + ADK Foundation

#### Day 1-2: Custom Claude Model Implementation
```go
// internal/models/anthropic/claude.go
package anthropic

import (
    "context"
    "fmt"
    "google.golang.org/adk/model"
    anthropic "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

type ClaudeModel struct {
    client    *anthropic.Client
    modelName string
}

func NewClaudeModel(apiKey, modelName string) (*ClaudeModel, error) {
    client := anthropic.NewClient(
        option.WithAPIKey(apiKey),
    )
    
    return &ClaudeModel{
        client:    client,
        modelName: modelName, // "claude-3-5-sonnet-20241022"
    }, nil
}

// Implement model.LLM interface
func (c *ClaudeModel) Generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
    // Transform ADK request -> Anthropic format
    messages := c.transformMessages(req.Messages)
    
    anthropicReq := &anthropic.MessageNewParams{
        Model:     anthropic.F(c.modelName),
        MaxTokens: anthropic.F(int64(4000)),
        Messages:  messages,
    }
    
    resp, err := c.client.Messages.New(ctx, anthropicReq)
    if err != nil {
        return nil, fmt.Errorf("claude api error: %w", err)
    }
    
    // Transform Anthropic response -> ADK format
    return c.transformResponse(resp), nil
}

func (c *ClaudeModel) transformMessages(adkMessages []*model.Message) []anthropic.MessageParam {
    // Convert ADK message format to Anthropic format
    var messages []anthropic.MessageParam
    for _, msg := range adkMessages {
        messages = append(messages, anthropic.NewUserMessage(msg.Content))
    }
    return messages
}

func (c *ClaudeModel) transformResponse(resp *anthropic.Message) *model.LLMResponse {
    // Convert Anthropic response to ADK format
    content := ""
    if len(resp.Content) > 0 {
        content = resp.Content[0].Text
    }
    
    return &model.LLMResponse{
        Text: content,
        // Add other fields as needed
    }
}
```

#### Day 3-4: ADK Integration with Claude
```go
// cmd/chatbot/main.go - Claude from day 1
package main

import (
    "context"
    "log"
    "os"
    
    "google.golang.org/adk/agent"
    "google.golang.org/adk/cmd/launcher/full"
    
    "github.com/lewisedginton/general_purpose_chatbot/internal/agents"
    "github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
)

func main() {
    ctx := context.Background()
    
    // Initialize Claude model
    claudeModel, err := anthropic.NewClaudeModel(
        os.Getenv("ANTHROPIC_API_KEY"),
        "claude-3-5-sonnet-20241022",
    )
    if err != nil {
        log.Fatalf("Failed to create Claude model: %v", err)
    }
    
    // Create agent loader with Claude
    agentLoader := agents.NewLoader(claudeModel)
    
    // Launch with full launcher (includes web UI for testing)
    config := &launcher.Config{
        AgentLoader: agentLoader,
    }
    
    launcher := full.NewLauncher()
    if err := launcher.Execute(ctx, config, os.Args[1:]); err != nil {
        log.Fatalf("Failed to launch: %v", err)
    }
}
```

#### Day 5-7: Basic Agent Setup
```go
// internal/agents/slack.go
func NewSlackAgent(model model.LLM) (agent.Agent, error) {
    return llmagent.New(llmagent.Config{
        Name: "slack_assistant", 
        Model: model,
        Description: "Helpful assistant for Slack workspace",
        Instruction: loadInstructionFile("config/agents/slack.txt"),
        Tools: []tool.Tool{
            // Start with simple built-in tools for testing
            functiontool.New("echo", "Echo back text for testing", echoTool),
        },
    })
}

// internal/agents/registry.go
func NewLoader(model model.LLM) agent.Loader {
    slackAgent, err := NewSlackAgent(model)
    if err != nil {
        log.Fatalf("Failed to create slack agent: %v", err)
    }
    
    return agent.NewSingleLoader(slackAgent)
}
```

### Week 2: Slack Integration
```go
// internal/connectors/slack/connector.go - Socket Mode first
package slack

import (
    "context"
    "log"
    
    "github.com/slack-go/slack"
    "github.com/slack-go/slack/socketmode"
)

type Connector struct {
    client     *slack.Client
    socketMode *socketmode.Client
    bridge     *bridge.Bridge // Bridge to ADK
}

func (c *Connector) Start(ctx context.Context) error {
    c.socketMode.Run()
    
    for envelope := range c.socketMode.Events {
        switch envelope.Type {
        case socketmode.EventTypeEventsAPI:
            c.handleEvent(envelope.Data.(slackevents.EventsAPIEvent))
        }
    }
    return nil
}
```

#### Day 8-14: Slack Connector & Bridge
```go
// internal/connectors/slack/connector.go - Socket Mode
package slack

import (
    "context"
    "log"
    
    "github.com/slack-go/slack"
    "github.com/slack-go/slack/socketmode"
)

type Connector struct {
    client     *slack.Client
    socketMode *socketmode.Client
    bridge     *bridge.Bridge
}

func (c *Connector) Start(ctx context.Context) error {
    c.socketMode.Run()
    
    for envelope := range c.socketMode.Events {
        switch envelope.Type {
        case socketmode.EventTypeEventsAPI:
            c.handleEvent(envelope.Data.(slackevents.EventsAPIEvent))
        }
    }
    return nil
}
```

```go
// internal/connectors/bridge/bridge.go
type Bridge struct {
    adkClient *http.Client // Call ADK REST API
    baseURL   string       // ADK server URL
}

func (b *Bridge) SendToAgent(agentName, sessionID, message string) (*Response, error) {
    // HTTP call to ADK REST API
    // Transform Slack message -> ADK format
    // Get response and transform back to Slack format
}
```

### Week 3-4: Production Polish

#### End-to-End Testing & Production Features
- **Local development** - Use ADK web UI to test agents
- **Slack integration** - Test with real Slack workspace  
- **Session persistence** - Conversations maintain context
- **Error handling** - Proper error responses and logging
- **Performance monitoring** - Health checks and metrics
- **Docker deployment** - Containerization for production

#### Enhanced Claude Integration (Optional)
```go
// Add streaming support to Claude model (if needed)
func (c *ClaudeModel) GenerateStream(ctx context.Context, req *model.LLMRequest) (<-chan *model.StreamChunk, error) {
    // Stream Claude responses for better UX
}

// Tool calling support built-in with Claude
func (c *ClaudeModel) SupportsTools() bool {
    return true // Claude excels at tool usage
}
```

## Configuration

### Environment Variables
```bash
# Claude Sonnet 4.5 (primary model)
export ANTHROPIC_API_KEY="sk-ant-..."

# Slack integration
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_APP_TOKEN="xapp-..."
export SLACK_SIGNING_SECRET="..."
```

### Agent Instructions
```
# config/agents/slack.txt
You are a helpful assistant in a Slack workspace for a development team.

You can help with:
- General development questions  
- Code explanations and debugging
- Technical discussions
- Project planning and brainstorming

Be concise and professional. Use Slack formatting when appropriate.
Ask clarifying questions when you need more context.
```

## Success Criteria

### Week 1
- [ ] Claude Sonnet 4.5 model implementation working
- [ ] ADK agent with Claude responds in web UI
- [ ] Basic agent conversation functionality

### Week 2  
- [ ] Full Slack integration (Socket Mode production-ready)
- [ ] Session persistence across conversations
- [ ] End-to-end Slack ↔ Claude conversation flow
- [ ] Error handling, logging, and monitoring

### Week 3-4
- [ ] Advanced agent capabilities (multi-step reasoning)
- [ ] Production deployment (Docker containers)
- [ ] Performance optimization and caching
- [ ] Documentation and testing suite
- [ ] Multi-agent orchestration (if needed)

## Risk Mitigation

1. **Claude support uncertainty** → Custom implementation bypasses Google dependency
2. **ADK Go limitations** → Well-documented official examples exist
3. **Slack complexity** → Socket Mode simpler than webhooks for MVP
4. **Model interface changes** → Wrapper pattern isolates changes

## Next Steps

1. **Review & approve this plan**
2. **Spawn implementation agents for each week**
3. **Set up development environment**
4. **Begin Week 1 implementation**

---

*This plan is based on current ADK Go v0.3.0 examples and official documentation. Will be updated as new features become available.*