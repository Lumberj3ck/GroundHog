package memory

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/schema"
)


type SummarisedMemory struct { 
	*memory.ConversationBuffer
	IntermediateStepsKey string
}

func NewSummarisedMemory() *SummarisedMemory{
    m := SummarisedMemory{
        ConversationBuffer: &memory.ConversationBuffer{
			ReturnMessages: false,
			InputKey:       "",
			OutputKey:      "output",
			HumanPrefix:    "Human",
			AIPrefix:       "AI",
			MemoryKey:      "history",
		},
    }
    m.ChatHistory = memory.NewChatMessageHistory()
	m.IntermediateStepsKey = "intermediateSteps"
    return &m
}

func (m *SummarisedMemory) SaveContext(
	ctx context.Context,
	inputValues map[string]any,
	outputValues map[string]any,
) error {
	userInputValue, err := memory.GetInputValue(inputValues, m.InputKey)
	if err != nil {
		return err
	}
	err = m.ChatHistory.AddUserMessage(ctx, userInputValue)
	if err != nil {
		return err
	}

	aiOutputValue, err := memory.GetInputValue(outputValues, m.OutputKey)
	if err != nil {
		return err
	}


	// c, _ := m.ChatHistory.Messages(context.Background())
	// slog.Info("Saved context into memory from shadowed method", "chat msgs", c)
	// slog.Info("Saved context into memory from shadowed method", "intermediateSteps", m.IntermediateStepsKey)

	intermediateSteps, ok := outputValues[m.IntermediateStepsKey]
	if !ok {
		return fmt.Errorf(
			"%w: %v do not contain inputKey %s",
			memory.ErrInvalidInputValues,
			inputValues,
			m.IntermediateStepsKey,
		)
	}
	agentSteps, ok := intermediateSteps.([]schema.AgentStep)
	if !ok{
		return fmt.Errorf("intermediateSteps key value %v not []schema.AgentStep", intermediateSteps)
	}

	aiMessage := llms.AIChatMessage{
		Content: aiOutputValue,
	}

	slog.Info("Saved context into memory from shadowed method", "intermediateSteps", intermediateSteps)
	observations := ""
	for _, action := range agentSteps{
		if action.Action.Tool != "calendar"{
			observations += fmt.Sprintf("Called tool <%v> with arguments <%v> ", action.Action.Tool , action.Action.ToolInput)
		}
	}

	if observations != ""{
		aiMessage.Content += "\nObservations:" + observations 
	} 

	err = m.ChatHistory.AddMessage(ctx, aiMessage)
	if err != nil {
		return err
	}
	slog.Info("Chat history ai message ", "aiMessage", aiMessage)

	return nil
}
