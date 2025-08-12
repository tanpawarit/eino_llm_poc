package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"eino_llm_poc/internal/storage"
	"eino_llm_poc/pkg"

	"github.com/joho/godotenv"
)

// Example session data structure
type ExampleSessionData struct {
	UserID       string                     `json:"user_id"`
	Messages     []pkg.ConversationMessage  `json:"messages"`
	LastActivity time.Time                  `json:"last_activity"`
	Metadata     map[string]any             `json:"metadata"`
}

func demoRedisSession() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	ctx := context.Background()

	// Create Redis storage instance
	redis, err := storage.NewRedisStorage(ctx)
	if err != nil {
		log.Fatalf("Failed to create Redis storage: %v", err)
	}
	defer redis.Close()

	// Test connection
	err = redis.Ping()
	if err != nil {
		log.Fatalf("Redis ping failed: %v", err)
	}
	fmt.Println("✅ Connected to Redis successfully")

	// Example session data
	sessionID := "user123_session"
	sessionData := ExampleSessionData{
		UserID: "user123",
		Messages: []pkg.ConversationMessage{
			{Role: "user", Content: "สวัสดีครับ"},
			{Role: "assistant", Content: "สวัสดีครับ! มีอะไรให้ช่วยเหลือไหมครับ?"},
		},
		LastActivity: time.Now(),
		Metadata: map[string]any{
			"language": "th",
			"channel":  "web",
		},
	}

	// Store session with default 40-minute TTL
	fmt.Println("📝 Storing session data...")
	err = redis.SetSession(sessionID, sessionData)
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
	ttl, err := redis.GetTTL(sessionID)
	if err != nil {
		log.Fatalf("Failed to get TTL: %v", err)
	}
	fmt.Printf("⏰ Session TTL: %v\n", ttl)

	// Retrieve session data
	fmt.Println("📖 Retrieving session data...")
	var retrievedData ExampleSessionData
	err = redis.GetSessionData(sessionID, &retrievedData)
	if err != nil {
		log.Fatalf("Failed to retrieve session: %v", err)
	}

	fmt.Printf("👤 Retrieved UserID: %s\n", retrievedData.UserID)
	fmt.Printf("💬 Messages count: %d\n", len(retrievedData.Messages))
	fmt.Printf("🌐 Language: %v\n", retrievedData.Metadata["language"])

	// Extend TTL to another 40 minutes
	fmt.Println("🔄 Extending session TTL...")
	err = redis.ExtendTTL(sessionID, storage.SessionTTL)
	if err != nil {
		log.Fatalf("Failed to extend TTL: %v", err)
	}

	// Get new TTL
	newTTL, err := redis.GetTTL(sessionID)
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
	demoRedisSession()
}