package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
	"github.com/lewisedginton/general_purpose_chatbot/internal/session_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Connector represents the Slack Socket Mode connector
type Connector struct {
	client     *slack.Client
	socketMode *socketmode.Client
	executor   *executor.Executor
	logger     logger.Logger
	commands   *CommandRegistry
	sessionMgr session_manager.Manager
	connected  bool
	mu         sync.RWMutex
}

// Config holds configuration for the Slack connector
type Config struct {
	BotToken string        // xoxb-*
	AppToken string        // xapp-*
	Debug    bool          // Enable debug logging for Slack API and Socket Mode
	Logger   logger.Logger // Structured logger instance
}

// NewConnector creates a new Slack connector with in-process executor
func NewConnector(config Config, exec *executor.Executor, sessionMgr session_manager.Manager) (*Connector, error) {
	if !strings.HasPrefix(config.BotToken, "xoxb-") {
		return nil, fmt.Errorf("invalid bot token format, expected xoxb-*")
	}
	if !strings.HasPrefix(config.AppToken, "xapp-") {
		return nil, fmt.Errorf("invalid app token format, expected xapp-*")
	}
	if exec == nil {
		return nil, fmt.Errorf("executor is required")
	}
	if sessionMgr == nil {
		return nil, fmt.Errorf("session manager is required")
	}
	if config.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Initialize Slack clients
	client := slack.New(
		config.BotToken,
		slack.OptionAppLevelToken(config.AppToken),
		slack.OptionDebug(config.Debug),
	)
	socketMode := socketmode.New(client, socketmode.OptionDebug(config.Debug))

	// Create a logger with Slack-specific context
	slackLogger := config.Logger.WithFields(logger.StringField("connector", "slack"))

	connector := &Connector{
		client:     client,
		socketMode: socketMode,
		executor:   exec,
		logger:     slackLogger,
		sessionMgr: sessionMgr,
	}

	// Setup slash command handlers
	connector.setupCommands()

	return connector, nil
}

// Start begins the Socket Mode connection and event handling
func (c *Connector) Start(ctx context.Context) error {
	c.logger.Info("Starting Slack Socket Mode connector")

	// Handle various event types
	go func() {
		for envelope := range c.socketMode.Events {
			switch envelope.Type {
			case socketmode.EventTypeConnecting:
				c.logger.Info("Connecting to Slack with Socket Mode")
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()

			case socketmode.EventTypeConnectionError:
				c.logger.Error("Connection failed", logger.StringField("data", fmt.Sprintf("%v", envelope.Data)))
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()

			case socketmode.EventTypeConnected:
				c.logger.Info("Connected to Slack with Socket Mode")
				c.mu.Lock()
				c.connected = true
				c.mu.Unlock()

			case socketmode.EventTypeHello:
				// Hello event confirms WebSocket connection - no action needed

			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := envelope.Data.(slackevents.EventsAPIEvent)
				if !ok {
					c.logger.Warn("Ignored non-EventsAPI event", logger.StringField("data", fmt.Sprintf("%+v", envelope)))
					continue
				}

				c.logger.Debug("Event received", logger.StringField("event_type", eventsAPIEvent.Type))
				c.socketMode.Ack(*envelope.Request)

				err := c.handleEvent(ctx, eventsAPIEvent)
				if err != nil {
					c.logger.Error("Failed to handle event", logger.ErrorField(err))
				}

			case socketmode.EventTypeInteractive:
				c.logger.Debug("Interactive event received")
				c.socketMode.Ack(*envelope.Request)
				// Handle interactive events if needed

			case socketmode.EventTypeSlashCommand:
				c.handleSlashCommand(ctx, envelope)

			default:
				c.logger.Warn("Unsupported event type received", logger.StringField("event_type", string(envelope.Type)))
			}
		}
	}()

	// Start the connection
	return c.socketMode.RunContext(ctx)
}

// handleEvent processes Slack events and routes them to the agent
func (c *Connector) handleEvent(ctx context.Context, event slackevents.EventsAPIEvent) error {
	if event.Type == slackevents.CallbackEvent {
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			return c.handleMessageEvent(ctx, ev)
		case *slackevents.AppMentionEvent:
			return c.handleAppMentionEvent(ctx, ev)
		}
	}
	return nil
}

// handleMessageEvent processes direct messages to the bot
func (c *Connector) handleMessageEvent(ctx context.Context, event *slackevents.MessageEvent) error {
	// Skip messages from bots to avoid loops
	if event.BotID != "" || event.SubType == "bot_message" {
		c.logger.Debug("Skipping bot message",
			logger.StringField("bot_id", event.BotID),
			logger.StringField("sub_type", event.SubType))
		return nil
	}

	// Skip system/automated message subtypes
	systemSubtypes := map[string]bool{
		// Channel activity
		"channel_join": true, "channel_leave": true, "channel_topic": true,
		"channel_purpose": true, "channel_name": true, "channel_archive": true,
		"channel_unarchive": true, "channel_convert_to_private": true,
		"channel_convert_to_public": true, "channel_posting_permissions": true,
		// Group activity
		"group_join": true, "group_leave": true, "group_topic": true,
		"group_purpose": true, "group_name": true, "group_archive": true,
		"group_unarchive": true,
		// File operations
		"file_share": true, "file_comment": true, "file_mention": true,
		// Message metadata (hidden events)
		"message_changed": true, "message_deleted": true, "message_replied": true,
		// Other system messages
		"pinned_item": true, "unpinned_item": true, "reminder_add": true,
		"ekm_access_denied": true, "assistant_app_thread": true,
	}
	if systemSubtypes[event.SubType] {
		c.logger.Debug("Skipping system message", logger.StringField("sub_type", event.SubType))
		return nil
	}

	// Skip messages without a user ID (additional safety check for system messages)
	if event.User == "" {
		c.logger.Debug("Skipping message without user ID", logger.StringField("sub_type", event.SubType))
		return nil
	}

	// Only process direct messages (DMs have channel type starting with D)
	if !strings.HasPrefix(event.Channel, "D") {
		return nil
	}

	c.logger.Info("Processing DM",
		logger.StringField("user_id", event.User),
		logger.StringField("channel", event.Channel))

	// Send message to agent via executor
	// Get or create session for this user
	sessionID, err := c.sessionMgr.GetOrCreateSession(ctx, "slack", event.User, event.Channel)
	if err != nil {
		c.logger.Error("Error getting session", logger.ErrorField(err))
		return fmt.Errorf("failed to get session: %w", err)
	}

	response, err := c.executor.Execute(ctx, executor.MessageRequest{
		UserID:    event.User,
		SessionID: sessionID,
		Message:   event.Text,
	}, c, func() string {
		return c.GetUserInfo(ctx, event.User)
	})
	if err != nil {
		c.logger.Error("Error from executor", logger.ErrorField(err))
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText("Sorry, I encountered an error processing your message.", false))
		return err
	}

	// Send response back to Slack
	if response.Text != "" {
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText(response.Text, false))
		if err != nil {
			c.logger.Error("Error sending message to Slack", logger.ErrorField(err))
			return err
		}
	}

	return nil
}

// handleAppMentionEvent processes @bot mentions in channels
func (c *Connector) handleAppMentionEvent(ctx context.Context, event *slackevents.AppMentionEvent) error {
	// Determine thread root: if already in a thread use that TS, otherwise this message starts the thread
	threadTS := event.ThreadTimeStamp
	if threadTS == "" {
		threadTS = event.TimeStamp
	}

	c.logger.Info("Processing mention",
		logger.StringField("user_id", event.User),
		logger.StringField("channel", event.Channel),
		logger.StringField("thread_ts", threadTS))

	// Remove the bot mention from the message text
	cleanText := c.removeBotMention(event.Text)

	// Thread-scoped session: all users in the same thread share one session
	scopeKey := fmt.Sprintf("thread:%s:%s", event.Channel, threadTS)

	sessionID, err := c.sessionMgr.GetOrCreateSession(ctx, "slack", scopeKey, event.Channel)
	if err != nil {
		c.logger.Error("Error getting session", logger.ErrorField(err))
		return fmt.Errorf("failed to get session: %w", err)
	}

	response, err := c.executor.Execute(ctx, executor.MessageRequest{
		UserID:    scopeKey,
		SessionID: sessionID,
		Message:   cleanText,
	}, c, func() string {
		return c.GetUserInfo(ctx, event.User)
	})
	if err != nil {
		c.logger.Error("Error from executor", logger.ErrorField(err))
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText("Sorry, I encountered an error processing your message.", false),
			slack.MsgOptionTS(threadTS))
		return err
	}

	// Send response back in the thread
	if response.Text != "" {
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText(response.Text, false),
			slack.MsgOptionTS(threadTS))
		if err != nil {
			c.logger.Error("Error sending message to Slack", logger.ErrorField(err))
			return err
		}
	}

	return nil
}

// removeBotMention removes @bot mentions from message text
func (c *Connector) removeBotMention(text string) string {
	// Remove <@UBOT_ID> mentions - this is a simplified approach
	// In production, you'd want to get the actual bot user ID and remove it properly
	cleaned := text
	// Simple regex-like removal for mentions
	if strings.Contains(text, "<@") {
		// Find and remove the mention part
		start := strings.Index(text, "<@")
		end := strings.Index(text[start:], ">")
		if end != -1 {
			cleaned = strings.TrimSpace(text[:start] + text[start+end+1:])
		}
	}
	return cleaned
}

// Stop gracefully stops the connector
func (c *Connector) Stop() error {
	c.logger.Info("Stopping Slack connector")
	// socketmode client doesn't have a direct stop method,
	// stopping is handled by context cancellation in RunContext
	return nil
}

// GetBotInfo returns information about the bot
func (c *Connector) GetBotInfo() (*slack.Bot, error) {
	auth, err := c.client.AuthTest()
	if err != nil {
		return nil, err
	}

	return c.client.GetBotInfo(slack.GetBotInfoParameters{Bot: auth.BotID})
}

// PlatformName returns the platform name
func (c *Connector) PlatformName() string {
	return "Slack"
}

// UserInfo returns user context information (legacy method for interface compatibility)
func (c *Connector) UserInfo() string {
	// This method is kept for backward compatibility but should not be used directly
	return ""
}

// GetUserInfo fetches user information from Slack API and returns a formatted string
func (c *Connector) GetUserInfo(ctx context.Context, userID string) string {
	if userID == "" {
		return ""
	}

	user, err := c.client.GetUserInfo(userID)
	if err != nil {
		c.logger.Warn("Failed to fetch user info",
			logger.StringField("user_id", userID),
			logger.ErrorField(err))
		return ""
	}

	// Format user information
	info := fmt.Sprintf("- User ID: %s\n", user.ID)

	if user.RealName != "" {
		info += fmt.Sprintf("- Real Name: %s\n", user.RealName)
	}

	if user.Name != "" {
		info += fmt.Sprintf("- Username: @%s\n", user.Name)
	}

	if user.Profile.DisplayName != "" && user.Profile.DisplayName != user.Name {
		info += fmt.Sprintf("- Display Name: %s\n", user.Profile.DisplayName)
	}

	if user.Profile.Email != "" {
		info += fmt.Sprintf("- Email: %s\n", user.Profile.Email)
	}

	if user.Profile.Title != "" {
		info += fmt.Sprintf("- Title: %s\n", user.Profile.Title)
	}

	if user.TZ != "" {
		info += fmt.Sprintf("- Timezone: %s\n", user.TZ)
	}

	return info
}

// FormattingGuide returns Slack-specific formatting instructions
func (c *Connector) FormattingGuide() string {
	return `# Slack Formatting Guide

## Text Formatting
- *Bold text*: Wrap text in asterisks (e.g., *bold*)
- _Italic text_: Wrap text in underscores (e.g., _italic_)
- ~Strikethrough~: Wrap text in tildes (e.g., ~strikethrough~)
- Inline code: Wrap text in backticks (e.g., ` + "`code`" + `)

## Code Blocks
Use triple backticks for multi-line code blocks:
` + "```" + `
code block
` + "```" + `

## Lists
- Use hyphens or asterisks for bullet points
- Lists must have blank lines before and after
- Each item on a new line

## Links
- Inline links: <https://example.com|Link Text>
- Auto-links: <https://example.com>

## Mentions
- User mentions: <@USER_ID>
- Channel mentions: <#CHANNEL_ID>

## Quotes
Use > at the start of a line for block quotes:
> This is a quote

## Important Notes
- Slack uses mrkdwn (a simplified Markdown variant), not full Markdown
- HTML tags are not supported and will be displayed as plain text
- Emoji can be used with :emoji_name: syntax`
}

// Ready returns nil if the Slack connector is connected and ready to receive requests,
// or an error if it's not ready.
func (c *Connector) Ready() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("slack connector not connected")
	}

	return nil
}
