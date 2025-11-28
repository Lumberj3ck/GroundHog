package main

import (
	"context"
	"encoding/json"
	"fmt"
	"groundhog/internal/agent"
	"groundhog/internal/notes"
	"groundhog/internal/patterns"
	"log"
	"net/http"
	"os"

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

func handleConnections(w http.ResponseWriter, r *http.Request, notesDir string, executor *agents.Executor) {
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

		var userInput string
		if msg.Message != "" {
			userInput = fmt.Sprintf("%s\n\nMy specific focus for this request is: \"%s\"", patternText, msg.Message)
		} else {
			userInput = patternText
		}

		n, err := notes.GetLastNotes(notesDir, 5)
		if err != nil{
			ws.WriteMessage(websocket.TextMessage, []byte("Couldn't get last notes"))
		}

		userInput += "\nNotes content: \n" + notes.PromptFormatNotes(n)

		fmt.Println(userInput)
		output, err := executor.Call(context.Background(), map[string]any{
			"input": userInput,
		})

		if err != nil {
			log.Printf("Agent Error: %v\n", err)
			log.Printf("Full response on error: %+v\n", output)

			if writeErr := ws.WriteMessage(websocket.TextMessage, []byte("Sorry, I encountered an error.")); writeErr != nil {
				log.Println("Write error:", writeErr)
			}
			continue
		}
		llmOut := output["output"]
		response, ok := llmOut.(string)
		if !ok{
			log.Println("Couldn't get proper output from llm")
		}
		ws.WriteMessage(websocket.TextMessage, []byte(response))
	}
}

func handlePatterns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	patternNames := make([]string, 0, len(patterns.AllPatterns))
	patternNames = append(patternNames, patterns.DefaultPattern) 

	for name := range patterns.AllPatterns {
		if name == patterns.DefaultPattern{
			continue
		}
		patternNames = append(patternNames, name)
	}
	if err := json.NewEncoder(w).Encode(patternNames); err != nil {
		log.Println("Failed to encode patterns:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func main() {
	notesDir := os.Getenv("NOTES_DIR")

	if notesDir == "" {
		log.Fatalf("Please, provide NOTES_DIR environmnet variable")
	}

	llm, err := openai.New(
		openai.WithBaseURL("https://api.groq.com/openai/v1"),
		openai.WithModel("openai/gpt-oss-20b"),
	)
	if err != nil {
		log.Fatal("Failed to initialize LLM:", err)
	}

	availableTools := []tools.Tool{
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
		handleConnections(w, r, notesDir, agentExecutor)
	})

	port := 8080
	log.Printf("Server starting on http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
