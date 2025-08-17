package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/joho/godotenv"
)

// ตัวอย่างสร้าง Graph พื้นฐานใน Eino
func main() {
	// Load environment
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

	// สร้าง model configuration
	config := &openai.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: "https://openrouter.ai/api/v1",
		Model:   "openai/gpt-3.5-turbo",
	}

	model, err := openai.NewChatModel(ctx, config)
	if err != nil {
		fmt.Printf("Error creating model: %v\n", err)
		return
	}

	// === ตัวอย่างที่ 1: Chain (ง่ายที่สุด) ===
	fmt.Println("=== 1. Basic Chain Example ===")
	runBasicChain(ctx, model)

	// === ตัวอย่างที่ 2: Graph พื้นฐาน ===
	fmt.Println("\n=== 2. Basic Graph Example ===")
	runBasicGraph(ctx, model)
}

// ตัวอย่าง Chain ที่เป็นพื้นฐาน
func runBasicChain(ctx context.Context, model *openai.ChatModel) {
	// สร้าง lambda functions
	step1 := compose.InvokableLambda(func(ctx context.Context, input []string) (string, error) {
		// Step 1: รวม input
		combined := ""
		for _, s := range input {
			combined += s + " "
		}
		fmt.Printf("Chain Step 1 - Combined input: %s\n", combined)
		return combined, nil
	})

	step2 := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		// Step 2: ประมวลผล
		processed := "Processed: " + input
		fmt.Printf("Chain Step 2 - Processed: %s\n", processed)
		return processed, nil
	})

	// สร้าง chain ที่เรียงลำดับ
	chain := compose.NewChain[[]string, string]().
		AppendLambda(step1).
		AppendLambda(step2)

	// compile chain
	runnable, err := chain.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling chain: %v\n", err)
		return
	}

	// ทดสอบ chain
	input := []string{"Hello", "Eino", "Graph"}
	result, err := runnable.Invoke(ctx, input)
	if err != nil {
		fmt.Printf("Error running chain: %v\n", err)
		return
	}
	fmt.Printf("Chain Result: %s\n", result)
}

// ตัวอย่าง Graph พื้นฐาน
func runBasicGraph(ctx context.Context, model *openai.ChatModel) {
	// สร้าง Graph
	graph := compose.NewGraph[string, string]()

	// สร้าง lambda nodes
	inputProcessor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		processed := "📝 Processing: " + input
		fmt.Printf("Node 'input_processor': %s\n", processed)
		return processed, nil
	})

	analyzer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		analyzed := "🔍 Analysis: " + input + " [analyzed]"
		fmt.Printf("Node 'analyzer': %s\n", analyzed)
		return analyzed, nil
	})

	formatter := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		formatted := "✨ Final: " + input + " [formatted]"
		fmt.Printf("Node 'formatter': %s\n", formatted)
		return formatted, nil
	})

	// เพิ่ม nodes (components)
	err := graph.AddLambdaNode("input_processor", inputProcessor)
	if err != nil {
		fmt.Printf("Error adding input_processor node: %v\n", err)
		return
	}

	err = graph.AddLambdaNode("analyzer", analyzer)
	if err != nil {
		fmt.Printf("Error adding analyzer node: %v\n", err)
		return
	}

	err = graph.AddLambdaNode("formatter", formatter)
	if err != nil {
		fmt.Printf("Error adding formatter node: %v\n", err)
		return
	}

	// เชื่อม nodes เข้าด้วยกัน (สร้าง edges)
	err = graph.AddEdge(compose.START, "input_processor")
	if err != nil {
		fmt.Printf("Error adding edge START->input_processor: %v\n", err)
		return
	}

	err = graph.AddEdge("input_processor", "analyzer")
	if err != nil {
		fmt.Printf("Error adding edge input_processor->analyzer: %v\n", err)
		return
	}

	err = graph.AddEdge("analyzer", "formatter")
	if err != nil {
		fmt.Printf("Error adding edge analyzer->formatter: %v\n", err)
		return
	}

	err = graph.AddEdge("formatter", compose.END)
	if err != nil {
		fmt.Printf("Error adding edge formatter->END: %v\n", err)
		return
	}

	// compile graph
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}

	// ทดสอบ graph
	input := "สวัสดี Eino Graph!"
	fmt.Printf("Graph Input: %s\n", input)

	result, err := runnable.Invoke(ctx, input)
	if err != nil {
		fmt.Printf("Error running graph: %v\n", err)
		return
	}

	fmt.Printf("Graph Final Result: %s\n", result)
}
