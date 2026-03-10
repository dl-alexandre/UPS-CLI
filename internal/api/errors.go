package api

import (
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// UPSError represents a structured UPS API error
type UPSError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
	StatusCode int    `json:"status_code"`
}

func (e *UPSError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("UPS API error %d - %s: %s (details: %s)",
			e.StatusCode, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("UPS API error %d - %s: %s",
		e.StatusCode, e.Code, e.Message)
}

// UPSStandardError represents the standard UPS error response structure
type UPSStandardError struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details string `json:"details,omitempty"`
	} `json:"errors"`
}

// ParseUPSError parses a UPS API error response into a structured error
func ParseUPSError(resp *resty.Response) error {
	statusCode := resp.StatusCode()

	// Try to parse structured error
	var stdErr UPSStandardError
	if err := json.Unmarshal(resp.Body(), &stdErr); err == nil && len(stdErr.Errors) > 0 {
		firstErr := stdErr.Errors[0]
		return &UPSError{
			StatusCode: statusCode,
			Code:       firstErr.Code,
			Message:    firstErr.Message,
			Details:    firstErr.Details,
		}
	}

	// Try alternative error format (tracking uses different structure)
	var trackingErr struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body(), &trackingErr); err == nil && trackingErr.Error.Code != "" {
		return &UPSError{
			StatusCode: statusCode,
			Code:       trackingErr.Error.Code,
			Message:    trackingErr.Error.Message,
		}
	}

	// Fallback to generic error
	return &UPSError{
		StatusCode: statusCode,
		Code:       fmt.Sprintf("HTTP%d", statusCode),
		Message:    resp.String(),
	}
}

// IsRetryableError determines if an error should trigger a retry
func IsRetryableError(err error) bool {
	if upsErr, ok := err.(*UPSError); ok {
		switch upsErr.StatusCode {
		case 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}
	return false
}
