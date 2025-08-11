package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// Tool interface - ทุก tool ต้อง implement
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, params map[string]interface{}) (string, error)
}

// WeatherTool - ดูสภาพอากาศ
type WeatherTool struct{}

func (w *WeatherTool) Name() string {
	return "get_weather"
}

func (w *WeatherTool) Description() string {
	return "Get weather information for a city. Parameters: {\"city\": \"city_name\"}"
}

func (w *WeatherTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	city, ok := params["city"].(string)
	if !ok {
		return "", fmt.Errorf("city parameter is required")
	}
	
	// Simulate API call (ในความเป็นจริงจะเรียก weather API จริง)
	weatherData := map[string]interface{}{
		"city":        city,
		"temperature": 28 + (len(city) % 10), // fake temperature
		"humidity":    60 + (len(city) % 30),
		"condition":   []string{"sunny", "cloudy", "rainy"}[len(city)%3],
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
	}
	
	result, _ := json.MarshalIndent(weatherData, "", "  ")
	return string(result), nil
}

// CalculatorTool - คำนวณเลข
type CalculatorTool struct{}

func (c *CalculatorTool) Name() string {
	return "calculator"
}

func (c *CalculatorTool) Description() string {
	return "Perform mathematical calculations. Parameters: {\"expression\": \"math_expression\"}"
}

func (c *CalculatorTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	expression, ok := params["expression"].(string)
	if !ok {
		return "", fmt.Errorf("expression parameter is required")
	}
	
	// Simple calculator (ใช้ bc command)
	cmd := exec.CommandContext(ctx, "bc", "-l")
	cmd.Stdin = strings.NewReader(expression)
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("calculation error: %v", err)
	}
	
	result := strings.TrimSpace(string(output))
	return fmt.Sprintf("Result: %s", result), nil
}

// FileSystemTool - จัดการไฟล์
type FileSystemTool struct{}

func (f *FileSystemTool) Name() string {
	return "filesystem"
}

func (f *FileSystemTool) Description() string {
	return "File system operations. Parameters: {\"action\": \"list|read|write\", \"path\": \"file_path\", \"content\": \"file_content\"}"
}

func (f *FileSystemTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action parameter is required")
	}
	
	path, _ := params["path"].(string)
	
	switch action {
	case "list":
		if path == "" {
			path = "."
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("cannot list directory: %v", err)
		}
		
		var files []string
		for _, entry := range entries {
			if entry.IsDir() {
				files = append(files, fmt.Sprintf("📁 %s/", entry.Name()))
			} else {
				files = append(files, fmt.Sprintf("📄 %s", entry.Name()))
			}
		}
		return strings.Join(files, "\n"), nil
		
	case "read":
		if path == "" {
			return "", fmt.Errorf("path parameter is required for read action")
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("cannot read file: %v", err)
		}
		return string(content), nil
		
	case "write":
		if path == "" {
			return "", fmt.Errorf("path parameter is required for write action")
		}
		content, _ := params["content"].(string)
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return "", fmt.Errorf("cannot write file: %v", err)
		}
		return fmt.Sprintf("File written successfully: %s", path), nil
		
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// HTTPTool - เรียก HTTP APIs
type HTTPTool struct{}

func (h *HTTPTool) Name() string {
	return "http_request"
}

func (h *HTTPTool) Description() string {
	return "Make HTTP requests. Parameters: {\"url\": \"http_url\", \"method\": \"GET|POST\", \"headers\": {}, \"body\": \"request_body\"}"
}

func (h *HTTPTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	url, ok := params["url"].(string)
	if !ok {
		return "", fmt.Errorf("url parameter is required")
	}
	
	method, ok := params["method"].(string)
	if !ok {
		method = "GET"
	}
	
	var body io.Reader
	if bodyStr, ok := params["body"].(string); ok {
		body = strings.NewReader(bodyStr)
	}
	
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return "", fmt.Errorf("cannot create request: %v", err)
	}
	
	// Add headers if provided
	if headers, ok := params["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if valueStr, ok := value.(string); ok {
				req.Header.Set(key, valueStr)
			}
		}
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("cannot read response: %v", err)
	}
	
	result := fmt.Sprintf("Status: %d %s\nResponse:\n%s", 
		resp.StatusCode, resp.Status, string(responseBody))
	
	return result, nil
}

// ToolManager - จัดการ tools ทั้งหมด
type ToolManager struct {
	tools map[string]Tool
	model *openai.ChatModel
	ctx   context.Context
}

func NewToolManager(model *openai.ChatModel, ctx context.Context) *ToolManager {
	tm := &ToolManager{
		tools: make(map[string]Tool),
		model: model,
		ctx:   ctx,
	}
	
	// ลงทะเบียน tools
	tm.RegisterTool(&WeatherTool{})
	tm.RegisterTool(&CalculatorTool{})
	tm.RegisterTool(&FileSystemTool{})
	tm.RegisterTool(&HTTPTool{})
	
	return tm
}

func (tm *ToolManager) RegisterTool(tool Tool) {
	tm.tools[tool.Name()] = tool
}

func (tm *ToolManager) GetAvailableTools() []Tool {
	var tools []Tool
	for _, tool := range tm.tools {
		tools = append(tools, tool)
	}
	return tools
}

func (tm *ToolManager) ExecuteTool(name string, params map[string]interface{}) (string, error) {
	tool, exists := tm.tools[name]
	if !exists {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	
	return tool.Execute(tm.ctx, params)
}

// AIAgent - AI ที่มี tools
type AIAgent struct {
	toolManager *ToolManager
	model       *openai.ChatModel
	ctx         context.Context
}

func NewAIAgent(model *openai.ChatModel, ctx context.Context) *AIAgent {
	return &AIAgent{
		toolManager: NewToolManager(model, ctx),
		model:       model,
		ctx:         ctx,
	}
}

func (agent *AIAgent) ProcessMessage(userMessage string) (string, error) {
	// สร้าง system prompt ที่มี tool descriptions
	toolDescriptions := ""
	for _, tool := range agent.toolManager.GetAvailableTools() {
		toolDescriptions += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
	}
	
	systemPrompt := fmt.Sprintf(`คุณเป็น AI ผู้ช่วยที่มีความสามารถพิเศษผ่าน tools ต่างๆ

Available tools:
%s

เมื่อผู้ใช้ถามคำถามที่ต้องใช้ tools:
1. ระบุ tool ที่จะใช้
2. ระบุ parameters ที่ต้องการ  
3. ตอบในรูปแบบ JSON:
   {"tool": "tool_name", "params": {"param1": "value1"}}

หากไม่ต้องใช้ tool ให้ตอบคำถามตามปกติ`, toolDescriptions)

	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userMessage),
	}
	
	response, err := agent.model.Generate(agent.ctx, messages)
	if err != nil {
		return "", err
	}
	
	// ตรวจสอบว่า response เป็น tool call หรือไม่
	responseText := response.Content
	
	// พยายาม parse JSON
	var toolCall struct {
		Tool   string                 `json:"tool"`
		Params map[string]interface{} `json:"params"`
	}
	
	if err := json.Unmarshal([]byte(responseText), &toolCall); err == nil && toolCall.Tool != "" {
		// เป็น tool call
		fmt.Printf("🔧 Using tool: %s\n", toolCall.Tool)
		fmt.Printf("📋 Parameters: %+v\n", toolCall.Params)
		
		result, err := agent.toolManager.ExecuteTool(toolCall.Tool, toolCall.Params)
		if err != nil {
			return fmt.Sprintf("❌ Tool error: %v", err), nil
		}
		
		// ส่ง result กลับไปให้ AI เพื่อสรุปผล
		followUpMessages := append(messages, 
			schema.AssistantMessage(responseText, nil),
			schema.UserMessage(fmt.Sprintf("Tool result: %s\n\nPlease summarize this result for the user in Thai.", result)),
		)
		
		finalResponse, err := agent.model.Generate(agent.ctx, followUpMessages)
		if err != nil {
			return fmt.Sprintf("Tool result:\n%s", result), nil
		}
		
		return finalResponse.Content, nil
	}
	
	// ไม่ใช่ tool call ตอบปกติ
	return responseText, nil
}

func toolsDemo() {
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

	agent := NewAIAgent(model, ctx)
	
	fmt.Println("🤖 AI Agent with Tools")
	fmt.Println("Available tools:")
	for _, tool := range agent.toolManager.GetAvailableTools() {
		fmt.Printf("  - %s: %s\n", tool.Name(), tool.Description())
	}
	fmt.Println()
	fmt.Println("ตัวอย่างคำถาม:")
	fmt.Println("- สภาพอากาศที่กรุงเทพเป็นอย่างไร?")
	fmt.Println("- คำนวณ 15 * 23 + 100")
	fmt.Println("- แสดงไฟล์ในโฟลเดอร์นี้")
	fmt.Println("- เรียก API https://api.github.com")
	fmt.Println()

	// Simple input loop
	for {
		fmt.Print("คุณ: ")
		var input string
		fmt.Scanln(&input)
		
		if input == "quit" || input == "exit" {
			break
		}
		
		response, err := agent.ProcessMessage(input)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		} else {
			fmt.Printf("🤖 AI: %s\n\n", response)
		}
	}
}

func main() {
	toolsDemo()
}