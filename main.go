package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

var (
	// Configure your OAuth2 settings here
	oauth2Config = &oauth2.Config{
		ClientID:     "",
		ClientSecret: "",
		RedirectURL:  "http://localhost:8080/callback",
		Scopes:       []string{"files:read"}, // Specify the scopes you need
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.figma.com/oauth",
			TokenURL: "https://www.figma.com/api/oauth/token",
		},
	}
	// Random string for state verification (should be more complex in production)
	oauthStateString = "random"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Panicf("Error loading .env file: %s", err)
	}

	oauth2Config.ClientID = os.Getenv("CLIENT_ID")
	oauth2Config.ClientSecret = os.Getenv("CLIENT_SECRET")

	http.HandleFunc("/callback", handleCallback)
	fmt.Println("Started running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	if state != oauthStateString {
		fmt.Printf("invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	token, err := oauth2Config.Exchange(context.Background(), code)
	if err != nil {
		fmt.Printf("oauthConf.Exchange() failed with '%s'\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	fmt.Fprintf(w, "Got token: %v\n", token)

	fmt.Println("Token: ", token)
}
