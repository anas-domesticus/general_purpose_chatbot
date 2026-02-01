package slack

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Connector represents the Slack Socket Mode connector
type Connector struct {
	client     *slack.Client
	socketMode *socketmode.Client
	executor   *executor.Executor
	logger     *log.Logger
}

// Config holds configuration for the Slack connector
type Config struct {
	BotToken string // xoxb-*
	AppToken string // xapp-*
	Debug    bool   // Enable debug logging for Slack API and Socket Mode
}

// NewConnector creates a new Slack connector with in-process executor
func NewConnector(config Config, exec *executor.Executor) (*Connector, error) {
	if !strings.HasPrefix(config.BotToken, "xoxb-") {
		return nil, fmt.Errorf("invalid bot token format, expected xoxb-*")
	}
	if !strings.HasPrefix(config.AppToken, "xapp-") {
		return nil, fmt.Errorf("invalid app token format, expected xapp-*")
	}
	if exec == nil {
		return nil, fmt.Errorf("executor is required")
	}

	// Initialize Slack clients
	client := slack.New(
		config.BotToken,
		slack.OptionAppLevelToken(config.AppToken),
		slack.OptionDebug(config.Debug),
	)
	socketMode := socketmode.New(client, socketmode.OptionDebug(config.Debug))

	logger := log.New(os.Stdout, "[SLACK-CONNECTOR] ", log.LstdFlags|log.Lshortfile)

	return &Connector{
		client:     client,
		socketMode: socketMode,
		executor:   exec,
		logger:     logger,
	}, nil
}

// Start begins the Socket Mode connection and event handling
func (c *Connector) Start(ctx context.Context) error {
	c.logger.Println("Starting Slack Socket Mode connector...")

	// Handle various event types
	go func() {
		for envelope := range c.socketMode.Events {
			switch envelope.Type {
			case socketmode.EventTypeConnecting:
				c.logger.Println("Connecting to Slack with Socket Mode...")

			case socketmode.EventTypeConnectionError:
				c.logger.Printf("Connection failed: %v", envelope.Data)

			case socketmode.EventTypeConnected:
				c.logger.Println("Connected to Slack with Socket Mode")

			case socketmode.EventTypeHello:
				// Hello event confirms WebSocket connection - no action needed

			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := envelope.Data.(slackevents.EventsAPIEvent)
				if !ok {
					c.logger.Printf("Ignored event: %+v", envelope)
					continue
				}

				c.logger.Printf("Event received: %s", eventsAPIEvent.Type)
				c.socketMode.Ack(*envelope.Request)

				err := c.handleEvent(ctx, eventsAPIEvent)
				if err != nil {
					c.logger.Printf("Failed to handle event: %v", err)
				}

			case socketmode.EventTypeInteractive:
				c.logger.Printf("Interactive event received")
				c.socketMode.Ack(*envelope.Request)
				// Handle interactive events if needed

			case socketmode.EventTypeSlashCommand:
				c.logger.Printf("Slash command received")
				c.socketMode.Ack(*envelope.Request)
				// Handle slash commands if needed

			default:
				c.logger.Printf("Unsupported event type received: %s", envelope.Type)
			}
		}
	}()

	// Start the connection
	return c.socketMode.RunContext(ctx)
}

// handleEvent processes Slack events and routes them to the agent
func (c *Connector) handleEvent(ctx context.Context, event slackevents.EventsAPIEvent) error {
	switch event.Type {
	case slackevents.CallbackEvent:
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
		c.logger.Printf("Skipping bot message (BotID: %s, SubType: %s)", event.BotID, event.SubType)
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
		c.logger.Printf("Skipping system message (SubType: %s)", event.SubType)
		return nil
	}

	// Skip messages without a user ID (additional safety check for system messages)
	if event.User == "" {
		c.logger.Printf("Skipping message without user ID (SubType: %s)", event.SubType)
		return nil
	}

	// Only process direct messages (DMs have channel type starting with D)
	if !strings.HasPrefix(event.Channel, "D") {
		return nil
	}

	c.logger.Printf("Processing DM from user %s: %s", event.User, event.Text)

	// Send message to agent via executor
	response, err := c.executor.Execute(ctx, executor.MessageRequest{
		UserID:    event.User,
		SessionID: fmt.Sprintf("slack_%s_%s", event.User, event.Channel),
		Message:   event.Text,
	}, c)

	if err != nil {
		c.logger.Printf("Error from executor: %v", err)
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText("Sorry, I encountered an error processing your message.", false))
		return err
	}

	// Send response back to Slack
	if response.Text != "" {
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText(response.Text, false))
		if err != nil {
			c.logger.Printf("Error sending message to Slack: %v", err)
			return err
		}
	}

	return nil
}

// handleAppMentionEvent processes @bot mentions in channels
func (c *Connector) handleAppMentionEvent(ctx context.Context, event *slackevents.AppMentionEvent) error {
	c.logger.Printf("Processing mention from user %s in channel %s: %s", event.User, event.Channel, event.Text)

	// Remove the bot mention from the message text
	cleanText := c.removeBotMention(event.Text)

	// Send message to agent via executor
	response, err := c.executor.Execute(ctx, executor.MessageRequest{
		UserID:    event.User,
		SessionID: fmt.Sprintf("slack_%s_%s", event.User, event.Channel),
		Message:   cleanText,
	}, c)

	if err != nil {
		c.logger.Printf("Error from executor: %v", err)
		// Send error message to channel
		_, _, err = c.client.PostMessage(event.Channel, slack.MsgOptionText("Sorry, I encountered an error processing your message.", false))
		return err
	}

	// Send response back to Slack
	if response.Text != "" {
		_, _, err = c.client.PostMessage(event.Channel, slack.MsgOptionText(response.Text, false))
		if err != nil {
			c.logger.Printf("Error sending message to Slack: %v", err)
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
	c.logger.Println("Stopping Slack connector...")
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
