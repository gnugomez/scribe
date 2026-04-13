package cmd

import (
	"strings"
	"testing"

	"github.com/jordi-jordi/scribe/internal/pool"
)

// --- mock pool ---

type mockPool struct {
	entries  []pool.Entry
	drained  bool
	cleared  bool
	addErr   error
	drainErr error
}

func (m *mockPool) Add(entries ...pool.Entry) error {
	m.entries = append(m.entries, entries...)
	return m.addErr
}
func (m *mockPool) Peek() ([]pool.Entry, error)  { return m.entries, nil }
func (m *mockPool) Drain() ([]pool.Entry, error) { m.drained = true; m.entries = nil; return m.entries, m.drainErr }
func (m *mockPool) Clear() error                 { m.cleared = true; m.entries = nil; return nil }

// --- mock git ---

type mockGit struct {
	key   string
	value string
	err   error
}

func (m *mockGit) AmendTrailer(key, value string) error {
	m.key = key
	m.value = value
	return m.err
}

// --- helpers ---

func entry(vendor, model string) pool.Entry {
	return pool.Entry{Vendor: vendor, Model: model}
}

// --- tests ---

func TestAmend_EmptyPool(t *testing.T) {
	p := &mockPool{}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil)

	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.key != "" {
		t.Error("expected no git amend when pool is empty")
	}
	if !strings.Contains(out.String(), "empty") {
		t.Errorf("expected 'empty' in output, got: %q", out.String())
	}
}

func TestAmend_CallsGitAndDrainsPool(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{entry("anthropic", "claude-sonnet-4-6")}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.key != "Assisted-By" {
		t.Errorf("expected trailer key 'Assisted-By', got %q", g.key)
	}
	if g.value != "anthropic:claude-sonnet-4-6" {
		t.Errorf("unexpected trailer value: %q", g.value)
	}
	if !p.drained {
		t.Error("pool should be drained after amend")
	}
}

func TestAmend_DeduplicatesPairs(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{
		entry("anthropic", "claude-sonnet-4-6"),
		entry("github", "gpt-4o"),
		entry("anthropic", "claude-sonnet-4-6"), // duplicate
	}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.value != "anthropic:claude-sonnet-4-6, github:gpt-4o" {
		t.Errorf("expected deduplicated trailer, got: %q", g.value)
	}
}

func TestAmend_DryRun_DoesNotAmendOrClear(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{entry("anthropic", "claude-sonnet-4-6")}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil)
	cmd.SetArgs([]string{"--dry-run"})
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.key != "" {
		t.Error("dry-run should not call git amend")
	}
	if p.drained {
		t.Error("dry-run should not drain the pool")
	}
}

func TestAmend_PreservesInsertionOrder(t *testing.T) {
	p := &mockPool{entries: []pool.Entry{
		entry("github", "gpt-4o"),
		entry("anthropic", "claude-sonnet-4-6"),
	}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.value != "github:gpt-4o, anthropic:claude-sonnet-4-6" {
		t.Errorf("expected insertion order preserved, got: %q", g.value)
	}
}
