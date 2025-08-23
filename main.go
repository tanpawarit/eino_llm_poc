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

	conversationConfig := model.ConversationConfig{
		TTL: ttlMinutes,
		NLU: struct {
			MaxTurns int `yaml:"max_turns"`
		}{MaxTurns: yamlConfig.ConversationConfig.NLU.MaxTurns},
		Response: struct {
			MaxTurns int `yaml:"max_turns"`
		}{MaxTurns: yamlConfig.ConversationConfig.Response.MaxTurns},
	}

	messagesManager, err := conversation.NewMessagesManager(ctx, conversationConfig)
	if err != nil {
		fmt.Printf("Error setting up conversation manager: %v\n", err)
		return
	}

	// Setup OpenAI model
	chatModelNLU, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
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

	g := compose.NewGraph[QueryInput, QueryOutput](
		compose.WithGenLocalState(func(ctx context.Context) *State {
			return &State{
				History: []*schema.Message{},
			}
		}),
	)

	inputConverterNLU := compose.InvokableLambda(func(ctx context.Context, input QueryInput) ([]*schema.Message, error) {
		log.Printf("Customer %s: Processing query - %s", input.CustomerID, input.Query)
		conversationCtx, err := messagesManager.ProcessNLUMessage(ctx, input.CustomerID, input.Query)
		if err != nil {
			log.Printf("Customer %s: Error getting conversation context: %v", input.CustomerID, err)
			return nil, err
		}

		log.Printf("Customer %s: Retrieved conversation context from Redis", input.CustomerID)

		// Generate system prompt
		systemPrompt := nlu.GetSystemTemplateProcessed(&yamlConfig.NLUConfig)

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
		log.Printf("Customer %s: inputConverterNode Messages: %v", input.CustomerID, messages)
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
		log.Printf("Customer %s: Model response - %s", customerID,
			out.Content[:(len(out.Content))])

		// Save response to Redis
		if err := messagesManager.SaveResponse(ctx, customerID, out.Content); err != nil {
			log.Printf("Warning: Failed to save response to Redis for customer %s: %v", customerID, err)
		} else {
			log.Printf("Customer %s: Successfully saved response to Redis", customerID)
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
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}

	// Test with multiple inputs
	inputs := []QueryInput{
		{CustomerID: "1321", Query: "สวัสดี"},
		{CustomerID: "1321", Query: "สนใจซื้อของ"},
		{CustomerID: "1321", Query: "ราคาเท่าไหร่"},
		{CustomerID: "1321", Query: "แพงจัง"},
		{CustomerID: "1321", Query: "ขอบคุณนะครับ"},
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
