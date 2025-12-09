package agent

import (
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/prompts"
	langchainTools "github.com/tmc/langchaingo/tools"
)

// NewAgent creates a new langchaingo agent that uses native tool calling so the
// model can invoke tools like calendar or calculator without hitting tool_choice errors.
func NewAgent(llm llms.Model, tools []langchainTools.Tool) (*agents.Executor, *agents.OpenAIFunctionsAgent) {
	extraMessages := []prompts.MessageFormatter{
		// Render history as a string to avoid executor casting chat messages.
		prompts.NewGenericMessagePromptTemplate("Chat history", "{{ .history }}", []string{"history"}),
	}

	agent := agents.NewOpenAIFunctionsAgent(llm, tools, agents.NewOpenAIOption().WithExtraMessages(extraMessages))

	return agents.NewExecutor(agent, agents.WithMaxIterations(5), agents.WithMemory(memory.NewConversationBuffer())), agent
}
