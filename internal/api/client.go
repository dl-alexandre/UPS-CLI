package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// ClientOptions contains configuration for the API client
type ClientOptions struct {
	BaseURL string
	Timeout int
	Verbose bool
	Debug   bool
}

// Client is the API client for making HTTP requests
type Client struct {
	client        *resty.Client
	tokenProvider *TokenProvider
	verbose       bool
	debug         bool
}

// NewClient creates a new API client
func NewClient(opts ClientOptions) *Client {
	client := resty.New()
	client.SetBaseURL(opts.BaseURL)
	client.SetTimeout(time.Duration(opts.Timeout) * time.Second)
	client.SetHeader("Accept", "application/json")
	client.SetHeader("User-Agent", "ups-cli/1.0.0")
	client.SetContentLength(true)

	if opts.Debug {
		client.SetDebug(true)
	}

	return &Client{
		client:  client,
		verbose: opts.Verbose,
		debug:   opts.Debug,
	}
}

// SetTokenProvider sets the OAuth token provider for authentication
func (c *Client) SetTokenProvider(provider *TokenProvider) {
	c.tokenProvider = provider
}

// doRequest executes an authenticated HTTP request
func (c *Client) doRequest(ctx context.Context, method, url string, body interface{}) (*resty.Response, error) {
	// Get OAuth token if token provider is configured
	if c.tokenProvider != nil {
		token, err := c.tokenProvider.GetToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		c.client.SetAuthToken(token)
	}

	req := c.client.R().SetContext(ctx)

	// Add UPS transaction headers for support/debugging
	req.SetHeader("transId", generateTransactionID())
	req.SetHeader("transactionSrc", "ups-cli")

	if body != nil {
		req.SetBody(body)
	}

	return req.Execute(method, url)
}

// generateTransactionID creates a simple transaction ID
func generateTransactionID() string {
	return fmt.Sprintf("upscli-%d", time.Now().UnixNano())
}

// Tracking types

// TrackingRequest represents a UPS tracking request
type TrackingRequest struct {
	TrackingNumber string `json:"trackingNumber"`
}

// TrackingResponse represents the UPS tracking response
type TrackingResponse struct {
	TrackingNumber string          `json:"trackingNumber"`
	StatusCode     string          `json:"statusCode"`
	StatusDesc     string          `json:"statusDescription"`
	Service        string          `json:"serviceCode"`
	ServiceDesc    string          `json:"serviceDescription,omitempty"`
	PickupDate     string          `json:"pickupDate,omitempty"`
	ScheduledDate  string          `json:"scheduledDeliveryDate,omitempty"`
	ActualDate     string          `json:"actualDeliveryDate,omitempty"`
	CurrentStatus  *TrackingStatus `json:"currentStatus,omitempty"`
	PackageCount   int             `json:"packageCount,omitempty"`
	ShipmentEvents []ShipmentEvent `json:"shipmentEvents,omitempty"`
	Error          *TrackingError  `json:"error,omitempty"`
}

// TrackingStatus represents the current tracking status
type TrackingStatus struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

// ShipmentEvent represents a single tracking event
type ShipmentEvent struct {
	Timestamp   string `json:"timestamp"`
	Code        string `json:"code,omitempty"`
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
	City        string `json:"city,omitempty"`
	State       string `json:"state,omitempty"`
	Country     string `json:"country,omitempty"`
}

// TrackingError represents a UPS API error
type TrackingError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Track tracks a UPS shipment by tracking number
func (c *Client) Track(ctx context.Context, trackingNumber string, locale string) (*TrackingResponse, error) {
	endpoint := fmt.Sprintf("/track/v1/details/%s", trackingNumber)

	// Add locale query parameter
	if locale == "" {
		locale = "en_US"
	}
	endpoint = endpoint + "?locale=" + locale

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result TrackingResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode tracking response: %w", err)
	}

	if result.Error != nil {
		return nil, &APIError{
			StatusCode: resp.StatusCode(),
			Message:    fmt.Sprintf("%s: %s", result.Error.Code, result.Error.Message),
		}
	}

	return &result, nil
}

// TrackRaw returns the raw tracking response body (for debugging)
func (c *Client) TrackRaw(ctx context.Context, trackingNumber string, locale string) ([]byte, error) {
	endpoint := fmt.Sprintf("/track/v1/details/%s", trackingNumber)

	// Add locale query parameter
	if locale == "" {
		locale = "en_US"
	}
	endpoint = endpoint + "?locale=" + locale

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return resp.Body(), nil
}

// Legacy example methods (kept for compatibility)

// Item represents a generic API resource
type Item struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Metadata    Metadata  `json:"metadata,omitempty"`
}

// Metadata contains additional resource information
type Metadata struct {
	Tags       []string          `json:"tags,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ListResponse represents a paginated list response
type ListResponse struct {
	Items      []Item `json:"items"`
	Total      int    `json:"total"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	HasMore    bool   `json:"has_more"`
	NextOffset int    `json:"next_offset,omitempty"`
}

// List retrieves a paginated list of items
func (c *Client) List(ctx context.Context, limit, offset int) (*ListResponse, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		SetQueryParam("limit", fmt.Sprintf("%d", limit)).
		SetQueryParam("offset", fmt.Sprintf("%d", offset)).
		Get("/api/v1/items")

	if err != nil {
		return nil, fmt.Errorf("failed to list items: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result ListResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Get retrieves a single item by ID
func (c *Client) Get(ctx context.Context, id string) (*Item, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		SetPathParam("id", id).
		Get("/api/v1/items/{id}")

	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, &NotFoundError{Resource: id}
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result Item
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Search searches for items matching the query
func (c *Client) Search(ctx context.Context, query string, limit int) (*ListResponse, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		SetQueryParam("q", query).
		SetQueryParam("limit", fmt.Sprintf("%d", limit)).
		Get("/api/v1/search")

	if err != nil {
		return nil, fmt.Errorf("failed to search items: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result ListResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// handleError processes error responses
func (c *Client) handleError(resp *resty.Response) error {
	return &APIError{
		StatusCode: resp.StatusCode(),
		Message:    resp.String(),
	}
}

// APIError represents an API error response
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// NotFoundError represents a resource not found error
type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("resource not found: %s", e.Resource)
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Errorf("validation error for %s: %s", e.Field, e.Message).Error()
}
