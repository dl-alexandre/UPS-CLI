package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TokenProvider handles OAuth2 client credentials flow for UPS API
type TokenProvider struct {
	clientID     string
	clientSecret string
	env          string
	tokenURL     string
	httpClient   *http.Client
	cacheDir     string

	// In-memory token caching
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
func NewTokenProvider(clientID, clientSecret, env, baseURL, cacheDir string) *TokenProvider {
	tokenURL := baseURL + "/security/v1/oauth/token"

	return &TokenProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		env:          env,
		tokenURL:     tokenURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		cacheDir:     cacheDir,
		mutex:        make(chan struct{}, 1),
	}
}

// GetToken returns a valid access token, fetching a new one if necessary
func (tp *TokenProvider) GetToken(ctx context.Context) (string, error) {
	tp.mutex <- struct{}{}        // Lock
	defer func() { <-tp.mutex }() // Unlock

	// Check in-memory cache first
	if tp.cachedToken != "" && time.Now().Before(tp.tokenExpiry.Add(-60*time.Second)) {
		return tp.cachedToken, nil
	}

	// Try to load from disk cache
	token, expiry, err := tp.loadTokenFromDisk()
	if err == nil && token != "" && time.Now().Before(expiry.Add(-60*time.Second)) {
		tp.cachedToken = token
		tp.tokenExpiry = expiry
		return token, nil
	}

	// Fetch new token
	newToken, newExpiry, err := tp.fetchToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch OAuth token: %w", err)
	}

	// Cache in memory
	tp.cachedToken = newToken
	tp.tokenExpiry = newExpiry

	// Cache to disk
	_ = tp.saveTokenToDisk(newToken, newExpiry)

	return newToken, nil
}

// fetchToken performs the OAuth2 client credentials flow
func (tp *TokenProvider) fetchToken(ctx context.Context) (string, time.Time, error) {
	// Create form data
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", tp.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create token request: %w", err)
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
		return "", time.Time{}, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to read token response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse token response: %w", err)
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return tokenResp.AccessToken, expiry, nil
}

// cacheKey generates a cache key based on client_id, env, and token URL
func (tp *TokenProvider) cacheKey() string {
	data := tp.clientID + ":" + tp.env + ":" + tp.tokenURL
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}

// tokenCachePath returns the path for the token cache file
func (tp *TokenProvider) tokenCachePath() string {
	if tp.cacheDir == "" {
		return ""
	}
	return filepath.Join(tp.cacheDir, "token-"+tp.cacheKey()+".json")
}

// saveTokenToDisk saves the token to disk with secure permissions (0600)
func (tp *TokenProvider) saveTokenToDisk(token string, expiry time.Time) error {
	if tp.cacheDir == "" {
		return nil
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(tp.cacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := TokenCache{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiry,
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal token cache: %w", err)
	}

	cachePath := tp.tokenCachePath()

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token cache: %w", err)
	}

	return nil
}

// loadTokenFromDisk loads a cached token from disk
func (tp *TokenProvider) loadTokenFromDisk() (string, time.Time, error) {
	if tp.cacheDir == "" {
		return "", time.Time{}, fmt.Errorf("no cache directory configured")
	}

	cachePath := tp.tokenCachePath()

	// Check if file exists
	info, err := os.Stat(cachePath)
	if err != nil {
		return "", time.Time{}, err
	}

	// Verify permissions are restrictive (owner only)
	mode := info.Mode().Perm()
	if mode != 0600 {
		// If permissions are too broad, delete the cache file for security
		_ = os.Remove(cachePath)
		return "", time.Time{}, fmt.Errorf("token cache had insecure permissions, deleted")
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return "", time.Time{}, err
	}

	var cache TokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return "", time.Time{}, err
	}

	return cache.AccessToken, cache.ExpiresAt, nil
}

// InvalidateToken clears the cached token (e.g., after a 401)
func (tp *TokenProvider) InvalidateToken() {
	tp.mutex <- struct{}{}
	defer func() { <-tp.mutex }()

	tp.cachedToken = ""
	tp.tokenExpiry = time.Time{}

	// Also remove from disk
	if tp.cacheDir != "" {
		_ = os.Remove(tp.tokenCachePath())
	}
}
