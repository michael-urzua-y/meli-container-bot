package meli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TokenResponse representa la respuesta de OAuth de MercadoLibre
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	UserID       int    `json:"user_id"`
	RefreshToken string `json:"refresh_token"`
}

const tokenFile = ".meli_token.json"

// GetAuthURL genera la URL para que el usuario autorice la app
func GetAuthURL() string {
	appID := os.Getenv("MELI_APP_ID")
	redirectURI := os.Getenv("MELI_REDIRECT_URI")
	return fmt.Sprintf(
		"https://auth.mercadolibre.cl/authorization?response_type=code&client_id=%s&redirect_uri=%s",
		appID, url.QueryEscape(redirectURI),
	)
}

// ExchangeCode intercambia el código de autorización por un access token
func ExchangeCode(code string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {os.Getenv("MELI_APP_ID")},
		"client_secret": {os.Getenv("MELI_SECRET_KEY")},
		"code":          {code},
		"redirect_uri":  {os.Getenv("MELI_REDIRECT_URI")},
	}

	return postToken(data)
}

// RefreshAccessToken renueva el token usando el refresh token
func RefreshAccessToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {os.Getenv("MELI_APP_ID")},
		"client_secret": {os.Getenv("MELI_SECRET_KEY")},
		"refresh_token": {refreshToken},
	}

	return postToken(data)
}

// LoadToken carga el token guardado en disco
func LoadToken() (*TokenResponse, error) {
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}
	var token TokenResponse
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// SaveToken guarda el token en disco
func SaveToken(token *TokenResponse) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenFile, data, 0600)
}

// GetValidToken obtiene un token válido, renovándolo si es necesario
func GetValidToken() (*TokenResponse, error) {
	token, err := LoadToken()
	if err != nil {
		return nil, fmt.Errorf("no hay token guardado, ejecuta primero: go run cmd/auth/main.go")
	}

	// Intentar renovar el token
	newToken, err := RefreshAccessToken(token.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("error renovando token: %w (re-autoriza con: go run cmd/auth/main.go)", err)
	}

	if err := SaveToken(newToken); err != nil {
		return nil, err
	}

	return newToken, nil
}

func postToken(data url.Values) (*TokenResponse, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(
		"https://api.mercadolibre.com/oauth/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error OAuth (%d): %s", resp.StatusCode, string(body))
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}
