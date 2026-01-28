package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// ConfigCommand returns a command for configuration operations
func ConfigCommand() *cli.Command {
	return &cli.Command{
		Name:    "config",
		Aliases: []string{"c"},
		Usage:   "Configuration operations",
		Subcommands: []*cli.Command{
			{
				Name:  "validate",
				Usage: "Validate configuration",
				Action: configValidateAction,
			},
		},
	}
}

func configValidateAction(ctx *cli.Context) error {
	log := getLogger(ctx)

	log.Info("Validating configuration")

	// Load and validate configuration
	cfg := &Config{}
	if err := config.GetConfigFromEnvVars(cfg); err != nil {
		log.Error("Configuration validation failed", logger.ErrorField(err))
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	log.Info("Configuration validation passed")
	fmt.Println("✅ Configuration is valid")
	return nil
}