package cmd

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jordi-jordi/scribe/internal/pool"
)

type errPeekPool struct {
	err error
}

func (e *errPeekPool) Add(entries ...pool.Entry) error { return nil }
func (e *errPeekPool) Peek() ([]pool.Entry, error)     { return nil, e.err }
func (e *errPeekPool) Drain() ([]pool.Entry, error)    { return nil, nil }
func (e *errPeekPool) Clear() error                    { return nil }

func TestPool_PrintsEntries(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{{
		Timestamp: time.Date(2026, time.April, 13, 12, 30, 31, 0, time.UTC),
		Vendor:    "anthropic",
		Model:     "claude",
		Payload:   `{"tool_name":"read_file"}`,
	}}}

	cmd := newPoolCmd(p, "/tmp/pool.jsonl")
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "2026-04-13T12:30:31Z  anthropic:claude") {
		t.Fatalf("expected timestamp/vendor/model line, got: %q", got)
	}
	if strings.Contains(got, "payload") {
		t.Fatalf("did not expect payload output without --debug, got: %q", got)
	}
}

func TestPool_DebugPrettyPrintsPayload(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{{
		Timestamp: time.Date(2026, time.April, 13, 12, 30, 31, 0, time.UTC),
		Vendor:    "anthropic",
		Model:     "claude",
		Payload:   `{"hook_event_name":"PostToolUse","tool_name":"read_file"}`,
	}}}

	cmd := newPoolCmd(p, "/tmp/pool.jsonl")
	cmd.SetArgs([]string{"--debug"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "payload:") {
		t.Fatalf("expected payload section, got: %q", got)
	}
	if !strings.Contains(got, `"hook_event_name": "PostToolUse"`) {
		t.Fatalf("expected pretty JSON payload, got: %q", got)
	}
}

func TestPool_DebugFallsBackToRawPayloadForInvalidJSON(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{{
		Timestamp: time.Date(2026, time.April, 13, 12, 30, 31, 0, time.UTC),
		Vendor:    "anthropic",
		Model:     "claude",
		Payload:   "{not-json}",
	}}}

	cmd := newPoolCmd(p, "/tmp/pool.jsonl")
	cmd.SetArgs([]string{"--debug"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "payload (raw): {not-json}") {
		t.Fatalf("expected raw payload fallback, got: %q", got)
	}
}

func TestPool_PeekError(t *testing.T) {
	cmd := newPoolCmd(&errPeekPool{err: errors.New("boom")}, "/tmp/pool.jsonl")
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "reading pool: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
