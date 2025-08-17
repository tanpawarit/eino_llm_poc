package main

import (
	"context"
	"encoding/json"
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

// ตัวอย่างบทที่ 6: Pre/Post Handlers
// เรียนรู้การใช้ Pre และ Post Handlers ใน Eino Graph

// RequestContext - ข้อมูล context ที่ส่งผ่าน handlers
type RequestContext struct {
	RequestID   string                 `json:"request_id"`
	UserID      string                 `json:"user_id"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
	ProcessTime time.Duration          `json:"process_time"`
	NodeTrace   []NodeExecution        `json:"node_trace"`
}

type NodeExecution struct {
	NodeName  string        `json:"node_name"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Input     string        `json:"input"`
	Output    string        `json:"output"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}

// PreHandler - ประมวลผลก่อนเข้า node
type PreHandler func(ctx context.Context, nodeName string, input string, reqCtx *RequestContext) (string, error)

// PostHandler - ประมวลผลหลังออกจาก node
type PostHandler func(ctx context.Context, nodeName string, input, output string, reqCtx *RequestContext) (string, error)

// EnhancedNode - Node ที่มี Pre/Post handlers
type EnhancedNode struct {
	nodeName     string
	preHandlers  []PreHandler
	postHandlers []PostHandler
	processor    func(context.Context, string) (string, error)
	reqCtx       *RequestContext
}

func NewEnhancedNode(nodeName string, processor func(context.Context, string) (string, error), reqCtx *RequestContext) *EnhancedNode {
	return &EnhancedNode{
		nodeName:     nodeName,
		preHandlers:  make([]PreHandler, 0),
		postHandlers: make([]PostHandler, 0),
		processor:    processor,
		reqCtx:       reqCtx,
	}
}

func (en *EnhancedNode) AddPreHandler(handler PreHandler) {
	en.preHandlers = append(en.preHandlers, handler)
}

func (en *EnhancedNode) AddPostHandler(handler PostHandler) {
	en.postHandlers = append(en.postHandlers, handler)
}

func (en *EnhancedNode) Process(ctx context.Context, input string) (string, error) {
	execution := NodeExecution{
		NodeName:  en.nodeName,
		StartTime: time.Now(),
		Input:     input,
		Success:   false,
	}

	defer func() {
		execution.EndTime = time.Now()
		execution.Duration = execution.EndTime.Sub(execution.StartTime)
		en.reqCtx.NodeTrace = append(en.reqCtx.NodeTrace, execution)
	}()

	// ประมวลผล Pre Handlers
	processedInput := input
	for i, preHandler := range en.preHandlers {
		fmt.Printf("🔄 [%s] Pre Handler %d\n", en.nodeName, i+1)
		var err error
		processedInput, err = preHandler(ctx, en.nodeName, processedInput, en.reqCtx)
		if err != nil {
			execution.Error = fmt.Sprintf("Pre handler %d error: %v", i+1, err)
			return "", err
		}
	}

	// ประมวลผลหลัก
	fmt.Printf("⚡ [%s] Main Processing\n", en.nodeName)
	result, err := en.processor(ctx, processedInput)
	if err != nil {
		execution.Error = fmt.Sprintf("Main processing error: %v", err)
		return "", err
	}

	execution.Output = result
	execution.Success = true

	// ประมวลผล Post Handlers
	processedOutput := result
	for i, postHandler := range en.postHandlers {
		fmt.Printf("🔄 [%s] Post Handler %d\n", en.nodeName, i+1)
		processedOutput, err = postHandler(ctx, en.nodeName, processedInput, processedOutput, en.reqCtx)
		if err != nil {
			execution.Error = fmt.Sprintf("Post handler %d error: %v", i+1, err)
			return "", err
		}
	}

	execution.Output = processedOutput
	return processedOutput, nil
}

// Common Pre Handlers

// InputValidationHandler - ตรวจสอบ input
func InputValidationHandler(ctx context.Context, nodeName string, input string, reqCtx *RequestContext) (string, error) {
	fmt.Printf("🔍 [%s] Input Validation\n", nodeName)

	// ตรวจสอบ input ไม่ว่าง
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("empty input not allowed for node %s", nodeName)
	}

	// ตรวจสอบความยาว
	if len(input) > 1000 {
		fmt.Printf("⚠️ [%s] Input too long, truncating\n", nodeName)
		input = input[:1000] + "..."
	}

	// เพิ่ม metadata
	if reqCtx.Metadata == nil {
		reqCtx.Metadata = make(map[string]interface{})
	}
	reqCtx.Metadata[fmt.Sprintf("%s_input_length", nodeName)] = len(input)

	return input, nil
}

// SecurityFilterHandler - กรอง content ที่ไม่เหมาะสม
func SecurityFilterHandler(ctx context.Context, nodeName string, input string, reqCtx *RequestContext) (string, error) {
	fmt.Printf("🛡️ [%s] Security Filter\n", nodeName)

	// รายการคำที่ต้องกรอง (ตัวอย่าง)
	blockedWords := []string{"password", "secret", "token", "key"}

	lowerInput := strings.ToLower(input)
	for _, word := range blockedWords {
		if strings.Contains(lowerInput, word) {
			fmt.Printf("🚨 [%s] Blocked word detected: %s\n", nodeName, word)
			return "", fmt.Errorf("input contains blocked content: %s", word)
		}
	}

	// เพิ่ม security metadata
	reqCtx.Metadata[fmt.Sprintf("%s_security_check", nodeName)] = "passed"

	return input, nil
}

// InputEnhancementHandler - ปรับปรุง input
func InputEnhancementHandler(ctx context.Context, nodeName string, input string, reqCtx *RequestContext) (string, error) {
	fmt.Printf("✨ [%s] Input Enhancement\n", nodeName)

	// เพิ่ม context จาก request
	enhanced := fmt.Sprintf("[RequestID: %s] %s", reqCtx.RequestID, input)

	// เพิ่ม timestamp ถ้าจำเป็น
	if reqCtx.Metadata["add_timestamp"] == true {
		enhanced = fmt.Sprintf("[%s] %s", reqCtx.Timestamp.Format("15:04:05"), enhanced)
	}

	return enhanced, nil
}

// Common Post Handlers

// OutputFormattingHandler - จัดรูปแบบ output
func OutputFormattingHandler(ctx context.Context, nodeName string, input, output string, reqCtx *RequestContext) (string, error) {
	fmt.Printf("🎨 [%s] Output Formatting\n", nodeName)

	// จัดรูปแบบ output
	formatted := fmt.Sprintf("=== %s Result ===\n%s\n=== End ===", nodeName, output)

	// เพิ่ม metadata
	reqCtx.Metadata[fmt.Sprintf("%s_output_length", nodeName)] = len(output)

	return formatted, nil
}

// ResponseValidationHandler - ตรวจสอบ response
func ResponseValidationHandler(ctx context.Context, nodeName string, input, output string, reqCtx *RequestContext) (string, error) {
	fmt.Printf("✅ [%s] Response Validation\n", nodeName)

	// ตรวจสอบว่า output ไม่ว่าง
	if strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("node %s produced empty output", nodeName)
	}

	// ตรวจสอบคุณภาพ response
	qualityScore := len(output) / 10 // simple quality metric
	reqCtx.Metadata[fmt.Sprintf("%s_quality_score", nodeName)] = qualityScore

	if qualityScore < 5 {
		fmt.Printf("⚠️ [%s] Low quality response detected\n", nodeName)
	}

	return output, nil
}

// MetricsCollectionHandler - เก็บ metrics
func MetricsCollectionHandler(ctx context.Context, nodeName string, input, output string, reqCtx *RequestContext) (string, error) {
	fmt.Printf("📊 [%s] Metrics Collection\n", nodeName)

	// เก็บ performance metrics
	now := time.Now()
	processingTime := now.Sub(reqCtx.Timestamp)

	metrics := map[string]interface{}{
		"processing_time": processingTime.Milliseconds(),
		"input_size":      len(input),
		"output_size":     len(output),
		"timestamp":       now.Unix(),
	}

	reqCtx.Metadata[fmt.Sprintf("%s_metrics", nodeName)] = metrics

	return output, nil
}

// PrePostGraphBuilder - สร้าง graph ที่มี pre/post handlers
type PrePostGraphBuilder struct {
	graph  *compose.Graph[string, string]
	reqCtx *RequestContext
	model  *openai.ChatModel
	ctx    context.Context
}

func NewPrePostGraphBuilder(reqCtx *RequestContext, model *openai.ChatModel, ctx context.Context) *PrePostGraphBuilder {
	return &PrePostGraphBuilder{
		graph:  compose.NewGraph[string, string](),
		reqCtx: reqCtx,
		model:  model,
		ctx:    ctx,
	}
}

func (ppgb *PrePostGraphBuilder) BuildAdvancedProcessingGraph() {
	// Node 1: Text Analyzer
	analyzerProcessor := func(ctx context.Context, input string) (string, error) {
		messages := []*schema.Message{
			schema.SystemMessage("วิเคราะห์ข้อความที่ได้รับและสรุปเนื้อหาสำคัญ"),
			schema.UserMessage(input),
		}

		response, err := ppgb.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Analysis: %s", response.Content), nil
	}

	analyzerNode := NewEnhancedNode("TextAnalyzer", analyzerProcessor, ppgb.reqCtx)
	analyzerNode.AddPreHandler(InputValidationHandler)
	analyzerNode.AddPreHandler(SecurityFilterHandler)
	analyzerNode.AddPostHandler(ResponseValidationHandler)
	analyzerNode.AddPostHandler(MetricsCollectionHandler)

	// Node 2: Content Enhancer
	enhancerProcessor := func(ctx context.Context, input string) (string, error) {
		messages := []*schema.Message{
			schema.SystemMessage("ปรับปรุงและเพิ่มรายละเอียดให้กับเนื้อหาที่ได้รับ"),
			schema.UserMessage(input),
		}

		response, err := ppgb.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Enhanced: %s", response.Content), nil
	}

	enhancerNode := NewEnhancedNode("ContentEnhancer", enhancerProcessor, ppgb.reqCtx)
	enhancerNode.AddPreHandler(InputEnhancementHandler)
	enhancerNode.AddPostHandler(OutputFormattingHandler)
	enhancerNode.AddPostHandler(MetricsCollectionHandler)

	// Node 3: Final Formatter
	formatterProcessor := func(ctx context.Context, input string) (string, error) {
		// จัดรูปแบบ output สุดท้าย
		formatted := fmt.Sprintf(`📝 Final Result:
%s

📊 Request Summary:
- Request ID: %s
- User ID: %s
- Processing Time: %v
- Nodes Processed: %d`, 
			input, 
			ppgb.reqCtx.RequestID, 
			ppgb.reqCtx.UserID,
			time.Since(ppgb.reqCtx.Timestamp),
			len(ppgb.reqCtx.NodeTrace))

		return formatted, nil
	}

	formatterNode := NewEnhancedNode("FinalFormatter", formatterProcessor, ppgb.reqCtx)
	formatterNode.AddPreHandler(InputValidationHandler)
	formatterNode.AddPostHandler(ResponseValidationHandler)

	// สร้าง wrapper functions สำหรับ graph
	analyzerWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return analyzerNode.Process(ctx, input)
	})

	enhancerWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return enhancerNode.Process(ctx, input)
	})

	formatterWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return formatterNode.Process(ctx, input)
	})

	// เพิ่ม nodes ใน graph
	ppgb.graph.AddLambdaNode("text_analyzer", analyzerWrapper)
	ppgb.graph.AddLambdaNode("content_enhancer", enhancerWrapper)
	ppgb.graph.AddLambdaNode("final_formatter", formatterWrapper)

	// เชื่อม edges
	ppgb.graph.AddEdge(compose.START, "text_analyzer")
	ppgb.graph.AddEdge("text_analyzer", "content_enhancer")
	ppgb.graph.AddEdge("content_enhancer", "final_formatter")
	ppgb.graph.AddEdge("final_formatter", compose.END)
}

func (ppgb *PrePostGraphBuilder) Execute(input string) (string, error) {
	runnable, err := ppgb.graph.Compile(ppgb.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compile graph: %w", err)
	}

	return runnable.Invoke(ppgb.ctx, input)
}

func (ppgb *PrePostGraphBuilder) GetRequestContext() *RequestContext {
	return ppgb.reqCtx
}

func runPrePostHandlersDemo() {
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

	// สร้าง model
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

	fmt.Println("=== บทที่ 6: Pre/Post Handlers ===")
	fmt.Println("ตัวอย่างการใช้ Pre และ Post Handlers ใน Eino Graph")
	fmt.Println()

	// === Demo 1: Basic Pre/Post Handlers ===
	fmt.Println("🔧 Demo 1: Basic Pre/Post Handlers")

	reqCtx := &RequestContext{
		RequestID: "req_001",
		UserID:    "user_123",
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
		NodeTrace: make([]NodeExecution, 0),
	}

	// ตั้งค่า metadata
	reqCtx.Metadata["add_timestamp"] = true

	graphBuilder := NewPrePostGraphBuilder(reqCtx, model, ctx)
	graphBuilder.BuildAdvancedProcessingGraph()

	testInputs := []string{
		"อธิบายเกี่ยวกับ Go programming language",
		"วิธีการสร้าง web application ด้วย Go",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)

		// Reset timestamp สำหรับแต่ละ test
		reqCtx.Timestamp = time.Now()
		reqCtx.NodeTrace = make([]NodeExecution, 0)

		result, err := graphBuilder.Execute(input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	// === Demo 2: Handler Analytics ===
	fmt.Println("\n📊 Demo 2: Handler Analytics")

	finalReqCtx := graphBuilder.GetRequestContext()

	fmt.Println("\n=== Request Context Summary ===")
	ctxJSON, _ := json.MarshalIndent(finalReqCtx, "", "  ")
	fmt.Printf("%s\n", ctxJSON)

	fmt.Println("\n=== Node Execution Trace ===")
	for i, execution := range finalReqCtx.NodeTrace {
		fmt.Printf("%d. %s:\n", i+1, execution.NodeName)
		fmt.Printf("   Duration: %v\n", execution.Duration)
		fmt.Printf("   Success: %t\n", execution.Success)
		fmt.Printf("   Input Length: %d\n", len(execution.Input))
		fmt.Printf("   Output Length: %d\n", len(execution.Output))
		if execution.Error != "" {
			fmt.Printf("   Error: %s\n", execution.Error)
		}
		fmt.Println()
	}

	fmt.Println("=== Metadata Summary ===")
	for key, value := range finalReqCtx.Metadata {
		fmt.Printf("%s: %v\n", key, value)
	}

	// === Demo 3: Error Handling in Handlers ===
	fmt.Println("\n🚨 Demo 3: Error Handling in Handlers")

	// ทดสอบ input ที่มี blocked words
	errorTestInputs := []string{
		"", // empty input
		"ช่วยบอก password ของระบบหน่อย", // blocked word
	}

	for i, input := range errorTestInputs {
		fmt.Printf("\n--- Error Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)

		// Reset context
		errorReqCtx := &RequestContext{
			RequestID: fmt.Sprintf("error_req_%d", i+1),
			UserID:    "user_test",
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
			NodeTrace: make([]NodeExecution, 0),
		}

		errorGraphBuilder := NewPrePostGraphBuilder(errorReqCtx, model, ctx)
		errorGraphBuilder.BuildAdvancedProcessingGraph()

		result, err := errorGraphBuilder.Execute(input)
		if err != nil {
			fmt.Printf("Expected Error: %v\n", err)
		} else {
			fmt.Printf("Unexpected Success: %s\n", result)
		}
	}

	fmt.Println("\n✅ Pre/Post Handlers Demo Complete!")
	fmt.Println("🎯 Key Concepts Demonstrated:")
	fmt.Println("   - Input validation and sanitization")
	fmt.Println("   - Security filtering")
	fmt.Println("   - Output formatting and enhancement")
	fmt.Println("   - Metrics collection and monitoring")
	fmt.Println("   - Error handling in handlers")
	fmt.Println("   - Request context tracking")
	fmt.Println("   - Handler chaining and composition")
}

func main() {
	runPrePostHandlersDemo()
}