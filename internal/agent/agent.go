package agent

import (
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	langchainTools "github.com/tmc/langchaingo/tools"
)

// NewAgent creates a new langchaingo agent that uses native tool calling so the
// model can invoke tools like calendar or calculator without hitting tool_choice errors.
func NewAgent(llm llms.Model, tools []langchainTools.Tool) (*agents.Executor, *agents.OpenAIFunctionsAgent) {
	agent := agents.NewOpenAIFunctionsAgent(llm, tools)

	return agents.NewExecutor(agent, agents.WithMaxIterations(5)), agent
}
