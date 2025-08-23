package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"eino_llm_poc/src"
	"eino_llm_poc/src/conversation"
	"eino_llm_poc/src/llm/nlu"
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

type QueryInputWithContext struct {
	CustomerID      string `json:"customer_id"`
	Query           string `json:"query"`
	ConversationCtx string `json:"conversation_ctx"`
}

type QueryOutput struct {
	CustomerID string            `json:"customer_id"`
	Result     model.NLUResponse `json:"result"`
}

const (
	NodeNLUBuildPrompt = "BuildPrompt"
	NodeBuildMessages  = "BuildMessages"
	NodeNLUChatModel   = "NLUChatModel"
	NodeNLUParser      = "ParseNLU"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	ctx := context.Background()
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	// Load configuration
	yamlConfig, err := src.LoadConfig("config.yaml")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// Setup conversation manager with config
	ttlMinutes := yamlConfig.ConversationConfig.TTL

	conversationConfig := conversation.ConversationConfig{
		TTL: time.Duration(ttlMinutes) * time.Minute,
		NLU: struct{ MaxTurns int }{MaxTurns: yamlConfig.ConversationConfig.NLU.MaxTurns},
	}

	conversationManager, err := conversation.NewConversationManager(ctx, conversationConfig)
	if err != nil {
		fmt.Printf("Error setting up conversation manager: %v\n", err)
		return
	}

	// Setup OpenAI model
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     "https://openrouter.ai/api/v1",
		Model:       yamlConfig.NLUConfig.Model,
		Temperature: &yamlConfig.NLUConfig.Temperature,
		MaxTokens:   &yamlConfig.NLUConfig.MaxTokens,
	})
	if err != nil {
		fmt.Printf("Error creating model: %v\n", err)
		return
	}

	// Create graph without state management
	g := compose.NewGraph[QueryInput, QueryOutput]()

	// ----- Nodes -----
	buildPrompt := compose.InvokableLambda(func(ctx context.Context, input QueryInput) (QueryInputWithContext, error) {
		// Use simplified ConversationManager for NLU processing
		conversationCtx, err := conversationManager.ProcessNLUMessage(ctx, input.CustomerID, input.Query)
		if err != nil {
			return QueryInputWithContext{}, err
		}

		log.Printf("Customer %s: Generated NLU Input with conversation context", input.CustomerID)

		return QueryInputWithContext{
			CustomerID:      input.CustomerID,
			Query:           input.Query,
			ConversationCtx: conversationCtx,
		}, nil
	})

	buildMessages := compose.InvokableLambda(func(ctx context.Context, input QueryInputWithContext) ([]*schema.Message, error) {
		// Generate system prompt with injected configuration
		systemPrompt := nlu.GetSystemTemplateProcessed(&yamlConfig.NLUConfig)

		// Create user prompt with conversation context
		userPrompt := input.ConversationCtx

		return []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userPrompt),
		}, nil
	})

	parseNLU := compose.InvokableLambda(func(ctx context.Context, resp *schema.Message) (QueryOutput, error) {
		// In a real implementation, we'd need to pass this through the graph context

		// Parse NLU response
		result, err := nlu.ParseNLUResponse(resp.Content)
		if err != nil {
			log.Printf("Error parsing NLU response: %v", err)
			return QueryOutput{}, err
		}

		// TODO: Fix this to get actual customerID from graph context
		customerID := "132" // Temporary hardcode

		// Save assistant response to conversation
		if err := conversationManager.SaveResponse(ctx, customerID, resp.Content); err != nil {
			log.Printf("Warning: Failed to save assistant response: %v", err)
		}

		log.Printf("Customer %s: Saved conversation to Redis", customerID)

		return QueryOutput{
			CustomerID: customerID,
			Result:     *result,
		}, nil
	})

	g.AddLambdaNode(NodeNLUBuildPrompt, buildPrompt)
	g.AddLambdaNode(NodeBuildMessages, buildMessages)
	g.AddChatModelNode(NodeNLUChatModel, chatModel)
	g.AddLambdaNode(NodeNLUParser, parseNLU)
	// Add edges
	g.AddEdge(compose.START, NodeNLUBuildPrompt)
	g.AddEdge(NodeNLUBuildPrompt, NodeBuildMessages)
	g.AddEdge(NodeBuildMessages, NodeNLUChatModel)
	g.AddEdge(NodeNLUChatModel, NodeNLUParser)
	g.AddEdge(NodeNLUParser, compose.END)

	// Compile graph
	runnable, err := g.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}

	// Test with multiple inputs
	inputs := []QueryInput{
		{CustomerID: "132", Query: "สวัสดี"},
		{CustomerID: "132", Query: "สนใจซื้อของ"},
		{CustomerID: "132", Query: "ราคาเท่าไหร่"},
		{CustomerID: "132", Query: "แพงจัง"},
		{CustomerID: "132", Query: "ขอบคุณนะครับ"},
	}

	for i, input := range inputs {
		fmt.Printf("\n=== Processing Input %d ===\n", i+1)
		fmt.Printf("Input: CustomerID=%s, Query=%s\n", input.CustomerID, input.Query)

		start := time.Now()
		result, err := runnable.Invoke(ctx, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Display results
		if result.Result.PrimaryIntent != "" {
			fmt.Printf("Primary Intent: %s, Language: %s, Score: %.3f\n",
				result.Result.PrimaryIntent,
				result.Result.PrimaryLanguage,
				result.Result.ImportanceScore)
		}

		fmt.Printf("⏱️ Time: %v\n", time.Since(start))
	}
}
