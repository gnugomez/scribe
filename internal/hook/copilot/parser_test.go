package copilot_test

import (
	"strings"
	"testing"

	"github.com/jordi-jordi/scribe/internal/hook/copilot"
)

var parser = &copilot.Parser{}

func TestParse_ModelFromPayload(t *testing.T) {
	input := `{"model":"gpt-4o","tool":"editFile","input":{}}`
	entries, err := parser.Parse(strings.NewReader(input), "github", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Model != "gpt-4o" {
		t.Errorf("expected model from payload, got %q", entries[0].Model)
	}
	if entries[0].Vendor != "github" {
		t.Errorf("expected vendor github, got %q", entries[0].Vendor)
	}
}

func TestParse_ModelFromCOPILOT_MODEL(t *testing.T) {
	t.Setenv("COPILOT_MODEL", "gpt-4.5")
	t.Setenv("GITHUB_COPILOT_MODEL", "")

	input := `{"tool":"editFile"}`
	entries, err := parser.Parse(strings.NewReader(input), "github", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "gpt-4.5" {
		t.Errorf("expected model from COPILOT_MODEL, got %q", entries[0].Model)
	}
}

func TestParse_ModelFromGITHUB_COPILOT_MODEL(t *testing.T) {
	t.Setenv("COPILOT_MODEL", "")
	t.Setenv("GITHUB_COPILOT_MODEL", "gpt-4o-mini")

	input := `{"tool":"editFile"}`
	entries, err := parser.Parse(strings.NewReader(input), "github", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "gpt-4o-mini" {
		t.Errorf("expected model from GITHUB_COPILOT_MODEL, got %q", entries[0].Model)
	}
}

func TestParse_ModelFromFallbackFlag(t *testing.T) {
	t.Setenv("COPILOT_MODEL", "")
	t.Setenv("GITHUB_COPILOT_MODEL", "")

	entries, err := parser.Parse(strings.NewReader(`{}`), "github", "my-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "my-model" {
		t.Errorf("expected fallback model, got %q", entries[0].Model)
	}
}

func TestParse_PayloadModelTakesPriority(t *testing.T) {
	t.Setenv("COPILOT_MODEL", "env-model")

	entries, err := parser.Parse(strings.NewReader(`{"model":"payload-model"}`), "github", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "payload-model" {
		t.Errorf("payload model should win, got %q", entries[0].Model)
	}
}

func TestParse_DefaultsTocopilotWhenNoModelAvailable(t *testing.T) {
	t.Setenv("COPILOT_MODEL", "")
	t.Setenv("GITHUB_COPILOT_MODEL", "")

	// Payload like VS Code Copilot Chat sends: no model field
	input := `{"hook_event_name":"PostToolUse","tool_name":"insert_edit_into_file"}`
	entries, err := parser.Parse(strings.NewReader(input), "github", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "copilot" {
		t.Errorf("expected default model %q, got %q", "copilot", entries[0].Model)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	entries, err := parser.Parse(strings.NewReader(""), "github", "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}
