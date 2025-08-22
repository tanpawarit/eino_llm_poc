package conversation

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

type ContextStrategy interface {
	BuildContext(messages []*schema.Message) string
	GetMaxTurns() int
}

// ====================== NLU ======================
// NLUContextStrategy - NLU processing last 5 messages
type NLUContextStrategy struct {
	maxTurns int
}

func NewNLUContextStrategy() *NLUContextStrategy {
	return &NLUContextStrategy{maxTurns: 5}
}

func (s *NLUContextStrategy) GetMaxTurns() int {
	return s.maxTurns
}

func (s *NLUContextStrategy) BuildContext(messages []*schema.Message) string {
	// Trim to last N messages
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

// ====================== Response ======================
// ResponseContextStrategy - Response processing last 10 messages

// Helper function
func trimTail(messages []*schema.Message, maxTurns int) []*schema.Message {
	if len(messages) <= maxTurns {
		return messages
	}
	return messages[len(messages)-maxTurns:]
}
