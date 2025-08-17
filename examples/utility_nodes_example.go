package main

import (
	"context"
	"encoding/json"
	"errors"
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

// DataItem represents a data item flowing through the pipeline
type DataItem struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Type     string                 `json:"type"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ProcessingResult represents the result of data processing
type ProcessingResult struct {
	Original   DataItem               `json:"original"`
	Processed  DataItem               `json:"processed"`
	Operations []string               `json:"operations"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// main function for utility nodes examples
func main() {
	if err := runUtilityNodesExample(); err != nil {
		log.Fatalf("Error running utility nodes example: %v", err)
	}
}

// ตัวอย่าง Utility Nodes - Passthrough, Transformer, และ Data Processing
func runUtilityNodesExample() error {
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

	// === ตัวอย่าง 1: Passthrough Node ===
	fmt.Println("=== Passthrough Node ===")
	if err := runPassthroughExample(ctx, model); err != nil {
		return fmt.Errorf("passthrough example failed: %w", err)
	}

	// === ตัวอย่าง 2: Data Transformer Nodes ===
	fmt.Println("\n=== Data Transformer Nodes ===")
	if err := runTransformerExample(ctx, model); err != nil {
		return fmt.Errorf("transformer example failed: %w", err)
	}

	// === ตัวอย่าง 3: Pipeline with Multiple Utilities ===
	fmt.Println("\n=== Pipeline with Multiple Utilities ===")
	if err := runPipelineExample(ctx, model); err != nil {
		return fmt.Errorf("pipeline example failed: %w", err)
	}

	return nil
}

// Passthrough Node Example
func runPassthroughExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Passthrough Node with Logging - ไม่เปลี่ยนข้อมูล แต่ log การทำงาน
	passthroughLogger := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("🔄 Passthrough Logger: Data flowing through\n")
		fmt.Printf("  Input: %s\n", input)
		fmt.Printf("  Timestamp: %s\n", time.Now().Format("15:04:05"))
		fmt.Printf("  Length: %d characters\n", len(input))
		
		// Simply pass the data through without modification
		return input, nil
	})

	// Passthrough Node with Validation - ตรวจสอบข้อมูลก่อนส่งต่อ
	passthroughValidator := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("✅ Passthrough Validator: Validating data\n")
		
		// Validate input
		if strings.TrimSpace(input) == "" {
			return "", errors.New("empty input is not allowed")
		}
		
		if len(input) > 1000 {
			fmt.Printf("  Warning: Input is very long (%d chars)\n", len(input))
		}
		
		if strings.Contains(strings.ToLower(input), "error") {
			fmt.Printf("  Warning: Input contains 'error' keyword\n")
		}
		
		fmt.Printf("  Validation passed ✓\n")
		return input, nil
	})

	// Main Processor - ประมวลผลหลัก
	mainProcessor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("🤖 Main Processor: Processing validated input\n")
		
		systemPrompt := `คุณเป็น AI ที่ปรับปรุงและเสริมเนื้อหาให้ดีขึ้น โดยไม่เปลี่ยนใจความหลัก`

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(fmt.Sprintf("ปรับปรุงเนื้อหานี้ให้ดีขึ้น: %s", input)),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("main processor generation failed: %w", err)
		}

		return response.Content, nil
	})

	// Passthrough Node with Metadata - เพิ่ม metadata โดยไม่เปลี่ยนเนื้อหา
	passthroughMetadata := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("📝 Passthrough Metadata: Adding processing metadata\n")
		
		metadata := map[string]interface{}{
			"processed_at":  time.Now().Format("2006-01-02 15:04:05"),
			"character_count": len(input),
			"word_count":     len(strings.Fields(input)),
			"lines_count":    len(strings.Split(input, "\n")),
		}

		metadataJson, _ := json.Marshal(metadata)
		
		// Add metadata as a comment (passthrough with enrichment)
		result := fmt.Sprintf("%s\n\n<!-- Processing Metadata: %s -->", input, string(metadataJson))
		
		fmt.Printf("  Added metadata: %s\n", string(metadataJson))
		return result, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("passthrough_logger", passthroughLogger)
	graph.AddLambdaNode("passthrough_validator", passthroughValidator)
	graph.AddLambdaNode("main_processor", mainProcessor)
	graph.AddLambdaNode("passthrough_metadata", passthroughMetadata)

	// เชื่อม edges
	graph.AddEdge(compose.START, "passthrough_logger")
	graph.AddEdge("passthrough_logger", "passthrough_validator")
	graph.AddEdge("passthrough_validator", "main_processor")
	graph.AddEdge("main_processor", "passthrough_metadata")
	graph.AddEdge("passthrough_metadata", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile passthrough graph: %w", err)
	}

	// ทดสอบ
	testInputs := []string{
		"Hello world! This is a simple message.",
		"Go เป็นภาษาโปรแกรมมิ่งที่เร็วและมีประสิทธิภาพ",
		"Error: something went wrong in the system",
		"", // Empty input to test validation
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Passthrough Test %d ---\n", i+1)
		fmt.Printf("Input: '%s'\n", input)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, input)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n%s\n", result)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// Data Transformer Example
func runTransformerExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[DataItem, ProcessingResult]()

	// Format Transformer - เปลี่ยนรูปแบบข้อมูล
	formatTransformer := compose.InvokableLambda(func(ctx context.Context, item DataItem) (DataItem, error) {
		fmt.Printf("🔄 Format Transformer: Transforming format\n")
		fmt.Printf("  Original type: %s\n", item.Type)
		
		var transformedContent string
		var newType string

		switch item.Type {
		case "text":
			// Transform to structured format
			transformedContent = fmt.Sprintf("📄 Text Document\nID: %s\nContent: %s", item.ID, item.Content)
			newType = "structured_text"
		case "question":
			// Transform to Q&A format
			transformedContent = fmt.Sprintf("❓ Question: %s", item.Content)
			newType = "formatted_question"
		case "code":
			// Transform to code block
			transformedContent = fmt.Sprintf("```\n%s\n```", item.Content)
			newType = "code_block"
		default:
			transformedContent = item.Content
			newType = item.Type
		}

		transformed := DataItem{
			ID:       item.ID,
			Content:  transformedContent,
			Type:     newType,
			Metadata: item.Metadata,
		}

		if transformed.Metadata == nil {
			transformed.Metadata = make(map[string]interface{})
		}
		transformed.Metadata["transformed_at"] = time.Now().Format("2006-01-02 15:04:05")
		transformed.Metadata["original_type"] = item.Type

		fmt.Printf("  New type: %s\n", newType)
		return transformed, nil
	})

	// Content Transformer - เปลี่ยนเนื้อหา
	contentTransformer := compose.InvokableLambda(func(ctx context.Context, item DataItem) (DataItem, error) {
		fmt.Printf("📝 Content Transformer: Enhancing content\n")
		fmt.Printf("  Processing: %s\n", item.Type)
		
		var prompt string
		switch item.Type {
		case "structured_text":
			prompt = "ปรับปรุงและเสริมรายละเอียดให้เนื้อหานี้ดีขึ้น"
		case "formatted_question":
			prompt = "เปลี่ยนคำถามนี้ให้ชัดเจนและครอบคลุมมากขึ้น"
		case "code_block":
			prompt = "เพิ่มคอมเมนต์และอธิบายโค้ดนี้"
		default:
			prompt = "ปรับปรุงเนื้อหานี้ให้ดีขึ้น"
		}

		systemPrompt := fmt.Sprintf(`คุณเป็นผู้เชี่ยวชาญในการปรับปรุงเนื้อหา: %s`, prompt)

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(item.Content),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return item, fmt.Errorf("content transformer generation failed: %w", err)
		}

		enhanced := DataItem{
			ID:       item.ID,
			Content:  response.Content,
			Type:     item.Type,
			Metadata: item.Metadata,
		}

		if enhanced.Metadata == nil {
			enhanced.Metadata = make(map[string]interface{})
		}
		enhanced.Metadata["content_enhanced"] = true
		enhanced.Metadata["enhancement_type"] = prompt

		fmt.Printf("  Content enhanced\n")
		return enhanced, nil
	})

	// Metadata Transformer - เปลี่ยน metadata
	metadataTransformer := compose.InvokableLambda(func(ctx context.Context, item DataItem) (DataItem, error) {
		fmt.Printf("🏷️ Metadata Transformer: Enriching metadata\n")
		
		enriched := item
		if enriched.Metadata == nil {
			enriched.Metadata = make(map[string]interface{})
		}

		// Add computed metadata
		enriched.Metadata["word_count"] = len(strings.Fields(item.Content))
		enriched.Metadata["character_count"] = len(item.Content)
		enriched.Metadata["line_count"] = len(strings.Split(item.Content, "\n"))
		enriched.Metadata["processing_pipeline"] = "format->content->metadata"
		enriched.Metadata["final_processed_at"] = time.Now().Format("2006-01-02 15:04:05")

		// Add complexity score
		complexity := "simple"
		if len(item.Content) > 200 {
			complexity = "medium"
		}
		if len(item.Content) > 500 {
			complexity = "complex"
		}
		enriched.Metadata["complexity"] = complexity

		fmt.Printf("  Added metadata: word_count=%d, complexity=%s\n", 
			enriched.Metadata["word_count"], complexity)
		
		return enriched, nil
	})

	// Result Builder - สร้างผลลัพธ์สุดท้าย
	resultBuilder := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (ProcessingResult, error) {
		fmt.Printf("🎯 Result Builder: Building final result\n")
		
		original, _ := input["original"].(DataItem)
		processed, _ := input["processed"].(DataItem)
		
		operations := []string{
			"format_transformation",
			"content_enhancement", 
			"metadata_enrichment",
		}

		result := ProcessingResult{
			Original:   original,
			Processed:  processed,
			Operations: operations,
			Metadata: map[string]interface{}{
				"pipeline_completed_at": time.Now().Format("2006-01-02 15:04:05"),
				"total_operations":      len(operations),
				"success":              true,
			},
		}

		fmt.Printf("  Built result with %d operations\n", len(operations))
		return result, nil
	})

	// Pipeline coordinator
	pipelineCoordinator := compose.InvokableLambda(func(ctx context.Context, original DataItem) (map[string]interface{}, error) {
		// Run through transformers
		step1, err := formatTransformer.Invoke(ctx, original)
		if err != nil {
			return nil, err
		}

		step2, err := contentTransformer.Invoke(ctx, step1)
		if err != nil {
			return nil, err
		}

		final, err := metadataTransformer.Invoke(ctx, step2)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"original":  original,
			"processed": final,
		}, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("pipeline_coordinator", pipelineCoordinator)
	graph.AddLambdaNode("result_builder", resultBuilder)

	// เชื่อม edges
	graph.AddEdge(compose.START, "pipeline_coordinator")
	graph.AddEdge("pipeline_coordinator", "result_builder")
	graph.AddEdge("result_builder", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile transformer graph: %w", err)
	}

	// ทดสอบ
	testItems := []DataItem{
		{
			ID:      "item1",
			Content: "Hello world! This is a test message.",
			Type:    "text",
			Metadata: map[string]interface{}{
				"source": "user_input",
			},
		},
		{
			ID:      "item2",
			Content: "How does Go handle concurrency?",
			Type:    "question",
			Metadata: map[string]interface{}{
				"category": "programming",
				"language": "go",
			},
		},
		{
			ID:      "item3",
			Content: "func main() {\n    fmt.Println(\"Hello\")\n}",
			Type:    "code",
			Metadata: map[string]interface{}{
				"language": "go",
			},
		},
	}

	for i, item := range testItems {
		fmt.Printf("\n--- Transformer Test %d ---\n", i+1)
		fmt.Printf("Original: %+v\n", item)
		
		testCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		result, err := runnable.Invoke(testCtx, item)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n")
		fmt.Printf("  Operations: %v\n", result.Operations)
		fmt.Printf("  Original Type: %s -> Final Type: %s\n", result.Original.Type, result.Processed.Type)
		fmt.Printf("  Processed Content: %s\n", result.Processed.Content)
		fmt.Printf("  Final Metadata: %+v\n", result.Processed.Metadata)
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// Pipeline with Multiple Utilities
func runPipelineExample(ctx context.Context, model *openai.ChatModel) error {
	graph := compose.NewGraph[string, string]()

	// Input Preprocessor - เตรียมข้อมูลก่อนเข้าสู่ pipeline
	inputPreprocessor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("🔧 Input Preprocessor: Preparing input\n")
		
		// Clean and normalize input
		cleaned := strings.TrimSpace(input)
		cleaned = strings.ReplaceAll(cleaned, "\t", " ")
		cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
		
		// Add preprocessing metadata
		processed := fmt.Sprintf("[PREPROCESSED:%s] %s", time.Now().Format("15:04:05"), cleaned)
		
		fmt.Printf("  Cleaned and normalized input\n")
		return processed, nil
	})

	// Passthrough Monitor - ติดตาม data flow
	passthroughMonitor := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("👁️ Passthrough Monitor: Monitoring data flow\n")
		fmt.Printf("  Data size: %d chars\n", len(input))
		fmt.Printf("  Contains preprocessing tag: %t\n", strings.Contains(input, "[PREPROCESSED:"))
		
		// Just pass through, but log important metrics
		if len(input) > 500 {
			fmt.Printf("  ⚠️ Large input detected\n")
		}
		
		return input, nil
	})

	// Content Validator - ตรวจสอบเนื้อหา
	contentValidator := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("✅ Content Validator: Validating content\n")
		
		// Extract actual content (remove preprocessing tag)
		content := input
		if idx := strings.Index(input, "] "); idx != -1 {
			content = input[idx+2:]
		}
		
		// Validate content
		if len(content) < 5 {
			return "", errors.New("content too short")
		}
		
		// Check for problematic content
		problematic := []string{"script", "eval", "delete"}
		for _, word := range problematic {
			if strings.Contains(strings.ToLower(content), word) {
				fmt.Printf("  ⚠️ Potentially problematic content detected: %s\n", word)
			}
		}
		
		fmt.Printf("  Content validation passed\n")
		return input, nil
	})

	// Smart Transformer - เปลี่ยนแปลงอัจฉริยะ
	smartTransformer := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("🧠 Smart Transformer: Intelligently transforming content\n")
		
		// Extract content
		content := input
		if idx := strings.Index(input, "] "); idx != -1 {
			content = input[idx+2:]
		}
		
		// Determine transformation type based on content
		var transformationType string
		var systemPrompt string
		
		contentLower := strings.ToLower(content)
		if strings.Contains(contentLower, "question") || strings.Contains(contentLower, "?") || strings.Contains(contentLower, "คำถาม") {
			transformationType = "question_enhancement"
			systemPrompt = "คุณเป็นผู้เชี่ยวชาญในการปรับปรุงคำถาม ให้ทำให้คำถามชัดเจนและครอบคลุมมากขึ้น"
		} else if strings.Contains(contentLower, "code") || strings.Contains(contentLower, "function") || strings.Contains(contentLower, "โค้ด") {
			transformationType = "code_explanation"
			systemPrompt = "คุณเป็นผู้เชี่ยวชาญในการอธิบายโค้ด ให้อธิบายและเสริมความเข้าใจ"
		} else if len(strings.Fields(content)) > 50 {
			transformationType = "content_summarization"
			systemPrompt = "คุณเป็นผู้เชี่ยวชาญในการสรุป ให้สรุปเนื้อหาให้กระชับแต่ครบถ้วน"
		} else {
			transformationType = "content_enhancement"
			systemPrompt = "คุณเป็นผู้เชี่ยวชาญในการปรับปรุงเนื้อหา ให้ปรับปรุงให้ดีขึ้น"
		}
		
		fmt.Printf("  Transformation type: %s\n", transformationType)
		
		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(content),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("smart transformer generation failed: %w", err)
		}

		// Add transformation metadata
		result := fmt.Sprintf("[TRANSFORMED:%s:%s] %s", transformationType, time.Now().Format("15:04:05"), response.Content)
		return result, nil
	})

	// Output Formatter - จัดรูปแบบผลลัพธ์
	outputFormatter := compose.InvokableLambda(func(ctx context.Context, input string) (string, error) {
		fmt.Printf("📋 Output Formatter: Formatting final output\n")
		
		// Extract all metadata and content
		parts := strings.Split(input, "] ")
		var metadata []string
		var content string
		
		for i, part := range parts {
			if i < len(parts)-1 && strings.HasPrefix(part, "[") {
				metadata = append(metadata, part[1:])
			} else {
				if i == len(parts)-1 {
					content = part
				} else {
					content = strings.Join(parts[i:], "] ")
					break
				}
			}
		}
		
		// Format final output
		var formattedOutput strings.Builder
		formattedOutput.WriteString("🎯 PROCESSING COMPLETE\n")
		formattedOutput.WriteString("=" + strings.Repeat("=", 50) + "\n\n")
		
		if len(metadata) > 0 {
			formattedOutput.WriteString("📊 Processing Pipeline:\n")
			for i, meta := range metadata {
				formattedOutput.WriteString(fmt.Sprintf("  %d. %s\n", i+1, meta))
			}
			formattedOutput.WriteString("\n")
		}
		
		formattedOutput.WriteString("📄 Final Content:\n")
		formattedOutput.WriteString(content)
		formattedOutput.WriteString("\n\n")
		formattedOutput.WriteString("✅ Pipeline completed at: " + time.Now().Format("2006-01-02 15:04:05"))
		
		fmt.Printf("  Formatted output with %d pipeline steps\n", len(metadata))
		return formattedOutput.String(), nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("input_preprocessor", inputPreprocessor)
	graph.AddLambdaNode("passthrough_monitor", passthroughMonitor)
	graph.AddLambdaNode("content_validator", contentValidator)
	graph.AddLambdaNode("smart_transformer", smartTransformer)
	graph.AddLambdaNode("output_formatter", outputFormatter)

	// เชื่อม edges
	graph.AddEdge(compose.START, "input_preprocessor")
	graph.AddEdge("input_preprocessor", "passthrough_monitor")
	graph.AddEdge("passthrough_monitor", "content_validator")
	graph.AddEdge("content_validator", "smart_transformer")
	graph.AddEdge("smart_transformer", "output_formatter")
	graph.AddEdge("output_formatter", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile pipeline graph: %w", err)
	}

	// ทดสอบ
	testInputs := []string{
		"What is the best way to handle errors in Go?",
		"func fibonacci(n int) int { if n <= 1 { return n } return fibonacci(n-1) + fibonacci(n-2) }",
		"Machine learning is a method of data analysis that automates analytical model building. It is a branch of artificial intelligence based on the idea that systems can learn from data, identify patterns and make decisions with minimal human intervention. The traditional machine learning process involves collecting data, preparing it, choosing a model, training the model, evaluating it, and then using it to make predictions.",
		"สวัสดีครับ วันนี้เป็นยังไงบ้าง?",
	}

	for i, input := range testInputs {
		fmt.Printf("\n--- Pipeline Test %d ---\n", i+1)
		fmt.Printf("Input: %s\n", input)
		
		testCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		result, err := runnable.Invoke(testCtx, input)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("Result:\n%s\n", result)
		fmt.Println(strings.Repeat("=", 80))
	}

	return nil
}

// Helper method to simulate Invoke since it's not available
func (l *compose.Lambda[T, R]) Invoke(ctx context.Context, input T) (R, error) {
	// This is a mock implementation since the actual Invoke method isn't available
	// In a real implementation, this would be handled by the Eino framework
	var zero R
	return zero, errors.New("Invoke method not available in this context")
}