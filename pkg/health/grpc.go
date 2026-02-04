// Package health provides health checking infrastructure for services.
package health

import (
	"context"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

const (
	// DefaultGRPCUpdateInterval is the default interval for updating gRPC health status
	DefaultGRPCUpdateInterval = 5 * time.Second
)

// grpcHealthUpdater manages the background updates to the gRPC health server.
type grpcHealthUpdater struct {
	checker        *HealthChecker
	healthServer   *health.Server
	updateInterval time.Duration
	stopChan       chan struct{}
	stopped        atomic.Bool
}

// RegisterWithGRPC registers the health checker with a gRPC server using the official
// gRPC health checking protocol (grpc.health.v1.Health).
//
// This starts a background goroutine that periodically checks readiness and updates
// the gRPC health status. The update interval is 5 seconds by default.
//
// The health service is registered with an empty service name (""), which represents
// the overall server health status.
func (h *HealthChecker) RegisterWithGRPC(server *grpc.Server) *grpcHealthUpdater {
	return h.RegisterWithGRPCAndInterval(server, DefaultGRPCUpdateInterval)
}

// RegisterWithGRPCAndInterval registers the health checker with a gRPC server and allows
// customization of the update interval.
func (h *HealthChecker) RegisterWithGRPCAndInterval(server *grpc.Server, updateInterval time.Duration) *grpcHealthUpdater {
	// Create gRPC health server
	healthServer := health.NewServer()

	// Register with gRPC
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	// Set initial status to NOT_SERVING until first check completes
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// Create updater
	updater := &grpcHealthUpdater{
		checker:        h,
		healthServer:   healthServer,
		updateInterval: updateInterval,
		stopChan:       make(chan struct{}),
	}

	// Start background updater
	go updater.run()

	if h.logger != nil {
		h.logger.Info("gRPC health service registered",
			logger.StringField("update_interval", updateInterval.String()),
		)
	}

	return updater
}

// run is the main loop for the gRPC health updater.
func (u *grpcHealthUpdater) run() {
	ticker := time.NewTicker(u.updateInterval)
	defer ticker.Stop()

	// Perform initial check immediately
	u.updateHealth()

	for {
		select {
		case <-ticker.C:
			u.updateHealth()
		case <-u.stopChan:
			// Graceful shutdown - mark as NOT_SERVING
			u.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
			if u.checker.logger != nil {
				u.checker.logger.Info("gRPC health updater stopped")
			}
			return
		}
	}
}

// updateHealth checks readiness and updates the gRPC health status.
func (u *grpcHealthUpdater) updateHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), u.updateInterval)
	defer cancel()

	status, err := u.checker.CheckReadiness(ctx)

	if err != nil || !status.Healthy {
		u.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		if u.checker.logger != nil {
			u.checker.logger.Debug("gRPC health status: NOT_SERVING",
				logger.StringField("reason", "readiness check failed"),
			)
		}
	} else {
		u.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		if u.checker.logger != nil {
			u.checker.logger.Debug("gRPC health status: SERVING")
		}
	}
}

// Stop gracefully stops the gRPC health updater.
// This should be called during application shutdown.
func (u *grpcHealthUpdater) Stop() {
	if u.stopped.CompareAndSwap(false, true) {
		close(u.stopChan)
	}
}

// Shutdown marks the service as shutting down and stops health updates.
// This is an alias for Stop() to match common shutdown patterns.
func (u *grpcHealthUpdater) Shutdown() {
	u.Stop()
}
