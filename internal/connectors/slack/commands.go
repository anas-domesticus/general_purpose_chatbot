// Package slack implements the Slack Socket Mode connector for the ACP chatbot.
package slack

import (
	"context"
	"fmt"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
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
		c.logger.Warn("Failed to parse slash command data", logger.StringField("data", fmt.Sprintf("%+v", envelope.Data)))
		c.socketMode.Ack(*envelope.Request)
		return
	}

	c.logger.Info("Received slash command",
		logger.StringField("command", cmd.Command),
		logger.StringField("user_id", cmd.UserID),
		logger.StringField("channel_id", cmd.ChannelID))

	response, err := c.commands.Handle(ctx, cmd)
	if err != nil {
		c.logger.Error("Error handling command",
			logger.StringField("command", cmd.Command),
			logger.ErrorField(err))
		response = map[string]interface{}{
			"text": "An error occurred while processing your command.",
		}
	}

	c.socketMode.Ack(*envelope.Request, response)
}
