package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

// ConversationHistory holds conversation messages
type ConversationHistory struct {
	Messages []*schema.Message `json:"messages"`
}

type ConversationManager struct {
	redis  *redis.Client
	config ConversationConfig
}

type ConversationConfig struct {
	TTL time.Duration
	NLU struct{ MaxTurns int }
}

func NewConversationManager(ctx context.Context, config ConversationConfig) (*ConversationManager, error) {
	// Get Redis URL from environment variable
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

	return &ConversationManager{
		redis:  client,
		config: config,
	}, nil
}

// ProcessNLUMessage
func (m *ConversationManager) ProcessNLUMessage(ctx context.Context, customerID, query string) (string, error) {
	// Use built-in NLU strategy with config
	strategy := &NLUStrategy{maxTurns: m.config.NLU.MaxTurns}
	return m.processMessage(ctx, customerID, query, strategy)
}

// SaveResponse saves assistant response to conversation
func (m *ConversationManager) SaveResponse(ctx context.Context, customerID, response string) error {
	return m.addMessage(ctx, customerID, schema.AssistantMessage(response, nil))
}

// GetHistory returns conversation history
func (m *ConversationManager) GetHistory(ctx context.Context, customerID string) ([]*schema.Message, error) {
	history, err := m.loadHistory(ctx, customerID)
	if err != nil {
		return nil, err
	}
	return history.Messages, nil
}

// ====================== Private Methods ======================

func (m *ConversationManager) processMessage(ctx context.Context, customerID, query string, strategy ContextStrategy) (string, error) {
	// 1. Save user message
	userMsg := schema.UserMessage(query)
	if err := m.addMessage(ctx, customerID, userMsg); err != nil {
		return "", err
	}

	// 2. Load history and build context
	history, err := m.loadHistory(ctx, customerID)
	if err != nil {
		return "", err
	}

	conversationContext := strategy.BuildContext(history.Messages)

	// 3. Build complete context with current message
	var fullContext strings.Builder
	fullContext.WriteString(conversationContext)
	fullContext.WriteString("\n<current_message_to_analyze>\n")
	fullContext.WriteString("UserMessage(" + query + ")\n")
	fullContext.WriteString("</current_message_to_analyze>")

	return fullContext.String(), nil
}

func (m *ConversationManager) addMessage(ctx context.Context, customerID string, message *schema.Message) error {
	history, err := m.loadHistory(ctx, customerID)
	if err != nil {
		return err
	}

	history.Messages = append(history.Messages, message)
	return m.saveHistory(ctx, customerID, history)
}

func (m *ConversationManager) loadHistory(ctx context.Context, customerID string) (*ConversationHistory, error) {
	key := "conversation:" + customerID
	data, err := m.redis.Get(ctx, key).Result()
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
	m.redis.Expire(ctx, key, m.config.TTL)
	return &history, nil
}

func (m *ConversationManager) saveHistory(ctx context.Context, customerID string, history *ConversationHistory) error {
	key := "conversation:" + customerID
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}

	return m.redis.Set(ctx, key, data, m.config.TTL).Err()
}

// ====================== Built-in Strategies ======================
// ContextStrategy interface for different context building strategies
type ContextStrategy interface {
	BuildContext(messages []*schema.Message) string
	GetMaxTurns() int
}

// NLUStrategy - Built-in NLU strategy
type NLUStrategy struct {
	maxTurns int
}

func (s *NLUStrategy) GetMaxTurns() int {
	return s.maxTurns
}

func (s *NLUStrategy) BuildContext(messages []*schema.Message) string {
	recentMessages := trimTail(messages, s.maxTurns)

	var contextBuilder strings.Builder
	contextBuilder.WriteString("<conversation_context>\n")

	for _, msg := range recentMessages {
		switch msg.Role {
		case schema.User:
			contextBuilder.WriteString("UserMessage(" + msg.Content + ")\n")
		case schema.Assistant:
			contextBuilder.WriteString("AssistantMessage(" + msg.Content + ")\n")
		}
	}

	contextBuilder.WriteString("</conversation_context>")
	return contextBuilder.String()
}

// ====================== Helper function ======================
func trimTail(messages []*schema.Message, maxTurns int) []*schema.Message {
	if len(messages) <= maxTurns {
		return messages
	}
	return messages[len(messages)-maxTurns:]
}
