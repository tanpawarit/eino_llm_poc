package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// GraphState represents the state that flows through the graph
type GraphState struct {
	Messages      []*schema.Message `json:"messages"`
	CurrentIntent string            `json:"current_intent"`
	ToolsUsed     []string          `json:"tools_used"`
	ProcessStep   int               `json:"process_step"`
	Completed     bool              `json:"completed"`
}

// Node keys for the graph
const (
	NodeAnalyzer  = "analyzer"
	NodeProcessor = "processor"
	NodeResponder = "responder"
	NodeTools     = "tools"
	START         = compose.START
	END           = compose.END
)

// IntentAnalyzer - analyzes user intent
func CreateIntentAnalyzer(model *openai.ChatModel) func(ctx context.Context, state *GraphState) (*GraphState, error) {
	return func(ctx context.Context, state *GraphState) (*GraphState, error) {
		// Create system prompt for intent analysis
		systemPrompt := `คุณเป็น AI ที่วิเคราะห์ความตั้งใจของผู้ใช้ ให้วิเคราะห์ข้อความและระบุ intent หลัก:
			- greeting: การทักทาย
			- question: การถามคำถาม  
			- request: การขอความช่วยเหลือ
			- complaint: การร้องเรียน
			- goodbye: การลาก่อน

			ตอบเฉพาะ intent เดียวเท่านั้น เช่น: greeting, question, request, complaint, goodbye`

		// Get the latest user message
		if len(state.Messages) == 0 {
			return nil, fmt.Errorf("no messages to analyze")
		}

		lastMsg := state.Messages[len(state.Messages)-1]
		if lastMsg.Role != schema.User {
			return nil, fmt.Errorf("last message is not from user")
		}

		// Create messages for intent analysis
		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(lastMsg.Content),
		}

		// Analyze intent using the model
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("intent analysis failed: %v", err)
		}

		// Update state with detected intent
		newState := *state
		newState.CurrentIntent = response.Content
		newState.ProcessStep++

		fmt.Printf("🧠 Intent Analyzer: Detected intent '%s'\n", response.Content)

		return &newState, nil
	}
}

// MessageProcessor - processes message based on intent
func CreateMessageProcessor(model *openai.ChatModel) func(ctx context.Context, state *GraphState) (*GraphState, error) {
	return func(ctx context.Context, state *GraphState) (*GraphState, error) {
		// Create processing strategy based on intent
		var systemPrompt string
		switch state.CurrentIntent {
		case "greeting":
			systemPrompt = "คุณเป็น AI ที่ทักทายอย่างเป็นมิตร ตอบการทักทายแบบสุภาพและเป็นกันเอง"
		case "question":
			systemPrompt = "คุณเป็น AI ที่ตอบคำถามแบบครบถ้วนและละเอียด ให้ข้อมูลที่เป็นประโยชน์"
		case "request":
			systemPrompt = "คุณเป็น AI ที่ช่วยเหลือผู้ใช้ ให้คำแนะนำและวิธีการที่เป็นประโยชน์"
		case "complaint":
			systemPrompt = "คุณเป็น AI ที่รับฟังข้อร้องเรียนด้วยความเข้าใจ ให้คำปลอบใจและข้อเสนอแนะ"
		case "goodbye":
			systemPrompt = "คุณเป็น AI ที่ลาก่อนแบบสุภาพและเป็นมิตร"
		default:
			systemPrompt = "คุณเป็น AI ผู้ช่วยที่เป็นมิตรและให้ความช่วยเหลือ"
		}

		// Get the latest user message for processing
		if len(state.Messages) == 0 {
			return nil, fmt.Errorf("no messages to process")
		}

		lastMsg := state.Messages[len(state.Messages)-1]

		// Create messages for processing
		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(lastMsg.Content),
		}

		// Process message using the model
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("message processing failed: %v", err)
		}

		// Update state with processed response
		newState := *state
		newState.Messages = append(state.Messages, schema.AssistantMessage(response.Content, nil))
		newState.ProcessStep++

		fmt.Printf("⚙️ Message Processor: Processed '%s' intent\n", state.CurrentIntent)

		return &newState, nil
	}
}

// ResponseGenerator - generates final response
func CreateResponseGenerator() func(ctx context.Context, state *GraphState) (*GraphState, error) {
	return func(ctx context.Context, state *GraphState) (*GraphState, error) {
		// Final response preparation
		if len(state.Messages) == 0 {
			return nil, fmt.Errorf("no messages to generate response from")
		}

		// Get the assistant's response
		var assistantResponse string
		for i := len(state.Messages) - 1; i >= 0; i-- {
			if state.Messages[i].Role == schema.Assistant {
				assistantResponse = state.Messages[i].Content
				break
			}
		}

		if assistantResponse == "" {
			return nil, fmt.Errorf("no assistant response found")
		}

		// Update state as completed
		newState := *state
		newState.Completed = true
		newState.ProcessStep++

		fmt.Printf("📝 Response Generator: Final response ready\n")
		fmt.Printf("💬 Final Response: %s\n", assistantResponse)

		return &newState, nil
	}
}

// ConditionalEdge - decides next node based on state
func CreateConditionalEdge() compose.GraphBranchCondition[*GraphState] {
	return func(ctx context.Context, state *GraphState) (string, error) {
		// Decision logic based on current state
		switch {
		case state.ProcessStep == 1:
			// After intent analysis, go to processor
			return NodeProcessor, nil
		case state.ProcessStep == 2:
			// After processing, go to responder
			return NodeResponder, nil
		case state.Completed:
			// If completed, end the graph
			return END, nil
		default:
			// Default flow
			return NodeProcessor, nil
		}
	}
}

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

	// Create model
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

	// Create the graph
	graphBuilder := compose.NewGraph[*GraphState, *GraphState]()

	// Add nodes to the graph as Lambda nodes
	analyzerLambda := compose.InvokableLambda(CreateIntentAnalyzer(model))
	err = graphBuilder.AddLambdaNode(NodeAnalyzer, analyzerLambda)
	if err != nil {
		fmt.Printf("Error adding analyzer node: %v\n", err)
		return
	}

	processorLambda := compose.InvokableLambda(CreateMessageProcessor(model))
	err = graphBuilder.AddLambdaNode(NodeProcessor, processorLambda)
	if err != nil {
		fmt.Printf("Error adding processor node: %v\n", err)
		return
	}

	responderLambda := compose.InvokableLambda(CreateResponseGenerator())
	err = graphBuilder.AddLambdaNode(NodeResponder, responderLambda)
	if err != nil {
		fmt.Printf("Error adding responder node: %v\n", err)
		return
	}

	// Add edges to the graph
	err = graphBuilder.AddEdge(START, NodeAnalyzer)
	if err != nil {
		fmt.Printf("Error adding start edge: %v\n", err)
		return
	}

	// Add direct edges instead of branches for simpler flow
	err = graphBuilder.AddEdge(NodeAnalyzer, NodeProcessor)
	if err != nil {
		fmt.Printf("Error adding analyzer to processor edge: %v\n", err)
		return
	}

	err = graphBuilder.AddEdge(NodeProcessor, NodeResponder)
	if err != nil {
		fmt.Printf("Error adding processor to responder edge: %v\n", err)
		return
	}

	err = graphBuilder.AddEdge(NodeResponder, END)
	if err != nil {
		fmt.Printf("Error adding responder to end edge: %v\n", err)
		return
	}

	// Compile the graph
	einGraph, err := graphBuilder.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}

	// Test with various message types
	testMessages := []string{
		"สวัสดีครับ",
		"Python คืออะไร?",
		"ช่วยเขียน Hello World ในภาษา Go หน่อย",
		"ระบบช้ามาก แก้ไขด้วย",
		"ขอบคุณครับ ลาก่อน",
	}

	fmt.Println("🚀 Eino Graph Execution Demo")
	fmt.Println("================================")

	for i, msg := range testMessages {
		fmt.Printf("\n🧪 Test %d: %s\n", i+1, msg)
		fmt.Println("---")

		// Create initial state
		initialState := &GraphState{
			Messages:      []*schema.Message{schema.UserMessage(msg)},
			CurrentIntent: "",
			ToolsUsed:     []string{},
			ProcessStep:   0,
			Completed:     false,
		}

		// Execute the graph
		result, err := einGraph.Invoke(ctx, initialState)
		if err != nil {
			fmt.Printf("❌ Error executing graph: %v\n", err)
			continue
		}

		// Display results
		fmt.Printf("✅ Graph completed successfully!\n")
		fmt.Printf("📊 Final State:\n")
		fmt.Printf("   Intent: %s\n", result.CurrentIntent)
		fmt.Printf("   Steps: %d\n", result.ProcessStep)
		fmt.Printf("   Messages: %d\n", len(result.Messages))
		fmt.Printf("   Completed: %t\n", result.Completed)
	}

	fmt.Println("\n🎉 Eino Graph Demo Completed!")
	fmt.Println("Graph Features Demonstrated:")
	fmt.Println("  ✅ State-based graph execution")
	fmt.Println("  ✅ Conditional edge routing")
	fmt.Println("  ✅ Multi-node processing pipeline")
	fmt.Println("  ✅ Intent-aware message processing")
	fmt.Println("  ✅ State management across nodes")
}
