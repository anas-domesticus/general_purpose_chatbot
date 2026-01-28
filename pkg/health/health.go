package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

// Check represents a single health check that can succeed or fail.
type Check interface {
	// Name returns the human-readable name of this check
	Name() string

	// Check performs the health check
	// Returns nil if healthy, error if unhealthy
	Check(ctx context.Context) error
}

// CheckFunc is a function adapter that allows simple functions to be used as checks.
type CheckFunc struct {
	name string
	fn   func(context.Context) error
}

// NewCheckFunc creates a new CheckFunc with the given name and function.
func NewCheckFunc(name string, fn func(context.Context) error) *CheckFunc {
	return &CheckFunc{
		name: name,
		fn:   fn,
	}
}

// Name returns the name of this check.
func (c *CheckFunc) Name() string {
	return c.name
}

// Check executes the check function.
func (c *CheckFunc) Check(ctx context.Context) error {
	return c.fn(ctx)
}

// CheckResult represents the result of a single health check execution.
type CheckResult struct {
	Name    string
	Healthy bool
	Error   string
	Latency time.Duration
}

// HealthStatus represents the overall health status.
type HealthStatus struct {
	Healthy bool
	Checks  []CheckResult
}

// HealthChecker manages and executes health checks for liveness and readiness probes.
type HealthChecker struct {
	livenessChecks   []Check
	readinessChecks  []Check
	timeout          time.Duration
	failureCount     map[string]int // Track consecutive failures per check
	failureThreshold int            // Number of consecutive failures before reporting unhealthy
	logger           logger.Logger
	mu               sync.RWMutex
}

// Option is a functional option for configuring HealthChecker.
type Option func(*HealthChecker)

// WithTimeout sets the timeout for individual health checks.
// Default is 5 seconds.
func WithTimeout(d time.Duration) Option {
	return func(h *HealthChecker) {
		h.timeout = d
	}
}

// WithLogger sets the logger for health check operations.
func WithLogger(l logger.Logger) Option {
	return func(h *HealthChecker) {
		h.logger = l
	}
}

// WithFailureThreshold sets the number of consecutive failures before a check is considered unhealthy.
// Default is 3.
func WithFailureThreshold(threshold int) Option {
	return func(h *HealthChecker) {
		if threshold > 0 {
			h.failureThreshold = threshold
		}
	}
}

// New creates a new HealthChecker with the given options.
func New(opts ...Option) *HealthChecker {
	h := &HealthChecker{
		timeout:          5 * time.Second,
		failureThreshold: 3,
		failureCount:     make(map[string]int),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// AddLivenessCheck adds a liveness check.
// Liveness checks determine if the process should be restarted.
func (h *HealthChecker) AddLivenessCheck(check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.livenessChecks = append(h.livenessChecks, check)
}

// AddReadinessCheck adds a readiness check.
// Readiness checks determine if the service can handle requests.
func (h *HealthChecker) AddReadinessCheck(check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.readinessChecks = append(h.readinessChecks, check)
}

// CheckLiveness executes all liveness checks and returns an error if any fail.
func (h *HealthChecker) CheckLiveness(ctx context.Context) (*HealthStatus, error) {
	h.mu.RLock()
	checks := h.livenessChecks
	h.mu.RUnlock()

	return h.executeChecks(ctx, checks)
}

// CheckReadiness executes all readiness checks and returns an error if any fail.
func (h *HealthChecker) CheckReadiness(ctx context.Context) (*HealthStatus, error) {
	h.mu.RLock()
	checks := h.readinessChecks
	h.mu.RUnlock()

	return h.executeChecks(ctx, checks)
}

// executeChecks runs all checks concurrently and aggregates the results.
func (h *HealthChecker) executeChecks(ctx context.Context, checks []Check) (*HealthStatus, error) {
	if len(checks) == 0 {
		// No checks configured - assume healthy
		return &HealthStatus{Healthy: true, Checks: []CheckResult{}}, nil
	}

	results := make([]CheckResult, len(checks))
	var wg sync.WaitGroup

	for i, check := range checks {
		wg.Add(1)
		go func(idx int, chk Check) {
			defer wg.Done()
			results[idx] = h.executeCheck(ctx, chk)
		}(i, check)
	}

	wg.Wait()

	// Aggregate results
	status := &HealthStatus{
		Healthy: true,
		Checks:  results,
	}

	var failedChecks []string
	for _, result := range results {
		if !result.Healthy {
			status.Healthy = false
			failedChecks = append(failedChecks, result.Name)
		}
	}

	if !status.Healthy {
		return status, fmt.Errorf("health checks failed: %v", failedChecks)
	}

	return status, nil
}

// executeCheck runs a single health check with timeout and failure threshold logic.
func (h *HealthChecker) executeCheck(parentCtx context.Context, check Check) CheckResult {
	ctx, cancel := context.WithTimeout(parentCtx, h.timeout)
	defer cancel()

	start := time.Now()
	err := check.Check(ctx)
	latency := time.Since(start)

	result := CheckResult{
		Name:    check.Name(),
		Latency: latency,
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if err != nil {
		// Increment failure count
		h.failureCount[check.Name()]++

		// Check if we've reached the threshold
		if h.failureCount[check.Name()] >= h.failureThreshold {
			result.Healthy = false
			result.Error = err.Error()

			if h.logger != nil {
				h.logger.Warn("Health check failed",
					logger.StringField("check", check.Name()),
					logger.StringField("error", err.Error()),
					logger.IntField("failures", h.failureCount[check.Name()]),
					logger.DurationField("latency", latency),
				)
			}
		} else {
			// Not enough failures yet - report as healthy
			result.Healthy = true
			if h.logger != nil {
				h.logger.Debug("Health check failed but below threshold",
					logger.StringField("check", check.Name()),
					logger.StringField("error", err.Error()),
					logger.IntField("failures", h.failureCount[check.Name()]),
					logger.IntField("threshold", h.failureThreshold),
				)
			}
		}
	} else {
		// Success - reset failure count
		h.failureCount[check.Name()] = 0
		result.Healthy = true

		if h.logger != nil {
			h.logger.Debug("Health check passed",
				logger.StringField("check", check.Name()),
				logger.DurationField("latency", latency),
			)
		}
	}

	return result
}