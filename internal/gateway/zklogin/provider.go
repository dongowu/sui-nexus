package zklogin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthProvider defines the interface for OAuth-based identity providers.
type OAuthProvider interface {
	// GetAuthURL returns the OAuth authorization URL with the given state.
	GetAuthURL(state string) string
	// ExchangeCode exchanges an authorization code for an ID token (JWT).
	ExchangeCode(ctx context.Context, code string) (string, error)
	// GetUserInfo extracts user info from the ID token.
	GetUserInfo(ctx context.Context, jwt string) (*UserInfo, error)
}

// UserInfo represents the user information from an OAuth provider.
type UserInfo struct {
	Address   string
	Subject   string // sub claim from JWT
	Email     string
	MaxEpoch  int64
}

// GoogleOAuthProvider implements OAuthProvider for Google.
type GoogleOAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	State        string // OAuth state for CSRF protection
}

// NewGoogleOAuthProvider creates a new Google OAuth provider.
func NewGoogleOAuthProvider(clientID, clientSecret, redirectURL string) *GoogleOAuthProvider {
	return &GoogleOAuthProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	}
}

// GetAuthURL returns the Google OAuth authorization URL.
func (p *GoogleOAuthProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", p.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "openid email")
	params.Set("state", state)
	params.Set("nonce", generateNonce())

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for an ID token.
func (p *GoogleOAuthProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("redirect_uri", p.RedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("oauth error: %s", tokenResp.Error)
	}

	return tokenResp.IDToken, nil
}

// GetUserInfo extracts user info from a Google ID token.
func (p *GoogleOAuthProvider) GetUserInfo(ctx context.Context, jwt string) (*UserInfo, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (second part)
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Extract required fields
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	aud, _ := claims["aud"].(string)

	// Verify audience matches our client ID
	if aud != p.ClientID {
		return nil, fmt.Errorf("invalid audience in JWT")
	}

	// Address is derived client-side via @mysten/zklogin (Poseidon + Blake2b).
	// The server does NOT compute the address — the client submits it with the proof.
	return &UserInfo{
		Address:  "", // filled client-side after proof generation
		Subject:  sub,
		Email:    email,
		MaxEpoch: 30,
	}, nil
}

// TwitchOAuthProvider implements OAuthProvider for Twitch.
type TwitchOAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// NewTwitchOAuthProvider creates a new Twitch OAuth provider.
func NewTwitchOAuthProvider(clientID, clientSecret, redirectURL string) *TwitchOAuthProvider {
	return &TwitchOAuthProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	}
}

// GetAuthURL returns the Twitch OAuth authorization URL.
func (p *TwitchOAuthProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", p.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "openid user:email")
	params.Set("state", state)
	params.Set("nonce", generateNonce())

	return "https://id.twitch.tv/oauth2/authorize?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for a token.
func (p *TwitchOAuthProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("redirect_uri", p.RedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://id.twitch.tv/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("oauth error: %s", tokenResp.Error)
	}

	return tokenResp.IDToken, nil
}

// GetUserInfo extracts user info from a Twitch ID token.
func (p *TwitchOAuthProvider) GetUserInfo(ctx context.Context, jwt string) (*UserInfo, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	return &UserInfo{
		Address:  "", // derived client-side via @mysten/zklogin
		Subject:  sub,
		Email:    email,
		MaxEpoch: 30,
	}, nil
}

// generateNonce generates a random nonce for OAuth security.
func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

