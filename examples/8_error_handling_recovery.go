package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// ตัวอย่างบทที่ 8: Error Handling และ Recovery
// เรียนรู้การจัดการ Error และ Recovery ใน Eino Graph

// ErrorType - ประเภทของ error
type ErrorType string

const (
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeRateLimit  ErrorType = "rate_limit"
	ErrorTypeInternal   ErrorType = "internal"
	ErrorTypeUnknown    ErrorType = "unknown"
)

// ErrorContext - ข้อมูล context ของ error
type ErrorContext struct {
	NodeName    string                 `json:"node_name"`
	ErrorType   ErrorType              `json:"error_type"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	RetryCount  int                    `json:"retry_count"`
	Input       string                 `json:"input"`
	Metadata    map[string]interface{} `json:"metadata"`
	Recoverable bool                   `json:"recoverable"`
}

// RecoveryStrategy - กลยุทธ์การ recovery
type RecoveryStrategy interface {
	CanRecover(errorCtx *ErrorContext) bool
	Recover(ctx context.Context, errorCtx *ErrorContext) (string, error)
	Name() string
}

// RetryStrategy - กลยุทธ์การ retry
type RetryStrategy struct {
	maxRetries    int
	baseDelay     time.Duration
	maxDelay      time.Duration
	backoffFactor float64
}

func NewRetryStrategy(maxRetries int, baseDelay, maxDelay time.Duration, backoffFactor float64) *RetryStrategy {
	return &RetryStrategy{
		maxRetries:    maxRetries,
		baseDelay:     baseDelay,
		maxDelay:      maxDelay,
		backoffFactor: backoffFactor,
	}
}

func (rs *RetryStrategy) Name() string {
	return "RetryStrategy"
}

func (rs *RetryStrategy) CanRecover(errorCtx *ErrorContext) bool {
	// สามารถ retry ได้ถ้ายังไม่เกิน max retries และเป็น error ที่ recoverable
	retryableTypes := []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeRateLimit}
	
	for _, errType := range retryableTypes {
		if errorCtx.ErrorType == errType && errorCtx.RetryCount < rs.maxRetries {
			return true
		}
	}
	return false
}

func (rs *RetryStrategy) Recover(ctx context.Context, errorCtx *ErrorContext) (string, error) {
	delay := rs.calculateDelay(errorCtx.RetryCount)
	
	fmt.Printf("🔄 [%s] Retrying after %v (attempt %d/%d)\n", 
		rs.Name(), delay, errorCtx.RetryCount+1, rs.maxRetries)
	
	time.Sleep(delay)
	return "", fmt.Errorf("retry needed")
}

func (rs *RetryStrategy) calculateDelay(retryCount int) time.Duration {
	delay := time.Duration(float64(rs.baseDelay) * rs.backoffFactor * float64(retryCount))
	if delay > rs.maxDelay {
		delay = rs.maxDelay
	}
	return delay
}

// FallbackStrategy - กลยุทธ์การ fallback
type FallbackStrategy struct {
	fallbackResponse string
}

func NewFallbackStrategy(fallbackResponse string) *FallbackStrategy {
	return &FallbackStrategy{
		fallbackResponse: fallbackResponse,
	}
}

func (fs *FallbackStrategy) Name() string {
	return "FallbackStrategy"
}

func (fs *FallbackStrategy) CanRecover(errorCtx *ErrorContext) bool {
	// สามารถ fallback ได้เสมอเป็น last resort
	return true
}

func (fs *FallbackStrategy) Recover(ctx context.Context, errorCtx *ErrorContext) (string, error) {
	fmt.Printf("🛡️ [%s] Using fallback response\n", fs.Name())
	
	fallback := fmt.Sprintf("%s\n\n⚠️ Note: This is a fallback response due to: %s", 
		fs.fallbackResponse, errorCtx.Message)
	
	return fallback, nil
}

// CircuitBreakerStrategy - กลยุทธ์ circuit breaker
type CircuitBreakerStrategy struct {
	failureThreshold int
	resetTimeout     time.Duration
	failures         map[string]int
	lastFailureTime  map[string]time.Time
	mu               sync.RWMutex
}

func NewCircuitBreakerStrategy(failureThreshold int, resetTimeout time.Duration) *CircuitBreakerStrategy {
	return &CircuitBreakerStrategy{
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		failures:         make(map[string]int),
		lastFailureTime:  make(map[string]time.Time),
	}
}

func (cbs *CircuitBreakerStrategy) Name() string {
	return "CircuitBreakerStrategy"
}

func (cbs *CircuitBreakerStrategy) CanRecover(errorCtx *ErrorContext) bool {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()
	
	nodeKey := errorCtx.NodeName
	
	// ตรวจสอบว่า circuit breaker เปิดอยู่หรือไม่
	if failures, exists := cbs.failures[nodeKey]; exists {
		if failures >= cbs.failureThreshold {
			// ตรวจสอบว่าควรรีเซ็ตหรือไม่
			if lastFailure, hasTime := cbs.lastFailureTime[nodeKey]; hasTime {
				if time.Since(lastFailure) > cbs.resetTimeout {
					return true // พร้อมรีเซ็ต
				}
				return false // circuit breaker ยังเปิดอยู่
			}
		}
	}
	
	return false
}

func (cbs *CircuitBreakerStrategy) Recover(ctx context.Context, errorCtx *ErrorContext) (string, error) {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()
	
	nodeKey := errorCtx.NodeName
	
	// รีเซ็ต circuit breaker
	delete(cbs.failures, nodeKey)
	delete(cbs.lastFailureTime, nodeKey)
	
	fmt.Printf("⚡ [%s] Circuit breaker reset for node: %s\n", cbs.Name(), nodeKey)
	
	return "", fmt.Errorf("circuit breaker reset, retry needed")
}

func (cbs *CircuitBreakerStrategy) RecordFailure(nodeName string) {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()
	
	cbs.failures[nodeName]++
	cbs.lastFailureTime[nodeName] = time.Now()
	
	if cbs.failures[nodeName] >= cbs.failureThreshold {
		fmt.Printf("🚨 [%s] Circuit breaker OPENED for node: %s (%d failures)\n", 
			cbs.Name(), nodeName, cbs.failures[nodeName])
	}
}

func (cbs *CircuitBreakerStrategy) IsCircuitOpen(nodeName string) bool {
	cbs.mu.RLock()
	defer cbs.mu.RUnlock()
	
	if failures, exists := cbs.failures[nodeName]; exists {
		return failures >= cbs.failureThreshold
	}
	return false
}

// ErrorRecoveryManager - จัดการ error recovery
type ErrorRecoveryManager struct {
	strategies []RecoveryStrategy
	circuitBreaker *CircuitBreakerStrategy
}

func NewErrorRecoveryManager(circuitBreaker *CircuitBreakerStrategy) *ErrorRecoveryManager {
	return &ErrorRecoveryManager{
		strategies:     make([]RecoveryStrategy, 0),
		circuitBreaker: circuitBreaker,
	}
}

func (erm *ErrorRecoveryManager) AddStrategy(strategy RecoveryStrategy) {
	erm.strategies = append(erm.strategies, strategy)
}

func (erm *ErrorRecoveryManager) HandleError(ctx context.Context, nodeName string, err error, input string) (string, error) {
	// วิเคราะห์ error
	errorCtx := erm.analyzeError(nodeName, err, input)
	
	fmt.Printf("🚨 [ErrorRecoveryManager] Handling error in %s: %s\n", nodeName, errorCtx.Message)
	
	// ลองใช้ recovery strategies
	for _, strategy := range erm.strategies {
		if strategy.CanRecover(errorCtx) {
			fmt.Printf("🔧 [ErrorRecoveryManager] Trying %s\n", strategy.Name())
			
			result, recoveryErr := strategy.Recover(ctx, errorCtx)
			
			if recoveryErr == nil {
				fmt.Printf("✅ [ErrorRecoveryManager] Recovered successfully with %s\n", strategy.Name())
				return result, nil
			}
			
			if recoveryErr.Error() == "retry needed" {
				errorCtx.RetryCount++
				return "", fmt.Errorf("retry required")
			}
			
			if recoveryErr.Error() == "circuit breaker reset, retry needed" {
				return "", fmt.Errorf("retry after circuit breaker reset")
			}
		}
	}
	
	// ถ้าไม่สามารถ recover ได้ บันทึกใน circuit breaker
	if erm.circuitBreaker != nil {
		erm.circuitBreaker.RecordFailure(nodeName)
	}
	
	return "", fmt.Errorf("unable to recover from error: %v", err)
}

func (erm *ErrorRecoveryManager) analyzeError(nodeName string, err error, input string) *ErrorContext {
	errorMsg := err.Error()
	
	var errorType ErrorType
	var recoverable bool
	
	switch {
	case strings.Contains(errorMsg, "network") || strings.Contains(errorMsg, "connection"):
		errorType = ErrorTypeNetwork
		recoverable = true
	case strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "deadline exceeded"):
		errorType = ErrorTypeTimeout
		recoverable = true
	case strings.Contains(errorMsg, "rate limit") || strings.Contains(errorMsg, "too many requests"):
		errorType = ErrorTypeRateLimit
		recoverable = true
	case strings.Contains(errorMsg, "validation") || strings.Contains(errorMsg, "invalid"):
		errorType = ErrorTypeValidation
		recoverable = false
	default:
		errorType = ErrorTypeInternal
		recoverable = true
	}
	
	return &ErrorContext{
		NodeName:    nodeName,
		ErrorType:   errorType,
		Message:     errorMsg,
		Timestamp:   time.Now(),
		RetryCount:  0,
		Input:       input,
		Metadata:    make(map[string]interface{}),
		Recoverable: recoverable,
	}
}

// ResilientNode - Node ที่มีความทนทานต่อ error
type ResilientNode struct {
	nodeName     string
	processor    func(context.Context, string) (string, error)
	errorManager *ErrorRecoveryManager
	maxRetries   int
}

func NewResilientNode(nodeName string, processor func(context.Context, string) (string, error), errorManager *ErrorRecoveryManager, maxRetries int) *ResilientNode {
	return &ResilientNode{
		nodeName:     nodeName,
		processor:    processor,
		errorManager: errorManager,
		maxRetries:   maxRetries,
	}
}

func (rn *ResilientNode) Process(ctx context.Context, input string) (string, error) {
	retryCount := 0
	
	for retryCount <= rn.maxRetries {
		// ตรวจสอบ circuit breaker
		if rn.errorManager.circuitBreaker != nil && rn.errorManager.circuitBreaker.IsCircuitOpen(rn.nodeName) {
			fmt.Printf("⚡ [%s] Circuit breaker is OPEN, skipping\n", rn.nodeName)
			return "", fmt.Errorf("circuit breaker open for node %s", rn.nodeName)
		}
		
		fmt.Printf("🔄 [%s] Processing attempt %d\n", rn.nodeName, retryCount+1)
		
		result, err := rn.processor(ctx, input)
		if err == nil {
			if retryCount > 0 {
				fmt.Printf("✅ [%s] Succeeded after %d retries\n", rn.nodeName, retryCount)
			}
			return result, nil
		}
		
		fmt.Printf("❌ [%s] Error: %v\n", rn.nodeName, err)
		
		// พยายาม recover
		recoveredResult, recoveryErr := rn.errorManager.HandleError(ctx, rn.nodeName, err, input)
		if recoveryErr == nil {
			return recoveredResult, nil
		}
		
		if strings.Contains(recoveryErr.Error(), "retry") {
			retryCount++
			continue
		}
		
		return "", err
	}
	
	return "", fmt.Errorf("max retries exceeded for node %s", rn.nodeName)
}

// UnreliableProcessor - ตัวประมวลผลที่ไม่เสถียร (สำหรับทดสอบ)
func CreateUnreliableProcessor(model *openai.ChatModel, failureRate float64, errorType ErrorType) func(context.Context, string) (string, error) {
	return func(ctx context.Context, input string) (string, error) {
		// จำลองความผิดพลาด
		if rand.Float64() < failureRate {
			switch errorType {
			case ErrorTypeNetwork:
				return "", errors.New("network connection failed")
			case ErrorTypeTimeout:
				return "", errors.New("request timeout exceeded")
			case ErrorTypeRateLimit:
				return "", errors.New("rate limit exceeded - too many requests")
			case ErrorTypeValidation:
				return "", errors.New("validation error - invalid input format")
			default:
				return "", errors.New("internal server error")
			}
		}
		
		// ประมวลผลปกติ
		messages := []*schema.Message{
			schema.SystemMessage("ประมวลผลข้อมูลที่ได้รับ"),
			schema.UserMessage(input),
		}
		
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", err
		}
		
		return response.Content, nil
	}
}

// ResilientGraph - Graph ที่มีความทนทานต่อ error
type ResilientGraph struct {
	graph        *compose.Graph[string, string]
	nodes        map[string]*ResilientNode
	errorManager *ErrorRecoveryManager
	model        *openai.ChatModel
	ctx          context.Context
}

func NewResilientGraph(model *openai.ChatModel, ctx context.Context) *ResilientGraph {
	circuitBreaker := NewCircuitBreakerStrategy(3, 30*time.Second)
	errorManager := NewErrorRecoveryManager(circuitBreaker)
	
	// เพิ่ม recovery strategies
	retryStrategy := NewRetryStrategy(3, 1*time.Second, 10*time.Second, 2.0)
	fallbackStrategy := NewFallbackStrategy("ขออภัย เกิดข้อผิดพลาดในการประมวลผล กรุณาลองใหม่อีกครั้ง")
	
	errorManager.AddStrategy(retryStrategy)
	errorManager.AddStrategy(circuitBreaker)
	errorManager.AddStrategy(fallbackStrategy)
	
	return &ResilientGraph{
		graph:        compose.NewGraph[string, string](),
		nodes:        make(map[string]*ResilientNode),
		errorManager: errorManager,
		model:        model,
		ctx:          ctx,
	}
}

func (rg *ResilientGraph) BuildResilientGraph() {
	// Node 1: Unreliable Analyzer (50% failure rate)
	analyzerProcessor := CreateUnreliableProcessor(rg.model, 0.5, ErrorTypeNetwork)
	analyzerNode := NewResilientNode("UnreliableAnalyzer", analyzerProcessor, rg.errorManager, 3)
	rg.nodes["analyzer"] = analyzerNode
	
	// Node 2: Timeout-prone Enhancer (30% failure rate)
	enhancerProcessor := CreateUnreliableProcessor(rg.model, 0.3, ErrorTypeTimeout)
	enhancerNode := NewResilientNode("TimeoutEnhancer", enhancerProcessor, rg.errorManager, 3)
	rg.nodes["enhancer"] = enhancerNode
	
	// Node 3: Rate-limited Formatter (20% failure rate)
	formatterProcessor := CreateUnreliableProcessor(rg.model, 0.2, ErrorTypeRateLimit)
	formatterNode := NewResilientNode("RateLimitedFormatter", formatterProcessor, rg.errorManager, 3)
	rg.nodes["formatter"] = formatterNode
	
	// สร้าง wrapper functions
	analyzerWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return rg.nodes["analyzer"].Process(ctx, input)
	})
	
	enhancerWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return rg.nodes["enhancer"].Process(ctx, input)
	})
	
	formatterWrapper := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return rg.nodes["formatter"].Process(ctx, input)
	})
	
	// เพิ่ม nodes ใน graph
	rg.graph.AddLambdaNode("analyzer", analyzerWrapper)
	rg.graph.AddLambdaNode("enhancer", enhancerWrapper)
	rg.graph.AddLambdaNode("formatter", formatterWrapper)
	
	// เชื่อม edges
	rg.graph.AddEdge(compose.START, "analyzer")
	rg.graph.AddEdge("analyzer", "enhancer")
	rg.graph.AddEdge("enhancer", "formatter")
	rg.graph.AddEdge("formatter", compose.END)
}

func (rg *ResilientGraph) Execute(input string) (string, error) {
	runnable, err := rg.graph.Compile(rg.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compile graph: %w", err)
	}
	
	return runnable.Invoke(rg.ctx, input)
}

func (rg *ResilientGraph) GetErrorManager() *ErrorRecoveryManager {
	return rg.errorManager
}

func runErrorHandlingRecoveryDemo() {
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

	fmt.Println("=== บทที่ 8: Error Handling และ Recovery ===")
	fmt.Println("ตัวอย่างการจัดการ Error และ Recovery ใน Eino Graph")
	fmt.Println()

	// === Demo 1: Basic Error Recovery ===
	fmt.Println("🛡️ Demo 1: Basic Error Recovery")

	resilientGraph := NewResilientGraph(model, ctx)
	resilientGraph.BuildResilientGraph()

	testInputs := []string{
		"อธิบายเกี่ยวกับ Go programming",
		"วิธีการใช้ goroutines ใน Go",
		"การจัดการ error ใน Go",
		"ความแตกต่างระหว่าง channels และ mutexes",
		"การเขียน unit tests ใน Go",
	}

	successCount := 0
	for i, input := range testInputs {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)

		result, err := resilientGraph.Execute(input)
		if err != nil {
			fmt.Printf("Final Error: %v\n", err)
		} else {
			fmt.Printf("Success: %s\n", result[:100]+"...")
			successCount++
		}
		fmt.Println(strings.Repeat("-", 80))
	}

	fmt.Printf("\n📊 Success Rate: %d/%d (%.1f%%)\n", 
		successCount, len(testInputs), float64(successCount)/float64(len(testInputs))*100)

	// === Demo 2: Circuit Breaker in Action ===
	fmt.Println("\n⚡ Demo 2: Circuit Breaker in Action")

	// ส่ง request หลายครั้งติดต่อกันเพื่อทำให้ circuit breaker เปิด
	for i := 0; i < 10; i++ {
		fmt.Printf("\nCircuit Breaker Test %d:\n", i+1)
		
		result, err := resilientGraph.Execute(fmt.Sprintf("Circuit breaker test %d", i+1))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Success: %s\n", result[:50]+"...")
		}
		
		// แสดงสถานะ circuit breaker
		cb := resilientGraph.GetErrorManager().circuitBreaker
		if cb.IsCircuitOpen("UnreliableAnalyzer") {
			fmt.Printf("🚨 Circuit breaker is OPEN for UnreliableAnalyzer\n")
		}
		if cb.IsCircuitOpen("TimeoutEnhancer") {
			fmt.Printf("🚨 Circuit breaker is OPEN for TimeoutEnhancer\n")
		}
		if cb.IsCircuitOpen("RateLimitedFormatter") {
			fmt.Printf("🚨 Circuit breaker is OPEN for RateLimitedFormatter\n")
		}
	}

	// === Demo 3: Recovery Strategy Testing ===
	fmt.Println("\n🔧 Demo 3: Recovery Strategy Testing")

	// ทดสอบ strategies แยกกัน
	strategies := resilientGraph.GetErrorManager().strategies
	for _, strategy := range strategies {
		fmt.Printf("\nTesting %s:\n", strategy.Name())
		
		// สร้าง mock error context
		mockErrorCtx := &ErrorContext{
			NodeName:    "TestNode",
			ErrorType:   ErrorTypeNetwork,
			Message:     "mock network error",
			Timestamp:   time.Now(),
			RetryCount:  0,
			Input:       "test input",
			Metadata:    make(map[string]interface{}),
			Recoverable: true,
		}
		
		if strategy.CanRecover(mockErrorCtx) {
			fmt.Printf("✅ Can recover with %s\n", strategy.Name())
			
			result, err := strategy.Recover(ctx, mockErrorCtx)
			if err != nil {
				if strings.Contains(err.Error(), "retry needed") {
					fmt.Printf("🔄 Strategy requests retry\n")
				} else {
					fmt.Printf("❌ Recovery failed: %v\n", err)
				}
			} else {
				fmt.Printf("✅ Recovery successful: %s\n", result[:50]+"...")
			}
		} else {
			fmt.Printf("❌ Cannot recover with %s\n", strategy.Name())
		}
	}

	fmt.Println("\n✅ Error Handling และ Recovery Demo Complete!")
	fmt.Println("🎯 Key Concepts Demonstrated:")
	fmt.Println("   - Retry strategy with exponential backoff")
	fmt.Println("   - Circuit breaker pattern")
	fmt.Println("   - Fallback mechanisms")
	fmt.Println("   - Error classification and recovery")
	fmt.Println("   - Resilient node design")
	fmt.Println("   - Recovery strategy chaining")
	fmt.Println("   - Failure rate simulation and testing")
}

func main() {
	runErrorHandlingRecoveryDemo()
}