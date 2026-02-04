// Package server provides the main server implementation for the chatbot.
package server

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
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/openai"
	"github.com/lewisedginton/general_purpose_chatbot/internal/monitoring"
	intsession "github.com/lewisedginton/general_purpose_chatbot/internal/session"
	"github.com/lewisedginton/general_purpose_chatbot/internal/session_manager"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Connector defines the interface for platform connectors
type Connector interface {
	Start(ctx context.Context) error
}

// Server encapsulates all the chatbot server components and lifecycle management
type Server struct {
	cfg               *appconfig.AppConfig
	log               logger.Logger
	executor          *executor.Executor
	slackConnector    *slack.Connector
	telegramConnector *telegram.Connector
	sessionService    session.Service
	sessionManager    session_manager.Manager
	cancel            context.CancelFunc
}

// New creates a new Server instance with all components initialised
func New(ctx context.Context, cfg *appconfig.AppConfig, log logger.Logger) (*Server, error) {
	s := &Server{
		cfg: cfg,
		log: log,
	}

	// Create LLM model instance based on configured provider
	llmModel, err := s.createLLMModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM model: %w", err)
	}

	// Create generic chat agent factory (shared across all platforms)
	chatAgentFactory, err := agents.NewChatAgent(llmModel, cfg.MCP, agents.AgentConfig{
		Name:        "chat_assistant",
		Platform:    "Multi-Platform",
		Description: "Claude-powered assistant with MCP capabilities",
		Logger:      log,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat agent factory: %w", err)
	}

	// Create session service based on configuration
	s.sessionService, err = s.createSessionService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session service: %w", err)
	}

	// Create session manager for tracking live sessions
	s.sessionManager, err = s.createSessionManager(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	// Create executor with agent factory (shared across all platforms)
	s.executor, err = executor.NewExecutor(chatAgentFactory, "chatbot", s.sessionService)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	// Create connectors (but don't start yet)
	if cfg.Slack.Enabled() {
		s.slackConnector, err = slack.NewConnector(slack.Config{
			BotToken: cfg.Slack.BotToken,
			AppToken: cfg.Slack.AppToken,
			Debug:    cfg.Slack.Debug,
			Logger:   log,
		}, s.executor, s.sessionManager)
		if err != nil {
			return nil, fmt.Errorf("failed to create Slack connector: %w", err)
		}
	}

	if cfg.Telegram.Enabled() {
		s.telegramConnector, err = telegram.NewConnector(telegram.Config{
			BotToken: cfg.Telegram.BotToken,
			Debug:    cfg.Telegram.Debug,
			Logger:   log,
		}, s.executor, s.sessionManager)
		if err != nil {
			return nil, fmt.Errorf("failed to create Telegram connector: %w", err)
		}
	}

	return s, nil
}

// Run starts the server and blocks until shutdown
func (s *Server) Run() error {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer cancel()

	s.setupGracefulShutdown()

	// Start pprof server for profiling
	go func() {
		s.log.Info("Starting pprof server on :6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			s.log.Error("pprof server failed", logger.ErrorField(err))
		}
	}()

	// Detect and start enabled connectors and services
	var wg sync.WaitGroup
	enabledCount := 0

	// Start health server
	if s.cfg.Health.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.startHealthServer(ctx); err != nil {
				s.log.Error("Health server failed", logger.ErrorField(err))
			}
		}()
	}

	// Start Slack connector if configured
	if s.slackConnector != nil {
		enabledCount++
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.log.Info("Starting Slack connector")
			if err := s.slackConnector.Start(ctx); err != nil {
				s.log.Error("Slack connector error", logger.ErrorField(err))
				cancel() // Trigger shutdown on error
			}
		}()
	} else {
		s.log.Info("Slack connector disabled (missing SLACK_BOT_TOKEN or SLACK_APP_TOKEN)")
	}

	// Start Telegram connector if configured
	if s.telegramConnector != nil {
		enabledCount++
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.log.Info("Starting Telegram bot polling")

			// Get and log bot info
			botInfo, err := s.telegramConnector.GetBotInfo(ctx)
			if err != nil {
				s.log.Warn("Failed to get Telegram bot info", logger.ErrorField(err))
			} else {
				s.log.Info("Telegram bot connected",
					logger.StringField("bot_username", botInfo.Username),
					logger.StringField("bot_first_name", botInfo.FirstName))
			}

			if err := s.telegramConnector.Start(ctx); err != nil {
				s.log.Error("Telegram connector error", logger.ErrorField(err))
				cancel() // Trigger shutdown on error
			}
		}()
	} else {
		s.log.Info("Telegram connector disabled (missing TELEGRAM_BOT_TOKEN)")
	}

	// Verify at least one connector is enabled
	if enabledCount == 0 {
		return fmt.Errorf("no connectors configured: please set environment variables for at least one platform (Slack or Telegram)")
	}

	s.log.Info("All enabled connectors started", logger.IntField("count", enabledCount))

	// Wait for all connectors to finish
	wg.Wait()
	s.log.Info("All connectors stopped")

	return nil
}

// startHealthServer initialises and starts the health check HTTP server
func (s *Server) startHealthServer(ctx context.Context) error {
	if !s.cfg.Health.Enabled {
		s.log.Info("Health checks disabled")
		return nil
	}

	s.log.Info("Starting health check server",
		logger.IntField("port", s.cfg.Health.Port),
		logger.StringField("liveness_path", s.cfg.Health.LivenessPath),
		logger.StringField("readiness_path", s.cfg.Health.ReadinessPath))

	// Create health monitor with connector checks
	healthMonitor := monitoring.NewHealthMonitor(monitoring.Config{
		Logger:            s.log,
		SlackConnector:    s.slackConnector,
		TelegramConnector: s.telegramConnector,
		Timeout:           s.cfg.Health.Timeout,
		FailureThreshold:  s.cfg.Health.FailureThreshold,
	})

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(s.cfg.Health.LivenessPath, healthMonitor.LivenessHandler())
	mux.HandleFunc(s.cfg.Health.ReadinessPath, healthMonitor.ReadinessHandler())
	mux.HandleFunc(s.cfg.Health.CombinedPath, healthMonitor.HealthHandler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Health.Port),
		Handler: mux,
	}

	// Start server in background
	go func() {
		s.log.Info("Health check server listening", logger.IntField("port", s.cfg.Health.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("Health server failed", logger.ErrorField(err))
		}
	}()

	// Wait for context cancellation, then shutdown gracefully
	<-ctx.Done()
	s.log.Info("Shutting down health server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		s.log.Error("Health server shutdown error", logger.ErrorField(err))
		return err
	}

	s.log.Info("Health server stopped")
	return nil
}

// createSessionService creates a session service based on configuration
func (s *Server) createSessionService(ctx context.Context) (session.Service, error) {
	cfg := &s.cfg.Session

	switch cfg.Backend {
	case "local":
		s.log.Info("Using local file-based session storage", logger.StringField("directory", cfg.LocalDir))

		// Ensure directory exists
		if err := os.MkdirAll(cfg.LocalDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create session directory: %w", err)
		}

		return intsession.NewSessionService(intsession.StorageConfig{
			Backend: "local",
			Local:   intsession.LocalConfig{BaseDir: cfg.LocalDir},
			Logger:  s.log,
		})

	case "s3":
		s.log.Info("Using S3-based session storage",
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
			Logger: s.log,
		})

	case "memory":
		s.log.Info("Using in-memory session storage")
		return session.InMemoryService(), nil

	default:
		return nil, fmt.Errorf("unsupported session backend: %s (must be 'local', 's3', or 'memory')", cfg.Backend)
	}
}

// createSessionManager creates a session manager based on configuration
func (s *Server) createSessionManager(ctx context.Context) (session_manager.Manager, error) {
	cfg := &s.cfg.Session
	var fileProvider intsession.FileProvider
	var metadataFile string

	switch cfg.Backend {
	case "local":
		s.log.Info("Using local file-based session manager", logger.StringField("directory", cfg.LocalDir))

		// Ensure directory exists
		if err := os.MkdirAll(cfg.LocalDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create session directory: %w", err)
		}

		fileProvider = intsession.NewLocalFileProvider(cfg.LocalDir)
		metadataFile = "sessions_metadata.json"

	case "s3":
		s.log.Info("Using S3-based session manager",
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
		s.log.Info("Using in-memory session manager (not persisted)")
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
		Logger:       s.log,
	})
}

// setupGracefulShutdown sets up signal handling for graceful shutdown
func (s *Server) setupGracefulShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		s.log.Info("Received shutdown signal", logger.StringField("signal", sig.String()))

		// Start graceful shutdown
		if s.cancel != nil {
			s.cancel()
		}

		// Give processes time to shutdown gracefully, then force exit
		time.AfterFunc(30*time.Second, func() {
			s.log.Warn("Force exiting due to timeout")
			os.Exit(1)
		})
	}()
}

// createLLMModel creates an LLM model instance based on the configured provider
func (s *Server) createLLMModel(ctx context.Context) (model.LLM, error) {
	provider := strings.ToLower(s.cfg.LLM.Provider)

	switch provider {
	case "claude":
		s.log.Info("Initialising Claude model",
			logger.StringField("model", s.cfg.Anthropic.Model))
		return anthropic.NewClaudeModel(s.cfg.Anthropic.APIKey, s.cfg.Anthropic.Model)

	case "gemini":
		s.log.Info("Initialising Gemini model",
			logger.StringField("model", s.cfg.Gemini.Model))

		// Configure the Gemini client
		clientConfig := &genai.ClientConfig{
			APIKey: s.cfg.Gemini.APIKey,
		}

		// If Vertex AI credentials are provided, use Vertex AI backend
		if s.cfg.Gemini.Project != "" && s.cfg.Gemini.Region != "" {
			clientConfig.Backend = genai.BackendVertexAI
			clientConfig.Project = s.cfg.Gemini.Project
			clientConfig.Location = s.cfg.Gemini.Region
			s.log.Info("Using Vertex AI backend",
				logger.StringField("project", s.cfg.Gemini.Project),
				logger.StringField("region", s.cfg.Gemini.Region))
		}

		return gemini.NewModel(ctx, s.cfg.Gemini.Model, clientConfig)

	case "openai":
		s.log.Info("Initialising OpenAI model",
			logger.StringField("model", s.cfg.OpenAI.Model))
		return openai.New(s.cfg.OpenAI.APIKey, s.cfg.OpenAI.Model)

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}
