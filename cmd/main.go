package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request, conversation chains.Chain) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()

	log.Println("Client connected")

	for {
		// Read message from browser
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println("Client disconnected:", err)
			break
		}

		// Call the chain with the user's message
		response, err := chains.Predict(context.Background(), conversation, map[string]any{
			"input": string(msg),
		})
		if err != nil {
			log.Println("LLM Error:", err)
			// Send error message back to client
			if writeErr := ws.WriteMessage(websocket.TextMessage, []byte("Sorry, I encountered an error.")); writeErr != nil {
				log.Println("Write error:", writeErr)
			}
			continue
		}

		// Write message back to browser
		if err := ws.WriteMessage(websocket.TextMessage, []byte(response)); err != nil {
			log.Println("Write error:", err)
			break
		}
	}
}

func main() {
	// Initialize LLM (replace with your details if necessary)
	llm, err := openai.New(
		openai.WithBaseURL("https://api.groq.com/openai/v1"),
		openai.WithModel("openai/gpt-oss-20b"),
	)
	if err != nil {
		log.Fatal("Failed to initialize LLM:", err)
	}

	// Create a new conversational chain with memory
	mem := memory.NewConversationBuffer()
	conversation := chains.NewConversation(llm, mem)

	// Serve the HTML file
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Configure websocket route
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, conversation)
	})

	// Start the server on localhost:8080
	port := 8080
	log.Printf("Server starting on http://localhost:%d\n", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
