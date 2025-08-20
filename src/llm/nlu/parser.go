package nlu

import (
	"eino_llm_poc/src/model"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ParsedTuple represents a single parsed tuple from the model output
type ParsedTuple struct {
	Type       string
	Name       string
	Value      string  // For entities
	Confidence float64
	Priority   float64 // For intents
	IsPrimary  bool    // For languages
	Metadata   map[string]any
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

// NewNLUProcessor creates a new NLU processor with default configuration
func NewNLUProcessor() *NLUProcessor {
	return &NLUProcessor{
		config: &ProcessorConfig{
			RecordDelimiter:     "##",
			TupleDelimiter:      "<||>",
			CompletionDelimiter: "<|COMPLETE|>",
		},
	}
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

	// Use the actual record delimiter from config
	recordDelim := n.config.RecordDelimiter
	if !strings.Contains(content, recordDelim) {
		recordDelim = "##" // fallback to config default
	}

	// Split by actual record delimiter used in the response
	records := strings.Split(content, recordDelim)

	for _, record := range records {
		trimmedRecord := strings.TrimSpace(record)
		if trimmedRecord == "" || trimmedRecord == n.config.CompletionDelimiter || trimmedRecord == "<|COMPLETE|>" {
			continue
		}

		tuple, err := n.parseTuple(trimmedRecord)
		if err != nil {
			log.Printf("Warning: Failed to parse tuple: %s, error: %v", trimmedRecord, err)
			continue
		}

		switch tuple.Type {
		case "intent":
			intent := model.Intent{
				Name:       tuple.Name,
				Confidence: tuple.Confidence,
				Priority:   tuple.Priority,
				Metadata:   tuple.Metadata,
			}
			response.Intents = append(response.Intents, intent)

		case "entity":
			entity := model.Entity{
				Type:       tuple.Name,
				Value:      tuple.Value,
				Confidence: tuple.Confidence,
				Metadata:   tuple.Metadata,
			}
			response.Entities = append(response.Entities, entity)

		case "language":
			language := model.Language{
				Code:       tuple.Name,
				Confidence: tuple.Confidence,
				IsPrimary:  tuple.IsPrimary,
				Metadata:   tuple.Metadata,
			}
			response.Languages = append(response.Languages, language)

		case "sentiment":
			response.Sentiment = model.Sentiment{
				Label:      tuple.Name,
				Confidence: tuple.Confidence,
				Metadata:   tuple.Metadata,
			}
		}
	}

	// Calculate derived fields after parsing
	n.calculateDerivedFields(response)

	return response, nil
}

// parseTuple parses a single tuple from the model output
func (n *NLUProcessor) parseTuple(tupleStr string) (*ParsedTuple, error) {
	// Input validation
	if tupleStr == "" {
		return nil, errors.New("empty tuple string")
	}

	// Check for reasonable length limits
	if len(tupleStr) > 2000 { // Prevent extremely long tuples
		return nil, fmt.Errorf("tuple string too long: %d characters (max: 2000)", len(tupleStr))
	}

	// Validate UTF-8
	if !utf8.ValidString(tupleStr) {
		return nil, errors.New("tuple contains invalid UTF-8 characters")
	}

	// Remove parentheses
	tupleStr = strings.Trim(tupleStr, "()")

	// Use the actual tuple delimiter from config
	tupleDelim := n.config.TupleDelimiter
	if !strings.Contains(tupleStr, tupleDelim) {
		tupleDelim = "<||>" // fallback to config default
	}

	// Split by tuple delimiter
	parts := strings.Split(tupleStr, tupleDelim)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid tuple format: expected at least 4 parts, got %d in: %s", len(parts), tupleStr)
	}

	// Validate each part
	for i, part := range parts {
		if !utf8.ValidString(part) {
			return nil, fmt.Errorf("tuple part %d contains invalid UTF-8: %s", i, part)
		}
	}

	// Create tuple with validated parts
	tupleType := strings.TrimSpace(parts[0])
	tupleName := strings.TrimSpace(parts[1])

	// Validate tuple type
	validTypes := map[string]bool{"intent": true, "entity": true, "language": true, "sentiment": true}
	if !validTypes[tupleType] {
		return nil, fmt.Errorf("invalid tuple type: %s", tupleType)
	}

	// Validate tuple name is not empty
	if tupleName == "" {
		return nil, errors.New("tuple name cannot be empty")
	}

	tuple := &ParsedTuple{
		Type:     tupleType,
		Name:     tupleName,
		Metadata: make(map[string]any),
	}

	// Parse based on tuple type
	switch tuple.Type {
	case "intent":
		if len(parts) >= 5 {
			if conf, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
				tuple.Confidence = conf
			}
			if prio, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
				tuple.Priority = prio
			}
			if len(parts) >= 5 {
				n.parseMetadata(strings.TrimSpace(parts[4]), &tuple.Metadata)
			}
		}

	case "entity":
		if len(parts) >= 4 {
			tuple.Value = strings.TrimSpace(parts[2])
			if conf, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
				tuple.Confidence = conf
			}
			if len(parts) >= 5 {
				n.parseMetadata(strings.TrimSpace(parts[4]), &tuple.Metadata)
			}
		}

	case "language":
		if len(parts) >= 5 {
			if conf, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
				tuple.Confidence = conf
			}
			primaryFlag := strings.TrimSpace(parts[3])
			tuple.IsPrimary = primaryFlag == "1"
			if len(parts) >= 5 {
				n.parseMetadata(strings.TrimSpace(parts[4]), &tuple.Metadata)
			}
		}

	case "sentiment":
		if len(parts) >= 4 {
			if conf, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
				tuple.Confidence = conf
			}
			if len(parts) >= 4 {
				n.parseMetadata(strings.TrimSpace(parts[3]), &tuple.Metadata)
			}
		}
	}

	return tuple, nil
}

// parseMetadata parses JSON metadata string with validation
func (n *NLUProcessor) parseMetadata(metadataStr string, metadata *map[string]any) {
	if metadataStr == "" {
		return
	}

	// Validate input
	if len(metadataStr) > 5000 { // Reasonable limit for metadata
		log.Printf("Warning: Metadata string too long (%d chars), skipping parse", len(metadataStr))
		return
	}

	if !utf8.ValidString(metadataStr) {
		log.Printf("Warning: Metadata contains invalid UTF-8, skipping parse")
		return
	}

	// Basic JSON structure validation
	metadataStr = strings.TrimSpace(metadataStr)
	if !strings.HasPrefix(metadataStr, "{") || !strings.HasSuffix(metadataStr, "}") {
		log.Printf("Warning: Metadata not in JSON object format, skipping parse")
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(metadataStr), &parsed); err != nil {
		log.Printf("Warning: Failed to parse metadata JSON: %v", err)
		return
	}

	// Validate parsed metadata size
	if len(parsed) > 50 { // Reasonable limit for metadata fields
		log.Printf("Warning: Too many metadata fields (%d), truncating to 50", len(parsed))
		// Keep only first 50 fields
		count := 0
		truncated := make(map[string]any)
		for k, v := range parsed {
			if count >= 50 {
				break
			}
			truncated[k] = v
			count++
		}
		*metadata = truncated
		return
	}

	*metadata = parsed
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

	// Calculate ImportanceScore based on intent confidence and priority
	if len(response.Intents) > 0 {
		totalScore := 0.0
		totalWeight := 0.0
		
		for _, intent := range response.Intents {
			// Weight combines confidence and priority
			weight := (intent.Confidence * 0.7) + (intent.Priority * 0.3)
			score := intent.Confidence * weight
			
			totalScore += score
			totalWeight += weight
		}
		
		if totalWeight > 0 {
			response.ImportanceScore = totalScore / totalWeight
		}
	}
}