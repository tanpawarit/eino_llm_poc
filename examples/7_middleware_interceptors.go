package main

import (
	"context"
	"crypto/md5"
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

// ตัวอย่างบทที่ 7: Middleware และ Interceptors
// เรียนรู้การสร้าง middleware และ interceptors สำหรับ Eino Graph

// MiddlewareContext - context สำหรับ middleware
type MiddlewareContext struct {
	RequestID    string                 `json:"request_id"`
	StartTime    time.Time              `json:"start_time"`
	UserAgent    string                 `json:"user_agent"`
	ClientIP     string                 `json:"client_ip"`
	Headers      map[string]string      `json:"headers"`
	Metadata     map[string]interface{} `json:"metadata"`
	Metrics      MiddlewareMetrics      `json:"metrics"`
	CacheEnabled bool                   `json:"cache_enabled"`
}

type MiddlewareMetrics struct {
	TotalRequests   int64         `json:"total_requests"`
	CacheHits       int64         `json:"cache_hits"`
	CacheMisses     int64         `json:"cache_misses"`
	AverageLatency  time.Duration `json:"average_latency"`
	ErrorCount      int64         `json:"error_count"`
	ThroughputPerSec float64      `json:"throughput_per_sec"`
}

// Middleware - interface สำหรับ middleware
type Middleware interface {
	Process(ctx context.Context, input string, next func(context.Context, string) (string, error), mCtx *MiddlewareContext) (string, error)
	Name() string
}

// Interceptor - interface สำหรับ interceptor
type Interceptor interface {
	BeforeExecution(ctx context.Context, input string, mCtx *MiddlewareContext) (string, error)
	AfterExecution(ctx context.Context, input, output string, mCtx *MiddlewareContext) (string, error)
	OnError(ctx context.Context, input string, err error, mCtx *MiddlewareContext) error
	Name() string
}

// CacheMiddleware - middleware สำหรับ caching
type CacheMiddleware struct {
	cache    sync.Map
	ttl      time.Duration
	hitCount int64
	missCount int64
}

func NewCacheMiddleware(ttl time.Duration) *CacheMiddleware {
	return &CacheMiddleware{
		cache: sync.Map{},
		ttl:   ttl,
	}
}

type CacheEntry struct {
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (cm *CacheMiddleware) Name() string {
	return "CacheMiddleware"
}

func (cm *CacheMiddleware) Process(ctx context.Context, input string, next func(context.Context, string) (string, error), mCtx *MiddlewareContext) (string, error) {
	if !mCtx.CacheEnabled {
		fmt.Printf("🔄 [%s] Cache disabled, skipping\n", cm.Name())
		return next(ctx, input)
	}

	// สร้าง cache key
	cacheKey := fmt.Sprintf("%x", md5.Sum([]byte(input)))
	
	// ตรวจสอบ cache
	if value, exists := cm.cache.Load(cacheKey); exists {
		entry := value.(CacheEntry)
		if time.Now().Before(entry.ExpiresAt) {
			cm.hitCount++
			mCtx.Metrics.CacheHits++
			fmt.Printf("✅ [%s] Cache HIT for key: %s\n", cm.Name(), cacheKey[:8])
			return entry.Value, nil
		} else {
			// ลบ cache ที่หมดอายุ
			cm.cache.Delete(cacheKey)
		}
	}

	// Cache miss - ดำเนินการต่อ
	cm.missCount++
	mCtx.Metrics.CacheMisses++
	fmt.Printf("❌ [%s] Cache MISS for key: %s\n", cm.Name(), cacheKey[:8])
	
	result, err := next(ctx, input)
	if err != nil {
		return "", err
	}

	// บันทึกใน cache
	entry := CacheEntry{
		Value:     result,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(cm.ttl),
	}
	cm.cache.Store(cacheKey, entry)
	fmt.Printf("💾 [%s] Cached result for key: %s\n", cm.Name(), cacheKey[:8])

	return result, nil
}

func (cm *CacheMiddleware) GetStats() (int64, int64) {
	return cm.hitCount, cm.missCount
}

// RateLimitMiddleware - middleware สำหรับ rate limiting
type RateLimitMiddleware struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

func NewRateLimitMiddleware(limit int, window time.Duration) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rlm *RateLimitMiddleware) Name() string {
	return "RateLimitMiddleware"
}

func (rlm *RateLimitMiddleware) Process(ctx context.Context, input string, next func(context.Context, string) (string, error), mCtx *MiddlewareContext) (string, error) {
	clientID := mCtx.ClientIP
	if clientID == "" {
		clientID = "default"
	}

	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	now := time.Now()
	
	// ลบ requests ที่เก่าเกินไป
	if requests, exists := rlm.requests[clientID]; exists {
		validRequests := make([]time.Time, 0)
		for _, reqTime := range requests {
			if now.Sub(reqTime) < rlm.window {
				validRequests = append(validRequests, reqTime)
			}
		}
		rlm.requests[clientID] = validRequests
	}

	// ตรวจสอบ rate limit
	if len(rlm.requests[clientID]) >= rlm.limit {
		fmt.Printf("🚫 [%s] Rate limit exceeded for client: %s\n", rlm.Name(), clientID)
		return "", fmt.Errorf("rate limit exceeded: %d requests per %v", rlm.limit, rlm.window)
	}

	// เพิ่ม request ปัจจุบัน
	rlm.requests[clientID] = append(rlm.requests[clientID], now)
	fmt.Printf("✅ [%s] Request allowed for client: %s (%d/%d)\n", rlm.Name(), clientID, len(rlm.requests[clientID]), rlm.limit)

	return next(ctx, input)
}

// LoggingInterceptor - interceptor สำหรับ logging
type LoggingInterceptor struct {
	logFile *os.File
}

func NewLoggingInterceptor(logPath string) (*LoggingInterceptor, error) {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	
	return &LoggingInterceptor{logFile: file}, nil
}

func (li *LoggingInterceptor) Name() string {
	return "LoggingInterceptor"
}

func (li *LoggingInterceptor) BeforeExecution(ctx context.Context, input string, mCtx *MiddlewareContext) (string, error) {
	logEntry := map[string]interface{}{
		"timestamp":   time.Now(),
		"request_id":  mCtx.RequestID,
		"event":       "before_execution",
		"input_size":  len(input),
		"client_ip":   mCtx.ClientIP,
		"user_agent":  mCtx.UserAgent,
	}

	logJSON, _ := json.Marshal(logEntry)
	li.logFile.WriteString(string(logJSON) + "\n")
	
	fmt.Printf("📝 [%s] Logged before execution\n", li.Name())
	return input, nil
}

func (li *LoggingInterceptor) AfterExecution(ctx context.Context, input, output string, mCtx *MiddlewareContext) (string, error) {
	duration := time.Since(mCtx.StartTime)
	
	logEntry := map[string]interface{}{
		"timestamp":    time.Now(),
		"request_id":   mCtx.RequestID,
		"event":        "after_execution",
		"input_size":   len(input),
		"output_size":  len(output),
		"duration_ms":  duration.Milliseconds(),
		"success":      true,
	}

	logJSON, _ := json.Marshal(logEntry)
	li.logFile.WriteString(string(logJSON) + "\n")
	
	fmt.Printf("📝 [%s] Logged after execution (took %v)\n", li.Name(), duration)
	return output, nil
}

func (li *LoggingInterceptor) OnError(ctx context.Context, input string, err error, mCtx *MiddlewareContext) error {
	logEntry := map[string]interface{}{
		"timestamp":  time.Now(),
		"request_id": mCtx.RequestID,
		"event":      "error",
		"error":      err.Error(),
		"input_size": len(input),
	}

	logJSON, _ := json.Marshal(logEntry)
	li.logFile.WriteString(string(logJSON) + "\n")
	
	fmt.Printf("🚨 [%s] Logged error: %v\n", li.Name(), err)
	return err
}

func (li *LoggingInterceptor) Close() {
	if li.logFile != nil {
		li.logFile.Close()
	}
}

// MetricsInterceptor - interceptor สำหรับเก็บ metrics
type MetricsInterceptor struct {
	totalRequests int64
	totalErrors   int64
	totalLatency  time.Duration
	mu            sync.Mutex
}

func NewMetricsInterceptor() *MetricsInterceptor {
	return &MetricsInterceptor{}
}

func (mi *MetricsInterceptor) Name() string {
	return "MetricsInterceptor"
}

func (mi *MetricsInterceptor) BeforeExecution(ctx context.Context, input string, mCtx *MiddlewareContext) (string, error) {
	mi.mu.Lock()
	mi.totalRequests++
	mCtx.Metrics.TotalRequests = mi.totalRequests
	mi.mu.Unlock()
	
	fmt.Printf("📊 [%s] Request #%d started\n", mi.Name(), mi.totalRequests)
	return input, nil
}

func (mi *MetricsInterceptor) AfterExecution(ctx context.Context, input, output string, mCtx *MiddlewareContext) (string, error) {
	duration := time.Since(mCtx.StartTime)
	
	mi.mu.Lock()
	mi.totalLatency += duration
	mCtx.Metrics.AverageLatency = mi.totalLatency / time.Duration(mi.totalRequests)
	mCtx.Metrics.ErrorCount = mi.totalErrors
	mi.mu.Unlock()
	
	fmt.Printf("📊 [%s] Request completed in %v (avg: %v)\n", mi.Name(), duration, mCtx.Metrics.AverageLatency)
	return output, nil
}

func (mi *MetricsInterceptor) OnError(ctx context.Context, input string, err error, mCtx *MiddlewareContext) error {
	mi.mu.Lock()
	mi.totalErrors++
	mCtx.Metrics.ErrorCount = mi.totalErrors
	mi.mu.Unlock()
	
	fmt.Printf("📊 [%s] Error #%d recorded\n", mi.Name(), mi.totalErrors)
	return err
}

func (mi *MetricsInterceptor) GetStats() (int64, int64, time.Duration) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	
	var avgLatency time.Duration
	if mi.totalRequests > 0 {
		avgLatency = mi.totalLatency / time.Duration(mi.totalRequests)
	}
	
	return mi.totalRequests, mi.totalErrors, avgLatency
}

// MiddlewareChain - chain ของ middleware และ interceptors
type MiddlewareChain struct {
	middlewares  []Middleware
	interceptors []Interceptor
	processor    func(context.Context, string) (string, error)
	mCtx         *MiddlewareContext
}

func NewMiddlewareChain(processor func(context.Context, string) (string, error)) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares:  make([]Middleware, 0),
		interceptors: make([]Interceptor, 0),
		processor:    processor,
		mCtx: &MiddlewareContext{
			StartTime:    time.Now(),
			Headers:      make(map[string]string),
			Metadata:     make(map[string]interface{}),
			CacheEnabled: true,
		},
	}
}

func (mc *MiddlewareChain) AddMiddleware(middleware Middleware) {
	mc.middlewares = append(mc.middlewares, middleware)
}

func (mc *MiddlewareChain) AddInterceptor(interceptor Interceptor) {
	mc.interceptors = append(mc.interceptors, interceptor)
}

func (mc *MiddlewareChain) SetContext(mCtx *MiddlewareContext) {
	mc.mCtx = mCtx
}

func (mc *MiddlewareChain) Execute(ctx context.Context, input string) (string, error) {
	mc.mCtx.StartTime = time.Now()
	mc.mCtx.RequestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	
	// Before interceptors
	processedInput := input
	for _, interceptor := range mc.interceptors {
		var err error
		processedInput, err = interceptor.BeforeExecution(ctx, processedInput, mc.mCtx)
		if err != nil {
			// Call error handlers
			for _, errInterceptor := range mc.interceptors {
				errInterceptor.OnError(ctx, input, err, mc.mCtx)
			}
			return "", err
		}
	}

	// สร้าง middleware chain
	finalProcessor := mc.processor
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		middleware := mc.middlewares[i]
		nextProcessor := finalProcessor
		finalProcessor = func(ctx context.Context, input string) (string, error) {
			return middleware.Process(ctx, input, nextProcessor, mc.mCtx)
		}
	}

	// Execute middleware chain
	result, err := finalProcessor(ctx, processedInput)
	if err != nil {
		// Call error handlers
		for _, interceptor := range mc.interceptors {
			interceptor.OnError(ctx, input, err, mc.mCtx)
		}
		return "", err
	}

	// After interceptors
	processedResult := result
	for _, interceptor := range mc.interceptors {
		processedResult, err = interceptor.AfterExecution(ctx, processedInput, processedResult, mc.mCtx)
		if err != nil {
			// Call error handlers
			for _, errInterceptor := range mc.interceptors {
				errInterceptor.OnError(ctx, input, err, mc.mCtx)
			}
			return "", err
		}
	}

	return processedResult, nil
}

func (mc *MiddlewareChain) GetContext() *MiddlewareContext {
	return mc.mCtx
}

// EnhancedGraphWithMiddleware - Graph ที่มี middleware
type EnhancedGraphWithMiddleware struct {
	graph   *compose.Graph[string, string]
	chains  map[string]*MiddlewareChain
	model   *openai.ChatModel
	ctx     context.Context
}

func NewEnhancedGraphWithMiddleware(model *openai.ChatModel, ctx context.Context) *EnhancedGraphWithMiddleware {
	return &EnhancedGraphWithMiddleware{
		graph:  compose.NewGraph[string, string](),
		chains: make(map[string]*MiddlewareChain),
		model:  model,
		ctx:    ctx,
	}
}

func (egm *EnhancedGraphWithMiddleware) BuildMiddlewareGraph() error {
	// สร้าง middleware และ interceptors
	cacheMiddleware := NewCacheMiddleware(5 * time.Minute)
	rateLimitMiddleware := NewRateLimitMiddleware(5, time.Minute)
	
	loggingInterceptor, err := NewLoggingInterceptor("/tmp/eino_middleware.log")
	if err != nil {
		return err
	}
	
	metricsInterceptor := NewMetricsInterceptor()

	// Node 1: Analyzer with middleware
	analyzerProcessor := func(ctx context.Context, input string) (string, error) {
		messages := []*schema.Message{
			schema.SystemMessage("วิเคราะห์และสรุปข้อความที่ได้รับ"),
			schema.UserMessage(input),
		}

		response, err := egm.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Analysis: %s", response.Content), nil
	}

	analyzerChain := NewMiddlewareChain(analyzerProcessor)
	analyzerChain.AddMiddleware(cacheMiddleware)
	analyzerChain.AddMiddleware(rateLimitMiddleware)
	analyzerChain.AddInterceptor(loggingInterceptor)
	analyzerChain.AddInterceptor(metricsInterceptor)
	
	mCtx1 := &MiddlewareContext{
		UserAgent:    "EinoBot/1.0",
		ClientIP:     "192.168.1.100",
		CacheEnabled: true,
		Headers:      map[string]string{"Content-Type": "application/json"},
		Metadata:     make(map[string]interface{}),
	}
	analyzerChain.SetContext(mCtx1)
	egm.chains["analyzer"] = analyzerChain

	// Node 2: Enhancer with different middleware
	enhancerProcessor := func(ctx context.Context, input string) (string, error) {
		messages := []*schema.Message{
			schema.SystemMessage("ปรับปรุงและขยายข้อมูลให้ละเอียดขึ้น"),
			schema.UserMessage(input),
		}

		response, err := egm.model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Enhanced: %s", response.Content), nil
	}

	enhancerChain := NewMiddlewareChain(enhancerProcessor)
	enhancerChain.AddMiddleware(cacheMiddleware) // ใช้ cache เดียวกัน
	enhancerChain.AddInterceptor(loggingInterceptor) // ใช้ logging เดียวกัน
	enhancerChain.AddInterceptor(metricsInterceptor) // ใช้ metrics เดียวกัน
	
	mCtx2 := &MiddlewareContext{
		UserAgent:    "EinoBot/1.0",
		ClientIP:     "192.168.1.101",
		CacheEnabled: true,
		Headers:      map[string]string{"Content-Type": "application/json"},
		Metadata:     make(map[string]interface{}),
	}
	enhancerChain.SetContext(mCtx2)
	egm.chains["enhancer"] = enhancerChain

	// สร้าง wrapper functions สำหรับ graph
	analyzerWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return egm.chains["analyzer"].Execute(ctx, input)
	})

	enhancerWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return egm.chains["enhancer"].Execute(ctx, input)
	})

	// เพิ่ม nodes ใน graph
	egm.graph.AddLambdaNode("analyzer", analyzerWrapper)
	egm.graph.AddLambdaNode("enhancer", enhancerWrapper)

	// เชื่อม edges
	egm.graph.AddEdge(compose.START, "analyzer")
	egm.graph.AddEdge("analyzer", "enhancer")
	egm.graph.AddEdge("enhancer", compose.END)

	return nil
}

func (egm *EnhancedGraphWithMiddleware) Execute(input string) (string, error) {
	runnable, err := egm.graph.Compile(egm.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compile graph: %w", err)
	}

	return runnable.Invoke(egm.ctx, input)
}

func (egm *EnhancedGraphWithMiddleware) GetMiddlewareChain(nodeName string) *MiddlewareChain {
	return egm.chains[nodeName]
}

func runMiddlewareInterceptorsDemo() {
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

	fmt.Println("=== บทที่ 7: Middleware และ Interceptors ===")
	fmt.Println("ตัวอย่างการใช้ Middleware และ Interceptors ใน Eino Graph")
	fmt.Println()

	// === Demo 1: Basic Middleware Chain ===
	fmt.Println("🔧 Demo 1: Basic Middleware Chain")

	enhancedGraph := NewEnhancedGraphWithMiddleware(model, ctx)
	err = enhancedGraph.BuildMiddlewareGraph()
	if err != nil {
		fmt.Printf("Error building middleware graph: %v\n", err)
		return
	}

	testInputs := []string{
		"อธิบายเกี่ยวกับ Go programming",
		"วิธีการใช้ goroutines",
		"อธิบายเกี่ยวกับ Go programming", // ทดสอบ cache
		"การจัดการ error ใน Go",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)

		result, err := enhancedGraph.Execute(input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Result: %s\n", strings.Split(result, "\n")[0]+"...") // แสดงบางส่วน
		fmt.Println(strings.Repeat("-", 60))
	}

	// === Demo 2: Middleware Statistics ===
	fmt.Println("\n📊 Demo 2: Middleware Statistics")

	// Cache statistics
	if analyzerChain := enhancedGraph.GetMiddlewareChain("analyzer"); analyzerChain != nil {
		for _, middleware := range analyzerChain.middlewares {
			if cacheMiddleware, ok := middleware.(*CacheMiddleware); ok {
				hits, misses := cacheMiddleware.GetStats()
				fmt.Printf("Cache Statistics: %d hits, %d misses (%.1f%% hit rate)\n", 
					hits, misses, float64(hits)/float64(hits+misses)*100)
			}
		}

		// Metrics statistics
		for _, interceptor := range analyzerChain.interceptors {
			if metricsInterceptor, ok := interceptor.(*MetricsInterceptor); ok {
				totalReqs, totalErrs, avgLatency := metricsInterceptor.GetStats()
				fmt.Printf("Metrics: %d requests, %d errors, avg latency: %v\n", 
					totalReqs, totalErrs, avgLatency)
			}
		}

		// Context information
		mCtx := analyzerChain.GetContext()
		fmt.Printf("Final Context Metrics: %+v\n", mCtx.Metrics)
	}

	// === Demo 3: Rate Limiting Test ===
	fmt.Println("\n🚫 Demo 3: Rate Limiting Test")

	// ทดสอบ rate limiting โดยส่ง request เกินขีดจำกัด
	for i := 0; i < 8; i++ {
		fmt.Printf("\nRate Limit Test %d:\n", i+1)
		
		result, err := enhancedGraph.Execute(fmt.Sprintf("Test request %d", i+1))
		if err != nil {
			fmt.Printf("Error (expected after 5 requests): %v\n", err)
		} else {
			fmt.Printf("Success: %s\n", result[:50]+"...")
		}
	}

	// ทำความสะอาด log file
	defer func() {
		if analyzerChain := enhancedGraph.GetMiddlewareChain("analyzer"); analyzerChain != nil {
			for _, interceptor := range analyzerChain.interceptors {
				if loggingInterceptor, ok := interceptor.(*LoggingInterceptor); ok {
					loggingInterceptor.Close()
				}
			}
		}
		
		// แสดง log file content
		if logContent, err := os.ReadFile("/tmp/eino_middleware.log"); err == nil {
			fmt.Println("\n📜 Log File Sample (last 200 chars):")
			content := string(logContent)
			if len(content) > 200 {
				content = "..." + content[len(content)-200:]
			}
			fmt.Printf("%s\n", content)
		}
	}()

	fmt.Println("\n✅ Middleware และ Interceptors Demo Complete!")
	fmt.Println("🎯 Key Concepts Demonstrated:")
	fmt.Println("   - Cache middleware for performance")
	fmt.Println("   - Rate limiting for protection")
	fmt.Println("   - Logging interceptor for monitoring")
	fmt.Println("   - Metrics collection for analytics")
	fmt.Println("   - Middleware chaining and composition")
	fmt.Println("   - Error handling across interceptors")
	fmt.Println("   - Context sharing between components")
}

func main() {
	runMiddlewareInterceptorsDemo()
}