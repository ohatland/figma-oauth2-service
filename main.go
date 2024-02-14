package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

type UserToken struct {
	AccessToken string
	ReadKey     string // used to find the stored token in memory
	WriteKey    string // used as state value for OAuth2
}

type GeneratedKeyPairResponse struct {
	ReadKey  string `json:"readKey"`
	WriteKey string `json:"writeKey"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"accessToken"`
}

var (
	// Configure your OAuth2 settings here
	oauth2Config = &oauth2.Config{
		ClientID:     "",
		ClientSecret: "",
		RedirectURL:  "http://localhost:8080/callback",
		Scopes:       []string{"file_variables:read"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.figma.com/oauth",
			TokenURL: "https://www.figma.com/api/oauth/token",
		},
	}

	userTokenList []UserToken

	letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Panicf("Error loading .env file: %s", err)
	}

	oauth2Config.ClientID = os.Getenv("CLIENT_ID")
	oauth2Config.ClientSecret = os.Getenv("CLIENT_SECRET")

	/*
		Order of operations:
		1. User generates keypair with /keypair
		2. User logs in with /login?writeKey=writeKey
		3. User gets token with /token?readKey=readKey
		4. User can now use the token to make requests to Figma's API

		Note: The keypair is stored in memory and will be lost if the server is restarted
	*/

	http.HandleFunc("/keypair", handleKeypair)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/token", handleToken)
	http.HandleFunc("/callback", handleCallback)
	fmt.Println("Started running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// create keypair for user, store it in memory and return it to the user
func handleKeypair(w http.ResponseWriter, r *http.Request) {

	userToken := generateKeyPair()

	response := GeneratedKeyPairResponse{
		ReadKey:  userToken.ReadKey,
		WriteKey: userToken.WriteKey,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// redirects user to Figma's OAuth2 login page
// user needs to already have a keypair
func handleLogin(w http.ResponseWriter, r *http.Request) {
	writeKey := r.FormValue("writeKey")
	if !checkIfWriteKeyExistsInMemory(writeKey) {
		fmt.Fprintln(w, "Invalid write key")
		return
	}
	url := oauth2Config.AuthCodeURL(writeKey)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// user retrive token for the stored keypair
func handleToken(w http.ResponseWriter, r *http.Request) {
	readKey := r.FormValue("readKey")

	accessToken, err := findAccessTokenInMemory(readKey)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	response := AccessTokenResponse{
		AccessToken: accessToken,
	}

	// TODO handle error
	removeAccessTokenFromMemory(accessToken)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

// retrive token from Figmas OAuth2 and store it in memory
func handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	code := r.FormValue("code")

	token, err := oauth2Config.Exchange(context.Background(), code)
	if err != nil {
		fmt.Printf("oauthConf.Exchange() failed with '%s'\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	if err := storeUserTokenInMemory(state, token); err != nil {
		fmt.Printf("%s\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	fmt.Fprintln(w, "Got token")
}

func generateKeyPair() UserToken {
	userToken := UserToken{
		AccessToken: "",
		ReadKey:     generateRandomString(64),
		WriteKey:    generateRandomString(64),
	}
	userTokenList = append(userTokenList, userToken)
	return userToken
}

func storeUserTokenInMemory(writeKey string, token *oauth2.Token) error {
	keyStored := false
	for i, userToken := range userTokenList {
		if userToken.WriteKey == writeKey {
			userTokenList[i].AccessToken = token.AccessToken
			keyStored = true
			return nil
		}
	}
	if !keyStored {
		return errors.New("could not find state value for received token")
	}

	return nil
}

// find oauth2 token in memory with readKey
func findAccessTokenInMemory(readKey string) (string, error) {
	for _, userToken := range userTokenList {
		if userToken.ReadKey == readKey {
			if userToken.AccessToken == "" {
				// when key pair is generated, and the access token is not yet created
				return "", errors.New("could not find AccessToken for given readKey")
			}
			return userToken.AccessToken, nil
		}
	}

	// when key pair does not exist
	return "", errors.New("could not find AccessToken for given readKey")
}

func checkIfWriteKeyExistsInMemory(writeKey string) bool {
	for _, userToken := range userTokenList {
		if userToken.WriteKey == writeKey {
			return true
		}
	}
	return false
}

func removeAccessTokenFromMemory(accessToken string) error {
	indexToRemove := -1
	for i, userToken := range userTokenList {
		if userToken.AccessToken == accessToken {
			indexToRemove = i
			break
		}
	}

	userTokenList = append(userTokenList[:indexToRemove], userTokenList[indexToRemove+1:]...)

	if indexToRemove == -1 {
		return errors.New("could not find token in memory")
	}
	return nil
}

// generateRandomString returns a random string of n characters
func generateRandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
