package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"groundhog/internal/notes"
	"groundhog/internal/patterns"

	"github.com/gorilla/websocket"
	"github.com/tmc/langchaingo/agents"

    "golang.org/x/oauth2"
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

type oauthHandler struct{
	oauthConfig *oauth2.Config
}

func newOauthHandler(oauth2Config *oauth2.Config) oauthHandler{
	return oauthHandler{
		oauthConfig: oauth2Config,
	}
}

func (o oauthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request){
	switch r.URL.Path{
	case "/oauth/login/":
		o.handleLogin(w, r)	
	case "/oauth/oauth2callback/":
		o.handleCallback(w, r)	
	default:
		http.NotFound(w, r)
	}
}

func (o *oauthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	// State can be a random string to protect against CSRF
	url := o.oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (o *oauthHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}
	token, err := o.oauthConfig.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Token exchange error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Use token to call Google APIs or store it for later use
	fmt.Fprintf(w, "Access Token: %s\nRefresh Token: %s", token.AccessToken, token.RefreshToken)
	// db save for token 
	// tools.Call(o.oauthConfig.TokenSource(context.Background(), token))
}

func New(notesDir string, agentExecutor *agents.Executor, oauthConfig *oauth2.Config) http.Handler {
	mux := http.NewServeMux()

	// API to get patterns
	mux.HandleFunc("/patterns", handlePatterns)

	// Websocket route
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, notesDir, agentExecutor)
	})

	if oauthConfig != nil{
		mux.Handle("/oauth/", newOauthHandler(oauthConfig))
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	return mux
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

