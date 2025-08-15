package model

import "time"

// ----------------------------------------------------
// ================ Config ================
// NLUConfig holds configuration for the NLU system
type NLUConfig struct {
	Model               string  `yaml:"model"`
	MaxTokens           int     `yaml:"max_tokens"`
	Temperature         float64 `yaml:"temperature"`
	ImportanceThreshold float64 `yaml:"importance_threshold"`
	DefaultIntent       string  `yaml:"default_intent"`
	AdditionalIntent    string  `yaml:"additional_intent"`
	DefaultEntity       string  `yaml:"default_entity"`
	AdditionalEntity    string  `yaml:"additional_entity"`
}

// ----------------------------------------------------
// ================ Request ================
type NLURequest struct {
	Text               string   `json:"text"`
	CustomerID         string   `json:"customer_id,omitempty"`
	DefaultIntents     []string `json:"default_intents,omitempty"`
	AdditionalIntents  []string `json:"additional_intents,omitempty"`
	DefaultEntities    []string `json:"default_entities,omitempty"`
	AdditionalEntities []string `json:"additional_entities,omitempty"`
}

// ----------------------------------------------------
// ================ Response ================
// Intent represents a detected user intent
type Intent struct {
	Name       string         `json:"name"`
	Confidence float64        `json:"confidence"`
	Priority   float64        `json:"priority"`
	Metadata   map[string]any `json:"metadata"`
}

// Entity represents an extracted entity from user input
type Entity struct {
	Type       string         `json:"type"`
	Value      string         `json:"value"`
	Confidence float64        `json:"confidence"`
	Position   []int          `json:"position,omitempty"`
	Metadata   map[string]any `json:"metadata"`
}

// Language represents detected language information
type Language struct {
	Code       string         `json:"code"` // ISO 3166 Alpha-3 code
	Confidence float64        `json:"confidence"`
	IsPrimary  bool           `json:"is_primary"` // 1 for primary, 0 for contained
	Metadata   map[string]any `json:"metadata"`
}

// Sentiment represents detected sentiment analysis
type Sentiment struct {
	Label      string         `json:"label"` // positive, negative, neutral
	Confidence float64        `json:"confidence"`
	Metadata   map[string]any `json:"metadata"`
}

// NLUResponse contains structured output from NLU processing
type NLUResponse struct {
	Intents         []Intent       `json:"intents"`
	Entities        []Entity       `json:"entities"`
	Languages       []Language     `json:"languages"`
	Sentiment       Sentiment      `json:"sentiment"`
	ImportanceScore float64        `json:"importance_score"`
	PrimaryIntent   string         `json:"primary_intent"`
	PrimaryLanguage string         `json:"primary_language"`
	Metadata        map[string]any `json:"metadata"`
	ParsingMetadata map[string]any `json:"parsing_metadata"`
	Timestamp       time.Time      `json:"timestamp"`
}
