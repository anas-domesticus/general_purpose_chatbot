# Go Modular Agent System Architecture Proposal

## Executive Summary

This document proposes a clean, modular architecture for building a pluggable agent system in Go that connects chat platforms (Telegram/Slack) to LLM sessions with comprehensive tool integration, including Google ADK, MCP servers, and Kubernetes-native capabilities.

## Research Summary

### Key Findings

1. **Go Plugin Patterns**: Go supports both compile-time (interface-based) and runtime (plugin package) modularity. Interface-based patterns are preferred for stability and cross-platform compatibility.

2. **Chat Bot Frameworks**: Multiple mature Go frameworks exist:
   - `go-telegram-bot-api` for Telegram
   - `go-chat-bot/bot` for multi-platform support
   - `slacker` for Slack-specific bots

3. **Agent Systems**: Google's ADK (Agent Development Kit) provides a robust Go foundation for agent orchestration with built-in session management and tool integration.

4. **MCP Integration**: Official Go SDK available (`github.com/modelcontextprotocol/go-sdk`) with strong community support.

5. **K8s Patterns**: Controller-runtime library provides solid foundations for Kubernetes-native applications.

## Proposed Architecture

### Core System Structure

```
go-agent-system/
├── cmd/
│   ├── agent-server/         # Main agent server
│   ├── chat-bridge/          # Chat platform bridge
│   └── k8s-controller/       # Kubernetes controller
├── internal/
│   ├── core/                 # Core agent logic
│   │   ├── agent/            # Agent management
│   │   ├── session/          # Session management
│   │   └── orchestrator/     # Agent orchestration
│   ├── connectors/           # Chat platform connectors
│   │   ├── telegram/
│   │   ├── slack/
│   │   └── interface.go      # Connector interface
│   ├── tools/                # Tool system
│   │   ├── registry/         # Tool registry
│   │   ├── mcp/              # MCP integration
│   │   └── plugins/          # Plugin system
│   ├── config/               # Configuration management
│   └── storage/              # State persistence
├── pkg/
│   ├── adk/                  # Google ADK integration
│   ├── k8s/                  # Kubernetes utilities
│   └── types/                # Shared types
├── configs/                  # Configuration files
│   ├── system-prompts/       # File-based prompts
│   └── deployment/           # K8s manifests
└── tools/                    # External tool integrations
    ├── k8s-monitor/          # Kubernetes monitoring tools
    └── mcp-servers/          # MCP server implementations
```

## Detailed Component Design

### 1. Core Agent System

#### Agent Manager (`internal/core/agent/`)

```go
// Agent represents a spawnable agent instance
type Agent struct {
    ID          string
    Type        AgentType
    Config      *AgentConfig
    Session     *session.Session
    Tools       []tool.Tool
    Parent      *Agent
    Children    []*Agent
    State       AgentState
}

// AgentManager handles agent lifecycle
type AgentManager interface {
    SpawnAgent(ctx context.Context, config *AgentConfig) (*Agent, error)
    GetAgent(id string) (*Agent, error)
    ListAgents() []*Agent
    TerminateAgent(ctx context.Context, id string) error
    OrchestateHierarchy(ctx context.Context, rootAgent *Agent) error
}
```

**Integration with Google ADK:**
```go
import (
    "google.golang.org/adk/agent"
    "google.golang.org/adk/agent/llmagent"
    "google.golang.org/adk/session"
)

type ADKAgentManager struct {
    adkSession *session.Session
    launcher   *launcher.Launcher
}

func (am *ADKAgentManager) SpawnAgent(ctx context.Context, config *AgentConfig) (*Agent, error) {
    adkAgent, err := llmagent.New(llmagent.Config{
        Name: config.Name,
        Instruction: config.SystemPrompt,
        Model: config.Model,
        Tools: am.convertToolsToADK(config.Tools),
    })
    if err != nil {
        return nil, err
    }
    
    return am.wrapADKAgent(adkAgent, config), nil
}
```

#### Session Management (`internal/core/session/`)

```go
type SessionManager interface {
    CreateSession(ctx context.Context, userID, channelID string) (*Session, error)
    GetSession(sessionID string) (*Session, error)
    UpdateSession(ctx context.Context, session *Session) error
    CloseSession(ctx context.Context, sessionID string) error
}

type Session struct {
    ID          string
    UserID      string
    ChannelID   string
    Platform    Platform
    Context     map[string]interface{}
    State       SessionState
    CreatedAt   time.Time
    LastActive  time.Time
    Agents      []*Agent
}
```

### 2. Chat Platform Connectors (`internal/connectors/`)

#### Connector Interface

```go
type ChatConnector interface {
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    SendMessage(ctx context.Context, msg *Message) error
    ReceiveMessages(ctx context.Context) <-chan *Message
    GetPlatformInfo() *PlatformInfo
}

type Message struct {
    ID        string
    UserID    string
    ChannelID string
    Text      string
    Metadata  map[string]interface{}
    Timestamp time.Time
}
```

#### Telegram Connector Implementation

```go
import (
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramConnector struct {
    bot     *tgbotapi.BotAPI
    updates tgbotapi.UpdatesChannel
    msgChan chan *Message
}

func (tc *TelegramConnector) Connect(ctx context.Context) error {
    bot, err := tgbotapi.NewBotAPI(tc.token)
    if err != nil {
        return err
    }
    
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    tc.updates = bot.GetUpdatesChan(u)
    tc.bot = bot
    
    go tc.processUpdates(ctx)
    return nil
}
```

#### Slack Connector Implementation

```go
import (
    "github.com/slack-go/slack"
    "github.com/slack-go/slack/socketmode"
)

type SlackConnector struct {
    api      *slack.Client
    socket   *socketmode.Client
    msgChan  chan *Message
}
```

### 3. Tool System (`internal/tools/`)

#### Tool Registry

```go
type ToolRegistry interface {
    RegisterTool(tool Tool) error
    GetTool(name string) (Tool, error)
    ListTools() []Tool
    LoadFromMCP(serverURL string) error
    LoadPlugins(pluginDir string) error
}

type Tool interface {
    Name() string
    Description() string
    Schema() *jsonschema.Schema
    Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
}
```

#### MCP Integration

```go
import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPToolAdapter struct {
    client *mcp.Client
    tools  map[string]*mcp.Tool
}

func (m *MCPToolAdapter) LoadFromMCP(serverURL string) error {
    client, err := mcp.NewClient(serverURL)
    if err != nil {
        return err
    }
    
    tools, err := client.ListTools(context.Background())
    if err != nil {
        return err
    }
    
    for _, tool := range tools {
        m.tools[tool.Name] = tool
    }
    
    return nil
}
```

### 4. Kubernetes Integration (`pkg/k8s/`)

#### K8s Monitoring Tools

```go
import (
    "k8s.io/client-go/kubernetes"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

type KubernetesMonitor struct {
    client    kubernetes.Interface
    namespace string
}

func (km *KubernetesMonitor) GetPodLogs(ctx context.Context, podName string, lines int) (string, error) {
    req := km.client.CoreV1().Pods(km.namespace).GetLogs(podName, &corev1.PodLogOptions{
        TailLines: int64Ptr(lines),
    })
    
    logs, err := req.Stream(ctx)
    if err != nil {
        return "", err
    }
    defer logs.Close()
    
    buf := new(bytes.Buffer)
    _, err = io.Copy(buf, logs)
    return buf.String(), err
}

func (km *KubernetesMonitor) ListPods(ctx context.Context) ([]corev1.Pod, error) {
    podList, err := km.client.CoreV1().Pods(km.namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, err
    }
    return podList.Items, nil
}
```

#### Custom Resource Definitions for Agent Management

```yaml
# configs/deployment/agent-crd.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: agents.agent-system.io
spec:
  group: agent-system.io
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              agentType:
                type: string
                enum: ["chat", "k8s-monitor", "general"]
              config:
                type: object
                properties:
                  systemPrompt:
                    type: string
                  tools:
                    type: array
                    items:
                      type: string
```

### 5. Configuration System (`internal/config/`)

#### File-based System Prompts

```go
type ConfigManager struct {
    promptsDir    string
    configDir     string
    watchChanges  bool
}

func (cm *ConfigManager) LoadSystemPrompt(agentType string) (string, error) {
    promptPath := filepath.Join(cm.promptsDir, fmt.Sprintf("%s.md", agentType))
    content, err := ioutil.ReadFile(promptPath)
    if err != nil {
        return "", err
    }
    return string(content), nil
}

func (cm *ConfigManager) WatchPromptChanges(ctx context.Context) <-chan *PromptChange {
    // Use fsnotify to watch for file changes
    changes := make(chan *PromptChange)
    // Implementation for file watching...
    return changes
}
```

### 6. Deployment Architecture

#### Docker Composition

```yaml
# docker-compose.yml
version: '3.8'
services:
  agent-server:
    build: ./cmd/agent-server
    environment:
      - GOOGLE_API_KEY=${GOOGLE_API_KEY}
      - TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
      - SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}
    volumes:
      - ./configs/system-prompts:/app/prompts
      - ./data:/app/data
    depends_on:
      - redis
      - postgres
  
  chat-bridge:
    build: ./cmd/chat-bridge
    depends_on:
      - agent-server
  
  k8s-controller:
    build: ./cmd/k8s-controller
    volumes:
      - ~/.kube/config:/root/.kube/config
  
  redis:
    image: redis:7-alpine
  
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: agent_system
```

#### Kubernetes Deployment

```yaml
# configs/deployment/agent-system.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agent-system
  template:
    metadata:
      labels:
        app: agent-system
    spec:
      containers:
      - name: agent-server
        image: agent-system:latest
        env:
        - name: GOOGLE_API_KEY
          valueFrom:
            secretKeyRef:
              name: agent-secrets
              key: google-api-key
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: prompts
          mountPath: /app/prompts
      volumes:
      - name: prompts
        configMap:
          name: system-prompts
```

## Implementation Strategy

### Phase 1: Core Foundation (Weeks 1-4)
1. **Core Agent System**
   - Implement basic Agent and AgentManager interfaces
   - Integrate Google ADK for agent orchestration
   - Build session management system

2. **Basic Chat Connectors**
   - Implement Telegram connector
   - Implement Slack connector
   - Create connector registry and routing

3. **Configuration Management**
   - File-based system prompt loading
   - Basic configuration structures

### Phase 2: Tool Integration (Weeks 5-8)
1. **Tool System**
   - Implement tool registry and plugin system
   - Integrate MCP client for external tools
   - Create basic tool adapters

2. **K8s Integration**
   - Basic Kubernetes monitoring tools
   - Pod logs and status queries
   - Custom resource definitions

### Phase 3: Advanced Features (Weeks 9-12)
1. **Agent Spawning**
   - Subagent spawning and management
   - Hierarchical agent structures
   - Inter-agent communication

2. **Production Readiness**
   - Comprehensive error handling
   - Monitoring and observability
   - Performance optimization

3. **ChatOps Features**
   - Advanced Kubernetes operations
   - Interactive Slack commands
   - Real-time monitoring alerts

### Phase 4: Scale & Polish (Weeks 13-16)
1. **Scalability**
   - Horizontal scaling patterns
   - Load balancing and failover
   - State persistence optimization

2. **Security**
   - Authentication and authorization
   - Secure tool execution
   - Audit logging

## Key Benefits of This Architecture

### 1. Modularity & Extensibility
- **Plugin System**: Interface-based design allows easy addition of new connectors, tools, and agent types
- **Google ADK Integration**: Leverages Google's mature agent framework while maintaining flexibility
- **MCP Compatibility**: Standard protocol support enables integration with existing tool ecosystems

### 2. Kubernetes-Native
- **Custom Resources**: Agents are first-class Kubernetes objects
- **Controller Pattern**: Standard K8s reconciliation loops for agent management
- **Cloud-Native**: Designed for container orchestration and horizontal scaling

### 3. Clean Architecture
- **Separation of Concerns**: Clear boundaries between chat handling, agent logic, and tool execution
- **Dependency Inversion**: Core logic depends on interfaces, not implementations
- **Testability**: Each component can be unit tested independently

### 4. Production Ready
- **Session Management**: Robust state management for multi-user scenarios
- **Error Handling**: Comprehensive error recovery and logging
- **Observability**: Built-in metrics and tracing capabilities

## Specific Use Case: Slack Bot for K8s Monitoring

```go
// Example implementation for the first use case
type K8sSlackBot struct {
    slackConnector *SlackConnector
    k8sMonitor     *KubernetesMonitor
    agentManager   *ADKAgentManager
}

func (bot *K8sSlackBot) HandleSlackCommand(ctx context.Context, msg *Message) error {
    // Parse command (e.g., "/k8s pods", "/k8s logs podname")
    command := parseK8sCommand(msg.Text)
    
    // Spawn specialized agent for K8s operations
    agentConfig := &AgentConfig{
        Type: AgentTypeK8sMonitor,
        SystemPrompt: bot.loadK8sPrompt(),
        Tools: []string{"k8s-pods", "k8s-logs", "k8s-events"},
    }
    
    agent, err := bot.agentManager.SpawnAgent(ctx, agentConfig)
    if err != nil {
        return err
    }
    
    // Execute command through agent
    result, err := agent.Execute(ctx, command)
    if err != nil {
        return err
    }
    
    // Send response back to Slack
    response := &Message{
        ChannelID: msg.ChannelID,
        Text: result.String(),
    }
    
    return bot.slackConnector.SendMessage(ctx, response)
}
```

## Conclusion

This architecture provides a robust, scalable foundation for building a modular agent system that meets all requirements:

- ✅ **Pluggable chat connectors** via interface-based design
- ✅ **File-based system prompts** with hot-reload capability  
- ✅ **Subagent spawning** through Google ADK integration
- ✅ **MCP server connectivity** via official Go SDK
- ✅ **Google ADK integration** as core orchestration layer
- ✅ **Tool plugin system** with registry and dynamic loading
- ✅ **K8s cluster integration** with custom resources and operators

The phased implementation approach ensures rapid iteration while building toward a production-ready system. The use of established patterns (Clean Architecture, Controller-Runtime) and mature libraries (Google ADK, MCP Go SDK) provides a solid foundation for long-term maintenance and scaling.