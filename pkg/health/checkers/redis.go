package checkers

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisChecker checks the health of a Redis connection.
type RedisChecker struct {
	client *redis.Client
	name   string
}

// NewRedisChecker creates a new Redis health checker.
// The name parameter allows customization of the check name (e.g., "redis-cache", "redis-sessions").
// If name is empty, defaults to "redis".
func NewRedisChecker(client *redis.Client, name string) *RedisChecker {
	if name == "" {
		name = "redis"
	}
	return &RedisChecker{
		client: client,
		name:   name,
	}
}

// Name returns the name of this health check.
func (r *RedisChecker) Name() string {
	return r.name
}

// Check performs a ping to the Redis server to verify connectivity.
func (r *RedisChecker) Check(ctx context.Context) error {
	err := r.client.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}