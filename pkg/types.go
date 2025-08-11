package pkg

import (
	"time"
)

// NLU Core Types for Natural Language Understanding

// Intent represents a detected user intent
type Intent struct {
	Name       string                 `json:"name"`
	Confidence float64               `json:"confidence"`
	Priority   float64               `json:"priority"`
	Metadata   map[string]any `json:"metadata"`
}

// Entity represents an extracted entity from user input
type Entity struct {
	Type       string                 `json:"type"`
	Value      string                 `json:"value"`
	Confidence float64               `json:"confidence"`
	Position   []int                  `json:"position,omitempty"`
	Metadata   map[string]any `json:"metadata"`
}

// Language represents detected language information
type Language struct {
	Code       string                 `json:"code"`        // ISO 3166 Alpha-3 code
	Confidence float64               `json:"confidence"`
	IsPrimary  bool                   `json:"is_primary"` // 1 for primary, 0 for contained
	Metadata   map[string]any `json:"metadata"`
}

// Sentiment represents detected sentiment analysis
type Sentiment struct {
	Label      string                 `json:"label"`      // positive, negative, neutral
	Confidence float64               `json:"confidence"`
	Metadata   map[string]any `json:"metadata"`
}

// ConversationMessage represents a message in conversation history
type ConversationMessage struct {
	Role    string `json:"role"`    // user, assistant, system
	Content string `json:"content"`
}

// NLURequest contains input data for NLU processing
type NLURequest struct {
	Text               string                 `json:"text"`
	DefaultIntents     []string               `json:"default_intents,omitempty"`
	AdditionalIntents  []string               `json:"additional_intents,omitempty"`
	DefaultEntities    []string               `json:"default_entities,omitempty"`
	AdditionalEntities []string               `json:"additional_entities,omitempty"`
	ConversationContext []ConversationMessage `json:"conversation_context,omitempty"`
}

// NLUResponse contains structured output from NLU processing
type NLUResponse struct {
	Intents        []Intent            `json:"intents"`
	Entities       []Entity            `json:"entities"`
	Languages      []Language          `json:"languages"`
	Sentiment      Sentiment           `json:"sentiment"`
	ImportanceScore float64            `json:"importance_score"`
	PrimaryIntent   string             `json:"primary_intent"`
	PrimaryLanguage string             `json:"primary_language"`
	Metadata        map[string]any     `json:"metadata"`
	ParsingMetadata map[string]any     `json:"parsing_metadata"`
	Timestamp       time.Time          `json:"timestamp"`
}

// NLUConfig holds configuration for the NLU system
type NLUConfig struct {
	Model               string  `json:"model"`
	APIKey              string  `json:"api_key"`
	BaseURL             string  `json:"base_url"`
	MaxTokens           int     `json:"max_tokens"`
	Temperature         float64 `json:"temperature"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	ImportanceThreshold float64 `json:"importance_threshold"`
	TupleDelimiter      string  `json:"tuple_delimiter"`
	RecordDelimiter     string  `json:"record_delimiter"`
	CompletionDelimiter string  `json:"completion_delimiter"`
	DefaultIntent       string  `json:"default_intent"`
	AdditionalIntent    string  `json:"additional_intent"`
	DefaultEntity       string  `json:"default_entity"`
	AdditionalEntity    string  `json:"additional_entity"`
}

// IntentConfig defines available intents with their priorities
type IntentConfig struct {
	Default    map[string]float64 `json:"default"`    // intent_name -> priority
	Additional map[string]float64 `json:"additional"` // intent_name -> priority
}

// EntityConfig defines available entity types
type EntityConfig struct {
	Default    []string `json:"default"`
	Additional []string `json:"additional"`
}

// NLUSystem interface defines the main NLU operations
type NLUSystem interface {
	Process(request NLURequest) (*NLUResponse, error)
	SetConfig(config NLUConfig)
	GetConfig() NLUConfig
}

// ParsedTuple represents a single parsed tuple from the model output
type ParsedTuple struct {
	Type       string                 `json:"type"`       // intent, entity, language, sentiment
	Name       string                 `json:"name"`       // identifier/type name
	Value      string                 `json:"value"`      // actual value for entities
	Confidence float64               `json:"confidence"`
	Priority   float64               `json:"priority,omitempty"`
	IsPrimary  bool                   `json:"is_primary,omitempty"`
	Metadata   map[string]any `json:"metadata"`
}