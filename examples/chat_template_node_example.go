package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// Template ข้อมูลสำหรับการเติมค่า
type TemplateData struct {
	UserName      string
	ProjectName   string
	Language      string
	Difficulty    string
	Context       string
	Question      string
	Examples      []string
	Requirements  []string
}

// main function for running chat template examples
func main() {
	runChatTemplateExample()
}

// ตัวอย่าง Chat Template Node - สร้าง prompt แบบ dynamic
func runChatTemplateExample() {
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

	// สร้าง Chat Model
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

	// === ตัวอย่าง 1: Basic Template Node ===
	fmt.Println("=== Basic Template Node ===")
	runBasicTemplateNode(ctx, model)

	// === ตัวอย่าง 2: Advanced Template with Conditions ===
	fmt.Println("\n=== Advanced Template with Conditions ===")
	runAdvancedTemplateNode(ctx, model)

	// === ตัวอย่าง 3: Multi-Template System ===
	fmt.Println("\n=== Multi-Template System ===")
	runMultiTemplateNode(ctx, model)
}

// Basic Template Node
func runBasicTemplateNode(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[TemplateData, string]()

	// Chat Template Node - เติมข้อมูลใน template
	basicTemplateNode := compose.InvokableLambda(func(ctx context.Context, data TemplateData) ([]*schema.Message, error) {
		// Basic template สำหรับสอนการเขียนโปรแกรม
		systemTemplate := `คุณเป็นครูสอน {{.Language}} ที่มีประสบการณ์สูง

Project Context: {{.ProjectName}}
Student Level: {{.Difficulty}}
Student Name: {{.UserName}}

ตอบคำถามให้เหมาะกับระดับของนักเรียน ใช้ภาษาที่เข้าใจง่าย และให้ตัวอย่างที่เกี่ยวข้องกับโปรเจค`

		userTemplate := `คำถาม: {{.Question}}

{{if .Context}}
บริบทเพิ่มเติม: {{.Context}}
{{end}}

{{if .Examples}}
ตัวอย่างที่เกี่ยวข้อง:
{{range .Examples}}
- {{.}}
{{end}}
{{end}}`

		// สร้าง templates
		systemTmpl, err := template.New("system").Parse(systemTemplate)
		if err != nil {
			return nil, fmt.Errorf("error parsing system template: %w", err)
		}

		userTmpl, err := template.New("user").Parse(userTemplate)
		if err != nil {
			return nil, fmt.Errorf("error parsing user template: %w", err)
		}

		// เติมข้อมูลใน templates
		var systemBuf strings.Builder
		if err := systemTmpl.Execute(&systemBuf, data); err != nil {
			return nil, fmt.Errorf("error executing system template: %w", err)
		}

		var userBuf strings.Builder
		if err := userTmpl.Execute(&userBuf, data); err != nil {
			return nil, fmt.Errorf("error executing user template: %w", err)
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemBuf.String()),
			schema.UserMessage(userBuf.String()),
		}

		fmt.Printf("🎯 Template Node: Generated %d messages for %s\n", len(messages), data.UserName)
		fmt.Printf("   System: %s...\n", systemBuf.String()[:100])
		fmt.Printf("   User: %s...\n", userBuf.String()[:100])

		return messages, nil
	})

	// Chat Model Node
	chatModelNode := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		fmt.Printf("🤖 Chat Model: Processing templated messages\n")
		
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("chat model error: %w", err)
		}

		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("template", basicTemplateNode)
	graph.AddLambdaNode("chat", chatModelNode)

	// เชื่อม edges
	graph.AddEdge(compose.START, "template")
	graph.AddEdge("template", "chat")
	graph.AddEdge("chat", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling graph: %v\n", err)
		return
	}

	// ทดสอบ
	testData := []TemplateData{
		{
			UserName:    "สมชาย",
			ProjectName: "Go Microservices API",
			Language:    "Go",
			Difficulty:  "Beginner",
			Question:    "Goroutine คืออะไร และใช้ยังไง?",
			Context:     "กำลังเรียนรู้ concurrent programming ใน Go",
			Examples:    []string{"go func()", "channels", "sync.WaitGroup"},
		},
		{
			UserName:    "แมรี่",
			ProjectName: "React E-commerce",
			Language:    "JavaScript",
			Difficulty:  "Intermediate",
			Question:    "วิธีการจัดการ state ใน React?",
			Examples:    []string{"useState", "useReducer", "Context API"},
		},
	}

	for i, data := range testData {
		fmt.Printf("\n--- Test %d ---\n", i+1)
		result, err := runnable.Invoke(ctx, data)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Response for %s:\n%s\n", data.UserName, result)
		fmt.Println(strings.Repeat("-", 80))
	}
}

// Advanced Template with Conditions
func runAdvancedTemplateNode(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[map[string]interface{}, string]()

	// Advanced Template Node with conditional logic
	advancedTemplateNode := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) ([]*schema.Message, error) {
		// Extract data from input
		templateType := input["template_type"].(string)
		data := input["data"].(map[string]interface{})

		var systemTemplate, userTemplate string

		// เลือก template ตามประเภท
		switch templateType {
		case "code_review":
			systemTemplate = `คุณเป็น Senior Developer ที่เชี่ยวชาญ {{.language}}

Review Guidelines:
{{if eq .difficulty "beginner"}}
- ให้คำแนะนำเบื้องต้น เน้นความเข้าใจ
- อธิบายแนวคิดพื้นฐาน
{{else if eq .difficulty "intermediate"}}
- ชี้จุดที่ต้องปรับปรุง และแนะนำ best practices
- แนะนำ patterns และ techniques
{{else}}
- ให้ feedback แบบละเอียด รวมถึง performance และ security
- แนะนำ advanced patterns และ optimizations
{{end}}

Project Type: {{.project_type}}`

			userTemplate = `โปรดรีวิวโค้ดนี้:

` + "```" + `{{.language}}
{{.code}}
` + "```" + `

{{if .specific_concerns}}
จุดที่อยากให้ดูเป็นพิเศษ:
{{range .specific_concerns}}
- {{.}}
{{end}}
{{end}}`

		case "documentation":
			systemTemplate = `คุณเป็นนักเขียนเอกสารเทคนิคที่เชี่ยวชาญ

Documentation Style:
{{if eq .audience "developer"}}
- เขียนให้นักพัฒนาเข้าใจ มีตัวอย่างโค้ด
- รวม API references และ code examples
{{else if eq .audience "user"}}
- เขียนให้ผู้ใช้งานทั่วไปเข้าใจ
- ใช้ภาษาง่าย หลีกเลี่ยงศัพท์เทคนิค
{{else}}
- เขียนแบบสมดุล เหมาะกับทั้งสองกลุ่ม
{{end}}

Project: {{.project_name}}`

			userTemplate = `สร้างเอกสารสำหรับ: {{.topic}}

{{if .sections}}
ส่วนที่ต้องการ:
{{range .sections}}
- {{.}}
{{end}}
{{end}}

{{if .examples_needed}}
ต้องการตัวอย่าง: {{.examples_needed}}
{{end}}`

		case "troubleshooting":
			systemTemplate = `คุณเป็นผู้เชี่ยวชาญด้าน troubleshooting ระบบ {{.system_type}}

Problem-Solving Approach:
1. วิเคราะห์อาการและสาเหตุที่เป็นไปได้
2. แนะนำขั้นตอนการแก้ไขตาม priority
3. ให้วิธีป้องกันปัญหาในอนาคต

{{if .urgency_level}}
Urgency Level: {{.urgency_level}}
{{if eq .urgency_level "critical"}}
⚠️ ให้แนวทางแก้ไขเร่งด่วนก่อน แล้วค่อยหาสาเหตุรากเดือน
{{end}}
{{end}}`

			userTemplate = `ปัญหาที่เกิดขึ้น:
{{.problem_description}}

{{if .error_messages}}
Error Messages:
{{range .error_messages}}
- {{.}}
{{end}}
{{end}}

{{if .system_info}}
ข้อมูลระบบ:
{{.system_info}}
{{end}}

{{if .steps_tried}}
ขั้นตอนที่ลองแล้ว:
{{range .steps_tried}}
- {{.}}
{{end}}
{{end}}`

		default:
			return nil, fmt.Errorf("unknown template type: %s", templateType)
		}

		// สร้างและเติมข้อมูลใน templates
		systemTmpl, err := template.New("system").Parse(systemTemplate)
		if err != nil {
			return nil, fmt.Errorf("error parsing system template: %w", err)
		}

		userTmpl, err := template.New("user").Parse(userTemplate)
		if err != nil {
			return nil, fmt.Errorf("error parsing user template: %w", err)
		}

		var systemBuf strings.Builder
		if err := systemTmpl.Execute(&systemBuf, data); err != nil {
			return nil, fmt.Errorf("error executing system template: %w", err)
		}

		var userBuf strings.Builder
		if err := userTmpl.Execute(&userBuf, data); err != nil {
			return nil, fmt.Errorf("error executing user template: %w", err)
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemBuf.String()),
			schema.UserMessage(userBuf.String()),
		}

		fmt.Printf("🎯 Advanced Template: Generated %s template\n", templateType)
		return messages, nil
	})

	// Chat Model Node
	chatModelNode := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("chat model error: %w", err)
		}
		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("advanced_template", advancedTemplateNode)
	graph.AddLambdaNode("chat", chatModelNode)

	// เชื่อม edges
	graph.AddEdge(compose.START, "advanced_template")
	graph.AddEdge("advanced_template", "chat")
	graph.AddEdge("chat", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling advanced graph: %v\n", err)
		return
	}

	// ทดสอบ templates ต่างๆ
	testCases := []map[string]interface{}{
		{
			"template_type": "code_review",
			"data": map[string]interface{}{
				"language":      "Go",
				"difficulty":    "intermediate",
				"project_type":  "Web API",
				"code":          "func process(data []string) {\n    for i := 0; i < len(data); i++ {\n        fmt.Println(data[i])\n    }\n}",
				"specific_concerns": []string{"performance", "Go idioms"},
			},
		},
		{
			"template_type": "documentation",
			"data": map[string]interface{}{
				"audience":        "developer",
				"project_name":    "Eino Graph Library",
				"topic":           "Getting Started Guide",
				"sections":        []string{"Installation", "Basic Usage", "Examples"},
				"examples_needed": "true",
			},
		},
		{
			"template_type": "troubleshooting",
			"data": map[string]interface{}{
				"system_type":         "Go Web Service",
				"urgency_level":       "high",
				"problem_description": "API response time เพิ่มขึ้นจาก 100ms เป็น 2000ms",
				"error_messages":      []string{"context deadline exceeded", "connection timeout"},
				"system_info":         "Go 1.21, 8GB RAM, Docker container",
				"steps_tried":         []string{"restart service", "check logs", "monitor CPU"},
			},
		},
	}

	for i, testCase := range testCases {
		templateType := testCase["template_type"].(string)
		fmt.Printf("\n--- Advanced Template Test %d (%s) ---\n", i+1, templateType)
		
		result, err := runnable.Invoke(ctx, testCase)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result for %s template:\n%s\n", templateType, result)
		fmt.Println(strings.Repeat("-", 80))
	}
}

// Multi-Template System
func runMultiTemplateNode(ctx context.Context, model *openai.ChatModel) {
	graph := compose.NewGraph[map[string]interface{}, string]()

	// Template Selector - เลือก template ที่เหมาะสม
	templateSelector := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (string, error) {
		userInput := input["user_input"].(string)
		_ = input["context"].(map[string]interface{}) // not used in this example

		// AI-powered template selection logic
		userInputLower := strings.ToLower(userInput)
		
		var selectedTemplate string
		if strings.Contains(userInputLower, "review") || strings.Contains(userInputLower, "โค้ด") {
			selectedTemplate = "code_review"
		} else if strings.Contains(userInputLower, "document") || strings.Contains(userInputLower, "เอกสาร") {
			selectedTemplate = "documentation"
		} else if strings.Contains(userInputLower, "problem") || strings.Contains(userInputLower, "error") || strings.Contains(userInputLower, "ปัญหา") {
			selectedTemplate = "troubleshooting"
		} else if strings.Contains(userInputLower, "explain") || strings.Contains(userInputLower, "อธิบาย") {
			selectedTemplate = "explanation"
		} else {
			selectedTemplate = "general"
		}

		fmt.Printf("🤖 Template Selector: Selected '%s' template for input\n", selectedTemplate)
		
		// Combine template type with original input
		result := fmt.Sprintf("%s|%s", selectedTemplate, userInput)
		return result, nil
	})

	// Multi-Template Node
	multiTemplateNode := compose.InvokableLambda(func(ctx context.Context, input string) ([]*schema.Message, error) {
		parts := strings.SplitN(input, "|", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid input format")
		}

		templateType := parts[0]
		userInput := parts[1]

		var systemPrompt string
		switch templateType {
		case "code_review":
			systemPrompt = "คุณเป็น Senior Developer ที่เชี่ยวชาญการ review โค้ด ให้คำแนะนำที่สร้างสรรค์และเป็นประโยชน์"
		case "documentation":
			systemPrompt = "คุณเป็นนักเขียนเอกสารเทคนิคที่เชี่ยวชาญ เขียนเอกสารที่ชัดเจนและเข้าใจง่าย"
		case "troubleshooting":
			systemPrompt = "คุณเป็นผู้เชี่ยวชาญด้าน troubleshooting ให้แนวทางแก้ไขปัญหาอย่างเป็นระบบ"
		case "explanation":
			systemPrompt = "คุณเป็นครูที่เก่งในการอธิบาย อธิบายให้เข้าใจง่ายและให้ตัวอย่างที่เหมาะสม"
		default:
			systemPrompt = "คุณเป็น AI ผู้ช่วยที่เป็นมิตรและมีความรู้กว้างขวาง"
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userInput),
		}

		fmt.Printf("🎯 Multi-Template: Using %s template\n", templateType)
		return messages, nil
	})

	// Chat Model Node
	chatModelNode := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("chat model error: %w", err)
		}
		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("selector", templateSelector)
	graph.AddLambdaNode("multi_template", multiTemplateNode)
	graph.AddLambdaNode("chat", chatModelNode)

	// เชื่อม edges
	graph.AddEdge(compose.START, "selector")
	graph.AddEdge("selector", "multi_template")
	graph.AddEdge("multi_template", "chat")
	graph.AddEdge("chat", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		fmt.Printf("Error compiling multi-template graph: %v\n", err)
		return
	}

	// ทดสอบ
	testInputs := []map[string]interface{}{
		{
			"user_input": "ช่วยรีวิวโค้ด Go function นี้ให้หน่อย",
			"context":    map[string]interface{}{"project": "web_api"},
		},
		{
			"user_input": "สร้างเอกสาร API documentation",
			"context":    map[string]interface{}{"audience": "developers"},
		},
		{
			"user_input": "API ของเราช้ามาก มีปัญหาอะไร",
			"context":    map[string]interface{}{"system": "microservices"},
		},
		{
			"user_input": "อธิบาย Eino Graph ให้ฟังหน่อย",
			"context":    map[string]interface{}{"level": "beginner"},
		},
		{
			"user_input": "สวัสดีครับ วันนี้เป็นยังไงบ้าง",
			"context":    map[string]interface{}{},
		},
	}

	for i, testInput := range testInputs {
		fmt.Printf("\n--- Multi-Template Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", testInput["user_input"])
		
		result, err := runnable.Invoke(ctx, testInput)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Response:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}
}