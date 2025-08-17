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

type RunState struct {
	History []*schema.Message `json:"history"`
}

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
	_ = chatModel
	if err != nil {
		fmt.Printf("Error creating model: %v\n", err)
		return
	}

	g := compose.NewGraph[QueryInput, QueryOutput]()

	// Test the NLU template with config values
	testInput := "สวัสดีครับ อยากซื้อรองเท้า"
	nluTemplate := nlu.CreateNLUTemplateFromConfig(testInput, &yamlConfig.NLUConfig)
	g.AddChatTemplateNode("nlu_tmpl", nluTemplate)

	g.AddEdge(compose.START, "nlu_tmpl")
	g.AddEdge("nlu_tmpl", compose.END)

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
