// Package main is the entry point for the chatbot application.
package main

import (
	"context"
	"fmt"
	"os"

	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/server"
	pkgconfig "github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

func main() {
	// Load configuration
	cfg := &appconfig.AppConfig{}
	if err := pkgconfig.GetConfigFromEnvVars(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialise structured logger
	log := logger.NewLogger(logger.Config{
		Level:   cfg.GetLogLevel(),
		Format:  cfg.Logging.Format,
		Service: cfg.ServiceName,
	})

	cfg.LogConfig(log)

	log.Info("Starting Multi-Platform Chatbot",
		logger.StringField("version", cfg.Version),
		logger.StringField("llm_provider", cfg.LLM.Provider),
		logger.StringField("llm_model", cfg.GetLLMModel()))

	// Create server with all components
	srv, err := server.New(context.Background(), cfg, log)
	if err != nil {
		log.Error("Failed to create server", logger.ErrorField(err))
		os.Exit(1)
	}

	// Run the server (blocks until shutdown)
	if err := srv.Run(); err != nil {
		log.Error("Server error", logger.ErrorField(err))
		os.Exit(1)
	}
}
