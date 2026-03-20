package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"

	acpclient "github.com/lewisedginton/general_purpose_chatbot/internal/acp"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"go.uber.org/zap"
)

// Connector represents the Slack Socket Mode connector.
type Connector struct {
	client      *slack.Client
	socketMode  *socketmode.Client
	acpExecutor *acpclient.Executor
	acpRouter   *acpclient.Router
	logger      *zap.SugaredLogger
	commands    *CommandRegistry
	connected   bool
	mu          sync.RWMutex
}

// Config holds configuration for the Slack connector.
type Config struct {
	BotToken string
	AppToken string
	Debug    bool
}

// NewConnector creates a new Slack connector wired to ACP.
func NewConnector(config Config, acpExec *acpclient.Executor, acpRouter *acpclient.Router, log *zap.SugaredLogger) (*Connector, error) {
	if !strings.HasPrefix(config.BotToken, "xoxb-") {
		return nil, fmt.Errorf("invalid bot token format, expected xoxb-*")
	}
	if !strings.HasPrefix(config.AppToken, "xapp-") {
		return nil, fmt.Errorf("invalid app token format, expected xapp-*")
	}
	if acpExec == nil {
		return nil, fmt.Errorf("acp executor is required")
	}
	if acpRouter == nil {
		return nil, fmt.Errorf("acp router is required")
	}
	if log == nil {
		return nil, fmt.Errorf("logger is required") //nolint:goerr113 // simple validation
	}

	client := slack.New(
		config.BotToken,
		slack.OptionAppLevelToken(config.AppToken),
		slack.OptionDebug(config.Debug),
	)
	sm := socketmode.New(client, socketmode.OptionDebug(config.Debug))

	slackLogger := log.With("connector", "slack")

	connector := &Connector{
		client:      client,
		socketMode:  sm,
		acpExecutor: acpExec,
		acpRouter:   acpRouter,
		logger:      slackLogger,
	}

	connector.setupCommands()

	return connector, nil
}

// Start begins the Socket Mode connection and event handling.
func (c *Connector) Start(ctx context.Context) error {
	c.logger.Info("Starting Slack Socket Mode connector")

	go func() {
		for envelope := range c.socketMode.Events {
			c.handleSocketEvent(ctx, envelope)
		}
	}()

	return c.socketMode.RunContext(ctx)
}

func (c *Connector) setConnected(connected bool) {
	c.mu.Lock()
	c.connected = connected
	c.mu.Unlock()
}

//nolint:gocyclo // socket event dispatch is inherently a large switch
func (c *Connector) handleSocketEvent(ctx context.Context, envelope socketmode.Event) {
	switch envelope.Type {
	case socketmode.EventTypeConnecting:
		c.logger.Info("Connecting to Slack with Socket Mode")
		c.setConnected(false)

	case socketmode.EventTypeConnectionError:
		c.logger.Errorw("Connection failed", "data", fmt.Sprintf("%v", envelope.Data))
		c.setConnected(false)

	case socketmode.EventTypeConnected:
		c.logger.Info("Connected to Slack with Socket Mode")
		c.setConnected(true)

	case socketmode.EventTypeHello:
		// Hello event confirms WebSocket connection

	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := envelope.Data.(slackevents.EventsAPIEvent)
		if !ok {
			c.logger.Warnw("Ignored non-EventsAPI event", "data", fmt.Sprintf("%+v", envelope))
			return
		}
		c.logger.Debugw("Event received", "event_type", eventsAPIEvent.Type)
		c.socketMode.Ack(*envelope.Request)
		// Errors are logged within the individual event handlers with full context.
		_ = c.handleEvent(ctx, eventsAPIEvent)

	case socketmode.EventTypeInteractive:
		c.logger.Debug("Interactive event received")
		c.socketMode.Ack(*envelope.Request)

	case socketmode.EventTypeSlashCommand:
		c.handleSlashCommand(ctx, envelope)

	case socketmode.EventTypeIncomingError:
		c.handleIncomingError(envelope)

	case socketmode.EventTypeErrorWriteFailed:
		c.handleWriteError(envelope)

	case socketmode.EventTypeErrorBadMessage:
		c.handleBadMessage(envelope)

	case socketmode.EventTypeInvalidAuth:
		c.logger.Error("Invalid authentication for Slack Socket Mode")

	case socketmode.EventTypeDisconnect:
		c.logger.Warn("Disconnected from Slack Socket Mode")
		c.setConnected(false)

	default:
		c.logger.Warnw("Unsupported event type received",
			"event_type", string(envelope.Type),
			"data", fmt.Sprintf("%+v", envelope.Data),
		)
	}
}

func (c *Connector) handleIncomingError(envelope socketmode.Event) {
	if err, ok := envelope.Data.(*slack.IncomingEventError); ok {
		c.logger.Warnw("Incoming event error from Slack", "error", err.ErrorObj)
	} else {
		c.logger.Warnw("Incoming event error from Slack", "data", fmt.Sprintf("%+v", envelope.Data))
	}
}

func (c *Connector) handleWriteError(envelope socketmode.Event) {
	if err, ok := envelope.Data.(*socketmode.ErrorWriteFailed); ok {
		c.logger.Errorw("Failed to write to Slack WebSocket", "error", err.Cause)
	} else {
		c.logger.Errorw("Failed to write to Slack WebSocket", "data", fmt.Sprintf("%+v", envelope.Data))
	}
}

func (c *Connector) handleBadMessage(envelope socketmode.Event) {
	if err, ok := envelope.Data.(*socketmode.ErrorBadMessage); ok {
		c.logger.Warnw("Bad message received from Slack", "error", err.Cause, "message", string(err.Message))
	} else {
		c.logger.Warnw("Bad message received from Slack", "data", fmt.Sprintf("%+v", envelope.Data))
	}
}

// handleEvent processes Slack events and routes them.
func (c *Connector) handleEvent(ctx context.Context, event slackevents.EventsAPIEvent) error {
	if event.Type == slackevents.CallbackEvent {
		switch ev := event.InnerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			return c.handleMessageEvent(ctx, ev)
		case *slackevents.AppMentionEvent:
			return c.handleAppMentionEvent(ctx, ev)
		}
	}
	return nil
}

// handleMessageEvent processes direct messages to the bot.
func (c *Connector) handleMessageEvent(ctx context.Context, event *slackevents.MessageEvent) error {
	// Skip bot messages to avoid loops.
	if event.BotID != "" || event.SubType == "bot_message" {
		return nil
	}

	// Skip system/automated message subtypes.
	systemSubtypes := map[string]bool{
		"channel_join": true, "channel_leave": true, "channel_topic": true,
		"channel_purpose": true, "channel_name": true, "channel_archive": true,
		"channel_unarchive": true, "group_join": true, "group_leave": true,
		"group_topic": true, "group_purpose": true, "group_name": true,
		"group_archive": true, "group_unarchive": true,
		"file_share": true, "file_comment": true, "file_mention": true,
		"message_changed": true, "message_deleted": true, "message_replied": true,
		"pinned_item": true, "unpinned_item": true, "reminder_add": true,
		"ekm_access_denied": true, "assistant_app_thread": true,
	}
	if systemSubtypes[event.SubType] {
		return nil
	}

	if event.User == "" {
		return nil
	}

	// Only process direct messages (channel IDs starting with D).
	if !strings.HasPrefix(event.Channel, "D") {
		return nil
	}

	c.logger.Infow("Processing DM",
		"user_id", event.User,
		"channel", event.Channel)

	// DMs use the default agent.
	scopeKey := fmt.Sprintf("slack:dm:%s", event.User)
	agentCfg, cwd := c.acpRouter.Resolve("")

	resp, err := c.acpExecutor.Execute(ctx, acpclient.Request{
		ScopeKey: scopeKey,
		Message:  event.Text,
	}, agentCfg, cwd)
	if err != nil {
		c.logger.Errorw("Failed to execute ACP request for DM",
			"scope", scopeKey, "user_id", event.User, "channel", event.Channel, "error", err)
		_, _, _ = c.client.PostMessage(event.Channel,
			slack.MsgOptionText("Sorry, I encountered an error processing your message.", false))
		return fmt.Errorf("execute DM for user %s: %w", event.User, err)
	}

	if resp.Text != "" {
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText(resp.Text, false))
		if err != nil {
			c.logger.Errorw("Failed to send DM response to Slack",
				"channel", event.Channel, "user_id", event.User, "error", err)
			return fmt.Errorf("send DM response: %w", err)
		}
	}

	return nil
}

// handleAppMentionEvent processes @bot mentions in channels.
func (c *Connector) handleAppMentionEvent(ctx context.Context, event *slackevents.AppMentionEvent) error {
	threadTS := event.ThreadTimeStamp
	if threadTS == "" {
		threadTS = event.TimeStamp
	}

	c.logger.Infow("Processing mention",
		"user_id", event.User,
		"channel", event.Channel,
		"thread_ts", threadTS)

	cleanText := c.removeBotMention(event.Text)

	scopeKey := fmt.Sprintf("slack:%s:%s", event.Channel, threadTS)
	agentCfg, cwd := c.acpRouter.Resolve(event.Channel)

	resp, err := c.acpExecutor.Execute(ctx, acpclient.Request{
		ScopeKey: scopeKey,
		Message:  cleanText,
	}, agentCfg, cwd)
	if err != nil {
		c.logger.Errorw("Failed to execute ACP request for mention",
			"scope", scopeKey, "channel", event.Channel, "thread_ts", threadTS, "error", err)
		_, _, _ = c.client.PostMessage(event.Channel,
			slack.MsgOptionText("Sorry, I encountered an error processing your message.", false),
			slack.MsgOptionTS(threadTS))
		return fmt.Errorf("execute mention in %s (thread %s): %w", event.Channel, threadTS, err)
	}

	if resp.Text != "" {
		_, _, err = c.client.PostMessage(event.Channel,
			slack.MsgOptionText(resp.Text, false),
			slack.MsgOptionTS(threadTS))
		if err != nil {
			c.logger.Errorw("Failed to send mention response to Slack",
				"channel", event.Channel, "thread_ts", threadTS, "error", err)
			return fmt.Errorf("send mention response to %s: %w", event.Channel, err)
		}
	}

	return nil
}

// removeBotMention removes @bot mentions from message text.
func (c *Connector) removeBotMention(text string) string {
	cleaned := text
	if strings.Contains(text, "<@") {
		start := strings.Index(text, "<@")
		end := strings.Index(text[start:], ">")
		if end != -1 {
			cleaned = strings.TrimSpace(text[:start] + text[start+end+1:])
		}
	}
	return cleaned
}

// Ready returns nil if connected, or an error otherwise.
func (c *Connector) Ready() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return fmt.Errorf("slack connector not connected")
	}
	return nil
}
