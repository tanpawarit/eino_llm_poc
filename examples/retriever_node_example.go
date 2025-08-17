package main

import (
	"context"
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

// Document represents a document in our knowledge base
type Document struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata"`
}

// MockRetriever simulates a vector database or document retriever
type MockRetriever struct {
	documents []Document
}

// NewMockRetriever creates a new mock retriever with sample documents
func NewMockRetriever() *MockRetriever {
	return &MockRetriever{
		documents: []Document{
			{
				ID:      "doc1",
				Content: "Eino Graph เป็น library สำหรับสร้าง workflow แบบ graph ใน Go ที่สามารถจัดการ node และ edge ได้อย่างยืดหยุ่น",
				Metadata: map[string]string{
					"topic":    "eino-graph",
					"language": "thai",
					"type":     "documentation",
				},
			},
			{
				ID:      "doc2",
				Content: "Goroutine เป็น lightweight thread ใน Go ที่ช่วยในการทำ concurrent programming โดยใช้คำสั่ง go func()",
				Metadata: map[string]string{
					"topic":    "goroutine",
					"language": "thai",
					"type":     "tutorial",
				},
			},
			{
				ID:      "doc3",
				Content: "Channels ใน Go เป็นวิธีการสื่อสารระหว่าง goroutines โดยใช้หลักการ 'Don't communicate by sharing memory; share memory by communicating'",
				Metadata: map[string]string{
					"topic":    "channels",
					"language": "thai",
					"type":     "tutorial",
				},
			},
			{
				ID:      "doc4",
				Content: "REST API design principles include using HTTP methods correctly, proper status codes, and following RESTful naming conventions",
				Metadata: map[string]string{
					"topic":    "rest-api",
					"language": "english",
					"type":     "documentation",
				},
			},
			{
				ID:      "doc5",
				Content: "Docker containerization allows applications to run consistently across different environments by packaging code with dependencies",
				Metadata: map[string]string{
					"topic":    "docker",
					"language": "english",
					"type":     "documentation",
				},
			},
			{
				ID:      "doc6",
				Content: "Database indexing improves query performance by creating data structures that speed up data retrieval operations",
				Metadata: map[string]string{
					"topic":    "database",
					"language": "english",
					"type":     "documentation",
				},
			},
		},
	}
}

// Retrieve searches for documents based on query
func (r *MockRetriever) Retrieve(ctx context.Context, query string, limit int) ([]Document, error) {
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}

	if limit <= 0 {
		limit = 3 // default limit
	}

	queryLower := strings.ToLower(query)
	var matches []Document

	// Simple keyword-based matching (in real implementation, this would use vector similarity)
	for _, doc := range r.documents {
		contentLower := strings.ToLower(doc.Content)
		
		// Check if query keywords are in content
		keywords := strings.Fields(queryLower)
		matchCount := 0
		for _, keyword := range keywords {
			if strings.Contains(contentLower, keyword) {
				matchCount++
			}
		}

		// If more than half of keywords match, include document
		if float64(matchCount)/float64(len(keywords)) > 0.3 {
			matches = append(matches, doc)
		}
	}

	// Limit results
	if len(matches) > limit {
		matches = matches[:limit]
	}

	fmt.Printf("🔍 Retriever: Found %d documents for query '%s'\n", len(matches), query)
	return matches, nil
}

// main function for running retriever examples
func main() {
	if err := runRetrieverExample(); err != nil {
		log.Fatalf("Error running retriever example: %v", err)
	}
}

// ตัวอย่าง Retriever Node - สำหรับ RAG (Retrieval-Augmented Generation)
func runRetrieverExample() error {
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

	// สร้าง Mock Retriever
	retriever := NewMockRetriever()

	// === ตัวอย่าง 1: Basic RAG Node ===
	fmt.Println("=== Basic RAG Node ===")
	if err := runBasicRAG(ctx, model, retriever); err != nil {
		return fmt.Errorf("basic RAG example failed: %w", err)
	}

	// === ตัวอย่าง 2: Advanced RAG with Context Filtering ===
	fmt.Println("\n=== Advanced RAG with Context Filtering ===")
	if err := runAdvancedRAG(ctx, model, retriever); err != nil {
		return fmt.Errorf("advanced RAG example failed: %w", err)
	}

	// === ตัวอย่าง 3: Multi-Step RAG ===
	fmt.Println("\n=== Multi-Step RAG ===")
	if err := runMultiStepRAG(ctx, model, retriever); err != nil {
		return fmt.Errorf("multi-step RAG example failed: %w", err)
	}

	return nil
}

// Basic RAG Node
func runBasicRAG(ctx context.Context, model *openai.ChatModel, retriever *MockRetriever) error {
	graph := compose.NewGraph[string, string]()

	// Retriever Node - ดึงข้อมูลที่เกี่ยวข้อง
	retrieverNode := compose.InvokableLambda(func(ctx context.Context, query string) ([]Document, error) {
		if strings.TrimSpace(query) == "" {
			return nil, errors.New("query cannot be empty")
		}

		fmt.Printf("🔍 Retriever Node: Searching for '%s'\n", query)
		
		documents, err := retriever.Retrieve(ctx, query, 3)
		if err != nil {
			return nil, fmt.Errorf("retrieval failed: %w", err)
		}

		// แสดงผลลัพธ์ที่พบ
		for i, doc := range documents {
			content := doc.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("  [%d] %s: %s\n", i+1, doc.ID, content)
		}

		return documents, nil
	})

	// Context Builder - รวม context จาก retrieved documents
	contextBuilder := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) ([]*schema.Message, error) {
		query, ok := input["query"].(string)
		if !ok || strings.TrimSpace(query) == "" {
			return nil, errors.New("query is required")
		}

		docs, ok := input["documents"].([]Document)
		if !ok {
			return nil, errors.New("documents are required")
		}

		// สร้าง context จาก documents
		var contextParts []string
		for _, doc := range docs {
			contextParts = append(contextParts, fmt.Sprintf("- %s", doc.Content))
		}

		context := strings.Join(contextParts, "\n")
		
		systemPrompt := fmt.Sprintf(`คุณเป็น AI ผู้ช่วยที่ตอบคำถามโดยอิงจากข้อมูล context ที่ให้มา

Context:
%s

ตอบคำถามโดยอิงจาก context ที่ให้มา ถ้าไม่มีข้อมูลใน context ให้บอกว่าไม่มีข้อมูลเพียงพอ`, context)

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(query),
		}

		fmt.Printf("📝 Context Builder: Built context with %d documents\n", len(docs))
		return messages, nil
	})

	// Input Combiner - รวม query และ documents เข้าด้วยกัน
	inputCombiner := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		// Input should contain both query and documents from previous nodes
		return input, nil
	})

	// Chat Model Node
	chatModelNode := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		if len(messages) == 0 {
			return "", errors.New("messages cannot be empty")
		}

		fmt.Printf("🤖 RAG Chat Model: Generating response with context\n")
		
		llamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		response, err := model.Generate(llamCtx, messages)
		if err != nil {
			return "", fmt.Errorf("chat model generation failed: %w", err)
		}

		if response == nil {
			return "", errors.New("received nil response from chat model")
		}

		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("retriever", retrieverNode)
	graph.AddLambdaNode("context_builder", contextBuilder)
	graph.AddLambdaNode("chat", chatModelNode)

	// สร้าง custom node สำหรับรวม input
	graph.AddLambdaNode("prepare_input", compose.InvokableLambda(func(ctx context.Context, query string) (map[string]interface{}, error) {
		// Retrieve documents first
		docs, err := retriever.Retrieve(ctx, query, 3)
		if err != nil {
			return nil, err
		}
		
		return map[string]interface{}{
			"query":     query,
			"documents": docs,
		}, nil
	}))

	// เชื่อม edges
	graph.AddEdge(compose.START, "prepare_input")
	graph.AddEdge("prepare_input", "context_builder")
	graph.AddEdge("context_builder", "chat")
	graph.AddEdge("chat", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile basic RAG graph: %w", err)
	}

	// ทดสอบ
	testQueries := []string{
		"Eino Graph คืออะไร?",
		"Goroutine ใช้งานยังไง?",
		"ช่วยอธิบาย Docker",
		"วิธีการทำ REST API",
	}

	for i, query := range testQueries {
		fmt.Printf("\n--- Basic RAG Test %d ---\n", i+1)
		fmt.Printf("Query: %s\n", query)
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, query)
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

// Advanced RAG with Context Filtering
func runAdvancedRAG(ctx context.Context, model *openai.ChatModel, retriever *MockRetriever) error {
	graph := compose.NewGraph[map[string]interface{}, string]()

	// Query Analyzer - วิเคราะห์ query เพื่อปรับปรุงการค้นหา
	queryAnalyzer := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		originalQuery, ok := input["query"].(string)
		if !ok || strings.TrimSpace(originalQuery) == "" {
			return nil, errors.New("query is required")
		}

		language, _ := input["language"].(string)
		if language == "" {
			language = "auto" // auto-detect
		}

		userLevel, _ := input["user_level"].(string)
		if userLevel == "" {
			userLevel = "intermediate"
		}

		// Analyze query to extract keywords and intent
		queryLower := strings.ToLower(originalQuery)
		var enhancedKeywords []string
		var queryType string

		// Detect query type
		if strings.Contains(queryLower, "คือ") || strings.Contains(queryLower, "what") || strings.Contains(queryLower, "อะไร") {
			queryType = "definition"
		} else if strings.Contains(queryLower, "ยังไง") || strings.Contains(queryLower, "how") || strings.Contains(queryLower, "วิธี") {
			queryType = "how-to"
		} else if strings.Contains(queryLower, "ทำไม") || strings.Contains(queryLower, "why") || strings.Contains(queryLower, "เพราะ") {
			queryType = "explanation"
		} else {
			queryType = "general"
		}

		// Extract and enhance keywords
		words := strings.Fields(originalQuery)
		for _, word := range words {
			word = strings.ToLower(strings.Trim(word, "?.,!"))
			if len(word) > 2 && !isStopWord(word) {
				enhancedKeywords = append(enhancedKeywords, word)
			}
		}

		enhancedQuery := strings.Join(enhancedKeywords, " ")

		fmt.Printf("🔬 Query Analyzer: Type='%s', Enhanced='%s'\n", queryType, enhancedQuery)

		return map[string]interface{}{
			"original_query":    originalQuery,
			"enhanced_query":    enhancedQuery,
			"query_type":       queryType,
			"language":         language,
			"user_level":       userLevel,
			"keywords":         enhancedKeywords,
		}, nil
	})

	// Advanced Retriever - ใช้ enhanced query และ filtering
	advancedRetriever := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) ([]Document, error) {
		enhancedQuery, _ := input["enhanced_query"].(string)
		language, _ := input["language"].(string)
		queryType, _ := input["query_type"].(string)

		fmt.Printf("🔍 Advanced Retriever: Searching with filters\n")
		
		// ค้นหาด้วย enhanced query
		allDocs, err := retriever.Retrieve(ctx, enhancedQuery, 6) // ขอมากกว่าแล้วค่อย filter
		if err != nil {
			return nil, fmt.Errorf("retrieval failed: %w", err)
		}

		// Filter documents based on language and type
		var filteredDocs []Document
		for _, doc := range allDocs {
			// Language filtering
			if language != "auto" && language != "" {
				if docLang, exists := doc.Metadata["language"]; exists && docLang != language {
					continue
				}
			}

			// Type-based scoring (prefer certain types for certain queries)
			include := true
			switch queryType {
			case "definition":
				// Prefer documentation
				if docType, exists := doc.Metadata["type"]; exists && docType != "documentation" {
					// Still include but with lower priority
				}
			case "how-to":
				// Prefer tutorials
				if docType, exists := doc.Metadata["type"]; exists && docType != "tutorial" {
					// Still include but with lower priority
				}
			}

			if include {
				filteredDocs = append(filteredDocs, doc)
			}
		}

		// Limit to top 3
		if len(filteredDocs) > 3 {
			filteredDocs = filteredDocs[:3]
		}

		fmt.Printf("  Filtered to %d documents (from %d)\n", len(filteredDocs), len(allDocs))
		return filteredDocs, nil
	})

	// Smart Context Builder - สร้าง context ที่ปรับตาม user level
	smartContextBuilder := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) ([]*schema.Message, error) {
		originalQuery, _ := input["original_query"].(string)
		docs, ok := input["documents"].([]Document)
		if !ok {
			return nil, errors.New("documents are required")
		}
		userLevel, _ := input["user_level"].(string)
		queryType, _ := input["query_type"].(string)

		// สร้าง context ตาม user level
		var contextIntro string
		switch userLevel {
		case "beginner":
			contextIntro = "ข้อมูลพื้นฐานที่เกี่ยวข้อง:"
		case "intermediate":
			contextIntro = "ข้อมูลเทคนิคที่เกี่ยวข้อง:"
		case "expert":
			contextIntro = "ข้อมูลเชิงลึกที่เกี่ยวข้อง:"
		default:
			contextIntro = "ข้อมูลที่เกี่ยวข้อง:"
		}

		var contextParts []string
		for i, doc := range docs {
			contextParts = append(contextParts, fmt.Sprintf("%d. %s", i+1, doc.Content))
		}

		context := fmt.Sprintf("%s\n%s", contextIntro, strings.Join(contextParts, "\n"))

		// ปรับ system prompt ตาม query type และ user level
		var systemPrompt string
		switch queryType {
		case "definition":
			systemPrompt = fmt.Sprintf(`คุณเป็นผู้เชี่ยวชาญที่อธิบายความหมายและแนวคิดให้เข้าใจง่าย

%s

ตอบคำถามโดยให้คำจำกัดความที่ชัดเจน พร้อมตัวอย่างประกอบ`, context)
		case "how-to":
			systemPrompt = fmt.Sprintf(`คุณเป็นครูสอนที่เก่งในการอธิบายขั้นตอนการทำงาน

%s

ตอบคำถามโดยแบ่งเป็นขั้นตอนที่ชัดเจน พร้อมคำอธิบาย`, context)
		case "explanation":
			systemPrompt = fmt.Sprintf(`คุณเป็นผู้เชี่ยวชาญที่อธิบายเหตุผลและที่มาที่ไป

%s

ตอบคำถามโดยอธิบายเหตุผล สาเหตุ และความสัมพันธ์`, context)
		default:
			systemPrompt = fmt.Sprintf(`คุณเป็น AI ผู้ช่วยที่มีความรู้กว้างขวาง

%s

ตอบคำถามอย่างครอบคลุมตามข้อมูลที่ให้มา`, context)
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(originalQuery),
		}

		fmt.Printf("🧠 Smart Context: Built %s prompt for %s level\n", queryType, userLevel)
		return messages, nil
	})

	// Data Flow Node - จัดการ data flow ระหว่าง nodes
	dataFlowNode := compose.InvokableLambda(func(ctx context.Context, analyzed map[string]interface{}) (map[string]interface{}, error) {
		// Retrieve documents using analyzed query
		docs, err := retriever.Retrieve(ctx, analyzed["enhanced_query"].(string), 6)
		if err != nil {
			return nil, err
		}

		// Add documents to the analyzed data
		result := make(map[string]interface{})
		for k, v := range analyzed {
			result[k] = v
		}
		result["documents"] = docs

		return result, nil
	})

	// Chat Model Node
	chatModelNode := compose.InvokableLambda(func(ctx context.Context, messages []*schema.Message) (string, error) {
		fmt.Printf("🤖 Advanced RAG Chat: Generating contextual response\n")
		
		llamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		response, err := model.Generate(llamCtx, messages)
		if err != nil {
			return "", fmt.Errorf("chat model generation failed: %w", err)
		}

		if response == nil {
			return "", errors.New("received nil response from chat model")
		}

		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("query_analyzer", queryAnalyzer)
	graph.AddLambdaNode("data_flow", dataFlowNode)
	graph.AddLambdaNode("context_builder", smartContextBuilder)
	graph.AddLambdaNode("chat", chatModelNode)

	// เชื่อม edges
	graph.AddEdge(compose.START, "query_analyzer")
	graph.AddEdge("query_analyzer", "data_flow")
	graph.AddEdge("data_flow", "context_builder")
	graph.AddEdge("context_builder", "chat")
	graph.AddEdge("chat", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile advanced RAG graph: %w", err)
	}

	// ทดสอบ
	testCases := []map[string]interface{}{
		{
			"query":      "Eino Graph คืออะไร?",
			"language":   "thai",
			"user_level": "beginner",
		},
		{
			"query":      "Goroutine ใช้งานยังไง?",
			"language":   "thai", 
			"user_level": "intermediate",
		},
		{
			"query":      "ทำไม Docker ถึงสำคัญ?",
			"language":   "auto",
			"user_level": "expert",
		},
		{
			"query":      "How to design REST API?",
			"language":   "english",
			"user_level": "intermediate",
		},
	}

	for i, testCase := range testCases {
		fmt.Printf("\n--- Advanced RAG Test %d ---\n", i+1)
		fmt.Printf("Query: %s (Level: %s, Lang: %s)\n", testCase["query"], testCase["user_level"], testCase["language"])
		
		testCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := runnable.Invoke(testCtx, testCase)
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

// Multi-Step RAG
func runMultiStepRAG(ctx context.Context, model *openai.ChatModel, retriever *MockRetriever) error {
	graph := compose.NewGraph[string, string]()

	// Step 1: Query Understanding
	queryUnderstanding := compose.InvokableLambda(func(ctx context.Context, query string) (map[string]interface{}, error) {
		fmt.Printf("📝 Step 1: Understanding query '%s'\n", query)
		
		// Use LLM to understand and decompose query
		messages := []*schema.Message{
			schema.SystemMessage(`คุณเป็นผู้เชี่ยวชาญในการวิเคราะห์คำถาม 
โปรดวิเคราะห์คำถามและระบุ:
1. ประเภทของคำถาม (definition, how-to, comparison, troubleshooting)
2. หัวข้อหลัก
3. คำค้นหาที่เหมาะสม
4. ระดับความซับซ้อน

ตอบในรูปแบบ JSON:
{
  "type": "ประเภทคำถาม",
  "main_topic": "หัวข้อหลัก", 
  "search_terms": ["คำค้นหา1", "คำค้นหา2"],
  "complexity": "basic|intermediate|advanced"
}`),
			schema.UserMessage(query),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("query understanding failed: %w", err)
		}

		fmt.Printf("  Analysis: %s\n", response.Content)

		return map[string]interface{}{
			"original_query": query,
			"analysis":      response.Content,
			"step":          1,
		}, nil
	})

	// Step 2: Knowledge Retrieval
	knowledgeRetrieval := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		originalQuery := input["original_query"].(string)
		
		fmt.Printf("🔍 Step 2: Retrieving knowledge\n")
		
		// Retrieve based on original query
		docs, err := retriever.Retrieve(ctx, originalQuery, 4)
		if err != nil {
			return nil, fmt.Errorf("knowledge retrieval failed: %w", err)
		}

		// If not enough docs, try with individual keywords
		if len(docs) < 2 {
			fmt.Printf("  Not enough docs, trying keyword search...\n")
			words := strings.Fields(strings.ToLower(originalQuery))
			for _, word := range words {
				if len(word) > 3 && !isStopWord(word) {
					moreDocs, err := retriever.Retrieve(ctx, word, 2)
					if err == nil {
						docs = append(docs, moreDocs...)
					}
				}
			}
		}

		// Remove duplicates
		uniqueDocs := make(map[string]Document)
		for _, doc := range docs {
			uniqueDocs[doc.ID] = doc
		}

		finalDocs := make([]Document, 0, len(uniqueDocs))
		for _, doc := range uniqueDocs {
			finalDocs = append(finalDocs, doc)
		}

		fmt.Printf("  Retrieved %d unique documents\n", len(finalDocs))

		result := make(map[string]interface{})
		for k, v := range input {
			result[k] = v
		}
		result["documents"] = finalDocs
		result["step"] = 2

		return result, nil
	})

	// Step 3: Context Synthesis
	contextSynthesis := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		docs := input["documents"].([]Document)
		originalQuery := input["original_query"].(string)
		analysis := input["analysis"].(string)
		
		fmt.Printf("🧠 Step 3: Synthesizing context\n")
		
		// Use LLM to synthesize retrieved information
		var docContents []string
		for i, doc := range docs {
			docContents = append(docContents, fmt.Sprintf("Document %d: %s", i+1, doc.Content))
		}

		combinedDocs := strings.Join(docContents, "\n\n")

		messages := []*schema.Message{
			schema.SystemMessage(`คุณเป็นผู้เชี่ยวชาญในการรวบรวมและสังเคราะห์ข้อมูล
โปรดรวบรวมข้อมูลจากเอกสารที่ให้มาและสร้างบริบทที่เหมาะสมสำหรับการตอบคำถาม

เอกสาร:
` + combinedDocs + `

การวิเคราะห์คำถาม:
` + analysis + `

โปรดสร้างบริบทที่:
1. รวมข้อมูลที่เกี่ยวข้องจากทุกเอกสาร
2. จัดลำดับความสำคัญ
3. เตรียมพร้อมสำหรับการตอบคำถาม`),
			schema.UserMessage(fmt.Sprintf("สังเคราะห์ข้อมูลสำหรับคำถาม: %s", originalQuery)),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("context synthesis failed: %w", err)
		}

		fmt.Printf("  Synthesized context (%d chars)\n", len(response.Content))

		result := make(map[string]interface{})
		for k, v := range input {
			result[k] = v
		}
		result["synthesized_context"] = response.Content
		result["step"] = 3

		return result, nil
	})

	// Step 4: Final Answer Generation
	answerGeneration := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (string, error) {
		originalQuery := input["original_query"].(string)
		synthesizedContext := input["synthesized_context"].(string)
		
		fmt.Printf("✨ Step 4: Generating final answer\n")
		
		messages := []*schema.Message{
			schema.SystemMessage(fmt.Sprintf(`คุณเป็น AI ผู้ช่วยที่ตอบคำถามอย่างครอบคลุมและแม่นยำ

บริบทที่สังเคราะห์แล้ว:
%s

โปรดตอบคำถามโดย:
1. ใช้ข้อมูลจากบริบทเป็นหลัก
2. ให้คำตอบที่ชัดเจนและเป็นระบบ
3. เพิ่มตัวอย่างหรือรายละเอียดที่เป็นประโยชน์
4. หากข้อมูลไม่เพียงพอ ให้บอกอย่างซื่อสัตย์`, synthesizedContext)),
			schema.UserMessage(originalQuery),
		}

		response, err := model.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("answer generation failed: %w", err)
		}

		fmt.Printf("  Generated final answer (%d chars)\n", len(response.Content))
		return response.Content, nil
	})

	// เพิ่ม nodes
	graph.AddLambdaNode("query_understanding", queryUnderstanding)
	graph.AddLambdaNode("knowledge_retrieval", knowledgeRetrieval)
	graph.AddLambdaNode("context_synthesis", contextSynthesis)
	graph.AddLambdaNode("answer_generation", answerGeneration)

	// เชื่อม edges
	graph.AddEdge(compose.START, "query_understanding")
	graph.AddEdge("query_understanding", "knowledge_retrieval")
	graph.AddEdge("knowledge_retrieval", "context_synthesis")
	graph.AddEdge("context_synthesis", "answer_generation")
	graph.AddEdge("answer_generation", compose.END)

	// Compile
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("failed to compile multi-step RAG graph: %w", err)
	}

	// ทดสอบ
	testQueries := []string{
		"เปรียบเทียบ Goroutine กับ Thread ปกติ",
		"วิธีการใช้ Eino Graph ร่วมกับ Docker",
		"ปัญหาที่พบบ่อยใน REST API และวิธีแก้ไข",
	}

	for i, query := range testQueries {
		fmt.Printf("\n--- Multi-Step RAG Test %d ---\n", i+1)
		fmt.Printf("Query: %s\n", query)
		
		testCtx, cancel := context.WithTimeout(ctx, 120*time.Second) // More time for multi-step
		result, err := runnable.Invoke(testCtx, query)
		cancel()
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("\nFinal Answer:\n%s\n", result)
		fmt.Println(strings.Repeat("=", 80))
	}

	return nil
}

// Helper function to check stop words
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"คือ": true, "เป็น": true, "และ": true, "หรือ": true, "ที่": true,
		"ใน": true, "กับ": true, "โดย": true, "ให้": true, "ได้": true,
		"มี": true, "ไม่": true, "แล้ว": true, "จะ": true, "ก็": true,
		"the": true, "is": true, "and": true, "or": true, "in": true,
		"to": true, "of": true, "for": true, "with": true, "by": true,
		"a": true, "an": true, "as": true, "at": true, "be": true,
	}
	return stopWords[word]
}