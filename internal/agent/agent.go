package agent

import (
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	langchainTools "github.com/tmc/langchaingo/tools"
)

// NewAgent creates a new langchain agent.
func NewAgent(llm llms.Model, tools []langchainTools.Tool) (*agents.Executor, *agents.OneShotZeroAgent) {
	agent := agents.NewOneShotAgent(llm, tools)

	return agents.NewExecutor(agent, agents.WithMaxIterations(5)), agent
}
