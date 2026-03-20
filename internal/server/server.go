// Package server provides the top-level application server.
package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	acpclient "github.com/lewisedginton/general_purpose_chatbot/internal/acp"
	"github.com/lewisedginton/general_purpose_chatbot/internal/config"
	"github.com/lewisedginton/general_purpose_chatbot/internal/connectors/slack"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/health"
	"go.uber.org/zap"
)

// Server is the top-level application server.
type Server struct {
	cfg            *config.AppConfig
	log            *zap.SugaredLogger
	acpExecutor    *acpclient.Executor
	acpRouter      *acpclient.Router
	slackConnector *slack.Connector
}

// New creates a new Server with the given configuration.
func New(_ context.Context, cfg *config.AppConfig, log *zap.SugaredLogger) (*Server, error) {
	s := &Server{cfg: cfg, log: log}

	s.acpRouter = acpclient.NewRouter(cfg.ACP)
	s.acpExecutor = acpclient.NewExecutor(log)

	if cfg.Slack.Enabled() {
		connector, err := slack.NewConnector(slack.Config{
			BotToken: cfg.Slack.BotToken,
			AppToken: cfg.Slack.AppToken,
			Debug:    cfg.Slack.Debug,
		}, s.acpExecutor, s.acpRouter, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create slack connector: %w", err)
		}
		s.slackConnector = connector
	}

	return s, nil
}

// Run starts all enabled connectors and blocks until they exit.
func (s *Server) Run() error {
	defer s.acpExecutor.Shutdown()

	if s.slackConnector == nil {
		return fmt.Errorf("no connectors enabled")
	}

	// Start health check HTTP server if enabled.
	if s.cfg.Health.Enabled {
		s.startHealthServer()
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	if s.slackConnector != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			if err := s.slackConnector.Start(ctx); err != nil {
				errCh <- fmt.Errorf("slack connector error: %w", err)
			}
		}()
	}

	// Wait for first error or all to finish.
	go func() {
		wg.Wait()
		close(errCh)
	}()

	if err, ok := <-errCh; ok {
		return err
	}
	return nil
}

func (s *Server) startHealthServer() {
	checker := health.New(
		health.WithLogger(s.log),
		health.WithTimeout(s.cfg.Health.Timeout),
		health.WithFailureThreshold(s.cfg.Health.FailureThreshold),
	)

	// Liveness: process is alive if this handler can respond.
	checker.AddLivenessCheck(health.NewCheckFunc("process", func(_ context.Context) error {
		return nil
	}))

	// Readiness: Slack connector is connected.
	if s.slackConnector != nil {
		checker.AddReadinessCheck(health.NewCheckFunc("slack", func(_ context.Context) error {
			return s.slackConnector.Ready()
		}))
	}

	mux := http.NewServeMux()
	mux.HandleFunc(s.cfg.Health.LivenessPath, checker.LivenessHandler())
	mux.HandleFunc(s.cfg.Health.ReadinessPath, checker.ReadinessHandler())

	addr := fmt.Sprintf(":%d", s.cfg.Health.Port)
	s.log.Infow("Starting health check server", "addr", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil { //nolint:gosec // health server; no need for timeouts
			s.log.Errorw("Health check server error", "error", err)
		}
	}()
}
