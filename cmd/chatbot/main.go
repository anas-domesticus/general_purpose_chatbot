// Package main is the entry point for the chatbot application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/server"
	pkgconfig "github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

func main() {
	configPath := flag.String("config", "", "Path to YAML configuration file (optional, env vars override file values)")
	flag.Parse()

	cfg := &appconfig.AppConfig{}
	if err := pkgconfig.GetConfig(cfg, *configPath, true); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	log := logger.NewLogger(logger.Config{
		Level:   cfg.GetLogLevel(),
		Format:  cfg.Logging.Format,
		Service: cfg.ServiceName,
	})

	cfg.LogConfig(log)

	log.Info("Starting ACP Chatbot",
		logger.StringField("version", cfg.Version))

	srv, err := server.New(context.Background(), cfg, log)
	if err != nil {
		log.Error("Failed to create server", logger.ErrorField(err))
		os.Exit(1)
	}

	if err := srv.Run(); err != nil {
		log.Error("Server error", logger.ErrorField(err))
		os.Exit(1)
	}
}
