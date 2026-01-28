# Go Modular Agent System Architecture Proposal

## Executive Summary

This proposal outlines a comprehensive modular agent system architecture built in Go, leveraging Google's Agent Development Kit (ADK) as the core framework. The system provides pluggable chat platform connectors, MCP server integration, tool plugins, and Kubernetes-native deployment patterns.

## Core Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Agent Orchestrator                       │
│                  (Google ADK Core)                          │
├─────────────────────────────────────────────────────────────┤
│  Chat Connectors  │  Agent Manager   │  Tool System         │
│  ├─ Telegram      │  ├─ Spawning     │  ├─ MCP Servers     │
│  ├─ Slack         │  ├─ Sessions     │  ├─ K8s Tools       │
│  └─ Discord       │  └─ State Mgmt   │  └─ Custom Tools    │
├─────────────────────────────────────────────────────────────┤
│              Configuration & Prompt Management              │
│                    (File-based System)                      │
├─────────────────────────────────────────────────────────────┤
│                  Kubernetes Integration                     │
│               (Operators & Native Patterns)                 │
└─────────────────────────────────────────────────────────────┘
```

## 1. Core Framework: Google ADK Integration

### Foundation Components

**Primary Dependencies:**
```go
import (
    "google.golang.org/adk/agent"
    "google.golang.org/adk/agent/llmagent" 
    "google.golang.org/adk/session"
    "google.golang.org/adk/model/gemini"
    "google.golang.org/adk/tool"
)
```

**Key ADK Features to Leverage:**
- **Multi-Agent Architecture**: Sequential, Parallel, and Loop agents
- **Built-in Session Management**: State persistence across interactions
- **Model Agnostic**: Support for Gemini, Claude, OpenAI via unified interface
- **Tool System**: Native tool registration and execution
- **A2A Protocol**: Agent-to-Agent communication

### Agent System Design

```go
// Core agent types leveraging ADK patterns
type AgentSystem struct {
    orchestrator   *agent.Agent           // Main coordinator
    chatHandlers   map[string]*agent.Agent // Platform-specific agents
    toolAgents     []*agent.Agent         // Specialized tool agents
    sessionManager *session.Manager       // ADK session management
}

// Multi-agent patterns using ADK primitives
func NewAgentSystem(cfg *Config) *AgentSystem {
    // Main orchestrator using SequentialAgent
    orchestrator, _ := llmagent.New(llmagent.Config{
        Name: "Orchestrator",
        Instructions: cfg.SystemPrompt,
        Tools: []tool.Tool{
            // Register MCP tools, K8s tools, etc.
        },
    })
    
    return &AgentSystem{
        orchestrator: orchestrator,
        chatHandlers: make(map[string]*agent.Agent),
        sessionManager: session.NewManager(),
    }
}
```

## 2. Plugin Architecture Pattern

### Interface-Based Plugin System

Based on research, we'll use Go's interface extension pattern combined with ADK's tool system:

```go
// Core plugin interfaces
type ChatConnector interface {
    Name() string
    Connect(ctx context.Context) error
    Listen(msgHandler MessageHandler) error
    Send(msg *Message) error
    Close() error
}

type ToolPlugin interface {
    tool.Tool  // Extend ADK's tool interface
    Initialize(config map[string]interface{}) error
    Cleanup() error
}

type AgentPlugin interface {
    agent.Agent // Extend ADK's agent interface
    Configure(cfg *PluginConfig) error
}
```

### Plugin Registry

```go
type PluginRegistry struct {
    chatConnectors map[string]ChatConnector
    toolPlugins    map[string]ToolPlugin
    agentPlugins   map[string]AgentPlugin
    mu            sync.RWMutex
}

func (r *PluginRegistry) RegisterChatConnector(name string, connector ChatConnector) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.chatConnectors[name] = connector
}
```

### Plugin Loading Strategy

**Option 1: Compiled-in Plugins (Recommended)**
```go
// Static registration at compile time
func init() {
    registry.RegisterChatConnector("telegram", &TelegramConnector{})
    registry.RegisterChatConnector("slack", &SlackConnector{})
    registry.RegisterToolPlugin("k8s", &KubernetesToolPlugin{})
}
```

**Option 2: Dynamic Loading via Go Plugins**
```go
// For advanced use cases requiring runtime loading
func loadPlugin(path string) error {
    p, err := plugin.Open(path)
    if err != nil {
        return err
    }
    
    sym, err := p.Lookup("NewConnector")
    if err != nil {
        return err
    }
    
    constructor := sym.(func() ChatConnector)
    connector := constructor()
    registry.RegisterChatConnector(connector.Name(), connector)
    return nil
}
```

## 3. Chat Platform Connectors

### Connector Implementations

**Telegram Connector** (using go-telegram/bot):
```go
type TelegramConnector struct {
    bot     *bot.Bot
    handler MessageHandler
}

func (t *TelegramConnector) Connect(ctx context.Context) error {
    opts := []bot.Option{
        bot.WithDefaultHandler(t.handleUpdate),
    }
    
    t.bot, err := bot.New(token, opts...)
    return err
}

func (t *TelegramConnector) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
    if update.Message != nil {
        msg := &Message{
            ID: strconv.Itoa(update.Message.MessageID),
            Text: update.Message.Text,
            UserID: strconv.FormatInt(update.Message.From.ID, 10),
            Platform: "telegram",
        }
        t.handler(msg)
    }
}
```

**Slack Connector** (using slack-go/slack):
```go
type SlackConnector struct {
    client  *slack.Client
    rtm     *slack.RTM
    handler MessageHandler
}

func (s *SlackConnector) Connect(ctx context.Context) error {
    s.client = slack.New(token)
    s.rtm = s.client.NewRTM()
    
    go s.rtm.ManageConnection()
    go s.listenForMessages()
    return nil
}
```

### Message Flow Architecture

```go
type MessageHandler func(*Message) error

type Message struct {
    ID       string
    Text     string
    UserID   string
    Platform string
    Metadata map[string]interface{}
}

// Central message router
type MessageRouter struct {
    agentSystem *AgentSystem
    connectors  map[string]ChatConnector
}

func (r *MessageRouter) HandleMessage(msg *Message) error {
    // Create ADK session context
    sess := r.agentSystem.sessionManager.GetOrCreate(msg.UserID)
    
    // Route to main orchestrator agent
    ctx := agent.InvocationContext{
        Session: sess,
        Message: msg.Text,
    }
    
    response := r.agentSystem.orchestrator.Process(ctx)
    
    // Send response back through appropriate connector
    connector := r.connectors[msg.Platform]
    return connector.Send(&Message{
        Text: response,
        UserID: msg.UserID,
    })
}
```

## 4. MCP Server Integration

### MCP Client Integration

Using the official Go SDK (github.com/modelcontextprotocol/go-sdk):

```go
import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPManager struct {
    servers map[string]*mcp.Client
    mu      sync.RWMutex
}

func (m *MCPManager) RegisterServer(name, endpoint string) error {
    client, err := mcp.NewClient(mcp.ClientOptions{
        Transport: mcp.StdioTransport(endpoint),
    })
    if err != nil {
        return err
    }
    
    m.mu.Lock()
    m.servers[name] = client
    m.mu.Unlock()
    return nil
}

func (m *MCPManager) CallTool(serverName, toolName string, args interface{}) (interface{}, error) {
    client := m.servers[serverName]
    return client.CallTool(mcp.CallToolRequest{
        Name:      toolName,
        Arguments: args,
    })
}
```

### MCP Tool Plugin Integration

```go
type MCPToolPlugin struct {
    mcpManager *MCPManager
    serverName string
    tools      []tool.Tool
}

func (p *MCPToolPlugin) Initialize(config map[string]interface{}) error {
    endpoint := config["endpoint"].(string)
    return p.mcpManager.RegisterServer(p.serverName, endpoint)
}

// Implement ADK tool interface
func (p *MCPToolPlugin) Call(ctx context.Context, input interface{}) (interface{}, error) {
    return p.mcpManager.CallTool(p.serverName, "default", input)
}
```

## 5. File-based Configuration System

### Configuration Structure

```yaml
# config/system.yaml
system:
  name: "ModularAgent"
  version: "1.0.0"
  
models:
  primary:
    provider: "gemini"
    model: "gemini-2.0-flash-exp"
    apiKey: "${GOOGLE_API_KEY}"
  
  fallback:
    provider: "anthropic"
    model: "claude-3-sonnet"
    apiKey: "${ANTHROPIC_API_KEY}"

connectors:
  telegram:
    enabled: true
    token: "${TELEGRAM_BOT_TOKEN}"
    
  slack:
    enabled: true  
    token: "${SLACK_BOT_TOKEN}"
    signingSecret: "${SLACK_SIGNING_SECRET}"

mcpServers:
  - name: "filesystem"
    endpoint: "mcp-filesystem-server"
    
  - name: "kubernetes"  
    endpoint: "mcp-k8s-server"

tools:
  - name: "kubernetes"
    config:
      kubeconfig: "${KUBECONFIG}"
      defaultNamespace: "default"
```

### Prompt Management

```
prompts/
├── system/
│   ├── base.md                 # Core system prompt
│   ├── kubernetes-ops.md       # K8s-specific instructions  
│   └── chat-handler.md         # Chat platform behaviors
├── agents/
│   ├── log-analyzer.md         # Specialized agents
│   └── incident-responder.md   
└── examples/
    ├── k8s-queries.md          # Example interactions
    └── troubleshooting.md      
```

### Dynamic Configuration Loading

```go
type ConfigManager struct {
    basePath      string
    systemConfig  *SystemConfig
    prompts       map[string]string
    watchers      []fsnotify.Watcher
}

func (c *ConfigManager) LoadPrompt(name string) (string, error) {
    if cached, exists := c.prompts[name]; exists {
        return cached, nil
    }
    
    content, err := os.ReadFile(filepath.Join(c.basePath, "prompts", name+".md"))
    if err != nil {
        return "", err
    }
    
    c.prompts[name] = string(content)
    return string(content), nil
}

func (c *ConfigManager) WatchForChanges() {
    // Implement file watching for hot-reload
    watcher, _ := fsnotify.NewWatcher()
    watcher.Add(filepath.Join(c.basePath, "prompts"))
    
    go func() {
        for event := range watcher.Events {
            if event.Op&fsnotify.Write == fsnotify.Write {
                // Clear cache and notify agents of prompt updates
                delete(c.prompts, strings.TrimSuffix(event.Name, ".md"))
            }
        }
    }()
}
```

## 6. Kubernetes Integration

### Operator Pattern Implementation

Using Operator SDK for K8s-native deployment:

```go
// Custom Resource Definition
type AgentDeployment struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   AgentDeploymentSpec   `json:"spec,omitempty"`
    Status AgentDeploymentStatus `json:"status,omitempty"`
}

type AgentDeploymentSpec struct {
    ConnectorConfigs []ConnectorConfig `json:"connectors"`
    ModelConfig      ModelConfig       `json:"model"`
    ToolConfigs      []ToolConfig      `json:"tools"`
    Replicas         int32             `json:"replicas"`
}

// Controller reconciliation
func (r *AgentDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var agentDeploy AgentDeployment
    err := r.Get(ctx, req.NamespacedName, &agentDeploy)
    if err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // Create/update Deployment based on agent configuration
    deployment := r.buildDeployment(&agentDeploy)
    return ctrl.Result{}, r.applyDeployment(ctx, deployment)
}
```

### Kubernetes Tools Plugin

```go
type KubernetesTool struct {
    clientset kubernetes.Interface
    namespace string
}

func (k *KubernetesTool) GetPods(ctx context.Context, req *GetPodsRequest) (*GetPodsResponse, error) {
    pods, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
        LabelSelector: req.LabelSelector,
    })
    if err != nil {
        return nil, err
    }
    
    return &GetPodsResponse{Pods: pods.Items}, nil
}

func (k *KubernetesTool) GetLogs(ctx context.Context, req *GetLogsRequest) (*GetLogsResponse, error) {
    podLogOpts := corev1.PodLogOptions{
        Container:  req.Container,
        Follow:     false,
        TailLines:  &req.Lines,
    }
    
    request := k.clientset.CoreV1().Pods(k.namespace).
        GetLogs(req.PodName, &podLogOpts)
    
    logs, err := request.Stream(ctx)
    if err != nil {
        return nil, err
    }
    defer logs.Close()
    
    content, err := io.ReadAll(logs)
    return &GetLogsResponse{Logs: string(content)}, err
}
```

## 7. Agent Spawning and Session Management

### Subagent Spawning Pattern

Leveraging ADK's multi-agent capabilities:

```go
type AgentSpawner struct {
    agentTemplates map[string]*llmagent.Config
    sessionManager *session.Manager
    activeAgents   map[string]*agent.Agent
    mu            sync.RWMutex
}

func (s *AgentSpawner) SpawnAgent(agentType string, sessionID string, task string) (*agent.Agent, error) {
    template := s.agentTemplates[agentType]
    if template == nil {
        return nil, fmt.Errorf("unknown agent type: %s", agentType)
    }
    
    // Create specialized agent instance
    agentConfig := *template // Copy template
    agentConfig.Instructions = fmt.Sprintf("%s\n\nTask: %s", template.Instructions, task)
    
    spawnedAgent, err := llmagent.New(agentConfig)
    if err != nil {
        return nil, err
    }
    
    // Register with session
    s.mu.Lock()
    agentKey := fmt.Sprintf("%s-%s", sessionID, agentType)
    s.activeAgents[agentKey] = spawnedAgent
    s.mu.Unlock()
    
    return spawnedAgent, nil
}

// Example: Spawn K8s troubleshooting agent
func (s *AgentSpawner) SpawnK8sAgent(sessionID string, issue string) (*agent.Agent, error) {
    return s.SpawnAgent("k8s-troubleshooter", sessionID, 
        fmt.Sprintf("Investigate and resolve: %s", issue))
}
```

### Session State Management

Using ADK's session system:

```go
type SessionState struct {
    UserID      string                 `json:"user_id"`
    Platform    string                 `json:"platform"`
    Context     map[string]interface{} `json:"context"`
    ActiveTasks []TaskState            `json:"active_tasks"`
    History     []Interaction          `json:"history"`
}

type TaskState struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Status    string    `json:"status"`
    AgentID   string    `json:"agent_id"`
    CreatedAt time.Time `json:"created_at"`
    Data      interface{} `json:"data"`
}

func (as *AgentSystem) ProcessMessage(userID, platform, message string) (string, error) {
    // Get or create session using ADK session management
    sess := as.sessionManager.GetOrCreate(userID)
    
    // Load session state
    var state SessionState
    if data, err := sess.State().Get("user_state"); err == nil {
        json.Unmarshal(data.([]byte), &state)
    }
    
    // Process with main orchestrator agent
    ctx := agent.InvocationContext{
        Session: sess,
        Input: message,
    }
    
    response := as.orchestrator.Process(ctx)
    
    // Update state
    state.History = append(state.History, Interaction{
        Input: message,
        Output: response,
        Timestamp: time.Now(),
    })
    
    stateJSON, _ := json.Marshal(state)
    sess.State().Set("user_state", stateJSON)
    
    return response, nil
}
```

## 8. Implementation Roadmap

### Phase 1: Core Foundation (Weeks 1-2)
1. Set up project structure with Go modules
2. Implement basic ADK integration
3. Create plugin registry and interface definitions
4. Build configuration management system
5. Implement file-based prompt loading

### Phase 2: Chat Connectors (Weeks 3-4)  
1. Implement Telegram connector using go-telegram/bot
2. Implement Slack connector using slack-go/slack
3. Create message routing and handler system
4. Add basic session management
5. Test multi-platform message flow

### Phase 3: Tool System (Weeks 5-6)
1. Integrate MCP client libraries
2. Build Kubernetes tools plugin
3. Implement tool registration and execution
4. Create MCP server discovery mechanism
5. Add tool result formatting

### Phase 4: Agent Management (Weeks 7-8)
1. Implement agent spawning system
2. Add subagent session management  
3. Create agent templates and configurations
4. Build task state tracking
5. Implement agent lifecycle management

### Phase 5: Kubernetes Integration (Weeks 9-10)
1. Create Kubernetes operator using Operator SDK
2. Implement custom resource definitions
3. Build deployment automation
4. Add monitoring and observability
5. Create Helm charts for deployment

### Phase 6: Advanced Features (Weeks 11-12)
1. Add hot-reload for configuration changes
2. Implement agent plugin discovery
3. Build metrics and monitoring dashboard
4. Add advanced logging and tracing
5. Performance optimization and testing

## 9. Directory Structure

```
modular-agent-system/
├── cmd/
│   ├── agent/                  # Main agent binary
│   ├── operator/               # K8s operator
│   └── cli/                    # Management CLI
├── internal/
│   ├── agent/                  # Core agent logic
│   ├── config/                 # Configuration management
│   ├── connectors/             # Chat platform connectors
│   │   ├── telegram/
│   │   ├── slack/
│   │   └── discord/
│   ├── plugins/                # Plugin system
│   │   ├── registry/
│   │   ├── mcp/
│   │   └── tools/
│   ├── k8s/                    # Kubernetes integration
│   │   ├── operator/
│   │   ├── controller/
│   │   └── tools/
│   └── session/                # Session management
├── pkg/                        # Public API
│   ├── types/
│   ├── client/
│   └── sdk/
├── configs/
│   ├── system.yaml
│   ├── connectors/
│   └── tools/
├── prompts/
│   ├── system/
│   ├── agents/
│   └── examples/
├── deploy/
│   ├── k8s/
│   ├── helm/
│   └── docker/
├── docs/
│   ├── architecture/
│   ├── plugins/
│   └── deployment/
└── examples/
    ├── basic-setup/
    ├── k8s-bot/
    └── custom-plugin/
```

## 10. Deployment Patterns

### Container Deployment

```dockerfile
# Multi-stage build
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o agent cmd/agent/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/agent .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/prompts ./prompts
CMD ["./agent"]
```

### Kubernetes Manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: modular-agent
spec:
  replicas: 2
  selector:
    matchLabels:
      app: modular-agent
  template:
    metadata:
      labels:
        app: modular-agent
    spec:
      containers:
      - name: agent
        image: modular-agent:latest
        env:
        - name: GOOGLE_API_KEY
          valueFrom:
            secretKeyRef:
              name: agent-secrets
              key: google-api-key
        - name: TELEGRAM_BOT_TOKEN
          valueFrom:
            secretKeyRef:
              name: agent-secrets  
              key: telegram-token
        volumeMounts:
        - name: config
          mountPath: /root/configs
        - name: prompts
          mountPath: /root/prompts
      volumes:
      - name: config
        configMap:
          name: agent-config
      - name: prompts
        configMap:
          name: agent-prompts
```

## 11. Key Benefits

### Technical Advantages
- **Leverages Google ADK**: Built on proven, production-ready framework
- **Model Agnostic**: Easy switching between LLM providers  
- **Kubernetes Native**: Follows cloud-native patterns
- **Plugin Architecture**: Extensible without core changes
- **Type Safety**: Go's strong typing prevents runtime errors
- **Concurrency**: Go's goroutines handle multiple conversations
- **Performance**: Compiled binary with minimal resource usage

### Operational Benefits
- **File-based Config**: Version-controlled configuration and prompts
- **Hot Reload**: Update prompts without restarting
- **Observability**: Built-in metrics and logging
- **Scalability**: Horizontal scaling via Kubernetes
- **Security**: Secure secrets management and RBAC
- **Maintainability**: Clear separation of concerns

### Developer Experience
- **Plugin SDK**: Easy to extend with new tools
- **Documentation**: Comprehensive guides and examples
- **Testing**: Unit and integration test frameworks
- **CLI Tools**: Management and debugging utilities
- **IDE Support**: Full Go toolchain integration

## 12. First Use Case: Slack K8s Bot

### Implementation Priority

For the initial Slack bot for Kubernetes log/pod queries:

```go
// K8s Slack Bot specialized agent
func NewK8sSlackBot(config *Config) *AgentSystem {
    k8sTool := &KubernetesTool{
        clientset: config.KubeClient,
        namespace: config.DefaultNamespace,
    }
    
    agent, _ := llmagent.New(llmagent.Config{
        Name: "K8sBot",
        Instructions: loadPrompt("k8s-operations"),
        Tools: []tool.Tool{k8sTool},
    })
    
    system := &AgentSystem{
        orchestrator: agent,
        chatHandlers: make(map[string]*agent.Agent),
    }
    
    // Register Slack connector
    slackConnector := &SlackConnector{
        token: config.SlackToken,
    }
    system.RegisterConnector("slack", slackConnector)
    
    return system
}
```

### Example Interactions

```
User: "Show me pods in production namespace"
Bot: "Here are the pods in the production namespace:
     • api-server-7d8f9 (Running, 2/2 ready)
     • database-5c4b1 (Running, 1/1 ready)  
     • worker-queue-8a3d2 (Pending, 0/1 ready)
     
     I notice worker-queue-8a3d2 is pending. Would you like me to investigate?"

User: "Yes, check the logs"
Bot: "Checking logs for worker-queue-8a3d2...
     
     Error found: ImagePullBackOff - unable to pull image 'myapp:v2.1.5'
     
     This appears to be an image availability issue. The image tag v2.1.5 
     may not exist in your registry. Would you like me to:
     1. Check recent deployments
     2. List available image tags
     3. Show deployment configuration?"
```

This architecture provides a solid foundation for building sophisticated agent systems while maintaining flexibility and scalability through its modular design.