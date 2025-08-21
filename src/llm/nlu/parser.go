package nlu

import (
	"eino_llm_poc/src/model"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Constants for parsing configuration
const (
	DefaultRecordDelimiter     = "##"
	DefaultTupleDelimiter      = "<||>"
	DefaultCompletionDelimiter = "<|COMPLETE|>"
	MaxTupleLength             = 2000
	MaxMetadataLength          = 5000
	MaxMetadataFields          = 50
)

// RawTuple represents a parsed tuple with string parts
type RawTuple struct {
	Type  string
	Parts []string
}

// TupleParser interface for type-specific parsing
type TupleParser interface {
	Parse(raw *RawTuple) error
	AddToResponse(response *model.NLUResponse)
}

// NLUProcessor handles parsing configuration
type NLUProcessor struct {
	config *ProcessorConfig
}

// ProcessorConfig contains parsing configuration
type ProcessorConfig struct {
	RecordDelimiter     string
	TupleDelimiter      string
	CompletionDelimiter string
}

// Specific parser implementations
type IntentParser struct {
	*model.Intent
}

type EntityParser struct {
	*model.Entity
}

type LanguageParser struct {
	*model.Language
}

type SentimentParser struct {
	*model.Sentiment
}

// NewNLUProcessor creates a new NLU processor with default configuration
func NewNLUProcessor() *NLUProcessor {
	return &NLUProcessor{
		config: &ProcessorConfig{
			RecordDelimiter:     DefaultRecordDelimiter,
			TupleDelimiter:      DefaultTupleDelimiter,
			CompletionDelimiter: DefaultCompletionDelimiter,
		},
	}
}

// Validation utility functions
func validateString(s string, maxLength int, fieldName string) error {
	if s == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	if len(s) > maxLength {
		return fmt.Errorf("%s too long: %d characters (max: %d)", fieldName, len(s), maxLength)
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("%s contains invalid UTF-8 characters", fieldName)
	}
	return nil
}

func parseFloat(s string, fieldName string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", fieldName, s)
	}
	return value, nil
}

func parseMetadataJSON(metadataStr string) (map[string]any, error) {
	if metadataStr == "" {
		return make(map[string]any), nil
	}

	if err := validateString(metadataStr, MaxMetadataLength, "metadata"); err != nil {
		return nil, err
	}

	metadataStr = strings.TrimSpace(metadataStr)
	if !strings.HasPrefix(metadataStr, "{") || !strings.HasSuffix(metadataStr, "}") {
		return nil, fmt.Errorf("metadata not in JSON object format")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(metadataStr), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %v", err)
	}

	if len(parsed) > MaxMetadataFields {
		return nil, fmt.Errorf("too many metadata fields: %d (max: %d)", len(parsed), MaxMetadataFields)
	}

	return parsed, nil
}

// Parser implementations
func (p *IntentParser) Parse(raw *RawTuple) error {
	if len(raw.Parts) < 4 {
		return fmt.Errorf("intent tuple requires at least 4 parts, got %d", len(raw.Parts))
	}

	var err error
	p.Intent.Name = strings.TrimSpace(raw.Parts[1])
	if err = validateString(p.Intent.Name, 100, "intent name"); err != nil {
		return err
	}

	if p.Intent.Confidence, err = parseFloat(raw.Parts[2], "confidence"); err != nil {
		return err
	}

	if p.Intent.Priority, err = parseFloat(raw.Parts[3], "priority"); err != nil {
		return err
	}

	if len(raw.Parts) >= 5 {
		if p.Intent.Metadata, err = parseMetadataJSON(raw.Parts[4]); err != nil {
			return err
		}
	} else {
		p.Intent.Metadata = make(map[string]any)
	}

	return nil
}

func (p *IntentParser) AddToResponse(response *model.NLUResponse) {
	response.Intents = append(response.Intents, *p.Intent)
}

func (p *EntityParser) Parse(raw *RawTuple) error {
	if len(raw.Parts) < 4 {
		return fmt.Errorf("entity tuple requires at least 4 parts, got %d", len(raw.Parts))
	}

	var err error
	p.Entity.Type = strings.TrimSpace(raw.Parts[1])
	if err = validateString(p.Entity.Type, 100, "entity type"); err != nil {
		return err
	}

	p.Entity.Value = strings.TrimSpace(raw.Parts[2])
	if err = validateString(p.Entity.Value, 500, "entity value"); err != nil {
		return err
	}

	if p.Entity.Confidence, err = parseFloat(raw.Parts[3], "confidence"); err != nil {
		return err
	}

	if len(raw.Parts) >= 5 {
		if p.Entity.Metadata, err = parseMetadataJSON(raw.Parts[4]); err != nil {
			return err
		}
	} else {
		p.Entity.Metadata = make(map[string]any)
	}

	// Extract position from metadata if available
	if pos, ok := p.Entity.Metadata["entity_position"].([]interface{}); ok && len(pos) == 2 {
		if start, ok1 := pos[0].(float64); ok1 {
			if end, ok2 := pos[1].(float64); ok2 {
				p.Entity.Position = []int{int(start), int(end)}
			}
		}
	}

	return nil
}

func (p *EntityParser) AddToResponse(response *model.NLUResponse) {
	response.Entities = append(response.Entities, *p.Entity)
}

func (p *LanguageParser) Parse(raw *RawTuple) error {
	if len(raw.Parts) < 4 {
		return fmt.Errorf("language tuple requires at least 4 parts, got %d", len(raw.Parts))
	}

	var err error
	p.Language.Code = strings.TrimSpace(raw.Parts[1])
	if err = validateString(p.Language.Code, 10, "language code"); err != nil {
		return err
	}

	if p.Language.Confidence, err = parseFloat(raw.Parts[2], "confidence"); err != nil {
		return err
	}

	// Parse primary language flag: "1" means this is the primary language, "0" or other means secondary
	primaryFlag := strings.TrimSpace(raw.Parts[3])
	p.Language.IsPrimary = primaryFlag == "1"

	if len(raw.Parts) >= 5 {
		if p.Language.Metadata, err = parseMetadataJSON(raw.Parts[4]); err != nil {
			return err
		}
	} else {
		p.Language.Metadata = make(map[string]any)
	}

	return nil
}

func (p *LanguageParser) AddToResponse(response *model.NLUResponse) {
	response.Languages = append(response.Languages, *p.Language)
}

func (p *SentimentParser) Parse(raw *RawTuple) error {
	if len(raw.Parts) < 3 {
		return fmt.Errorf("sentiment tuple requires at least 3 parts, got %d", len(raw.Parts))
	}

	var err error
	p.Sentiment.Label = strings.TrimSpace(raw.Parts[1])
	if err = validateString(p.Sentiment.Label, 50, "sentiment label"); err != nil {
		return err
	}

	if p.Sentiment.Confidence, err = parseFloat(raw.Parts[2], "confidence"); err != nil {
		return err
	}

	if len(raw.Parts) >= 4 {
		if p.Sentiment.Metadata, err = parseMetadataJSON(raw.Parts[3]); err != nil {
			return err
		}
	} else {
		p.Sentiment.Metadata = make(map[string]any)
	}

	return nil
}

func (p *SentimentParser) AddToResponse(response *model.NLUResponse) {
	response.Sentiment = *p.Sentiment
}

// Factory function to create appropriate parser based on tuple type
// Returns the specific parser implementation for the given tuple type
func createParser(tupleType string) (TupleParser, error) {
	switch tupleType {
	case "intent":
		return &IntentParser{Intent: &model.Intent{}}, nil
	case "entity":
		return &EntityParser{Entity: &model.Entity{}}, nil
	case "language":
		return &LanguageParser{Language: &model.Language{}}, nil
	case "sentiment":
		return &SentimentParser{Sentiment: &model.Sentiment{}}, nil
	default:
		return nil, fmt.Errorf("unknown tuple type: %s", tupleType)
	}
}

// parseRawTuple converts a tuple string into a structured RawTuple
// Example input: "(intent<||>book_flight<||>0.85<||>0.9<||>{\"context\":\"travel\"})"
func (n *NLUProcessor) parseRawTuple(tupleStr string) (*RawTuple, error) {
	if err := validateString(tupleStr, MaxTupleLength, "tuple string"); err != nil {
		return nil, err
	}

	// Remove parentheses
	tupleStr = strings.Trim(tupleStr, "()")

	// Split by tuple delimiter
	parts := strings.Split(tupleStr, n.config.TupleDelimiter)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid tuple format: expected at least 3 parts, got %d", len(parts))
	}

	tupleType := strings.TrimSpace(parts[0])
	if tupleType == "" {
		return nil, fmt.Errorf("tuple type cannot be empty")
	}

	return &RawTuple{
		Type:  tupleType,
		Parts: parts,
	}, nil
}

// ParseResponse parses the complete model response into structured NLUResponse
func (n *NLUProcessor) ParseResponse(content string) (*model.NLUResponse, error) {
	response := &model.NLUResponse{
		Intents:         []model.Intent{},
		Entities:        []model.Entity{},
		Languages:       []model.Language{},
		ImportanceScore: 0.0,
		PrimaryIntent:   "",
		PrimaryLanguage: "",
		Metadata:        make(map[string]any),
		ParsingMetadata: map[string]any{"status": "success"},
		Timestamp:       time.Now(),
	}

	// Split by record delimiter
	records := strings.Split(content, n.config.RecordDelimiter)

	for _, record := range records {
		trimmedRecord := strings.TrimSpace(record)
		if n.shouldSkipRecord(trimmedRecord) {
			continue
		}

		if err := n.parseRecord(trimmedRecord, response); err != nil {
			log.Printf("Warning: Failed to parse tuple: %s, error: %v", trimmedRecord, err)
			continue
		}
	}

	// Calculate derived fields after parsing
	n.calculateDerivedFields(response)

	return response, nil
}

// shouldSkipRecord determines if a record should be ignored during parsing
// Skips empty records and completion delimiter markers
func (n *NLUProcessor) shouldSkipRecord(record string) bool {
	return record == "" ||
		record == n.config.CompletionDelimiter ||
		record == DefaultCompletionDelimiter
}

// parseRecord processes a single tuple record and adds it to the response
// Orchestrates: raw parsing -> type-specific parsing -> response integration
func (n *NLUProcessor) parseRecord(record string, response *model.NLUResponse) error {
	rawTuple, err := n.parseRawTuple(record)
	if err != nil {
		return err
	}

	parser, err := createParser(rawTuple.Type)
	if err != nil {
		return err
	}

	if err := parser.Parse(rawTuple); err != nil {
		return err
	}

	parser.AddToResponse(response)
	return nil
}

// calculateDerivedFields calculates PrimaryIntent, PrimaryLanguage, and ImportanceScore
func (n *NLUProcessor) calculateDerivedFields(response *model.NLUResponse) {
	// Calculate PrimaryIntent (highest confidence intent)
	if len(response.Intents) > 0 {
		highestConfidence := 0.0
		primaryIntent := ""

		for _, intent := range response.Intents {
			if intent.Confidence > highestConfidence {
				highestConfidence = intent.Confidence
				primaryIntent = intent.Name
			}
		}
		response.PrimaryIntent = primaryIntent
	}

	// Calculate PrimaryLanguage (language with IsPrimary=true, or highest confidence)
	if len(response.Languages) > 0 {
		for _, lang := range response.Languages {
			if lang.IsPrimary {
				response.PrimaryLanguage = lang.Code
				break
			}
		}

		// Fallback to highest confidence if no primary found
		if response.PrimaryLanguage == "" {
			highestConfidence := 0.0
			primaryLanguage := ""

			for _, lang := range response.Languages {
				if lang.Confidence > highestConfidence {
					highestConfidence = lang.Confidence
					primaryLanguage = lang.Code
				}
			}
			response.PrimaryLanguage = primaryLanguage
		}
	}

	// Calculate ImportanceScore using primary intent weighted score
	if len(response.Intents) > 0 {
		// Find primary intent (highest confidence)
		primary := response.Intents[0]
		for _, intent := range response.Intents {
			if intent.Confidence > primary.Confidence {
				primary = intent
			}
		}

		// Simple weighted formula: 80% confidence + 20% priority
		response.ImportanceScore = (primary.Confidence * 0.6) + (primary.Priority * 0.4)
	}
}

// calculateDerivedFields - Derived Field Calculation Function
//
// Purpose: Computes high-level insights from parsed NLU data using statistical aggregation
// and business logic integration.
//
// Derived Fields & Formulas:
//
// 1. PrimaryIntent
//    Formula: argmax(confidence) over all intents
//    Logic: Selects intent with highest confidence score
//    Use: Primary user intention classification
//
// 2. PrimaryLanguage
//    Formula: Priority-based selection with statistical fallback
//    Logic: if any(language.IsPrimary == true):
//               return language.Code
//           else:
//               return argmax(confidence) over all languages
//    Use: Primary language identification for processing
//
// 3. ImportanceScore
//    Formula: ImportanceScore = (Confidence × 0.6) + (Priority × 0.4)
//    Weights: 60% model confidence + 40% business priority
//    Range: 0.0 to 1.0 (normalized)
//    Use: Business decision-making and routing priority
