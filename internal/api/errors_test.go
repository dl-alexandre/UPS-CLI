package api

import (
	"net/http"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected bool
	}{
		{"429 Too Many Requests", 429, true},
		{"500 Internal Server Error", 500, true},
		{"502 Bad Gateway", 502, true},
		{"503 Service Unavailable", 503, true},
		{"504 Gateway Timeout", 504, true},
		{"400 Bad Request", 400, false},
		{"401 Unauthorized", 401, false},
		{"403 Forbidden", 403, false},
		{"404 Not Found", 404, false},
		{"422 Unprocessable Entity", 422, false},
		{"200 OK", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &UPSError{
				StatusCode: tt.status,
				Code:       "TEST",
				Message:    "test error",
			}
			got := IsRetryableError(err)
			if got != tt.expected {
				t.Errorf("IsRetryableError(status=%d) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestIsRetryableError_NonUPSError(t *testing.T) {
	// Non-UPSError should not be retryable
	err := &APIError{
		StatusCode: 500,
		Message:    "some error",
	}
	got := IsRetryableError(err)
	if got {
		t.Error("IsRetryableError should return false for non-UPSError")
	}
}

func TestParseUPSError_StandardFormat(t *testing.T) {
	// This test would need a mocked resty.Response
	// For now, we test the UPSError struct directly
	err := &UPSError{
		StatusCode: http.StatusBadRequest,
		Code:       "250002",
		Message:    "Invalid or missing required parameter",
		Details:    "postal code",
	}

	expected := "UPS API error 400 - 250002: Invalid or missing required parameter (details: postal code)"
	if err.Error() != expected {
		t.Errorf("UPSError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestParseUPSError_NoDetails(t *testing.T) {
	err := &UPSError{
		StatusCode: http.StatusUnauthorized,
		Code:       "401",
		Message:    "Authentication failed",
	}

	expected := "UPS API error 401 - 401: Authentication failed"
	if err.Error() != expected {
		t.Errorf("UPSError.Error() = %q, want %q", err.Error(), expected)
	}
}
