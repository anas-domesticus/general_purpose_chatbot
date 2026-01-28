package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	commands "github.com/lewisedginton/general_purpose_chatbot/internal/cli"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

func main() {
	app := &cli.App{
		Name:    "general-purpose-chatbot",
		Usage:   "A general purpose chatbot CLI application",
		Version: "1.0.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "info",
				Usage:   "Log level (debug, info, warn, error)",
				EnvVars: []string{"LOG_LEVEL"},
			},
			&cli.StringFlag{
				Name:    "config-file",
				Value:   "",
				Usage:   "Path to configuration file",
				EnvVars: []string{"CONFIG_FILE"},
			},
		},
		Before: func(ctx *cli.Context) error {
			// Initialize global logger from flags
			logLevel := logger.ParseLevel(ctx.String("log-level"))
			log := logger.NewLogger(logger.Config{
				Level:   logLevel,
				Format:  "json",
				Service: "general-purpose-chatbot",
			})
			
			// Store logger in context for commands to use
			ctx.App.Metadata = map[string]interface{}{
				"logger": log,
			}
			
			return nil
		},
		Commands: []*cli.Command{
			commands.ConfigCommand(),
			commands.ServerCommand(),
			commands.ChatbotCommand(),
			commands.SlackCommand(),
		},
	}

	if err := app.RunContext(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}