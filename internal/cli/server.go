package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/utils"
)

// ServerCommand returns a command for server operations
func ServerCommand() *cli.Command {
	return &cli.Command{
		Name:    "server",
		Aliases: []string{"s"},
		Usage:   "Server operations",
		Subcommands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start the API server",
				Action: serverStartAction,
			},
		},
	}
}

func serverStartAction(ctx *cli.Context) error {
	log := getLogger(ctx)

	// Load configuration from environment variables
	cfg := &Config{}
	if err := config.GetConfigFromEnvVars(cfg); err != nil {
		log.Error("Failed to load config", logger.ErrorField(err))
		return fmt.Errorf("failed to load config: %w", err)
	}

	log.Info("Configuration loaded successfully")

	// Create server
	s, err := NewSimpleServer(log, cfg)
	if err != nil {
		log.Error("Failed to create server", logger.ErrorField(err))
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server
	errChan, closer, gracefulCloser, err := s.Listen()
	if err != nil {
		log.Error("Failed to start server", logger.ErrorField(err))
		return fmt.Errorf("failed to start server: %w", err)
	}

	log.Info("HTTP service started successfully")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Merge error channels
	mergedErrChan := utils.MergeErrorChans(errChan)

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Info("Received shutdown signal", logger.StringField("signal", sig.String()))
		gracefulCloser()
		log.Info("Server exited gracefully")
	case err := <-mergedErrChan:
		if err != nil {
			log.Error("Fatal server error occurred", logger.ErrorField(err))
			closer()
			return fmt.Errorf("server error: %w", err)
		} else {
			log.Info("Server exited normally")
		}
	}

	return nil
}