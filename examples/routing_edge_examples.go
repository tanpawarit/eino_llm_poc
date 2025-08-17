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

// UserRequest represents a user's request with metadata
type UserRequest struct {
	Message    string            `json:"message"`
	UserType   string            `json:"user_type"`   // beginner, intermediate, expert
	Topic      string            `json:"topic"`       // programming, math, general
	Language   string            `json:"language"`    // thai, english
	Priority   string            `json:"priority"`    // low, normal, high, urgent
	Metadata   map[string]string `json:"metadata"`
}

// RoutingDecision represents routing logic output
type RoutingDecision struct {
	NextNode   string            `json:"next_node"`
	Confidence float64           `json:"confidence"`
	Reasoning  string            `json:"reasoning"`
	Metadata   map[string]string `json:"metadata"`
}

// CollectedResult represents aggregated results from multiple nodes
type CollectedResult struct {
	Results    []string          `json:"results"`
	Sources    []string          `json:"sources"`
	Confidence float64           `json:"confidence"`
	Summary    string            `json:"summary"`
	Metadata   map[string]string `json:"metadata"`
}

// main function for routing and edge examples
func main() {
	if err := runRoutingEdgeExamples(); err != nil {
		log.Fatalf("Error running routing edge examples: %v", err)
	}
}

// ตัวอย่าง Router, Collector, และ Broadcaster Nodes พร้อม Advanced Edges
func runRoutingEdgeExamples() error {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or couldn't be loaded: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return errors.New("OPENROUTER_API_KEY environment variable is required")
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

	// === ตัวอย่าง 1: Router Node with Conditional Edges ===
	fmt.Println("=== Router Node with Conditional Edges ===")
	if err := runConditionalRouterExample(ctx, model); err != nil {
		return fmt.Errorf("conditional router example failed: %w", err)
	}

	// === ตัวอย่าง 2: Collector Node (Fan-in Pattern) ===
	fmt.Println("\n=== Collector Node (Fan-in Pattern) ===")
	if err := runCollectorExample(ctx, model); err != nil {
		return fmt.Errorf("collector example failed: %w", err)
	}

	// === ตัวอย่าง 3: Broadcaster Node (Fan-out Pattern) ===
	fmt.Println("\n=== Broadcaster Node (Fan-out Pattern) ===")
	if err := runBroadcasterExample(ctx, model); err != nil {
		return fmt.Errorf("broadcaster example failed: %w", err)
	}

	return nil
}

// Router Node with Conditional Edges
func runConditionalRouterExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[UserRequest, string]()

	// Request Analyzer - วิเคราะห์คำขอของผู้ใช้
	requestAnalyzer := compose.InvokableLambda(func(ctx context.Context, request UserRequest) (RoutingDecision, error) {
		fmt.Printf("🔍 Request Analyzer: Analyzing request from %s user\n", request.UserType)
		fmt.Printf("  Message: %s\n", request.Message)
		fmt.Printf("  Topic: %s, Language: %s, Priority: %s\n", request.Topic, request.Language, request.Priority)

		// Smart routing logic
		var nextNode string
		var confidence float64
		var reasoning string

		messageLower := strings.ToLower(request.Message)

		// Topic-based routing
		switch request.Topic {
		case "programming":
			if strings.Contains(messageLower, "bug") || strings.Contains(messageLower, "error") || strings.Contains(messageLower, "debug") {
				nextNode = "debug_specialist"
				confidence = 0.9
				reasoning = "Detected debugging/error-related programming question"
			} else if strings.Contains(messageLower, "performance") || strings.Contains(messageLower, "optimize") {
				nextNode = "performance_specialist"
				confidence = 0.85
				reasoning = "Detected performance optimization question"
			} else {
				nextNode = "programming_assistant"
				confidence = 0.8
				reasoning = "General programming question"
			}
		case "math":
			if strings.Contains(messageLower, "calculate") || strings.Contains(messageLower, "คำนวณ") {
				nextNode = "math_calculator"
				confidence = 0.95
				reasoning = "Detected calculation request"
			} else {
				nextNode = "math_tutor"
				confidence = 0.8
				reasoning = "Math learning/explanation request"
			}
		default:
			// Priority-based routing for general topics
			switch request.Priority {
			case "urgent":
				nextNode = "priority_handler"
				confidence = 0.9
				reasoning = "High priority request needs immediate attention"
			case "high":
				nextNode = "senior_assistant"
				confidence = 0.8
				reasoning = "High priority request"
			default:
				nextNode = "general_assistant"
				confidence = 0.7
				reasoning = "Standard general request"
			}
		}

		// User level adjustment
		if request.UserType == "beginner" && (nextNode == "debug_specialist" || nextNode == "performance_specialist") {
			nextNode = "beginner_programming_helper"
			reasoning = "Adjusted for beginner level user"
		}

		decision := RoutingDecision{
			NextNode:   nextNode,
			Confidence: confidence,
			Reasoning:  reasoning,
			Metadata: map[string]string{
				"user_type": request.UserType,
				"topic":     request.Topic,
				"priority":  request.Priority,
			},
		}

		fmt.Printf("🎯 Routing Decision: %s (confidence: %.2f)\n", nextNode, confidence)
		fmt.Printf("  Reasoning: %s\n", reasoning)

		return decision, nil
	})

	// Different specialist nodes
	debugSpecialist := compose.InvokableLambda(func(ctx context.Context, request UserRequest) (string, error) {
		fmt.Printf("🐛 Debug Specialist: Handling debug request\n")
		
		systemPrompt := `คุณเป็นผู้เชี่ยวชาญด้าน debugging และการแก้ไขปัญหาโค้ด
ให้วิเคราะห์ปัญหาอย่างเป็นระบบ และแนะนำวิธีการแก้ไขที่ชัดเจน`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(request.Message),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("debug specialist generation failed: %w", err)
		}

		return fmt.Sprintf("🐛 Debug Specialist Analysis:\n%s", response.Content), nil
	})

	performanceSpecialist := compose.InvokableLambda(func(ctx context.Context, request UserRequest) (string, error) {
		fmt.Printf("⚡ Performance Specialist: Handling optimization request\n")
		
		systemPrompt := `คุณเป็นผู้เชี่ยวชาญด้าน performance optimization
ให้วิเคราะห์และแนะนำวิธีการปรับปรุงประสิทธิภาพ พร้อมวัดผลที่ชัดเจน`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(request.Message),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("performance specialist generation failed: %w", err)
		}

		return fmt.Sprintf("⚡ Performance Specialist Analysis:\n%s", response.Content), nil
	})

	mathCalculator := compose.InvokableLambda(func(ctx context.Context, request UserRequest) (string, error) {
		fmt.Printf("🧮 Math Calculator: Processing calculation\n")
		
		systemPrompt := `คุณเป็นเครื่องคิดเลขที่แม่นยำ ให้คำนวณและแสดงขั้นตอนอย่างชัดเจน`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(request.Message),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("math calculator generation failed: %w", err)
		}

		return fmt.Sprintf("🧮 Math Calculator Result:\n%s", response.Content), nil
	})

	generalAssistant := compose.InvokableLambda(func(ctx context.Context, request UserRequest) (string, error) {
		fmt.Printf("🤖 General Assistant: Handling general request\n")
		
		var systemPrompt string
		switch request.UserType {
		case "beginner":
			systemPrompt = "คุณเป็น AI ผู้ช่วยที่เป็นมิตร อธิบายให้เข้าใจง่ายสำหรับมือใหม่"
		case "expert":
			systemPrompt = "คุณเป็น AI ผู้ช่วยที่ให้คำตอบเชิงลึกสำหรับผู้เชี่ยวชาญ"
		default:
			systemPrompt = "คุณเป็น AI ผู้ช่วยที่ให้คำตอบที่สมดุลและเป็นประโยชน์"
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(request.Message),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("general assistant generation failed: %w", err)
		}

		return fmt.Sprintf("🤖 General Assistant Response:\n%s", response.Content), nil
	})

	// Router Logic Node - ใช้ routing decision เพื่อส่งต่อ
	routerLogic := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (string, error) {
		request, ok := input["request"].(UserRequest)
		if !ok {
			return "", errors.New("request not found in input")
		}

		decision, ok := input["decision"].(RoutingDecision)
		if !ok {
			return "", errors.New("routing decision not found in input")
		}

		fmt.Printf("🚦 Router Logic: Routing to %s\n", decision.NextNode)

		// Execute appropriate specialist
		switch decision.NextNode {
		case "debug_specialist":
			return debugSpecialist.Invoke(ctx, request)
		case "performance_specialist":
			return performanceSpecialist.Invoke(ctx, request)
		case "math_calculator":
			return mathCalculator.Invoke(ctx, request)
		default:
			return generalAssistant.Invoke(ctx, request)
		}
	})

	// Combiner node to prepare data for router
	combiner := compose.InvokableLambda(func(ctx context.Context, request UserRequest) (map[string]interface{}, error) {
		// Analyze request first
		decision, err := requestAnalyzer.Invoke(ctx, request)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"request":  request,
			"decision": decision,
		}, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("combiner", combiner)
	graph.AddLambdaNode("router_logic", routerLogic)

	// เชื่อม edges
	graph.AddEdge(compose.START, "combiner")
	graph.AddEdge("combiner", "router_logic")
	graph.AddEdge("router_logic", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile conditional router graph: %w", err)
	}

	// ทดสอบ conditional routing
	testRequests := []UserRequest{
		{
			Message:  "My Go application has a memory leak, how do I debug it?",
			UserType: "intermediate",
			Topic:    "programming",
			Language: "english",
			Priority: "high",
		},
		{
			Message:  "How to optimize database queries?",
			UserType: "expert",
			Topic:    "programming",
			Language: "english",
			Priority: "normal",
		},
		{
			Message:  "Calculate 15 * 25 + 100",
			UserType: "beginner",
			Topic:    "math",
			Language: "english",
			Priority: "normal",
		},
		{
			Message:  "What is machine learning?",
			UserType: "beginner",
			Topic:    "general",
			Language: "english",
			Priority: "low",
		},
		{
			Message:  "URGENT: Production server is down!",
			UserType: "expert",
			Topic:    "general",
			Language: "english",
			Priority: "urgent",
		},
	}

	for i, request := range testRequests {
		fmt.Printf("\n--- Conditional Router Test %d ---\n", i+1)
		fmt.Printf("Request: %+v\n", request)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, request)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Response:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// Collector Node (Fan-in Pattern)
func runCollectorExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Question Broadcaster - แยกคำถามไปหลาย specialists
	questionBroadcaster := compose.InvokableLambda(func(ctx context.Context, question string) (map[string]string, error) {
		fmt.Printf("📡 Question Broadcaster: Broadcasting question to specialists\n")
		fmt.Printf("  Question: %s\n", question)

		// Broadcast to multiple specialists
		return map[string]string{
			"technical":    question,
			"practical":    question,
			"theoretical":  question,
		}, nil
	})

	// Technical Specialist
	technicalSpecialist := compose.InvokableLambda(func(ctx context.Context, question string) (string, error) {
		fmt.Printf("🔧 Technical Specialist: Analyzing technical aspects\n")
		
		systemPrompt := `คุณเป็นผู้เชี่ยวชาญด้านเทคนิค มุ่งเน้นรายละเอียดการทำงาน implementation และ best practices`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(question),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("technical specialist generation failed: %w", err)
		}

		return fmt.Sprintf("Technical Analysis: %s", response.Content), nil
	})

	// Practical Specialist
	practicalSpecialist := compose.InvokableLambda(func(ctx context.Context, question string) (string, error) {
		fmt.Printf("💼 Practical Specialist: Focusing on real-world application\n")
		
		systemPrompt := `คุณเป็นผู้เชี่ยวชาญด้านการประยุกต์ใช้จริง มุ่งเน้นตัวอย่างและการใช้งานในโลกจริง`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(question),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("practical specialist generation failed: %w", err)
		}

		return fmt.Sprintf("Practical Perspective: %s", response.Content), nil
	})

	// Theoretical Specialist
	theoreticalSpecialist := compose.InvokableLambda(func(ctx context.Context, question string) (string, error) {
		fmt.Printf("📚 Theoretical Specialist: Explaining concepts and theory\n")
		
		systemPrompt := `คุณเป็นผู้เชี่ยวชาญด้านทฤษฎี มุ่งเน้นหลักการ แนวคิด และพื้นฐานทางวิชาการ`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(question),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("theoretical specialist generation failed: %w", err)
		}

		return fmt.Sprintf("Theoretical Foundation: %s", response.Content), nil
	})

	// Result Collector - รวบรวมผลลัพธ์จากทุก specialists
	resultCollector := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (CollectedResult, error) {
		fmt.Printf("🗂️ Result Collector: Aggregating specialist responses\n")
		
		// Extract results from different specialists
		question, _ := input["question"].(string)
		
		// Collect all results
		var allResults []string
		var sources []string

		// Get results from each specialist
		if tech, err := technicalSpecialist.Invoke(ctx, question); err == nil {
			allResults = append(allResults, tech)
			sources = append(sources, "Technical Specialist")
		}

		if practical, err := practicalSpecialist.Invoke(ctx, question); err == nil {
			allResults = append(allResults, practical)
			sources = append(sources, "Practical Specialist")
		}

		if theoretical, err := theoreticalSpecialist.Invoke(ctx, question); err == nil {
			allResults = append(allResults, theoretical)
			sources = append(sources, "Theoretical Specialist")
		}

		// Calculate confidence based on consensus
		confidence := 0.8 // Base confidence
		if len(allResults) >= 3 {
			confidence = 0.95 // High confidence with multiple perspectives
		}

		collected := CollectedResult{
			Results:    allResults,
			Sources:    sources,
			Confidence: confidence,
			Summary:    fmt.Sprintf("Collected %d perspectives on: %s", len(allResults), question),
			Metadata: map[string]string{
				"collection_method": "parallel_specialists",
				"num_sources":       fmt.Sprintf("%d", len(sources)),
			},
		}

		fmt.Printf("  Collected %d results from %d sources\n", len(allResults), len(sources))
		return collected, nil
	})

	// Final Synthesizer - สังเคราะห์คำตอบสุดท้าย
	finalSynthesizer := compose.InvokableLambda(func(ctx context.Context, collected CollectedResult) (string, error) {
		fmt.Printf("🧠 Final Synthesizer: Creating comprehensive response\n")
		
		// Combine all perspectives into comprehensive answer
		combinedInput := fmt.Sprintf("Question: %s\n\nPerspectives from specialists:\n", collected.Summary)
		for i, result := range collected.Results {
			combinedInput += fmt.Sprintf("\n%s:\n%s\n", collected.Sources[i], result)
		}

		systemPrompt := `คุณเป็นผู้เชี่ยวชาญในการสังเคราะห์ข้อมูลจากหลายมุมมอง
รวบรวมและสังเคราะห์ข้อมูลจากผู้เชี่ยวชาญต่างๆ เป็นคำตอบที่ครอบคลุมและสมดุล
ให้ครอบคลุมทั้งด้านเทคนิค การประยุกต์ใช้ และทฤษฎี`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(combinedInput),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("final synthesizer generation failed: %w", err)
		}

		return fmt.Sprintf("🎯 Comprehensive Answer (Confidence: %.2f):\n%s\n\nSources: %s", 
			collected.Confidence, response.Content, strings.Join(collected.Sources, ", ")), nil
	})

	// Data flow nodes
	dataFlow1 := compose.InvokableLambda(func(ctx context.Context, question string) (map[string]interface{}, error) {
		collected, err := resultCollector.Invoke(ctx, map[string]interface{}{"question": question})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"collected": collected}, nil
	})

	extractCollected := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (CollectedResult, error) {
		collected, ok := input["collected"].(CollectedResult)
		if !ok {
			return CollectedResult{}, errors.New("collected result not found")
		}
		return collected, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("data_flow", dataFlow1)
	graph.AddLambdaNode("extract_collected", extractCollected)
	graph.AddLambdaNode("final_synthesizer", finalSynthesizer)

	// เชื่อม edges
	graph.AddEdge(compose.START, "data_flow")
	graph.AddEdge("data_flow", "extract_collected")
	graph.AddEdge("extract_collected", "final_synthesizer")
	graph.AddEdge("final_synthesizer", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile collector graph: %w", err)
	}

	// ทดสอบ collector pattern
	testQuestions := []string{
		"Explain Go channels and goroutines",
		"How does Docker containerization work?",
		"What are the principles of REST API design?",
	}

	for i, question := range testQuestions {
		fmt.Printf("\n--- Collector Test %d ---\n", i+1)
		fmt.Printf("Question: %s\n", question)
		
		testCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		result, err := runnable.Invoke(testCtx, question)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n%s\n", result)
		fmt.Println(strings.Repeat("=", 80))
	}

	return nil
}

// Broadcaster Node (Fan-out Pattern)
func runBroadcasterExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Content Broadcaster - ส่งข้อมูลไปหลาย processors พร้อมกัน
	contentBroadcaster := compose.InvokableLambda(func(ctx context.Context, content string) (map[string]interface{}, error) {
		fmt.Printf("📡 Content Broadcaster: Broadcasting to multiple processors\n")
		fmt.Printf("  Content length: %d characters\n", len(content))

		// Broadcast to different processors simultaneously
		return map[string]interface{}{
			"content":           content,
			"summary_request":   content,
			"translate_request": content,
			"analyze_request":   content,
		}, nil
	})

	// Summary Processor
	summaryProcessor := compose.InvokableLambda(func(ctx context.Context, content string) (string, error) {
		fmt.Printf("📝 Summary Processor: Creating summary\n")
		
		systemPrompt := `คุณเป็นผู้เชี่ยวชาญในการสรุปเนื้อหา ให้สรุปใจความสำคัญอย่างกระชับและชัดเจน`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(fmt.Sprintf("สรุปเนื้อหานี้: %s", content)),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("summary processor generation failed: %w", err)
		}

		return fmt.Sprintf("📝 Summary: %s", response.Content), nil
	})

	// Translation Processor
	translationProcessor := compose.InvokableLambda(func(ctx context.Context, content string) (string, error) {
		fmt.Printf("🌐 Translation Processor: Translating content\n")
		
		systemPrompt := `คุณเป็นผู้เชี่ยวชาญการแปลภาษา ให้แปลเนื้อหาเป็นภาษาไทยและภาษาอังกฤษ`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(fmt.Sprintf("แปลเนื้อหานี้เป็นทั้งไทยและอังกฤษ: %s", content)),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("translation processor generation failed: %w", err)
		}

		return fmt.Sprintf("🌐 Translation: %s", response.Content), nil
	})

	// Analysis Processor
	analysisProcessor := compose.InvokableLambda(func(ctx context.Context, content string) (string, error) {
		fmt.Printf("🔍 Analysis Processor: Analyzing content\n")
		
		systemPrompt := `คุณเป็นนักวิเคราะห์เนื้อหา ให้วิเคราะห์โทนเสียง ความซับซ้อน และจุดเด่นของเนื้อหา`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(fmt.Sprintf("วิเคราะห์เนื้อหานี้: %s", content)),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("analysis processor generation failed: %w", err)
		}

		return fmt.Sprintf("🔍 Analysis: %s", response.Content), nil
	})

	// Result Aggregator - รวบรวมผลลัพธ์จากทุก processors
	resultAggregator := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (string, error) {
		fmt.Printf("🗂️ Result Aggregator: Collecting all processor results\n")
		
		content, _ := input["content"].(string)
		
		// Process content with all processors in parallel
		var results []string
		
		// Get summary
		if summary, err := summaryProcessor.Invoke(ctx, content); err == nil {
			results = append(results, summary)
		} else {
			results = append(results, fmt.Sprintf("📝 Summary: Error - %v", err))
		}

		// Get translation
		if translation, err := translationProcessor.Invoke(ctx, content); err == nil {
			results = append(results, translation)
		} else {
			results = append(results, fmt.Sprintf("🌐 Translation: Error - %v", err))
		}

		// Get analysis
		if analysis, err := analysisProcessor.Invoke(ctx, content); err == nil {
			results = append(results, analysis)
		} else {
			results = append(results, fmt.Sprintf("🔍 Analysis: Error - %v", err))
		}

		// Combine all results
		finalResult := fmt.Sprintf("📡 Broadcast Processing Results:\n\nOriginal Content: %s\n\n%s", 
			content, strings.Join(results, "\n\n"))

		fmt.Printf("  Aggregated %d processor results\n", len(results))
		return finalResult, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("content_broadcaster", contentBroadcaster)
	graph.AddLambdaNode("result_aggregator", resultAggregator)

	// เชื่อม edges
	graph.AddEdge(compose.START, "content_broadcaster")
	graph.AddEdge("content_broadcaster", "result_aggregator")
	graph.AddEdge("result_aggregator", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile broadcaster graph: %w", err)
	}

	// ทดสอบ broadcaster pattern
	testContents := []string{
		"Go เป็นภาษาโปรแกรมมิ่งที่พัฒนาโดย Google มีความเร็วสูงและรองรับ concurrency ได้ดี",
		"Machine learning is a subset of artificial intelligence that enables computers to learn without being explicitly programmed.",
		"Docker ช่วยให้นักพัฒนาสามารถแพ็คแอปพลิเคชันและ dependencies เข้าไปใน container ที่สามารถรันได้ในสภาพแวดล้อมต่างๆ",
	}

	for i, content := range testContents {
		fmt.Printf("\n--- Broadcaster Test %d ---\n", i+1)
		fmt.Printf("Content: %s\n", content)
		
		testCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		result, err := runnable.Invoke(testCtx, content)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n%s\n", result)
		fmt.Println(strings.Repeat("=", 80))
	}

	return nil
}