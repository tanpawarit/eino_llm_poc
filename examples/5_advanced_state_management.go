package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// ตัวอย่างบทที่ 5: Advanced State Management
// เรียนรู้การจัดการ State ขั้นสูงในระบบ Eino Graph

// SharedState - State ที่แชร์ระหว่าง nodes
type SharedState struct {
	mu              sync.RWMutex
	conversationID  string
	userProfile     map[string]interface{}
	sessionData     map[string]interface{}
	contextHistory  []string
	processingSteps []ProcessingStep
	metrics         ProcessingMetrics
}

type ProcessingStep struct {
	NodeName  string    `json:"node_name"`
	Timestamp time.Time `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

type ProcessingMetrics struct {
	TotalSteps    int           `json:"total_steps"`
	SuccessSteps  int           `json:"success_steps"`
	TotalDuration time.Duration `json:"total_duration"`
	AverageTime   time.Duration `json:"average_time"`
}

func NewSharedState(conversationID string) *SharedState {
	return &SharedState{
		conversationID:  conversationID,
		userProfile:     make(map[string]interface{}),
		sessionData:     make(map[string]interface{}),
		contextHistory:  make([]string, 0),
		processingSteps: make([]ProcessingStep, 0),
		metrics:         ProcessingMetrics{},
	}
}

// Thread-safe methods for shared state
func (s *SharedState) SetUserProfile(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userProfile[key] = value
}

func (s *SharedState) GetUserProfile(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.userProfile[key]
	return value, exists
}

func (s *SharedState) SetSessionData(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionData[key] = value
}

func (s *SharedState) GetSessionData(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.sessionData[key]
	return value, exists
}

func (s *SharedState) AddContextHistory(context string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextHistory = append(s.contextHistory, context)
	
	// เก็บแค่ 10 context ล่าสุด
	if len(s.contextHistory) > 10 {
		s.contextHistory = s.contextHistory[1:]
	}
}

func (s *SharedState) GetContextHistory() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return copy to prevent race conditions
	history := make([]string, len(s.contextHistory))
	copy(history, s.contextHistory)
	return history
}

func (s *SharedState) AddProcessingStep(step ProcessingStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.processingSteps = append(s.processingSteps, step)
	s.updateMetrics()
}

func (s *SharedState) updateMetrics() {
	// อัพเดท metrics (ต้องเรียกใน lock แล้ว)
	s.metrics.TotalSteps = len(s.processingSteps)
	s.metrics.SuccessSteps = 0
	s.metrics.TotalDuration = 0
	
	for _, step := range s.processingSteps {
		if step.Success {
			s.metrics.SuccessSteps++
		}
		s.metrics.TotalDuration += step.Duration
	}
	
	if s.metrics.TotalSteps > 0 {
		s.metrics.AverageTime = s.metrics.TotalDuration / time.Duration(s.metrics.TotalSteps)
	}
}

func (s *SharedState) GetMetrics() ProcessingMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

// StatefulProcessor - ตัวประมวลผลที่มี state
type StatefulProcessor struct {
	nodeName string
	state    *SharedState
	model    *openai.ChatModel
}

func NewStatefulProcessor(nodeName string, state *SharedState, model *openai.ChatModel) *StatefulProcessor {
	return &StatefulProcessor{
		nodeName: nodeName,
		state:    state,
		model:    model,
	}
}

func (p *StatefulProcessor) Process(ctx context.Context, input string) (string, error) {
	startTime := time.Now()
	
	// สร้าง processing step
	step := ProcessingStep{
		NodeName:  p.nodeName,
		Timestamp: startTime,
		Success:   false,
	}
	
	defer func() {
		step.Duration = time.Since(startTime)
		p.state.AddProcessingStep(step)
	}()
	
	fmt.Printf("🔄 [%s] Processing: %s\n", p.nodeName, input)
	
	// ดึง context history
	history := p.state.GetContextHistory()
	
	// ดึง user profile
	userLang, _ := p.state.GetUserProfile("language")
	userRole, _ := p.state.GetUserProfile("role")
	
	// สร้าง context-aware prompt
	var contextPrompt string
	if len(history) > 0 {
		contextPrompt = fmt.Sprintf("Previous context: %v\n", history[len(history)-1:])
	}
	
	var rolePrompt string
	if userRole != nil {
		rolePrompt = fmt.Sprintf("User role: %s\n", userRole)
	}
	
	var langPrompt string
	if userLang != nil {
		langPrompt = fmt.Sprintf("Respond in: %s\n", userLang)
	}
	
	systemPrompt := fmt.Sprintf(`%s%s%sYou are processing in node: %s
Context: %s`, rolePrompt, langPrompt, contextPrompt, p.nodeName, input)
	
	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(input),
	}
	
	// เรียกใช้ model
	response, err := p.model.Generate(ctx, messages)
	if err != nil {
		step.ErrorMsg = err.Error()
		return "", fmt.Errorf("error in %s: %w", p.nodeName, err)
	}
	
	result := response.Content
	
	// อัพเดท state
	p.state.AddContextHistory(fmt.Sprintf("%s: %s", p.nodeName, result))
	p.state.SetSessionData(fmt.Sprintf("last_%s_result", p.nodeName), result)
	
	step.Success = true
	fmt.Printf("✅ [%s] Result: %s\n", p.nodeName, result)
	
	return result, nil
}

// StateAwareGraph - Graph ที่รู้จัก state
type StateAwareGraph struct {
	graph  *compose.Graph[string, string]
	state  *SharedState
	model  *openai.ChatModel
	ctx    context.Context
}

func NewStateAwareGraph(state *SharedState, model *openai.ChatModel, ctx context.Context) *StateAwareGraph {
	return &StateAwareGraph{
		graph: compose.NewGraph[string, string](),
		state: state,
		model: model,
		ctx:   ctx,
	}
}

func (sag *StateAwareGraph) BuildPersonalizedAssistantGraph() {
	// Node 1: User Profile Analyzer
	profileAnalyzer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		processor := NewStatefulProcessor("ProfileAnalyzer", sag.state, sag.model)
		
		// วิเคราะห์โปรไฟล์ผู้ใช้จากข้อความ
		prompt := fmt.Sprintf(`Analyze this user message and extract profile information:
"%s"

Extract:
- Language preference (thai/english/etc)
- User role/profession (if mentioned)
- Technical level (beginner/intermediate/advanced)
- Current topic/domain

Return as JSON format.`, input)
		
		result, err := processor.Process(ctx, prompt)
		if err != nil {
			return "", err
		}
		
		// ในระบบจริงจะ parse JSON และอัพเดท profile
		// สำหรับ demo เราจะ hardcode
		sag.state.SetUserProfile("language", "thai")
		sag.state.SetUserProfile("role", "developer")
		sag.state.SetUserProfile("level", "intermediate")
		
		return fmt.Sprintf("Profile updated: %s", result), nil
	})
	
	// Node 2: Context Builder
	contextBuilder := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		processor := NewStatefulProcessor("ContextBuilder", sag.state, sag.model)
		
		// สร้าง rich context จาก history และ profile
		history := sag.state.GetContextHistory()
		level, _ := sag.state.GetUserProfile("level")
		
		contextPrompt := fmt.Sprintf(`Build rich context for this request:
"%s"

User level: %v
Recent context: %v

Create a context-aware prompt that considers user's background and conversation history.`, 
			input, level, history)
		
		return processor.Process(ctx, contextPrompt)
	})
	
	// Node 3: Intelligent Response Generator
	responseGenerator := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		processor := NewStatefulProcessor("ResponseGenerator", sag.state, sag.model)
		
		// สร้างคำตอบที่ personalized
		role, _ := sag.state.GetUserProfile("role")
		lang, _ := sag.state.GetUserProfile("language")
		
		personalizedPrompt := fmt.Sprintf(`Generate a personalized response for:
"%s"

User is a %v who prefers %v language.
Make the response appropriate for their background and technical level.
Include practical examples relevant to their role.`, input, role, lang)
		
		return processor.Process(ctx, personalizedPrompt)
	})
	
	// Node 4: State Updater
	stateUpdater := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		startTime := time.Now()
		
		// อัพเดท session data ต่างๆ
		sag.state.SetSessionData("last_response", input)
		sag.state.SetSessionData("last_update", time.Now())
		
		// ตรวจสอบ quality ของ response
		if len(input) > 100 {
			sag.state.SetSessionData("response_quality", "detailed")
		} else {
			sag.state.SetSessionData("response_quality", "brief")
		}
		
		duration := time.Since(startTime)
		fmt.Printf("🔄 [StateUpdater] Updated session state (took %v)\n", duration)
		
		return input, nil
	})
	
	// เพิ่ม nodes
	sag.graph.AddLambdaNode("profile_analyzer", profileAnalyzer)
	sag.graph.AddLambdaNode("context_builder", contextBuilder)
	sag.graph.AddLambdaNode("response_generator", responseGenerator)
	sag.graph.AddLambdaNode("state_updater", stateUpdater)
	
	// เชื่อม edges
	sag.graph.AddEdge(compose.START, "profile_analyzer")
	sag.graph.AddEdge("profile_analyzer", "context_builder")
	sag.graph.AddEdge("context_builder", "response_generator")
	sag.graph.AddEdge("response_generator", "state_updater")
	sag.graph.AddEdge("state_updater", compose.END)
}

func (sag *StateAwareGraph) Execute(input string) (string, error) {
	runnable, err := sag.graph.Compile(sag.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to compile graph: %w", err)
	}
	
	return runnable.Invoke(sag.ctx, input)
}

func (sag *StateAwareGraph) GetState() *SharedState {
	return sag.state
}

func runAdvancedStateDemo() {
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

	fmt.Println("=== บทที่ 5: Advanced State Management ===")
	fmt.Println("ตัวอย่างการจัดการ State ขั้นสูงใน Eino Graph")
	fmt.Println()

	// === Demo 1: Basic State Management ===
	fmt.Println("📊 Demo 1: Basic State Management")
	
	state := NewSharedState("conversation_001")
	stateGraph := NewStateAwareGraph(state, model, ctx)
	stateGraph.BuildPersonalizedAssistantGraph()
	
	testQueries := []string{
		"สวัสดี ผมเป็น Go developer มือใหม่ อยากเรียนรู้ Eino",
		"ช่วยอธิบาย Graph pattern ให้ฟังหน่อย",
		"มี example การใช้งานจริงไหม",
	}
	
	for i, query := range testQueries {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", query)
		
		result, err := stateGraph.Execute(query)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result: %s\n", result)
		
		// แสดง state information
		metrics := state.GetMetrics()
		fmt.Printf("📈 Metrics: %d steps, %d success, avg time: %v\n", 
			metrics.TotalSteps, metrics.SuccessSteps, metrics.AverageTime)
	}
	
	// === Demo 2: State Persistence and Recovery ===
	fmt.Println("\n📁 Demo 2: State Inspection")
	
	// แสดง detailed state information
	fmt.Println("\n=== State Summary ===")
	fmt.Printf("Conversation ID: %s\n", state.conversationID)
	
	fmt.Println("\nUser Profile:")
	if lang, exists := state.GetUserProfile("language"); exists {
		fmt.Printf("  Language: %v\n", lang)
	}
	if role, exists := state.GetUserProfile("role"); exists {
		fmt.Printf("  Role: %v\n", role)
	}
	if level, exists := state.GetUserProfile("level"); exists {
		fmt.Printf("  Level: %v\n", level)
	}
	
	fmt.Println("\nSession Data:")
	if lastResponse, exists := state.GetSessionData("last_response"); exists {
		fmt.Printf("  Last Response: %v\n", lastResponse)
	}
	if quality, exists := state.GetSessionData("response_quality"); exists {
		fmt.Printf("  Response Quality: %v\n", quality)
	}
	
	fmt.Println("\nContext History:")
	history := state.GetContextHistory()
	for i, ctx := range history {
		fmt.Printf("  %d: %s\n", i+1, ctx)
	}
	
	fmt.Println("\nProcessing Steps:")
	finalMetrics := state.GetMetrics()
	fmt.Printf("  Total Steps: %d\n", finalMetrics.TotalSteps)
	fmt.Printf("  Success Rate: %.1f%%\n", 
		float64(finalMetrics.SuccessSteps)/float64(finalMetrics.TotalSteps)*100)
	fmt.Printf("  Total Duration: %v\n", finalMetrics.TotalDuration)
	fmt.Printf("  Average Step Time: %v\n", finalMetrics.AverageTime)
	
	// === Demo 3: Multi-Session State Management ===
	fmt.Println("\n👥 Demo 3: Multi-Session State Management")
	
	sessionManager := make(map[string]*SharedState)
	
	// สร้าง multiple sessions
	sessions := []string{"user_001", "user_002", "user_003"}
	for _, sessionID := range sessions {
		sessionManager[sessionID] = NewSharedState(sessionID)
		
		// จำลองการตั้งค่า profile ที่แตกต่างกัน
		switch sessionID {
		case "user_001":
			sessionManager[sessionID].SetUserProfile("role", "frontend_developer")
			sessionManager[sessionID].SetUserProfile("language", "thai")
		case "user_002":
			sessionManager[sessionID].SetUserProfile("role", "backend_engineer")
			sessionManager[sessionID].SetUserProfile("language", "english")
		case "user_003":
			sessionManager[sessionID].SetUserProfile("role", "devops")
			sessionManager[sessionID].SetUserProfile("language", "thai")
		}
	}
	
	fmt.Println("Created sessions with different profiles:")
	for sessionID, sessionState := range sessionManager {
		role, _ := sessionState.GetUserProfile("role")
		lang, _ := sessionState.GetUserProfile("language")
		fmt.Printf("  %s: %v (%v)\n", sessionID, role, lang)
	}
	
	fmt.Println("\n✅ Advanced State Management Demo Complete!")
	fmt.Println("🎯 Key Concepts Demonstrated:")
	fmt.Println("   - Thread-safe shared state")
	fmt.Println("   - Processing metrics tracking")
	fmt.Println("   - Context history management")
	fmt.Println("   - User profile personalization")
	fmt.Println("   - Session-aware processing")
	fmt.Println("   - Multi-session state isolation")
}

func main() {
	runAdvancedStateDemo()
}