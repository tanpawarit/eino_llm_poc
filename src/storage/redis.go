package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	SessionTTL    = 60 * time.Minute
	sessionPrefix = "session:"
)

// RedisStorage implements SessionStorage using Redis
type RedisStorage struct {
	client *redis.Client
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

	return &RedisStorage{client: client}, nil
}

// key generates a Redis key for the given session ID
func (r *RedisStorage) key(sessionID string) string {
	return sessionPrefix + sessionID
}

// Set stores session data with TTL
func (r *RedisStorage) Set(ctx context.Context, sessionID string, data any, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	err = r.client.Set(ctx, r.key(sessionID), jsonData, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set session data: %w", err)
	}

	return nil
}

// SetSession stores session data with the default TTL (convenience method)
func (r *RedisStorage) SetSession(ctx context.Context, sessionID string, data any) error {
	return r.Set(ctx, sessionID, data, SessionTTL)
}

// Get retrieves session data from Redis
func (r *RedisStorage) Get(ctx context.Context, sessionID string, dest any) error {
	jsonData, err := r.client.Get(ctx, r.key(sessionID)).Result()
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

// GetAndTouch read and extend TTL
func (r *RedisStorage) GetAndTouch(ctx context.Context, sessionID string, dest any) error {
	cmd := r.client.Do(ctx, "GETEX", r.key(sessionID), "EX", SessionTTL)
	s, err := cmd.Text()
	if err != nil {
		if err == redis.Nil || s == "" {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to GETEX: %w", err)
	}
	if err := json.Unmarshal([]byte(s), dest); err != nil {
		return fmt.Errorf("failed to unmarshal session data: %w", err)
	}
	return nil
}

// GetSession is a convenience method that uses the struct's context
func (r *RedisStorage) GetSessionData(sessionID string, dest any) error {
	return r.Get(context.Background(), sessionID, dest)
}

// Delete removes session from Redis
func (r *RedisStorage) Delete(ctx context.Context, sessionID string) error {
	err := r.client.Del(ctx, r.key(sessionID)).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// DeleteSession is a convenience method
func (r *RedisStorage) DeleteSession(sessionID string) error {
	return r.Delete(context.Background(), sessionID)
}

// ExtendTTL extends the TTL of a session
func (r *RedisStorage) ExtendTTL(ctx context.Context, sessionID string, ttl time.Duration) error {
	err := r.client.Expire(ctx, r.key(sessionID), ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to extend TTL: %w", err)
	}
	return nil
}

// Exists checks if a session exists
func (r *RedisStorage) Exists(ctx context.Context, sessionID string) (bool, error) {
	count, err := r.client.Exists(ctx, r.key(sessionID)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check session existence: %w", err)
	}
	return count > 0, nil
}

// SessionExists is a convenience method
func (r *RedisStorage) SessionExists(sessionID string) (bool, error) {
	return r.Exists(context.Background(), sessionID)
}

// GetTTL gets remaining TTL for a session
func (r *RedisStorage) GetTTL(ctx context.Context, sessionID string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, r.key(sessionID)).Result()
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
func (r *RedisStorage) Ping(ctx context.Context) error {
	_, err := r.client.Ping(ctx).Result()
	return err
}
