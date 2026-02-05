package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
	"github.com/lewisedginton/general_purpose_chatbot/internal/session_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// Connector represents the Telegram connector
type Connector struct {
	bot        *bot.Bot
	executor   *executor.Executor
	logger     logger.Logger
	commands   *CommandRegistry
	sessionMgr session_manager.Manager
}

// Config holds configuration for the Telegram connector
type Config struct {
	BotToken string        // Bot token from @BotFather
	Debug    bool          // Enable debug logging
	Logger   logger.Logger // Structured logger instance
}

// NewConnector creates a new Telegram connector with in-process executor
func NewConnector(config Config, exec *executor.Executor, sessionMgr session_manager.Manager) (*Connector, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("bot token is required")
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

	// Create a logger with Telegram-specific context
	telegramLogger := config.Logger.WithFields(logger.StringField("connector", "telegram"))

	// Create the connector instance first
	connector := &Connector{
		executor:   exec,
		logger:     telegramLogger,
		sessionMgr: sessionMgr,
	}

	// Initialize Telegram bot with default handler
	opts := []bot.Option{
		bot.WithDefaultHandler(connector.handleUpdate),
	}

	if config.Debug {
		opts = append(opts, bot.WithDebug())
	}

	b, err := bot.New(config.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	connector.bot = b
	telegramLogger.Info("Telegram bot initialized successfully")

	// Setup command handlers
	connector.setupCommands()

	return connector, nil
}

// Start begins polling for updates
func (c *Connector) Start(ctx context.Context) error {
	c.logger.Info("Starting Telegram bot polling")

	// Start polling - this blocks until context is canceled
	c.bot.Start(ctx)

	return nil
}

// handleUpdate processes all incoming Telegram updates
func (c *Connector) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Only process text messages for now
	if update.Message == nil || update.Message.Text == "" {
		c.logger.Debug("Skipping non-text message or empty update")
		return
	}

	// Skip messages from bots to avoid loops
	if update.Message.From.IsBot {
		c.logger.Debug("Skipping bot message", logger.StringField("username", update.Message.From.Username))
		return
	}

	// Check if this is a command and handle it separately
	if c.commands.IsCommand(update.Message.Text) {
		err := c.handleCommand(ctx, b, update)
		if err != nil {
			c.logger.Error("Error handling command", logger.ErrorField(err))
		}
		return
	}

	c.logger.Info("Processing message",
		logger.Int64Field("user_id", update.Message.From.ID),
		logger.StringField("username", update.Message.From.Username))

	// Create user info function that captures the user ID
	userID := fmt.Sprintf("%d", update.Message.From.ID)
	chatID := fmt.Sprintf("%d", update.Message.Chat.ID)

	// Get or create session for this user
	sessionID, err := c.sessionMgr.GetOrCreateSession(ctx, "telegram", userID, chatID)
	if err != nil {
		c.logger.Error("Error getting session", logger.ErrorField(err))
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Sorry, I encountered an error creating your session.",
		})
		return
	}

	// Send message to agent via executor
	response, err := c.executor.Execute(ctx, executor.MessageRequest{
		UserID:    userID,
		SessionID: sessionID,
		Message:   update.Message.Text,
	}, c, func() string {
		return c.GetUserInfo(ctx, userID)
	})
	if err != nil {
		c.logger.Error("Error from executor", logger.ErrorField(err))
		// Send error message to user
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Sorry, I encountered an error processing your message.",
		})
		if err != nil {
			c.logger.Error("Error sending error message", logger.ErrorField(err))
		}
		return
	}

	// Send response back to Telegram
	if response.Text != "" {
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   response.Text,
		})
		if err != nil {
			c.logger.Error("Error sending message to Telegram", logger.ErrorField(err))
			return
		}
	}
}

// Stop gracefully stops the connector
func (c *Connector) Stop() error {
	c.logger.Info("Stopping Telegram connector")
	// Stopping is handled by context cancellation in Start
	return nil
}

// GetBotInfo returns information about the bot
func (c *Connector) GetBotInfo(ctx context.Context) (*models.User, error) {
	return c.bot.GetMe(ctx)
}

// PlatformName returns the platform name
func (c *Connector) PlatformName() string {
	return "Telegram"
}

// UserInfo returns user context information (legacy method for interface compatibility)
func (c *Connector) UserInfo() string {
	// This method is kept for backward compatibility but should not be used directly
	return ""
}

// GetUserInfo fetches user information from Telegram and returns a formatted string
func (c *Connector) GetUserInfo(ctx context.Context, userID string) string {
	if userID == "" {
		return ""
	}

	// Parse userID to int64
	var id int64
	if _, err := fmt.Sscanf(userID, "%d", &id); err != nil {
		c.logger.Warn("Failed to parse user ID",
			logger.StringField("user_id", userID),
			logger.ErrorField(err))
		return ""
	}

	// Note: Telegram Bot API doesn't have a direct "get user info" method
	// We can only get user info from updates/messages or using getChat for the user
	// For now, we'll use what we can get from getChat
	chat, err := c.bot.GetChat(ctx, &bot.GetChatParams{
		ChatID: id,
	})
	if err != nil {
		c.logger.Warn("Failed to fetch user info",
			logger.StringField("user_id", userID),
			logger.ErrorField(err))
		return ""
	}

	// Format user information
	info := fmt.Sprintf("- User ID: %d\n", chat.ID)

	if chat.FirstName != "" {
		info += fmt.Sprintf("- First Name: %s\n", chat.FirstName)
	}

	if chat.LastName != "" {
		info += fmt.Sprintf("- Last Name: %s\n", chat.LastName)
	}

	if chat.Username != "" {
		info += fmt.Sprintf("- Username: @%s\n", chat.Username)
	}

	if chat.Bio != "" {
		info += fmt.Sprintf("- Bio: %s\n", chat.Bio)
	}

	return info
}

// FormattingGuide returns Telegram-specific formatting instructions
func (c *Connector) FormattingGuide() string {
	return `# Telegram Formatting Guide

## Text Formatting (MarkdownV2)
- *Bold text*: Wrap text in asterisks (e.g., *bold*)
- _Italic text_: Wrap text in underscores (e.g., _italic_)
- __Underline__: Wrap text in double underscores (e.g., __underline__)
- ~Strikethrough~: Wrap text in tildes (e.g., ~strikethrough~)
- ||Spoiler||: Wrap text in double pipes (e.g., ||spoiler||)
- Inline code: Wrap text in backticks (e.g., ` + "`code`" + `)

## Code Blocks
Use triple backticks with optional language for syntax highlighting:
` + "```python" + `
def hello():
    print("Hello, World!")
` + "```" + `

## Links
- Inline links: [Link Text](https://example.com)
- User mentions: [User Name](tg://user?id=USER_ID)

## Special Characters
In MarkdownV2, these characters must be escaped with backslash: _ * [ ] ( ) ~ ` + "`" + ` > # + - = | { } . !

## HTML Formatting (Alternative)
Telegram also supports HTML formatting:
- Bold: <b>text</b>
- Italic: <i>text</i>
- Underline: <u>text</u>
- Strikethrough: <s>text</s>
- Code: <code>text</code>
- Pre-formatted: <pre>code block</pre>
- Links: <a href="URL">text</a>

## Important Notes
- By default, messages are sent as plain text
- For formatted text, the parse_mode must be set to "MarkdownV2" or "HTML"
- Emoji are supported natively using Unicode characters
- Maximum message length is 4096 characters`
}

// Ready returns nil if the Telegram connector is initialized and ready to receive requests,
// or an error if it's not ready.
func (c *Connector) Ready() error {
	if c.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}
	return nil
}
