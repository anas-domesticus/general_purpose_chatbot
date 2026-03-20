// Package slack implements the Slack Socket Mode connector for the ACP chatbot.
package slack

import (
	"context"
	"fmt"

	goslack "github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

// CommandHandler handles a specific slash command.
type CommandHandler func(ctx context.Context, cmd goslack.SlashCommand) (interface{}, error)

// CommandRegistry manages slash command handlers.
type CommandRegistry struct {
	handlers map[string]CommandHandler
}

// NewCommandRegistry creates a new command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		handlers: make(map[string]CommandHandler),
	}
}

// Register adds a command handler to the registry.
func (r *CommandRegistry) Register(command string, handler CommandHandler) {
	r.handlers[command] = handler
}

// Handle processes a slash command event.
func (r *CommandRegistry) Handle(ctx context.Context, cmd goslack.SlashCommand) (interface{}, error) {
	handler, exists := r.handlers[cmd.Command]
	if !exists {
		return map[string]interface{}{
			"text": fmt.Sprintf("Unknown command: %s", cmd.Command),
		}, nil
	}
	return handler(ctx, cmd)
}

// handleHelpCommand handles the /help command.
func (c *Connector) handleHelpCommand(_ context.Context, _ goslack.SlashCommand) (interface{}, error) {
	helpText := `*Available Commands:*

• */help* - Show this help message

Send me a DM or @mention me in a channel to chat!`

	return map[string]interface{}{
		"text": helpText,
	}, nil
}

// setupCommands initializes the command registry.
func (c *Connector) setupCommands() {
	c.commands = NewCommandRegistry()
	c.commands.Register("/help", func(ctx context.Context, cmd goslack.SlashCommand) (interface{}, error) {
		return c.handleHelpCommand(ctx, cmd)
	})
}

// handleSlashCommand processes incoming slash command events.
func (c *Connector) handleSlashCommand(ctx context.Context, envelope socketmode.Event) {
	cmd, ok := envelope.Data.(goslack.SlashCommand)
	if !ok {
		c.logger.Warnw("Failed to parse slash command data", "data", fmt.Sprintf("%+v", envelope.Data))
		c.socketMode.Ack(*envelope.Request)
		return
	}

	c.logger.Infow("Received slash command",
		"command", cmd.Command,
		"user_id", cmd.UserID,
		"channel_id", cmd.ChannelID)

	response, err := c.commands.Handle(ctx, cmd)
	if err != nil {
		c.logger.Errorw("Error handling command",
			"command", cmd.Command,
			"error", err)
		response = map[string]interface{}{
			"text": "An error occurred while processing your command.",
		}
	}

	c.socketMode.Ack(*envelope.Request, response)
}
