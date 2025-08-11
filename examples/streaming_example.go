package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// StreamHandler - จัดการ streaming response
type StreamHandler struct {
	model *openai.ChatModel
	ctx   context.Context
}

func NewStreamHandler(model *openai.ChatModel, ctx context.Context) *StreamHandler {
	return &StreamHandler{
		model: model,
		ctx:   ctx,
	}
}

// StreamChat - ส่งข้อความและรับ response แบบ streaming
func (sh *StreamHandler) StreamChat(messages []*schema.Message, callback func(chunk string)) error {
	// สำหรับ Eino, เราจะใช้วิธี simulate streaming
	// โดยการแบ่ง response ออกเป็น chunks
	
	response, err := sh.model.Generate(sh.ctx, messages)
	if err != nil {
		return err
	}
	
	// แบ่ง response ออกเป็น words และส่งทีละคำ
	words := strings.Fields(response.Content)
	
	for i, word := range words {
		// ส่ง word พร้อม space (ยกเว้นคำสุดท้าย)
		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}
		
		callback(chunk)
		
		// หน่วงเวลาเล็กน้อยเพื่อให้เห็น streaming effect
		time.Sleep(50 * time.Millisecond)
	}
	
	return nil
}

// AdvancedStreamHandler - streaming handler ที่ซับซ้อนขึ้น
type AdvancedStreamHandler struct {
	model        *openai.ChatModel
	ctx          context.Context
	chunkSize    int
	streamDelay  time.Duration
}

func NewAdvancedStreamHandler(model *openai.ChatModel, ctx context.Context) *AdvancedStreamHandler {
	return &AdvancedStreamHandler{
		model:       model,
		ctx:         ctx,
		chunkSize:   3, // ส่งทีละ 3 คำ
		streamDelay: 80 * time.Millisecond,
	}
}

func (ash *AdvancedStreamHandler) StreamChatAdvanced(messages []*schema.Message, 
	onChunk func(chunk string),
	onComplete func(fullResponse string),
	onError func(err error)) {
	
	go func() {
		response, err := ash.model.Generate(ash.ctx, messages)
		if err != nil {
			onError(err)
			return
		}
		
		words := strings.Fields(response.Content)
		fullResponse := ""
		
		// ส่งเป็น chunks
		for i := 0; i < len(words); i += ash.chunkSize {
			end := i + ash.chunkSize
			if end > len(words) {
				end = len(words)
			}
			
			chunk := strings.Join(words[i:end], " ")
			if end < len(words) {
				chunk += " "
			}
			
			fullResponse += chunk
			onChunk(chunk)
			
			time.Sleep(ash.streamDelay)
		}
		
		onComplete(fullResponse)
	}()
}

// TypewriterEffect - เอฟเฟค typewriter สำหรับ console
func TypewriterEffect(text string, delay time.Duration) {
	for _, char := range text {
		fmt.Print(string(char))
		time.Sleep(delay)
	}
	fmt.Println()
}

// ProgressIndicator - แสดง loading animation
type ProgressIndicator struct {
	running bool
}

func NewProgressIndicator() *ProgressIndicator {
	return &ProgressIndicator{running: false}
}

func (p *ProgressIndicator) Start() {
	p.running = true
	go func() {
		chars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for p.running {
			fmt.Printf("\r%s กำลังคิด...", chars[i])
			i = (i + 1) % len(chars)
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Print("\r                    \r") // clear line
	}()
}

func (p *ProgressIndicator) Stop() {
	p.running = false
	time.Sleep(150 * time.Millisecond) // ให้เวลา goroutine หยุด
}

func streamingDemo() {
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

	streamHandler := NewStreamHandler(model, ctx)
	advancedHandler := NewAdvancedStreamHandler(model, ctx)
	
	fmt.Println("🔄 Eino Streaming Demo")
	fmt.Println("Commands:")
	fmt.Println("  /simple <message> - Simple streaming")
	fmt.Println("  /advanced <message> - Advanced streaming with callbacks")
	fmt.Println("  /typewriter <message> - Typewriter effect")
	fmt.Println("  /help - Show this help")
	fmt.Println("  /quit - Exit")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print(">> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		if input == "/quit" || input == "quit" {
			break
		}
		
		if input == "/help" {
			fmt.Println("Available streaming modes:")
			fmt.Println("1. Simple - Basic word-by-word streaming")
			fmt.Println("2. Advanced - Chunk-based streaming with callbacks")
			fmt.Println("3. Typewriter - Character-by-character effect")
			fmt.Println()
			continue
		}
		
		parts := strings.SplitN(input, " ", 2)
		command := parts[0]
		message := ""
		if len(parts) > 1 {
			message = parts[1]
		}
		
		if message == "" && command != "/help" {
			fmt.Println("กรุณาใส่ข้อความ เช่น: /simple สวัสดี")
			continue
		}
		
		messages := []*schema.Message{
			schema.SystemMessage("คุณเป็น AI ผู้ช่วยที่ตอบคำถามแบบเป็นมิตร"),
			schema.UserMessage(message),
		}
		
		switch command {
		case "/simple":
			fmt.Print("🤖 AI: ")
			err := streamHandler.StreamChat(messages, func(chunk string) {
				fmt.Print(chunk)
			})
			if err != nil {
				fmt.Printf("\nError: %v", err)
			}
			fmt.Println("\n")
			
		case "/advanced":
			fmt.Print("🤖 AI: ")
			
			// สร้าง progress indicator
			progress := NewProgressIndicator()
			progress.Start()
			
			// ใช้ channel เพื่อรอให้ streaming เสร็จ
			done := make(chan bool)
			
			advancedHandler.StreamChatAdvanced(messages,
				// onChunk
				func(chunk string) {
					progress.Stop() // หยุด loading animation
					fmt.Print(chunk)
				},
				// onComplete
				func(fullResponse string) {
					fmt.Println("\n✅ Streaming completed")
					fmt.Printf("📊 Total length: %d characters\n\n", len(fullResponse))
					done <- true
				},
				// onError
				func(err error) {
					progress.Stop()
					fmt.Printf("\n❌ Error: %v\n\n", err)
					done <- true
				},
			)
			
			// รอให้ streaming เสร็จ
			<-done
			
		case "/typewriter":
			// รับ response ปกติก่อน
			progress := NewProgressIndicator()
			progress.Start()
			
			response, err := model.Generate(ctx, messages)
			progress.Stop()
			
			if err != nil {
				fmt.Printf("Error: %v\n\n", err)
				continue
			}
			
			fmt.Print("🤖 AI: ")
			TypewriterEffect(response.Content, 30*time.Millisecond)
			fmt.Println()
			
		default:
			fmt.Println("Unknown command. Type /help for help.\n")
		}
	}
}

func main() {
	streamingDemo()
}