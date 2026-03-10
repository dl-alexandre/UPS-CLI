package api

import (
	"testing"
	"time"
)

func TestItem_Validation(t *testing.T) {
	item := &Item{
		ID:          "test-id",
		Name:        "Test Item",
		Description: "A test item",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata: Metadata{
			Tags:       []string{"test", "example"},
			Attributes: map[string]string{"key": "value"},
		},
	}

	if item.ID == "" {
		t.Error("ID should not be empty")
	}

	if item.Name == "" {
		t.Error("Name should not be empty")
	}
}

func TestListResponse_Validation(t *testing.T) {
	response := &ListResponse{
		Items: []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
		},
		Total:      10,
		Limit:      2,
		Offset:     0,
		HasMore:    true,
		NextOffset: 2,
	}

	if len(response.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(response.Items))
	}

	if response.Total != 10 {
		t.Errorf("expected total 10, got %d", response.Total)
	}

	if !response.HasMore {
		t.Error("expected HasMore to be true")
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		StatusCode: 404,
		Message:    "Not found",
	}

	expected := "API error 404: Not found"
	if err.Error() != expected {
		t.Errorf("expected error message %q, got %q", expected, err.Error())
	}
}

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{
		Resource: "test-resource",
	}

	expected := "resource not found: test-resource"
	if err.Error() != expected {
		t.Errorf("expected error message %q, got %q", expected, err.Error())
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "name",
		Message: "cannot be empty",
	}

	expected := "validation error for name: cannot be empty"
	if err.Error() != expected {
		t.Errorf("expected error message %q, got %q", expected, err.Error())
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient(ClientOptions{
		BaseURL: "https://api.example.com",
		Timeout: 30,
		Verbose: false,
		Debug:   false,
	})

	if client == nil {
		t.Error("client should not be nil")
	}
}
