# General Purpose Chatbot Framework Research

**Request Date**: 2026-01-28  
**Requester**: Lewis  
**Focus**: General-purpose (not K8s-specific) extensible chatbot framework with plugin system

## Top Contenders - General Purpose Frameworks

### **1. Flottbot (⭐ Best Match for Requirements)**
- **GitHub**: https://github.com/target/flottbot
- **Language**: Go 
- **Company**: Target (Enterprise backing)
- **License**: Apache 2.0 (Open Source)

**Key Features**:
- ✅ **YAML Configuration**: All bot behavior configured via YAML files
- ✅ **Multi-language Scripts**: Write functionality in any language, not just Go
- ✅ **Multi-platform**: Slack (✔), Discord (🚧), Google Chat (🚧), Telegram (🚧)
- ✅ **Lightweight**: "Dumb" bots that call external scripts/APIs
- ✅ **Container Ready**: Multiple Docker images with language runtimes
- ✅ **Kubernetes**: Helm chart available
- ✅ **Plugin Philosophy**: External scripts contain business logic

**Architecture Philosophy**: 
- Simple, lightweight bots that interact with APIs and scripts
- Business logic housed in external scripts (any language)
- Configuration-driven approach

**Status**: ✅ Actively maintained, production-ready

---

### **2. go-chat-bot (Classic Multi-Platform)**
- **GitHub**: https://github.com/go-chat-bot/bot
- **Language**: Go
- **License**: MIT (Open Source)

**Key Features**:
- ✅ **Multi-platform**: IRC, Slack, Telegram, RocketChat, Google Chat
- ✅ **Plugin System**: Import-based plugin architecture
- ✅ **Mature**: Well-established codebase
- ✅ **Simple**: Straightforward plugin development

**Limitations**:
- Plugins must be written in Go
- Requires rebuilding bot for new plugins
- Less configuration-driven than Flottbot

**Status**: ✅ Maintained, proven in production

---

## **Classic ChatOps Frameworks (Non-Go)**

### **3. Hubot (JavaScript/CoffeeScript)**
- **Most Popular**: Industry standard for ChatOps
- **Language**: JavaScript/CoffeeScript (Node.js)
- **Platforms**: Slack, Discord, IRC, Shell, many adapters
- **Plugin Ecosystem**: Massive plugin library
- **Limitation**: JavaScript-based, not Go

### **4. Errbot (Python)**
- **Language**: Python
- **Features**: Plugin system, web UI, built-in admin commands
- **Platforms**: Slack, Discord, Telegram, IRC, many more
- **Strength**: Very extensible, great for Python shops
- **Limitation**: Python-based, not Go

### **5. Lita (Ruby)**  
- **Language**: Ruby
- **Features**: Plugin system, comprehensive adapter list
- **Status**: Less active development
- **Limitation**: Ruby-based, not Go

---

## **Modern Automation/Workflow Integration**

### **6. StackStorm**
- **Purpose**: Event-driven automation platform  
- **ChatOps**: Has Hubot integration built-in
- **Features**: Workflow engine, rule engine, visual workflow builder
- **Complexity**: Heavy-weight, full automation platform
- **Use Case**: If you need full workflow orchestration beyond chat

### **7. n8n**
- **Purpose**: Workflow automation (Zapier alternative)
- **Features**: Visual workflow builder, 200+ integrations
- **ChatOps**: Can integrate with chat platforms as part of workflows
- **Language**: Node.js/TypeScript

---

## **Key Findings & Gap Analysis**

### **What Exists:**
✅ **Multi-platform chat connectors** (all frameworks support this)  
✅ **Plugin systems** (various approaches)  
✅ **Configuration-driven** behavior (Flottbot excels here)  
✅ **External script execution** (Flottbot's specialty)  

### **What's Missing:**
❌ **Google ADK integration** (none found)  
❌ **MCP server integration** (none found)  
❌ **Subagent spawning** (none found)  
❌ **File-based system prompts** (none found)  
❌ **Agent-to-agent communication** (none found)  

### **Lewis's Vision vs. Existing Tools**

**Closest Match**: **Flottbot** gets about 60% there:
- ✅ Go-based
- ✅ YAML configuration  
- ✅ External script execution
- ✅ Multi-platform chat support
- ✅ Enterprise-ready
- ❌ No Google ADK integration
- ❌ No MCP support  
- ❌ No subagent concepts
- ❌ No AI-native features

---

## **Strategic Recommendations**

### **Option 1: Extend Flottbot**
**Pros**: 
- Solid foundation with Target backing
- Already has multi-platform + YAML config
- Could fork and add Google ADK + MCP + subagents

**Cons**: 
- Would require significant modifications
- May not align with Target's roadmap

### **Option 2: Build New Framework** 
**Pros**:
- Perfect fit for your modular vision
- Google ADK native integration
- MCP support from day one  
- Subagent architecture designed in
- File-based prompt management

**Cons**:
- Development time vs. existing solution
- Need to build platform connectors from scratch

### **Option 3: Hybrid Approach**
**Study Flottbot's patterns** for:
- YAML configuration approach
- External script execution model  
- Multi-platform adapter patterns

**Build your own** with:
- Google ADK integration
- MCP server connectivity
- Subagent spawning
- Modern plugin architecture

---

## **Conclusion**

While excellent general-purpose chatbot frameworks exist (especially **Flottbot**), none provide the specific modern features Lewis envisions:

- Google ADK integration
- MCP server support  
- Subagent spawning
- File-based AI prompt management
- Agent-to-agent communication

**Verdict**: The Go modular agent architecture Lewis described would fill a genuine gap in the ecosystem. Existing tools provide great inspiration (especially Flottbot's configuration approach), but his vision represents a next-generation ChatOps/AI agent platform. 🚀