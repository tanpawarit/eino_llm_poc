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
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// MockRetrieverImpl implements the retriever interface for RAG
type MockRetrieverImpl struct {
	documents []schema.Document
}

// NewMockRetrieverImpl creates a new mock retriever
func NewMockRetrieverImpl() *MockRetrieverImpl {
	return &MockRetrieverImpl{
		documents: []schema.Document{
			{
				PageContent: "Eino Graph เป็น framework สำหรับสร้าง workflow ใน Go ที่มีความยืดหยุ่นสูง",
				Metadata: map[string]any{
					"source": "eino_docs",
					"topic":  "framework",
				},
			},
			{
				PageContent: "Go channels เป็นเครื่องมือสำหรับการสื่อสารระหว่าง goroutines อย่างปลอดภัย",
				Metadata: map[string]any{
					"source": "go_docs",
					"topic":  "concurrency",
				},
			},
			{
				PageContent: "Docker containers ช่วยให้แอปพลิเคชันทำงานได้อย่างสม่ำเสมอในทุกสภาพแวดล้อม",
				Metadata: map[string]any{
					"source": "docker_docs",
					"topic":  "containerization",
				},
			},
		},
	}
}

// Retrieve implements the retriever interface
func (r *MockRetrieverImpl) Retrieve(ctx context.Context, query string, opts ...any) ([]schema.Document, error) {
	fmt.Printf("🔍 Native Retriever: Searching for '%s'\n", query)
	
	var results []schema.Document
	queryLower := strings.ToLower(query)
	
	for _, doc := range r.documents {
		if strings.Contains(strings.ToLower(doc.PageContent), queryLower) {
			results = append(results, doc)
		}
	}
	
	fmt.Printf("  Found %d documents\n", len(results))
	return results, nil
}

// GetType returns the retriever type
func (r *MockRetrieverImpl) GetType() string {
	return "mock_retriever"
}

// MockToolImpl implements a simple tool
type MockToolImpl struct {
	name        string
	description string
}

// Call implements the tool interface
func (t *MockToolImpl) Call(ctx context.Context, args string) (string, error) {
	fmt.Printf("🔧 Native Tool '%s': Called with args '%s'\n", t.name, args)
	
	switch t.name {
	case "calculator":
		return fmt.Sprintf("Calculator result for: %s", args), nil
	case "weather":
		return fmt.Sprintf("Weather info for: %s", args), nil
	case "translator":
		return fmt.Sprintf("Translation of: %s", args), nil
	default:
		return fmt.Sprintf("Tool %s processed: %s", t.name, args), nil
	}
}

// GetName returns the tool name
func (t *MockToolImpl) GetName() string {
	return t.name
}

// GetDescription returns the tool description
func (t *MockToolImpl) GetDescription() string {
	return t.description
}

// GetType returns the tool type
func (t *MockToolImpl) GetType() string {
	return "mock_tool"
}

// main function for native Eino nodes examples
func main() {
	if err := runNativeEinoNodesExample(); err != nil {
		log.Fatalf("Error running native Eino nodes example: %v", err)
	}
}

// ตัวอย่าง Native Eino Nodes
func runNativeEinoNodesExample() error {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or couldn't be loaded: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENROUTER_API_KEY environment variable is required")
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

	// === ตัวอย่าง 1: AddChatModelNode ===
	fmt.Println("=== Native AddChatModelNode ===")
	if err := runChatModelNodeExample(ctx, model); err != nil {
		return fmt.Errorf("chat model node example failed: %w", err)
	}

	// === ตัวอย่าง 2: AddRetrieverNode ===
	fmt.Println("\n=== Native AddRetrieverNode ===")
	if err := runRetrieverNodeExample(ctx, model); err != nil {
		return fmt.Errorf("retriever node example failed: %w", err)
	}

	// === ตัวอย่าง 3: AddToolsNode ===
	fmt.Println("\n=== Native AddToolsNode ===")
	if err := runToolsNodeExample(ctx, model); err != nil {
		return fmt.Errorf("tools node example failed: %w", err)
	}

	// === ตัวอย่าง 4: AddPassthroughNode ===
	fmt.Println("\n=== Native AddPassthroughNode ===")
	if err := runPassthroughNodeExample(ctx, model); err != nil {
		return fmt.Errorf("passthrough node example failed: %w", err)
	}

	// === ตัวอย่าง 5: AddGraphNode (Sub-graph) ===
	fmt.Println("\n=== Native AddGraphNode (Sub-graph) ===")
	if err := runSubGraphExample(ctx, model); err != nil {
		return fmt.Errorf("sub-graph example failed: %w", err)
	}

	return nil
}

// AddChatModelNode Example
func runChatModelNodeExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[[]*schema.Message, *schema.Message]()

	// Input Validator
	inputValidator := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error) {
		fmt.Printf("✅ Input Validator: Validating %d messages\n", len(messages))
		
		if len(messages) == 0 {
			return nil, fmt.Errorf("no messages provided")
		}
		
		for i, msg := range messages {
			if msg == nil {
				return nil, fmt.Errorf("message %d is nil", i)
			}
			if strings.TrimSpace(msg.Content) == "" {
				return nil, fmt.Errorf("message %d has empty content", i)
			}
		}
		
		fmt.Printf("  All messages validated successfully\n")
		return messages, nil
	})

	// Output Formatter
	outputFormatter := compose.InvokableLambda(func(ctx context.Context, response *schema.Message) (string, error) {
		fmt.Printf("📋 Output Formatter: Formatting response\n")
		
		if response == nil {
			return "", fmt.Errorf("response is nil")
		}
		
		formatted := fmt.Sprintf("🤖 AI Response:\n%s\n\n📊 Metadata:\n- Role: %s\n- Content Length: %d chars", 
			response.Content, response.Role, len(response.Content))
		
		fmt.Printf("  Response formatted successfully\n")
		return formatted, nil
	})

	// เพิ่ม nodes - ใช้ native AddChatModelNode
	if err := graph.AddLambdaNode("input_validator", inputValidator); err != nil {
		return fmt.Errorf("failed to add input validator: %w", err)
	}
	
	if err := graph.AddChatModelNode("chat_model", model); err != nil {
		return fmt.Errorf("failed to add chat model node: %w", err)
	}
	
	if err := graph.AddLambdaNode("output_formatter", outputFormatter); err != nil {
		return fmt.Errorf("failed to add output formatter: %w", err)
	}

	// เชื่อม edges
	if err := graph.AddEdge(compose.START, "input_validator"); err != nil {
		return fmt.Errorf("failed to add start edge: %w", err)
	}
	if err := graph.AddEdge("input_validator", "chat_model"); err != nil {
		return fmt.Errorf("failed to add validator to chat model edge: %w", err)
	}
	if err := graph.AddEdge("chat_model", "output_formatter"); err != nil {
		return fmt.Errorf("failed to add chat model to formatter edge: %w", err)
	}
	if err := graph.AddEdge("output_formatter", compose.END); err != nil {
		return fmt.Errorf("failed to add end edge: %w", err)
	}

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile chat model graph: %w", err)
	}

	// ทดสอบ
	testCases := [][]*schema.Message{
		{
			schema.SystemMessage("คุณเป็น AI ผู้ช่วยที่เป็นมิตร"),
			schema.UserMessage("สวัสดีครับ วันนี้เป็นอย่างไรบ้าง?"),
		},
		{
			schema.UserMessage("อธิบาย Eino Graph ให้ฟังหน่อย"),
		},
		{
			schema.SystemMessage("คุณเป็นครูสอนคณิตศาสตร์"),
			schema.UserMessage("2 + 2 เท่ากับเท่าไหร่?"),
		},
	}

	for i, messages := range testCases {
		fmt.Printf("\n--- Chat Model Test %d ---\n", i+1)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, messages)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// AddRetrieverNode Example
func runRetrieverNodeExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// สร้าง retriever
	retriever := NewMockRetrieverImpl()

	// Query Preprocessor
	queryPreprocessor := compose.InvokableLambda(func(ctx context.Context, query string) (string, error) {
		fmt.Printf("🔧 Query Preprocessor: Processing query\n")
		
		// Clean and normalize query
		cleaned := strings.TrimSpace(query)
		cleaned = strings.ToLower(cleaned)
		
		fmt.Printf("  Original: %s\n", query)
		fmt.Printf("  Cleaned: %s\n", cleaned)
		
		return cleaned, nil
	})

	// Context Builder
	contextBuilder := compose.InvokableLambda(func(ctx context.Context, docs []schema.Document) ([]*schema.Message, error) {
		fmt.Printf("📝 Context Builder: Building context from %d documents\n", len(docs))
		
		if len(docs) == 0 {
			return []*schema.Message{
				schema.SystemMessage("คุณเป็น AI ผู้ช่วยที่ตอบคำถาม"),
				schema.UserMessage("ขออภัย ไม่พบข้อมูลที่เกี่ยวข้อง"),
			}, nil
		}
		
		var contextParts []string
		for i, doc := range docs {
			contextParts = append(contextParts, fmt.Sprintf("%d. %s", i+1, doc.PageContent))
		}
		
		context := strings.Join(contextParts, "\n")
		systemPrompt := fmt.Sprintf(`คุณเป็น AI ผู้ช่วยที่ตอบคำถามโดยอิงจากข้อมูลที่ให้มา

ข้อมูลอ้างอิง:
%s

โปรดตอบคำถามโดยอิงจากข้อมูลด้านบน หากไม่มีข้อมูลเพียงพอให้บอกอย่างชัดเจน`, context)

		return []*schema.Message{
			schema.SystemMessage(systemPrompt),
		}, nil
	})

	// Response Combiner
	responseCombiner := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (*schema.Message, error) {
		fmt.Printf("🔗 Response Combiner: Combining context and response\n")
		
		contextMsgs, ok := input["context"].([]*schema.Message)
		if !ok || len(contextMsgs) == 0 {
			return nil, fmt.Errorf("no context messages")
		}
		
		originalQuery, ok := input["query"].(string)
		if !ok {
			return nil, fmt.Errorf("no original query")
		}
		
		// Add user query to context messages
		allMessages := append(contextMsgs, schema.UserMessage(originalQuery))
		
		// Generate response with chat model
		response, err := model.Generate(ctx, allMessages)
		if err != nil {
			return nil, fmt.Errorf("failed to generate response: %w", err)
		}
		
		return response, nil
	})

	// Final Formatter
	finalFormatter := compose.InvokableLambda(func(ctx context.Context, response *schema.Message) (string, error) {
		fmt.Printf("📋 Final Formatter: Formatting final response\n")
		
		formatted := fmt.Sprintf("🔍 RAG Response:\n%s\n\n🤖 Generated by: Native Eino Retriever + Chat Model", 
			response.Content)
		
		return formatted, nil
	})

	// Pipeline coordinator
	pipelineCoordinator := compose.InvokableLambda(func(ctx context.Context, query string) (map[string]interface{}, error) {
		// Preprocess query
		processedQuery, err := queryPreprocessor.Invoke(ctx, query)
		if err != nil {
			return nil, err
		}
		
		// Retrieve documents
		docs, err := retriever.Retrieve(ctx, processedQuery)
		if err != nil {
			return nil, err
		}
		
		// Build context
		contextMsgs, err := contextBuilder.Invoke(ctx, docs)
		if err != nil {
			return nil, err
		}
		
		return map[string]interface{}{
			"query":   query,
			"context": contextMsgs,
		}, nil
	})

	// เพิ่ม nodes - ใช้ native AddRetrieverNode
	if err := graph.AddLambdaNode("pipeline_coordinator", pipelineCoordinator); err != nil {
		return fmt.Errorf("failed to add pipeline coordinator: %w", err)
	}
	
	if err := graph.AddLambdaNode("response_combiner", responseCombiner); err != nil {
		return fmt.Errorf("failed to add response combiner: %w", err)
	}
	
	if err := graph.AddLambdaNode("final_formatter", finalFormatter); err != nil {
		return fmt.Errorf("failed to add final formatter: %w", err)
	}

	// เชื่อม edges
	if err := graph.AddEdge(compose.START, "pipeline_coordinator"); err != nil {
		return fmt.Errorf("failed to add start edge: %w", err)
	}
	if err := graph.AddEdge("pipeline_coordinator", "response_combiner"); err != nil {
		return fmt.Errorf("failed to add coordinator to combiner edge: %w", err)
	}
	if err := graph.AddEdge("response_combiner", "final_formatter"); err != nil {
		return fmt.Errorf("failed to add combiner to formatter edge: %w", err)
	}
	if err := graph.AddEdge("final_formatter", compose.END); err != nil {
		return fmt.Errorf("failed to add end edge: %w", err)
	}

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile retriever graph: %w", err)
	}

	// ทดสอบ
	testQueries := []string{
		"Eino Graph คืออะไร?",
		"Go channels ใช้งานยังไง?",
		"ข้อมูลเกี่ยวกับ Docker",
		"วิธีการทำ machine learning",
	}

	for i, query := range testQueries {
		fmt.Printf("\n--- Retriever Test %d ---\n", i+1)
		fmt.Printf("Query: %s\n", query)
		
		testCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		result, err := runnable.Invoke(testCtx, query)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// AddToolsNode Example  
func runToolsNodeExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// สร้าง tools
	calculatorTool := &MockToolImpl{
		name:        "calculator",
		description: "Perform mathematical calculations",
	}
	
	weatherTool := &MockToolImpl{
		name:        "weather", 
		description: "Get weather information",
	}
	
	translatorTool := &MockToolImpl{
		name:        "translator",
		description: "Translate text between languages",
	}

	// สร้าง ToolsNode
	toolsNode := compose.NewToolsNode([]any{calculatorTool, weatherTool, translatorTool})

	// Tool Selector
	toolSelector := compose.InvokableLambda(func(ctx context.Context, userInput string) (string, error) {
		fmt.Printf("🎯 Tool Selector: Analyzing user input\n")
		fmt.Printf("  Input: %s\n", userInput)
		
		inputLower := strings.ToLower(userInput)
		var selectedTool string
		var args string
		
		if strings.Contains(inputLower, "calculate") || strings.Contains(inputLower, "math") {
			selectedTool = "calculator"
			args = userInput
		} else if strings.Contains(inputLower, "weather") || strings.Contains(inputLower, "อากาศ") {
			selectedTool = "weather"
			args = userInput
		} else if strings.Contains(inputLower, "translate") || strings.Contains(inputLower, "แปล") {
			selectedTool = "translator"
			args = userInput
		} else {
			selectedTool = "calculator" // default
			args = userInput
		}
		
		result := fmt.Sprintf("%s:%s", selectedTool, args)
		fmt.Printf("  Selected: %s with args: %s\n", selectedTool, args)
		
		return result, nil
	})

	// Tool Result Formatter
	toolResultFormatter := compose.InvokableLambda(func(ctx context.Context, toolResult string) (string, error) {
		fmt.Printf("📋 Tool Result Formatter: Formatting tool output\n")
		
		formatted := fmt.Sprintf("🔧 Tool Execution Result:\n%s\n\n✅ Executed by: Native Eino ToolsNode", 
			toolResult)
		
		return formatted, nil
	})

	// เพิ่ม nodes - ใช้ native AddToolsNode
	if err := graph.AddLambdaNode("tool_selector", toolSelector); err != nil {
		return fmt.Errorf("failed to add tool selector: %w", err)
	}
	
	if err := graph.AddToolsNode("tools_node", toolsNode); err != nil {
		return fmt.Errorf("failed to add tools node: %w", err)
	}
	
	if err := graph.AddLambdaNode("tool_result_formatter", toolResultFormatter); err != nil {
		return fmt.Errorf("failed to add tool result formatter: %w", err)
	}

	// เชื่อม edges
	if err := graph.AddEdge(compose.START, "tool_selector"); err != nil {
		return fmt.Errorf("failed to add start edge: %w", err)
	}
	if err := graph.AddEdge("tool_selector", "tools_node"); err != nil {
		return fmt.Errorf("failed to add selector to tools edge: %w", err)
	}
	if err := graph.AddEdge("tools_node", "tool_result_formatter"); err != nil {
		return fmt.Errorf("failed to add tools to formatter edge: %w", err)
	}
	if err := graph.AddEdge("tool_result_formatter", compose.END); err != nil {
		return fmt.Errorf("failed to add end edge: %w", err)
	}

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile tools graph: %w", err)
	}

	// ทดสอบ
	testInputs := []string{
		"calculate 10 + 20",
		"what's the weather like today?",
		"translate hello to thai",
		"help me with math problem",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Tools Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, input)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// AddPassthroughNode Example
func runPassthroughNodeExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Input Logger
	inputLogger := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("📝 Input Logger: Logging input\n")
		fmt.Printf("  Input: %s\n", input)
		fmt.Printf("  Timestamp: %s\n", time.Now().Format("15:04:05"))
		
		return input, nil
	})

	// Main Processor  
	mainProcessor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("🤖 Main Processor: Processing input\n")
		
		processed := fmt.Sprintf("Processed: %s", strings.ToUpper(input))
		return processed, nil
	})

	// Output Logger
	outputLogger := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("📋 Output Logger: Logging output\n")
		fmt.Printf("  Output: %s\n", input)
		fmt.Printf("  Length: %d chars\n", len(input))
		
		return input, nil
	})

	// เพิ่ม nodes - ใช้ native AddPassthroughNode
	if err := graph.AddLambdaNode("input_logger", inputLogger); err != nil {
		return fmt.Errorf("failed to add input logger: %w", err)
	}
	
	// PassthroughNode ที่ไม่เปลี่ยนแปลงข้อมูล แต่ผ่านไปยัง node ถัดไป
	if err := graph.AddPassthroughNode("passthrough_monitor"); err != nil {
		return fmt.Errorf("failed to add passthrough node: %w", err)
	}
	
	if err := graph.AddLambdaNode("main_processor", mainProcessor); err != nil {
		return fmt.Errorf("failed to add main processor: %w", err)
	}
	
	if err := graph.AddPassthroughNode("passthrough_relay"); err != nil {
		return fmt.Errorf("failed to add passthrough relay: %w", err)
	}
	
	if err := graph.AddLambdaNode("output_logger", outputLogger); err != nil {
		return fmt.Errorf("failed to add output logger: %w", err)
	}

	// เชื่อม edges
	if err := graph.AddEdge(compose.START, "input_logger"); err != nil {
		return fmt.Errorf("failed to add start edge: %w", err)
	}
	if err := graph.AddEdge("input_logger", "passthrough_monitor"); err != nil {
		return fmt.Errorf("failed to add logger to passthrough edge: %w", err)
	}
	if err := graph.AddEdge("passthrough_monitor", "main_processor"); err != nil {
		return fmt.Errorf("failed to add passthrough to processor edge: %w", err)
	}
	if err := graph.AddEdge("main_processor", "passthrough_relay"); err != nil {
		return fmt.Errorf("failed to add processor to relay edge: %w", err)
	}
	if err := graph.AddEdge("passthrough_relay", "output_logger"); err != nil {
		return fmt.Errorf("failed to add relay to logger edge: %w", err)
	}
	if err := graph.AddEdge("output_logger", compose.END); err != nil {
		return fmt.Errorf("failed to add end edge: %w", err)
	}

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile passthrough graph: %w", err)
	}

	// ทดสอบ
	testInputs := []string{
		"hello world",
		"native eino nodes",
		"passthrough example",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Passthrough Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, input)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Final Result: %s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// AddGraphNode (Sub-graph) Example
func runSubGraphExample(ctx context.Context, model *openai.ChatModel) error {
	// สร้าง sub-graph สำหรับการประมวลผลข้อความ
	textProcessorGraph := compose.NewGraph[string, string]()
	
	// Text Analyzer
	textAnalyzer := compose.InvokableLambda(func(ctx context.Context, text string) (string, error) {
		fmt.Printf("📊 Sub-graph Text Analyzer: Analyzing text\n")
		
		words := len(strings.Fields(text))
		chars := len(text)
		
		analysis := fmt.Sprintf("Analysis: %d words, %d characters", words, chars)
		return analysis, nil
	})
	
	// Text Enhancer
	textEnhancer := compose.InvokableLambda(func(ctx context.Context, analysis string) (string, error) {
		fmt.Printf("✨ Sub-graph Text Enhancer: Enhancing analysis\n")
		
		enhanced := fmt.Sprintf("Enhanced %s with formatting", analysis)
		return enhanced, nil
	})
	
	// เพิ่ม nodes ใน sub-graph
	if err := textProcessorGraph.AddLambdaNode("text_analyzer", textAnalyzer); err != nil {
		return fmt.Errorf("failed to add text analyzer to sub-graph: %w", err)
	}
	if err := textProcessorGraph.AddLambdaNode("text_enhancer", textEnhancer); err != nil {
		return fmt.Errorf("failed to add text enhancer to sub-graph: %w", err)
	}
	
	// เชื่อม edges ใน sub-graph
	if err := textProcessorGraph.AddEdge(compose.START, "text_analyzer"); err != nil {
		return fmt.Errorf("failed to add start edge in sub-graph: %w", err)
	}
	if err := textProcessorGraph.AddEdge("text_analyzer", "text_enhancer"); err != nil {
		return fmt.Errorf("failed to add analyzer to enhancer edge in sub-graph: %w", err)
	}
	if err := textProcessorGraph.AddEdge("text_enhancer", compose.END); err != nil {
		return fmt.Errorf("failed to add end edge in sub-graph: %w", err)
	}

	// สร้าง main graph
	mainGraph := compose.NewGraph[string, string]()
	
	// Input Preparer
	inputPreparer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("🔧 Main Graph Input Preparer: Preparing input\n")
		
		prepared := fmt.Sprintf("Prepared: %s", strings.TrimSpace(input))
		return prepared, nil
	})
	
	// Output Finalizer
	outputFinalizer := compose.InvokableLambda(func(ctx context.Context, subGraphResult string) (string, error) {
		fmt.Printf("🎯 Main Graph Output Finalizer: Finalizing output\n")
		
		finalized := fmt.Sprintf("🔄 Sub-graph Processing Complete:\n%s\n\n📊 Processed by: Native Eino Sub-graph", 
			subGraphResult)
		return finalized, nil
	})
	
	// เพิ่ม nodes ใน main graph - ใช้ native AddGraphNode
	if err := mainGraph.AddLambdaNode("input_preparer", inputPreparer); err != nil {
		return fmt.Errorf("failed to add input preparer to main graph: %w", err)
	}
	
	if err := mainGraph.AddGraphNode("text_processor_subgraph", textProcessorGraph); err != nil {
		return fmt.Errorf("failed to add sub-graph to main graph: %w", err)
	}
	
	if err := mainGraph.AddLambdaNode("output_finalizer", outputFinalizer); err != nil {
		return fmt.Errorf("failed to add output finalizer to main graph: %w", err)
	}

	// เชื่อม edges ใน main graph
	if err := mainGraph.AddEdge(compose.START, "input_preparer"); err != nil {
		return fmt.Errorf("failed to add start edge in main graph: %w", err)
	}
	if err := mainGraph.AddEdge("input_preparer", "text_processor_subgraph"); err != nil {
		return fmt.Errorf("failed to add preparer to sub-graph edge: %w", err)
	}
	if err := mainGraph.AddEdge("text_processor_subgraph", "output_finalizer"); err != nil {
		return fmt.Errorf("failed to add sub-graph to finalizer edge: %w", err)
	}
	if err := mainGraph.AddEdge("output_finalizer", compose.END); err != nil {
		return fmt.Errorf("failed to add end edge in main graph: %w", err)
	}

	// Compile
	runnable, err := mainGraph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile main graph with sub-graph: %w", err)
	}

	// ทดสอบ
	testInputs := []string{
		"This is a sample text for testing sub-graph functionality",
		"Native Eino nodes are very powerful",
		"Sub-graphs allow complex workflow composition",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Sub-graph Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, input)
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

// Helper method to simulate Invoke for mock implementations
func (l *compose.Lambda[T, R]) Invoke(ctx context.Context, input T) (R, error) {
	// This is a mock implementation
	var zero R
	return zero, fmt.Errorf("invoke method not available in this context")
}