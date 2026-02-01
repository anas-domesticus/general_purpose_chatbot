package telegram

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
)

// Connector represents the Telegram connector
type Connector struct {
	bot      *bot.Bot
	executor *executor.Executor
	logger   *log.Logger
}

// Config holds configuration for the Telegram connector
type Config struct {
	BotToken string // Bot token from @BotFather
	Debug    bool   // Enable debug logging
}

// NewConnector creates a new Telegram connector with in-process executor
func NewConnector(config Config, exec *executor.Executor) (*Connector, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}
	if exec == nil {
		return nil, fmt.Errorf("executor is required")
	}

	logger := log.New(os.Stdout, "[TELEGRAM-CONNECTOR] ", log.LstdFlags|log.Lshortfile)

	// Create the connector instance first
	connector := &Connector{
		executor: exec,
		logger:   logger,
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
	logger.Println("Telegram bot initialized successfully")

	return connector, nil
}

// Start begins polling for updates
func (c *Connector) Start(ctx context.Context) error {
	c.logger.Println("Starting Telegram bot polling...")

	// Start polling - this blocks until context is cancelled
	c.bot.Start(ctx)

	return nil
}

// handleUpdate processes all incoming Telegram updates
func (c *Connector) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Only process text messages for now
	if update.Message == nil || update.Message.Text == "" {
		c.logger.Printf("Skipping non-text message or empty update")
		return
	}

	// Skip messages from bots to avoid loops
	if update.Message.From.IsBot {
		c.logger.Printf("Skipping bot message from %s", update.Message.From.Username)
		return
	}

	c.logger.Printf("Processing message from user %d (%s): %s",
		update.Message.From.ID,
		update.Message.From.Username,
		update.Message.Text)

	// Create session ID from chat and user
	sessionID := fmt.Sprintf("telegram_%d_%d",
		update.Message.From.ID,
		update.Message.Chat.ID)

	// Send message to agent via executor
	response, err := c.executor.Execute(ctx, executor.MessageRequest{
		UserID:    fmt.Sprintf("%d", update.Message.From.ID),
		SessionID: sessionID,
		Message:   update.Message.Text,
	})

	if err != nil {
		c.logger.Printf("Error from executor: %v", err)
		// Send error message to user
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Sorry, I encountered an error processing your message.",
		})
		if err != nil {
			c.logger.Printf("Error sending error message: %v", err)
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
			c.logger.Printf("Error sending message to Telegram: %v", err)
			return
		}
	}
}

// Stop gracefully stops the connector
func (c *Connector) Stop() error {
	c.logger.Println("Stopping Telegram connector...")
	// Stopping is handled by context cancellation in Start
	return nil
}

// GetBotInfo returns information about the bot
func (c *Connector) GetBotInfo(ctx context.Context) (*models.User, error) {
	return c.bot.GetMe(ctx)
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
