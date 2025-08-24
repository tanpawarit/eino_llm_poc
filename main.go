package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"eino_llm_poc/src"
	"eino_llm_poc/src/conversation"
	"eino_llm_poc/src/llm/nlu"
	"eino_llm_poc/src/logger"
	"eino_llm_poc/src/model"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

type QueryInput struct {
	CustomerID string `json:"customer_id"`
	Query      string `json:"query"`
}

type State struct {
	History    []*schema.Message
	CustomerID string
}

type QueryOutput struct {
	Result model.NLUResponse `json:"result"`
}

const (
	NodeInputConverter = "InputConverter"
	NodeNLUChatModel   = "NLUChatModel"
	NodeParser         = "Parser"
)

func main() {
	if err := godotenv.Load(); err != nil {
		// Will use default values if .env not found
	}

	ctx := context.Background()
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	// Load configuration from environment variables
	config, err := src.LoadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Error loading config")
		return
	}

	// Initialize logger with configuration
	if err := logger.InitLogger(config.LogConfig); err != nil {
		logger.Fatal().Err(err).Msg("Error initializing logger")
		return
	}

	// Setup conversation manager with config
	conversationConfig := config.ConversationConfig

	messagesManager, err := conversation.NewMessagesManager(ctx, conversationConfig)
	if err != nil {
		logger.Error().Err(err).Msg("Error setting up conversation manager")
		return
	}

	// Setup OpenAI model
	chatModelNLU, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     "https://openrouter.ai/api/v1",
		Model:       config.NLUConfig.Model,
		Temperature: &config.NLUConfig.Temperature,
		MaxTokens:   &config.NLUConfig.MaxTokens,
	})
	if err != nil {
		logger.Error().Err(err).Msg("Error creating model")
		return
	}

	g := compose.NewGraph[QueryInput, QueryOutput](
		compose.WithGenLocalState(func(ctx context.Context) *State {
			return &State{
				History: []*schema.Message{},
			}
		}),
	)

	inputConverterNLU := compose.InvokableLambda(func(ctx context.Context, input QueryInput) ([]*schema.Message, error) {
		logger.Info().Str("customer_id", input.CustomerID).Str("query", input.Query).Msg("Processing query")
		conversationCtx, err := messagesManager.ProcessNLUMessage(ctx, input.CustomerID, input.Query)
		if err != nil {
			logger.Error().Str("customer_id", input.CustomerID).Err(err).Msg("Error getting conversation context")
			return nil, err
		}

		logger.Debug().Str("customer_id", input.CustomerID).Msg("Retrieved conversation context from Redis")

		// Generate system prompt
		systemPrompt := nlu.GetSystemTemplateProcessed(&config.NLUConfig)

		// Create messages with customerID in Extra
		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(conversationCtx),
		}

		for _, msg := range messages {
			if msg.Extra == nil {
				msg.Extra = make(map[string]interface{})
			}
			msg.Extra["customerID"] = input.CustomerID
		}
		logger.Debug().Str("customer_id", input.CustomerID).Int("message_count", len(messages)).Msg("Generated input converter messages")

		// Pretty print messages for debugging
		for i, msg := range messages {
			// Print full content if not too long
			content := msg.Content
			logger.Debug().
				Str("customer_id", input.CustomerID).
				Int("message_index", i).
				Str("role", string(msg.Role)).
				Interface("extra", msg.Extra).
				Msg("Message: " + content)
		}
		return messages, nil
	})

	preHandlerNLU := func(ctx context.Context, in []*schema.Message, state *State) ([]*schema.Message, error) {
		// Extract customerID from first message and store in state
		if len(in) > 0 && len(state.CustomerID) == 0 {
			if cid, ok := in[0].Extra["customerID"].(string); ok {
				state.CustomerID = cid
			}
		}
		state.History = append(state.History, in...)
		return state.History, nil
	}

	postHandlerNLU := func(ctx context.Context, out *schema.Message, state *State) (*schema.Message, error) {
		customerID := state.CustomerID
		logger.Debug().Str("customer_id", customerID).Int("response_length", len(out.Content)).Msg("Received model response")

		// Save response to Redis
		if err := messagesManager.SaveResponse(ctx, customerID, out.Content); err != nil {
			logger.Warn().Str("customer_id", customerID).Err(err).Msg("Failed to save response to Redis")
		} else {
			logger.Debug().Str("customer_id", customerID).Msg("Successfully saved response to Redis")
		}
		// Update history
		state.History = append(state.History, out)
		return out, nil
	}

	parserNLU := compose.InvokableLambda(func(ctx context.Context, resp *schema.Message) (QueryOutput, error) {
		result, err := nlu.ParseNLUResponse(resp.Content)
		if err != nil {
			return QueryOutput{}, err
		}
		if result == nil {
			return QueryOutput{}, fmt.Errorf("received nil result from ParseNLUResponse")
		}
		return QueryOutput{
			Result: *result,
		}, nil
	})

	// Add nodes to graph
	g.AddLambdaNode(NodeInputConverter, inputConverterNLU)
	g.AddChatModelNode(NodeNLUChatModel, chatModelNLU,
		compose.WithStatePreHandler(preHandlerNLU),
		compose.WithStatePostHandler(postHandlerNLU),
	)
	g.AddLambdaNode(NodeParser, parserNLU)

	// Wire the nodes
	g.AddEdge(compose.START, NodeInputConverter)
	g.AddEdge(NodeInputConverter, NodeNLUChatModel)
	g.AddEdge(NodeNLUChatModel, NodeParser)
	g.AddEdge(NodeParser, compose.END)

	// Compile graph
	runnable, err := g.Compile(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Error compiling graph")
		return
	}

	// Test with multiple inputs
	inputs := []QueryInput{
		{CustomerID: "1111", Query: "สวัสดี"},
		{CustomerID: "1111", Query: "สนใจซื้อของ"},
		{CustomerID: "1111", Query: "ราคาเท่าไหร่"},
		{CustomerID: "1111", Query: "แพงจัง"},
		{CustomerID: "1111", Query: "ขอบคุณนะครับ"},
	}

	logger.Info().Int("total_inputs", len(inputs)).Msg("Starting batch processing")

	for i, input := range inputs {
		fmt.Printf("\n=== Processing Input %d ===\n", i+1)
		fmt.Printf("Input: CustomerID=%s, Query=%s\n", input.CustomerID, input.Query)

		start := time.Now()
		result, err := runnable.Invoke(ctx, input)
		if err != nil {
			logger.Error().Str("customer_id", input.CustomerID).Err(err).Msg("Error processing input")
			continue
		}

		// Display detailed QueryOutput results
		duration := time.Since(start)

		// Log detailed parsing results summary
		logger.Debug().Str("customer_id", input.CustomerID).
			Int("intents_count", len(result.Result.Intents)).
			Int("entities_count", len(result.Result.Entities)).
			Int("languages_count", len(result.Result.Languages)).
			Str("primary_intent", result.Result.PrimaryIntent).
			Str("primary_language", result.Result.PrimaryLanguage).
			Float64("importance_score", result.Result.ImportanceScore).
			Str("sentiment_label", result.Result.Sentiment.Label).
			Float64("sentiment_confidence", result.Result.Sentiment.Confidence).
			Dur("processing_time", duration).
			Msg("QueryOutput parsing summary")

		// Log detailed intents
		for i, intent := range result.Result.Intents {
			logger.Debug().Str("customer_id", input.CustomerID).
				Int("intent_index", i).
				Str("intent_name", intent.Name).
				Float64("intent_confidence", intent.Confidence).
				Float64("intent_priority", intent.Priority).
				Interface("intent_metadata", intent.Metadata).
				Msg("Intent details")
		}

		// Log detailed entities
		for i, entity := range result.Result.Entities {
			logger.Debug().Str("customer_id", input.CustomerID).
				Int("entity_index", i).
				Str("entity_type", entity.Type).
				Str("entity_value", entity.Value).
				Float64("entity_confidence", entity.Confidence).
				Interface("entity_position", entity.Position).
				Interface("entity_metadata", entity.Metadata).
				Msg("Entity details")
		}

		// Log detailed languages
		for i, language := range result.Result.Languages {
			logger.Debug().Str("customer_id", input.CustomerID).
				Int("language_index", i).
				Str("language_code", language.Code).
				Float64("language_confidence", language.Confidence).
				Bool("is_primary", language.IsPrimary).
				Interface("language_metadata", language.Metadata).
				Msg("Language details")
		}
	}

	logger.Info().Msg("Batch processing completed")
}
