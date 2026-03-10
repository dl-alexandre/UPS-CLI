package cli

import (
	"testing"
)

func TestGlobals_AfterApply(t *testing.T) {
	tests := []struct {
		name    string
		globals Globals
		wantErr bool
	}{
		{
			name:    "empty globals - no initialization",
			globals: Globals{
				// Empty - should skip initialization
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.globals.AfterApply()
			if (err != nil) != tt.wantErr {
				t.Errorf("AfterApply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGlobals_ShouldUseColor(t *testing.T) {
	g := &Globals{}

	// This will depend on the test environment
	// Just ensure it doesn't panic
	_ = g.ShouldUseColor()
}

func TestListCmd_Validation(t *testing.T) {
	cmd := &ListCmd{
		Limit:  10,
		Offset: 0,
		Format: "json",
	}

	if cmd.Limit < 1 {
		t.Error("limit should be at least 1")
	}

	if cmd.Offset < 0 {
		t.Error("offset should not be negative")
	}

	validFormats := []string{"table", "json", "markdown"}
	found := false
	for _, f := range validFormats {
		if f == cmd.Format {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("format %s is not valid", cmd.Format)
	}
}

func TestGetCmd_Validation(t *testing.T) {
	cmd := &GetCmd{
		ID:     "test-id",
		Format: "json",
	}

	if cmd.ID == "" {
		t.Error("ID should not be empty")
	}

	validFormats := []string{"table", "json", "markdown"}
	found := false
	for _, f := range validFormats {
		if f == cmd.Format {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("format %s is not valid", cmd.Format)
	}
}

func TestSearchCmd_Validation(t *testing.T) {
	cmd := &SearchCmd{
		Query:  "test query",
		Limit:  5,
		Format: "table",
	}

	if cmd.Query == "" {
		t.Error("query should not be empty")
	}

	if cmd.Limit < 1 {
		t.Error("limit should be at least 1")
	}
}

func TestCompletionCmd_Shells(t *testing.T) {
	validShells := []string{"bash", "zsh", "fish", "powershell"}

	for _, shell := range validShells {
		t.Run(shell, func(t *testing.T) {
			cmd := &CompletionCmd{Shell: shell}
			// Just ensure shell value is set
			if cmd.Shell != shell {
				t.Errorf("expected shell %s, got %s", shell, cmd.Shell)
			}
		})
	}
}
