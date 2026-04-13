package cmd

import (
	"strings"
	"testing"

	// Ensure parsers are registered (same as root.go does in production).
	_ "github.com/jordi-jordi/scribe/internal/hook/claudecode"
	_ "github.com/jordi-jordi/scribe/internal/hook/copilot"
)

func TestHook_AddsToPool_ClaudeCode(t *testing.T) {
	p := &mockPool{}
	stdin := strings.NewReader(`{"tool_name":"Write","model":"claude-sonnet-4-6"}`)

	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(stdin)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude-code"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.entries) != 1 {
		t.Fatalf("expected 1 pool entry, got %d", len(p.entries))
	}
	if p.entries[0].Vendor != "anthropic" {
		t.Errorf("expected vendor anthropic, got %q", p.entries[0].Vendor)
	}
	if p.entries[0].Model != "claude-sonnet-4-6" {
		t.Errorf("expected model from payload, got %q", p.entries[0].Model)
	}
}

func TestHook_AddsToPool_Copilot(t *testing.T) {
	p := &mockPool{}
	stdin := strings.NewReader(`{"model":"gpt-4o","tool":"editFile"}`)

	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(stdin)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "github", "--format", "copilot"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.entries) != 1 {
		t.Fatalf("expected 1 pool entry, got %d", len(p.entries))
	}
	if p.entries[0].Vendor != "github" || p.entries[0].Model != "gpt-4o" {
		t.Errorf("unexpected entry: %+v", p.entries[0])
	}
}

func TestHook_OutsideRepo_ExitsCleanly(t *testing.T) {
	p := &mockPool{}

	// poolPath == "" signals we're not in a git repo.
	cmd := newHookCmd(p, "")
	cmd.SetIn(strings.NewReader(`{"model":"claude-sonnet-4-6"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected clean exit outside repo, got: %v", err)
	}
	if len(p.entries) != 0 {
		t.Error("should not add to pool when outside a git repo")
	}
}

func TestHook_UnknownFormat_ExitsCleanly(t *testing.T) {
	p := &mockPool{}

	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "nonexistent"})

	// Should exit 0, not return an error (must not block the calling tool).
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected clean exit for unknown format, got: %v", err)
	}
	if len(p.entries) != 0 {
		t.Error("should not add anything for unknown format")
	}
}

func TestHook_EmptyPayload_AddsNothing(t *testing.T) {
	p := &mockPool{}

	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.entries) != 0 {
		t.Errorf("expected 0 entries for empty payload, got %d", len(p.entries))
	}
}

func TestHook_FallbackModelFromFlag(t *testing.T) {
	t.Setenv("CLAUDE_MODEL", "")
	p := &mockPool{}

	cmd := newHookCmd(p, "/fake/pool/path")
	// Payload has no model field.
	cmd.SetIn(strings.NewReader(`{"tool_name":"Write"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--model", "claude-opus-4-6"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(p.entries))
	}
	if p.entries[0].Model != "claude-opus-4-6" {
		t.Errorf("expected model from --model flag, got %q", p.entries[0].Model)
	}
}
