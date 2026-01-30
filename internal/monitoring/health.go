package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/health"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/health/checkers"
	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// HealthMonitor manages health checks and monitoring endpoints for the application
type HealthMonitor struct {
	checker   *health.HealthChecker
	logger    logger.Logger
	startTime time.Time
}

// Config holds configuration for the health monitor
type Config struct {
	Logger          logger.Logger
	AnthropicAPIURL string // URL for Anthropic API health check
	DatabaseURL     string // Optional: Database connection string for health check
}

// NewHealthMonitor creates a new health monitor with configured checks
func NewHealthMonitor(cfg Config) *HealthMonitor {
	checker := health.New(
		health.WithLogger(cfg.Logger),
		health.WithTimeout(10*time.Second),
		health.WithFailureThreshold(3),
	)

	// Add basic liveness checks
	checker.AddLivenessCheck(health.NewCheckFunc("process", func(ctx context.Context) error {
		// Process is running if we can execute this check
		return nil
	}))

	// Add readiness checks

	// Anthropic API health check
	if cfg.AnthropicAPIURL != "" {
		anthropicChecker := checkers.NewHTTPChecker(cfg.AnthropicAPIURL, "anthropic_api")
		checker.AddReadinessCheck(anthropicChecker)
	}

	// Database health check (if configured)
	if cfg.DatabaseURL != "" {
		// For now, just add a placeholder - this would need actual DB connection
		checker.AddReadinessCheck(health.NewCheckFunc("database", func(ctx context.Context) error {
			// TODO: Implement actual database ping
			return nil
		}))
	}

	return &HealthMonitor{
		checker:   checker,
		logger:    cfg.Logger,
		startTime: time.Now(),
	}
}

// LivenessHandler returns an HTTP handler for Kubernetes liveness probes
// GET /health/live - Returns 200 if the process is alive and can handle requests
func (hm *HealthMonitor) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		status, err := hm.checker.CheckLiveness(ctx)

		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"uptime":    time.Since(hm.startTime).String(),
			"checks":    status.Checks,
		}

		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			response["status"] = "unhealthy"
			response["error"] = err.Error()
			w.WriteHeader(http.StatusServiceUnavailable)
			hm.logger.Error("Liveness check failed", logger.ErrorField(err))
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler returns an HTTP handler for Kubernetes readiness probes
// GET /health/ready - Returns 200 if the service can handle requests (dependencies are healthy)
func (hm *HealthMonitor) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		status, err := hm.checker.CheckReadiness(ctx)

		response := map[string]interface{}{
			"status":    "ready",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"checks":    status.Checks,
		}

		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			response["status"] = "not_ready"
			response["error"] = err.Error()
			w.WriteHeader(http.StatusServiceUnavailable)
			hm.logger.Error("Readiness check failed", logger.ErrorField(err))
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// HealthHandler returns a combined health endpoint that includes both liveness and readiness
// GET /health - Returns comprehensive health status
func (hm *HealthMonitor) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		livenessStatus, livenessErr := hm.checker.CheckLiveness(ctx)
		readinessStatus, readinessErr := hm.checker.CheckReadiness(ctx)

		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"uptime":    time.Since(hm.startTime).String(),
			"version":   getVersion(),
			"liveness": map[string]interface{}{
				"status": "healthy",
				"checks": livenessStatus.Checks,
			},
			"readiness": map[string]interface{}{
				"status": "ready",
				"checks": readinessStatus.Checks,
			},
		}

		w.Header().Set("Content-Type", "application/json")

		// Determine overall status
		overallHealthy := true

		if livenessErr != nil {
			response["liveness"].(map[string]interface{})["status"] = "unhealthy"
			response["liveness"].(map[string]interface{})["error"] = livenessErr.Error()
			overallHealthy = false
		}

		if readinessErr != nil {
			response["readiness"].(map[string]interface{})["status"] = "not_ready"
			response["readiness"].(map[string]interface{})["error"] = readinessErr.Error()
			overallHealthy = false
		}

		if !overallHealthy {
			response["status"] = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// getVersion returns the application version
// In a real deployment, this would come from build flags or environment variables
func getVersion() string {
	// TODO: Implement version tracking via build flags or environment
	return "dev" // placeholder
}

// RegisterHandlers registers all health check endpoints on the provided mux
func (hm *HealthMonitor) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/health", hm.HealthHandler())
	mux.HandleFunc("/health/live", hm.LivenessHandler())
	mux.HandleFunc("/health/ready", hm.ReadinessHandler())
}

// ShutdownCheck adds a shutdown check to mark the service as not ready during shutdown
func (hm *HealthMonitor) ShutdownCheck() {
	// Add a readiness check that will fail once shutdown begins
	hm.checker.AddReadinessCheck(health.NewCheckFunc("shutdown", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}))
}
