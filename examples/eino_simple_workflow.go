package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// WorkflowStep represents each step in our processing workflow
type WorkflowStep struct {
	StepName string         `json:"step_name"`
	Input    map[string]any `json:"input"`
	Output   string         `json:"output"`
	Duration time.Duration  `json:"duration"`
	Success  bool           `json:"success"`
	Error    error          `json:"error,omitempty"`
}

// WorkflowResult represents the complete workflow execution result
type WorkflowResult struct {
	UserMessage     string         `json:"user_message"`
	DetectedIntent  string         `json:"detected_intent"`
	ProcessedResult string         `json:"processed_result"`
	Steps           []WorkflowStep `json:"steps"`
	TotalDuration   time.Duration  `json:"total_duration"`
	Success         bool           `json:"success"`
}

// Step 1: Intent Detection Chain
func CreateIntentDetectionChain(ctx context.Context, model *openai.ChatModel) (compose.Runnable[map[string]any, *schema.Message], error) {
	systemText := `คุณเป็น AI ที่วิเคราะห์ความตั้งใจของผู้ใช้ 

ให้วิเคราะห์ข้อความและจำแนกเป็น intent ดังนี้:
- greeting: การทักทาย เช่น สวัสดี, หวัดดี, hello
- question: การถามคำถาม เช่น อะไร, ทำไม, อย่างไร
- request: การขอความช่วยเหลือ เช่น ช่วย, สอน, แนะนำ
- calculation: การคำนวณ เช่น บวก, ลบ, คูณ, หาร
- goodbye: การลาก่อน เช่น บาย, ลาก่อน, สวัสดี

ตอบเฉพาะชื่อ intent ที่ตรงที่สุด เช่น: greeting, question, request, calculation, goodbye`

	userText := `ข้อความที่ต้องวิเคราะห์: {message}

Intent:`

	// Create template messages
	messages := []schema.MessagesTemplate{
		schema.SystemMessage(systemText),
		schema.UserMessage(userText),
	}

	// Create the template
	template := prompt.FromMessages(schema.FString, messages...)

	// Create and compile the chain
	chain, err := compose.NewChain[map[string]any, *schema.Message]().
		AppendChatTemplate(template).
		AppendChatModel(model).
		Compile(ctx)

	return chain, err
}

// Step 2: Content Processing Chain
func CreateContentProcessingChain(ctx context.Context, model *openai.ChatModel) (compose.Runnable[map[string]any, *schema.Message], error) {
	systemText := `คุณเป็น AI ผู้ช่วยที่ประมวลผลข้อความตาม intent ที่ระบุ

วิธีการประมวลผลตาม intent:
- greeting: ตอบการทักทายอย่างเป็นมิตรและสุภาพ
- question: ตอบคำถามด้วยข้อมูลที่ถูกต้องและครบถ้วน
- request: ให้คำแนะนำและความช่วยเหลือที่เป็นประโยชน์
- calculation: คำนวณและแสดงผลลัพธ์อย่างชัดเจน
- goodbye: ลาก่อนอย่างสุภาพและเป็นมิตร

ตอบในภาษาไทยที่เข้าใจง่ายและเป็นประโยชน์`

	userText := `ข้อความจากผู้ใช้: {message}
Intent ที่ตรวจพบ: {intent}

การตอบสนอง:`

	// Create template messages
	messages := []schema.MessagesTemplate{
		schema.SystemMessage(systemText),
		schema.UserMessage(userText),
	}

	// Create the template
	template := prompt.FromMessages(schema.FString, messages...)

	// Create and compile the chain
	chain, err := compose.NewChain[map[string]any, *schema.Message]().
		AppendChatTemplate(template).
		AppendChatModel(model).
		Compile(ctx)

	return chain, err
}

// Execute workflow with multiple chains
func ExecuteWorkflow(ctx context.Context, intentChain, processingChain compose.Runnable[map[string]any, *schema.Message], userMessage string) (*WorkflowResult, error) {
	startTime := time.Now()

	result := &WorkflowResult{
		UserMessage: userMessage,
		Steps:       []WorkflowStep{},
		Success:     false,
	}

	fmt.Printf("🚀 Starting workflow for: %s\n", userMessage)

	// Step 1: Intent Detection
	step1Start := time.Now()
	intentResponse, err := intentChain.Invoke(ctx, map[string]any{
		"message": userMessage,
	})
	step1Duration := time.Since(step1Start)

	step1 := WorkflowStep{
		StepName: "intent_detection",
		Input:    map[string]any{"message": userMessage},
		Output:   intentResponse.Content,
		Duration: step1Duration,
		Success:  err == nil,
		Error:    err,
	}
	result.Steps = append(result.Steps, step1)

	if err != nil {
		result.TotalDuration = time.Since(startTime)
		return result, fmt.Errorf("intent detection failed: %v", err)
	}

	// Clean up intent response
	detectedIntent := strings.TrimSpace(strings.ToLower(intentResponse.Content))
	result.DetectedIntent = detectedIntent

	fmt.Printf("📊 Step 1 Complete: Intent = %s (%.2fms)\n", detectedIntent, float64(step1Duration.Nanoseconds())/1000000)

	// Step 2: Content Processing
	step2Start := time.Now()
	processResponse, err := processingChain.Invoke(ctx, map[string]any{
		"message": userMessage,
		"intent":  detectedIntent,
	})
	step2Duration := time.Since(step2Start)

	step2 := WorkflowStep{
		StepName: "content_processing",
		Input:    map[string]any{"message": userMessage, "intent": detectedIntent},
		Output:   processResponse.Content,
		Duration: step2Duration,
		Success:  err == nil,
		Error:    err,
	}
	result.Steps = append(result.Steps, step2)

	if err != nil {
		result.TotalDuration = time.Since(startTime)
		return result, fmt.Errorf("content processing failed: %v", err)
	}

	result.ProcessedResult = processResponse.Content
	result.TotalDuration = time.Since(startTime)
	result.Success = true

	fmt.Printf("📝 Step 2 Complete: Response generated (%.2fms)\n", float64(step2Duration.Nanoseconds())/1000000)
	fmt.Printf("✅ Workflow completed successfully (%.2fms total)\n", float64(result.TotalDuration.Nanoseconds())/1000000)

	return result, nil
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

	// Create model configuration
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

	// Create workflow chains
	fmt.Printf("⚙️ Creating Eino workflow chains...\n")

	intentChain, err := CreateIntentDetectionChain(ctx, model)
	if err != nil {
		fmt.Printf("Error creating intent chain: %v\n", err)
		return
	}

	processingChain, err := CreateContentProcessingChain(ctx, model)
	if err != nil {
		fmt.Printf("Error creating processing chain: %v\n", err)
		return
	}

	fmt.Printf("✅ Chains created successfully!\n\n")

	// Test scenarios
	testScenarios := []struct {
		message     string
		description string
	}{
		{"สวัสดีครับ", "Greeting"},
		{"Python คืออะไร?", "Question"},
		{"ช่วยเขียนโปรแกรม Hello World หน่อย", "Help Request"},
		{"คำนวณ 15 + 27", "Calculation"},
		{"ขอบคุณครับ บายครับ", "Goodbye"},
		{"สอนการใช้ Git หน่อยครับ", "Learning Request"},
	}

	fmt.Println("🔄 Eino Simple Workflow Demo")
	fmt.Println("============================")

	for i, scenario := range testScenarios {
		fmt.Printf("\n🧪 Test %d: %s\n", i+1, scenario.description)
		fmt.Printf("Input: %s\n", scenario.message)
		fmt.Println("---")

		// Execute the workflow
		result, err := ExecuteWorkflow(ctx, intentChain, processingChain, scenario.message)

		if err != nil {
			fmt.Printf("❌ Workflow failed: %v\n", err)
			continue
		}

		// Display detailed results
		fmt.Printf("🎯 Final Result:\n")
		fmt.Printf("   Intent: %s\n", result.DetectedIntent)
		fmt.Printf("   Response: %s\n", result.ProcessedResult)
		fmt.Printf("   Steps: %d\n", len(result.Steps))
		fmt.Printf("   Total Time: %.2fms\n", float64(result.TotalDuration.Nanoseconds())/1000000)
		fmt.Printf("   Success: %t\n", result.Success)

		// Show step breakdown
		fmt.Printf("📈 Step Breakdown:\n")
		for j, step := range result.Steps {
			status := "✅"
			if !step.Success {
				status = "❌"
			}
			fmt.Printf("   %d. %s %s (%.2fms)\n",
				j+1,
				step.StepName,
				status,
				float64(step.Duration.Nanoseconds())/1000000)
		}
	}

	fmt.Println("\n🎉 Eino Simple Workflow Demo Completed!")
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("  ✅ Multi-step Eino chain workflows")
	fmt.Println("  ✅ Template-based prompt engineering")
	fmt.Println("  ✅ Intent-driven content processing")
	fmt.Println("  ✅ Performance monitoring and timing")
	fmt.Println("  ✅ Error handling and step tracking")
	fmt.Println("  ✅ Real LLM integration")
	fmt.Println("  ✅ Structured workflow execution")
}
