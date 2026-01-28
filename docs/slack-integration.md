# Slack Integration Guide

This guide explains how to set up the Slack Socket Mode connector with the ADK bridge for end-to-end message routing.

## Architecture Overview

```
Slack (Socket Mode) → Slack Connector → ADK Bridge → ADK Agent (Claude) → Response → Slack
```

## Components

1. **Slack Connector** (`internal/connectors/slack/connector.go`)
   - Handles Slack Socket Mode events
   - Processes direct messages and @mentions
   - Manages reconnection logic

2. **ADK Bridge** (`internal/connectors/bridge/bridge.go`)
   - Bridges between Slack and ADK REST API
   - Handles message format transformation
   - Manages session state

3. **Slack Bot** (`cmd/slack-bot/main.go`)
   - Entry point for the Slack integration
   - Coordinates between connector and bridge

## Setup Instructions

### 1. Create Slack App

1. Go to https://api.slack.com/apps
2. Click "Create New App" → "From scratch"
3. Name your app and select your workspace
4. Go to "OAuth & Permissions" and add these Bot Token Scopes:
   - `app_mentions:read` - Read mentions
   - `channels:history` - Read channel messages
   - `chat:write` - Send messages
   - `im:history` - Read DM history
   - `im:read` - Read DMs

5. Install the app to your workspace
6. Copy the "Bot User OAuth Token" (starts with `xoxb-`)

### 2. Enable Socket Mode

1. Go to "Socket Mode" in your app settings
2. Enable Socket Mode
3. Create an App-Level Token with `connections:write` scope
4. Copy the App Token (starts with `xapp-`)

### 3. Configure Event Subscriptions

1. Go to "Event Subscriptions"
2. Enable Events
3. Subscribe to these Bot Events:
   - `app_mention` - Bot mentions in channels
   - `message.im` - Direct messages to bot

### 4. Set Environment Variables

```bash
# Slack tokens
export SLACK_BOT_TOKEN="xoxb-your-bot-token"
export SLACK_APP_TOKEN="xapp-your-app-token"

# ADK configuration
export ADK_BASE_URL="http://localhost:8000"  # Optional, defaults to localhost:8000
export ADK_AGENT_NAME="slack_assistant"      # Optional, defaults to slack_assistant

# Claude API key (for ADK agent)
export ANTHROPIC_API_KEY="sk-ant-your-api-key"
```

### 5. Run the System

#### Terminal 1: Start ADK Server
```bash
cd /home/admin/clawd/code/general_purpose_chatbot
go run cmd/chatbot/main.go web api
```

The ADK server will start on http://localhost:8000 with both web UI and REST API.

#### Terminal 2: Start Slack Bot
```bash
cd /home/admin/clawd/code/general_purpose_chatbot
go run cmd/slack-bot/main.go
```

## Usage

### Direct Messages
Send a DM to your bot:
```
User: Hello, what's the weather like?
Bot: I'd be happy to help with weather information! However, I don't currently have access to real-time weather data...
```

### Channel Mentions
Mention the bot in a channel:
```
User: @YourBot can you help me with Go programming?
Bot: I'd love to help you with Go programming! What specific aspect would you like to learn about?
```

## Message Flow

1. **Slack Event** → Socket Mode receives event
2. **Event Processing** → Connector handles message/mention events
3. **Message Bridge** → Bridge transforms message to ADK format
4. **ADK API Call** → Bridge calls `/run` endpoint
5. **Agent Processing** → Claude processes message via ADK agent
6. **Response Bridge** → Bridge extracts response text
7. **Slack Response** → Connector sends message back to Slack

## API Format Examples

### ADK Request Format
```json
{
  "appName": "slack_assistant",
  "userId": "U1234567890",
  "sessionId": "slack_U1234567890_D1234567890",
  "newMessage": {
    "role": "user",
    "parts": [{"text": "Hello, how are you?"}]
  }
}
```

### ADK Response Format
```json
[
  {
    "id": "event1",
    "timestamp": 1643723400.123,
    "author": "slack_assistant",
    "content": {
      "role": "model",
      "parts": [{"text": "Hello! I'm doing well, thank you for asking. How can I help you today?"}]
    }
  }
]
```

## Error Handling

The system includes comprehensive error handling:

- **Connection Errors**: Automatic reconnection for Socket Mode
- **API Errors**: Graceful error messages sent to users
- **Session Management**: Automatic session creation
- **Rate Limiting**: Respects Slack rate limits

## Troubleshooting

### Bot not responding to DMs
- Check that the bot has `im:history` and `im:read` scopes
- Verify the bot token is correct
- Ensure Socket Mode is enabled

### Bot not responding to mentions
- Check that the bot has `app_mentions:read` scope
- Verify the bot is added to the channel
- Check event subscriptions are configured

### ADK connection issues
- Ensure ADK server is running on the configured URL
- Check that the agent name exists
- Verify network connectivity between services

### Debug mode
Set environment variable for verbose logging:
```bash
export DEBUG=true
go run cmd/slack-bot/main.go
```

## Development

### Testing locally
1. Use ngrok or similar tool if you need webhooks (not needed for Socket Mode)
2. Create a test Slack workspace
3. Use the ADK web UI at http://localhost:8000 to test agents directly

### Adding new features
- Modify `internal/connectors/slack/connector.go` for Slack-specific features
- Modify `internal/connectors/bridge/bridge.go` for ADK integration changes
- Add new event handlers in the connector as needed

## Security Considerations

- Keep Slack tokens secure and rotate them regularly
- Use HTTPS in production
- Implement proper logging without exposing sensitive data
- Consider implementing user access controls
- Validate all inputs from Slack events

## Production Deployment

For production deployment:
1. Use container orchestration (Docker/Kubernetes)
2. Set up monitoring and alerting
3. Implement health checks
4. Use secure secret management
5. Configure proper logging and metrics
6. Set up load balancing if needed

## Performance Notes

- Socket Mode has lower latency than webhooks
- Sessions are created per user-channel combination
- The bridge includes connection pooling and timeouts
- Consider caching strategies for high-traffic environments