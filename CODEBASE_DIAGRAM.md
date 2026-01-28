# General Purpose Chatbot - Codebase Architecture

```
📁 general_purpose_chatbot/
├── 📚 Documentation & Planning
│   ├── README.md              # Project overview & vision
│   ├── IMPLEMENTATION_PLAN.md # Development roadmap
│   ├── CLAUDE_STRATEGY.md     # AI model strategy
│   ├── NEXT_STEPS.md          # Current tasks
│   └── PRODUCTION.md          # Deployment guide
│
├── 🚀 Entry Points (cmd/)
│   ├── chatbot/               # Main chatbot service
│   │   └── main.go           # Primary application entry
│   ├── cli/                  # Command-line interface
│   │   └── main.go          # CLI tool entry point
│   └── slack-bot/            # Slack-specific bot
│       └── main.go          # Slack bot entry point
│
├── 🧠 Core Business Logic (internal/)
│   ├── agents/               # Agent management & registry
│   │   ├── registry.go      # Agent factory & registration
│   │   └── slack.go         # Slack-specific agent logic
│   │
│   ├── models/               # AI model integrations
│   │   └── anthropic/        # Claude/Anthropic integration
│   │       ├── claude.go    # Claude API client
│   │       ├── claude_test.go # Unit tests
│   │       └── transform.go # Message transformations
│   │
│   ├── connectors/           # Platform integrations
│   │   ├── bridge/          # Generic connector bridge
│   │   │   └── bridge.go   # Platform abstraction layer
│   │   └── slack/           # Slack connector
│   │       └── connector.go # Slack API integration
│   │
│   ├── cli/                 # CLI-specific internals
│   │   ├── persistence/     # Database layer
│   │   │   ├── migrations/  # SQL migrations
│   │   │   │   ├── 001_create_users_table.up.sql
│   │   │   │   └── 001_create_users_table.down.sql
│   │   │   ├── sqlc/        # Generated SQL code
│   │   │   │   ├── db.go
│   │   │   │   ├── models.go
│   │   │   │   ├── querier.go
│   │   │   │   └── queries.sql.go
│   │   │   ├── queries.sql  # SQL queries
│   │   │   ├── repository.go # Data access layer
│   │   │   └── migrations.go # Migration management
│   │   └── simple_server.go # Basic HTTP server
│   │
│   ├── config/              # Configuration management
│   │   └── config.go       # App configuration
│   ├── middleware/          # HTTP middleware
│   │   └── recovery.go     # Panic recovery
│   └── monitoring/          # Health & metrics
│       └── health.go       # Health check endpoints
│
├── 📦 Shared Libraries (pkg/)
│   ├── config/             # Shared configuration utilities
│   ├── health/             # Health check framework
│   │   └── checkers/       # Health check implementations
│   ├── httpmiddleware/     # HTTP middleware
│   ├── logger/             # Structured logging
│   ├── metrics/            # Prometheus metrics
│   ├── prefixed_uuid/      # UUID utilities
│   └── utils/              # General utilities
│
├── 📋 Configuration
│   ├── config/             # Configuration files
│   │   └── agents/         # Agent-specific config
│   ├── examples/           # Usage examples
│   └── sqlc.yaml          # SQL code generation config
│
├── 📖 Documentation
│   ├── docs/
│   │   ├── architecture/   # Technical design docs
│   │   └── research/       # Market research & requirements
│   └── scripts/            # Build & deployment scripts
│
└── 🔧 DevOps
    └── .github/
        └── workflows/
            └── ci.yml      # Continuous integration
```

## Architecture Overview

### 🏗️ Core Components

1. **Entry Points (`cmd/`)**
   - Multiple deployment modes: standalone chatbot, CLI tool, Slack bot
   - Each with its own main.go for different use cases

2. **Business Logic (`internal/`)**
   - **Agents**: Core agent orchestration and platform-specific logic
   - **Models**: AI model integrations (currently Anthropic Claude)
   - **Connectors**: Platform abstraction layer for Slack, Telegram, etc.
   - **Persistence**: Database layer with SQLC-generated queries

3. **Shared Libraries (`pkg/`)**
   - Reusable components for logging, metrics, health checks
   - HTTP middleware and utilities
   - Configuration management

### 🔄 Data Flow

```
Chat Platform (Slack/Telegram/Discord)
         ↓
    Connector (bridge)
         ↓
    Agent Registry
         ↓
    AI Model (Claude)
         ↓
    Response Processing
         ↓
    Back to Platform
```

### 🎯 Key Features

- **Multi-platform support** via connector abstraction
- **AI-powered conversations** using Anthropic Claude
- **Database persistence** with migrations and SQLC
- **Health monitoring** and metrics collection  
- **Kubernetes-ready** architecture
- **Hot-reloadable configuration**

### 🚀 Current Status
- ✅ Core architecture defined
- ✅ Database layer implemented
- ✅ Slack connector built
- 🔧 Ready for Google ADK integration
- 🔧 MCP server integration planned