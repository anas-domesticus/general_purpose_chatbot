package cli

import (
	"github.com/urfave/cli/v2"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// getLogger retrieves the logger from the CLI context metadata
func getLogger(ctx *cli.Context) logger.Logger {
	if ctx.App.Metadata != nil {
		if log, ok := ctx.App.Metadata["logger"].(logger.Logger); ok {
			return log
		}
	}
	
	// Fallback to default logger if not found
	return logger.NewLogger(logger.Config{
		Level:   logger.InfoLevel,
		Format:  "json",
		Service: "boilerplate-cli",
	})
}