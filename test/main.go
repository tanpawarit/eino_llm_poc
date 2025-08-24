package main

import (
	"context"
	"eino_llm_poc/src"
	"eino_llm_poc/src/llm/nlu"
	"eino_llm_poc/src/logger"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

type QueryInput struct {
	CustomerID string
	Query      string
}

type QueryOutput struct {
	Response string
}

const (
	NodeNLUInputTransform = "InputTransformer"
	NodeNLUChatModel      = "ChatModel"
	NodeNLUOutputParser   = "OutputParser"
)

func main() {
	if err := godotenv.Load(); err != nil {
		// Will use default values if .env not found
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	config, err := src.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// Initialize logger with configuration
	if err := logger.InitLogger(config.LogConfig); err != nil {
		fmt.Printf("Error initializing logger: %v\n", err)
		return
	}
	logger.Debug().Str("api_key_set", func() string {
		if apiKey != "" {
			return "yes"
		}
		return "no"
	}()).Str("base_url", baseURL).Msg("Environment configuration loaded")
	logger.Debug().Interface("nlu_config", config.NLUConfig).Msg("NLU configuration loaded")

	// สร้าง model configuration with NLU config injection
	modelConfig := &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     "https://openrouter.ai/api/v1",
		Model:       config.NLUConfig.Model,
		MaxTokens:   &config.NLUConfig.MaxTokens,
		Temperature: &config.NLUConfig.Temperature,
	}

	ctx := context.Background()
	chatModel, err := openai.NewChatModel(ctx, modelConfig)
	if err != nil {
		logger.Error().Err(err).Msg("Error creating model")
		return
	}

	g := compose.NewGraph[QueryInput, QueryOutput]()

	// Create NLU template as InvokableLambda node
	nluTemplateFunc := compose.InvokableLambda(func(ctx context.Context, input QueryInput) ([]*schema.Message, error) {
		// Construct the input text with conversation context format
		NLUinput := `<conversation_context>
			UserMessage(สวัสดีจ้า)
			AssistantMessage(ดีจ้า)
			UserMessage(ซื้อของหน่อยจ้า)
			AssistantMessage(ได้เลยจ้า)
			</conversation_context>
			<current_message_to_analyze>
			UserMessage(` + input.Query + `)
			</current_message_to_analyze>`

		messages := []*schema.Message{
			schema.SystemMessage(nlu.GetSystemTemplateProcessed(&config.NLUConfig)),
			schema.UserMessage(NLUinput),
		}
		logger.Debug().Int("message_count", len(messages)).Msg("Generated NLU template messages")
		return messages, nil
	})

	// Add node to convert chat model output to QueryOutput
	outputTransform := compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (QueryOutput, error) {
		return QueryOutput{
			Response: input.Content,
		}, nil
	})

	g.AddLambdaNode(NodeNLUInputTransform, nluTemplateFunc)
	g.AddChatModelNode(NodeNLUChatModel, chatModel)
	g.AddLambdaNode(NodeNLUOutputParser, outputTransform)

	g.AddEdge(compose.START, NodeNLUInputTransform)
	g.AddEdge(NodeNLUInputTransform, NodeNLUChatModel)
	g.AddEdge(NodeNLUChatModel, NodeNLUOutputParser)
	g.AddEdge(NodeNLUOutputParser, compose.END)

	// Compile graph
	runnable, err := g.Compile(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Error compiling graph")
		return
	}

	input := QueryInput{
		CustomerID: "12345",
		Query:      "สวัสดีครับ อยากซื้อรองเท้า",
	}
	logger.Info().Str("customer_id", input.CustomerID).Str("query", input.Query).Msg("Starting test processing")

	start := time.Now()
	result, err := runnable.Invoke(ctx, input)
	duration := time.Since(start)

	if err != nil {
		logger.Error().Err(err).Msg("Error running graph")
		return
	}

	logger.Info().Dur("processing_time", duration).Str("result", result.Response).Msg("Test processing completed successfully")
	fmt.Printf("Result: %+v\n", result)
	fmt.Printf("⏱️ Total time: %v\n", duration)
}
