// Package telegram provides the Telegram bot connector for the chatbot.
package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// CommandHandler handles a specific Telegram bot command
type CommandHandler func(ctx context.Context, b *bot.Bot, update *models.Update) (string, error)

// CommandRegistry manages bot command handlers
type CommandRegistry struct {
	handlers map[string]CommandHandler
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		handlers: make(map[string]CommandHandler),
	}
}

// Register adds a command handler to the registry
func (r *CommandRegistry) Register(command string, handler CommandHandler) {
	r.handlers[command] = handler
}

// Handle processes a command from an update
func (r *CommandRegistry) Handle(ctx context.Context, b *bot.Bot, update *models.Update) (string, error) {
	if update.Message == nil || update.Message.Text == "" {
		return "", nil
	}

	// Parse command and arguments from message text
	text := update.Message.Text
	if !strings.HasPrefix(text, "/") {
		return "", nil
	}

	// Split command from arguments
	parts := strings.SplitN(text, " ", 2)
	command := parts[0]

	// Look up handler
	handler, exists := r.handlers[command]
	if !exists {
		return "Unknown command: " + command, nil
	}

	return handler(ctx, b, update)
}

// IsCommand checks if a message is a command
func (r *CommandRegistry) IsCommand(text string) bool {
	return strings.HasPrefix(text, "/")
}

// handleNewCommand handles the /new command
func (c *Connector) handleNewCommand(ctx context.Context, _ *bot.Bot, update *models.Update) (string, error) {
	userID := fmt.Sprintf("%d", update.Message.From.ID)
	chatID := fmt.Sprintf("%d", update.Message.Chat.ID)

	sessionID, err := c.sessionMgr.CreateNewSession(ctx, "telegram", userID, chatID)
	if err != nil {
		return "Failed to create new session.", err
	}
	return fmt.Sprintf("Started new conversation! (Session: %s)", sessionID), nil
}

// handleHelpCommand handles the /help command
func (c *Connector) handleHelpCommand(ctx context.Context, b *bot.Bot, update *models.Update) (string, error) {
	helpText := `Available Commands:

/new - Start a new conversation
/help - Show this help message`

	return helpText, nil
}

// setupCommands initialises the command registry with all available commands
func (c *Connector) setupCommands() {
	c.commands = NewCommandRegistry()
	c.commands.Register("/new", func(ctx context.Context, b *bot.Bot, update *models.Update) (string, error) {
		return c.handleNewCommand(ctx, b, update)
	})
	c.commands.Register("/help", func(ctx context.Context, b *bot.Bot, update *models.Update) (string, error) {
		return c.handleHelpCommand(ctx, b, update)
	})
}

// handleCommand processes a command update
func (c *Connector) handleCommand(ctx context.Context, b *bot.Bot, update *models.Update) error {
	c.logger.Info("Processing command",
		logger.Int64Field("user_id", update.Message.From.ID),
		logger.StringField("username", update.Message.From.Username),
		logger.StringField("command", update.Message.Text))

	// Handle the command via registry
	response, err := c.commands.Handle(ctx, b, update)
	if err != nil {
		c.logger.Error("Error handling command",
			logger.StringField("command", update.Message.Text),
			logger.ErrorField(err))
		response = "An error occurred while processing your command."
	}

	// Send response if we have one
	if response != "" {
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   response,
		})
		if err != nil {
			c.logger.Error("Error sending command response", logger.ErrorField(err))
			return err
		}
	}

	return nil
}
