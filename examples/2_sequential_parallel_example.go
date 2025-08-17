package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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

	// === ตัวอย่าง Sequential Graph (ไม่มี merge) ===
	fmt.Println("=== Sequential Processing Graph ===")
	runSequentialGraph(ctx, model)

	// === ตัวอย่าง Parallel with Single Output ===
	fmt.Println("\n=== Parallel with Single Output ===")
	runParallelSingleOutput(ctx, model)

	// === ตัวอย่าง Branch Selection ===
	fmt.Println("\n=== Branch Selection Example ===")
	runBranchSelection(ctx, model)
}

// Sequential Graph - แต่ละ step ทำตามลำดับ
func runSequentialGraph(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[string, string]()

	// Step 1: Input validation
	validator := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		if len(input) == 0 {
			return "❌ Empty input", nil
		}
		result := fmt.Sprintf("✅ Valid input: %s", input)
		fmt.Printf("Validator: %s\n", result)
		return input, nil // ส่ง original input ต่อไป
	})

	// Step 2: Text processing
	processor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		processed := strings.ToUpper(input)
		result := fmt.Sprintf("🔄 Processed: %s", processed)
		fmt.Printf("Processor: %s\n", result)
		return processed, nil
	})

	// Step 3: Add metadata
	enricher := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		enriched := fmt.Sprintf("📝 Enriched: %s [length=%d]", input, len(input))
		fmt.Printf("Enricher: %s\n", enriched)
		return enriched, nil
	})

	// Step 4: Final formatting
	formatter := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		final := fmt.Sprintf("🎯 Final: %s", input)
		fmt.Printf("Formatter: %s\n", final)
		return final, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("validator", validator)
	graph.AddLambdaNode("processor", processor)
	graph.AddLambdaNode("enricher", enricher)
	graph.AddLambdaNode("formatter", formatter)

	// เชื่อม edges แบบ sequential
	graph.AddEdge(compose.START, "validator")
	graph.AddEdge("validator", "processor")
	graph.AddEdge("processor", "enricher")
	graph.AddEdge("enricher", "formatter")
	graph.AddEdge("formatter", compose.END)

	// Compile และทดสอบ
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling sequential graph: %v\n", err)
		return
	}

	input := "สวัสดี Eino Graph!"
	fmt.Printf("Input: %s\n", input)

	start := time.Now()
	result, err := runnable.Invoke(ctx, input)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error running sequential graph: %v\n", err)
		return
	}

	fmt.Printf("Sequential Result: %s\n", result)
	fmt.Printf("⏱️ Total time: %v\n", duration)
}

// Parallel processing แต่มี single output path
func runParallelSingleOutput(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[string, string]()

	// Task 1: Quick analysis
	quickAnalysis := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		time.Sleep(30 * time.Millisecond)
		result := fmt.Sprintf("⚡ Quick: %d chars", len(input))
		fmt.Printf("Quick Analysis: %s\n", result)
		return result, nil
	})

	// Task 2: Deep analysis (takes longer)
	deepAnalysis := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		time.Sleep(100 * time.Millisecond)
		words := strings.Fields(input)
		result := fmt.Sprintf("🔍 Deep: %d words, %d chars", len(words), len(input))
		fmt.Printf("Deep Analysis: %s\n", result)
		return result, nil
	})

	// Final processor - รับแค่ผลลัพธ์จาก deep analysis
	finalProcessor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		final := fmt.Sprintf("✅ Final: %s", input)
		fmt.Printf("Final: %s\n", final)
		return final, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("quick", quickAnalysis)
	graph.AddLambdaNode("deep", deepAnalysis)
	graph.AddLambdaNode("final", finalProcessor)

	// เชื่อม edges - ทั้งสอง task ทำงาน parallel
	graph.AddEdge(compose.START, "quick")
	graph.AddEdge(compose.START, "deep")

	// แต่ final processor รับแค่จาก deep analysis
	graph.AddEdge("deep", "final")
	graph.AddEdge("final", compose.END)

	// Compile และทดสอบ
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling parallel single output graph: %v\n", err)
		return
	}

	input := "สวัสดี Eino Graph Parallel Processing!"
	fmt.Printf("Input: %s\n", input)

	start := time.Now()
	result, err := runnable.Invoke(ctx, input)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Error running parallel single output graph: %v\n", err)
		return
	}

	fmt.Printf("Result: %s\n", result)
	fmt.Printf("⏱️ Total time: %v (both tasks ran in parallel)\n", duration)
}

// Branch selection based on input
func runBranchSelection(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[string, string]()

	// Single processor that handles both cases internally
	conditionalProcessor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		var result string
		
		if len(input) < 20 {
			// Short text processing
			result = fmt.Sprintf("📝 Short Processed: %s [simple]", input)
			fmt.Printf("Short Path: %s\n", result)
		} else {
			// Long text processing
			words := strings.Fields(input)
			result = fmt.Sprintf("📚 Long Processed: %s [advanced: %d words]", input, len(words))
			fmt.Printf("Long Path: %s\n", result)
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
		fmt.Printf("Error compiling branch selection graph: %v\n", err)
		return
	}

	inputs := []string{
		"สั้น",
		"ข้อความที่ยาวกว่าเดิมมากเพื่อทดสอบการเลือก branch ที่เหมาะสม",
	}

	for i, input := range inputs {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)

		start := time.Now()
		result, err := runnable.Invoke(ctx, input)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Result: %s\n", result)
		fmt.Printf("⏱️ Time: %v\n", duration)
	}
}
