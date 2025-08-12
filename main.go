package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"eino_llm_poc/internal/config"
	"eino_llm_poc/internal/core"
	"eino_llm_poc/internal/nodes"
	"eino_llm_poc/internal/storage"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ctx := context.Background()

	// Get API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENROUTER_API_KEY environment variable")
		return
	}

	// Load configuration from config.yaml
	yamlConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		fmt.Printf("Error loading config.yaml: %v\n", err)
		return
	}

	// Build configurations
	nluConfig := config.BuildNLUConfig(yamlConfig, apiKey)
	coreConfig := config.BuildCoreConfig(nluConfig)

	// Initialize storage managers
	sessionMgr := storage.NewMemorySessionManager(int(coreConfig.Redis.TTL))
	longtermMgr := storage.NewJSONLongtermManager(coreConfig.Storage.LongtermDir)

	// Create graph processor
	processor := core.NewGraphProcessor(coreConfig)

	// Create and add nodes
	nluNode, err := nodes.NewNLUNode(ctx, nluConfig, longtermMgr)
	if err != nil {
		fmt.Printf("Error creating NLU node: %v\n", err)
		return
	}

	routingNode := nodes.NewRoutingNode(sessionMgr, longtermMgr, coreConfig)
	responseNode := nodes.NewResponseNode(coreConfig)
	toolsNode := nodes.NewToolsNode(coreConfig)

	// Add nodes to processor
	if err := processor.AddNode(nluNode); err != nil {
		fmt.Printf("Error adding NLU node: %v\n", err)
		return
	}
	if err := processor.AddNode(routingNode); err != nil {
		fmt.Printf("Error adding routing node: %v\n", err)
		return
	}
	if err := processor.AddNode(responseNode); err != nil {
		fmt.Printf("Error adding response node: %v\n", err)
		return
	}
	if err := processor.AddNode(toolsNode); err != nil {
		fmt.Printf("Error adding tools node: %v\n", err)
		return
	}

	// Parse intents and entities from config for test data
	defaultIntents := strings.Split(yamlConfig.NLU.DefaultIntent, ", ")
	additionalIntents := strings.Split(yamlConfig.NLU.AdditionalIntent, ", ")
	defaultEntities := strings.Split(yamlConfig.NLU.DefaultEntity, ", ")
	additionalEntities := strings.Split(yamlConfig.NLU.AdditionalEntity, ", ")

	// Test with sample requests for Thai computer sales domain
	testRequests := []core.ProcessorInput{
		{
			UserMessage: "สวัสดีครับ อยากซื้อโน้ตบุ๊ครับ",
			CustomerID:  "tan123",
		},
		{
			UserMessage: "ขอราคา MacBook หน่อย",
			CustomerID:  "tan123",
		},
		{
			UserMessage: "มี Apple สินค้าอะไรบ้าง",
			CustomerID:  "tan123",
		},
		{
			UserMessage: "เปรียบเทียบสินค้าให้หน่อย",
			CustomerID:  "tan123",
		},
		{
			UserMessage: "ขอบคุณครับ ไม่เอาแล้ว",
			CustomerID:  "tan123",
		},
	}

	// Process each test request using graph flow
	for i, request := range testRequests {
		fmt.Printf("\n=== Test %d - Graph Flow ===\n", i+1)
		fmt.Printf("Input: %s\n", request.UserMessage)

		// Execute graph flow
		output, err := processor.Execute(ctx, request)
		if err != nil {
			fmt.Printf("Error processing request: %v\n", err)
			continue
		}

		// Display results
		fmt.Printf("Response: %s\n", output.Response)
		fmt.Printf("Processing Time: %dms\n", output.ProcessingTime)

		if output.NLUAnalysis != nil {
			fmt.Printf("Primary Intent: %s (importance: %.3f)\n",
				output.NLUAnalysis.PrimaryIntent, output.NLUAnalysis.ImportanceScore)
		}

		if len(output.ToolsExecuted) > 0 {
			fmt.Printf("Tools Used: %s\n", strings.Join(output.ToolsExecuted, ", "))
		}

		// Pretty print detailed output (optional)
		if detailedOutput, err := json.MarshalIndent(output, "", "  "); err == nil {
			fmt.Printf("\nDetailed Output:\n%s\n", detailedOutput)
		}
	}

	fmt.Printf("\n🎉 Graph Flow Demo Completed!\n")

	// Print graph statistics
	fmt.Printf("\n📊 Graph Statistics:\n")
	fmt.Printf("   Nodes: NLU, Routing, Response\n")
	fmt.Printf("   Default Flow: %s → %s → %s\n", "NLU", "Routing", "Response")
	fmt.Printf("   Storage: Session (Memory), Longterm (JSON)\n")
	fmt.Printf("   Config Source: config.yaml + environment variables\n")

	// Print intents/entities loaded (for reference)
	fmt.Printf("\n📋 Loaded Configuration:\n")
	fmt.Printf("   Default Intents: %v\n", defaultIntents)
	fmt.Printf("   Additional Intents: %v\n", additionalIntents)
	fmt.Printf("   Default Entities: %v\n", defaultEntities)
	fmt.Printf("   Additional Entities: %v\n", additionalEntities)
}
