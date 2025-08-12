package nodes

import (
	"context"
	"eino_llm_poc/internal/core"
	"eino_llm_poc/internal/storage"
	"eino_llm_poc/pkg"
	"fmt"
	"log"
)

// RoutingNode handles session management and context optimization
type RoutingNode struct {
	sessionMgr  storage.SessionManager
	longtermMgr storage.LongtermManager
	config      core.Config
}

// NewRoutingNode creates a new routing node
func NewRoutingNode(sessionMgr storage.SessionManager, longtermMgr storage.LongtermManager, config core.Config) *RoutingNode {
	return &RoutingNode{
		sessionMgr:  sessionMgr,
		longtermMgr: longtermMgr,
		config:      config,
	}
}

// Execute processes session management and context routing
func (r *RoutingNode) Execute(ctx context.Context, input core.NodeInput) (core.NodeOutput, error) {
	log.Printf("🔀 Processing routing for customer: %s", input.CustomerID)

	// Load or create session
	session, err := r.loadOrCreateSession(ctx, input)
	if err != nil {
		return core.NodeOutput{Error: err}, nil
	}

	// Add current user message to session
	userMessage := pkg.ConversationMessage{
		Role:    "user",
		Content: input.UserMessage,
	}
	if err := r.sessionMgr.AddMessage(ctx, input.CustomerID, userMessage); err != nil {
		log.Printf("Warning: Failed to add message to session: %v", err)
	}

	// Create optimized context for LLM
	optimizedContext := storage.GetOptimizedContext(session, 10) // Last 10 messages

	// Create routing context
	routingContext := &core.RoutingContext{
		OptimizedContext: optimizedContext,
		SessionMemory:    session,
		ContextSize:      len(optimizedContext),
	}

	// Prepare output
	output := core.NodeOutput{
		Data: map[string]any{
			"routing_context":      routingContext,
			"session_updated":      true,
			"conversation_context": session.Messages,
			"optimized_context":    optimizedContext,
		},
		NextNode: "response",
		Complete: false,
	}

	log.Printf("✅ Routing completed: context_size=%d, messages=%d", len(optimizedContext), len(session.Messages))

	return output, nil
}

// GetName returns the node name
func (r *RoutingNode) GetName() string {
	return "routing"
}

// GetType returns the node type
func (r *RoutingNode) GetType() core.NodeType {
	return core.NodeTypeRouting
}

// loadOrCreateSession loads existing session or creates new one from longterm memory
func (r *RoutingNode) loadOrCreateSession(ctx context.Context, input core.NodeInput) (*core.Session, error) {
	// Try to load existing session
	session, err := r.sessionMgr.GetSession(ctx, input.CustomerID)
	if err == nil {
		log.Printf("📱 Loaded existing session for customer: %s", input.CustomerID)
		return session, nil
	}

	log.Printf("📱 No existing session found, creating new one for customer: %s", input.CustomerID)

	// Try to load from longterm memory
	longtermEntries, err := r.longtermMgr.LoadMemory(input.CustomerID)
	if err != nil {
		log.Printf("Warning: Failed to load longterm memory: %v", err)
		longtermEntries = []pkg.LongtermMemoryEntry{}
	}

	// Create new session (possibly from longterm memory)
	if len(longtermEntries) > 0 {
		log.Printf("💾 Creating session from %d longterm memory entries", len(longtermEntries))
		session = storage.CreateSessionFromLongterm(input.CustomerID, longtermEntries)
	} else {
		log.Printf("🆕 Creating fresh session")
		session = &core.Session{
			CustomerID: input.CustomerID,
			Messages:   []pkg.ConversationMessage{},
			Metadata:   make(map[string]any),
		}
	}

	// Save the new session
	if err := r.sessionMgr.SaveSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save new session: %v", err)
	}

	return session, nil
}
