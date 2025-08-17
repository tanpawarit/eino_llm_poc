package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// ตัวอย่างบทที่ 9: Advanced Graph Patterns
// เรียนรู้รูปแบบ Graph ขั้นสูงใน Eino

// GraphPattern - ประเภทของ graph pattern
type GraphPattern string

const (
	PatternPipeline    GraphPattern = "pipeline"
	PatternFanOut      GraphPattern = "fan_out"
	PatternFanIn       GraphPattern = "fan_in"
	PatternScatterGather GraphPattern = "scatter_gather"
	PatternMapReduce   GraphPattern = "map_reduce"
	PatternWorkflow    GraphPattern = "workflow"
	PatternDynamic     GraphPattern = "dynamic"
)

// TaskResult - ผลลัพธ์จาก task
type TaskResult struct {
	NodeName  string                 `json:"node_name"`
	Input     string                 `json:"input"`
	Output    string                 `json:"output"`
	Metadata  map[string]interface{} `json:"metadata"`
	Duration  time.Duration          `json:"duration"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
}

// WorkflowContext - context สำหรับ workflow
type WorkflowContext struct {
	WorkflowID  string                 `json:"workflow_id"`
	StartTime   time.Time              `json:"start_time"`
	Results     map[string]TaskResult  `json:"results"`
	Metadata    map[string]interface{} `json:"metadata"`
	CurrentStep string                 `json:"current_step"`
	Status      string                 `json:"status"`
	mu          sync.RWMutex
}

func NewWorkflowContext(workflowID string) *WorkflowContext {
	return &WorkflowContext{
		WorkflowID: workflowID,
		StartTime:  time.Now(),
		Results:    make(map[string]TaskResult),
		Metadata:   make(map[string]interface{}),
		Status:     "running",
	}
}

func (wc *WorkflowContext) AddResult(nodeName string, result TaskResult) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.Results[nodeName] = result
}

func (wc *WorkflowContext) GetResult(nodeName string) (TaskResult, bool) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	result, exists := wc.Results[nodeName]
	return result, exists
}

func (wc *WorkflowContext) SetCurrentStep(step string) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.CurrentStep = step
}

func (wc *WorkflowContext) SetStatus(status string) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.Status = status
}

// 1. Fan-Out/Fan-In Pattern
type FanOutFanInGraph struct {
	graph   *compose.Graph[string, string]
	model   *openai.ChatModel
	ctx     context.Context
	wfCtx   *WorkflowContext
}

func NewFanOutFanInGraph(model *openai.ChatModel, ctx context.Context) *FanOutFanInGraph {
	return &FanOutFanInGraph{
		graph: compose.NewGraph[string, string](),
		model: model,
		ctx:   ctx,
		wfCtx: NewWorkflowContext("fanout_fanin_001"),
	}
}

func (fofig *FanOutFanInGraph) BuildFanOutFanInGraph() {
	// Node 1: Splitter - แยกงานออกเป็นหลายส่วน
	splitter := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		fofig.wfCtx.SetCurrentStep("splitter")
		
		fmt.Printf("📋 [Splitter] Splitting input into multiple tasks\n")
		
		// แยก input เป็น 3 ส่วน สำหรับประมวลผลแบบ parallel
		tasks := []string{
			fmt.Sprintf("summarize: %s", input),
			fmt.Sprintf("analyze_sentiment: %s", input),
			fmt.Sprintf("extract_keywords: %s", input),
		}
		
		result := strings.Join(tasks, "|")
		
		taskResult := TaskResult{
			NodeName: "splitter",
			Input:    input,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
			Metadata: map[string]interface{}{
				"task_count": len(tasks),
			},
		}
		fofig.wfCtx.AddResult("splitter", taskResult)
		
		return result, nil
	})

	// Node 2a: Summarizer
	summarizer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		fofig.wfCtx.SetCurrentStep("summarizer")
		
		tasks := strings.Split(input, "|")
		var summarizeTask string
		for _, task := range tasks {
			if strings.HasPrefix(task, "summarize:") {
				summarizeTask = strings.TrimPrefix(task, "summarize: ")
				break
			}
		}
		
		fmt.Printf("📝 [Summarizer] Processing: %s\n", summarizeTask[:min(50, len(summarizeTask))]+"...")
		
		messages := []*schema.Message{
			schema.SystemMessage("สรุปเนื้อหาที่ได้รับให้กระชับและชัดเจน"),
			schema.UserMessage(summarizeTask),
		}
		
		response, err := fofig.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}
		
		result := fmt.Sprintf("Summary: %s", response.Content)
		
		taskResult := TaskResult{
			NodeName: "summarizer",
			Input:    summarizeTask,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
		}
		fofig.wfCtx.AddResult("summarizer", taskResult)
		
		return result, nil
	})

	// Node 2b: Sentiment Analyzer
	sentimentAnalyzer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		fofig.wfCtx.SetCurrentStep("sentiment_analyzer")
		
		tasks := strings.Split(input, "|")
		var sentimentTask string
		for _, task := range tasks {
			if strings.HasPrefix(task, "analyze_sentiment:") {
				sentimentTask = strings.TrimPrefix(task, "analyze_sentiment: ")
				break
			}
		}
		
		fmt.Printf("😊 [SentimentAnalyzer] Processing: %s\n", sentimentTask[:min(50, len(sentimentTask))]+"...")
		
		messages := []*schema.Message{
			schema.SystemMessage("วิเคราะห์อารมณ์ของข้อความ (positive/negative/neutral) พร้อมให้เหตุผล"),
			schema.UserMessage(sentimentTask),
		}
		
		response, err := fofig.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}
		
		result := fmt.Sprintf("Sentiment: %s", response.Content)
		
		taskResult := TaskResult{
			NodeName: "sentiment_analyzer",
			Input:    sentimentTask,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
		}
		fofig.wfCtx.AddResult("sentiment_analyzer", taskResult)
		
		return result, nil
	})

	// Node 2c: Keyword Extractor
	keywordExtractor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		fofig.wfCtx.SetCurrentStep("keyword_extractor")
		
		tasks := strings.Split(input, "|")
		var keywordTask string
		for _, task := range tasks {
			if strings.HasPrefix(task, "extract_keywords:") {
				keywordTask = strings.TrimPrefix(task, "extract_keywords: ")
				break
			}
		}
		
		fmt.Printf("🔑 [KeywordExtractor] Processing: %s\n", keywordTask[:min(50, len(keywordTask))]+"...")
		
		messages := []*schema.Message{
			schema.SystemMessage("สกัดคำสำคัญจากข้อความ ให้ 5-10 คำสำคัญ"),
			schema.UserMessage(keywordTask),
		}
		
		response, err := fofig.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}
		
		result := fmt.Sprintf("Keywords: %s", response.Content)
		
		taskResult := TaskResult{
			NodeName: "keyword_extractor",
			Input:    keywordTask,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
		}
		fofig.wfCtx.AddResult("keyword_extractor", taskResult)
		
		return result, nil
	})

	// Node 3: Aggregator - รวมผลลัพธ์
	aggregator := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Time{}
		fofig.wfCtx.SetCurrentStep("aggregator")
		
		fmt.Printf("📊 [Aggregator] Combining results from parallel processors\n")
		
		// รวมผลลัพธ์จาก parallel nodes
		// ใน implementation จริง เราจะรอให้ทุก parallel node เสร็จ
		// แต่ในตัวอย่างนี้เราจะใช้ข้อมูลจาก context
		
		summaryResult, _ := fofig.wfCtx.GetResult("summarizer")
		sentimentResult, _ := fofig.wfCtx.GetResult("sentiment_analyzer")
		keywordResult, _ := fofig.wfCtx.GetResult("keyword_extractor")
		
		combinedResult := fmt.Sprintf(`🔍 Text Analysis Report:

%s

%s

%s

📈 Processing Summary:
- Total processing time: %v
- All tasks completed successfully`,
			summaryResult.Output,
			sentimentResult.Output,
			keywordResult.Output,
			time.Since(fofig.wfCtx.StartTime))
		
		taskResult := TaskResult{
			NodeName: "aggregator",
			Input:    input,
			Output:   combinedResult,
			Duration: time.Since(startTime),
			Success:  true,
			Metadata: map[string]interface{}{
				"components_count": 3,
			},
		}
		fofig.wfCtx.AddResult("aggregator", taskResult)
		fofig.wfCtx.SetStatus("completed")
		
		return combinedResult, nil
	})

	// เพิ่ม nodes
	fofig.graph.AddLambdaNode("splitter", splitter)
	fofig.graph.AddLambdaNode("summarizer", summarizer)
	fofig.graph.AddLambdaNode("sentiment_analyzer", sentimentAnalyzer)
	fofig.graph.AddLambdaNode("keyword_extractor", keywordExtractor)
	fofig.graph.AddLambdaNode("aggregator", aggregator)

	// เชื่อม edges - fan-out pattern
	fofig.graph.AddEdge(compose.START, "splitter")
	
	// Fan-out: splitter ไปยัง parallel processors
	fofig.graph.AddEdge("splitter", "summarizer")
	fofig.graph.AddEdge("splitter", "sentiment_analyzer")
	fofig.graph.AddEdge("splitter", "keyword_extractor")
	
	// Fan-in: parallel processors มาที่ aggregator
	fofig.graph.AddEdge("summarizer", "aggregator")
	fofig.graph.AddEdge("sentiment_analyzer", "aggregator")
	fofig.graph.AddEdge("keyword_extractor", "aggregator")
	
	fofig.graph.AddEdge("aggregator", compose.END)
}

func (fofig *FanOutFanInGraph) Execute(input string) (string, error) {
	runnable, err := fofig.graph.Compile(fofig.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compile fan-out/fan-in graph: %w", err)
	}
	
	return runnable.Invoke(fofig.ctx, input)
}

func (fofig *FanOutFanInGraph) GetWorkflowContext() *WorkflowContext {
	return fofig.wfCtx
}

// 2. Map-Reduce Pattern
type MapReduceGraph struct {
	graph *compose.Graph[[]string, string]
	model *openai.ChatModel
	ctx   context.Context
	wfCtx *WorkflowContext
}

func NewMapReduceGraph(model *openai.ChatModel, ctx context.Context) *MapReduceGraph {
	return &MapReduceGraph{
		graph: compose.NewGraph[[]string, string](),
		model: model,
		ctx:   ctx,
		wfCtx: NewWorkflowContext("mapreduce_001"),
	}
}

func (mrg *MapReduceGraph) BuildMapReduceGraph() {
	// Map Phase: ประมวลผลแต่ละ item แยกกัน
	mapper := compose.InvokableLambda(func(ctx context.Context, inputs []string) (string, error) {
		startTime := time.Now()
		mrg.wfCtx.SetCurrentStep("mapper")
		
		fmt.Printf("🗺️ [Mapper] Processing %d items\n", len(inputs))
		
		var mappedResults []string
		
		for i, input := range inputs {
			fmt.Printf("📝 [Mapper] Processing item %d: %s\n", i+1, input[:min(30, len(input))]+"...")
			
			messages := []*schema.Message{
				schema.SystemMessage("สรุปข้อความนี้ให้สั้นและกระชับ ไม่เกิน 100 คำ"),
				schema.UserMessage(input),
			}
			
			response, err := mrg.model.Generate(ctx, messages)
			if err != nil {
				return "", fmt.Errorf("map error for item %d: %w", i, err)
			}
			
			mappedResults = append(mappedResults, response.Content)
		}
		
		result := strings.Join(mappedResults, "|||")
		
		taskResult := TaskResult{
			NodeName: "mapper",
			Input:    strings.Join(inputs, "; "),
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
			Metadata: map[string]interface{}{
				"items_processed": len(inputs),
			},
		}
		mrg.wfCtx.AddResult("mapper", taskResult)
		
		return result, nil
	})

	// Reduce Phase: รวมผลลัพธ์ทั้งหมด
	reducer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		mrg.wfCtx.SetCurrentStep("reducer")
		
		mappedResults := strings.Split(input, "|||")
		fmt.Printf("🔄 [Reducer] Combining %d mapped results\n", len(mappedResults))
		
		// รวมผลลัพธ์ทั้งหมดเป็น summary ใหญ่
		combinedInput := strings.Join(mappedResults, "\n\n")
		
		messages := []*schema.Message{
			schema.SystemMessage("รวบรวมและสรุปข้อมูลทั้งหมดเป็นรายงานเดียว ให้ครอบคลุมประเด็นสำคัญทั้งหมด"),
			schema.UserMessage(fmt.Sprintf("สรุปข้อมูลต่อไปนี้:\n\n%s", combinedInput)),
		}
		
		response, err := mrg.model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("reduce error: %w", err)
		}
		
		finalResult := fmt.Sprintf(`📄 Map-Reduce Summary Report:

%s

📊 Processing Details:
- Items processed: %d
- Processing time: %v
- Pattern: Map-Reduce`,
			response.Content,
			len(mappedResults),
			time.Since(mrg.wfCtx.StartTime))
		
		taskResult := TaskResult{
			NodeName: "reducer",
			Input:    input,
			Output:   finalResult,
			Duration: time.Since(startTime),
			Success:  true,
			Metadata: map[string]interface{}{
				"results_combined": len(mappedResults),
			},
		}
		mrg.wfCtx.AddResult("reducer", taskResult)
		mrg.wfCtx.SetStatus("completed")
		
		return finalResult, nil
	})

	// เพิ่ม nodes
	mrg.graph.AddLambdaNode("mapper", mapper)
	mrg.graph.AddLambdaNode("reducer", reducer)

	// เชื่อม edges
	mrg.graph.AddEdge(compose.START, "mapper")
	mrg.graph.AddEdge("mapper", "reducer")
	mrg.graph.AddEdge("reducer", compose.END)
}

func (mrg *MapReduceGraph) Execute(inputs []string) (string, error) {
	runnable, err := mrg.graph.Compile(mrg.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compile map-reduce graph: %w", err)
	}
	
	return runnable.Invoke(mrg.ctx, inputs)
}

func (mrg *MapReduceGraph) GetWorkflowContext() *WorkflowContext {
	return mrg.wfCtx
}

// 3. Dynamic Routing Pattern
type DynamicRoutingGraph struct {
	graph *compose.Graph[string, string]
	model *openai.ChatModel
	ctx   context.Context
	wfCtx *WorkflowContext
}

func NewDynamicRoutingGraph(model *openai.ChatModel, ctx context.Context) *DynamicRoutingGraph {
	return &DynamicRoutingGraph{
		graph: compose.NewGraph[string, string](),
		model: model,
		ctx:   ctx,
		wfCtx: NewWorkflowContext("dynamic_routing_001"),
	}
}

func (drg *DynamicRoutingGraph) BuildDynamicRoutingGraph() {
	// Router Node: ตัดสินใจเส้นทาง
	router := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		drg.wfCtx.SetCurrentStep("router")
		
		fmt.Printf("🔀 [Router] Analyzing input to determine routing\n")
		
		// วิเคราะห์ประเภทของ input
		var route string
		lowerInput := strings.ToLower(input)
		
		switch {
		case strings.Contains(lowerInput, "code") || strings.Contains(lowerInput, "programming") || strings.Contains(lowerInput, "function"):
			route = "code_analyzer"
		case strings.Contains(lowerInput, "business") || strings.Contains(lowerInput, "market") || strings.Contains(lowerInput, "strategy"):
			route = "business_analyzer"
		case strings.Contains(lowerInput, "technical") || strings.Contains(lowerInput, "system") || strings.Contains(lowerInput, "architecture"):
			route = "tech_analyzer"
		default:
			route = "general_analyzer"
		}
		
		result := fmt.Sprintf("ROUTE:%s|%s", route, input)
		
		taskResult := TaskResult{
			NodeName: "router",
			Input:    input,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
			Metadata: map[string]interface{}{
				"selected_route": route,
			},
		}
		drg.wfCtx.AddResult("router", taskResult)
		
		fmt.Printf("➡️ [Router] Routing to: %s\n", route)
		return result, nil
	})

	// Code Analyzer
	codeAnalyzer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		drg.wfCtx.SetCurrentStep("code_analyzer")
		
		// แยก route info กับ actual input
		parts := strings.SplitN(input, "|", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], "ROUTE:code_analyzer") {
			return input, nil // ไม่ใช่งานของเรา
		}
		
		actualInput := parts[1]
		fmt.Printf("💻 [CodeAnalyzer] Analyzing code-related content\n")
		
		messages := []*schema.Message{
			schema.SystemMessage("คุณเป็นผู้เชี่ยวชาญด้านการเขียนโปรแกรม วิเคราะห์โค้ดหรือเนื้อหาที่เกี่ยวกับการเขียนโปรแกรม"),
			schema.UserMessage(actualInput),
		}
		
		response, err := drg.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}
		
		result := fmt.Sprintf("💻 Code Analysis: %s", response.Content)
		
		taskResult := TaskResult{
			NodeName: "code_analyzer",
			Input:    actualInput,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
		}
		drg.wfCtx.AddResult("code_analyzer", taskResult)
		
		return result, nil
	})

	// Business Analyzer
	businessAnalyzer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		drg.wfCtx.SetCurrentStep("business_analyzer")
		
		parts := strings.SplitN(input, "|", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], "ROUTE:business_analyzer") {
			return input, nil
		}
		
		actualInput := parts[1]
		fmt.Printf("📈 [BusinessAnalyzer] Analyzing business content\n")
		
		messages := []*schema.Message{
			schema.SystemMessage("คุณเป็นที่ปรึกษาธุรกิจ วิเคราะห์เนื้อหาที่เกี่ยวกับธุรกิจ การตลาด และกลยุทธ์"),
			schema.UserMessage(actualInput),
		}
		
		response, err := drg.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}
		
		result := fmt.Sprintf("📈 Business Analysis: %s", response.Content)
		
		taskResult := TaskResult{
			NodeName: "business_analyzer",
			Input:    actualInput,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
		}
		drg.wfCtx.AddResult("business_analyzer", taskResult)
		
		return result, nil
	})

	// General Analyzer
	generalAnalyzer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		drg.wfCtx.SetCurrentStep("general_analyzer")
		
		parts := strings.SplitN(input, "|", 2)
		actualInput := input
		if len(parts) == 2 && strings.HasPrefix(parts[0], "ROUTE:") {
			actualInput = parts[1]
		}
		
		fmt.Printf("🔍 [GeneralAnalyzer] General content analysis\n")
		
		messages := []*schema.Message{
			schema.SystemMessage("วิเคราะห์และสรุปเนื้อหาที่ได้รับอย่างครอบคลุม"),
			schema.UserMessage(actualInput),
		}
		
		response, err := drg.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}
		
		result := fmt.Sprintf("🔍 General Analysis: %s", response.Content)
		
		taskResult := TaskResult{
			NodeName: "general_analyzer",
			Input:    actualInput,
			Output:   result,
			Duration: time.Since(startTime),
			Success:  true,
		}
		drg.wfCtx.AddResult("general_analyzer", taskResult)
		
		return result, nil
	})

	// เพิ่ม nodes
	drg.graph.AddLambdaNode("router", router)
	drg.graph.AddLambdaNode("code_analyzer", codeAnalyzer)
	drg.graph.AddLambdaNode("business_analyzer", businessAnalyzer)
	drg.graph.AddLambdaNode("general_analyzer", generalAnalyzer)

	// เชื่อม edges - dynamic routing
	drg.graph.AddEdge(compose.START, "router")
	drg.graph.AddEdge("router", "code_analyzer")
	drg.graph.AddEdge("router", "business_analyzer")
	drg.graph.AddEdge("router", "general_analyzer")
	drg.graph.AddEdge("code_analyzer", compose.END)
	drg.graph.AddEdge("business_analyzer", compose.END)
	drg.graph.AddEdge("general_analyzer", compose.END)
}

func (drg *DynamicRoutingGraph) Execute(input string) (string, error) {
	runnable, err := drg.graph.Compile(drg.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compile dynamic routing graph: %w", err)
	}
	
	return runnable.Invoke(drg.ctx, input)
}

func (drg *DynamicRoutingGraph) GetWorkflowContext() *WorkflowContext {
	return drg.wfCtx
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runAdvancedGraphPatternsDemo() {
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

	fmt.Println("=== บทที่ 9: Advanced Graph Patterns ===")
	fmt.Println("ตัวอย่างรูปแบบ Graph ขั้นสูงใน Eino")
	fmt.Println()

	// === Demo 1: Fan-Out/Fan-In Pattern ===
	fmt.Println("🔀 Demo 1: Fan-Out/Fan-In Pattern")
	
	fanOutFanInGraph := NewFanOutFanInGraph(model, ctx)
	fanOutFanInGraph.BuildFanOutFanInGraph()

	testInput := "Go เป็นภาษาโปรแกรมมิ่งที่พัฒนาโดย Google มีความเร็วสูงและเหมาะสำหรับการพัฒนา microservices และ cloud applications"

	fmt.Printf("Input: %s\n\n", testInput)
	
	result1, err := fanOutFanInGraph.Execute(testInput)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result:\n%s\n", result1)
	}

	// แสดง workflow context
	wfCtx1 := fanOutFanInGraph.GetWorkflowContext()
	fmt.Printf("\n📊 Workflow Context:\n")
	wfJSON1, _ := json.MarshalIndent(wfCtx1, "", "  ")
	fmt.Printf("%s\n", wfJSON1)

	fmt.Println(strings.Repeat("=", 80))

	// === Demo 2: Map-Reduce Pattern ===
	fmt.Println("\n🗺️ Demo 2: Map-Reduce Pattern")
	
	mapReduceGraph := NewMapReduceGraph(model, ctx)
	mapReduceGraph.BuildMapReduceGraph()

	testInputs := []string{
		"Go มีระบบ garbage collection ที่มีประสิทธิภาพ",
		"Goroutines ทำให้การเขียน concurrent programming ง่ายขึ้น",
		"Go modules ช่วยในการจัดการ dependencies",
		"Interface ใน Go ทำให้โค้ดมีความยืดหยุ่น",
	}

	fmt.Printf("Inputs (%d items):\n", len(testInputs))
	for i, input := range testInputs {
		fmt.Printf("  %d. %s\n", i+1, input)
	}
	fmt.Println()
	
	result2, err := mapReduceGraph.Execute(testInputs)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result:\n%s\n", result2)
	}

	// แสดง workflow context
	wfCtx2 := mapReduceGraph.GetWorkflowContext()
	fmt.Printf("\n📊 Map-Reduce Workflow:\n")
	for nodeName, result := range wfCtx2.Results {
		fmt.Printf("- %s: %v (success: %t)\n", nodeName, result.Duration, result.Success)
	}

	fmt.Println(strings.Repeat("=", 80))

	// === Demo 3: Dynamic Routing Pattern ===
	fmt.Println("\n🔀 Demo 3: Dynamic Routing Pattern")
	
	dynamicGraph := NewDynamicRoutingGraph(model, ctx)
	dynamicGraph.BuildDynamicRoutingGraph()

	testCases := []string{
		"ช่วยเขียน function สำหรับ sorting array ใน Go",
		"วิเคราะห์กลยุทธ์การตลาดสำหรับ startup",
		"อธิบายสถาปัตยกรรม microservices",
		"วิธีการทำ meditation ให้มีประสิทธิภาพ",
	}

	for i, testCase := range testCases {
		fmt.Printf("\n--- Dynamic Routing Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", testCase)
		
		result3, err := dynamicGraph.Execute(testCase)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Result: %s\n", result3[:100]+"...")
		}
		
		// แสดงเส้นทางที่เลือก
		wfCtx3 := dynamicGraph.GetWorkflowContext()
		if routerResult, exists := wfCtx3.GetResult("router"); exists {
			if route, ok := routerResult.Metadata["selected_route"]; ok {
				fmt.Printf("🎯 Selected Route: %s\n", route)
			}
		}
	}

	fmt.Println("\n✅ Advanced Graph Patterns Demo Complete!")
	fmt.Println("🎯 Key Patterns Demonstrated:")
	fmt.Println("   - Fan-Out/Fan-In: Parallel processing with aggregation")
	fmt.Println("   - Map-Reduce: Distributed processing pattern")
	fmt.Println("   - Dynamic Routing: Conditional execution paths")
	fmt.Println("   - Workflow Context: State management across patterns")
	fmt.Println("   - Task Result Tracking: Performance monitoring")
	fmt.Println("   - Pattern Composition: Combining multiple patterns")
}

func main() {
	runAdvancedGraphPatternsDemo()
}