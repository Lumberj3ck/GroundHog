package agent

import (
	"fmt"
	"log"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/prompts"
	langchainTools "github.com/tmc/langchaingo/tools"
)

// NewAgent creates a new langchaingo agent that uses native tool calling so the
// model can invoke tools like calendar or calculator without hitting tool_choice errors.
func NewAgent(tools []langchainTools.Tool) (*agents.Executor) {
	llm, err := openai.New(
		openai.WithBaseURL("https://api.groq.com/openai/v1"),
		openai.WithModel("openai/gpt-oss-20b"),
	)
	if err != nil {
		log.Fatal("Failed to initialize LLM:", err)
	}

	extraMessages := []prompts.MessageFormatter{
		// Render history as a string to avoid executor casting chat messages.
		prompts.NewGenericMessagePromptTemplate("Chat history", "{{ .history }}", []string{"history"}),
	}

	tn := time.Now()
	today := tn.Format("Monday Jan 2, 2006")
	zone, _ := tn.Zone()

	systemMessage := fmt.Sprintf(`You are the Groundhog assistant. Today is %s, timezone is %s. Help users manage schedules and tasks using the provided tools. Default to tool use whenever information must be fetched, created, or updated instead of inventing details. Keep answers brief and actionable.  When asked to edit a calendar event, first obtain the event ID via the calendar tools (e.g., list or search) before attempting any update.`, today, zone)

	baseAgent := agents.NewOpenAIFunctionsAgent(
		llm,
		tools,
		agents.NewOpenAIOption().WithExtraMessages(extraMessages),
		agents.NewOpenAIOption().WithSystemMessage(systemMessage),
	)
	myAgent := &OpenAIParametriesedFunctionsAgent{OpenAIFunctionsAgent: baseAgent}
	return agents.NewExecutor(myAgent, agents.WithMaxIterations(10))
}
