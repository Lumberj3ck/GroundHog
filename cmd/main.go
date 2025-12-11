package main

import (
	"flag"
	"fmt"
	"groundhog/internal/agent"
	"groundhog/internal/notes"
	"groundhog/internal/server"
	gtools "groundhog/internal/tools/calendar"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/tools"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	withCredsFile := flag.String("with-creds-file", "", "filename with json creds of the service acount")
	withOauth := flag.Bool("with-creds-oauth", false, "enable oauth authentication with the app")
	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	notesDir := os.Getenv("NOTES_DIR")

	if notesDir == "" {
		log.Fatalf("Please, provide NOTES_DIR environmnet variable")
	}


	calendarEnabled := *withCredsFile != "" || *withOauth
	availableTools := []tools.Tool{
		tools.Calculator{},
		notes.NewTool(notesDir, 5),
	}
	if calendarEnabled {
		availableTools = append(
			availableTools,
			gtools.New(*withCredsFile),
			gtools.NewAddEvent(*withCredsFile),
			gtools.NewEditEvent(*withCredsFile),
		)
	}

	agentExecutor := agent.NewAgent(availableTools)

	var oauthConfig *oauth2.Config
	if *withOauth {
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
		oauthConfig = &oauth2.Config{
			ClientID:     googleClientId,
			ClientSecret: googleSecret,
			RedirectURL:  googleRedirectUrl,
			Scopes: []string{
				"https://www.googleapis.com/auth/calendar",
			},
			Endpoint: google.Endpoint,
		}
	}

	server := server.New(agentExecutor, oauthConfig)
	port := 8080
	log.Printf("Server starting on http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), server); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
