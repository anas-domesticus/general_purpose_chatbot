// Package main is the entry point for the chatbot application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/server"
	pkgconfig "github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "", "Path to YAML configuration file (optional, env vars override file values)")
	flag.Parse()

	cfg := &appconfig.AppConfig{}
	if err := pkgconfig.GetConfig(cfg, *configPath, true); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Create zap logger based on config.
	var zapLogger *zap.Logger
	var err error
	if strings.EqualFold(cfg.Logging.Level, "debug") {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = zapLogger.Sync() }()

	log := zapLogger.Sugar().With("service", cfg.ServiceName)

	cfg.LogConfig(log)

	log.Infow("Starting ACP Chatbot", "version", cfg.Version)

	srv, err := server.New(context.Background(), cfg, log)
	if err != nil {
		log.Fatalw("Failed to create server", "error", err)
	}

	if err := srv.Run(); err != nil {
		log.Fatalw("Server error", "error", err)
	}
}
