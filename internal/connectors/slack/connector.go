package slack

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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

	// Cached bot identity (lazy-initialized via ensureBotIdentity)
	botUserID string
	botBotID  string
	initOnce  sync.Once

	// User display name cache to avoid repeated API calls
	userNameCache map[string]string
	cacheMu       sync.RWMutex
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
		client:        client,
		socketMode:    socketMode,
		executor:      exec,
		logger:        slackLogger,
		sessionMgr:    sessionMgr,
		userNameCache: make(map[string]string),
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

			case socketmode.EventTypeIncomingError:
				if err, ok := envelope.Data.(*slack.IncomingEventError); ok {
					c.logger.Warn("Incoming event error from Slack", logger.ErrorField(err.ErrorObj))
				} else {
					c.logger.Warn("Incoming event error from Slack", logger.StringField("data", fmt.Sprintf("%+v", envelope.Data)))
				}

			case socketmode.EventTypeErrorWriteFailed:
				if err, ok := envelope.Data.(*socketmode.ErrorWriteFailed); ok {
					c.logger.Error("Failed to write to Slack WebSocket", logger.ErrorField(err.Cause))
				} else {
					c.logger.Error("Failed to write to Slack WebSocket", logger.StringField("data", fmt.Sprintf("%+v", envelope.Data)))
				}

			case socketmode.EventTypeErrorBadMessage:
				if err, ok := envelope.Data.(*socketmode.ErrorBadMessage); ok {
					c.logger.Warn("Bad message received from Slack", logger.ErrorField(err.Cause), logger.StringField("message", string(err.Message)))
				} else {
					c.logger.Warn("Bad message received from Slack", logger.StringField("data", fmt.Sprintf("%+v", envelope.Data)))
				}

			case socketmode.EventTypeInvalidAuth:
				c.logger.Error("Invalid authentication for Slack Socket Mode")

			case socketmode.EventTypeDisconnect:
				c.logger.Warn("Disconnected from Slack Socket Mode")
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()

			default:
				c.logger.Warn("Unsupported event type received",
					logger.StringField("event_type", string(envelope.Type)),
					logger.StringField("data", fmt.Sprintf("%+v", envelope.Data)),
				)
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

	// Fetch the full message from the API so we get attachments, blocks, and files
	// (the AppMentionEvent only carries the plain Text field).
	cleanText := c.removeBotMention(c.fetchFullMessageText(ctx, event.Channel, event.TimeStamp, event.Text))

	// Fetch thread context if this is a reply in an existing thread
	threadContext := c.getThreadContext(ctx, event.Channel, threadTS, event.TimeStamp)

	// Compose the full message with thread context if available
	fullMessage := cleanText
	if threadContext != "" {
		userName := c.resolveUserName(ctx, event.User, "")
		fullMessage = fmt.Sprintf("%s\n\n%s's message to you: %s", threadContext, userName, cleanText)
	}

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
		Message:   fullMessage,
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

// ensureBotIdentity lazily fetches and caches the bot's own user ID and bot ID.
func (c *Connector) ensureBotIdentity() {
	c.initOnce.Do(func() {
		auth, err := c.client.AuthTest()
		if err != nil {
			c.logger.Warn("Failed to cache bot identity", logger.ErrorField(err))
			return
		}
		c.botUserID = auth.UserID
		c.botBotID = auth.BotID
	})
}

// resolveUserName resolves a Slack user ID or bot ID to a display name.
func (c *Connector) resolveUserName(ctx context.Context, userID, botID string) string {
	c.ensureBotIdentity()

	if botID != "" && botID == c.botBotID {
		return "You (assistant)"
	}
	if botID != "" {
		return "Bot"
	}
	if userID == "" {
		return "Unknown"
	}

	// Check cache
	c.cacheMu.RLock()
	if name, ok := c.userNameCache[userID]; ok {
		c.cacheMu.RUnlock()
		return name
	}
	c.cacheMu.RUnlock()

	// Fetch from API
	user, err := c.client.GetUserInfoContext(ctx, userID)
	if err != nil {
		return fmt.Sprintf("<@%s>", userID)
	}

	name := user.Name
	if user.Profile.DisplayName != "" {
		name = user.Profile.DisplayName
	} else if user.RealName != "" {
		name = user.RealName
	}

	c.cacheMu.Lock()
	c.userNameCache[userID] = name
	c.cacheMu.Unlock()

	return name
}

// extractMessageText extracts readable text from a Slack message, falling back
// through Text → Attachments → Blocks → Files when the primary text field is empty.
// This handles bot/integration messages (e.g. AlertManager) that put content in
// attachments or blocks rather than the plain text field.
func extractMessageText(msg slack.Message) string {
	if msg.Text != "" {
		return msg.Text
	}

	var parts []string

	// Try attachments (common for webhooks and integrations)
	for _, att := range msg.Attachments {
		if att.Pretext != "" {
			parts = append(parts, att.Pretext)
		}
		if att.Title != "" {
			parts = append(parts, att.Title)
		}
		if att.Text != "" {
			parts = append(parts, att.Text)
		}
		for _, field := range att.Fields {
			entry := field.Title
			if field.Value != "" {
				if entry != "" {
					entry += ": "
				}
				entry += field.Value
			}
			if entry != "" {
				parts = append(parts, entry)
			}
		}
		if len(parts) == 0 && att.Fallback != "" {
			parts = append(parts, att.Fallback)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}

	// Try blocks (Block Kit)
	for _, block := range msg.Blocks.BlockSet {
		switch b := block.(type) {
		case *slack.HeaderBlock:
			if b.Text != nil && b.Text.Text != "" {
				parts = append(parts, b.Text.Text)
			}
		case *slack.SectionBlock:
			if b.Text != nil && b.Text.Text != "" {
				parts = append(parts, b.Text.Text)
			}
			for _, field := range b.Fields {
				if field != nil && field.Text != "" {
					parts = append(parts, field.Text)
				}
			}
		case *slack.RichTextBlock:
			parts = append(parts, extractRichTextBlock(b)...)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}

	// Try files (file shares with no accompanying text)
	for _, file := range msg.Files {
		label := file.Title
		if label == "" {
			label = file.Name
		}
		if label != "" {
			parts = append(parts, fmt.Sprintf("[File: %s]", label))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}

	return ""
}

// extractRichTextBlock recursively extracts plain text from a RichTextBlock's elements.
func extractRichTextBlock(block *slack.RichTextBlock) []string {
	var parts []string
	for _, elem := range block.Elements {
		switch el := elem.(type) {
		case *slack.RichTextSection:
			var sectionText strings.Builder
			for _, se := range el.Elements {
				switch ste := se.(type) {
				case *slack.RichTextSectionTextElement:
					sectionText.WriteString(ste.Text)
				case *slack.RichTextSectionLinkElement:
					if ste.Text != "" {
						sectionText.WriteString(ste.Text)
					} else {
						sectionText.WriteString(ste.URL)
					}
				}
			}
			if sectionText.Len() > 0 {
				parts = append(parts, sectionText.String())
			}
		case *slack.RichTextList:
			for _, item := range el.Elements {
				if section, ok := item.(*slack.RichTextSection); ok {
					var itemText strings.Builder
					for _, se := range section.Elements {
						switch ste := se.(type) {
						case *slack.RichTextSectionTextElement:
							itemText.WriteString(ste.Text)
						case *slack.RichTextSectionLinkElement:
							if ste.Text != "" {
								itemText.WriteString(ste.Text)
							} else {
								itemText.WriteString(ste.URL)
							}
						}
					}
					if itemText.Len() > 0 {
						parts = append(parts, "- "+itemText.String())
					}
				}
			}
		case *slack.RichTextQuote:
			var quoteText strings.Builder
			for _, se := range el.Elements {
				switch ste := se.(type) {
				case *slack.RichTextSectionTextElement:
					quoteText.WriteString(ste.Text)
				case *slack.RichTextSectionLinkElement:
					if ste.Text != "" {
						quoteText.WriteString(ste.Text)
					} else {
						quoteText.WriteString(ste.URL)
					}
				}
			}
			if quoteText.Len() > 0 {
				parts = append(parts, "> "+quoteText.String())
			}
		case *slack.RichTextPreformatted:
			var codeText strings.Builder
			for _, se := range el.Elements {
				switch ste := se.(type) {
				case *slack.RichTextSectionTextElement:
					codeText.WriteString(ste.Text)
				case *slack.RichTextSectionLinkElement:
					if ste.Text != "" {
						codeText.WriteString(ste.Text)
					} else {
						codeText.WriteString(ste.URL)
					}
				}
			}
			if codeText.Len() > 0 {
				parts = append(parts, "```\n"+codeText.String()+"\n```")
			}
		}
	}
	return parts
}

// fetchFullMessageText retrieves the complete Slack message (with attachments, blocks, files)
// for a given channel and timestamp, and extracts readable text from it.
// Falls back to fallbackText if the API call fails or no richer content is found.
func (c *Connector) fetchFullMessageText(ctx context.Context, channelID, timestamp, fallbackText string) string {
	msgs, _, _, err := c.client.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: timestamp,
		Limit:     1,
		Inclusive: true,
	})
	if err != nil {
		c.logger.Warn("Failed to fetch full message, using event text",
			logger.StringField("channel", channelID),
			logger.StringField("ts", timestamp),
			logger.ErrorField(err))
		return fallbackText
	}

	for _, msg := range msgs {
		if msg.Timestamp == timestamp {
			if text := extractMessageText(msg); text != "" {
				return text
			}
			return fallbackText
		}
	}

	return fallbackText
}

// formatSlackTimestamp converts a Slack timestamp (e.g. "1234567890.123456") to
// a human-readable format like "[2026-02-16 09:12 UTC]".
func formatSlackTimestamp(ts string) string {
	parts := strings.SplitN(ts, ".", 2)
	if len(parts) == 0 {
		return ""
	}
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format("[2006-01-02 15:04 UTC]")
}

// getThreadContext fetches thread history and formats it as context for the LLM.
// Returns empty string if this is a new thread (no prior messages) or on error.
func (c *Connector) getThreadContext(ctx context.Context, channelID, threadTS, currentMsgTS string) string {
	// If this message starts the thread, there's no prior context
	if threadTS == currentMsgTS {
		return ""
	}

	msgs, hasMore, _, err := c.client.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
		Limit:     50,
	})
	if err != nil {
		c.logger.Warn("Failed to fetch thread replies",
			logger.StringField("channel", channelID),
			logger.StringField("thread_ts", threadTS),
			logger.ErrorField(err))
		return ""
	}

	if len(msgs) == 0 {
		return ""
	}

	var threadContext strings.Builder
	threadContext.WriteString("[Thread Context - Previous messages in this thread]\n")

	if hasMore {
		threadContext.WriteString("[...earlier messages omitted, showing most recent messages]\n")
	}

	hasContent := false
	for _, msg := range msgs {
		if msg.Timestamp == currentMsgTS {
			continue
		}

		displayName := c.resolveUserName(ctx, msg.User, msg.BotID)
		text := c.removeBotMention(extractMessageText(msg))
		if text == "" {
			continue
		}

		if ts := formatSlackTimestamp(msg.Timestamp); ts != "" {
			threadContext.WriteString(fmt.Sprintf("%s %s: %s\n", ts, displayName, text))
		} else {
			threadContext.WriteString(fmt.Sprintf("%s: %s\n", displayName, text))
		}
		hasContent = true
	}

	if !hasContent {
		return ""
	}

	threadContext.WriteString("[End of Thread Context]")
	return threadContext.String()
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
