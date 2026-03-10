package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func TestDoWithRetry_POSTBodyResent(t *testing.T) {
	requestCount := 0
	var lastBody []byte

	// Create a test server that fails twice then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Read and store the body
		body, _ := io.ReadAll(r.Body)
		lastBody = body

		if requestCount < 3 {
			// Return 500 for first 2 requests
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
			return
		}

		// Success on 3rd request
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := resty.New()
	client.SetBaseURL(server.URL)

	requestBody := map[string]string{"test": "data", "value": "123"}

	buildRequest := func() *resty.Request {
		req := client.R()
		req.Method = "POST"
		req.URL = "/test"
		req.SetBody(requestBody)
		return req
	}

	retryCfg := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond, // Fast for tests
		MaxDelay:   100 * time.Millisecond,
		RetryStatus: map[int]bool{
			http.StatusInternalServerError: true,
		},
	}

	ctx := t.Context()
	resp, err := DoWithRetry(ctx, buildRequest, retryCfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode())
	}

	if requestCount != 3 {
		t.Fatalf("expected 3 requests (2 retries), got %d", requestCount)
	}

	// Verify the body was non-empty on all attempts
	if len(lastBody) == 0 {
		t.Fatal("last request body was empty")
	}

	// Verify the body contains our data
	var receivedBody map[string]string
	if err := json.Unmarshal(lastBody, &receivedBody); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if receivedBody["test"] != "data" || receivedBody["value"] != "123" {
		t.Fatalf("body data mismatch: got %v", receivedBody)
	}
}

func TestDoWithRetry_SameTransId(t *testing.T) {
	transIds := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transIds = append(transIds, r.Header.Get("transId"))

		if len(transIds) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := resty.New()
	client.SetBaseURL(server.URL)

	buildRequest := func() *resty.Request {
		req := client.R()
		req.Method = "GET"
		req.URL = "/test"
		return req
	}

	retryCfg := RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryStatus: map[int]bool{
			http.StatusServiceUnavailable: true,
		},
	}

	ctx := t.Context()
	_, err := DoWithRetry(ctx, buildRequest, retryCfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(transIds) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(transIds))
	}

	// Verify all requests used the same transId
	if transIds[0] == "" {
		t.Fatal("transId was empty")
	}

	if transIds[0] != transIds[1] {
		t.Fatalf("transId changed between retries: %s vs %s", transIds[0], transIds[1])
	}
}

func TestDoWithRetry_RespectsRetryAfter(t *testing.T) {
	requestCount := 0
	startTime := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if requestCount == 1 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := resty.New()
	client.SetBaseURL(server.URL)

	buildRequest := func() *resty.Request {
		req := client.R()
		req.Method = "GET"
		req.URL = "/test"
		return req
	}

	retryCfg := DefaultRetryConfig()
	retryCfg.BaseDelay = 5 * time.Second // Would be longer than Retry-After if not respected

	ctx := t.Context()
	_, err := DoWithRetry(ctx, buildRequest, retryCfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	elapsed := time.Since(startTime)

	// Should have waited at least 1 second (Retry-After), but not 5 seconds (base delay)
	if elapsed < 900*time.Millisecond {
		t.Fatalf("Retry-After not respected: elapsed %v", elapsed)
	}

	if elapsed > 3*time.Second {
		t.Fatalf("waited too long: elapsed %v", elapsed)
	}
}
