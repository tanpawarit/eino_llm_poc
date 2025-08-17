package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// main function for running chat model examples
func main() {
	if err := runChatModelExample(); err != nil {
		log.Fatalf("Error running chat model example: %v", err)
	}
}

// ตัวอย่าง Chat Model Node - เชื่อมกับ LLM จริง
func runChatModelExample() error {
	// Load environment
	if err := godotenv.Load(); err != nil {
		// .env file is optional, just log warning
		log.Printf("Warning: .env file not found or couldn't be loaded: %v", err)
	}

	// Create context with timeout for the entire example
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return errors.New("OPENROUTER_API_KEY environment variable is required")
	}

	// Validate API key format
	if len(strings.TrimSpace(apiKey)) < 10 {
		return errors.New("OPENROUTER_API_KEY appears to be invalid (too short)")
	}

	// สร้าง Chat Model
	config := &openai.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: "https://openrouter.ai/api/v1",
		Model:   "openai/gpt-3.5-turbo",
	}

	model, err := openai.NewChatModel(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create chat model: %w", err)
	}

	// === ตัวอย่าง 1: Direct Chat Model Node ===
	fmt.Println("=== Direct Chat Model Node ===")
	if err := runDirectChatModel(ctx, model); err != nil {
		return fmt.Errorf("direct chat model example failed: %w", err)
	}

	// === ตัวอย่าง 2: Multi-Role Chat Model ===
	fmt.Println("\n=== Multi-Role Chat Model ===")
	if err := runMultiRoleChatModel(ctx, model); err != nil {
		return fmt.Errorf("multi-role chat model example failed: %w", err)
	}

	// === ตัวอย่าง 3: Chat Model with Context ===
	fmt.Println("\n=== Chat Model with Context ===")
	if err := runChatModelWithContext(ctx, model); err != nil {
		return fmt.Errorf("contextual chat model example failed: %w", err)
	}

	return nil
}

// validateMessages validates input messages
func validateMessages(messages []*schema.Message) error {
	if len(messages) == 0 {
		return errors.New("messages cannot be empty")
	}

	for i, msg := range messages {
		if msg == nil {
			return fmt.Errorf("message at index %d is nil", i)
		}
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("message at index %d has empty content", i)
		}
		if msg.Role == "" {
			return fmt.Errorf("message at index %d has empty role", i)
		}
	}

	return nil
}

// Direct Chat Model Node
func runDirectChatModel(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[[]*schema.Message, *schema.Message]()

	// Chat Model Node ที่รับ messages และส่งคืน response message
	chatModelNode := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
		// Validate input messages
		if err := validateMessages(messages); err != nil {
			return nil, fmt.Errorf("invalid messages: %w", err)
		}

		fmt.Printf("🤖 Chat Model: Processing %d messages\n", len(messages))
		
		// แสดง messages ที่เข้ามา
		for i, msg := range messages {
			// Truncate long content for display
			content := msg.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("  [%d] %s: %s\n", i+1, msg.Role, content)
		}

		// Create timeout context for LLM call
		llamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		response, err := model.Generate(llamCtx, messages)
		if err != nil {
			return nil, fmt.Errorf("chat model error: %w", err)
		}

		if response == nil {
			return nil, errors.New("received nil response from chat model")
		}

		fmt.Printf("🤖 Response: %s\n", response.Content)
		return response, nil
	})

	// เพิ่ม node
	graph.AddLambdaNode("chat_model", chatModelNode)
	graph.AddEdge(compose.START, "chat_model")
	graph.AddEdge("chat_model", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile graph: %w", err)
	}

	// ทดสอบ
	testMessages := [][]*schema.Message{
		{
			schema.SystemMessage("คุณเป็น AI ผู้ช่วยที่เชี่ยวชาญเรื่องการเขียนโปรแกรม"),
			schema.UserMessage("อธิบาย Eino Graph ให้ฟังหน่อย"),
		},
		{
			schema.UserMessage("Go channel ใช้ยังไง?"),
		},
	}

	for i, messages := range testMessages {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		
		// Create timeout for individual test
		testCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		result, err := runnable.Invoke(testCtx, messages)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		if result == nil {
			fmt.Printf("Error: received nil result\n")
			continue
		}
		
		fmt.Printf("Final Result: %s\n", result.Content)
	}

	return nil
}

// Multi-Role Chat Model
func runMultiRoleChatModel(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Message Formatter - แปลง string เป็น messages สำหรับ role ต่างๆ
	teacherFormatter := compose.InvokableLambda(func(ctx context.Context, input string) ([]*schema.Message, error) {
		messages := []*schema.Message{
			schema.SystemMessage("คุณเป็นครูสอนคอมพิวเตอร์ที่เป็นมิตร อธิบายให้เข้าใจง่ายและให้ตัวอย่าง"),
			schema.UserMessage(input),
		}
		fmt.Printf("👩‍🏫 Teacher Role: Prepared messages for teaching\n")
		return messages, nil
	})

	codeReviewerFormatter := compose.InvokableLambda(func(ctx context.Context, input string) ([]*schema.Message, error) {
		messages := []*schema.Message{
			schema.SystemMessage("คุณเป็น Senior Developer ที่ review code อย่างละเอียด ชี้จุดที่ต้องปรับปรุงและให้คำแนะนำ"),
			schema.UserMessage(input),
		}
		fmt.Printf("👨‍💻 Code Reviewer Role: Prepared messages for code review\n")
		return messages, nil
	})

	architectFormatter := compose.InvokableLambda(func(ctx context.Context, input string) ([]*schema.Message, error) {
		messages := []*schema.Message{
			schema.SystemMessage("คุณเป็น Software Architect ที่มีประสบการณ์สูง มองภาพรวมของระบบและให้คำแนะนำเชิงสถาปัตยกรรม"),
			schema.UserMessage(input),
		}
		fmt.Printf("🏗️ Architect Role: Prepared messages for architecture guidance\n")
		return messages, nil
	})

	// Chat Model Nodes สำหรับ role ต่างๆ
	chatModelNode := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		// Validate input
		if err := validateMessages(messages); err != nil {
			return "", fmt.Errorf("invalid messages: %w", err)
		}

		// Create timeout for LLM call
		llamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		response, err := model.Generate(llamCtx, messages)
		if err != nil {
			return "", fmt.Errorf("chat model generation failed: %w", err)
		}

		if response == nil {
			return "", errors.New("received nil response from chat model")
		}

		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("teacher_formatter", teacherFormatter)
	graph.AddLambdaNode("reviewer_formatter", codeReviewerFormatter)
	graph.AddLambdaNode("architect_formatter", architectFormatter)
	graph.AddLambdaNode("teacher_chat", chatModelNode)
	graph.AddLambdaNode("reviewer_chat", chatModelNode)
	graph.AddLambdaNode("architect_chat", chatModelNode)

	// สร้าง branches สำหรับ role ต่างๆ
	graph.AddEdge(compose.START, "teacher_formatter")
	graph.AddEdge("teacher_formatter", "teacher_chat")
	graph.AddEdge("teacher_chat", compose.END)

	// Compile และทดสอบแต่ละ role
	teacherGraph, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile teacher graph: %w", err)
	}

	// ทดสอบ Teacher Role
	fmt.Printf("\n--- Teacher Mode ---\n")
	testCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	result, err := teacherGraph.Invoke(testCtx, "Goroutine คืออะไร และใช้งานยังไง?")
	cancel()
	if err != nil {
		fmt.Printf("Teacher mode error: %v\n", err)
	} else {
		fmt.Printf("👩‍🏫 Teacher: %s\n", result)
	}

	// สร้าง graph ใหม่สำหรับ Code Reviewer
	reviewerGraph := compose.NewGraph[string, string]()
	reviewerGraph.AddLambdaNode("reviewer_formatter", codeReviewerFormatter)
	reviewerGraph.AddLambdaNode("reviewer_chat", chatModelNode)
	reviewerGraph.AddEdge(compose.START, "reviewer_formatter")
	reviewerGraph.AddEdge("reviewer_formatter", "reviewer_chat")
	reviewerGraph.AddEdge("reviewer_chat", compose.END)

	reviewerRunnable, err := reviewerGraph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile reviewer graph: %w", err)
	}
	fmt.Printf("\n--- Code Reviewer Mode ---\n")
	testCtx, cancel = context.WithTimeout(ctx, 45*time.Second)
	result, err = reviewerRunnable.Invoke(testCtx, "func process(data []string) { for i := 0; i < len(data); i++ { fmt.Println(data[i]) } }")
	cancel()
	if err != nil {
		fmt.Printf("Code reviewer mode error: %v\n", err)
	} else {
		fmt.Printf("👨‍💻 Code Reviewer: %s\n", result)
	}

	// สร้าง graph ใหม่สำหรับ Architect
	architectGraph := compose.NewGraph[string, string]()
	architectGraph.AddLambdaNode("architect_formatter", architectFormatter)
	architectGraph.AddLambdaNode("architect_chat", chatModelNode)
	architectGraph.AddEdge(compose.START, "architect_formatter")
	architectGraph.AddEdge("architect_formatter", "architect_chat")
	architectGraph.AddEdge("architect_chat", compose.END)

	architectRunnable, err := architectGraph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile architect graph: %w", err)
	}
	fmt.Printf("\n--- Software Architect Mode ---\n")
	testCtx, cancel = context.WithTimeout(ctx, 45*time.Second)
	result, err = architectRunnable.Invoke(testCtx, "ออกแบบ microservices สำหรับระบบ e-commerce")
	cancel()
	if err != nil {
		fmt.Printf("Architect mode error: %v\n", err)
	} else {
		fmt.Printf("🏗️ Architect: %s\n", result)
	}

	return nil
}

// validateContextInput validates context input parameters
func validateContextInput(input map[string]interface{}) error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	question, ok := input["question"].(string)
	if !ok || strings.TrimSpace(question) == "" {
		return errors.New("question is required and must be a non-empty string")
	}

	_, ok = input["project_info"].(string)
	if !ok {
		return errors.New("project_info is required and must be a string")
	}

	userLevel, ok := input["user_level"].(string)
	if !ok || strings.TrimSpace(userLevel) == "" {
		return errors.New("user_level is required and must be a non-empty string")
	}

	// Validate user level
	validLevels := map[string]bool{"beginner": true, "intermediate": true, "expert": true}
	if !validLevels[userLevel] {
		return fmt.Errorf("user_level must be one of: beginner, intermediate, expert, got: %s", userLevel)
	}

	return nil
}

// Chat Model with Context
func runChatModelWithContext(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[map[string]interface{}, string]()

	// Context Builder - สร้าง context จากข้อมูลต่างๆ
	contextBuilder := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) ([]*schema.Message, error) {
		// Validate input first
		if err := validateContextInput(input); err != nil {
			return nil, fmt.Errorf("invalid context input: %w", err)
		}

		userQuestion := input["question"].(string)
		projectInfo := input["project_info"].(string)
		userLevel := input["user_level"].(string)

		var systemPrompt string
		switch userLevel {
		case "beginner":
			systemPrompt = fmt.Sprintf(`คุณเป็น AI ผู้ช่วยสำหรับมือใหม่ 
Project Context: %s

ตอบคำถามให้เข้าใจง่าย ใช้ภาษาธรรมดา และให้ตัวอย่างเสมอ`, projectInfo)
		case "intermediate":
			systemPrompt = fmt.Sprintf(`คุณเป็น AI ผู้ช่วยสำหรับคนที่มีประสบการณ์ปานกลาง
Project Context: %s

ให้คำตอบที่มีรายละเอียดเทคนิค และแนะนำ best practices`, projectInfo)
		case "expert":
			systemPrompt = fmt.Sprintf(`คุณเป็น AI ผู้ช่วยสำหรับผู้เชี่ยวชาญ
Project Context: %s

ให้คำตอบที่เข้าสู่รายละเอียดลึก พร้อมข้อควรระวังและการปรับแต่งขั้นสูง`, projectInfo)
		default:
			systemPrompt = fmt.Sprintf(`Project Context: %s`, projectInfo)
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userQuestion),
		}

		fmt.Printf("🔧 Context Builder: Built context for %s level user\n", userLevel)
		return messages, nil
	})

	// Chat Model Node
	contextualChatModel := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		// Validate messages
		if err := validateMessages(messages); err != nil {
			return "", fmt.Errorf("invalid messages for contextual chat: %w", err)
		}

		fmt.Printf("🤖 Contextual Chat Model: Processing with system context\n")
		
		// Create timeout for LLM call
		llamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		response, err := model.Generate(llamCtx, messages)
		if err != nil {
			return "", fmt.Errorf("contextual chat model generation failed: %w", err)
		}

		if response == nil {
			return "", errors.New("received nil response from contextual chat model")
		}

		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("context_builder", contextBuilder)
	graph.AddLambdaNode("contextual_chat", contextualChatModel)

	// เชื่อม edges
	graph.AddEdge(compose.START, "context_builder")
	graph.AddEdge("context_builder", "contextual_chat")
	graph.AddEdge("contextual_chat", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile contextual graph: %w", err)
	}

	// ทดสอบกับ context ต่างๆ
	testCases := []map[string]interface{}{
		{
			"question":     "Eino Graph ใช้ parallel processing ยังไง?",
			"project_info": "Go microservices project using Eino for workflow orchestration",
			"user_level":   "beginner",
		},
		{
			"question":     "วิธีการ optimize performance ของ Graph",
			"project_info": "High-traffic API gateway with complex routing logic",
			"user_level":   "intermediate",
		},
		{
			"question":     "Custom node implementation patterns และ memory management",
			"project_info": "Enterprise-grade workflow engine with custom extensions",
			"user_level":   "expert",
		},
	}

	for i, testCase := range testCases {
		fmt.Printf("\n--- Context Test %d (%s level) ---\n", i+1, testCase["user_level"])
		fmt.Printf("Question: %s\n", testCase["question"])
		fmt.Printf("Context: %s\n", testCase["project_info"])

		// Create timeout for individual test
		testCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		result, err := runnable.Invoke(testCtx, testCase)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\nResponse:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}