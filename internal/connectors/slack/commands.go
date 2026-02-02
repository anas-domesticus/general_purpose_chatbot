package slack

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

// CommandHandler handles a specific slash command
type CommandHandler func(ctx context.Context, cmd slack.SlashCommand) (interface{}, error)

// CommandRegistry manages slash command handlers
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

// Handle processes a slash command event
func (r *CommandRegistry) Handle(ctx context.Context, cmd slack.SlashCommand) (interface{}, error) {
	handler, exists := r.handlers[cmd.Command]
	if !exists {
		return map[string]interface{}{
			"text": fmt.Sprintf("Unknown command: %s", cmd.Command),
		}, nil
	}

	return handler(ctx, cmd)
}

// handleNewCommand handles the /new command
func (c *Connector) handleNewCommand(ctx context.Context, cmd slack.SlashCommand) (interface{}, error) {
	sessionID, err := c.sessionMgr.CreateNewSession(ctx, "slack", cmd.UserID, cmd.ChannelID)
	if err != nil {
		return map[string]interface{}{
			"text": "Failed to create new session.",
		}, err
	}
	return map[string]interface{}{
		"text": fmt.Sprintf("Started new conversation! (Session: %s)", sessionID),
	}, nil
}

// handleHelpCommand handles the /help command
func (c *Connector) handleHelpCommand(ctx context.Context, cmd slack.SlashCommand) (interface{}, error) {
	helpText := `*Available Commands:*

• */new* - Start a new conversation
• */help* - Show this help message`

	return map[string]interface{}{
		"text": helpText,
	}, nil
}

// setupCommands initializes the command registry with all available commands
func (c *Connector) setupCommands() {
	c.commands = NewCommandRegistry()
	c.commands.Register("/new", func(ctx context.Context, cmd slack.SlashCommand) (interface{}, error) {
		return c.handleNewCommand(ctx, cmd)
	})
	c.commands.Register("/help", func(ctx context.Context, cmd slack.SlashCommand) (interface{}, error) {
		return c.handleHelpCommand(ctx, cmd)
	})
}

// handleSlashCommand processes incoming slash command events
func (c *Connector) handleSlashCommand(ctx context.Context, envelope socketmode.Event) {
	cmd, ok := envelope.Data.(slack.SlashCommand)
	if !ok {
		c.logger.Printf("Failed to parse slash command data: %+v", envelope.Data)
		c.socketMode.Ack(*envelope.Request)
		return
	}

	c.logger.Printf("Received slash command: %s from user %s in channel %s", cmd.Command, cmd.UserID, cmd.ChannelID)

	// Handle the command via registry
	response, err := c.commands.Handle(ctx, cmd)
	if err != nil {
		c.logger.Printf("Error handling command %s: %v", cmd.Command, err)
		response = map[string]interface{}{
			"text": "An error occurred while processing your command.",
		}
	}

	// Acknowledge the command with the response
	c.socketMode.Ack(*envelope.Request, response)
}
