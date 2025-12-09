package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"groundhog/internal/patterns"
	"groundhog/internal/tools/calendar"

	"github.com/gorilla/websocket"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/tools"

	"golang.org/x/oauth2"

	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	OauthToken *oauth2.Token `json:"token,omitempty"`
	jwt.RegisteredClaims
}

var hmacSecret = []byte(os.Getenv("JWT_SECRET"))

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

type oauthHandler struct {
	oauthConfig *oauth2.Config
}

func newOauthHandler(oauth2Config *oauth2.Config) http.Handler {
	return &oauthHandler{
		oauthConfig: oauth2Config,
	}
}

func (o *oauthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
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
	url := o.oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
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

	cookie := createTokenCookie(token, w)
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func createToken(token *oauth2.Token) (string, error) {
	claims := TokenClaims{
		OauthToken: token,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "localhost",
		},
	}

	jwt_token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return jwt_token.SignedString(hmacSecret)
}

func verifyToken(tokenStr string) (*TokenClaims, error) {
	// Parse the token, providing a key function.
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(t *jwt.Token) (any, error) {
		// Ensure the signing method is HMAC‑SHA256.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return hmacSecret, nil
	})
	if err != nil {
		return nil, err
	}

	// Validate the token and extract claims.
	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func authMiddleware(oauthConfig *oauth2.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("Auth")
		if err != nil {
			log.Println(r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		claims, err := verifyToken(cookie.Value)
		if err != nil {
			log.Println("Incorrect or expired jwt token")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var tokenSource oauth2.TokenSource
		if claims.OauthToken != nil {
			if oauthConfig != nil {
				tokenSource = oauthConfig.TokenSource(r.Context(), claims.OauthToken)
			} else {
				tokenSource = oauth2.StaticTokenSource(claims.OauthToken)
			}
		}
		if tokenSource != nil {
			r = r.WithContext(context.WithValue(r.Context(), "OauthTokenSource", tokenSource))
		}
		next(w, r)
	}
}

func createTokenCookie(token *oauth2.Token, w http.ResponseWriter) http.Cookie {
	t, err := createToken(token)
	if err != nil {
		w.Write([]byte("Couldn't create jwt token"))
		log.Println(err)
		return http.Cookie{}
	}
	cookie := http.Cookie{
		Name:     "Auth",
		Value:    t,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}
	return cookie
}

func groundhogLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			log.Println("Parse form error")
			w.Write([]byte("Wrong request form"))
			return
		}
		password, ok := r.Form["password"]
		if !ok {
			w.Write([]byte("Provide pasword field"))
			return
		}
		master_password := os.Getenv("MASTER_PASSWORD")

		if master_password == "" {
			w.Write([]byte("Initialise master password"))
			return
		}
		if subtle.ConstantTimeCompare([]byte(password[0]), []byte(master_password)) == 1 {
			cookie := createTokenCookie(nil, w)
			http.SetCookie(w, &cookie)
			http.Redirect(w, r, "/", http.StatusSeeOther)
		} else {
			w.Write([]byte("Incorrect password"))
		}
	} else {
		w.Header().Add("Content-Type", "text/html")
		w.Write([]byte(`
		<form action="/login" method="POST">
		<input name="password" placeholder="provide a password"/>
		<button type="submit">Submit</button>
		</form>
			`))
	}
}

func CallendarHandler(c *calendar.Calendar) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := c.Call(r.Context(), "")
		if err != nil {
			log.Println(err)
		}
		w.Write([]byte(resp))
	}
}

func New(agentExecutor *agents.Executor, oauthConfig *oauth2.Config) http.Handler {
	mux := http.NewServeMux()

	// API to get patterns
	var calendarTool tools.Tool
	tools := agentExecutor.Agent.GetTools()
	for _, tool := range tools {
		if tool.Name() == "calendar" {
			calendarTool = tool
		}
	}
	if calendarTool != nil {
		c, ok := calendarTool.(*calendar.Calendar)
		if !ok {
			fmt.Println("Couldn't create calendar tool")
		} else {
			mux.HandleFunc("/calendar", authMiddleware(oauthConfig, CallendarHandler(c)))
		}
	}

	// API to get patterns
	mux.HandleFunc("/login", groundhogLoginHandler)

	// API to get patterns
	mux.HandleFunc("/patterns", handlePatterns)

	// Websocket route
	mux.HandleFunc("/ws", authMiddleware(oauthConfig, func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, agentExecutor)
	}))

	if oauthConfig != nil {
		mux.Handle("/oauth/", newOauthHandler(oauthConfig))
	}

	mux.HandleFunc("/", authMiddleware(oauthConfig, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	}))

	return mux
}

func handleConnections(w http.ResponseWriter, r *http.Request, executor *agents.Executor) {
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


		output, err := chains.Call(context.Background(), executor, map[string]any{
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
		if !ok {
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
		if name == patterns.DefaultPattern {
			continue
		}
		patternNames = append(patternNames, name)
	}
	if err := json.NewEncoder(w).Encode(patternNames); err != nil {
		log.Println("Failed to encode patterns:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
