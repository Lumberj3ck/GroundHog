package main

import (
	"context"
	"fmt"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/llms/ollama"
	"log"
)

func main() {
	llm, err := ollama.New(ollama.WithModel("llama3.1:8b"))
	if err != nil {
		log.Fatal(err)
	}
	query := "Tell me, what model you are"


	ctx := context.Background()
	_, err = llms.GenerateFromSinglePrompt(ctx, llm, query,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Printf("chunk len=%d: %s\n", len(chunk), chunk)
			return nil
		}))
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println("Response:\n", completion)
}
