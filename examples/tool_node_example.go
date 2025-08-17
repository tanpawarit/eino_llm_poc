package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// Tool represents a function that can be called by the agent
type Tool struct {
	Name        string                                      `json:"name"`
	Description string                                      `json:"description"`
	Function    func(ctx context.Context, args string) (string, error) `json:"-"`
}

// ToolCall represents a tool call request from LLM
type ToolCall struct {
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolName string `json:"tool_name"`
	Success  bool   `json:"success"`
	Result   string `json:"result"`
	Error    string `json:"error,omitempty"`
}

// main function for running tool node examples
func main() {
	if err := runToolNodeExample(); err != nil {
		log.Fatalf("Error running tool node example: %v", err)
	}
}

// ตัวอย่าง Tool Node - สำหรับ Agent ที่ใช้เครื่องมือ
func runToolNodeExample() error {
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

	// === ตัวอย่าง 1: Basic Tool Node ===
	fmt.Println("=== Basic Tool Node ===")
	if err := runBasicToolNode(ctx, model); err != nil {
		return fmt.Errorf("basic tool node example failed: %w", err)
	}

	// === ตัวอย่าง 2: Calculator Agent ===
	fmt.Println("\n=== Calculator Agent ===")
	if err := runCalculatorAgent(ctx, model); err != nil {
		return fmt.Errorf("calculator agent example failed: %w", err)
	}

	// === ตัวอย่าง 3: Multi-Tool Agent ===
	fmt.Println("\n=== Multi-Tool Agent ===")
	if err := runMultiToolAgent(ctx, model); err != nil {
		return fmt.Errorf("multi-tool agent example failed: %w", err)
	}

	return nil
}

// Basic Tool Node
func runBasicToolNode(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// สร้าง Tools
	tools := map[string]Tool{
		"get_current_time": {
			Name:        "get_current_time",
			Description: "Get the current time in a specific timezone",
			Function: func(ctx context.Context, args string) (string, error) {
				// Parse timezone from args (simple JSON)
				var params map[string]string
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				timezone := params["timezone"]
				if timezone == "" {
					timezone = "Asia/Bangkok"
				}

				// Load timezone
				loc, err := time.LoadLocation(timezone)
				if err != nil {
					return "", fmt.Errorf("invalid timezone: %w", err)
				}

				currentTime := time.Now().In(loc)
				return fmt.Sprintf("Current time in %s: %s", timezone, currentTime.Format("2006-01-02 15:04:05")), nil
			},
		},
		"weather_info": {
			Name:        "weather_info",
			Description: "Get mock weather information for a city",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]string
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				city := params["city"]
				if city == "" {
					return "", errors.New("city parameter is required")
				}

				// Mock weather data
				mockWeather := map[string]string{
					"bangkok":   "Sunny, 32°C, Humidity: 75%",
					"chiang mai": "Partly cloudy, 28°C, Humidity: 60%",
					"phuket":    "Rainy, 30°C, Humidity: 85%",
					"london":    "Cloudy, 15°C, Humidity: 80%",
					"tokyo":     "Clear, 25°C, Humidity: 55%",
				}

				weather, exists := mockWeather[strings.ToLower(city)]
				if !exists {
					weather = "Weather data not available for this city"
				}

				return fmt.Sprintf("Weather in %s: %s", city, weather), nil
			},
		},
	}

	// Tool Dispatcher - ตัดสินใจและเรียกใช้ tool
	toolDispatcher := compose.InvokableLambda(func(ctx context.Context, userMessage string) ([]*schema.Message, error) {
		// สร้าง system prompt ที่บรรยาย tools
		var toolDescriptions []string
		for _, tool := range tools {
			toolDescriptions = append(toolDescriptions, fmt.Sprintf("- %s: %s", tool.Name, tool.Description))
		}

		systemPrompt := fmt.Sprintf(`คุณเป็น AI Assistant ที่สามารถใช้เครื่องมือต่างๆ ได้

Available Tools:
%s

ถ้าผู้ใช้ต้องการใช้เครื่องมือ ให้ตอบในรูปแบบ JSON:
{
  "action": "use_tool",
  "tool_call": {
    "tool_name": "ชื่อเครื่องมือ",
    "arguments": "{\"param1\": \"value1\"}"
  }
}

ถ้าไม่ต้องการใช้เครื่องมือ ให้ตอบแบบปกติ`, strings.Join(toolDescriptions, "\n"))

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userMessage),
		}

		fmt.Printf("🔧 Tool Dispatcher: Analyzing user request\n")
		return messages, nil
	})

	// Tool Executor - เรียกใช้ tool และจัดการผลลัพธ์
	toolExecutor := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		fmt.Printf("🤖 Tool Executor: Processing request\n")
		
		// Get LLM response first
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("LLM generation failed: %w", err)
		}

		content := strings.TrimSpace(response.Content)
		
		// Check if response is a tool call
		if strings.Contains(content, "\"action\": \"use_tool\"") {
			fmt.Printf("🔧 Detected tool call request\n")
			
			// Parse tool call
			var toolRequest struct {
				Action   string   `json:"action"`
				ToolCall ToolCall `json:"tool_call"`
			}

			if err := json.Unmarshal([]byte(content), &toolRequest); err != nil {
				return fmt.Sprintf("Error parsing tool call: %v\nOriginal response: %s", err, content), nil
			}

			// Execute tool
			tool, exists := tools[toolRequest.ToolCall.ToolName]
			if !exists {
				return fmt.Sprintf("Unknown tool: %s", toolRequest.ToolCall.ToolName), nil
			}

			fmt.Printf("⚡ Executing tool: %s\n", tool.Name)
			result, err := tool.Function(ctx, toolRequest.ToolCall.Arguments)
			if err != nil {
				return fmt.Sprintf("Tool execution failed: %v", err), nil
			}

			fmt.Printf("✅ Tool result: %s\n", result)
			return result, nil
		}

		// Return normal response
		fmt.Printf("💬 Normal response (no tool needed)\n")
		return content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("tool_dispatcher", toolDispatcher)
	graph.AddLambdaNode("tool_executor", toolExecutor)

	// เชื่อม edges
	graph.AddEdge(compose.START, "tool_dispatcher")
	graph.AddEdge("tool_dispatcher", "tool_executor")
	graph.AddEdge("tool_executor", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile basic tool graph: %w", err)
	}

	// ทดสอบ
	testQueries := []string{
		"เวลาตอนนี้เท่าไหร่?",
		"อากาศที่กรุงเทพมหานครเป็นอย่างไร?",
		"เวลาที่โตเกียวตอนนี้",
		"สวัสดีครับ",
		"อากาศที่เชียงใหม่วันนี้",
	}

	for i, query := range testQueries {
		fmt.Printf("\n--- Basic Tool Test %d ---\n", i+1)
		fmt.Printf("Query: %s\n", query)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, query)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Response: %s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// Calculator Agent
func runCalculatorAgent(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Calculator Tools
	calculatorTools := map[string]Tool{
		"add": {
			Name:        "add",
			Description: "Add two numbers",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]float64
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				a, okA := params["a"]
				b, okB := params["b"]
				if !okA || !okB {
					return "", errors.New("parameters 'a' and 'b' are required")
				}

				result := a + b
				return fmt.Sprintf("%.2f + %.2f = %.2f", a, b, result), nil
			},
		},
		"multiply": {
			Name:        "multiply",
			Description: "Multiply two numbers",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]float64
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				a, okA := params["a"]
				b, okB := params["b"]
				if !okA || !okB {
					return "", errors.New("parameters 'a' and 'b' are required")
				}

				result := a * b
				return fmt.Sprintf("%.2f × %.2f = %.2f", a, b, result), nil
			},
		},
		"power": {
			Name:        "power",
			Description: "Calculate power (a^b)",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]float64
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				base, okBase := params["base"]
				exponent, okExp := params["exponent"]
				if !okBase || !okExp {
					return "", errors.New("parameters 'base' and 'exponent' are required")
				}

				result := math.Pow(base, exponent)
				return fmt.Sprintf("%.2f^%.2f = %.2f", base, exponent, result), nil
			},
		},
		"sqrt": {
			Name:        "sqrt",
			Description: "Calculate square root",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]float64
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				number, ok := params["number"]
				if !ok {
					return "", errors.New("parameter 'number' is required")
				}

				if number < 0 {
					return "", errors.New("cannot calculate square root of negative number")
				}

				result := math.Sqrt(number)
				return fmt.Sprintf("√%.2f = %.2f", number, result), nil
			},
		},
	}

	// Math Problem Analyzer
	mathAnalyzer := compose.InvokableLambda(func(ctx context.Context, userQuery string) ([]*schema.Message, error) {
		// สร้าง tool descriptions
		var toolDescriptions []string
		for _, tool := range calculatorTools {
			toolDescriptions = append(toolDescriptions, fmt.Sprintf("- %s: %s", tool.Name, tool.Description))
		}

		systemPrompt := fmt.Sprintf(`คุณเป็น Math Assistant ที่สามารถแก้โจทย์คณิตศาสตร์

Available Calculator Tools:
%s

วิเคราะห์โจทย์และใช้เครื่องมือที่เหมาะสม
ถ้าต้องการใช้เครื่องมือ ให้ตอบในรูปแบบ:
{
  "action": "calculate",
  "tool_name": "ชื่อเครื่องมือ",
  "arguments": "{\"param\": value}",
  "explanation": "อธิบายขั้นตอน"
}

ถ้าโจทย์ซับซ้อน อาจต้องใช้หลายขั้นตอน

ตัวอย่าง:
- "5 + 3" → ใช้ add tool
- "2 × 4" → ใช้ multiply tool
- "2^3" → ใช้ power tool
- "√16" → ใช้ sqrt tool`, strings.Join(toolDescriptions, "\n"))

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userQuery),
		}

		fmt.Printf("🧮 Math Analyzer: Analyzing problem\n")
		return messages, nil
	})

	// Calculator Executor
	calculatorExecutor := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		fmt.Printf("🤖 Calculator Executor: Processing\n")
		
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("LLM generation failed: %w", err)
		}

		content := strings.TrimSpace(response.Content)
		
		// Check for calculation request
		if strings.Contains(content, "\"action\": \"calculate\"") {
			fmt.Printf("🧮 Detected calculation request\n")
			
			var calcRequest struct {
				Action      string `json:"action"`
				ToolName    string `json:"tool_name"`
				Arguments   string `json:"arguments"`
				Explanation string `json:"explanation"`
			}

			if err := json.Unmarshal([]byte(content), &calcRequest); err != nil {
				return fmt.Sprintf("Error parsing calculation request: %v", err), nil
			}

			// Execute calculation
			tool, exists := calculatorTools[calcRequest.ToolName]
			if !exists {
				return fmt.Sprintf("Unknown calculator tool: %s", calcRequest.ToolName), nil
			}

			fmt.Printf("⚡ Executing: %s\n", tool.Name)
			fmt.Printf("📝 Explanation: %s\n", calcRequest.Explanation)
			
			result, err := tool.Function(ctx, calcRequest.Arguments)
			if err != nil {
				return fmt.Sprintf("Calculation failed: %v", err), nil
			}

			return fmt.Sprintf("%s\n\nResult: %s", calcRequest.Explanation, result), nil
		}

		return content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("math_analyzer", mathAnalyzer)
	graph.AddLambdaNode("calculator_executor", calculatorExecutor)

	// เชื่อม edges
	graph.AddEdge(compose.START, "math_analyzer")
	graph.AddEdge("math_analyzer", "calculator_executor")
	graph.AddEdge("calculator_executor", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile calculator graph: %w", err)
	}

	// ทดสอบ
	testProblems := []string{
		"5 + 3 เท่ากับเท่าไหร่?",
		"คำนวณ 7 × 8",
		"2 ยกกำลัง 5",
		"รากที่สองของ 64",
		"(5 + 3) × 2", // Complex calculation
	}

	for i, problem := range testProblems {
		fmt.Printf("\n--- Calculator Test %d ---\n", i+1)
		fmt.Printf("Problem: %s\n", problem)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, problem)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Solution:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// Multi-Tool Agent
func runMultiToolAgent(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Comprehensive Tool Set
	allTools := map[string]Tool{
		// Math tools
		"calculate": {
			Name:        "calculate",
			Description: "Perform basic math operations (add, subtract, multiply, divide)",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]interface{}
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				operation, ok := params["operation"].(string)
				if !ok {
					return "", errors.New("operation parameter is required")
				}

				aFloat, ok := params["a"].(float64)
				if !ok {
					return "", errors.New("parameter 'a' must be a number")
				}

				bFloat, ok := params["b"].(float64)
				if !ok {
					return "", errors.New("parameter 'b' must be a number")
				}

				var result float64
				switch operation {
				case "add":
					result = aFloat + bFloat
				case "subtract":
					result = aFloat - bFloat
				case "multiply":
					result = aFloat * bFloat
				case "divide":
					if bFloat == 0 {
						return "", errors.New("division by zero")
					}
					result = aFloat / bFloat
				default:
					return "", fmt.Errorf("unknown operation: %s", operation)
				}

				return fmt.Sprintf("%.2f %s %.2f = %.2f", aFloat, operation, bFloat, result), nil
			},
		},
		// Text tools
		"text_analysis": {
			Name:        "text_analysis",
			Description: "Analyze text (count words, characters, find patterns)",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]string
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				text, ok := params["text"]
				if !ok {
					return "", errors.New("text parameter is required")
				}

				analysis, ok := params["analysis"]
				if !ok {
					analysis = "basic"
				}

				switch analysis {
				case "basic":
					words := len(strings.Fields(text))
					chars := len(text)
					lines := len(strings.Split(text, "\n"))
					return fmt.Sprintf("Text Analysis:\n- Words: %d\n- Characters: %d\n- Lines: %d", words, chars, lines), nil
				case "uppercase":
					return fmt.Sprintf("Uppercase: %s", strings.ToUpper(text)), nil
				case "lowercase":
					return fmt.Sprintf("Lowercase: %s", strings.ToLower(text)), nil
				case "reverse":
					runes := []rune(text)
					for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
						runes[i], runes[j] = runes[j], runes[i]
					}
					return fmt.Sprintf("Reversed: %s", string(runes)), nil
				default:
					return "", fmt.Errorf("unknown analysis type: %s", analysis)
				}
			},
		},
		// Unit conversion
		"unit_converter": {
			Name:        "unit_converter",
			Description: "Convert between units (temperature, length, weight)",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]interface{}
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				value, ok := params["value"].(float64)
				if !ok {
					return "", errors.New("value parameter is required and must be a number")
				}

				fromUnit, ok := params["from"].(string)
				if !ok {
					return "", errors.New("from parameter is required")
				}

				toUnit, ok := params["to"].(string)
				if !ok {
					return "", errors.New("to parameter is required")
				}

				var result float64
				var unit string

				// Temperature conversion
				if fromUnit == "celsius" && toUnit == "fahrenheit" {
					result = (value * 9/5) + 32
					unit = "°F"
				} else if fromUnit == "fahrenheit" && toUnit == "celsius" {
					result = (value - 32) * 5/9
					unit = "°C"
				} else if fromUnit == "celsius" && toUnit == "kelvin" {
					result = value + 273.15
					unit = "K"
				} else if fromUnit == "kelvin" && toUnit == "celsius" {
					result = value - 273.15
					unit = "°C"
				} else if fromUnit == "meter" && toUnit == "feet" {
					result = value * 3.28084
					unit = "ft"
				} else if fromUnit == "feet" && toUnit == "meter" {
					result = value / 3.28084
					unit = "m"
				} else if fromUnit == "kg" && toUnit == "pounds" {
					result = value * 2.20462
					unit = "lbs"
				} else if fromUnit == "pounds" && toUnit == "kg" {
					result = value / 2.20462
					unit = "kg"
				} else {
					return "", fmt.Errorf("unsupported conversion: %s to %s", fromUnit, toUnit)
				}

				return fmt.Sprintf("%.2f %s = %.2f %s", value, fromUnit, result, unit), nil
			},
		},
		// Random generator
		"random_generator": {
			Name:        "random_generator",
			Description: "Generate random numbers, passwords, or choices",
			Function: func(ctx context.Context, args string) (string, error) {
				var params map[string]interface{}
				if err := json.Unmarshal([]byte(args), &params); err != nil {
					return "", fmt.Errorf("invalid arguments: %w", err)
				}

				genType, ok := params["type"].(string)
				if !ok {
					return "", errors.New("type parameter is required")
				}

				switch genType {
				case "number":
					min, okMin := params["min"].(float64)
					max, okMax := params["max"].(float64)
					if !okMin || !okMax {
						return "", errors.New("min and max parameters are required for number generation")
					}
					
					// Simple pseudo-random (not cryptographically secure)
					result := min + (max-min)*0.5 // Mock random for demo
					return fmt.Sprintf("Random number between %.0f and %.0f: %.0f", min, max, result), nil
				
				case "choice":
					choices, ok := params["choices"].([]interface{})
					if !ok {
						return "", errors.New("choices parameter is required for choice generation")
					}
					
					if len(choices) == 0 {
						return "", errors.New("choices cannot be empty")
					}
					
					// Mock choice selection (first item for demo)
					chosen := choices[0]
					return fmt.Sprintf("Random choice: %v", chosen), nil
				
				default:
					return "", fmt.Errorf("unknown generation type: %s", genType)
				}
			},
		},
	}

	// Intelligent Tool Router
	toolRouter := compose.InvokableLambda(func(ctx context.Context, userQuery string) ([]*schema.Message, error) {
		// Create comprehensive tool descriptions
		var toolDescriptions []string
		for _, tool := range allTools {
			toolDescriptions = append(toolDescriptions, fmt.Sprintf("- %s: %s", tool.Name, tool.Description))
		}

		systemPrompt := fmt.Sprintf(`คุณเป็น AI Agent ที่มีเครื่องมือหลากหลาย

Available Tools:
%s

วิเคราะห์คำขอของผู้ใช้และเลือกเครื่องมือที่เหมาะสม:

1. ถ้าเป็นคำถามคณิตศาสตร์ → ใช้ calculate
2. ถ้าเป็นการวิเคราะห์ข้อความ → ใช้ text_analysis  
3. ถ้าเป็นการแปลงหน่วย → ใช้ unit_converter
4. ถ้าต้องการสุ่ม → ใช้ random_generator
5. ถ้าเป็นคำถามทั่วไป → ตอบเองไม่ต้องใช้เครื่องมือ

รูปแบบการใช้เครื่องมือ:
{
  "action": "use_tool",
  "tool_name": "ชื่อเครื่องมือ",
  "arguments": "{\"param1\": \"value1\", \"param2\": value2}",
  "reasoning": "เหตุผลที่เลือกเครื่องมือนี้"
}`, strings.Join(toolDescriptions, "\n"))

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userQuery),
		}

		fmt.Printf("🎯 Tool Router: Analyzing request and selecting tools\n")
		return messages, nil
	})

	// Multi-Tool Executor
	multiToolExecutor := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		fmt.Printf("🚀 Multi-Tool Executor: Processing\n")
		
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("LLM generation failed: %w", err)
		}

		content := strings.TrimSpace(response.Content)
		
		// Check for tool usage
		if strings.Contains(content, "\"action\": \"use_tool\"") {
			fmt.Printf("🔧 Detected tool usage request\n")
			
			var toolRequest struct {
				Action    string `json:"action"`
				ToolName  string `json:"tool_name"`
				Arguments string `json:"arguments"`
				Reasoning string `json:"reasoning"`
			}

			if err := json.Unmarshal([]byte(content), &toolRequest); err != nil {
				return fmt.Sprintf("Error parsing tool request: %v", err), nil
			}

			// Execute tool
			tool, exists := allTools[toolRequest.ToolName]
			if !exists {
				return fmt.Sprintf("Unknown tool: %s", toolRequest.ToolName), nil
			}

			fmt.Printf("⚡ Executing tool: %s\n", tool.Name)
			fmt.Printf("💭 Reasoning: %s\n", toolRequest.Reasoning)
			
			result, err := tool.Function(ctx, toolRequest.Arguments)
			if err != nil {
				return fmt.Sprintf("Tool execution failed: %v", err), nil
			}

			return fmt.Sprintf("Reasoning: %s\n\nResult: %s", toolRequest.Reasoning, result), nil
		}

		// Return normal response
		return content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("tool_router", toolRouter)
	graph.AddLambdaNode("multi_tool_executor", multiToolExecutor)

	// เชื่อม edges
	graph.AddEdge(compose.START, "tool_router")
	graph.AddEdge("tool_router", "multi_tool_executor")
	graph.AddEdge("multi_tool_executor", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile multi-tool graph: %w", err)
	}

	// ทดสอบ
	testRequests := []string{
		"คำนวณ 15 + 25",
		"วิเคราะห์ข้อความ 'Hello World! How are you today?'",
		"แปลง 25 องศาเซลเซียส เป็นฟาเรนไฮต์",
		"สุ่มตัวเลขระหว่าง 1 ถึง 100",
		"เปลี่ยนข้อความ 'Hello' เป็นตัวพิมพ์ใหญ่",
		"แปลง 180 ซม. เป็นฟุต",
		"สุ่มเลือกจาก ['แอปเปิล', 'กล้วย', 'ส้ม']",
		"วันนี้อากาศเป็นอย่างไรบ้าง?",
	}

	for i, request := range testRequests {
		fmt.Printf("\n--- Multi-Tool Test %d ---\n", i+1)
		fmt.Printf("Request: %s\n", request)
		
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