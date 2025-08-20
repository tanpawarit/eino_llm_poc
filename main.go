package main

import (
	"context"
	"eino_llm_poc/src"
	"eino_llm_poc/src/llm/nlu"
	"fmt"
	"log"
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
		log.Println("No .env file found")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	yamlConfig, err := src.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("env Config: %+v\n", apiKey)
	fmt.Printf("env Config: %+v\n", baseURL)
	fmt.Printf("NLU Config: %+v\n", yamlConfig.NLUConfig)

	// สร้าง model configuration with NLU config injection
	config := &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     "https://openrouter.ai/api/v1",
		Model:       yamlConfig.NLUConfig.Model,
		MaxTokens:   &yamlConfig.NLUConfig.MaxTokens,
		Temperature: &yamlConfig.NLUConfig.Temperature,
	}

	ctx := context.Background()
	chatModel, err := openai.NewChatModel(ctx, config)
	if err != nil {
		fmt.Printf("Error creating model: %v\n", err)
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
			schema.SystemMessage(nlu.GetSystemTemplateProcessed(&yamlConfig.NLUConfig)),
			schema.UserMessage(NLUinput),
		}
		log.Println(messages)
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

	// Compile และทดสอบ
	runnable, err := g.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}

	input := QueryInput{
		CustomerID: "12345",
		Query:      "สวัสดีครับ อยากซื้อรองเท้า",
	}
	fmt.Printf("Input: %+v\n", input)

	start := time.Now()
	result, err := runnable.Invoke(ctx, input)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error running graph: %v\n", err)
		return
	}

	fmt.Printf("Result: %+v\n", result)
	fmt.Printf("⏱️ Total time: %v\n", duration)
}
