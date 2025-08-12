package nodes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"encoding/json"

	"eino_llm_poc/pkg"
	"eino_llm_poc/internal/core"
	"eino_llm_poc/internal/storage"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// NLUNode handles Natural Language Understanding analysis
type NLUNode struct {
	config         pkg.NLUConfig
	model          openai.ChatModel
	template       prompt.ChatTemplate
	chain          compose.Runnable[map[string]any, *schema.Message]
	longtermMgr    storage.LongtermManager
}

// NewNLUNode creates a new NLU processing node
func NewNLUNode(ctx context.Context, config pkg.NLUConfig, longtermMgr storage.LongtermManager) (*NLUNode, error) {
	maxTokens := config.MaxTokens
	temperature := float32(config.Temperature)

	modelConfig := &openai.ChatModelConfig{
		APIKey:      config.APIKey,
		BaseURL:     config.BaseURL,
		Model:       config.Model,
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
	}

	model, err := openai.NewChatModel(ctx, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating chat model: %v", err)
	}

	// Create the NLU template with delimiters from config
	template := createNLUTemplate(config)

	// Create the Eino chain: Template → ChatModel
	chain, err := compose.NewChain[map[string]any, *schema.Message]().
		AppendChatTemplate(template).
		AppendChatModel(model).
		Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating Eino chain: %v", err)
	}

	return &NLUNode{
		config:      config,
		model:       *model,
		template:    template,
		chain:       chain,
		longtermMgr: longtermMgr,
	}, nil
}

// Execute processes NLU analysis for the input
func (n *NLUNode) Execute(ctx context.Context, input core.NodeInput) (core.NodeOutput, error) {
	// Validate input
	if input.UserMessage == "" {
		return core.NodeOutput{Error: errors.New("user message cannot be empty")}, nil
	}

	// Create NLU request
	request := pkg.NLURequest{
		Text:                input.UserMessage,
		CustomerID:          input.CustomerID,
		ConversationContext: input.ConversationContext,
		DefaultIntents:      strings.Split(n.config.DefaultIntent, ", "),
		AdditionalIntents:   strings.Split(n.config.AdditionalIntent, ", "),
		DefaultEntities:     strings.Split(n.config.DefaultEntity, ", "),
		AdditionalEntities:  strings.Split(n.config.AdditionalEntity, ", "),
	}

	// Validate request
	if err := n.validateNLURequest(request); err != nil {
		return core.NodeOutput{Error: fmt.Errorf("invalid request: %v", err)}, nil
	}

	// Process NLU analysis
	response, err := n.processNLU(ctx, request)
	if err != nil {
		return core.NodeOutput{Error: err}, nil
	}

	// Check if should save to longterm memory
	if storage.ShouldSaveToLongterm(response, n.config.ImportanceThreshold) {
		entry := storage.CreateLongtermEntry(request, response)
		if err := n.longtermMgr.SaveEntry(entry); err != nil {
			log.Printf("Warning: Failed to save to longterm memory: %v", err)
		}
	}

	// Prepare output
	output := core.NodeOutput{
		Data: map[string]any{
			"nlu_response": response,
			"importance_score": response.ImportanceScore,
			"longterm_saved": storage.ShouldSaveToLongterm(response, n.config.ImportanceThreshold),
		},
		NextNode: "routing",
		Complete: false,
	}

	return output, nil
}

// GetName returns the node name
func (n *NLUNode) GetName() string {
	return "nlu"
}

// GetType returns the node type
func (n *NLUNode) GetType() core.NodeType {
	return core.NodeTypeNLU
}

// processNLU performs the actual NLU analysis
func (n *NLUNode) processNLU(ctx context.Context, request pkg.NLURequest) (*pkg.NLUResponse, error) {
	log.Printf("🧠 Analyzing message with NLU, message_length=%d", len(request.Text))
	analysisStart := time.Now()

	// Format intent and entity lists for the prompt
	defaultIntents := strings.Join(request.DefaultIntents, ", ")
	additionalIntents := strings.Join(request.AdditionalIntents, ", ")
	defaultEntities := strings.Join(request.DefaultEntities, ", ")
	additionalEntities := strings.Join(request.AdditionalEntities, ", ")

	// Create template variables for Eino
	templateVars := map[string]any{
		"input_text":        request.Text,
		"default_intent":    defaultIntents,
		"additional_intent": additionalIntents,
		"default_entity":    defaultEntities,
		"additional_entity": additionalEntities,
	}

	// Handle conversation context if provided
	if len(request.ConversationContext) > 0 {
		contextContent := "<conversation_context>\n"
		for i, msg := range request.ConversationContext {
			contextContent += fmt.Sprintf("%d. [%s]: %s\n", i+1, strings.ToUpper(msg.Role), msg.Content)
		}
		contextContent += "</conversation_context>\n\n"
		contextContent += fmt.Sprintf("<current_message_to_analyze>\n%s\n</current_message_to_analyze>", request.Text)
		templateVars["input_text"] = contextContent
	}

	// Execute the Eino chain
	out, err := n.chain.Invoke(ctx, templateVars)
	var response *pkg.NLUResponse
	
	if err != nil {
		log.Printf("Warning: LLM API failed, using fallback NLU: %v", err)
		// Create fallback NLU response
		response = n.createFallbackNLU(request)
	} else {
		// Parse the response from the Eino chain output
		response, err = n.parseResponse(out.Content)
		if err != nil {
			log.Printf("Warning: NLU parsing failed, using fallback: %v", err)
			response = n.createFallbackNLU(request)
		}
	}

	// Calculate importance score and other derived fields
	n.calculateDerivedFields(response)
	response.Timestamp = time.Now()

	// Log analysis results
	analysisTime := time.Since(analysisStart)
	log.Printf("NLU analysis completed: intents_found=%d, entities_found=%d, importance_score=%.3f, analysis_time_ms=%.2f",
		len(response.Intents), len(response.Entities), response.ImportanceScore, float64(analysisTime.Nanoseconds())/1000000)

	if analysisTime > 5*time.Second {
		log.Printf("Warning: Slow NLU analysis detected: analysis_time_ms=%.2f, message_length=%d",
			float64(analysisTime.Nanoseconds())/1000000, len(request.Text))
	}

	// Print analysis summary
	fmt.Printf("\n📊 NLU Analysis Summary:\n")
	fmt.Printf("   Primary Intent: %s\n", response.PrimaryIntent)
	fmt.Printf("   Entities Found: %d\n", len(response.Entities))
	fmt.Printf("   Language: %s\n", response.PrimaryLanguage)
	if response.Sentiment.Label != "" {
		fmt.Printf("   Sentiment: %s\n", response.Sentiment.Label)
	} else {
		fmt.Printf("   Sentiment: None\n")
	}
	fmt.Printf("   Importance Score: %.3f\n", response.ImportanceScore)

	return response, nil
}

// validateNLURequest validates the input request for security and correctness
func (n *NLUNode) validateNLURequest(request pkg.NLURequest) error {
	// Validate text input
	if request.Text == "" {
		return errors.New("input text cannot be empty")
	}

	// Check text length limits (prevent excessive API costs and processing time)
	const maxTextLength = 10000 // 10K characters max
	if len(request.Text) > maxTextLength {
		return fmt.Errorf("input text too long: %d characters (max: %d)", len(request.Text), maxTextLength)
	}

	// Validate UTF-8 encoding
	if !utf8.ValidString(request.Text) {
		return errors.New("input text contains invalid UTF-8 characters")
	}

	// Check for potentially malicious content (basic checks)
	if strings.Contains(request.Text, n.config.TupleDelimiter) {
		return fmt.Errorf("input text contains reserved delimiter: %s", n.config.TupleDelimiter)
	}
	if strings.Contains(request.Text, n.config.RecordDelimiter) {
		return fmt.Errorf("input text contains reserved delimiter: %s", n.config.RecordDelimiter)
	}
	if strings.Contains(request.Text, n.config.CompletionDelimiter) {
		return fmt.Errorf("input text contains reserved delimiter: %s", n.config.CompletionDelimiter)
	}

	// Validate intent and entity lists
	const maxIntentEntities = 50 // Reasonable limit for performance
	if len(request.DefaultIntents) > maxIntentEntities {
		return fmt.Errorf("too many default intents: %d (max: %d)", len(request.DefaultIntents), maxIntentEntities)
	}
	if len(request.AdditionalIntents) > maxIntentEntities {
		return fmt.Errorf("too many additional intents: %d (max: %d)", len(request.AdditionalIntents), maxIntentEntities)
	}
	if len(request.DefaultEntities) > maxIntentEntities {
		return fmt.Errorf("too many default entities: %d (max: %d)", len(request.DefaultEntities), maxIntentEntities)
	}
	if len(request.AdditionalEntities) > maxIntentEntities {
		return fmt.Errorf("too many additional entities: %d (max: %d)", len(request.AdditionalEntities), maxIntentEntities)
	}

	// Validate conversation context
	const maxContextMessages = 20
	if len(request.ConversationContext) > maxContextMessages {
		return fmt.Errorf("too many conversation context messages: %d (max: %d)", len(request.ConversationContext), maxContextMessages)
	}

	// Validate each conversation message
	for i, msg := range request.ConversationContext {
		if msg.Content == "" {
			return fmt.Errorf("conversation context message %d has empty content", i)
		}
		if len(msg.Content) > 1000 { // Reasonable limit for context messages
			return fmt.Errorf("conversation context message %d too long: %d characters (max: 1000)", i, len(msg.Content))
		}
		if !utf8.ValidString(msg.Content) {
			return fmt.Errorf("conversation context message %d contains invalid UTF-8", i)
		}
		// Validate role
		validRoles := map[string]bool{"user": true, "assistant": true, "system": true}
		if !validRoles[msg.Role] {
			return fmt.Errorf("conversation context message %d has invalid role: %s", i, msg.Role)
		}
	}

	return nil
}

// parseResponse parses the tuple-delimited output from the model
func (n *NLUNode) parseResponse(content string) (*pkg.NLUResponse, error) {
	response := &pkg.NLUResponse{
		Intents:         []pkg.Intent{},
		Entities:        []pkg.Entity{},
		Languages:       []pkg.Language{},
		Metadata:        make(map[string]any),
		ParsingMetadata: map[string]any{"status": "success"},
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
			intent := pkg.Intent{
				Name:       tuple.Name,
				Confidence: tuple.Confidence,
				Priority:   tuple.Priority,
				Metadata:   tuple.Metadata,
			}
			response.Intents = append(response.Intents, intent)

		case "entity":
			entity := pkg.Entity{
				Type:       tuple.Name,
				Value:      tuple.Value,
				Confidence: tuple.Confidence,
				Metadata:   tuple.Metadata,
			}
			response.Entities = append(response.Entities, entity)

		case "language":
			language := pkg.Language{
				Code:       tuple.Name,
				Confidence: tuple.Confidence,
				IsPrimary:  tuple.IsPrimary,
				Metadata:   tuple.Metadata,
			}
			response.Languages = append(response.Languages, language)

		case "sentiment":
			response.Sentiment = pkg.Sentiment{
				Label:      tuple.Name,
				Confidence: tuple.Confidence,
				Metadata:   tuple.Metadata,
			}
		}
	}

	return response, nil
}

// parseTuple parses a single tuple from the model output
func (n *NLUNode) parseTuple(tupleStr string) (*pkg.ParsedTuple, error) {
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

	tuple := &pkg.ParsedTuple{
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
func (n *NLUNode) parseMetadata(metadataStr string, metadata *map[string]any) {
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

// calculateDerivedFields calculates importance score and sets derived fields
func (n *NLUNode) calculateDerivedFields(response *pkg.NLUResponse) {
	// Set primary intent (highest confidence)
	if len(response.Intents) > 0 {
		highest := response.Intents[0]
		for _, intent := range response.Intents {
			if intent.Confidence > highest.Confidence {
				highest = intent
			}
		}
		response.PrimaryIntent = highest.Name
	}

	// Set primary language
	if len(response.Languages) > 0 {
		for _, lang := range response.Languages {
			if lang.IsPrimary {
				response.PrimaryLanguage = lang.Code
				break
			}
		}
		// Fallback to first language if no primary found
		if response.PrimaryLanguage == "" {
			response.PrimaryLanguage = response.Languages[0].Code
		}
	}

	// Calculate importance score (matching original logic)
	response.ImportanceScore = n.calculateImportanceScore(response)
}

// calculateImportanceScore calculates the importance score based on NLU results
func (n *NLUNode) calculateImportanceScore(response *pkg.NLUResponse) float64 {
	score := 0.0

	// Intent contribution (40% weight)
	if len(response.Intents) > 0 {
		maxIntentConfidence := 0.0
		for _, intent := range response.Intents {
			if intent.Confidence > maxIntentConfidence {
				maxIntentConfidence = intent.Confidence
			}
		}
		score += maxIntentConfidence * 0.4
	}

	// Entity contribution (30% weight)
	if len(response.Entities) > 0 {
		entityScore := 0.0
		for _, entity := range response.Entities {
			entityScore += entity.Confidence
		}
		entityScore = entityScore / float64(len(response.Entities)) // Average confidence
		score += entityScore * 0.3
	}

	// Sentiment contribution (20% weight)
	if response.Sentiment.Label != "" {
		score += response.Sentiment.Confidence * 0.2
	}

	// Language confidence contribution (10% weight)
	if len(response.Languages) > 0 {
		maxLangConfidence := 0.0
		for _, lang := range response.Languages {
			if lang.Confidence > maxLangConfidence {
				maxLangConfidence = lang.Confidence
			}
		}
		score += maxLangConfidence * 0.1
	}

	return score
}

// createFallbackNLU creates a simple fallback NLU response
func (n *NLUNode) createFallbackNLU(request pkg.NLURequest) *pkg.NLUResponse {
	text := strings.ToLower(request.Text)
	
	// Simple rule-based intent detection
	var primaryIntent string
	var confidence float64 = 0.8
	
	if strings.Contains(text, "สวัสดี") || strings.Contains(text, "hello") || strings.Contains(text, "hi") {
		primaryIntent = "greet"
	} else if strings.Contains(text, "ราคา") || strings.Contains(text, "price") || strings.Contains(text, "เท่าไหร่") {
		primaryIntent = "ask_price"
	} else if strings.Contains(text, "ซื้อ") || strings.Contains(text, "อยากได้") || strings.Contains(text, "สนใจ") {
		primaryIntent = "purchase_intent"
	} else if strings.Contains(text, "เปรียบเทียบ") || strings.Contains(text, "compare") {
		primaryIntent = "compare_product"
	} else if strings.Contains(text, "มี") && (strings.Contains(text, "สินค้า") || strings.Contains(text, "อะไร")) {
		primaryIntent = "inquiry_intent"
	} else if strings.Contains(text, "ขอบคุณ") || strings.Contains(text, "thank") {
		primaryIntent = "greet"
	} else {
		primaryIntent = "inquiry_intent"
	}
	
	// Create basic entities
	var entities []pkg.Entity
	productKeywords := []string{"โน้ตบุ๊ค", "notebook", "macbook", "apple", "lenovo", "คอมพิวเตอร์", "computer", "เมาส์", "mouse"}
	
	for _, keyword := range productKeywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			entities = append(entities, pkg.Entity{
				Type:       "product",
				Value:      keyword,
				Confidence: 0.9,
				Metadata:   map[string]any{"source": "fallback"},
			})
			break
		}
	}
	
	// Create intents slice
	intents := []pkg.Intent{
		{
			Name:       primaryIntent,
			Confidence: confidence,
			Priority:   0.8,
			Metadata:   map[string]any{"source": "fallback"},
		},
	}
	
	// Create languages
	languages := []pkg.Language{
		{
			Code:       "THA",
			Confidence: 0.95,
			IsPrimary:  true,
			Metadata:   map[string]any{"source": "fallback"},
		},
	}
	
	return &pkg.NLUResponse{
		Intents:         intents,
		Entities:        entities,
		Languages:       languages,
		PrimaryIntent:   primaryIntent,
		PrimaryLanguage: "THA",
		Sentiment: pkg.Sentiment{
			Label:      "neutral",
			Confidence: 0.7,
			Metadata:   map[string]any{"source": "fallback"},
		},
		ImportanceScore:    0.7,
		Metadata:          map[string]any{"mode": "fallback"},
		ParsingMetadata:   map[string]any{"status": "fallback"},
	}
}

// Template creation functions (moved from main.go)

// getSystemTemplate returns the system instructions template with placeholders
func getSystemTemplate() string {
	return `You are an expert NLU system. Follow the instructions precisely and return structured output.

-Goal-
Given a user utterance, detect and extract the user's **intent**, **entities**, **language**, and **sentiment**. You are also provided with pre-declared lists of possible default and additional intents and entities.

STRICT RULES:
1. You MUST ONLY extract intents/entities that appear in either default or additional lists
2. DO NOT create new intents or entities not in the provided lists
3. If user input doesn't match any intent, use the closest matching intent from the lists
4. Common greetings (สวัสดี, หวัดดี, hello, hi, good morning) should ALWAYS be classified as "greet"
5. Only extract entities that are EXPLICITLY mentioned in the current message being analyzed

IMPORTANT: Only extract entities that are EXPLICITLY mentioned in the current message being analyzed. Do NOT use entities from conversation context unless they appear in the current message text.

-Steps-
1. Identify the **top 3 intent(s)** that match the message. Consider both default_intent and additional_intent lists with their priority scores.
Format each intent as:
(intent{TD}<intent_name_in_snake_case>{TD}<confidence>{TD}<priority_score>{TD}<metadata>)

2. Identify all **entities** present in the message, using both default_entity and additional_entity types.
STRICT RULE: Only extract entities that are LITERALLY PRESENT in the current message text. Do not infer or assume entities from context.
Format each entity as:
(entity{TD}<entity_type>{TD}<entity_value>{TD}<confidence>{TD}<metadata>)

3. Detect **all languages** present in the message using ISO 3166 Alpha-3 country codes. Return primary language first, followed by additional detected languages. Use 1 for primary language and 0 for contained languages.
Format each language as:
(language{TD}<language_code_iso_alpha3>{TD}<confidence>{TD}<primary_flag>{TD}<metadata>)

4. Detect the **sentiment** expressed in the message.
Format:
(sentiment{TD}<label>{TD}<confidence>{TD}<metadata>)

5. Return the output as a list separated by **{RD}**

6. When complete, return {CD}

######################
-Examples-
######################

Example 1:
text: I want to book a flight to Paris next week.
default_intent: book_flight:0.9, cancel_flight:0.7
additional_intent: greet:0.3, track_flight:0.5
default_entity: location, date
additional_entity: airline, person
######################
Output:
(intent{TD}book_flight{TD}0.95{TD}0.9{TD}{{"extracted_from": "default", "context": "travel_booking"}})
{RD}
(intent{TD}track_flight{TD}0.25{TD}0.5{TD}{{"extracted_from": "additional", "context": "travel_inquiry"}})
{RD}
(intent{TD}cancel_flight{TD}0.15{TD}0.7{TD}{{"extracted_from": "default", "context": "travel_cancellation"}})
{RD}
(entity{TD}location{TD}Paris{TD}0.98{TD}{{"entity_position": [25, 30], "entity_category": "geographic"}})
{RD}
(entity{TD}date{TD}next week{TD}0.94{TD}{{"entity_position": [31, 40], "entity_category": "temporal"}})
{RD}
(language{TD}USA{TD}1.0{TD}1{TD}{{"primary_language": true, "script": "latin", "detected_tokens": 9}})
{RD}
(sentiment{TD}neutral{TD}0.80{TD}{{"polarity": 0.1, "subjectivity": 0.3, "emotion": "neutral"}})
{CD}

######################

Example 2:
text: อยากซื้อรองเท้า Hello!
default_intent: purchase_intent:0.8
additional_intent: ask_product:0.6, cancel_order:0.4
default_entity: product
additional_entity: brand, color
######################
Output:
(intent{TD}purchase_intent{TD}0.95{TD}0.8{TD}{{"extracted_from": "default", "context": "shopping_intent"}})
{RD}
(intent{TD}ask_product{TD}0.30{TD}0.6{TD}{{"extracted_from": "additional", "context": "product_inquiry"}})
{RD}
(intent{TD}cancel_order{TD}0.10{TD}0.4{TD}{{"extracted_from": "additional", "context": "order_cancellation"}})
{RD}
(entity{TD}product{TD}รองเท้า{TD}0.97{TD}{{"entity_position": [6, 12], "entity_category": "product", "language": "thai"}})
{RD}
(language{TD}THA{TD}0.85{TD}1{TD}{{"primary_language": true, "script": "thai", "detected_tokens": 2}})
{RD}
(language{TD}USA{TD}0.95{TD}0{TD}{{"primary_language": false, "script": "latin", "detected_tokens": 1}})
{RD}
(sentiment{TD}positive{TD}0.75{TD}{{"polarity": 0.6, "subjectivity": 0.4, "emotion": "desire"}})
{CD}`
}

// getUserTemplate returns the user data template with placeholders
func getUserTemplate() string {
	return `text: {input_text}
default_intent: {default_intent}
additional_intent: {additional_intent}
default_entity: {default_entity}
additional_entity: {additional_entity}

Output:`
}

// createNLUTemplate creates the Eino ChatTemplate for NLU analysis
func createNLUTemplate(config pkg.NLUConfig) prompt.ChatTemplate {
	// Get system template and replace placeholders efficiently
	systemText := getSystemTemplate()

	// Use strings.Replacer for multiple replacements - more efficient than multiple ReplaceAll calls
	replacer := strings.NewReplacer(
		"{TD}", config.TupleDelimiter,
		"{RD}", config.RecordDelimiter,
		"{CD}", config.CompletionDelimiter,
	)
	systemText = replacer.Replace(systemText)

	// Get user template (no placeholder replacements needed here)
	userText := getUserTemplate()

	// Create messages for the template - SystemMessage for instructions, UserMessage for data
	messages := []schema.MessagesTemplate{
		schema.SystemMessage(systemText),
		schema.UserMessage(userText),
	}

	// Create the ChatTemplate with proper format type
	template := prompt.FromMessages(schema.FString, messages...)

	return template
}