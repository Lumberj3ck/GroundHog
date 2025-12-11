package agent

import (
	"fmt"
	"log"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/prompts"
	langchainTools "github.com/tmc/langchaingo/tools"
)

// NewAgent creates a new langchaingo agent that uses native tool calling so the
// model can invoke tools like calendar or calculator without hitting tool_choice errors.
func NewAgent(tools []langchainTools.Tool) (*agents.Executor, *agents.OpenAIFunctionsAgent) {
	llm, err := openai.New(
		openai.WithBaseURL("https://api.groq.com/openai/v1"),
		openai.WithModel("openai/gpt-oss-120b"),
	)
	if err != nil {
		log.Fatal("Failed to initialize LLM:", err)
	}

	extraMessages := []prompts.MessageFormatter{
		// Render history as a string to avoid executor casting chat messages.
		prompts.NewGenericMessagePromptTemplate("Chat history", "{{ .history }}", []string{"history"}),
	}

	today := time.Now().Format("Monday Jan 2, 2006")

	systemMessage := fmt.Sprintf(`You are the Groundhog assistant. Today is %s. Help users manage schedules and tasks using the provided tools. Default to tool use whenever information must be fetched, created, or updated instead of inventing details. Keep answers brief and actionable.  When asked to edit a calendar event, first obtain the event ID via the calendar tools (e.g., list or search) before attempting any update.`, today)

	agent := agents.NewOpenAIFunctionsAgent(llm, tools, agents.NewOpenAIOption().WithExtraMessages(extraMessages), agents.NewOpenAIOption().WithSystemMessage(systemMessage))

	return agents.NewExecutor(agent, agents.WithMaxIterations(10), agents.WithMemory(memory.NewConversationBuffer())), agent
}
