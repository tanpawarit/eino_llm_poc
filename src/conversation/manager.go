package conversation

import (
	"context"
	"strings"

	"eino_llm_poc/src/model"

	"github.com/cloudwego/eino/schema"
)

type MessagesManager struct {
	storage      StorageAdapter
	nluMaxTurns  int
	respMaxTurns int
}

func NewMessagesManager(ctx context.Context, config model.ConversationConfig) (*MessagesManager, error) {
	storage, err := NewRedisStorageAdapter(ctx, config.TTL)
	if err != nil {
		return nil, err
	}
	return &MessagesManager{
		storage:      storage,
		nluMaxTurns:  config.NLU.MaxTurns,
		respMaxTurns: config.Response.MaxTurns,
	}, nil
}

// =========== Function for NLU ===========
func (cm *MessagesManager) ProcessNLUMessage(ctx context.Context, customerID string, query string) (string, error) {
	// 1. Save user message
	userMsg := schema.UserMessage(query)
	if err := cm.storage.AddMessage(ctx, customerID, userMsg); err != nil {
		return "", err
	}

	// 2. Load history and build context
	history, err := cm.storage.LoadHistory(ctx, customerID)
	if err != nil {
		return "", err
	}

	conversationContext := cm.buildNLUContext(history.Messages)

	// 3. Build complete context with current message
	var fullContext strings.Builder
	fullContext.WriteString(conversationContext)
	fullContext.WriteString("\n<current_message_to_analyze>\n")
	fullContext.WriteString("UserMessage(" + query + ")\n")
	fullContext.WriteString("</current_message_to_analyze>")

	return fullContext.String(), nil
}

func (cm *MessagesManager) buildNLUContext(messages []*schema.Message) string {
	recentMessages := trimTail(messages, cm.nluMaxTurns)

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

// =========== Function for Response ===========
func (cm *MessagesManager) SaveResponse(ctx context.Context, customerID string, content string) error {
	assistantMsg := schema.AssistantMessage(content, nil)
	return cm.storage.AddMessage(ctx, customerID, assistantMsg)
}

// ====================== Helper function ======================
func trimTail(messages []*schema.Message, maxTurns int) []*schema.Message {
	if len(messages) <= maxTurns {
		return messages
	}
	return messages[len(messages)-maxTurns:]
}
