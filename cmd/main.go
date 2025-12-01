package main

import (
	"fmt"
	"groundhog/internal/agent"
	"groundhog/internal/server"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)



func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

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

	googleClientId := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientId == "" {
		log.Fatalf("Please, provide GOOGLE_CLIENT_ID environmnet variable")
	}
	googleSecret := os.Getenv("GOOGLE_SECRET")
	if googleSecret == "" {
		log.Fatalf("Please, provide GOOGLE_SECRET environmnet variable")
	}
	googleRedirectUrl := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleRedirectUrl == "" {
		log.Fatalf("Please, provide GOOGLE_REDIRECT_URL environmnet variable")
	}


	oauthConfig := &oauth2.Config{
		ClientID:     googleClientId,
		ClientSecret: googleSecret,
		RedirectURL: googleRedirectUrl,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	server := server.New(notesDir, agentExecutor, oauthConfig)
	port := 8080
	log.Printf("Server starting on http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), server); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
