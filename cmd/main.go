package main

import (
	"flag"
	"fmt"
	"groundhog/internal/agent"
	"groundhog/internal/server"
	gtools "groundhog/internal/tools/calendar"
	"groundhog/internal/tools/notes"
	gtasks "groundhog/internal/tools/tasks"

	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/tools"

	tea "github.com/charmbracelet/bubbletea"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	withCredsFile := flag.String("with-creds-file", "", "filename with json creds of the service acount")

	withOauth := flag.Bool("with-creds-oauth", false, "enable oauth authentication with the app")

	serve_tui := flag.Bool("serve-tui", false, "serve web ui")

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
		notes.NewNotesPlanner(notesDir, 5),
	}
	if calendarEnabled {
		availableTools = append(
			availableTools,
			gtools.NewListEvent(*withCredsFile),
			gtools.NewAddEvent(*withCredsFile),
			gtools.NewEditEvent(*withCredsFile),
			gtasks.NewListTasks(*withCredsFile),
			gtasks.NewAddTask(*withCredsFile),
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
				"https://www.googleapis.com/auth/tasks",
			},
			Endpoint: google.Endpoint,
		}
	}

	if !*serve_tui{
		server := server.New(agentExecutor, oauthConfig)
		port := 8080
		log.Printf("Server starting on http://localhost:%d\n", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), server); err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		if len(os.Getenv("DEBUG")) > 0{
			f, err := tea.LogToFile("debug.log", "debug")
			if err != nil {
				fmt.Println("fatal:", err)
				os.Exit(1)
			}
			defer f.Close()
		}


		msgChan := make(chan *oauth2.Token, 1)
		go func(){
			oauthHandler := server.NewOauthHandler(oauthConfig, false, false, msgChan)
			http.Handle("/oauth/", oauthHandler)
			port := 8080
			log.Printf("Server starting on http://localhost:%d\n", port)
			if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
				log.Fatal("ListenAndServe: ", err)
			}
		}()

		p := tea.NewProgram(initialModel(agentExecutor, msgChan))
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	}
}
