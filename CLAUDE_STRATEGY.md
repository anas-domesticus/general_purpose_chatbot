# Claude Integration Strategy - Risk Mitigation

**Problem:** Official ADK Go only supports Gemini. Claude support uncertain.
**Requirement:** Production system needs Claude Sonnet 4.5 from day one.

## Option A: Custom Model Implementation (RECOMMENDED)

Implement the `model.LLM` interface directly for Anthropic API.

### Pros
✅ Full control over Claude integration  
✅ No dependency on Google's roadmap  
✅ Still get all ADK benefits (agents, tools, sessions)  
✅ Can switch back to official if they add support later  

### Implementation
```go
// internal/models/anthropic/claude.go
package anthropic

import (
    "context"
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
        modelName: modelName, // "claude-3-5-sonnet-20241022" or latest
    }, nil
}

// Implement model.LLM interface
func (c *ClaudeModel) Generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
    // Transform ADK request -> Anthropic format
    anthropicReq := &anthropic.MessageNewParams{
        Model:     anthropic.F(c.modelName),
        MaxTokens: anthropic.F(int64(4000)),
        Messages: c.transformMessages(req.Messages),
    }
    
    resp, err := c.client.Messages.New(ctx, anthropicReq)
    if err != nil {
        return nil, err
    }
    
    // Transform Anthropic response -> ADK format
    return c.transformResponse(resp), nil
}
```

### Usage in agents
```go
// Create Claude model (Sonnet 4.5 recommended)
claudeModel, err := anthropic.NewClaudeModel(
    os.Getenv("ANTHROPIC_API_KEY"),
    "claude-3-5-sonnet-20241022", // or latest available
)

// Rest of ADK code unchanged
agent, err := llmagent.New(llmagent.Config{
    Name: "slack_assistant",
    Model: claudeModel, // Custom implementation
    Instruction: "...",
    Tools: []tool.Tool{...},
})
```

---

## Option B: LiteLLM Proxy Service

Run Python LiteLLM as separate service, ADK calls via HTTP.

### Architecture
```
[ADK Go App] -> [LiteLLM Proxy] -> [Claude API]
     |              |                    |
   Port 8080    Port 4000          Anthropic
```

### Implementation
```go
// internal/models/litellm/proxy.go
type LiteLLMProxy struct {
    baseURL string // http://localhost:4000
    apiKey  string
}

func (l *LiteLLMProxy) Generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
    // HTTP call to LiteLLM proxy
    // Proxy handles Claude API translation
}
```

### Setup
```yaml
# docker-compose.yml
services:
  litellm:
    image: ghcr.io/berriai/litellm:main-latest
    ports:
      - "4000:4000"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    volumes:
      - ./litellm_config.yaml:/app/config.yaml
    command: --config /app/config.yaml

  chatbot:
    build: .
    ports:
      - "8080:8080"
    environment:
      - LITELLM_PROXY_URL=http://litellm:4000
```

### Pros
✅ Proven LiteLLM integration  
✅ Can easily switch models  
✅ No ADK interface implementation needed  

### Cons
❌ Extra service to manage  
❌ Network dependency  
❌ Python dependency in Go stack  

---

## Option C: Community ADK Fork

Use `github.com/go-a2a/adk-go` which claims Anthropic support.

### Pros
✅ Claims "fully compatible with Python SDK"  
✅ Explicitly mentions Anthropic support  
✅ Drop-in replacement for official ADK  

### Cons
❌ Community project, not Google-backed  
❌ Maintenance concerns  
❌ May lag behind official features  

---

## Recommendation: Option A (Custom Implementation)

**Why:**
- **Lowest risk** - Full control over Claude integration
- **Production ready** - No external dependencies
- **Future-proof** - Can migrate to official support later
- **Clean architecture** - Implements standard interface

**Implementation effort:** ~2-3 days to build robust Claude model implementation

**Fallback:** If custom implementation proves difficult, pivot to Option B (proxy)

## Updated Implementation Plan

**Week 1:**
- Day 1-2: Build custom Claude model implementation
- Day 3-4: Test with basic ADK agent
- Day 5-7: Integrate with Slack connector

This eliminates the Gemini dependency entirely and starts with Claude from day one.

**Thoughts?** This approach gives you Claude immediately while keeping all ADK benefits, with no dependency on Google's roadmap.