package main

import (
	"context"
	"eino_llm_poc/src"
	"eino_llm_poc/src/llm/nlu"
	"eino_llm_poc/src/model"
	"eino_llm_poc/src/storage"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

type QueryInputWithState struct {
	CustomerID      string
	Query           string
	ConversationCtx string // Pre-built conversation context
}

type QueryOutputWithState struct {
	NLUResult  *model.NLUResponse
	ParseError error
}

type SessionState struct {
	CustomerID string
	History    ConversationHistory
}

type ConversationHistory struct {
	Messages []*schema.Message `json:"messages"`
}

const (
	NodeNLUBuildPrompt = "BuildPrompt"
	NodeNLUChatModel   = "NLUChatModel"
	NodeNLUParser      = "ParseNLU"
	MaxNLUContext      = 5
)

func trimTail(messages []*schema.Message, maxTurns int) []*schema.Message {
	if len(messages) <= maxTurns {
		return messages
	}
	return messages[len(messages)-maxTurns:]
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	yamlConfig, err := src.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("NLU Config: %+v\n", yamlConfig.NLUConfig)

	// Initialize Redis storage for conversation history (required)
	ctx := context.Background()
	redisStore, err := storage.NewRedisStorage[ConversationHistory](ctx)
	if err != nil {
		log.Fatal("Redis is required for session management:", err)
	}
	fmt.Println("Redis storage initialized")

	// Model configuration
	config := &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     "https://openrouter.ai/api/v1",
		Model:       yamlConfig.NLUConfig.Model,
		MaxTokens:   &yamlConfig.NLUConfig.MaxTokens,
		Temperature: &yamlConfig.NLUConfig.Temperature,
	}

	chatModel, err := openai.NewChatModel(ctx, config)
	if err != nil {
		fmt.Printf("Error creating model: %v\n", err)
		return
	}

	genStateFunc := func(ctx context.Context) *SessionState {
		return &SessionState{
			CustomerID: "",
			History:    ConversationHistory{Messages: []*schema.Message{}},
		}
	}
	// Graph with state management
	g := compose.NewGraph[QueryInputWithState, QueryOutputWithState](
		compose.WithGenLocalState(genStateFunc),
	)
	// State Pre Handler: Load conversation from Redis and prepare context
	statePreHandler := func(ctx context.Context, input QueryInputWithState, state *SessionState) (QueryInputWithState, error) {
		state.CustomerID = input.CustomerID

		// Load full conversation history from Redis
		h, err := redisStore.GetAndTouch(ctx, input.CustomerID)
		if err != nil {
			h = ConversationHistory{Messages: nil}
		}
		state.History = h

		// Add current user message to full history
		state.History.Messages = append(state.History.Messages, schema.UserMessage(input.Query))

		// Build conversation context using only last 5 messages for NLU
		recentMessages := trimTail(state.History.Messages, MaxNLUContext)

		var contextBuilder strings.Builder
		contextBuilder.WriteString("<conversation_context>\n")
		for _, msg := range recentMessages {
			switch msg.Role {
			case schema.User:
				contextBuilder.WriteString("UserMessage(" + msg.Content + ")\n")
			case schema.Assistant:
				contextBuilder.WriteString("AssistantMessage(" + msg.Content + ")\n")
			}
		}

		contextBuilder.WriteString("</conversation_context>\n")
		contextBuilder.WriteString("<current_message_to_analyze>\n")
		contextBuilder.WriteString("UserMessage(" + input.Query + ")\n")
		contextBuilder.WriteString("</current_message_to_analyze>")

		log.Printf("Customer %s: Loaded %d messages, now %v ",
			input.CustomerID, len(state.History.Messages), state.History.Messages)

		// Return modified input with conversation context
		return QueryInputWithState{
			CustomerID:      input.CustomerID,
			Query:           input.Query,
			ConversationCtx: contextBuilder.String(),
		}, nil
	}

	// State Post Handler: Save updated conversation back to Redis
	statePostHandler := func(ctx context.Context, output []*schema.Message, state *SessionState) ([]*schema.Message, error) {
		// Save updated conversation history back to Redis
		if err := redisStore.SetSession(ctx, state.CustomerID, state.History); err != nil {
			log.Printf("Warning: Failed to save conversation history for %s: %v", state.CustomerID, err)
		} else {
			log.Printf("Customer %s: Saved conversation with %d messages to Redis",
				state.CustomerID, len(state.History.Messages))
		}
		return output, nil
	}

	// Build prompt for NLU processing with conversation context
	buildPromptFunc := compose.InvokableLambda(func(ctx context.Context, input QueryInputWithState) ([]*schema.Message, error) {
		messages := []*schema.Message{
			schema.SystemMessage(nlu.GetSystemTemplateProcessed(&yamlConfig.NLUConfig)),
			schema.UserMessage(input.ConversationCtx), // Pre-built conversation context
		}
		log.Printf("Customer %s: Generated NLU Input with conversation context", input.CustomerID)
		return messages, nil
	})

	// Parse NLU response from chat model output
	parseNLUFunc := compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (QueryOutputWithState, error) {
		processor := nlu.NewNLUProcessor()
		nluResult, parseErr := processor.ParseResponse(input.Content)

		return QueryOutputWithState{
			NLUResult:  nluResult,
			ParseError: parseErr,
		}, nil
	})

	// Add nodes with state handlers
	g.AddLambdaNode(NodeNLUBuildPrompt, buildPromptFunc,
		compose.WithStatePreHandler(statePreHandler),
		compose.WithStatePostHandler(statePostHandler))
	g.AddChatModelNode(NodeNLUChatModel, chatModel)
	g.AddLambdaNode(NodeNLUParser, parseNLUFunc)

	// Add edges
	g.AddEdge(compose.START, NodeNLUBuildPrompt)
	g.AddEdge(NodeNLUBuildPrompt, NodeNLUChatModel)
	g.AddEdge(NodeNLUChatModel, NodeNLUParser)
	g.AddEdge(NodeNLUParser, compose.END)

	// Compile graph
	runnable, err := g.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}

	// Test with multiple inputs
	inputs := []QueryInputWithState{
		{CustomerID: "132", Query: "สวัสดี"},
		{CustomerID: "132", Query: "สนใจซื้อของ"},
		{CustomerID: "132", Query: "ราคาเท่าไหร่"},
		{CustomerID: "132", Query: "เเพงจัง"},
		{CustomerID: "132", Query: "ขอบคุณนะครับ"},
	}

	for i, input := range inputs {
		fmt.Printf("\n=== Processing Input %d ===\n", i+1)
		fmt.Printf("Input: CustomerID=%s, Query=%s\n", input.CustomerID, input.Query)

		start := time.Now()
		result, err := runnable.Invoke(ctx, input)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("Error running graph: %v\n", err)
			continue
		}

		if result.ParseError != nil {
			fmt.Printf("❌ Parse Error: %v\n", result.ParseError)
		} else if result.NLUResult != nil {
			fmt.Printf("✅ Primary Intent: %s, Language: %s, Score: %.3f\n",
				result.NLUResult.PrimaryIntent, result.NLUResult.PrimaryLanguage,
				result.NLUResult.ImportanceScore)
		}

		fmt.Printf("⏱️ Time: %v\n", duration)
	}
}
