package model

import "time"

// ----------------------------------------------------
// ================ Config ================
// LogConfig holds configuration for zerolog
type LogConfig struct {
	Level      string `envconfig:"LOG_LEVEL" default:"info"`          // debug, info, warn, error, fatal, panic
	Format     string `envconfig:"LOG_FORMAT" default:"json"`         // json, console
	TimeFormat string `envconfig:"LOG_TIME_FORMAT" default:"rfc3339"` // rfc3339, unix, iso8601
	Output     string `envconfig:"LOG_OUTPUT" default:"stdout"`       // stdout, stderr, file
	FilePath   string `envconfig:"LOG_FILE_PATH" default:"logs/app.log"`
}

type ConversationConfig struct {
	TTL int `envconfig:"CONVERSATION_TTL" default:"15"`
	NLU struct {
		MaxTurns int `envconfig:"CONVERSATION_NLU_MAX_TURNS" default:"5"`
	}
	Response struct {
		MaxTurns int `envconfig:"CONVERSATION_RESPONSE_MAX_TURNS" default:"10"`
	}
}

// NLUConfig holds configuration for the NLU system
type NLUConfig struct {
	Model               string  `envconfig:"NLU_MODEL" default:"openai/gpt-3.5-turbo"`
	MaxTokens           int     `envconfig:"NLU_MAX_TOKENS" default:"2000"`
	Temperature         float32 `envconfig:"NLU_TEMPERATURE" default:"0.1"`
	ImportanceThreshold float64 `envconfig:"NLU_IMPORTANCE_THRESHOLD" default:"0.6"`
	DefaultIntent       string  `envconfig:"NLU_DEFAULT_INTENT" default:"greet:0.1, purchase_intent:0.8, inquiry_intent:0.7, support_intent:0.6, complain_intent:0.6"`
	AdditionalIntent    string  `envconfig:"NLU_ADDITIONAL_INTENT" default:"complaint:0.5, cancel_order:0.4, ask_price:0.6, compare_product:0.5, delivery_issue:0.7"`
	DefaultEntity       string  `envconfig:"NLU_DEFAULT_ENTITY" default:"product, quantity, brand, price"`
	AdditionalEntity    string  `envconfig:"NLU_ADDITIONAL_ENTITY" default:"color, model, spec, budget, warranty, delivery"`
}

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
