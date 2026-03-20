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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	configPath := flag.String("config", "", "Path to YAML configuration file (optional, env vars override file values)")
	flag.Parse()

	cfg := &appconfig.AppConfig{}
	if err := pkgconfig.GetConfig(cfg, *configPath, true); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Parse the configured log level. Supported values: debug, info, warn, error.
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level %q: %v\n", cfg.Logging.Level, err)
		os.Exit(1)
	}

	// Use development config for debug (human-readable), production for everything else.
	var zapCfg zap.Config
	if level <= zapcore.DebugLevel {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)

	zapLogger, err := zapCfg.Build()
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
