package storage

import (
	"context"
	"fmt"
	"time"
	"eino_llm_poc/pkg"
	"eino_llm_poc/internal/core"
)

// SessionManager handles short-term memory (Redis) operations
type SessionManager interface {
	GetSession(ctx context.Context, customerID string) (*core.Session, error)
	SaveSession(ctx context.Context, session *core.Session) error
	AddMessage(ctx context.Context, customerID string, message pkg.ConversationMessage) error
	DeleteSession(ctx context.Context, customerID string) error
}

// MemorySessionManager is an in-memory implementation for development
type MemorySessionManager struct {
	sessions map[string]*core.Session
	ttl      time.Duration
}

// NewMemorySessionManager creates a new in-memory session manager
func NewMemorySessionManager(ttlSeconds int) SessionManager {
	return &MemorySessionManager{
		sessions: make(map[string]*core.Session),
		ttl:      time.Duration(ttlSeconds) * time.Second,
	}
}

// GetSession retrieves a session by customer ID
func (m *MemorySessionManager) GetSession(ctx context.Context, customerID string) (*core.Session, error) {
	session, exists := m.sessions[customerID]
	if !exists {
		return nil, fmt.Errorf("session not found for customer: %s", customerID)
	}

	// Check if session has expired
	if time.Now().Unix()-session.UpdatedAt > int64(m.ttl.Seconds()) {
		delete(m.sessions, customerID)
		return nil, fmt.Errorf("session expired for customer: %s", customerID)
	}

	return session, nil
}

// SaveSession saves or updates a session
func (m *MemorySessionManager) SaveSession(ctx context.Context, session *core.Session) error {
	if session.CustomerID == "" {
		return fmt.Errorf("customer ID cannot be empty")
	}

	now := time.Now().Unix()
	if session.CreatedAt == 0 {
		session.CreatedAt = now
	}
	session.UpdatedAt = now

	if session.Metadata == nil {
		session.Metadata = make(map[string]any)
	}

	m.sessions[session.CustomerID] = session
	return nil
}

// AddMessage adds a message to the session
func (m *MemorySessionManager) AddMessage(ctx context.Context, customerID string, message pkg.ConversationMessage) error {
	session, err := m.GetSession(ctx, customerID)
	if err != nil {
		// Create new session if not found
		session = &core.Session{
			CustomerID: customerID,
			Messages:   []pkg.ConversationMessage{},
			Metadata:   make(map[string]any),
		}
	}

	session.Messages = append(session.Messages, message)
	return m.SaveSession(ctx, session)
}

// DeleteSession removes a session
func (m *MemorySessionManager) DeleteSession(ctx context.Context, customerID string) error {
	delete(m.sessions, customerID)
	return nil
}

// CreateSessionFromLongterm creates a new session from longterm memory data
func CreateSessionFromLongterm(customerID string, longtermData []pkg.LongtermMemoryEntry) *core.Session {
	session := &core.Session{
		CustomerID: customerID,
		Messages:   []pkg.ConversationMessage{},
		Metadata:   make(map[string]any),
	}

	// Convert longterm entries to conversation messages
	for _, entry := range longtermData {
		// Add user message
		session.Messages = append(session.Messages, pkg.ConversationMessage{
			Role:    "user",
			Content: entry.InputText,
		})

		// Add assistant response (derived from NLU analysis)
		if entry.NLUResponse != nil {
			response := fmt.Sprintf("I understand you want to %s", entry.NLUResponse.PrimaryIntent)
			session.Messages = append(session.Messages, pkg.ConversationMessage{
				Role:    "assistant",
				Content: response,
			})
		}
	}

	// Set metadata
	session.Metadata["longterm_entries"] = len(longtermData)
	session.Metadata["initialized_from"] = "longterm_memory"

	return session
}

// GetOptimizedContext creates optimized context from session for LLM
func GetOptimizedContext(session *core.Session, maxMessages int) string {
	if session == nil || len(session.Messages) == 0 {
		return ""
	}

	messages := session.Messages
	if len(messages) > maxMessages {
		// Keep only the most recent messages
		messages = messages[len(messages)-maxMessages:]
	}

	contextContent := "<conversation_history>\n"
	for i, msg := range messages {
		contextContent += fmt.Sprintf("%d. [%s]: %s\n", i+1, msg.Role, msg.Content)
	}
	contextContent += "</conversation_history>"

	return contextContent
}

// SessionStats provides statistics about a session
type SessionStats struct {
	CustomerID     string `json:"customer_id"`
	MessageCount   int    `json:"message_count"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	DurationMinutes int64 `json:"duration_minutes"`
}

// GetSessionStats returns statistics for a session
func GetSessionStats(session *core.Session) SessionStats {
	stats := SessionStats{
		CustomerID:   session.CustomerID,
		MessageCount: len(session.Messages),
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
	}

	if session.CreatedAt > 0 && session.UpdatedAt > 0 {
		stats.DurationMinutes = (session.UpdatedAt - session.CreatedAt) / 60
	}

	return stats
}

// ValidateSession checks if a session is valid
func ValidateSession(session *core.Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if session.CustomerID == "" {
		return fmt.Errorf("customer ID cannot be empty")
	}

	// Check message format
	for i, msg := range session.Messages {
		if msg.Content == "" {
			return fmt.Errorf("message %d has empty content", i)
		}

		validRoles := map[string]bool{"user": true, "assistant": true, "system": true}
		if !validRoles[msg.Role] {
			return fmt.Errorf("message %d has invalid role: %s", i, msg.Role)
		}
	}

	return nil
}