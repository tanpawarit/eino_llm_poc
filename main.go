package main

import (
	"context"
	"eino_llm_poc/src"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

type RunState struct {
	History []*schema.Message
	Logs    []string
}

func genState(_ context.Context) *RunState {
	return &RunState{
		History: make([]*schema.Message, 0, 10),
		Logs:    make([]string, 0, 10),
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	config, err := src.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("env Config: %+v\n", apiKey)
	fmt.Printf("env Config: %+v\n", baseURL)
	fmt.Printf("NLU Config: %+v\n", config.NLUConfig)
}
