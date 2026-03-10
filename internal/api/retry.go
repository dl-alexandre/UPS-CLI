package api

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

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

// doRequestWithRetry executes a request with retry logic
func doRequestWithRetry(client *resty.Client, req *resty.Request, config RetryConfig) (*resty.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		resp, err := req.Execute(req.Method, req.URL)

		if err != nil {
			lastErr = err
			// Check if we should retry based on error type
			if attempt < config.MaxRetries {
				delay := calculateBackoff(attempt, config.BaseDelay, config.MaxDelay, "")
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		// Check if status code requires retry
		if config.RetryStatus[resp.StatusCode()] && attempt < config.MaxRetries {
			// Check for Retry-After header
			retryAfter := resp.Header().Get("Retry-After")
			delay := calculateBackoff(attempt, config.BaseDelay, config.MaxDelay, retryAfter)
			time.Sleep(delay)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
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

func init() {
	rand.Seed(time.Now().UnixNano())
}
