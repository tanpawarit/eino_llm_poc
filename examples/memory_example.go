package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// ConversationMemory - Simple in-memory conversation storage
type ConversationMemory struct {
	messages    []*schema.Message
	maxMessages int
}

func NewConversationMemory(maxMessages int) *ConversationMemory {
	return &ConversationMemory{
		messages:    make([]*schema.Message, 0),
		maxMessages: maxMessages,
	}
}

func (m *ConversationMemory) AddMessage(message *schema.Message) {
	m.messages = append(m.messages, message)
	
	// ถ้าเกินขีดจำกัด ให้ลบข้อความเก่า (แต่เก็บ system message ไว้)
	if len(m.messages) > m.maxMessages {
		// หา system message
		systemMessages := make([]*schema.Message, 0)
		otherMessages := make([]*schema.Message, 0)
		
		for _, msg := range m.messages {
			if msg.Role == "system" {
				systemMessages = append(systemMessages, msg)
			} else {
				otherMessages = append(otherMessages, msg)
			}
		}
		
		// เก็บเฉพาะข้อความล่าสุด
		keepCount := m.maxMessages - len(systemMessages)
		if len(otherMessages) > keepCount {
			otherMessages = otherMessages[len(otherMessages)-keepCount:]
		}
		
		// รวมกัน
		m.messages = make([]*schema.Message, 0)
		m.messages = append(m.messages, systemMessages...)
		m.messages = append(m.messages, otherMessages...)
	}
}

func (m *ConversationMemory) GetMessages() []*schema.Message {
	return m.messages
}

func (m *ConversationMemory) Clear() {
	// เก็บเฉพาะ system messages
	systemMessages := make([]*schema.Message, 0)
	for _, msg := range m.messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		}
	}
	m.messages = systemMessages
}

// SessionManager - จัดการหลาย conversation sessions
type SessionManager struct {
	sessions map[string]*ConversationMemory
	model    *openai.ChatModel
	ctx      context.Context
}

func NewSessionManager(model *openai.ChatModel, ctx context.Context) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ConversationMemory),
		model:    model,
		ctx:      ctx,
	}
}

func (sm *SessionManager) GetOrCreateSession(sessionID string, systemPrompt string) *ConversationMemory {
	if session, exists := sm.sessions[sessionID]; exists {
		return session
	}
	
	// สร้าง session ใหม่
	session := NewConversationMemory(20) // จำได้สูงสุด 20 ข้อความ
	if systemPrompt != "" {
		session.AddMessage(schema.SystemMessage(systemPrompt))
	}
	
	sm.sessions[sessionID] = session
	return session
}

func (sm *SessionManager) Chat(sessionID string, userMessage string) (string, error) {
	session := sm.sessions[sessionID]
	if session == nil {
		return "", fmt.Errorf("session %s not found", sessionID)
	}
	
	// เพิ่ม user message
	session.AddMessage(schema.UserMessage(userMessage))
	
	// เรียกใช้ model
	response, err := sm.model.Generate(sm.ctx, session.GetMessages())
	if err != nil {
		return "", err
	}
	
	// เพิ่ม assistant response
	session.AddMessage(schema.AssistantMessage(response.Content, nil))
	
	return response.Content, nil
}

func main() {
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

	// สร้าง Session Manager
	sessionManager := NewSessionManager(model, ctx)
	
	// สร้าง sessions ต่างๆ (จะสร้างเมื่อใช้งาน)
	_ = sessionManager.GetOrCreateSession("code", 
		"คุณเป็นผู้เชี่ยวชาญ Go programming ที่ช่วยเขียนและรีวิวโค้ด")
	
	_ = sessionManager.GetOrCreateSession("chat", 
		"คุณเป็น AI ผู้ช่วยที่เป็นมิตรและช่วยเหลือในเรื่องทั่วไป")

	fmt.Println("🧠 Eino Memory System Demo")
	fmt.Println("Commands:")
	fmt.Println("  /code <message> - Chat with coding assistant")
	fmt.Println("  /chat <message> - Chat with general assistant") 
	fmt.Println("  /sessions - List all sessions")
	fmt.Println("  /clear <session> - Clear session memory")
	fmt.Println("  /quit - Exit")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print(">> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		if input == "/quit" {
			break
		}
		
		parts := strings.SplitN(input, " ", 2)
		command := parts[0]
		message := ""
		if len(parts) > 1 {
			message = parts[1]
		}
		
		switch command {
		case "/code":
			if message == "" {
				fmt.Println("Usage: /code <your coding question>")
				continue
			}
			response, err := sessionManager.Chat("code", message)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("🔧 Code Assistant: %s\n\n", response)
			}
			
		case "/chat":
			if message == "" {
				fmt.Println("Usage: /chat <your message>")
				continue
			}
			response, err := sessionManager.Chat("chat", message)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("💬 Chat Assistant: %s\n\n", response)
			}
			
		case "/sessions":
			fmt.Println("Active sessions:")
			for sessionID, session := range sessionManager.sessions {
				messageCount := len(session.GetMessages())
				fmt.Printf("  - %s: %d messages\n", sessionID, messageCount)
			}
			fmt.Println()
			
		case "/clear":
			if message == "" {
				fmt.Println("Usage: /clear <session_id>")
				continue
			}
			if session, exists := sessionManager.sessions[message]; exists {
				session.Clear()
				fmt.Printf("Cleared session: %s\n", message)
			} else {
				fmt.Printf("Session not found: %s\n", message)
			}
			
		default:
			fmt.Println("Unknown command. Type /quit to exit.")
		}
	}
	
	// แสดงสถิติก่อนออก
	fmt.Println("\n📊 Session Statistics:")
	for sessionID, session := range sessionManager.sessions {
		messages := session.GetMessages()
		userCount := 0
		assistantCount := 0
		for _, msg := range messages {
			switch msg.Role {
			case "user":
				userCount++
			case "assistant":
				assistantCount++
			}
		}
		fmt.Printf("%s: %d user messages, %d assistant responses\n", 
			sessionID, userCount, assistantCount)
	}
}