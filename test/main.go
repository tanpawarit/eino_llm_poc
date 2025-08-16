package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"eino_llm_poc/src/storage"

	"github.com/joho/godotenv"
)

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Example session data structure
type ExampleSessionData struct {
	UserID       string         `json:"user_id"`
	Messages     []Message      `json:"messages"`
	LastActivity time.Time      `json:"last_activity"`
	Metadata     map[string]any `json:"metadata"`
}

func demoRedisSession() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	ctx := context.Background()

	// Create Redis storage instance with type parameter
	redis, err := storage.NewRedisStorage[ExampleSessionData](ctx)
	if err != nil {
		log.Fatalf("Failed to create Redis storage: %v", err)
	}
	defer redis.Close()

	// Test connection
	err = redis.Ping(ctx)
	if err != nil {
		log.Fatalf("Redis ping failed: %v", err)
	}
	fmt.Println("✅ Connected to Redis successfully")

	// Example session data
	sessionID := "user123_session"
	sessionData := ExampleSessionData{
		UserID: "user123",
		Messages: []Message{
			{Role: "user", Content: "สวัสดีครับ"},
			{Role: "assistant", Content: "สวัสดีครับ! มีอะไรให้ช่วยเหลือไหมครับ?"},
		},
		LastActivity: time.Now(),
		Metadata: map[string]any{
			"language": "th",
			"channel":  "web",
		},
	}

	// Store session with default 60-minute TTL
	fmt.Println("📝 Storing session data...")
	err = redis.SetSession(ctx, sessionID, sessionData)
	if err != nil {
		log.Fatalf("Failed to store session: %v", err)
	}

	// Check if session exists
	exists, err := redis.SessionExists(sessionID)
	if err != nil {
		log.Fatalf("Failed to check session existence: %v", err)
	}
	fmt.Printf("🔍 Session exists: %v\n", exists)

	// Get TTL
	ttl, err := redis.GetTTL(ctx, sessionID)
	if err != nil {
		log.Fatalf("Failed to get TTL: %v", err)
	}
	fmt.Printf("⏰ Session TTL: %v\n", ttl)

	// Retrieve session data
	fmt.Println("📖 Retrieving session data...")
	retrievedData, err := redis.GetSessionData(sessionID)
	if err != nil {
		log.Fatalf("Failed to retrieve session: %v", err)
	}

	fmt.Printf("👤 Retrieved UserID: %s\n", retrievedData.UserID)
	fmt.Printf("💬 Messages count: %d\n", len(retrievedData.Messages))
	fmt.Printf("🌐 Language: %v\n", retrievedData.Metadata["language"])

	// Extend TTL to another 60 minutes
	fmt.Println("🔄 Extending session TTL...")
	err = redis.ExtendTTL(ctx, sessionID, storage.SessionTTL)
	if err != nil {
		log.Fatalf("Failed to extend TTL: %v", err)
	}

	// Get new TTL
	newTTL, err := redis.GetTTL(ctx, sessionID)
	if err != nil {
		log.Fatalf("Failed to get new TTL: %v", err)
	}
	fmt.Printf("⏰ Extended TTL: %v\n", newTTL)

	// Clean up - delete session
	fmt.Println("🧹 Cleaning up session...")
	err = redis.DeleteSession(sessionID)
	if err != nil {
		log.Fatalf("Failed to delete session: %v", err)
	}

	// Verify deletion
	exists, err = redis.SessionExists(sessionID)
	if err != nil {
		log.Fatalf("Failed to check session existence after deletion: %v", err)
	}
	fmt.Printf("🔍 Session exists after deletion: %v\n", exists)

	fmt.Println("✅ Redis session demo completed successfully!")
}

func main() {
	fmt.Println("🚀 Starting Redis Session Demo")
	demoRedisSession()
}
