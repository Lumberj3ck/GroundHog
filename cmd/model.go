package main

// A simple program demonstrating the text area component from the Bubbles
// component library.

import (
	"context"
	"encoding/json"
	"fmt"
	"groundhog/internal/tools/calendar"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/memory"
	"golang.org/x/oauth2"
)

const gap = "\n\n"

type (
	errMsg error
)

const CONF_DIR = ".groundhog"
const CREDS_FILE = "oauth_creds.json"

type AiResponseMsg string
type ErrorMsg string
type TokenMsg oauth2.Token

type authenticator struct {
	ts        oauth2.TokenSource
	config    oauth2.Config
	credsFile string
}

func NewAuthenticator(config oauth2.Config, credsFile string) authenticator {
	return authenticator{
		config:    config,
		credsFile: credsFile,
	}
}

func (a authenticator) LoggedIn() bool {
	ts, err := a.LoadToken()

	if err != nil {
		slog.Error("Error while loading token source: ", "error", err)
		if a.credsFile == "" {
			return false
		}
	}

	t, err := ts.Token()
	slog.Info("LoggedIn check", "err_on_token", err, "token", t.AccessToken)
	if err != nil && a.credsFile == "" {
		return false
	}

	return true

}

func (a authenticator) LoadToken() (oauth2.TokenSource, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("Error finding home dir: %v", err)
	}
	fp := filepath.Join(home, CONF_DIR, CREDS_FILE)

	file, err := os.Open(fp)

	if err != nil {
		return nil, fmt.Errorf("Error openning file: %v", err)
	}

	var token oauth2.Token
	decoder := json.NewDecoder(file)
	decoder.Decode(&token)

	ts := a.config.TokenSource(context.Background(), &token)
	return ts, nil
}

func (a authenticator) SaveToken(token *oauth2.Token) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Error finding home dir: %v", err)
	}

	dir := filepath.Join(home, CONF_DIR)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %v", err)
	}

	fp := filepath.Join(dir, CREDS_FILE)
	file, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("Error openning file: %v", err)
	}

	encoder := json.NewEncoder(file)
	err = encoder.Encode(token)

	if err != nil {
		return fmt.Errorf("Error encoding auth token: %v", strings.ToLower(err.Error()))
	}
	return nil
}

type model struct {
	viewport      viewport.Model
	textarea      textarea.Model
	executor      *agents.Executor
	authenticator authenticator
	msgChan       chan *oauth2.Token
	messages      []string
	senderStyle   lipgloss.Style
	err           error
}

func initialModel(executor *agents.Executor, msgChan chan *oauth2.Token, oauthConfig oauth2.Config, credsFile string) model {
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
	a := NewAuthenticator(oauthConfig, credsFile)
	ts, err := a.LoadToken()
	if err != nil {
		slog.Info("Error while loading ts: ", "error", err)
	} else {
		a.ts = ts
	}

	return model{
		textarea:      ta,
		messages:      []string{},
		executor:      executor,
		authenticator: a,
		msgChan:       msgChan,
		viewport:      vp,
		senderStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		err:           nil,
	}
}

func receiveOauthToken(m model) tea.Cmd {
	return func() tea.Msg {
		log.Println("Waiting for token")
		token := <-m.msgChan
		ts := oauth2.StaticTokenSource(token)
		m.authenticator.ts = ts
		log.Println(m.authenticator.ts.Token())

		err := m.authenticator.SaveToken(token)
		if err != nil {
			log.Printf("Error saving token %v \n", err)
		}
		log.Println("Received token ", token)
		return TokenMsg(*token)
	}
}

func (m model) Init() tea.Cmd {
	var authCmd tea.Cmd
	if !m.authenticator.LoggedIn() {
		exec.Command("xdg-open", "http://localhost:8080/oauth/login/").Start()
		authCmd = receiveOauthToken(m)
	}

	m.executor.Memory = memory.NewConversationBuffer()
	return tea.Batch(textarea.Blink, authCmd)
}

func handleUserMsgCmd(msg string, m model) tea.Cmd {
	return func() tea.Msg {
		slog.Info("User: ", "msg", msg)
		ctx := context.WithValue(context.Background(), calendar.ContextAuthKey, m.authenticator.ts)
		output, err := chains.Call(ctx, m.executor, map[string]any{
			"input": msg,
		})

		if err != nil {
			slog.Info("Received an error: ", "error", err)
			return ErrorMsg(err.Error())
		}

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
		tiCmd    tea.Cmd
		vpCmd    tea.Cmd
		huMsgCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case TokenMsg:
		ts := oauth2.StaticTokenSource((*oauth2.Token)(&msg))
		m.authenticator.ts = ts
	case AiResponseMsg:
		m = m.handleAddMessage(string(msg), "AI")
	case ErrorMsg:
		m = m.handleAddMessage(string(msg), "Error from agent")
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
			slog.Debug(m.textarea.Value())
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
