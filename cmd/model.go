package main

// A simple program demonstrating the text area component from the Bubbles
// component library.

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/memory"
)

const gap = "\n\n"

type (
	errMsg error
)

type model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	executor 	*agents.Executor
	messages    []string
	senderStyle lipgloss.Style
	err         error
}

func initialModel(executor *agents.Executor) model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 280

	ta.SetWidth(30)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	ta.ShowLineNumbers = false

	vp := viewport.New(30, 5)
	vp.SetContent(`Welcome to the chat room!
Type a message and press Enter to send.`)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		textarea:    ta,
		messages:    []string{},
		executor: 	 executor, 
		viewport:    vp,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		err:         nil,
	}
}

func (m model) Init() tea.Cmd {
	m.executor.Memory = memory.NewConversationBuffer() 
	return textarea.Blink
}

type AiResponseMsg string

func handleUserMsgCmd(msg string, m model) tea.Cmd {
	return func() tea.Msg {
		log.Println(msg)
		output, _ := chains.Call(context.TODO(), m.executor, map[string]any{
			"input": msg,
		})
		llmOut := output["output"]
		response, ok := llmOut.(string)

		if !ok {
			log.Println("Couldn't get proper output from llm")
		}
		log.Println(response)

		return AiResponseMsg(response)
	}
}

func (m model) handleAddMessage(msg string, role string) model {
	m.messages = append(m.messages, m.senderStyle.Render(fmt.Sprintf("%s : ", role))+msg)
	m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
	m.textarea.Reset()
	m.viewport.GotoBottom()
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		huMsgCmd tea.Cmd	
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case AiResponseMsg:
		m = m.handleAddMessage(string(msg), "AI")

	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)

		if len(m.messages) > 0 {
			// Wrap content before setting it.
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		}
		m.viewport.GotoBottom()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println(m.textarea.Value())
			return m, tea.Quit
		case tea.KeyEnter:
			tm := m.textarea.Value()
			m = m.handleAddMessage(tm, "You")
			huMsgCmd = handleUserMsgCmd(tm, m)
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd, huMsgCmd)
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s%s%s",
		m.viewport.View(),
		gap,
		m.textarea.View(),
	)
}

