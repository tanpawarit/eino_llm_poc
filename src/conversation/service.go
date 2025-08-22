package conversation

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ProcessMessage handles adding user message and building context for NLU
func (s *Service) ProcessMessage(ctx context.Context, customerID, query string, strategy ContextStrategy) (string, error) {
	// Add user message to conversation history
	userMsg := schema.UserMessage(query)
	if err := s.repo.AddMessage(ctx, customerID, userMsg); err != nil {
		return "", err
	}

	// Get context for the specific model using strategy
	conversationContext, err := s.repo.GetContextForModel(ctx, customerID, strategy)
	if err != nil {
		return "", err
	}

	// Build complete NLU input with current message analysis
	var fullContext strings.Builder
	fullContext.WriteString(conversationContext)
	fullContext.WriteString("\n<current_message_to_analyze>\n")
	fullContext.WriteString("UserMessage(" + query + ")\n")
	fullContext.WriteString("</current_message_to_analyze>")

	return fullContext.String(), nil
}

// SaveResponse saves the assistant's response to conversation history
func (s *Service) SaveResponse(ctx context.Context, customerID, response string) error {
	assistantMsg := schema.AssistantMessage(response, nil)
	return s.repo.AddMessage(ctx, customerID, assistantMsg)
}

// GetHistory returns the full conversation history
func (s *Service) GetHistory(ctx context.Context, customerID string) (*ConversationHistory, error) {
	return s.repo.Load(ctx, customerID)
}
