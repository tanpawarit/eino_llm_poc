package core

import (
	"context"
	"eino_llm_poc/pkg"
)

// Node represents a single processing unit in the graph flow
type Node interface {
	Execute(ctx context.Context, input NodeInput) (NodeOutput, error)
	GetName() string
	GetType() NodeType
}

// NodeType defines the different types of nodes in the graph
type NodeType string

const (
	NodeTypeNLU      NodeType = "nlu"
	NodeTypeRouting  NodeType = "routing"
	NodeTypeResponse NodeType = "response"
	NodeTypeTools    NodeType = "tools"
	NodeTypeStorage  NodeType = "storage"
)

// NodeInput contains the input data for a node
type NodeInput struct {
	UserMessage         string                      `json:"user_message"`
	CustomerID          string                      `json:"customer_id"`
	ConversationContext []pkg.ConversationMessage   `json:"conversation_context"`
	SessionData         map[string]any              `json:"session_data"`
	NLUResult           *pkg.NLUResponse            `json:"nlu_result,omitempty"`
	RoutingContext      *RoutingContext             `json:"routing_context,omitempty"`
	ToolsRequired       []string                    `json:"tools_required,omitempty"`
	Metadata            map[string]any              `json:"metadata"`
}

// NodeOutput contains the output data from a node
type NodeOutput struct {
	Data     map[string]any `json:"data"`
	NextNode string         `json:"next_node,omitempty"`
	Error    error          `json:"error,omitempty"`
	Complete bool           `json:"complete"`
}

// RoutingContext contains context information for routing decisions
type RoutingContext struct {
	OptimizedContext string     `json:"optimized_context"`
	SessionMemory    *Session   `json:"session_memory"`
	LongtermMemory   []any      `json:"longterm_memory"`
	ContextSize      int        `json:"context_size"`
}

// Session represents short-term memory (Redis)
type Session struct {
	CustomerID   string                    `json:"customer_id"`
	Messages     []pkg.ConversationMessage `json:"messages"`
	CreatedAt    int64                     `json:"created_at"`
	UpdatedAt    int64                     `json:"updated_at"`
	Metadata     map[string]any            `json:"metadata"`
}

// GraphProcessor orchestrates the execution of nodes in a graph flow
type GraphProcessor interface {
	Execute(ctx context.Context, input ProcessorInput) (*ProcessorOutput, error)
	AddNode(node Node) error
	GetNode(name string) (Node, error)
	SetFlow(flow GraphFlow) error
}

// ProcessorInput is the main input for the graph processor
type ProcessorInput struct {
	UserMessage string `json:"user_message"`
	CustomerID  string `json:"customer_id"`
}

// ProcessorOutput is the main output from the graph processor
type ProcessorOutput struct {
	Response        string         `json:"response"`
	NLUAnalysis     *pkg.NLUResponse `json:"nlu_analysis,omitempty"`
	SessionUpdated  bool           `json:"session_updated"`
	LongtermSaved   bool           `json:"longterm_saved"`
	ToolsExecuted   []string       `json:"tools_executed,omitempty"`
	ProcessingTime  int64          `json:"processing_time_ms"`
	Metadata        map[string]any `json:"metadata"`
}

// GraphFlow defines the execution flow between nodes
type GraphFlow struct {
	StartNode string                    `json:"start_node"`
	Edges     map[string][]GraphEdge    `json:"edges"` // node_name -> possible next nodes
}

// GraphEdge represents a connection between two nodes with conditions
type GraphEdge struct {
	To        string            `json:"to"`
	Condition map[string]any    `json:"condition,omitempty"`
	Priority  int               `json:"priority"`
}

// Config holds all configuration for the graph processor
type Config struct {
	NLU      pkg.NLUConfig     `json:"nlu"`
	Redis    RedisConfig       `json:"redis"`
	Storage  StorageConfig     `json:"storage"`
	Graph    GraphConfig       `json:"graph"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL string `json:"url"`
	TTL int    `json:"ttl"` // session TTL in seconds
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	LongtermDir string `json:"longterm_dir"`
}

// GraphConfig holds graph flow configuration
type GraphConfig struct {
	DefaultFlow    GraphFlow `json:"default_flow"`
	EnableParallel bool      `json:"enable_parallel"`
	MaxConcurrency int       `json:"max_concurrency"`
}