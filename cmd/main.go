package main

import (
	"context"
	"fmt"
	"groundhog/internal/agent"
	"groundhog/internal/patterns"

	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/tools"
)

func main() {
	// Initialize LLM
	llm, err := ollama.New(ollama.WithModel("deepseek-r1:14b"))
	if err != nil {
		fmt.Printf("Failed to create Ollama client: %s\n", err)
		return
	}

	// Initialize Tools
	availableTools := []tools.Tool{
	}
	// availableTools := []tools.Tool{}

	// Create Agent
	executor := agent.NewAgent(llm, availableTools)
	// executor := agent.NewAgent(llm)

	// Execute Agent
	journalNotes := `
	Journal Note 1:
	Today was a busy day. I had a meeting with the team and worked on the new feature.
	Tomorrow, I need to finish the feature and prepare for the demo.

	Journal Note 2:
	I feel a bit tired today. I will try to rest more tomorrow.
	Tomorrow, I will focus on my well-being.
	`
	prompt := patterns.AnalyseMyDay + "\n" + journalNotes

	response, err := executor.Call(context.Background(), map[string]any{"input": prompt})
	if err != nil {
		fmt.Printf("Agent execution failed: %s\n", err)
		return
	}

	fmt.Println("Agent response:")
	fmt.Println(response)
}
