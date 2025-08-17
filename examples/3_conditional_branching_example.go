package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/joho/godotenv"
)

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

	// === ตัวอย่าง Conditional Branching ===
	fmt.Println("=== Conditional Branching Graph ===")
	runConditionalBranching(ctx, model)

	// === ตัวอย่าง Multi-Branch Processing ===
	fmt.Println("\n=== Multi-Branch Processing ===")
	runMultiBranchProcessing(ctx, model)
}

// Conditional Branching ด้วย explicit routing
func runConditionalBranching(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[string, string]()

	// Single router that handles routing internally (avoiding merge conflicts)
	router := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		// วิเคราะห์ input และติด metadata
		var route, analysis, result string

		if len(input) < 15 {
			route = "short"
			analysis = "SHORT_TEXT"
			result = fmt.Sprintf("📝 Short Processed: '%s' [quick response]", input)
			fmt.Printf("🔍 Analyzer: Input '%s' -> Route: %s, Analysis: %s\n", input, route, analysis)
			fmt.Printf("Short Processor: %s\n", result)
		} else if strings.Contains(strings.ToLower(input), "urgent") || strings.Contains(strings.ToLower(input), "ด่วน") {
			route = "urgent"
			analysis = "URGENT_REQUEST"
			result = fmt.Sprintf("🚨 Urgent Processed: '%s' [high priority response]", input)
			fmt.Printf("🔍 Analyzer: Input '%s' -> Route: %s, Analysis: %s\n", input, route, analysis)
			fmt.Printf("Urgent Processor: %s\n", result)
		} else {
			route = "normal"
			analysis = "NORMAL_TEXT"
			words := strings.Fields(input)
			result = fmt.Sprintf("📚 Normal Processed: '%s' [detailed analysis: %d words]", input, len(words))
			fmt.Printf("🔍 Analyzer: Input '%s' -> Route: %s, Analysis: %s\n", input, route, analysis)
			fmt.Printf("Normal Processor: %s\n", result)
		}

		final := fmt.Sprintf("✅ Final Result: %s", result)
		fmt.Printf("Aggregator: %s\n", final)
		return final, nil
	})

	// เพิ่ม node เดียว
	graph.AddLambdaNode("router", router)

	// เชื่อม edges แบบง่าย
	graph.AddEdge(compose.START, "router")
	graph.AddEdge("router", compose.END)

	// Compile และทดสอบ
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling conditional graph: %v\n", err)
		return
	}

	testInputs := []string{
		"สั้น",
		"ข้อความด่วนที่ต้องการการตอบสนองเร็ว",
		"ข้อความปกติที่ต้องการการวิเคราะห์อย่างละเอียดและครอบคลุม",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)

		result, err := runnable.Invoke(ctx, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Result: %s\n", result)
	}
}

// Multi-branch processing ที่แต่ละ branch ทำงานแตกต่างกัน
func runMultiBranchProcessing(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[string, string]()

	// Single processor that handles all cases internally (avoiding merge conflicts)
	conditionalProcessor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		// จำแนกประเภทของ input
		var category, result string
		inputLower := strings.ToLower(input)

		if strings.Contains(inputLower, "question") || strings.Contains(inputLower, "คำถาม") || strings.Contains(inputLower, "?") {
			category = "QUESTION"
			result = fmt.Sprintf("❓ Q&A Response: '%s' -> This looks like a question requiring detailed analysis", input)
			fmt.Printf("🏷️ Classifier: '%s' -> Category: %s\n", input, category)
			fmt.Printf("Question Processor: %s\n", result)
		} else if strings.Contains(inputLower, "command") || strings.Contains(inputLower, "คำสั่ง") {
			category = "COMMAND"
			result = fmt.Sprintf("⚡ Command Response: '%s' -> Executing command with high priority", input)
			fmt.Printf("🏷️ Classifier: '%s' -> Category: %s\n", input, category)
			fmt.Printf("Command Processor: %s\n", result)
		} else if len(input) < 10 {
			category = "SIMPLE"
			result = fmt.Sprintf("⚡ Simple Response: '%s' -> Quick processing completed", input)
			fmt.Printf("🏷️ Classifier: '%s' -> Category: %s\n", input, category)
			fmt.Printf("Simple Processor: %s\n", result)
		} else {
			category = "COMPLEX"
			words := strings.Fields(input)
			chars := len(input)
			result = fmt.Sprintf("🔬 Complex Response: '%s' -> Advanced analysis [%d words, %d chars]", input, len(words), chars)
			fmt.Printf("🏷️ Classifier: '%s' -> Category: %s\n", input, category)
			fmt.Printf("Complex Processor: %s\n", result)
		}

		return result, nil
	})

	// เพิ่ม node เดียว
	graph.AddLambdaNode("conditional_proc", conditionalProcessor)

	// เชื่อม edges แบบง่าย
	graph.AddEdge(compose.START, "conditional_proc")
	graph.AddEdge("conditional_proc", compose.END)

	// Compile และทดสอบ
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling multi-branch graph: %v\n", err)
		return
	}

	testInputs := []string{
		"Hello?",
		"สั้น",
		"Execute command now",
		"นี่คือข้อความที่ยาวและซับซ้อนที่ต้องการการวิเคราะห์อย่างละเอียดจากระบบ",
		"What is the weather today?",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)

		result, err := runnable.Invoke(ctx, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Result: %s\n", result)
	}
}
