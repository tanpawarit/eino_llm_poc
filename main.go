package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"eino_llm_poc/pkg"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// YAMLConfig represents the structure of config.yaml
type YAMLConfig struct {
	NLU struct {
		DefaultIntent       string  `yaml:"default_intent"`
		AdditionalIntent    string  `yaml:"additional_intent"`
		DefaultEntity       string  `yaml:"default_entity"`
		AdditionalEntity    string  `yaml:"additional_entity"`
		TupleDelimiter      string  `yaml:"tuple_delimiter"`
		RecordDelimiter     string  `yaml:"record_delimiter"`
		CompletionDelimiter string  `yaml:"completion_delimiter"`
		ImportanceThreshold float64 `yaml:"importance_threshold"`
	} `yaml:"nlu"`
}

// loadConfig loads configuration from config.yaml
func loadConfig(filepath string) (*YAMLConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config YAMLConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v", err)
	}

	return &config, nil
}

// getBaseTemplate returns the base template with placeholders
func getBaseTemplate() string {
	return `
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
{CD}

######################
-Real Data-
######################
text: {input_text}
default_intent: {default_intent}
additional_intent: {additional_intent}
default_entity: {default_entity}
additional_entity: {additional_entity}
######################
Output:
`
}

// createNLUTemplate creates the Eino ChatTemplate for NLU analysis
func createNLUTemplate(config pkg.NLUConfig) prompt.ChatTemplate {
	// Get base template and replace placeholders
	templateText := getBaseTemplate()
	templateText = strings.ReplaceAll(templateText, "{TD}", config.TupleDelimiter)
	templateText = strings.ReplaceAll(templateText, "{RD}", config.RecordDelimiter)
	templateText = strings.ReplaceAll(templateText, "{CD}", config.CompletionDelimiter)

	// Create messages for the template
	// TODO use templateText as a SystemMessage and UserMessage is a last 5 messages in this session(redis shorterm mem)
	messages := []schema.MessagesTemplate{
		schema.SystemMessage("You are an expert NLU system. Follow the instructions precisely and return structured output."),
		schema.UserMessage(templateText),
	}

	// Create the ChatTemplate with proper format type
	template := prompt.FromMessages(schema.FString, messages...)

	return template
}

// NLUProcessor handles NLU operations using Eino components
type NLUProcessor struct {
	config   pkg.NLUConfig
	model    openai.ChatModel
	template prompt.ChatTemplate
	chain    compose.Runnable[map[string]any, *schema.Message]
}

// NewNLUProcessor creates a new NLU processor with Eino chain composition
func NewNLUProcessor(ctx context.Context, config pkg.NLUConfig) (*NLUProcessor, error) {
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

	return &NLUProcessor{
		config:   config,
		model:    *model,
		template: template,
		chain:    chain,
	}, nil
}

// Process performs NLU analysis using the Eino chain
func (n *NLUProcessor) Process(ctx context.Context, request pkg.NLURequest) (*pkg.NLUResponse, error) {
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
	if err != nil {
		return nil, fmt.Errorf("error executing Eino chain: %v", err)
	}

	// Parse the response from the Eino chain output
	response, err := n.parseResponse(out.Content)
	if err != nil {
		log.Printf("Warning: NLU parsing failed, using fallback: %v", err)
		return nil, fmt.Errorf("error parsing NLU response: %v", err)
	}

	// Calculate importance score and other derived fields
	n.calculateDerivedFields(response)

	response.Timestamp = time.Now()

	// Log analysis results (matching Python behavior)
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

// parseResponse parses the tuple-delimited output from the model
func (n *NLUProcessor) parseResponse(content string) (*pkg.NLUResponse, error) {
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
		record = strings.TrimSpace(record)
		if record == "" || record == n.config.CompletionDelimiter || record == "<|COMPLETE|>" {
			continue
		}

		tuple, err := n.parseTuple(record)
		if err != nil {
			log.Printf("Warning: Failed to parse tuple: %s, error: %v", record, err)
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
func (n *NLUProcessor) parseTuple(tupleStr string) (*pkg.ParsedTuple, error) {
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
		return nil, fmt.Errorf("invalid tuple format: %s", tupleStr)
	}

	tuple := &pkg.ParsedTuple{
		Type:     strings.TrimSpace(parts[0]),
		Name:     strings.TrimSpace(parts[1]),
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

// parseMetadata parses JSON metadata string
func (n *NLUProcessor) parseMetadata(metadataStr string, metadata *map[string]any) {
	if metadataStr != "" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(metadataStr), &parsed); err == nil {
			*metadata = parsed
		}
	}
}

// calculateDerivedFields calculates importance score and sets derived fields
func (n *NLUProcessor) calculateDerivedFields(response *pkg.NLUResponse) {
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

	// Calculate importance score (matching Python logic)
	response.ImportanceScore = n.calculateImportanceScore(response)
}

// calculateImportanceScore calculates the importance score based on NLU results
func (n *NLUProcessor) calculateImportanceScore(response *pkg.NLUResponse) float64 {
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

// ShouldSaveToLongterm determines if NLU analysis should be saved to long-term memory
func (n *NLUProcessor) ShouldSaveToLongterm(response *pkg.NLUResponse) bool {
	return response.ImportanceScore >= n.config.ImportanceThreshold
}

// GetBusinessInsights extracts business insights from NLU analysis
func (n *NLUProcessor) GetBusinessInsights(response *pkg.NLUResponse) map[string]any {
	insights := make(map[string]any)

	// Intent insights
	if len(response.Intents) > 0 {
		intentData := make([]map[string]any, len(response.Intents))
		for i, intent := range response.Intents {
			intentData[i] = map[string]any{
				"name":       intent.Name,
				"confidence": intent.Confidence,
			}
		}
		insights["intents"] = intentData
	}

	// Entity insights
	if len(response.Entities) > 0 {
		entityData := make([]map[string]any, len(response.Entities))
		for i, entity := range response.Entities {
			entityData[i] = map[string]any{
				"type":       entity.Type,
				"value":      entity.Value,
				"confidence": entity.Confidence,
			}
		}
		insights["entities"] = entityData
	}

	// Language insights
	if len(response.Languages) > 0 {
		languageData := make([]map[string]any, len(response.Languages))
		for i, lang := range response.Languages {
			languageData[i] = map[string]any{
				"code":       lang.Code,
				"confidence": lang.Confidence,
				"is_primary": lang.IsPrimary,
			}
		}
		insights["languages"] = languageData
	}

	// Sentiment insights
	if response.Sentiment.Label != "" {
		insights["sentiment"] = map[string]any{
			"label":      response.Sentiment.Label,
			"confidence": response.Sentiment.Confidence,
		}
	}

	return insights
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ctx := context.Background()

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENROUTER_API_KEY environment variable")
		return
	}

	// Load configuration from config.yaml
	yamlConfig, err := loadConfig("config.yaml")
	if err != nil {
		fmt.Printf("Error loading config.yaml: %v\n", err)
		return
	}

	// Configure NLU system using values from config.yaml
	config := pkg.NLUConfig{
		Model:               "openai/gpt-3.5-turbo",
		APIKey:              apiKey,
		BaseURL:             "https://openrouter.ai/api/v1",
		MaxTokens:           1500,
		Temperature:         0.1,
		ConfidenceThreshold: 0.5,
		ImportanceThreshold: yamlConfig.NLU.ImportanceThreshold,
		TupleDelimiter:      yamlConfig.NLU.TupleDelimiter,
		RecordDelimiter:     yamlConfig.NLU.RecordDelimiter,
		CompletionDelimiter: yamlConfig.NLU.CompletionDelimiter,
		DefaultIntent:       yamlConfig.NLU.DefaultIntent,
		AdditionalIntent:    yamlConfig.NLU.AdditionalIntent,
		DefaultEntity:       yamlConfig.NLU.DefaultEntity,
		AdditionalEntity:    yamlConfig.NLU.AdditionalEntity,
	}

	// Create NLU processor
	nluProcessor, err := NewNLUProcessor(ctx, config)
	if err != nil {
		fmt.Printf("Error creating NLU processor: %v\n", err)
		return
	}

	// Parse intents and entities from config
	defaultIntents := strings.Split(yamlConfig.NLU.DefaultIntent, ", ")
	additionalIntents := strings.Split(yamlConfig.NLU.AdditionalIntent, ", ")
	defaultEntities := strings.Split(yamlConfig.NLU.DefaultEntity, ", ")
	additionalEntities := strings.Split(yamlConfig.NLU.AdditionalEntity, ", ")

	// Test with sample requests for Thai computer sales domain
	testRequests := []pkg.NLURequest{
		{
			Text:               "สวัสดีครับ อยากซื้อโน้ตบุ๊ครับ",
			DefaultIntents:     defaultIntents,
			AdditionalIntents:  additionalIntents,
			DefaultEntities:    defaultEntities,
			AdditionalEntities: additionalEntities,
		},
	}

	// Process each test request
	for i, request := range testRequests {
		fmt.Printf("\n=== Test %d ===\n", i+1)
		fmt.Printf("Input: %s\n", request.Text)

		response, err := nluProcessor.Process(ctx, request)
		if err != nil {
			fmt.Printf("Error processing request: %v\n", err)
			continue
		}

		// Pretty print the response
		output, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling response: %v\n", err)
			continue
		}

		fmt.Printf("Output: %s\n", output)
	}
}
