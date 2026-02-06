// Package server provides the main server implementation for the chatbot.
package server

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: pprof is intentionally enabled for debugging
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lewisedginton/general_purpose_chatbot/internal/agents"
	"github.com/lewisedginton/general_purpose_chatbot/internal/artifact_service"
	appconfig "github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/executor"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/slack"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/telegram"
	"github.com/lewisedginton/general_purpose_chatbot/internal/memory_service"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/anthropic"
	"github.com/lewisedginton/general_purpose_chatbot/internal/models/openai"
	"github.com/lewisedginton/general_purpose_chatbot/internal/monitoring"
	"github.com/lewisedginton/general_purpose_chatbot/internal/prompt_manager"
	"github.com/lewisedginton/general_purpose_chatbot/internal/session_manager"
	"github.com/lewisedginton/general_purpose_chatbot/internal/skills_manager"
	"github.com/lewisedginton/general_purpose_chatbot/internal/storage_manager"
	"github.com/lewisedginton/general_purpose_chatbot/internal/tools/agent_info"
	"github.com/lewisedginton/general_purpose_chatbot/internal/tools/http_request"
	"github.com/lewisedginton/general_purpose_chatbot/internal/tools/web_search"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
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
	storageManager    *storage_manager.StorageManager
	sessionManager    session_manager.Manager
	memoryService     memory.Service
	artifactService   artifact.Service
	skillsManager     skills_manager.Manager
	promptManager     *prompt_manager.PromptManager
	cancel            context.CancelFunc
}

// New creates a new Server instance with all components initialized
//
//nolint:revive // cognitive-complexity: Server initialization requires sequential component setup
func New(ctx context.Context, cfg *appconfig.AppConfig, log logger.Logger) (*Server, error) {
	s := &Server{
		cfg: cfg,
		log: log,
	}

	// Create storage manager (handles persistence for sessions and metadata)
	var err error
	s.storageManager, err = s.createStorageManager(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage manager: %w", err)
	}

	// Create session manager (includes ADK session service)
	s.sessionManager, err = s.createSessionManager() //nolint:contextcheck // Session manager creation doesn't need request context
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	// Create memory service (uses storage manager with "memory" namespace)
	s.memoryService = s.createMemoryService()

	// Create skills manager
	s.skillsManager, err = s.createSkillsManager() //nolint:contextcheck // Skills manager creation doesn't need request context
	if err != nil {
		return nil, fmt.Errorf("failed to create skills manager: %w", err)
	}

	// Create artifact service
	s.artifactService = s.createArtifactService()

	// Create prompt manager using local filesystem (prompts are part of deployment, not user data)
	promptProvider := storage_manager.NewLocalFileProvider("prompts")
	s.promptManager = prompt_manager.New(promptProvider)

	// Create LLM model instance based on configured provider
	llmModel, err := s.createLLMModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM model: %w", err)
	}

	// Create tools for the agent
	tools, err := s.createTools(llmModel) //nolint:contextcheck // Tool creation doesn't need request context
	if err != nil {
		return nil, fmt.Errorf("failed to create tools: %w", err)
	}

	// Create generic chat agent factory (shared across all platforms)
	chatAgentFactory, err := agents.NewChatAgent(ctx, llmModel, cfg.MCP, agents.AgentConfig{
		Name:           "chat_assistant",
		Platform:       "Multi-Platform",
		Description:    "AI assistant with MCP capabilities",
		Logger:         log,
		PromptProvider: s.promptManager,
	}, tools)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat agent factory: %w", err)
	}

	// Create executor with agent factory (shared across all platforms)
	s.executor, err = executor.NewExecutorWithConfig(executor.Config{
		AgentFactory:    chatAgentFactory,
		AppName:         "chatbot",
		SessionService:  s.sessionManager.GetADKSessionService(),
		ArtifactService: s.artifactService,
		MemoryService:   s.memoryService,
		Logger:          log,
	})
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
//
//nolint:revive // cognitive-complexity: Server orchestration requires managing multiple connectors
func (s *Server) Run() error {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer cancel()

	s.setupGracefulShutdown()

	// Start pprof server for profiling (localhost only for security)
	go func() {
		s.log.Info("Starting pprof server on :6060")
		pprofServer := &http.Server{
			Addr:              "localhost:6060",
			Handler:           nil, // Uses DefaultServeMux with pprof handlers
			ReadHeaderTimeout: 10 * time.Second,
		}
		if err := pprofServer.ListenAndServe(); err != nil {
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

// startHealthServer initializes and starts the health check HTTP server
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
		Addr:              fmt.Sprintf(":%d", s.cfg.Health.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:contextcheck // New context needed for shutdown
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck // Using new context for graceful shutdown
		s.log.Error("Health server shutdown error", logger.ErrorField(err))
		return err
	}

	s.log.Info("Health server stopped")
	return nil
}

// createStorageManager creates a storage manager based on configuration
func (s *Server) createStorageManager(ctx context.Context) (*storage_manager.StorageManager, error) {
	cfg := &s.cfg.Storage

	switch cfg.Backend {
	case "local":
		s.log.Info("Using local file-based storage", logger.StringField("directory", cfg.LocalDir))

		// Ensure directory exists (0750 needed for directory traversal)
		if err := os.MkdirAll(cfg.LocalDir, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create storage directory: %w", err)
		}

		return storage_manager.New(storage_manager.Config{
			Backend: storage_manager.BackendLocal,
			LocalConfig: &storage_manager.LocalConfig{
				BaseDir: cfg.LocalDir,
			},
		})

	case "s3":
		s.log.Info("Using S3-based storage",
			logger.StringField("bucket", cfg.S3Bucket),
			logger.StringField("prefix", cfg.S3Prefix),
			logger.StringField("region", cfg.S3Region))

		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3 bucket is required when using S3 storage")
		}

		// Load AWS configuration
		configOptions := []func(*awsconfig.LoadOptions) error{}

		if cfg.S3Profile != "" {
			configOptions = append(configOptions, awsconfig.WithSharedConfigProfile(cfg.S3Profile))
		}

		if cfg.S3Region != "" {
			configOptions = append(configOptions, awsconfig.WithRegion(cfg.S3Region))
		}

		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, configOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		// Create S3 client
		s3Client := s3.NewFromConfig(awsCfg)

		return storage_manager.New(storage_manager.Config{
			Backend: storage_manager.BackendS3,
			S3Config: &storage_manager.S3Config{
				Bucket: cfg.S3Bucket,
				Prefix: cfg.S3Prefix,
				Client: s3Client,
			},
		})

	default:
		return nil, fmt.Errorf("unsupported storage backend: %s (must be 'local' or 's3')", cfg.Backend)
	}
}

// createSessionManager creates a session manager using the storage manager
func (s *Server) createSessionManager() (session_manager.Manager, error) {
	// Use storage manager with "sessions" namespace
	provider := s.storageManager.GetProvider("sessions")

	return session_manager.New(session_manager.Config{
		MetadataFile: "sessions.json",
		FileProvider: provider,
		Logger:       s.log,
	})
}

// createSkillsManager creates a skills manager using the storage manager
func (s *Server) createSkillsManager() (skills_manager.Manager, error) {
	// Use storage manager with "skills" namespace
	provider := s.storageManager.GetProvider("skills")

	return skills_manager.New(skills_manager.Config{
		FileProvider: provider,
		Logger:       s.log,
	})
}

// createArtifactService creates an artifact service using the storage manager.
func (s *Server) createArtifactService() artifact.Service {
	// Use storage manager with "artifacts" namespace
	provider := s.storageManager.GetProvider("artifacts")
	return artifact_service.NewArtifactService(provider, s.log)
}

// createMemoryService creates a memory service using the storage manager
func (s *Server) createMemoryService() memory.Service {
	// Use storage manager with "memory" namespace
	provider := s.storageManager.GetProvider("memory")

	return memory_service.New(memory_service.Config{
		FileProvider: provider,
		Logger:       s.log,
	})
}

// createTools creates the tools for the agent
func (s *Server) createTools(llmModel model.LLM) ([]tool.Tool, error) {
	var tools []tool.Tool

	// Create agent info tool
	agentInfoTool, err := agent_info.New(agent_info.Config{
		AgentName:   "chat_assistant",
		Platform:    "Multi-Platform",
		Description: "AI assistant with MCP capabilities",
		Model:       llmModel,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent info tool: %w", err)
	}
	tools = append(tools, agentInfoTool)

	// Create HTTP request tool
	httpRequestTool, err := http_request.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create http request tool: %w", err)
	}
	tools = append(tools, httpRequestTool)

	// Add skills tools
	skillsTools, err := s.skillsManager.Tools()
	if err != nil {
		return nil, fmt.Errorf("failed to create skills tools: %w", err)
	}
	tools = append(tools, skillsTools...)

	// Add prompt manager tools
	promptTools, err := s.promptManager.Tools()
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt tools: %w", err)
	}
	tools = append(tools, promptTools...)

	// Add web search tool if API key is configured
	if s.cfg.Search.Enabled() {
		webSearchTool, err := web_search.New(web_search.Config{
			APIKey:  s.cfg.Search.APIKey,
			BaseURL: s.cfg.Search.BaseURL,
			Timeout: s.cfg.Search.Timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create web search tool: %w", err)
		}
		tools = append(tools, webSearchTool)
		s.log.Info("Web search tool enabled")
	}

	return tools, nil
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
		s.log.Info("Initializing Claude model",
			logger.StringField("model", s.cfg.Anthropic.Model))
		return anthropic.NewClaudeModel(s.cfg.Anthropic.APIKey, s.cfg.Anthropic.Model)

	case "gemini":
		s.log.Info("Initializing Gemini model",
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
		s.log.Info("Initializing OpenAI model",
			logger.StringField("model", s.cfg.OpenAI.Model))
		return openai.New(s.cfg.OpenAI.APIKey, s.cfg.OpenAI.Model)

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}
