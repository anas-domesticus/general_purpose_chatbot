package telegram

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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
func handleNewCommand(ctx context.Context, b *bot.Bot, update *models.Update) (string, error) {
	// TODO: Implement /new command logic
	return "The /new command is not yet implemented.", nil
}

// handleHelpCommand handles the /help command
func handleHelpCommand(ctx context.Context, b *bot.Bot, update *models.Update) (string, error) {
	// TODO: Implement /help command logic
	return "The /help command is not yet implemented.", nil
}

// setupCommands initializes the command registry with all available commands
func (c *Connector) setupCommands() {
	c.commands = NewCommandRegistry()
	c.commands.Register("/new", handleNewCommand)
	c.commands.Register("/help", handleHelpCommand)
}

// handleCommand processes a command update
func (c *Connector) handleCommand(ctx context.Context, b *bot.Bot, update *models.Update) error {
	c.logger.Printf("Processing command from user %d (%s): %s",
		update.Message.From.ID,
		update.Message.From.Username,
		update.Message.Text)

	// Handle the command via registry
	response, err := c.commands.Handle(ctx, b, update)
	if err != nil {
		c.logger.Printf("Error handling command %s: %v", update.Message.Text, err)
		response = "An error occurred while processing your command."
	}

	// Send response if we have one
	if response != "" {
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   response,
		})
		if err != nil {
			c.logger.Printf("Error sending command response: %v", err)
			return err
		}
	}

	return nil
}
