package api

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

// RequestBuilder is a function that builds a fresh request for each retry attempt
type RequestBuilder func() *resty.Request

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	RetryStatus map[int]bool // Status codes to retry
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		RetryStatus: map[int]bool{
			http.StatusTooManyRequests:     true, // 429
			http.StatusInternalServerError: true, // 500
			http.StatusBadGateway:          true, // 502
			http.StatusServiceUnavailable:  true, // 503
			http.StatusGatewayTimeout:      true, // 504
		},
	}
}

// NoRetryConfig returns a config that never retries (for ship operations)
func NoRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  0,
		BaseDelay:   0,
		MaxDelay:    0,
		RetryStatus: map[int]bool{},
	}
}

// SafeRetryConfig returns config that only retries on 429 (for semi-idempotent operations)
func SafeRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 2,
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		RetryStatus: map[int]bool{
			http.StatusTooManyRequests: true, // 429 only
		},
	}
}

// DoWithRetry executes a request with retry logic
// The builder is called fresh for each attempt to avoid body reuse issues
func DoWithRetry(ctx context.Context, buildRequest RequestBuilder, config RetryConfig) (*resty.Response, error) {
	var lastErr error
	var lastResp *resty.Response

	// Generate a single transId for all attempts of this logical request
	transId := generateTransactionID()

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Build a fresh request for each attempt
		req := buildRequest()

		// Add consistent transaction ID for all attempts
		req.SetHeader("transId", transId)
		req.SetHeader("transactionSrc", "ups-cli")

		// Set context
		req.SetContext(ctx)

		resp, err := req.Execute(req.Method, req.URL)

		if err != nil {
			lastErr = err
			// Check if we should retry based on error type
			if attempt < config.MaxRetries {
				delay := calculateBackoff(attempt, config.BaseDelay, config.MaxDelay, "")
				time.Sleep(delay)
				continue
			}
			return nil, fmt.Errorf("request failed after %d attempts: %w", attempt+1, err)
		}

		lastResp = resp

		// Check if status code requires retry
		if config.RetryStatus[resp.StatusCode()] && attempt < config.MaxRetries {
			// Check for Retry-After header
			retryAfter := resp.Header().Get("Retry-After")
			delay := calculateBackoff(attempt, config.BaseDelay, config.MaxDelay, retryAfter)
			time.Sleep(delay)
			continue
		}

		// Success or non-retryable status code
		return resp, nil
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, fmt.Errorf("max retries exceeded (%d attempts): %w", config.MaxRetries+1, lastErr)
	}

	if lastResp != nil {
		return lastResp, nil
	}

	return nil, fmt.Errorf("max retries exceeded (%d attempts)", config.MaxRetries+1)
}

// calculateBackoff calculates delay with exponential backoff
func calculateBackoff(attempt int, baseDelay, maxDelay time.Duration, retryAfter string) time.Duration {
	// Check for Retry-After header first
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			return time.Duration(seconds) * time.Second
		}
		// Try parsing as HTTP date
		if t, err := http.ParseTime(retryAfter); err == nil {
			delay := time.Until(t)
			if delay > 0 && delay < maxDelay {
				return delay
			}
		}
	}

	// Exponential backoff: baseDelay * 2^attempt
	backoff := float64(baseDelay) * math.Pow(2, float64(attempt))
	delay := time.Duration(backoff)

	// Add jitter (±25%)
	jitter := float64(delay) * 0.25
	delay = time.Duration(float64(delay) + (jitter * (2*rand.Float64() - 1)))

	// Cap at max delay
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// generateTransactionID creates a unique transaction ID
func generateTransactionID() string {
	return fmt.Sprintf("upscli-%d", time.Now().UnixNano())
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
