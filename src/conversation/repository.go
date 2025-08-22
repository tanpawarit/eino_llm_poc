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

type Repository interface {
	Load(ctx context.Context, customerID string) (*ConversationHistory, error)
	Save(ctx context.Context, customerID string, history *ConversationHistory) error
	AddMessage(ctx context.Context, customerID string, message *schema.Message) error
	GetContextForModel(ctx context.Context, customerID string, strategy ContextStrategy) (string, error)
}

type RedisRepository struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisRepository(ctx context.Context, ttl time.Duration) (*RedisRepository, error) {
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
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisRepository{
		client: client,
		ttl:    ttl,
	}, nil
}

func (r *RedisRepository) Load(ctx context.Context, customerID string) (*ConversationHistory, error) {
	key := "conversation:" + customerID
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return &ConversationHistory{Messages: []*schema.Message{}}, nil
		}
		return nil, err
	}

	var history ConversationHistory
	if err := json.Unmarshal([]byte(data), &history); err != nil {
		return nil, err
	}

	// Refresh TTL
	r.client.Expire(ctx, key, r.ttl)
	return &history, nil
}

func (r *RedisRepository) Save(ctx context.Context, customerID string, history *ConversationHistory) error {
	key := "conversation:" + customerID
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, r.ttl).Err()
}

func (r *RedisRepository) AddMessage(ctx context.Context, customerID string, message *schema.Message) error {
	history, err := r.Load(ctx, customerID)
	if err != nil {
		return err
	}

	history.Messages = append(history.Messages, message)
	return r.Save(ctx, customerID, history)
}

func (r *RedisRepository) GetContextForModel(ctx context.Context, customerID string, strategy ContextStrategy) (string, error) {
	history, err := r.Load(ctx, customerID)
	if err != nil {
		return "", err
	}

	return strategy.BuildContext(history.Messages), nil
}
