package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/slack"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/telegram"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
	intsession "github.com/lewisedginton/general_purpose_chatbot/internal/session"
	"github.com/lewisedginton/general_purpose_chatbot/internal/session_manager"
	pkgconfig "github.com/lewisedginton/general_purpose_chatbot/pkg/config"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func main() {
	// Load configuration
	cfg := &appconfig.AppConfig{}
	if err := pkgconfig.GetConfigFromEnvVars(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
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

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupGracefulShutdown(cancel, log)

	// Start pprof server for profiling
	go func() {
		log.Info("Starting pprof server on :6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Error("pprof server failed", logger.ErrorField(err))
		}
	}()

	// Create LLM model instance based on configured provider
	llmModel, err := createLLMModel(ctx, cfg, log)
	if err != nil {
		log.Error("Failed to create LLM model", logger.ErrorField(err))
		os.Exit(1)
	}

	// Create generic chat agent factory (shared across all platforms)
	// Note: nil formatting provider for now - will be platform-specific in the future
	chatAgentFactory, err := agents.NewChatAgent(llmModel, cfg.MCP, agents.AgentConfig{
		Name:        "chat_assistant",
		Platform:    "Multi-Platform",
		Description: "Claude-powered assistant with MCP capabilities",
	})
	if err != nil {
		log.Error("Failed to create chat agent factory", logger.ErrorField(err))
		os.Exit(1)
	}

	// Create session service based on configuration
	sessionService, err := createSessionService(ctx, &cfg.Session, log)
	if err != nil {
		log.Error("Failed to create session service", logger.ErrorField(err))
		os.Exit(1)
	}

	// Create session manager for tracking live sessions
	sessionMgr, err := createSessionManager(ctx, &cfg.Session, log)
	if err != nil {
		log.Error("Failed to create session manager", logger.ErrorField(err))
		os.Exit(1)
	}

	// Create executor with agent factory (shared across all platforms)
	// Note: nil formatting provider for now - will be platform-specific in the future
	exec, err := executor.NewExecutor(chatAgentFactory, "chatbot", sessionService)
	if err != nil {
		log.Error("Failed to create executor", logger.ErrorField(err))
		os.Exit(1)
	}

	// Detect and start enabled connectors
	var wg sync.WaitGroup
	enabledCount := 0

	// Start Slack connector if configured
	if cfg.Slack.Enabled() {
		enabledCount++
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := startSlackConnector(ctx, cfg, exec, sessionMgr, log); err != nil {
				log.Error("Slack connector failed", logger.ErrorField(err))
				cancel() // Trigger shutdown on error
			}
		}()
	} else {
		log.Info("Slack connector disabled (missing SLACK_BOT_TOKEN or SLACK_APP_TOKEN)")
	}

	// Start Telegram connector if configured
	if cfg.Telegram.Enabled() {
		enabledCount++
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := startTelegramConnector(ctx, cfg, exec, sessionMgr, log); err != nil {
				log.Error("Telegram connector failed", logger.ErrorField(err))
				cancel() // Trigger shutdown on error
			}
		}()
	} else {
		log.Info("Telegram connector disabled (missing TELEGRAM_BOT_TOKEN)")
	}

	// Verify at least one connector is enabled
	if enabledCount == 0 {
		log.Error("No connectors configured. Please set environment variables for at least one platform (Slack or Telegram)")
		os.Exit(1)
	}

	log.Info("All enabled connectors started", logger.IntField("count", enabledCount))

	// Wait for all connectors to finish
	wg.Wait()
	log.Info("All connectors stopped")
}

// startSlackConnector initializes and starts the Slack connector
func startSlackConnector(ctx context.Context, cfg *appconfig.AppConfig, exec *executor.Executor, sessionMgr session_manager.Manager, log logger.Logger) error {
	log.Info("Starting Slack connector")

	// Create Slack connector
	slackConnector, err := slack.NewConnector(slack.Config{
		BotToken: cfg.Slack.BotToken,
		AppToken: cfg.Slack.AppToken,
		Debug:    cfg.Slack.Debug,
	}, exec, sessionMgr)
	if err != nil {
		return fmt.Errorf("failed to create Slack connector: %w", err)
	}

	// Start connector (blocks until context is cancelled)
	if err := slackConnector.Start(ctx); err != nil {
		return fmt.Errorf("Slack connector error: %w", err)
	}

	return nil
}

// startTelegramConnector initializes and starts the Telegram connector
func startTelegramConnector(ctx context.Context, cfg *appconfig.AppConfig, exec *executor.Executor, sessionMgr session_manager.Manager, log logger.Logger) error {
	log.Info("Starting Telegram connector")

	// Create Telegram connector
	telegramConnector, err := telegram.NewConnector(telegram.Config{
		BotToken: cfg.Telegram.BotToken,
		Debug:    cfg.Telegram.Debug,
	}, exec, sessionMgr)
	if err != nil {
		return fmt.Errorf("failed to create Telegram connector: %w", err)
	}

	// Get and log bot info
	botInfo, err := telegramConnector.GetBotInfo(ctx)
	if err != nil {
		log.Warn("Failed to get Telegram bot info", logger.ErrorField(err))
	} else {
		log.Info("Telegram bot connected",
			logger.StringField("bot_username", botInfo.Username),
			logger.StringField("bot_first_name", botInfo.FirstName))
	}

	// Start connector (blocks until context is cancelled)
	if err := telegramConnector.Start(ctx); err != nil {
		return fmt.Errorf("Telegram connector error: %w", err)
	}

	return nil
}

// createSessionService creates a session service based on configuration
func createSessionService(ctx context.Context, cfg *appconfig.SessionConfig, log logger.Logger) (session.Service, error) {
	switch cfg.Backend {
	case "local":
		log.Info("Using local file-based session storage", logger.StringField("directory", cfg.LocalDir))

		// Ensure directory exists
		if err := os.MkdirAll(cfg.LocalDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create session directory: %w", err)
		}

		return intsession.NewSessionService(intsession.StorageConfig{
			Backend: "local",
			Local:   intsession.LocalConfig{BaseDir: cfg.LocalDir},
			Logger:  log,
		})

	case "s3":
		log.Info("Using S3-based session storage",
			logger.StringField("bucket", cfg.S3Bucket),
			logger.StringField("prefix", cfg.S3Prefix),
			logger.StringField("region", cfg.S3Region))

		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3 bucket is required when using S3 session storage")
		}

		// Load AWS configuration
		var awsCfg aws.Config
		var err error

		configOptions := []func(*awsconfig.LoadOptions) error{}

		if cfg.S3Profile != "" {
			configOptions = append(configOptions, awsconfig.WithSharedConfigProfile(cfg.S3Profile))
		}

		if cfg.S3Region != "" {
			configOptions = append(configOptions, awsconfig.WithRegion(cfg.S3Region))
		}

		awsCfg, err = awsconfig.LoadDefaultConfig(ctx, configOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		// Create S3 client
		s3Client := s3.NewFromConfig(awsCfg)
		awsS3Client := intsession.NewAWSS3Client(s3Client)

		return intsession.NewSessionService(intsession.StorageConfig{
			Backend: "s3",
			S3: intsession.S3Config{
				Bucket:   cfg.S3Bucket,
				Prefix:   cfg.S3Prefix,
				S3Client: awsS3Client,
			},
			Logger: log,
		})

	case "memory":
		log.Info("Using in-memory session storage")
		return session.InMemoryService(), nil

	default:
		return nil, fmt.Errorf("unsupported session backend: %s (must be 'local', 's3', or 'memory')", cfg.Backend)
	}
}

// createSessionManager creates a session manager based on configuration
func createSessionManager(ctx context.Context, cfg *appconfig.SessionConfig, log logger.Logger) (session_manager.Manager, error) {
	var fileProvider intsession.FileProvider
	var metadataFile string

	switch cfg.Backend {
	case "local":
		log.Info("Using local file-based session manager", logger.StringField("directory", cfg.LocalDir))

		// Ensure directory exists
		if err := os.MkdirAll(cfg.LocalDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create session directory: %w", err)
		}

		fileProvider = intsession.NewLocalFileProvider(cfg.LocalDir)
		metadataFile = "sessions_metadata.json"

	case "s3":
		log.Info("Using S3-based session manager",
			logger.StringField("bucket", cfg.S3Bucket),
			logger.StringField("prefix", cfg.S3Prefix))

		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3 bucket is required when using S3 session storage")
		}

		// Load AWS configuration
		var awsCfg aws.Config
		var err error

		configOptions := []func(*awsconfig.LoadOptions) error{}

		if cfg.S3Profile != "" {
			configOptions = append(configOptions, awsconfig.WithSharedConfigProfile(cfg.S3Profile))
		}

		if cfg.S3Region != "" {
			configOptions = append(configOptions, awsconfig.WithRegion(cfg.S3Region))
		}

		awsCfg, err = awsconfig.LoadDefaultConfig(ctx, configOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		// Create S3 client
		s3Client := s3.NewFromConfig(awsCfg)
		awsS3Client := intsession.NewAWSS3Client(s3Client)

		fileProvider = intsession.NewS3FileProvider(cfg.S3Bucket, cfg.S3Prefix, awsS3Client)
		metadataFile = "sessions_metadata.json"

	case "memory":
		log.Info("Using in-memory session manager (not persisted)")
		// For memory backend, use a temporary directory for metadata
		tmpDir := os.TempDir()
		fileProvider = intsession.NewLocalFileProvider(tmpDir)
		metadataFile = "sessions_metadata_memory.json"

	default:
		return nil, fmt.Errorf("unsupported session backend: %s (must be 'local', 's3', or 'memory')", cfg.Backend)
	}

	return session_manager.New(session_manager.Config{
		MetadataFile: metadataFile,
		FileProvider: fileProvider,
		Logger:       log,
	})
}

// setupGracefulShutdown sets up signal handling for graceful shutdown
func setupGracefulShutdown(cancel context.CancelFunc, log logger.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info("Received shutdown signal", logger.StringField("signal", sig.String()))

		// Start graceful shutdown
		cancel()

		// Give processes time to shutdown gracefully, then force exit
		time.AfterFunc(30*time.Second, func() {
			log.Warn("Force exiting due to timeout")
			os.Exit(1)
		})
	}()
}

// createLLMModel creates an LLM model instance based on the configured provider
func createLLMModel(ctx context.Context, cfg *appconfig.AppConfig, log logger.Logger) (model.LLM, error) {
	provider := strings.ToLower(cfg.LLM.Provider)

	switch provider {
	case "claude":
		log.Info("Initializing Claude model",
			logger.StringField("model", cfg.Anthropic.Model))
		return anthropic.NewClaudeModel(cfg.Anthropic.APIKey, cfg.Anthropic.Model)

	case "gemini":
		log.Info("Initializing Gemini model",
			logger.StringField("model", cfg.Gemini.Model))

		// Configure the Gemini client
		clientConfig := &genai.ClientConfig{
			APIKey: cfg.Gemini.APIKey,
		}

		// If Vertex AI credentials are provided, use Vertex AI backend
		if cfg.Gemini.Project != "" && cfg.Gemini.Region != "" {
			clientConfig.Backend = genai.BackendVertexAI
			clientConfig.Project = cfg.Gemini.Project
			clientConfig.Location = cfg.Gemini.Region
			log.Info("Using Vertex AI backend",
				logger.StringField("project", cfg.Gemini.Project),
				logger.StringField("region", cfg.Gemini.Region))
		}

		return gemini.NewModel(ctx, cfg.Gemini.Model, clientConfig)

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}
