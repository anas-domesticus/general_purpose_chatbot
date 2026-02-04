package utils

import (
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/metrics"
)

// ServerConfig represents configuration options for creating a Server.
type ServerConfig struct {
	GrpcListenPort int
	HttpListenPort int
	Metrics        metrics.Metrics
}

// Server represents a server configuration with logging, metrics, and port settings.
type Server struct {
	Log            logger.Logger
	GrpcListenPort int
	HttpListenPort int
	Metrics        metrics.Metrics
}

// NewServer creates a new Server instance with the given logger and configuration.
// If config is nil, default values are used (gRPC: 8000, HTTP: 8080).
//
// Usage:
//
//	config := &ServerConfig{
//		GrpcListenPort: 9000,
//		HttpListenPort: 9001,
//		Metrics:        metrics,
//	}
//	server := NewServer(logger, config)
func NewServer(log logger.Logger, config *ServerConfig) Server {
	server := Server{
		Log:            log,
		GrpcListenPort: 8000, // default gRPC port
		HttpListenPort: 8080, // default HTTP port
	}

	if config != nil {
		if config.GrpcListenPort != 0 {
			server.GrpcListenPort = config.GrpcListenPort
		}
		if config.HttpListenPort != 0 {
			server.HttpListenPort = config.HttpListenPort
		}
		if config.Metrics.TotalHttpRequestsCounter != nil || config.Metrics.TotalGrpcRequestsCounter != nil {
			server.Metrics = config.Metrics
		}
	}

	return server
}
