package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
)

// Request types for routing
type RequestType string

const (
	RequestTypeCalculation   RequestType = "calculation"
	RequestTypeTranslation   RequestType = "translation" 
	RequestTypeDataAnalysis  RequestType = "data_analysis"
	RequestTypeTextProcessing RequestType = "text_processing"
	RequestTypeTimeOperation RequestType = "time_operation"
	RequestTypeUnknown       RequestType = "unknown"
)

// Routing decision structure
type RoutingDecision struct {
	RequestType   RequestType            `json:"request_type"`
	Confidence    float64                `json:"confidence"`
	Parameters    map[string]interface{} `json:"parameters"`
	Reasoning     string                 `json:"reasoning"`
	NextNode      string                 `json:"next_node"`
	Timestamp     time.Time              `json:"timestamp"`
}

// Processing result structure
type ProcessingResult struct {
	OriginalInput   string                 `json:"original_input"`
	Route          RoutingDecision        `json:"route"`
	ProcessedData  interface{}            `json:"processed_data"`
	ExecutionPath  []string               `json:"execution_path"`
	TotalDuration  time.Duration          `json:"total_duration"`
	Success        bool                   `json:"success"`
	Message        string                 `json:"message"`
}

// main function
func main() {
	runRouterBranchExample()
}

// Router/Branch Node Examples
func runRouterBranchExample() {
	fmt.Println("=== Router/Branch Node Examples ===")
	
	// === ตัวอย่าง 1: Simple Router ===
	fmt.Println("\n=== Simple Router ===")
	runSimpleRouter()
	
	// === ตัวอย่าง 2: Advanced Conditional Router ===
	fmt.Println("\n=== Advanced Conditional Router ===")
	runAdvancedRouter()
	
	// === ตัวอย่าง 3: Multi-Level Branch System ===
	fmt.Println("\n=== Multi-Level Branch System ===")
	runMultiLevelBranch()
}

// Intent Classification function
func classifyIntent(ctx context.Context, input string) (RoutingDecision, error) {
	fmt.Printf("🧭 Intent Router analyzing: %s\n", input)
	
	inputLower := strings.ToLower(input)
	var requestType RequestType
	var confidence float64
	var reasoning string
	var nextNode string
	
	// Simple intent classification logic
	if strings.Contains(inputLower, "calculate") || strings.Contains(inputLower, "math") || 
	   strings.Contains(inputLower, "add") || strings.Contains(inputLower, "multiply") {
		requestType = RequestTypeCalculation
		confidence = 0.9
		reasoning = "Contains mathematical operation keywords"
		nextNode = "math_processor"
	} else if strings.Contains(inputLower, "translate") || strings.Contains(inputLower, "แปล") {
		requestType = RequestTypeTranslation
		confidence = 0.85
		reasoning = "Contains translation keywords"
		nextNode = "translation_processor"
	} else if strings.Contains(inputLower, "analyze") || strings.Contains(inputLower, "stats") || 
			 strings.Contains(inputLower, "data") {
		requestType = RequestTypeDataAnalysis
		confidence = 0.8
		reasoning = "Contains data analysis keywords"
		nextNode = "data_processor"
	} else if strings.Contains(inputLower, "text") || strings.Contains(inputLower, "words") || 
			 strings.Contains(inputLower, "count") {
		requestType = RequestTypeTextProcessing
		confidence = 0.75
		reasoning = "Contains text processing keywords"
		nextNode = "text_processor"
	} else if strings.Contains(inputLower, "time") || strings.Contains(inputLower, "date") {
		requestType = RequestTypeTimeOperation
		confidence = 0.7
		reasoning = "Contains time-related keywords"
		nextNode = "time_processor"
	} else {
		requestType = RequestTypeUnknown
		confidence = 0.3
		reasoning = "No clear intent identified"
		nextNode = "default_processor"
	}
	
	decision := RoutingDecision{
		RequestType: requestType,
		Confidence:  confidence,
		Parameters:  map[string]interface{}{"original_input": input},
		Reasoning:   reasoning,
		NextNode:    nextNode,
		Timestamp:   time.Now(),
	}
	
	fmt.Printf("   → Routed to: %s (confidence: %.2f)\n", nextNode, confidence)
	fmt.Printf("   → Reasoning: %s\n", reasoning)
	
	return decision, nil
}

// Math processing function
func processMath(ctx context.Context, decision RoutingDecision) (ProcessingResult, error) {
	fmt.Printf("🔢 Math Processor handling request\n")
	
	input := decision.Parameters["original_input"].(string)
	
	// Simple math extraction and calculation
	result := map[string]interface{}{
		"operation": "detected_math",
		"message": "Math operation would be processed here",
	}
	
	// Extract numbers from input
	words := strings.Fields(input)
	var numbers []float64
	for _, word := range words {
		if num, err := strconv.ParseFloat(word, 64); err == nil {
			numbers = append(numbers, num)
		}
	}
	
	if len(numbers) >= 2 {
		sum := 0.0
		for _, num := range numbers {
			sum += num
		}
		result["numbers_found"] = numbers
		result["sum"] = sum
		result["operation"] = "addition"
	}
	
	return ProcessingResult{
		OriginalInput: input,
		Route:        decision,
		ProcessedData: result,
		ExecutionPath: []string{"intent_router", "math_processor"},
		Success:      true,
		Message:      "Math processing completed",
	}, nil
}

// Text processing function
func processText(ctx context.Context, decision RoutingDecision) (ProcessingResult, error) {
	fmt.Printf("📝 Text Processor handling request\n")
	
	input := decision.Parameters["original_input"].(string)
	
	words := strings.Fields(input)
	result := map[string]interface{}{
		"word_count":      len(words),
		"character_count": len(input),
		"words":          words,
		"uppercase":      strings.ToUpper(input),
	}
	
	return ProcessingResult{
		OriginalInput: input,
		Route:        decision,
		ProcessedData: result,
		ExecutionPath: []string{"intent_router", "text_processor"},
		Success:      true,
		Message:      "Text processing completed",
	}, nil
}

// Default processing function
func processDefault(ctx context.Context, decision RoutingDecision) (ProcessingResult, error) {
	fmt.Printf("🤔 Default Processor handling unknown request\n")
	
	input := decision.Parameters["original_input"].(string)
	
	result := map[string]interface{}{
		"message": "Request type not recognized",
		"suggestions": []string{
			"Try mathematical operations (e.g., 'calculate 5 + 3')",
			"Try text processing (e.g., 'count words in this text')",
			"Try data analysis (e.g., 'analyze these numbers')",
		},
	}
	
	return ProcessingResult{
		OriginalInput: input,
		Route:        decision,
		ProcessedData: result,
		ExecutionPath: []string{"intent_router", "default_processor"},
		Success:      false,
		Message:      "Unknown request type",
	}, nil
}

// Simple Router
func runSimpleRouter() {
	graph := compose.NewGraph[string, ProcessingResult]()
	
	// Intent Classifier Router
	intentRouter := compose.InvokableLambda(classifyIntent)
	
	// Math Processor Node
	mathProcessor := compose.InvokableLambda(processMath)
	
	// Text Processor Node
	textProcessor := compose.InvokableLambda(processText)
	
	// Default Processor Node
	defaultProcessor := compose.InvokableLambda(processDefault)
	
	// Add nodes
	graph.AddLambdaNode("intent_router", intentRouter)
	graph.AddLambdaNode("math_processor", mathProcessor)
	graph.AddLambdaNode("text_processor", textProcessor)
	graph.AddLambdaNode("default_processor", defaultProcessor)
	
	// Add edges
	graph.AddEdge(compose.START, "intent_router")
	graph.AddEdge("intent_router", "math_processor")
	graph.AddEdge("intent_router", "text_processor")
	graph.AddEdge("intent_router", "default_processor")
	graph.AddEdge("math_processor", compose.END)
	graph.AddEdge("text_processor", compose.END)
	graph.AddEdge("default_processor", compose.END)
	
	// Note: Real conditional routing would require more complex graph setup
	fmt.Printf("⚠️  Note: This demonstrates routing logic - actual conditional edges require different approach\n")
	
	// Simulate routing by calling processors directly based on decision
	testInputs := []string{
		"calculate 10 + 20 + 30",
		"count words in this sentence",
		"hello world how are you",
		"what is the weather today",
	}
	
	for i, input := range testInputs {
		fmt.Printf("\n--- Simple Router Test %d ---\n", i+1)
		
		// Get routing decision
		decision, _ := classifyIntent(context.Background(), input)
		
		// Route to appropriate processor
		var result ProcessingResult
		switch decision.NextNode {
		case "math_processor":
			result, _ = processMath(context.Background(), decision)
		case "text_processor":
			result, _ = processText(context.Background(), decision)
		default:
			result, _ = processDefault(context.Background(), decision)
		}
		
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Result:\n%s\n", resultJSON)
	}
}

// Advanced Conditional Router
func runAdvancedRouter() {
	graph := compose.NewGraph[map[string]interface{}, interface{}]()
	
	// Advanced Router with multiple conditions
	advancedRouter := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
		userInput := input["input"].(string)
		userLevel := input["user_level"].(string)
		context := input["context"].(map[string]interface{})
		
		fmt.Printf("🎯 Advanced Router analyzing complex request\n")
		fmt.Printf("   Input: %s\n", userInput)
		fmt.Printf("   User Level: %s\n", userLevel)
		fmt.Printf("   Context: %+v\n", context)
		
		// Multi-factor routing decision
		inputLower := strings.ToLower(userInput)
		var route string
		var processingParams map[string]interface{}
		
		// Factor 1: Content analysis
		isDataRequest := strings.Contains(inputLower, "data") || strings.Contains(inputLower, "analyze")
		isMathRequest := strings.Contains(inputLower, "calculate") || strings.Contains(inputLower, "math")
		isComplexRequest := len(strings.Fields(userInput)) > 10
		
		// Factor 2: User level
		isExpertUser := userLevel == "expert"
		isBeginnerUser := userLevel == "beginner"
		
		// Factor 3: Context
		hasProjectContext := context["project_type"] != nil
		isUrgent := context["priority"] == "high"
		
		// Routing logic
		if isMathRequest && isExpertUser {
			route = "advanced_math_processor"
			processingParams = map[string]interface{}{
				"mode": "expert",
				"include_steps": true,
				"precision": "high",
			}
		} else if isMathRequest && isBeginnerUser {
			route = "simple_math_processor"
			processingParams = map[string]interface{}{
				"mode": "beginner",
				"explain_steps": true,
				"use_examples": true,
			}
		} else if isDataRequest && hasProjectContext {
			route = "project_aware_analyzer"
			processingParams = map[string]interface{}{
				"project_type": context["project_type"],
				"context_aware": true,
			}
		} else if isComplexRequest {
			route = "multi_step_processor"
			processingParams = map[string]interface{}{
				"break_down": true,
				"step_by_step": true,
			}
		} else if isUrgent {
			route = "priority_processor"
			processingParams = map[string]interface{}{
				"fast_mode": true,
				"priority": "high",
			}
		} else {
			route = "general_processor"
			processingParams = map[string]interface{}{
				"mode": "standard",
			}
		}
		
		result := map[string]interface{}{
			"selected_route": route,
			"routing_factors": map[string]interface{}{
				"is_data_request":     isDataRequest,
				"is_math_request":     isMathRequest,
				"is_complex_request":  isComplexRequest,
				"is_expert_user":      isExpertUser,
				"is_beginner_user":    isBeginnerUser,
				"has_project_context": hasProjectContext,
				"is_urgent":           isUrgent,
			},
			"processing_params": processingParams,
			"original_input":    userInput,
			"timestamp":         time.Now(),
		}
		
		fmt.Printf("   → Selected Route: %s\n", route)
		fmt.Printf("   → Processing Params: %+v\n", processingParams)
		
		return result, nil
	})
	
	// Add node
	graph.AddLambdaNode("advanced_router", advancedRouter)
	graph.AddEdge(compose.START, "advanced_router")
	graph.AddEdge("advanced_router", compose.END)
	
	// Compile
	runnable, err := graph.Compile(context.Background())
	if err != nil {
		fmt.Printf("Error compiling advanced router: %v\n", err)
		return
	}
	
	// Test cases
	testCases := []map[string]interface{}{
		{
			"input":      "calculate the derivative of x^2 + 3x + 2",
			"user_level": "expert",
			"context":    map[string]interface{}{"project_type": "research", "priority": "normal"},
		},
		{
			"input":      "what is 2 + 2?",
			"user_level": "beginner",
			"context":    map[string]interface{}{"priority": "normal"},
		},
		{
			"input":      "analyze user engagement data for our mobile app and provide insights on retention patterns over the last quarter",
			"user_level": "intermediate",
			"context":    map[string]interface{}{"project_type": "mobile_analytics", "priority": "high"},
		},
		{
			"input":      "help me understand how to implement a complex machine learning pipeline with data preprocessing, feature engineering, model training, validation, and deployment phases",
			"user_level": "intermediate",
			"context":    map[string]interface{}{"project_type": "ml", "priority": "normal"},
		},
		{
			"input":      "urgent: fix production issue",
			"user_level": "expert",
			"context":    map[string]interface{}{"priority": "high", "environment": "production"},
		},
	}
	
	for i, testCase := range testCases {
		fmt.Printf("\n--- Advanced Router Test %d ---\n", i+1)
		
		result, err := runnable.Invoke(context.Background(), testCase)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Advanced Routing Result:\n%s\n", resultJSON)
	}
}

// Multi-Level Branch System
func runMultiLevelBranch() {
	graph := compose.NewGraph[map[string]interface{}, interface{}]()
	
	// Level 1: Domain Router
	domainRouter := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (string, error) {
		userInput := strings.ToLower(input["input"].(string))
		
		fmt.Printf("🌐 Level 1 - Domain Router\n")
		
		var domain string
		if strings.Contains(userInput, "math") || strings.Contains(userInput, "calculate") || 
		   strings.Contains(userInput, "number") {
			domain = "mathematics"
		} else if strings.Contains(userInput, "text") || strings.Contains(userInput, "word") || 
				 strings.Contains(userInput, "language") {
			domain = "language"
		} else if strings.Contains(userInput, "data") || strings.Contains(userInput, "analyze") || 
				 strings.Contains(userInput, "stats") {
			domain = "analytics"
		} else {
			domain = "general"
		}
		
		result := fmt.Sprintf("%s|%s", domain, input["input"].(string))
		fmt.Printf("   → Domain: %s\n", domain)
		return result, nil
	})
	
	// Level 2: Complexity Router
	complexityRouter := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		parts := strings.SplitN(input, "|", 2)
		domain := parts[0]
		userInput := parts[1]
		
		fmt.Printf("🎚️  Level 2 - Complexity Router (Domain: %s)\n", domain)
		
		inputLower := strings.ToLower(userInput)
		var complexity string
		
		// Complexity indicators
		wordCount := len(strings.Fields(userInput))
		hasMultipleOperations := strings.Count(inputLower, "and") > 0 || strings.Count(inputLower, "then") > 0
		hasAdvancedTerms := strings.Contains(inputLower, "complex") || 
						   strings.Contains(inputLower, "advanced") || 
						   strings.Contains(inputLower, "detail")
		
		if wordCount > 15 || hasMultipleOperations || hasAdvancedTerms {
			complexity = "complex"
		} else if wordCount > 8 {
			complexity = "moderate"
		} else {
			complexity = "simple"
		}
		
		result := fmt.Sprintf("%s|%s|%s", domain, complexity, userInput)
		fmt.Printf("   → Complexity: %s\n", complexity)
		return result, nil
	})
	
	// Level 3: Final Processor Router
	finalProcessor := compose.InvokableLambda(func(ctx context.Context, input string) (interface{}, error) {
		parts := strings.SplitN(input, "|", 3)
		domain := parts[0]
		complexity := parts[1]
		userInput := parts[2]
		
		fmt.Printf("⚙️  Level 3 - Final Processor (Domain: %s, Complexity: %s)\n", domain, complexity)
		
		// Determine final processing strategy
		var strategy string
		var estimatedTime time.Duration
		var resources []string
		
		switch {
		case domain == "mathematics" && complexity == "complex":
			strategy = "advanced_math_engine"
			estimatedTime = 30 * time.Second
			resources = []string{"symbolic_math", "numerical_solver", "graph_plotter"}
			
		case domain == "mathematics" && complexity == "simple":
			strategy = "basic_calculator"
			estimatedTime = 2 * time.Second
			resources = []string{"arithmetic_engine"}
			
		case domain == "language" && complexity == "complex":
			strategy = "nlp_pipeline"
			estimatedTime = 45 * time.Second
			resources = []string{"tokenizer", "parser", "semantic_analyzer", "generator"}
			
		case domain == "analytics" && complexity == "complex":
			strategy = "advanced_analytics"
			estimatedTime = 60 * time.Second
			resources = []string{"data_processor", "statistical_engine", "ml_models", "visualizer"}
			
		default:
			strategy = "general_purpose"
			estimatedTime = 10 * time.Second
			resources = []string{"basic_processor"}
		}
		
		result := map[string]interface{}{
			"processing_strategy": strategy,
			"estimated_time":     estimatedTime.String(),
			"required_resources": resources,
			"routing_path": []string{
				fmt.Sprintf("domain_router→%s", domain),
				fmt.Sprintf("complexity_router→%s", complexity),
				fmt.Sprintf("final_processor→%s", strategy),
			},
			"original_input": userInput,
			"processing_metadata": map[string]interface{}{
				"domain":     domain,
				"complexity": complexity,
				"word_count": len(strings.Fields(userInput)),
			},
		}
		
		fmt.Printf("   → Strategy: %s\n", strategy)
		fmt.Printf("   → Est. Time: %s\n", estimatedTime)
		fmt.Printf("   → Resources: %v\n", resources)
		
		return result, nil
	})
	
	// Add nodes
	graph.AddLambdaNode("domain_router", domainRouter)
	graph.AddLambdaNode("complexity_router", complexityRouter)
	graph.AddLambdaNode("final_processor", finalProcessor)
	
	// Add edges
	graph.AddEdge(compose.START, "domain_router")
	graph.AddEdge("domain_router", "complexity_router")
	graph.AddEdge("complexity_router", "final_processor")
	graph.AddEdge("final_processor", compose.END)
	
	// Compile
	runnable, err := graph.Compile(context.Background())
	if err != nil {
		fmt.Printf("Error compiling multi-level branch: %v\n", err)
		return
	}
	
	// Test cases
	testCases := []map[string]interface{}{
		{
			"input": "calculate 2 + 2",
		},
		{
			"input": "solve the complex differential equation dy/dx = x^2 + sin(x) and provide step-by-step solution",
		},
		{
			"input": "count words",
		},
		{
			"input": "analyze the semantic structure of this text and identify themes, sentiment, and key entities",
		},
		{
			"input": "find average",
		},
		{
			"input": "perform comprehensive statistical analysis on customer behavior data including correlation analysis, clustering, and predictive modeling",
		},
	}
	
	for i, testCase := range testCases {
		fmt.Printf("\n--- Multi-Level Branch Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", testCase["input"])
		
		result, err := runnable.Invoke(context.Background(), testCase)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Multi-Level Branch Result:\n%s\n", resultJSON)
		fmt.Println(strings.Repeat("-", 80))
	}
}