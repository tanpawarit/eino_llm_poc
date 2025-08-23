package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

type ConversationHistory struct {
	Messages []*schema.Message `json:"messages"`
}

type StorageAdapter interface {
	LoadHistory(ctx context.Context, customerID string) (*ConversationHistory, error)
	SaveHistory(ctx context.Context, customerID string, history *ConversationHistory, ttl time.Duration) error
	AddMessage(ctx context.Context, customerID string, message *schema.Message) error
	RefreshTTL(ctx context.Context, customerID string, ttl time.Duration) error
	HealthCheck(ctx context.Context) error
}

type RedisStorageAdapter struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStorageAdapter(ctx context.Context, ttlSeconds int) (*RedisStorageAdapter, error) {
	// Get Redis URL from environment variable
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable is required")
	}

	// Parse Redis URL
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorageAdapter{
		client: client,
		ttl:    time.Duration(ttlSeconds) * time.Second,
	}, nil
}

// ======= Implement StorageAdapter interface methods =======
func (r *RedisStorageAdapter) LoadHistory(ctx context.Context, customerID string) (*ConversationHistory, error) {
	key := "conversation:" + customerID
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return &ConversationHistory{Messages: []*schema.Message{}}, nil
		}
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	var history ConversationHistory
	if err := json.Unmarshal([]byte(data), &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history: %w", err)
	}

	return &history, nil
}

func (r *RedisStorageAdapter) SaveHistory(ctx context.Context, customerID string, history *ConversationHistory, ttl time.Duration) error {
	key := "conversation:" + customerID
	data, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisStorageAdapter) AddMessage(ctx context.Context, customerID string, message *schema.Message) error {
	// Load existing history
	history, err := r.LoadHistory(ctx, customerID)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}
	history.Messages = append(history.Messages, message)
	return r.SaveHistory(ctx, customerID, history, r.ttl)
}

func (r *RedisStorageAdapter) RefreshTTL(ctx context.Context, customerID string, ttl time.Duration) error {
	key := "conversation:" + customerID
	return r.client.Expire(ctx, key, ttl).Err()
}

func (r *RedisStorageAdapter) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
