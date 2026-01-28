package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/metrics"
)

// SimpleServer represents a basic HTTP server for demonstration
type SimpleServer struct {
	log     logger.Logger
	config  *Config
	metrics metrics.Metrics
	server  *http.Server
}

// NewSimpleServer creates a new simple HTTP server instance
func NewSimpleServer(log logger.Logger, cfg *Config) (*SimpleServer, error) {
	// Create metrics with HTTP support
	m := metrics.NewMetrics(cfg.Metrics.EnableHttpMetrics, false, cfg.Metrics.EnableJobMetrics, log)

	s := &SimpleServer{
		log:     log,
		config:  cfg,
		metrics: m,
	}

	// Create HTTP server
	router := s.createRouter()
	s.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:        router,
		ReadTimeout:    cfg.HTTP.ReadTimeout(),
		WriteTimeout:   cfg.HTTP.WriteTimeout(),
		IdleTimeout:    cfg.HTTP.IdleTimeout(),
		MaxHeaderBytes: cfg.HTTP.MaxHeaderBytes,
	}

	log.Info("Simple HTTP server initialized",
		logger.IntField("http_port", cfg.HTTP.Port))

	return s, nil
}

// createRouter sets up all routes and middleware
func (s *SimpleServer) createRouter() http.Handler {
	r := chi.NewRouter()

	// Basic middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.log.HTTPMiddleware)

	// Routes
	r.Get("/", s.indexHandler)
	r.Get("/health", s.healthHandler)

	return r
}

// Listen starts the HTTP server and returns channels for error handling
func (s *SimpleServer) Listen() (chan error, func(), func(), error) {
	errChan := make(chan error, 1)

	// Start HTTP server
	go func() {
		s.log.Info("Starting HTTP server", logger.StringField("addr", s.server.Addr))
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	closer := func() {
		s.log.Info("Forcefully closing HTTP server")
		if err := s.Close(); err != nil {
			s.log.Error("Error during forced shutdown", logger.StringField("error", err.Error()))
		}
	}

	gracefulCloser := func() {
		s.log.Info("Gracefully closing HTTP server")
		if err := s.GracefulShutdown(); err != nil {
			s.log.Error("Error during graceful shutdown", logger.StringField("error", err.Error()))
		}
	}

	return errChan, closer, gracefulCloser, nil
}

// GracefulShutdown gracefully shuts down the HTTP server
func (s *SimpleServer) GracefulShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	return nil
}

// Close forcefully shuts down the server
func (s *SimpleServer) Close() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// Handler methods

func (s *SimpleServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"message": "Hello from Go boilerplate CLI server! 👋",
		"time":    time.Now().Format(time.RFC3339),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *SimpleServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}