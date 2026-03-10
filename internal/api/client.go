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
	client  *resty.Client
	verbose bool
	debug   bool
}

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

// NewClient creates a new API client
func NewClient(opts ClientOptions) *Client {
	client := resty.New()
	client.SetBaseURL(opts.BaseURL)
	client.SetTimeout(time.Duration(opts.Timeout) * time.Second)
	client.SetHeader("Accept", "application/json")
	client.SetHeader("User-Agent", "ups/1.0.0")

	if opts.Debug {
		client.SetDebug(true)
	}

	return &Client{
		client:  client,
		verbose: opts.Verbose,
		debug:   opts.Debug,
	}
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
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}
