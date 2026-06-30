// Copyright (c) 2026 Eclipse Foundation AISBL
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package hook_test

import (
	"strings"
	"testing"

	claudeParser "github.com/gnugomez/scribe/hook/claude"
)

var parser = &claudeParser.Parser{}

func TestParse_ModelFromPayload(t *testing.T) {
	input := `{"tool_name":"Write","tool_input":{"file_path":"/tmp/a.ts"},"model":"claude-sonnet-4-6"}`
	entries, err := parser.Parse(strings.NewReader(input), "anthropic", "fallback-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Model != "claude-sonnet-4-6" {
		t.Errorf("expected model from payload, got %q", entries[0].Model)
	}
	if entries[0].ModelSource != "payload" {
		t.Errorf("expected model source payload, got %q", entries[0].ModelSource)
	}
	if entries[0].Vendor != "anthropic" {
		t.Errorf("expected vendor anthropic, got %q", entries[0].Vendor)
	}
}

func TestParse_ModelFromFallbackFlag(t *testing.T) {
	input := `{"tool_name":"Write","tool_input":{"file_path":"/tmp/a.ts"}}`
	entries, err := parser.Parse(strings.NewReader(input), "anthropic", "my-fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Model != "my-fallback" {
		t.Errorf("expected fallback model, got %q", entries[0].Model)
	}
	if entries[0].ModelSource != "flag" {
		t.Errorf("expected model source flag, got %q", entries[0].ModelSource)
	}
}

func TestParse_PayloadModelTakesPriorityOverFlag(t *testing.T) {
	input := `{"model":"payload-model"}`
	entries, err := parser.Parse(strings.NewReader(input), "anthropic", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "payload-model" {
		t.Errorf("payload model should win over flag fallback, got %q", entries[0].Model)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	entries, err := parser.Parse(strings.NewReader(""), "anthropic", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries for empty input, got %d", len(entries))
	}
}

func TestParse_BlankLines(t *testing.T) {
	entries, err := parser.Parse(strings.NewReader("\n\n\n"), "anthropic", "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries for blank input, got %d", len(entries))
	}
}

func TestParse_MalformedJSON_StillProducesEntry(t *testing.T) {
	// Malformed JSON means model falls through to fallback — we still record
	// an event because the hook fired (the LLM was used).
	entries, err := parser.Parse(strings.NewReader("{not-json}"), "anthropic", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry even for malformed JSON, got %d", len(entries))
	}
	if entries[0].Model != "fallback" {
		t.Errorf("expected fallback model, got %q", entries[0].Model)
	}
}

func TestParse_OnlyFirstLineIsConsumed(t *testing.T) {
	// Claude Code sends one object per invocation; we return after first line.
	input := `{"model":"first"}` + "\n" + `{"model":"second"}`
	entries, err := parser.Parse(strings.NewReader(input), "anthropic", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Model != "first" {
		t.Errorf("expected model from first line, got %q", entries[0].Model)
	}
}

func TestParse_StoresSessionID(t *testing.T) {
	input := `{"hook_event_name":"SessionStart","session_id":"sess-123","model":"claude-sonnet-4-6"}`
	entries, err := parser.Parse(strings.NewReader(input), "anthropic", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SessionID != "sess-123" {
		t.Fatalf("expected session id sess-123, got %q", entries[0].SessionID)
	}
	if entries[0].EventName != "SessionStart" {
		t.Errorf("expected EventName=SessionStart, got %q", entries[0].EventName)
	}
}

func TestParse_PostToolUseEventName(t *testing.T) {
	input := `{"hook_event_name":"PostToolUse","session_id":"sess-123","tool_name":"Write"}`
	entries, err := parser.Parse(strings.NewReader(input), "anthropic", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].EventName != "PostToolUse" {
		t.Errorf("expected EventName=PostToolUse, got %q", entries[0].EventName)
	}
}
