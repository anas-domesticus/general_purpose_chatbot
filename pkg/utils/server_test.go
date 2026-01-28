package utils

import (
	"testing"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/metrics"
)

func TestNewServer(t *testing.T) {
	t.Run("creates server with default values", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		server := NewServer(log, nil)

		if server.GrpcListenPort != 8000 {
			t.Errorf("Expected default gRPC port 8000, got %d", server.GrpcListenPort)
		}

		if server.HttpListenPort != 8080 {
			t.Errorf("Expected default HTTP port 8080, got %d", server.HttpListenPort)
		}

		if server.Log != log {
			t.Error("Logger was not set correctly")
		}
	})

	t.Run("creates server with custom gRPC port", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		config := &ServerConfig{
			GrpcListenPort: 9000,
		}
		server := NewServer(log, config)

		if server.GrpcListenPort != 9000 {
			t.Errorf("Expected gRPC port 9000, got %d", server.GrpcListenPort)
		}

		if server.HttpListenPort != 8080 {
			t.Errorf("Expected default HTTP port 8080, got %d", server.HttpListenPort)
		}
	})

	t.Run("creates server with custom HTTP port", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		config := &ServerConfig{
			HttpListenPort: 9001,
		}
		server := NewServer(log, config)

		if server.GrpcListenPort != 8000 {
			t.Errorf("Expected default gRPC port 8000, got %d", server.GrpcListenPort)
		}

		if server.HttpListenPort != 9001 {
			t.Errorf("Expected HTTP port 9001, got %d", server.HttpListenPort)
		}
	})

	t.Run("creates server with custom metrics", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		m := metrics.NewMetrics(true, false, false, log)
		config := &ServerConfig{
			Metrics: m,
		}
		server := NewServer(log, config)

		// Basic check that metrics is set (detailed testing would require more setup)
		if server.Metrics.TotalHttpRequestsCounter == nil {
			t.Error("Metrics was not set correctly")
		}
	})

	t.Run("creates server with all custom options", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		m := metrics.NewMetrics(false, true, false, log)
		config := &ServerConfig{
			GrpcListenPort: 9000,
			HttpListenPort: 9001,
			Metrics:        m,
		}
		server := NewServer(log, config)

		if server.GrpcListenPort != 9000 {
			t.Errorf("Expected gRPC port 9000, got %d", server.GrpcListenPort)
		}

		if server.HttpListenPort != 9001 {
			t.Errorf("Expected HTTP port 9001, got %d", server.HttpListenPort)
		}

		if server.Metrics.TotalGrpcRequestsCounter == nil {
			t.Error("Metrics was not set correctly")
		}
	})

	t.Run("ignores zero values in config", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		config := &ServerConfig{
			GrpcListenPort: 0, // Should be ignored
			HttpListenPort: 9001,
		}
		server := NewServer(log, config)

		if server.GrpcListenPort != 8000 {
			t.Errorf("Expected default gRPC port 8000, got %d", server.GrpcListenPort)
		}

		if server.HttpListenPort != 9001 {
			t.Errorf("Expected HTTP port 9001, got %d", server.HttpListenPort)
		}
	})
}

func TestServerConfig(t *testing.T) {
	t.Run("ServerConfig struct creation", func(t *testing.T) {
		log := logger.NewLogger(logger.Config{
			Level:   logger.InfoLevel,
			Format:  "json",
			Service: "test",
		})

		m := metrics.NewMetrics(true, true, false, log)
		config := &ServerConfig{
			GrpcListenPort: 12345,
			HttpListenPort: 54321,
			Metrics:        m,
		}

		if config.GrpcListenPort != 12345 {
			t.Errorf("Expected gRPC port 12345, got %d", config.GrpcListenPort)
		}

		if config.HttpListenPort != 54321 {
			t.Errorf("Expected HTTP port 54321, got %d", config.HttpListenPort)
		}

		if config.Metrics.TotalHttpRequestsCounter == nil {
			t.Error("HTTP metrics was not set correctly")
		}

		if config.Metrics.TotalGrpcRequestsCounter == nil {
			t.Error("gRPC metrics was not set correctly")
		}
	})
}