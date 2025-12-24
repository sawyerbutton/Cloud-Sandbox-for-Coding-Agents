package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionKeyPrefix = "session:"
)

// RedisCache implements Cache using Redis
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

// DefaultRedisConfig returns default Redis configuration
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		PoolSize: 10,
	}
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(config RedisConfig, defaultTTL time.Duration) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
		PoolSize: config.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{
		client: client,
		ttl:    defaultTTL,
	}, nil
}

// sessionKey generates the Redis key for a session
func sessionKey(id string) string {
	return sessionKeyPrefix + id
}

// Get retrieves a session from cache
func (c *RedisCache) Get(ctx context.Context, id string) (*Session, error) {
	data, err := c.client.Get(ctx, sessionKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get session from cache: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// Set stores a session in cache
func (c *RedisCache) Set(ctx context.Context, session *Session, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.ttl
	}

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := c.client.Set(ctx, sessionKey(session.ID), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set session in cache: %w", err)
	}

	return nil
}

// Delete removes a session from cache
func (c *RedisCache) Delete(ctx context.Context, id string) error {
	if err := c.client.Del(ctx, sessionKey(id)).Err(); err != nil {
		return fmt.Errorf("failed to delete session from cache: %w", err)
	}
	return nil
}

// Touch updates the TTL of a cached session
func (c *RedisCache) Touch(ctx context.Context, id string, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.ttl
	}

	if err := c.client.Expire(ctx, sessionKey(id), ttl).Err(); err != nil {
		return fmt.Errorf("failed to touch session in cache: %w", err)
	}
	return nil
}

// Close closes the Redis client
func (c *RedisCache) Close() error {
	return c.client.Close()
}
