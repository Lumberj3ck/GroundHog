package main

import (
	"context"
	"encoding/json"
	"fmt"
	"groundhog/internal/agent"
	"groundhog/internal/patterns"
	customTools "groundhog/internal/tools"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
)

// WebSocketMessage defines the structure for incoming JSON messages from the frontend.
type WebSocketMessage struct {
	Message string `json:"message"`
	Pattern string `json:"pattern"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request, executor *agents.Executor) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()

	log.Println("Client connected")

	var toolDescs string
	var toolNames string
	for _, tool := range executor.Agent.(*agents.OneShotZeroAgent).Tools {
		toolDescs += fmt.Sprintf("%s: %s\n", tool.Name(), tool.Description())
		toolNames += tool.Name() + ", "
	}
	toolNames = toolNames[:len(toolNames)-2] // Remove trailing comma and space

	for {
		// Read message from browser
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			log.Println("Client disconnected:", err)
			break
		}

		// Unmarshal the JSON message
		var msg WebSocketMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Println("Invalid JSON message:", err)
			continue
		}

		// Look up the pattern text
		patternText, ok := patterns.AllPatterns[msg.Pattern]
		if !ok {
			log.Println("Invalid pattern received:", msg.Pattern)
			patternText = "Please act on the following request."
		}

		// Combine the pattern and message into a single, clear instruction for the agent.
		var userInput string
		if msg.Message != "" {
			userInput = fmt.Sprintf("%s\n\nMy specific focus for this request is: \"%s\"", patternText, msg.Message)
		} else {
			userInput = patternText
		}

		fmt.Println(userInput)


		output, err := executor.Call(context.Background(), map[string]any{
			"input": msg.Message,
		})
		response := output["output"].(string)
		log.Println(output)

		if err != nil {
			log.Printf("Agent Error: %v\n", err)
			// Also log the full response map if available, it might contain partial data
			log.Printf("Full response on error: %+v\n", response)

			if writeErr := ws.WriteMessage(websocket.TextMessage, []byte("Sorry, I encountered an error.")); writeErr != nil {
				log.Println("Write error:", writeErr)
			}
			continue
		}

		ws.WriteMessage(websocket.TextMessage, []byte(response))
		log.Printf("Agent Response: %+v\n", response)
	}
}

func handlePatterns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	patternNames := make([]string, 0, len(patterns.AllPatterns))
	for name := range patterns.AllPatterns {
		patternNames = append(patternNames, name)
	}
	if err := json.NewEncoder(w).Encode(patternNames); err != nil {
		log.Println("Failed to encode patterns:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func main() {
	llm, err := openai.New(
		openai.WithBaseURL("https://api.groq.com/openai/v1"),
		openai.WithModel("openai/gpt-oss-20b"),
	)
	if err != nil {
		log.Fatal("Failed to initialize LLM:", err)
	}

	availableTools := []tools.Tool{
		customTools.NotesReader{},
		tools.Calculator{},
	}

	agentExecutor, _ := agent.NewAgent(llm, availableTools)

	// --- HTTP Server Setup ---
	// Serve the HTML file
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// API to get patterns
	http.HandleFunc("/patterns", handlePatterns)

	// Websocket route
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, agentExecutor)
	})

	port := 8080
	log.Printf("Server starting on http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
