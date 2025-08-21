package main

import (
	"context"
	"eino_llm_poc/src"
	"eino_llm_poc/src/llm/nlu"
	"eino_llm_poc/src/model"
	"encoding/json"
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
	NLUResult  *model.NLUResponse
	ParseError error
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

	// Add node to parse NLU output using the new parser
	outputTransform := compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (QueryOutput, error) {
		// Create NLU processor
		processor := nlu.NewNLUProcessor()

		// Parse the response content
		nluResult, parseErr := processor.ParseResponse(input.Content)

		return QueryOutput{
			NLUResult:  nluResult,
			ParseError: parseErr,
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
		Query:      "สวัสดี ของไม่มาถึงสักที ",
	}
	fmt.Printf("Input: %+v\n", input)

	start := time.Now()
	result, err := runnable.Invoke(ctx, input)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error running graph: %v\n", err)
		return
	}

	if result.ParseError != nil {
		fmt.Printf("❌ Parse Error: %v\n", result.ParseError)
	} else if result.NLUResult != nil {

		fmt.Printf("\n✅ Parsed NLU Result (JSON):\n")
		jsonBytes, err := json.MarshalIndent(result.NLUResult, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling JSON: %v\n", err)
			fmt.Printf("%+v\n", result.NLUResult)
		} else {
			fmt.Printf("%s\n", string(jsonBytes))
		}
		// Display Summary Information
		fmt.Printf("\n📊 Summary:\n")
		fmt.Printf("  Primary Intent: %s\n", result.NLUResult.PrimaryIntent)
		fmt.Printf("  Primary Language: %s\n", result.NLUResult.PrimaryLanguage)
		fmt.Printf("  Importance Score: %.3f\n", result.NLUResult.ImportanceScore)
		fmt.Printf("  Parsed at: %s\n", result.NLUResult.Timestamp.Format("2006-01-02 15:04:05"))
	}

	fmt.Printf("\n⏱️ Total time: %v\n", duration)
}
