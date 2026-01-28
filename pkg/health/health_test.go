package health

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCheck is a simple test implementation of the Check interface.
type mockCheck struct {
	name      string
	err       error
	sleepTime time.Duration
}

func (m *mockCheck) Name() string {
	return m.name
}

func (m *mockCheck) Check(ctx context.Context) error {
	if m.sleepTime > 0 {
		select {
		case <-time.After(m.sleepTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.err
}

func TestNew(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		h := New()
		assert.NotNil(t, h)
		assert.Equal(t, 5*time.Second, h.timeout)
		assert.Equal(t, 3, h.failureThreshold)
		assert.NotNil(t, h.failureCount)
	})

	t.Run("with custom timeout", func(t *testing.T) {
		h := New(WithTimeout(10 * time.Second))
		assert.Equal(t, 10*time.Second, h.timeout)
	})

	t.Run("with custom failure threshold", func(t *testing.T) {
		h := New(WithFailureThreshold(5))
		assert.Equal(t, 5, h.failureThreshold)
	})

	t.Run("invalid failure threshold ignored", func(t *testing.T) {
		h := New(WithFailureThreshold(0))
		assert.Equal(t, 3, h.failureThreshold) // default
	})
}

func TestCheckFunc(t *testing.T) {
	t.Run("successful check", func(t *testing.T) {
		check := NewCheckFunc("test", func(ctx context.Context) error {
			return nil
		})

		assert.Equal(t, "test", check.Name())
		assert.NoError(t, check.Check(context.Background()))
	})

	t.Run("failing check", func(t *testing.T) {
		expectedErr := errors.New("test error")
		check := NewCheckFunc("test", func(ctx context.Context) error {
			return expectedErr
		})

		assert.Equal(t, "test", check.Name())
		assert.Equal(t, expectedErr, check.Check(context.Background()))
	})
}

func TestHealthChecker_Liveness(t *testing.T) {
	t.Run("no checks configured", func(t *testing.T) {
		h := New()
		status, err := h.CheckLiveness(context.Background())

		assert.NoError(t, err)
		assert.True(t, status.Healthy)
		assert.Empty(t, status.Checks)
	})

	t.Run("single successful check", func(t *testing.T) {
		h := New()
		h.AddLivenessCheck(&mockCheck{name: "test", err: nil})

		status, err := h.CheckLiveness(context.Background())

		assert.NoError(t, err)
		assert.True(t, status.Healthy)
		require.Len(t, status.Checks, 1)
		assert.Equal(t, "test", status.Checks[0].Name)
		assert.True(t, status.Checks[0].Healthy)
		assert.Empty(t, status.Checks[0].Error)
	})

	t.Run("single failing check", func(t *testing.T) {
		h := New(WithFailureThreshold(1))
		h.AddLivenessCheck(&mockCheck{name: "test", err: errors.New("test error")})

		status, err := h.CheckLiveness(context.Background())

		assert.Error(t, err)
		assert.False(t, status.Healthy)
		require.Len(t, status.Checks, 1)
		assert.Equal(t, "test", status.Checks[0].Name)
		assert.False(t, status.Checks[0].Healthy)
		assert.Equal(t, "test error", status.Checks[0].Error)
	})

	t.Run("multiple checks mixed results", func(t *testing.T) {
		h := New(WithFailureThreshold(1))
		h.AddLivenessCheck(&mockCheck{name: "success", err: nil})
		h.AddLivenessCheck(&mockCheck{name: "failure", err: errors.New("test error")})

		status, err := h.CheckLiveness(context.Background())

		assert.Error(t, err)
		assert.False(t, status.Healthy)
		assert.Len(t, status.Checks, 2)

		// Find checks by name
		var successCheck, failureCheck CheckResult
		for _, check := range status.Checks {
			if check.Name == "success" {
				successCheck = check
			} else if check.Name == "failure" {
				failureCheck = check
			}
		}

		assert.True(t, successCheck.Healthy)
		assert.False(t, failureCheck.Healthy)
	})
}

func TestHealthChecker_Readiness(t *testing.T) {
	t.Run("no checks configured", func(t *testing.T) {
		h := New()
		status, err := h.CheckReadiness(context.Background())

		assert.NoError(t, err)
		assert.True(t, status.Healthy)
		assert.Empty(t, status.Checks)
	})

	t.Run("single successful check", func(t *testing.T) {
		h := New()
		h.AddReadinessCheck(&mockCheck{name: "test", err: nil})

		status, err := h.CheckReadiness(context.Background())

		assert.NoError(t, err)
		assert.True(t, status.Healthy)
		require.Len(t, status.Checks, 1)
		assert.Equal(t, "test", status.Checks[0].Name)
		assert.True(t, status.Checks[0].Healthy)
	})

	t.Run("single failing check", func(t *testing.T) {
		h := New(WithFailureThreshold(1))
		h.AddReadinessCheck(&mockCheck{name: "test", err: errors.New("test error")})

		status, err := h.CheckReadiness(context.Background())

		assert.Error(t, err)
		assert.False(t, status.Healthy)
		require.Len(t, status.Checks, 1)
		assert.Equal(t, "test", status.Checks[0].Name)
		assert.False(t, status.Checks[0].Healthy)
	})
}

func TestHealthChecker_FailureThreshold(t *testing.T) {
	t.Run("failure below threshold", func(t *testing.T) {
		h := New(WithFailureThreshold(3))
		check := &mockCheck{name: "test", err: errors.New("test error")}
		h.AddLivenessCheck(check)

		// First two failures should still report healthy
		for i := 0; i < 2; i++ {
			status, err := h.CheckLiveness(context.Background())
			assert.NoError(t, err)
			assert.True(t, status.Healthy)
			require.Len(t, status.Checks, 1)
			assert.True(t, status.Checks[0].Healthy)
		}

		// Third failure should report unhealthy
		status, err := h.CheckLiveness(context.Background())
		assert.Error(t, err)
		assert.False(t, status.Healthy)
		require.Len(t, status.Checks, 1)
		assert.False(t, status.Checks[0].Healthy)
	})

	t.Run("recovery resets failure count", func(t *testing.T) {
		h := New(WithFailureThreshold(3))
		check := &mockCheck{name: "test", err: errors.New("test error")}
		h.AddLivenessCheck(check)

		// Two failures
		for i := 0; i < 2; i++ {
			h.CheckLiveness(context.Background())
		}

		// Success should reset counter
		check.err = nil
		status, err := h.CheckLiveness(context.Background())
		assert.NoError(t, err)
		assert.True(t, status.Healthy)

		// Next failure should start counting from 1 again
		check.err = errors.New("test error")
		status, err = h.CheckLiveness(context.Background())
		assert.NoError(t, err) // Should still be healthy (count = 1)
		assert.True(t, status.Healthy)
	})
}

func TestHealthChecker_Timeout(t *testing.T) {
	t.Run("check times out", func(t *testing.T) {
		h := New(WithTimeout(100*time.Millisecond), WithFailureThreshold(1))
		h.AddLivenessCheck(&mockCheck{
			name:      "slow",
			sleepTime: 200 * time.Millisecond,
		})

		start := time.Now()
		status, err := h.CheckLiveness(context.Background())
		duration := time.Since(start)

		assert.Error(t, err)
		assert.False(t, status.Healthy)
		assert.Less(t, duration, 150*time.Millisecond) // Should timeout quickly

		require.Len(t, status.Checks, 1)
		assert.False(t, status.Checks[0].Healthy)
		assert.Contains(t, status.Checks[0].Error, "context deadline exceeded")
	})
}

func TestHealthChecker_ConcurrentChecks(t *testing.T) {
	t.Run("checks run concurrently", func(t *testing.T) {
		h := New()
		
		// Add multiple slow checks
		for i := 0; i < 3; i++ {
			h.AddLivenessCheck(&mockCheck{
				name:      fmt.Sprintf("slow-%d", i),
				sleepTime: 100 * time.Millisecond,
			})
		}

		start := time.Now()
		status, err := h.CheckLiveness(context.Background())
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.True(t, status.Healthy)
		assert.Len(t, status.Checks, 3)

		// Should complete in ~100ms (concurrent) not ~300ms (sequential)
		assert.Less(t, duration, 150*time.Millisecond)
	})
}