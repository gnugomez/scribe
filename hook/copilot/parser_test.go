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

	copilotParser "github.com/gnugomez/scribe/hook/copilot"
)

var parser = &copilotParser.Parser{}

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
	if entries[0].ModelSource != "payload" {
		t.Errorf("expected model source payload, got %q", entries[0].ModelSource)
	}
	if entries[0].Vendor != "github" {
		t.Errorf("expected vendor github, got %q", entries[0].Vendor)
	}
}

func TestParse_ModelFromFallbackFlag(t *testing.T) {
	entries, err := parser.Parse(strings.NewReader(`{}`), "github", "my-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "my-model" {
		t.Errorf("expected fallback model, got %q", entries[0].Model)
	}
	if entries[0].ModelSource != "flag" {
		t.Errorf("expected model source flag, got %q", entries[0].ModelSource)
	}
}

func TestParse_PayloadModelTakesPriority(t *testing.T) {
	entries, err := parser.Parse(strings.NewReader(`{"model":"payload-model"}`), "github", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "payload-model" {
		t.Errorf("payload model should win, got %q", entries[0].Model)
	}
}

func TestParse_DefaultsTocopilotWhenNoModelAvailable(t *testing.T) {
	// Payload like VS Code Copilot Chat sends: no model field
	input := `{"hook_event_name":"PostToolUse","tool_name":"insert_edit_into_file"}`
	entries, err := parser.Parse(strings.NewReader(input), "github", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Model != "copilot" {
		t.Errorf("expected default model %q, got %q", "copilot", entries[0].Model)
	}
	if entries[0].ModelSource != "default" {
		t.Errorf("expected model source default, got %q", entries[0].ModelSource)
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

func TestParse_StoresSessionID(t *testing.T) {
	input := `{"hook_event_name":"PostToolUse","session_id":"sess-copilot-123"}`
	entries, err := parser.Parse(strings.NewReader(input), "github", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SessionID != "sess-copilot-123" {
		t.Fatalf("expected session id sess-copilot-123, got %q", entries[0].SessionID)
	}
	if entries[0].EventName != "PostToolUse" {
		t.Errorf("expected EventName=PostToolUse, got %q", entries[0].EventName)
	}
}

func TestParse_SessionStartEventName(t *testing.T) {
	input := `{"hook_event_name":"SessionStart","session_id":"sess-copilot-123","model":"gpt-4o"}`
	entries, err := parser.Parse(strings.NewReader(input), "github", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].EventName != "SessionStart" {
		t.Errorf("expected EventName=SessionStart, got %q", entries[0].EventName)
	}
}
