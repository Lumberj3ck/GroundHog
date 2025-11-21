package main

import (
	"context"
	"fmt"
	"groundhog/internal/agent"
	"groundhog/internal/patterns"
	"log"

	// "github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/prompts"
	"github.com/tmc/langchaingo/tools"
)

func main() {
	// Initialize LLM
	// llm, err := ollama.New(ollama.WithModel("deepseek-r1:14b"))
	llm, err := openai.New(openai.WithBaseURL("https://api.groq.com/openai/v1"), openai.WithModel("openai/gpt-oss-20b"))

	if err != nil {
		fmt.Printf("Failed to create Ollama client: %s\n", err)
		return
	}

	// Initialize Tools
	availableTools := []tools.Tool{
		tools.Calculator{},
	}

	// Create Agent
	executor, _ := agent.NewAgent(llm, availableTools)
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

	// fmt.Println(agent_chain.Chain)

	if err != nil {
		fmt.Printf("Agent execution failed: %s\n", err)
		return
	}
	fmt.Println("Agent response:")
	fmt.Println(response)

	fmt.Println("Chain response:")

	dialogue := fmt.Sprintf("User: %s\nAgent: %s", prompt, response["output"])
	analysisPrompt := fmt.Sprintf("Analyse this chat and summarise in two sentences what we were speaking about:\n\n%s", dialogue)

	analysisChain := chains.NewLLMChain(llm, prompts.NewPromptTemplate("{{.input}}", []string{"input"}))

	res, err := chains.Predict(context.Background(), analysisChain, map[string]any{"input": analysisPrompt})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("analysis chain")
	fmt.Println(res)
}
