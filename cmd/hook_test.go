package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jordi-jordi/scribe/internal/pool"

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
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})

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

func TestHook_SessionStartStoresSessionAndModel(t *testing.T) {
	p := &mockPool{}
	stdin := strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"abc-123","model":"claude-sonnet-4-6"}`)

	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(stdin)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(p.entries))
	}
	if p.entries[0].SessionID != "abc-123" {
		t.Fatalf("expected session id to be stored, got %q", p.entries[0].SessionID)
	}
	if p.entries[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("expected model from SessionStart payload, got %q", p.entries[0].Model)
	}
}

func TestHook_UsesSessionModelWhenPostToolUseHasNoModel(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{{
		Vendor:    "anthropic",
		Model:     "claude-sonnet-4-6",
		SessionID: "sess-42",
	}}}

	stdin := strings.NewReader(`{"hook_event_name":"PostToolUse","session_id":"sess-42","tool_name":"Read","tool_input":{}}`)
	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(stdin)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.entries) != 2 {
		t.Fatalf("expected 2 entries (existing + new), got %d", len(p.entries))
	}
	last := p.entries[len(p.entries)-1]
	if last.Model != "claude-sonnet-4-6" {
		t.Fatalf("expected model to be matched from session, got %q", last.Model)
	}
}

func TestHook_TracksModelChangesWithinSameSession(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{{
		Vendor:    "anthropic",
		Model:     "claude-sonnet-4-6",
		SessionID: "sess-77",
	}}}

	// First payload carries a newer model for the same session.
	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PostToolUse","session_id":"sess-77","model":"claude-opus-4-6","tool_name":"Read"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error on known-model event: %v", err)
	}

	// Second payload has no model, should resolve to the latest one from same session.
	cmd = newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PostToolUse","session_id":"sess-77","tool_name":"Edit"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error on unknown-model event: %v", err)
	}

	last := p.entries[len(p.entries)-1]
	if last.Model != "claude-opus-4-6" {
		t.Fatalf("expected latest session model to be used, got %q", last.Model)
	}
}

func TestHook_DoesNotMixSessionModelAcrossVendors(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{{
		Vendor:    "github",
		Model:     "gpt-4.1",
		SessionID: "shared-session",
	}}}

	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PostToolUse","session_id":"shared-session","tool_name":"Read"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	last := p.entries[len(p.entries)-1]
	if last.Model == "gpt-4.1" {
		t.Fatalf("model leaked across vendors; got %q", last.Model)
	}
}

func TestResolveModelFromTranscript(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")
	content := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"/model"}}`,
		`{"type":"assistant","message":{"model":"claude-haiku-4-5-20251001"}}`,
		`{"type":"assistant","message":{"model":"claude-sonnet-4-6"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	payload := `{"transcript_path":"` + transcriptPath + `"}`
	got, ok := resolveModelFromTranscript(payload)
	if !ok {
		t.Fatal("expected transcript model resolution to succeed")
	}
	if got != "claude-sonnet-4-6" {
		t.Fatalf("expected latest assistant model, got %q", got)
	}
}

func TestResolveModelFromTranscript_NoPath(t *testing.T) {
	if _, ok := resolveModelFromTranscript(`{"hook_event_name":"PostToolUse"}`); ok {
		t.Fatal("expected no model without transcript_path")
	}
}

func TestHook_TranscriptModelOverridesStaleSessionCache(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")
	content := strings.Join([]string{
		`{"type":"assistant","message":{"model":"claude-opus-4-6"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	p := &mockPool{entries: []pool.Entry{{
		Vendor:    "anthropic",
		Model:     "claude-sonnet-4-6",
		SessionID: "sess-99",
	}}}

	payload := `{"hook_event_name":"PostToolUse","session_id":"sess-99","transcript_path":"` + transcriptPath + `","tool_name":"Edit"}`
	cmd := newHookCmd(p, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(payload))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last := p.entries[len(p.entries)-1]
	if last.Model != "claude-opus-4-6" {
		t.Fatalf("expected transcript model to override stale cache, got %q", last.Model)
	}
}
