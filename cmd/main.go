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
	"github.com/tmc/langchaingo/memory"
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

func handleConnections(w http.ResponseWriter, r *http.Request, agent *agents.Executor) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()

	log.Println("Client connected")

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

		// Construct the prompt for the agent
		prompt := fmt.Sprintf(
			"Pattern: \"%s\"\nUser Request: \"%s\"\n\nUse your tools if necessary to answer the request.",
			patternText,
			msg.Message,
		)

		// Call the agent
		response, err := agent.Call(context.Background(), map[string]any{
			"input": prompt,
		})
		if err != nil {
			log.Println("Agent Error:", err)
			if writeErr := ws.WriteMessage(websocket.TextMessage, []byte("Sorry, I encountered an error.")); writeErr != nil {
				log.Println("Write error:", writeErr)
			}
			continue
		}

		// Send response back to browser
		if err := ws.WriteMessage(websocket.TextMessage, []byte(response["output"].(string))); err != nil {
			log.Println("Write error:", err)
			break
		}
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
	// Initialize LLM
	llm, err := openai.New(
		openai.WithBaseURL("https://api.groq.com/openai/v1"),
		openai.WithModel("openai/gpt-oss-20b"),
	)
	if err != nil {
		log.Fatal("Failed to initialize LLM:", err)
	}

	// Initialize Tools
	availableTools := []tools.Tool{
		customTools.NotesReader{},
		tools.Calculator{},
	}

	// Create memory for the agent
	mem := memory.NewConversationBuffer()

	// Create the agent executor
	agentExecutor, _ := agent.NewAgent(llm, availableTools)
	agentExecutor.Memory = mem // Attach memory to the executor

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
