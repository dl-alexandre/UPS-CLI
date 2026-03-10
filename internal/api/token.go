package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenProvider handles OAuth2 client credentials flow for UPS API
type TokenProvider struct {
	clientID     string
	clientSecret string
	tokenURL     string
	httpClient   *http.Client

	// Token caching
	cachedToken string
	tokenExpiry time.Time
	mutex       chan struct{}
}

// TokenResponse represents the UPS OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// TokenCache represents a cached token for storage
type TokenCache struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Scope       string    `json:"scope"`
}

// NewTokenProvider creates a new OAuth token provider
func NewTokenProvider(clientID, clientSecret, baseURL string) *TokenProvider {
	tokenURL := baseURL + "/security/v1/oauth/token"

	return &TokenProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     tokenURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		mutex:        make(chan struct{}, 1),
	}
}

// GetToken returns a valid access token, fetching a new one if necessary
func (tp *TokenProvider) GetToken(ctx context.Context) (string, error) {
	tp.mutex <- struct{}{}        // Lock
	defer func() { <-tp.mutex }() // Unlock

	// Check if we have a valid cached token
	if tp.cachedToken != "" && time.Now().Before(tp.tokenExpiry.Add(-30*time.Second)) {
		return tp.cachedToken, nil
	}

	// Fetch new token
	token, err := tp.fetchToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch OAuth token: %w", err)
	}

	return token, nil
}

// fetchToken performs the OAuth2 client credentials flow
func (tp *TokenProvider) fetchToken(ctx context.Context) (string, error) {
	// Create form data
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", tp.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Add basic auth header
	auth := base64.StdEncoding.EncodeToString([]byte(tp.clientID + ":" + tp.clientSecret))
	req.Header.Set("Authorization", "Basic "+auth)

	// Execute request
	resp, err := tp.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Cache the token
	tp.cachedToken = tokenResp.AccessToken
	tp.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tokenResp.AccessToken, nil
}

// GetTokenCache returns the token in a cacheable format
func (tp *TokenProvider) GetTokenCache() *TokenCache {
	tp.mutex <- struct{}{}
	defer func() { <-tp.mutex }()

	if tp.cachedToken == "" {
		return nil
	}

	return &TokenCache{
		AccessToken: tp.cachedToken,
		TokenType:   "Bearer",
		ExpiresAt:   tp.tokenExpiry,
	}
}

// RestoreTokenCache restores a cached token
func (tp *TokenProvider) RestoreTokenCache(cache *TokenCache) error {
	tp.mutex <- struct{}{}
	defer func() { <-tp.mutex }()

	if cache == nil {
		return nil
	}

	// Only restore if not expired
	if time.Now().After(cache.ExpiresAt) {
		return nil
	}

	tp.cachedToken = cache.AccessToken
	tp.tokenExpiry = cache.ExpiresAt

	return nil
}

// InvalidateToken clears the cached token (e.g., after a 401)
func (tp *TokenProvider) InvalidateToken() {
	tp.mutex <- struct{}{}
	defer func() { <-tp.mutex }()

	tp.cachedToken = ""
	tp.tokenExpiry = time.Time{}
}
