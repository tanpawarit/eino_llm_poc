package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Constants for session management
const (
	// SessionTTL is the default session TTL (40 minutes)
	SessionTTL = 40 * time.Minute
)

// RedisStorage handles Redis operations for session storage
type RedisStorage struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisStorage creates a new Redis storage instance
func NewRedisStorage(ctx context.Context) (*RedisStorage, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable is required")
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	_, err = client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{
		client: client,
		ctx:    ctx,
	}, nil
}

// SetSessionData stores session data with TTL (40 minutes)
func (r *RedisStorage) SetSessionData(sessionID string, data any, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := fmt.Sprintf("session:%s", sessionID)
	err = r.client.Set(r.ctx, key, jsonData, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set session data: %w", err)
	}

	return nil
}

// SetSession stores session data with the default 40-minute TTL
func (r *RedisStorage) SetSession(sessionID string, data any) error {
	return r.SetSessionData(sessionID, data, SessionTTL)
}

// GetSessionData retrieves session data from Redis
func (r *RedisStorage) GetSessionData(sessionID string, dest any) error {
	key := fmt.Sprintf("session:%s", sessionID)
	jsonData, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to get session data: %w", err)
	}

	err = json.Unmarshal([]byte(jsonData), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return nil
}

// DeleteSession removes session from Redis
func (r *RedisStorage) DeleteSession(sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	err := r.client.Del(r.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// ExtendTTL extends the TTL of a session
func (r *RedisStorage) ExtendTTL(sessionID string, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	err := r.client.Expire(r.ctx, key, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to extend TTL: %w", err)
	}
	return nil
}

// SessionExists checks if a session exists
func (r *RedisStorage) SessionExists(sessionID string) (bool, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	count, err := r.client.Exists(r.ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check session existence: %w", err)
	}
	return count > 0, nil
}

// GetTTL gets remaining TTL for a session
func (r *RedisStorage) GetTTL(sessionID string) (time.Duration, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	ttl, err := r.client.TTL(r.ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}
	return ttl, nil
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}

// Ping tests Redis connection
func (r *RedisStorage) Ping() error {
	_, err := r.client.Ping(r.ctx).Result()
	return err
}
