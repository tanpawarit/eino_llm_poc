package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
)

// Enhanced Tool interface with metadata
type EnhancedTool interface {
	Name() string
	Description() string
	Category() string
	Execute(ctx context.Context, params map[string]interface{}) (ToolResult, error)
	GetSchema() ToolSchema
}

// Tool result with metadata
type ToolResult struct {
	Success   bool                   `json:"success"`
	Data      interface{}            `json:"data"`
	Message   string                 `json:"message"`
	Metadata  map[string]interface{} `json:"metadata"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// Tool schema for validation
type ToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Required    []string               `json:"required"`
	Returns     string                 `json:"returns"`
}

// === MATH TOOLS ===

// CalculatorTool - เครื่องคิดเลขขั้นสูง
type CalculatorTool struct{}

func (c *CalculatorTool) Name() string { return "calculator" }
func (c *CalculatorTool) Description() string { return "Advanced mathematical calculator" }
func (c *CalculatorTool) Category() string { return "math" }

func (c *CalculatorTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "calculator",
		Description: "Perform mathematical calculations including basic arithmetic, trigonometry, and advanced functions",
		Parameters: map[string]interface{}{
			"operation": "string (add, subtract, multiply, divide, power, sqrt, sin, cos, tan, log)",
			"values":    "array of numbers",
			"angle_unit": "string (degrees, radians) - for trigonometric functions",
		},
		Required: []string{"operation", "values"},
		Returns:  "number",
	}
}

func (c *CalculatorTool) Execute(ctx context.Context, params map[string]interface{}) (ToolResult, error) {
	start := time.Now()
	
	operation, ok := params["operation"].(string)
	if !ok {
		return ToolResult{
			Success: false,
			Message: "operation parameter is required",
			Timestamp: time.Now(),
		}, nil
	}

	valuesInterface, ok := params["values"].([]interface{})
	if !ok {
		return ToolResult{
			Success: false,
			Message: "values parameter must be an array",
			Timestamp: time.Now(),
		}, nil
	}

	// Convert interface{} to float64
	var values []float64
	for _, v := range valuesInterface {
		switch val := v.(type) {
		case float64:
			values = append(values, val)
		case int:
			values = append(values, float64(val))
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				values = append(values, f)
			} else {
				return ToolResult{
					Success: false,
					Message: fmt.Sprintf("invalid number: %s", val),
					Timestamp: time.Now(),
				}, nil
			}
		}
	}

	if len(values) == 0 {
		return ToolResult{
			Success: false,
			Message: "at least one value is required",
			Timestamp: time.Now(),
		}, nil
	}

	var result float64
	var metadata = make(map[string]interface{})

	switch operation {
	case "add":
		for _, v := range values {
			result += v
		}
	case "subtract":
		if len(values) < 2 {
			return ToolResult{Success: false, Message: "subtract requires at least 2 values", Timestamp: time.Now()}, nil
		}
		result = values[0]
		for i := 1; i < len(values); i++ {
			result -= values[i]
		}
	case "multiply":
		result = 1
		for _, v := range values {
			result *= v
		}
	case "divide":
		if len(values) < 2 {
			return ToolResult{Success: false, Message: "divide requires at least 2 values", Timestamp: time.Now()}, nil
		}
		result = values[0]
		for i := 1; i < len(values); i++ {
			if values[i] == 0 {
				return ToolResult{Success: false, Message: "division by zero", Timestamp: time.Now()}, nil
			}
			result /= values[i]
		}
	case "power":
		if len(values) != 2 {
			return ToolResult{Success: false, Message: "power requires exactly 2 values", Timestamp: time.Now()}, nil
		}
		result = math.Pow(values[0], values[1])
	case "sqrt":
		if len(values) != 1 {
			return ToolResult{Success: false, Message: "sqrt requires exactly 1 value", Timestamp: time.Now()}, nil
		}
		if values[0] < 0 {
			return ToolResult{Success: false, Message: "sqrt of negative number", Timestamp: time.Now()}, nil
		}
		result = math.Sqrt(values[0])
	case "sin", "cos", "tan":
		if len(values) != 1 {
			return ToolResult{Success: false, Message: fmt.Sprintf("%s requires exactly 1 value", operation), Timestamp: time.Now()}, nil
		}
		
		angle := values[0]
		angleUnit, _ := params["angle_unit"].(string)
		if angleUnit == "degrees" {
			angle = angle * math.Pi / 180
			metadata["converted_to_radians"] = angle
		}
		
		switch operation {
		case "sin":
			result = math.Sin(angle)
		case "cos":
			result = math.Cos(angle)
		case "tan":
			result = math.Tan(angle)
		}
	case "log":
		if len(values) != 1 {
			return ToolResult{Success: false, Message: "log requires exactly 1 value", Timestamp: time.Now()}, nil
		}
		if values[0] <= 0 {
			return ToolResult{Success: false, Message: "log of non-positive number", Timestamp: time.Now()}, nil
		}
		result = math.Log(values[0])
	default:
		return ToolResult{
			Success: false,
			Message: fmt.Sprintf("unknown operation: %s", operation),
			Timestamp: time.Now(),
		}, nil
	}

	metadata["operation"] = operation
	metadata["input_values"] = values
	
	return ToolResult{
		Success:   true,
		Data:      result,
		Message:   fmt.Sprintf("Calculated %s of %v = %f", operation, values, result),
		Metadata:  metadata,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, nil
}

// === STRING TOOLS ===

// TextProcessorTool - เครื่องมือประมวลผลข้อความ
type TextProcessorTool struct{}

func (t *TextProcessorTool) Name() string { return "text_processor" }
func (t *TextProcessorTool) Description() string { return "Advanced text processing operations" }
func (t *TextProcessorTool) Category() string { return "text" }

func (t *TextProcessorTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "text_processor",
		Description: "Process text with various operations like count, transform, analyze",
		Parameters: map[string]interface{}{
			"operation": "string (count_words, count_chars, uppercase, lowercase, reverse, remove_spaces, extract_numbers)",
			"text":      "string - input text to process",
			"options":   "object - additional options for specific operations",
		},
		Required: []string{"operation", "text"},
		Returns:  "object with processed result",
	}
}

func (t *TextProcessorTool) Execute(ctx context.Context, params map[string]interface{}) (ToolResult, error) {
	start := time.Now()
	
	operation, ok := params["operation"].(string)
	if !ok {
		return ToolResult{Success: false, Message: "operation parameter is required", Timestamp: time.Now()}, nil
	}

	text, ok := params["text"].(string)
	if !ok {
		return ToolResult{Success: false, Message: "text parameter is required", Timestamp: time.Now()}, nil
	}

	var result interface{}
	var message string
	metadata := map[string]interface{}{
		"operation":    operation,
		"input_length": len(text),
	}

	switch operation {
	case "count_words":
		words := strings.Fields(text)
		result = len(words)
		message = fmt.Sprintf("Text contains %d words", len(words))
		metadata["words"] = words
		
	case "count_chars":
		result = len(text)
		message = fmt.Sprintf("Text contains %d characters", len(text))
		metadata["with_spaces"] = len(text)
		metadata["without_spaces"] = len(strings.ReplaceAll(text, " ", ""))
		
	case "uppercase":
		result = strings.ToUpper(text)
		message = "Converted text to uppercase"
		
	case "lowercase":
		result = strings.ToLower(text)
		message = "Converted text to lowercase"
		
	case "reverse":
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		result = string(runes)
		message = "Reversed text"
		
	case "remove_spaces":
		result = strings.ReplaceAll(text, " ", "")
		message = "Removed all spaces from text"
		
	case "extract_numbers":
		var numbers []string
		words := strings.Fields(text)
		for _, word := range words {
			if _, err := strconv.ParseFloat(word, 64); err == nil {
				numbers = append(numbers, word)
			}
		}
		result = numbers
		message = fmt.Sprintf("Extracted %d numbers from text", len(numbers))
		metadata["numbers"] = numbers
		
	default:
		return ToolResult{
			Success: false,
			Message: fmt.Sprintf("unknown operation: %s", operation),
			Timestamp: time.Now(),
		}, nil
	}

	return ToolResult{
		Success:   true,
		Data:      result,
		Message:   message,
		Metadata:  metadata,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, nil
}

// === TIME TOOLS ===

// TimeUtilsTool - เครื่องมือเวลา
type TimeUtilsTool struct{}

func (t *TimeUtilsTool) Name() string { return "time_utils" }
func (t *TimeUtilsTool) Description() string { return "Time and date utilities" }
func (t *TimeUtilsTool) Category() string { return "time" }

func (t *TimeUtilsTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "time_utils",
		Description: "Utilities for time and date operations",
		Parameters: map[string]interface{}{
			"operation": "string (current_time, format_time, add_duration, time_diff, timezone_convert)",
			"datetime":  "string - input datetime (RFC3339 format)",
			"format":    "string - output format (Go time format)",
			"duration":  "string - duration to add (e.g., '1h30m')",
			"timezone":  "string - target timezone",
		},
		Required: []string{"operation"},
		Returns:  "object with time result",
	}
}

func (t *TimeUtilsTool) Execute(ctx context.Context, params map[string]interface{}) (ToolResult, error) {
	start := time.Now()
	
	operation, ok := params["operation"].(string)
	if !ok {
		return ToolResult{Success: false, Message: "operation parameter is required", Timestamp: time.Now()}, nil
	}

	var result interface{}
	var message string
	metadata := map[string]interface{}{
		"operation": operation,
	}

	switch operation {
	case "current_time":
		now := time.Now()
		result = map[string]interface{}{
			"rfc3339":   now.Format(time.RFC3339),
			"unix":      now.Unix(),
			"formatted": now.Format("2006-01-02 15:04:05"),
			"timezone":  now.Location().String(),
		}
		message = "Retrieved current time"
		
	case "format_time":
		datetimeStr, ok := params["datetime"].(string)
		if !ok {
			return ToolResult{Success: false, Message: "datetime parameter is required", Timestamp: time.Now()}, nil
		}
		
		dt, err := time.Parse(time.RFC3339, datetimeStr)
		if err != nil {
			return ToolResult{Success: false, Message: fmt.Sprintf("invalid datetime format: %v", err), Timestamp: time.Now()}, nil
		}
		
		format, ok := params["format"].(string)
		if !ok {
			format = "2006-01-02 15:04:05"
		}
		
		result = dt.Format(format)
		message = fmt.Sprintf("Formatted datetime using format: %s", format)
		metadata["input_datetime"] = datetimeStr
		metadata["format_used"] = format
		
	case "add_duration":
		datetimeStr, ok := params["datetime"].(string)
		if !ok {
			return ToolResult{Success: false, Message: "datetime parameter is required", Timestamp: time.Now()}, nil
		}
		
		durationStr, ok := params["duration"].(string)
		if !ok {
			return ToolResult{Success: false, Message: "duration parameter is required", Timestamp: time.Now()}, nil
		}
		
		dt, err := time.Parse(time.RFC3339, datetimeStr)
		if err != nil {
			return ToolResult{Success: false, Message: fmt.Sprintf("invalid datetime format: %v", err), Timestamp: time.Now()}, nil
		}
		
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return ToolResult{Success: false, Message: fmt.Sprintf("invalid duration format: %v", err), Timestamp: time.Now()}, nil
		}
		
		newTime := dt.Add(duration)
		result = map[string]interface{}{
			"original": dt.Format(time.RFC3339),
			"new_time": newTime.Format(time.RFC3339),
			"duration_added": duration.String(),
		}
		message = fmt.Sprintf("Added %s to datetime", duration)
		
	default:
		return ToolResult{
			Success: false,
			Message: fmt.Sprintf("unknown operation: %s", operation),
			Timestamp: time.Now(),
		}, nil
	}

	return ToolResult{
		Success:   true,
		Data:      result,
		Message:   message,
		Metadata:  metadata,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, nil
}

// === DATA TOOLS ===

// DataAnalyzerTool - เครื่องมือวิเคราะห์ข้อมูล
type DataAnalyzerTool struct{}

func (d *DataAnalyzerTool) Name() string { return "data_analyzer" }
func (d *DataAnalyzerTool) Description() string { return "Data analysis and statistics" }
func (d *DataAnalyzerTool) Category() string { return "data" }

func (d *DataAnalyzerTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "data_analyzer",
		Description: "Analyze arrays of numbers for statistical insights",
		Parameters: map[string]interface{}{
			"operation": "string (stats, sort, filter, group)",
			"data":      "array of numbers",
			"criteria":  "object - criteria for filtering or grouping",
		},
		Required: []string{"operation", "data"},
		Returns:  "object with analysis results",
	}
}

func (d *DataAnalyzerTool) Execute(ctx context.Context, params map[string]interface{}) (ToolResult, error) {
	start := time.Now()
	
	operation, ok := params["operation"].(string)
	if !ok {
		return ToolResult{Success: false, Message: "operation parameter is required", Timestamp: time.Now()}, nil
	}

	dataInterface, ok := params["data"].([]interface{})
	if !ok {
		return ToolResult{Success: false, Message: "data parameter must be an array", Timestamp: time.Now()}, nil
	}

	// Convert to float64 array
	var data []float64
	for _, v := range dataInterface {
		switch val := v.(type) {
		case float64:
			data = append(data, val)
		case int:
			data = append(data, float64(val))
		}
	}

	if len(data) == 0 {
		return ToolResult{Success: false, Message: "data array is empty", Timestamp: time.Now()}, nil
	}

	var result interface{}
	var message string
	metadata := map[string]interface{}{
		"operation": operation,
		"data_size": len(data),
	}

	switch operation {
	case "stats":
		// Calculate statistics
		sum := 0.0
		min := data[0]
		max := data[0]
		
		for _, v := range data {
			sum += v
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		
		mean := sum / float64(len(data))
		
		// Calculate standard deviation
		variance := 0.0
		for _, v := range data {
			variance += math.Pow(v-mean, 2)
		}
		variance /= float64(len(data))
		stdDev := math.Sqrt(variance)
		
		result = map[string]interface{}{
			"count":    len(data),
			"sum":      sum,
			"mean":     mean,
			"min":      min,
			"max":      max,
			"variance": variance,
			"std_dev":  stdDev,
			"range":    max - min,
		}
		message = fmt.Sprintf("Calculated statistics for %d data points", len(data))
		
	case "sort":
		sortedData := make([]float64, len(data))
		copy(sortedData, data)
		
		// Simple bubble sort for demonstration
		for i := 0; i < len(sortedData)-1; i++ {
			for j := 0; j < len(sortedData)-i-1; j++ {
				if sortedData[j] > sortedData[j+1] {
					sortedData[j], sortedData[j+1] = sortedData[j+1], sortedData[j]
				}
			}
		}
		
		result = map[string]interface{}{
			"original": data,
			"sorted":   sortedData,
		}
		message = fmt.Sprintf("Sorted %d data points", len(data))
		
	default:
		return ToolResult{
			Success: false,
			Message: fmt.Sprintf("unknown operation: %s", operation),
			Timestamp: time.Now(),
		}, nil
	}

	return ToolResult{
		Success:   true,
		Data:      result,
		Message:   message,
		Metadata:  metadata,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, nil
}

// Enhanced Tool Manager
type EnhancedToolManager struct {
	tools      map[string]EnhancedTool
	categories map[string][]string
	history    []ToolExecution
}

type ToolExecution struct {
	ToolName   string        `json:"tool_name"`
	Parameters interface{}   `json:"parameters"`
	Result     ToolResult    `json:"result"`
	Timestamp  time.Time     `json:"timestamp"`
	Duration   time.Duration `json:"duration"`
}

func NewEnhancedToolManager() *EnhancedToolManager {
	tm := &EnhancedToolManager{
		tools:      make(map[string]EnhancedTool),
		categories: make(map[string][]string),
		history:    make([]ToolExecution, 0),
	}
	
	// Register tools
	tm.RegisterTool(&CalculatorTool{})
	tm.RegisterTool(&TextProcessorTool{})
	tm.RegisterTool(&TimeUtilsTool{})
	tm.RegisterTool(&DataAnalyzerTool{})
	
	return tm
}

func (tm *EnhancedToolManager) RegisterTool(tool EnhancedTool) {
	tm.tools[tool.Name()] = tool
	
	category := tool.Category()
	if _, exists := tm.categories[category]; !exists {
		tm.categories[category] = make([]string, 0)
	}
	tm.categories[category] = append(tm.categories[category], tool.Name())
}

func (tm *EnhancedToolManager) ExecuteTool(toolName string, params map[string]interface{}) (ToolResult, error) {
	tool, exists := tm.tools[toolName]
	if !exists {
		return ToolResult{
			Success: false,
			Message: fmt.Sprintf("tool not found: %s", toolName),
			Timestamp: time.Now(),
		}, nil
	}
	
	start := time.Now()
	result, err := tool.Execute(context.Background(), params)
	duration := time.Since(start)
	
	// Record execution
	execution := ToolExecution{
		ToolName:   toolName,
		Parameters: params,
		Result:     result,
		Timestamp:  time.Now(),
		Duration:   duration,
	}
	tm.history = append(tm.history, execution)
	
	return result, err
}

func (tm *EnhancedToolManager) GetToolsByCategory() map[string][]string {
	return tm.categories
}

func (tm *EnhancedToolManager) GetToolSchema(toolName string) (ToolSchema, error) {
	tool, exists := tm.tools[toolName]
	if !exists {
		return ToolSchema{}, fmt.Errorf("tool not found: %s", toolName)
	}
	
	return tool.GetSchema(), nil
}

func (tm *EnhancedToolManager) GetExecutionHistory() []ToolExecution {
	return tm.history
}

// main function
func main() {
	runEnhancedToolExample()
}

// Enhanced Tool Node Examples
func runEnhancedToolExample() {
	fmt.Println("=== Enhanced Tool Node Examples ===")
	
	// === ตัวอย่าง 1: Basic Tool Execution ===
	fmt.Println("\n=== Basic Tool Execution ===")
	runBasicToolExecution()
	
	// === ตัวอย่าง 2: Tool Pipeline ===
	fmt.Println("\n=== Tool Pipeline ===")
	runToolPipeline()
	
	// === ตัวอย่าง 3: Conditional Tool Selection ===
	fmt.Println("\n=== Conditional Tool Selection ===")
	runConditionalToolSelection()
}

// Basic Tool Execution
func runBasicToolExecution() {
	graph := compose.NewGraph[map[string]interface{}, ToolResult]()
	toolManager := NewEnhancedToolManager()
	
	// Tool Execution Node
	toolNode := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (ToolResult, error) {
		toolName := input["tool"].(string)
		params := input["params"].(map[string]interface{})
		
		fmt.Printf("🔧 Executing tool: %s\n", toolName)
		fmt.Printf("   Parameters: %+v\n", params)
		
		result, err := toolManager.ExecuteTool(toolName, params)
		if err != nil {
			return ToolResult{
				Success: false,
				Message: fmt.Sprintf("Tool execution error: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		
		fmt.Printf("   Result: %s\n", result.Message)
		return result, nil
	})
	
	// Add node
	graph.AddLambdaNode("tool_executor", toolNode)
	graph.AddEdge(compose.START, "tool_executor")
	graph.AddEdge("tool_executor", compose.END)
	
	// Compile
	runnable, err := graph.Compile(context.Background())
	if err != nil {
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}
	
	// Test cases
	testCases := []map[string]interface{}{
		{
			"tool": "calculator",
			"params": map[string]interface{}{
				"operation": "add",
				"values":    []interface{}{10, 20, 30},
			},
		},
		{
			"tool": "text_processor", 
			"params": map[string]interface{}{
				"operation": "count_words",
				"text":      "Hello world this is a test message",
			},
		},
		{
			"tool": "time_utils",
			"params": map[string]interface{}{
				"operation": "current_time",
			},
		},
		{
			"tool": "data_analyzer",
			"params": map[string]interface{}{
				"operation": "stats",
				"data":      []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
		},
	}
	
	for i, testCase := range testCases {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		result, err := runnable.Invoke(context.Background(), testCase)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Tool Result:\n%s\n", resultJSON)
	}
}

// Tool Pipeline
func runToolPipeline() {
	graph := compose.NewGraph[string, map[string]interface{}]()
	toolManager := NewEnhancedToolManager()
	
	// Text Analysis Pipeline
	textAnalyzer := compose.InvokableLambda(func(ctx context.Context, text string) (map[string]interface{}, error) {
		fmt.Printf("📝 Text Analysis Pipeline for: %s\n", text)
		
		// Step 1: Count words
		wordResult, _ := toolManager.ExecuteTool("text_processor", map[string]interface{}{
			"operation": "count_words",
			"text":      text,
		})
		
		// Step 2: Count characters
		charResult, _ := toolManager.ExecuteTool("text_processor", map[string]interface{}{
			"operation": "count_chars", 
			"text":      text,
		})
		
		// Step 3: Extract numbers
		numberResult, _ := toolManager.ExecuteTool("text_processor", map[string]interface{}{
			"operation": "extract_numbers",
			"text":      text,
		})
		
		// If we found numbers, analyze them
		var statsResult *ToolResult
		if numberResult.Success {
			if numbers, ok := numberResult.Data.([]string); ok && len(numbers) > 0 {
				// Convert strings to numbers
				var numData []interface{}
				for _, numStr := range numbers {
					if num, err := strconv.ParseFloat(numStr, 64); err == nil {
						numData = append(numData, num)
					}
				}
				
				if len(numData) > 0 {
					result, _ := toolManager.ExecuteTool("data_analyzer", map[string]interface{}{
						"operation": "stats",
						"data":      numData,
					})
					statsResult = &result
				}
			}
		}
		
		pipeline := map[string]interface{}{
			"original_text": text,
			"word_count":    wordResult.Data,
			"char_count":    charResult.Data,
			"numbers_found": numberResult.Data,
		}
		
		if statsResult != nil {
			pipeline["number_stats"] = statsResult.Data
		}
		
		return pipeline, nil
	})
	
	// Add node
	graph.AddLambdaNode("text_analyzer", textAnalyzer)
	graph.AddEdge(compose.START, "text_analyzer")
	graph.AddEdge("text_analyzer", compose.END)
	
	// Compile
	runnable, err := graph.Compile(context.Background())
	if err != nil {
		fmt.Printf("Error compiling pipeline: %v\n", err)
		return
	}
	
	// Test pipeline
	testTexts := []string{
		"Hello world! This text has 123 and 456 numbers in it.",
		"Go programming is awesome. Version 1.21 brings many new features!",
		"Data: 10 20 30 40 50. Average should be calculated.",
	}
	
	for i, text := range testTexts {
		fmt.Printf("\n--- Pipeline Test %d ---\n", i+1)
		result, err := runnable.Invoke(context.Background(), text)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Pipeline Result:\n%s\n", resultJSON)
	}
}

// Conditional Tool Selection
func runConditionalToolSelection() {
	graph := compose.NewGraph[map[string]interface{}, interface{}]()
	toolManager := NewEnhancedToolManager()
	
	// Smart Tool Selector
	smartSelector := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
		userIntent := strings.ToLower(input["intent"].(string))
		data := input["data"]
		
		fmt.Printf("🤖 Smart Tool Selector analyzing intent: %s\n", userIntent)
		
		var result ToolResult
		var err error
		
		if strings.Contains(userIntent, "calculate") || strings.Contains(userIntent, "math") {
			// Use calculator
			if params, ok := data.(map[string]interface{}); ok {
				result, err = toolManager.ExecuteTool("calculator", params)
			}
		} else if strings.Contains(userIntent, "text") || strings.Contains(userIntent, "words") {
			// Use text processor
			if params, ok := data.(map[string]interface{}); ok {
				result, err = toolManager.ExecuteTool("text_processor", params)
			}
		} else if strings.Contains(userIntent, "time") || strings.Contains(userIntent, "date") {
			// Use time utils
			if params, ok := data.(map[string]interface{}); ok {
				result, err = toolManager.ExecuteTool("time_utils", params)
			}
		} else if strings.Contains(userIntent, "analyze") || strings.Contains(userIntent, "stats") {
			// Use data analyzer
			if params, ok := data.(map[string]interface{}); ok {
				result, err = toolManager.ExecuteTool("data_analyzer", params)
			}
		} else {
			return map[string]interface{}{
				"error": "Unable to determine appropriate tool for intent",
				"available_categories": toolManager.GetToolsByCategory(),
			}, nil
		}
		
		if err != nil {
			return map[string]interface{}{
				"error": err.Error(),
			}, nil
		}
		
		return map[string]interface{}{
			"tool_result": result,
			"selected_for": userIntent,
		}, nil
	})
	
	// Add node
	graph.AddLambdaNode("smart_selector", smartSelector)
	graph.AddEdge(compose.START, "smart_selector")
	graph.AddEdge("smart_selector", compose.END)
	
	// Compile
	runnable, err := graph.Compile(context.Background())
	if err != nil {
		fmt.Printf("Error compiling selector: %v\n", err)
		return
	}
	
	// Test conditional selection
	testCases := []map[string]interface{}{
		{
			"intent": "I want to calculate the sum of these numbers",
			"data": map[string]interface{}{
				"operation": "add",
				"values":    []interface{}{15, 25, 35},
			},
		},
		{
			"intent": "Analyze this text for me",
			"data": map[string]interface{}{
				"operation": "count_words",
				"text":      "Artificial Intelligence is transforming the world",
			},
		},
		{
			"intent": "What time is it now?",
			"data": map[string]interface{}{
				"operation": "current_time",
			},
		},
		{
			"intent": "Give me stats for this dataset",
			"data": map[string]interface{}{
				"operation": "stats",
				"data":      []interface{}{100, 200, 150, 300, 250, 180},
			},
		},
		{
			"intent": "I don't know what I want",
			"data": map[string]interface{}{},
		},
	}
	
	for i, testCase := range testCases {
		fmt.Printf("\n--- Smart Selection Test %d ---\n", i+1)
		fmt.Printf("User Intent: %s\n", testCase["intent"])
		
		result, err := runnable.Invoke(context.Background(), testCase)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Smart Selection Result:\n%s\n", resultJSON)
		fmt.Println(strings.Repeat("-", 60))
	}
}