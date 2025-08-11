package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

func main() {
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

	// === ตัวอย่างการใช้ Schema Messages ===

	fmt.Println("=== 1. Basic Messages ===")
	basicMessages := []*schema.Message{
		schema.SystemMessage("คุณเป็น AI ที่ช่วยสอนการเขียนโปรแกรม Go"),
		schema.UserMessage("อธิบาย goroutines ให้ฟังหน่อย"),
	}
	
	response1, err := model.Generate(ctx, basicMessages)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Response: %s\n\n", response1.Content)

	fmt.Println("=== 2. Multi-turn Conversation ===")
	conversation := []*schema.Message{
		schema.SystemMessage("คุณเป็นผู้เชี่ยวชาญ Go programming"),
		schema.UserMessage("channel ใน Go ใช้ทำอะไร?"),
		// สมมติว่าได้ response แล้ว
		schema.AssistantMessage("Channel ใน Go ใช้สำหรับการสื่อสารระหว่าง goroutines อย่างปลอดภัย...", nil),
		schema.UserMessage("ยกตัวอย่างการใช้งาน channel ให้ดูหน่อย"),
	}
	
	response2, err := model.Generate(ctx, conversation)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Response: %s\n\n", response2.Content)

	fmt.Println("=== 3. Message Properties ===")
	// ดูข้อมูลของ message
	msg := schema.UserMessage("test message")
	fmt.Printf("Message Content: %s\n", msg.Content)
	fmt.Printf("Message Role: %s\n", msg.Role)
	
	// ตัวอย่าง message types ต่างๆ
	systemMsg := schema.SystemMessage("System prompt")
	userMsg := schema.UserMessage("User input")
	assistantMsg := schema.AssistantMessage("Assistant response", nil)
	
	fmt.Printf("System Message Role: %s\n", systemMsg.Role)
	fmt.Printf("User Message Role: %s\n", userMsg.Role)
	fmt.Printf("Assistant Message Role: %s\n", assistantMsg.Role)
}