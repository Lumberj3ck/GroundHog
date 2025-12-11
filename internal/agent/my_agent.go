package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools"
)

const scratchpadKey = "agent_scratchpad"

// parameterizedTool allows tools to expose structured parameter schemas.
type parameterizedTool interface {
	tools.Tool
	Parameters() map[string]interface{}
}

// OpenAIParametriesedFunctionsAgent wraps the OpenAIFunctionsAgent to customize tool schemas and parsing.
type OpenAIParametriesedFunctionsAgent struct {
	*agents.OpenAIFunctionsAgent
}

func (o *OpenAIParametriesedFunctionsAgent) functions() []llms.FunctionDefinition {
	res := make([]llms.FunctionDefinition, 0, len(o.Tools))
	for _, tool := range o.Tools {
		params := defaultFunctionParameters()
		if pt, ok := tool.(parameterizedTool); ok {
			if custom := pt.Parameters(); custom != nil {
				params = custom
			}
		}
		res = append(res, llms.FunctionDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  params,
		})
	}
	return res
}

func (o *OpenAIParametriesedFunctionsAgent) Plan(
	ctx context.Context,
	intermediateSteps []schema.AgentStep,
	inputs map[string]string,
	options ...chains.ChainCallOption,
) ([]schema.AgentAction, *schema.AgentFinish, error) {
	fullInputs := make(map[string]any, len(inputs))
	for key, value := range inputs {
		fullInputs[key] = value
	}
	fullInputs[scratchpadKey] = o.constructScratchPad(intermediateSteps)

	var stream func(ctx context.Context, chunk []byte) error

	if o.CallbacksHandler != nil {
		stream = func(ctx context.Context, chunk []byte) error {
			o.CallbacksHandler.HandleStreamingFunc(ctx, chunk)
			return nil
		}
	}

	prompt, err := o.Prompt.FormatPrompt(fullInputs)
	if err != nil {
		return nil, nil, err
	}

	mcList := make([]llms.MessageContent, len(prompt.Messages()))
	for i, msg := range prompt.Messages() {
		role := msg.GetType()
		text := msg.GetContent()

		var mc llms.MessageContent

		switch p := msg.(type) {
		case llms.ToolChatMessage:
			mc = llms.MessageContent{
				Role: role,
				Parts: []llms.ContentPart{llms.ToolCallResponse{
					ToolCallID: p.ID,
					Content:    p.Content,
				}},
			}

		case llms.FunctionChatMessage:
			mc = llms.MessageContent{
				Role: role,
				Parts: []llms.ContentPart{llms.ToolCallResponse{
					Name:    p.Name,
					Content: p.Content,
				}},
			}

		case llms.AIChatMessage:
			if len(p.ToolCalls) > 0 {
				toolCallParts := make([]llms.ContentPart, 0, len(p.ToolCalls))
				for _, tc := range p.ToolCalls {
					toolCallParts = append(toolCallParts, llms.ToolCall{
						ID:           tc.ID,
						Type:         tc.Type,
						FunctionCall: tc.FunctionCall,
					})
				}
				mc = llms.MessageContent{
					Role:  role,
					Parts: toolCallParts,
				}
			} else {
				mc = llms.MessageContent{
					Role:  role,
					Parts: []llms.ContentPart{llms.TextContent{Text: text}},
				}
			}
		default:
			mc = llms.MessageContent{
				Role:  role,
				Parts: []llms.ContentPart{llms.TextContent{Text: text}},
			}
		}
		mcList[i] = mc
	}

	llmOptions := []llms.CallOption{llms.WithFunctions(o.functions()), llms.WithStreamingFunc(stream)}
	llmOptions = append(llmOptions, chains.GetLLMCallOptions(options...)...)

	result, err := o.LLM.GenerateContent(ctx, mcList, llmOptions...)
	if result != nil {
		if result.Choices[0].FuncCall != nil{
		fmt.Println("Generated ouput from llm: ", result.Choices[0].FuncCall.Name)
		}
	}
	if err != nil {
		return nil, nil, err
	}

	return o.ParseOutput(result)
}

func (o *OpenAIParametriesedFunctionsAgent) ParseOutput(contentResp *llms.ContentResponse) (
	[]schema.AgentAction, *schema.AgentFinish, error,
) {
	if contentResp == nil || len(contentResp.Choices) == 0 {
		return nil, nil, fmt.Errorf("no choices in response")
	}
	choice := contentResp.Choices[0]

	if len(choice.ToolCalls) > 0 {
		actions := make([]schema.AgentAction, 0, len(choice.ToolCalls))

		for _, toolCall := range choice.ToolCalls {
			functionName := toolCall.FunctionCall.Name
			rawArgs := toolCall.FunctionCall.Arguments
			normalized := normalizeToolInput(rawArgs)

			contentMsg := ""
			if choice.Content != "" {
				contentMsg = fmt.Sprintf(" responded: %s", choice.Content)
			}

			actions = append(actions, schema.AgentAction{
				Tool:      functionName,
				ToolInput: normalized,
				Log:       fmt.Sprintf("Invoking: %s with %s%s", functionName, normalized, contentMsg),
				ToolID:    toolCall.ID,
			})
		}

		return actions, nil, nil
	}

	if choice.FuncCall != nil {
		functionCall := choice.FuncCall
		functionName := functionCall.Name
		rawArgs := functionCall.Arguments
		normalized := normalizeToolInput(rawArgs)

		contentMsg := ""
		if choice.Content != "" {
			contentMsg = fmt.Sprintf(" responded: %s", choice.Content)
		}

		return []schema.AgentAction{
			{
				Tool:      functionName,
				ToolInput: normalized,
				Log:       fmt.Sprintf("Invoking: %s with %s%s", functionName, normalized, contentMsg),
				ToolID:    "",
			},
		}, nil, nil
	}

	return nil, &schema.AgentFinish{
		ReturnValues: map[string]any{
			"output": choice.Content,
		},
		Log: choice.Content,
	}, nil
}

func (o *OpenAIParametriesedFunctionsAgent) constructScratchPad(steps []schema.AgentStep) []llms.ChatMessage {
	if len(steps) == 0 {
		return nil
	}

	messages := make([]llms.ChatMessage, 0)

	var currentToolCalls []llms.ToolCall
	var currentLog string

	for i, step := range steps {
		if i == 0 || step.Action.Log != steps[i-1].Action.Log {
			if len(currentToolCalls) > 0 {
				messages = append(messages, llms.AIChatMessage{
					Content:   currentLog,
					ToolCalls: currentToolCalls,
				})
				for j := i - len(currentToolCalls); j < i; j++ {
					messages = append(messages, llms.ToolChatMessage{
						ID:      steps[j].Action.ToolID,
						Content: steps[j].Observation,
					})
				}
				currentToolCalls = nil
			}
			currentLog = step.Action.Log
		}

		currentToolCalls = append(currentToolCalls, llms.ToolCall{
			ID:   step.Action.ToolID,
			Type: "function",
			FunctionCall: &llms.FunctionCall{
				Name:      step.Action.Tool,
				Arguments: step.Action.ToolInput,
			},
		})
	}

	if len(currentToolCalls) > 0 {
		messages = append(messages, llms.AIChatMessage{
			Content:   currentLog,
			ToolCalls: currentToolCalls,
		})
		for j := len(steps) - len(currentToolCalls); j < len(steps); j++ {
			messages = append(messages, llms.ToolChatMessage{
				ID:      steps[j].Action.ToolID,
				Content: steps[j].Observation,
			})
		}
	}

	return messages
}

func normalizeToolInput(argStr string) string {
	trimmed := strings.TrimSpace(argStr)
	if trimmed == "" {
		return ""
	}

	var toolInputMap map[string]any
	if err := json.Unmarshal([]byte(trimmed), &toolInputMap); err != nil {
		return trimmed
	}

	if arg1, ok := toolInputMap["__arg1"]; ok {
		if argVal, ok := arg1.(string); ok {
			return argVal
		}
	}

	normalized, err := json.Marshal(toolInputMap)
	if err != nil {
		return trimmed
	}

	return string(normalized)
}

func defaultFunctionParameters() map[string]any {
	return map[string]any{
		"properties": map[string]any{
			"__arg1": map[string]string{"title": "__arg1", "type": "string"},
		},
		"required": []string{"__arg1"},
		"type":     "object",
	}
}
