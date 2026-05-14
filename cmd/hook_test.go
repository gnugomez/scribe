package cmd

import (
	"strings"
	"testing"

	"github.com/gnugomez/scribe/store"

	// Ensure parsers are registered (same as root.go does in production).
	_ "github.com/gnugomez/scribe/hook/claude"
	_ "github.com/gnugomez/scribe/hook/copilot"
)

// hookCmd is a convenience helper for tests that don't need a pre-seeded session store.
func hookCmd(edit, session *mockEditPool, payload, format, vendor string) error {
	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(payload))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", vendor, "--format", format})
	return cmd.Execute()
}

func TestHook_AddsToPool_ClaudeCode(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}
	stdin := strings.NewReader(`{"tool_name":"Write","model":"claude-sonnet-4-6"}`)

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(stdin)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 edit entry, got %d", len(edit.entries))
	}
	if edit.entries[0].Vendor != "anthropic" {
		t.Errorf("expected vendor anthropic, got %q", edit.entries[0].Vendor)
	}
	if edit.entries[0].Model != "claude-sonnet-4-6" {
		t.Errorf("expected model from payload, got %q", edit.entries[0].Model)
	}
}

func TestHook_AddsToPool_Copilot(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}
	stdin := strings.NewReader(`{"model":"gpt-4o","tool":"editFile"}`)

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(stdin)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "github", "--format", "copilot"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 edit entry, got %d", len(edit.entries))
	}
	if edit.entries[0].Vendor != "github" || edit.entries[0].Model != "gpt-4o" {
		t.Errorf("unexpected entry: %+v", edit.entries[0])
	}
}

func TestHook_OutsideRepo_ExitsCleanly(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}

	cmd := newHookCmd(edit, session, "")
	cmd.SetIn(strings.NewReader(`{"model":"claude-sonnet-4-6"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected clean exit outside repo, got: %v", err)
	}
	if len(edit.entries) != 0 {
		t.Error("should not add to pool when outside a git repo")
	}
}

func TestHook_UnknownFormat_ExitsCleanly(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "nonexistent"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected clean exit for unknown format, got: %v", err)
	}
	if len(edit.entries) != 0 {
		t.Error("should not add anything for unknown format")
	}
}

func TestHook_EmptyPayload_AddsNothing(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edit.entries) != 0 {
		t.Errorf("expected 0 entries for empty payload, got %d", len(edit.entries))
	}
}

func TestHook_FallbackModelFromFlag(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{"tool_name":"Write"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--model", "claude-opus-4-6"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(edit.entries))
	}
	if edit.entries[0].Model != "claude-opus-4-6" {
		t.Errorf("expected model from --model flag, got %q", edit.entries[0].Model)
	}
}

// TestHook_SessionStartGoesToSessionStore verifies that SessionStart events are
// stored in the session store only and do NOT appear in the edit store.
func TestHook_SessionStartGoesToSessionStore(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"abc-123","model":"claude-sonnet-4-6"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edit.entries) != 0 {
		t.Fatalf("SessionStart should NOT appear in edit store, got %d entries", len(edit.entries))
	}
	if len(session.entries) != 1 {
		t.Fatalf("expected 1 session store entry, got %d", len(session.entries))
	}
	if session.entries[0].SessionID != "abc-123" {
		t.Errorf("expected session id abc-123, got %q", session.entries[0].SessionID)
	}
	if session.entries[0].Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %q", session.entries[0].Model)
	}
}

func TestHook_UsesSessionModelWhenPostToolUseHasNoModel(t *testing.T) {
	// Seed the session store (not the edit store) with a known model.
	session := &mockEditPool{entries: []store.Entry{{
		Vendor:      "anthropic",
		Model:       "claude-sonnet-4-6",
		ModelSource: "payload",
		SessionID:   "sess-42",
		EventName:   "SessionStart",
	}}}
	edit := &mockEditPool{}

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PostToolUse","session_id":"sess-42","tool_name":"Read","tool_input":{}}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 edit entry, got %d", len(edit.entries))
	}
	if edit.entries[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("expected model matched from session store, got %q", edit.entries[0].Model)
	}
	if edit.entries[0].ModelSource != "session" {
		t.Fatalf("expected model source session, got %q", edit.entries[0].ModelSource)
	}
}

// TestHook_SessionModelSurvivesDrain is the regression test for the bug that
// was proven by TestHook_SessionModelLostAfterDrain. Session data lives in the
// session store which is never cleared, so model resolution always works.
func TestHook_SessionModelSurvivesDrain(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}

	run := func(payload string) {
		cmd := newHookCmd(edit, session, "/fake/pool/path")
		cmd.SetIn(strings.NewReader(payload))
		cmd.SetOut(&strings.Builder{})
		cmd.SetErr(&strings.Builder{})
		cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("hook error: %v", err)
		}
	}

	// 1. SessionStart fires — goes to session store.
	run(`{"hook_event_name":"SessionStart","session_id":"sess-fix","model":"claude-sonnet-4-6"}`)
	if len(edit.entries) != 0 {
		t.Fatalf("SessionStart should not be in edit store")
	}
	if len(session.entries) != 1 {
		t.Fatalf("expected session entry, got %d", len(session.entries))
	}

	// 2. Amend — drains EDIT store only. Session store is untouched.
	_, _ = edit.Drain()

	// 3. PostToolUse fires — session store still has the model.
	run(`{"hook_event_name":"PostToolUse","session_id":"sess-fix","tool_name":"Write"}`)

	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 edit entry after drain+hook, got %d", len(edit.entries))
	}
	if edit.entries[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("session model should survive edit drain, got %q", edit.entries[0].Model)
	}
	if edit.entries[0].ModelSource != "session" {
		t.Fatalf("expected model source session, got %q", edit.entries[0].ModelSource)
	}
}

// TestHook_PayloadModelPersistedToSessionPool verifies that when a PostToolUse
// carries the model in its payload but no SessionStart was recorded with it,
// the model gets persisted to the session pool so future events (after amend
// drains the edit pool) can still resolve it.
func TestHook_PayloadModelPersistedToSessionPool(t *testing.T) {
	edit := &mockEditPool{}
	session := &mockEditPool{}

	run := func(payload string) {
		cmd := newHookCmd(edit, session, "/fake/pool/path")
		cmd.SetIn(strings.NewReader(payload))
		cmd.SetOut(&strings.Builder{})
		cmd.SetErr(&strings.Builder{})
		cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("hook error: %v", err)
		}
	}

	// 1. SessionStart without model — session pool only has fallback "claude".
	run(`{"hook_event_name":"SessionStart","session_id":"sess-99"}`)
	if len(session.entries) != 1 {
		t.Fatalf("expected 1 session entry, got %d", len(session.entries))
	}
	if session.entries[0].Model != "claude" {
		t.Fatalf("expected fallback model in session entry, got %q", session.entries[0].Model)
	}

	// 2. PostToolUse WITH model in payload — model gets persisted to session pool.
	run(`{"hook_event_name":"PostToolUse","session_id":"sess-99","model":"claude-sonnet-4-6","tool_name":"Write"}`)
	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 edit entry, got %d", len(edit.entries))
	}
	if edit.entries[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("expected payload model, got %q", edit.entries[0].Model)
	}
	// Session pool should now have an additional entry with the real model.
	if len(session.entries) < 2 {
		t.Fatalf("expected model to be persisted to session pool, got %d entries", len(session.entries))
	}

	// 3. Simulate amend — drain edit pool.
	_, _ = edit.Drain()

	// 4. PostToolUse WITHOUT model — should still resolve from session pool.
	run(`{"hook_event_name":"PostToolUse","session_id":"sess-99","tool_name":"Read"}`)
	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 edit entry after drain, got %d", len(edit.entries))
	}
	if edit.entries[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("expected session-resolved model after drain, got %q", edit.entries[0].Model)
	}
	if edit.entries[0].ModelSource != "session" {
		t.Fatalf("expected model source session, got %q", edit.entries[0].ModelSource)
	}
}

func TestHook_DoesNotMixSessionModelAcrossVendors(t *testing.T) {
	// Seed session store with a github entry.
	session := &mockEditPool{entries: []store.Entry{{
		Vendor:      "github",
		Model:       "gpt-4.1",
		ModelSource: "payload",
		SessionID:   "shared-session",
		EventName:   "SessionStart",
	}}}
	edit := &mockEditPool{}

	cmd := newHookCmd(edit, session, "/fake/pool/path")
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PostToolUse","session_id":"shared-session","tool_name":"Read"}`))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--vendor", "anthropic", "--format", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(edit.entries) != 1 {
		t.Fatalf("expected 1 edit entry, got %d", len(edit.entries))
	}
	if edit.entries[0].Model == "gpt-4.1" {
		t.Fatalf("model leaked across vendors; got %q", edit.entries[0].Model)
	}
}
