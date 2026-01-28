package checkers

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisChecker(t *testing.T) {
	t.Run("uses provided name", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		defer client.Close()

		checker := NewRedisChecker(client, "redis-sessions")
		assert.Equal(t, "redis-sessions", checker.Name())
	})

	t.Run("uses default name when empty", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		defer client.Close()

		checker := NewRedisChecker(client, "")
		assert.Equal(t, "redis", checker.Name())
	})

	t.Run("fails when redis is unreachable", func(t *testing.T) {
		// Connect to non-existent Redis instance
		client := redis.NewClient(&redis.Options{
			Addr:         "localhost:1", // Invalid port
			DialTimeout:  100,
			ReadTimeout:  100,
			WriteTimeout: 100,
		})
		defer client.Close()

		checker := NewRedisChecker(client, "test")
		err := checker.Check(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis ping failed")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{
			Addr: "localhost:1", // Will timeout
		})
		defer client.Close()

		checker := NewRedisChecker(client, "test")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := checker.Check(ctx)
		assert.Error(t, err)
	})
}

// Note: For integration tests with a real Redis instance, you would run:
// func TestRedisCheckerIntegration(t *testing.T) {
//     if testing.Short() {
//         t.Skip("Skipping integration test")
//     }
//
//     client := redis.NewClient(&redis.Options{
//         Addr: "localhost:6379",
//     })
//     defer client.Close()
//
//     checker := NewRedisChecker(client, "test")
//     err := checker.Check(context.Background())
//     assert.NoError(t, err)
// }