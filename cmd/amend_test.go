package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnugomez/scribe/store"
)

// --- mock pools ---

type mockEditPool struct {
	entries  []store.Entry
	drained  bool
	cleared  bool
	addErr   error
	drainErr error
}

func (m *mockEditPool) Add(entries ...store.Entry) error {
	m.entries = append(m.entries, entries...)
	return m.addErr
}
func (m *mockEditPool) Peek() ([]store.Entry, error) { return m.entries, nil }
func (m *mockEditPool) Drain() ([]store.Entry, error) {
	m.drained = true
	got := m.entries
	m.entries = nil
	return got, m.drainErr
}
func (m *mockEditPool) Clear() error { m.cleared = true; m.entries = nil; return nil }

type mockSessionPool struct {
	entries []store.Entry
}

func (m *mockSessionPool) Add(entries ...store.Entry) error {
	m.entries = append(m.entries, entries...)
	return nil
}
func (m *mockSessionPool) Peek() ([]store.Entry, error) { return m.entries, nil }

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

func entry(vendor, model string) store.Entry {
	return store.Entry{Vendor: vendor, Model: model}
}

// --- tests ---

func TestAmend_EmptyPool(t *testing.T) {
	p := &mockEditPool{}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil, "", nil)

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
	p := &mockEditPool{entries: []store.Entry{entry("anthropic", "claude-sonnet-4-6")}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil, "", nil)
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
	p := &mockEditPool{entries: []store.Entry{
		entry("anthropic", "claude-sonnet-4-6"),
		entry("github", "gpt-4o"),
		entry("anthropic", "claude-sonnet-4-6"), // duplicate
	}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil, "", nil)
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
	p := &mockEditPool{entries: []store.Entry{entry("anthropic", "claude-sonnet-4-6")}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil, "", nil)
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
	p := &mockEditPool{entries: []store.Entry{
		entry("github", "gpt-4o"),
		entry("anthropic", "claude-sonnet-4-6"),
	}}
	g := &mockGit{}
	cmd := newAmendCmd(p, g, nil, "", nil)
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.value != "github:gpt-4o, anthropic:claude-sonnet-4-6" {
		t.Errorf("expected insertion order preserved, got: %q", g.value)
	}
}

// --- stale-pool guard tests ---

// headFile creates a temp sentinel file containing the given hash and returns
// the dir and file path.
func headFile(t *testing.T, hash string) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "pool-head")
	if err := os.WriteFile(path, []byte(hash), 0o644); err != nil {
		t.Fatalf("writing pool-head: %v", err)
	}
	return dir, path
}

func hashFn(hash string) func() (string, error) {
	return func() (string, error) { return hash, nil }
}

// TestAmend_StalePool_ClearsEntries verifies the primary use-case: a
// hard-reset changed HEAD, so stale pool entries must not be applied.
func TestAmend_StalePool_ClearsEntries(t *testing.T) {
	_, headPath := headFile(t, "old-hash")
	p := &mockEditPool{entries: []store.Entry{entry("anthropic", "claude-sonnet-4-6")}}
	g := &mockGit{}

	var out strings.Builder
	cmd := newAmendCmd(p, g, nil, headPath, hashFn("current-hash"))
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.key != "" {
		t.Error("expected no git amend when pool is stale")
	}
	if !p.cleared {
		t.Error("expected pool to be cleared when stale")
	}
	if !strings.Contains(out.String(), "pool empty") {
		t.Errorf("expected 'pool empty' in output, got: %q", out.String())
	}
}

// TestAmend_FreshPool_AmendsNormally verifies that a matching sentinel does
// not interfere with the normal amend path.
func TestAmend_FreshPool_AmendsNormally(t *testing.T) {
	_, headPath := headFile(t, "current-hash")
	p := &mockEditPool{entries: []store.Entry{entry("anthropic", "claude-sonnet-4-6")}}
	g := &mockGit{}

	cmd := newAmendCmd(p, g, nil, headPath, hashFn("current-hash"))
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.key != "Assisted-By" {
		t.Errorf("expected git amend on fresh pool, got key=%q", g.key)
	}
	if !p.drained {
		t.Error("expected pool to be drained after successful amend")
	}
}

// TestAmend_StaleDetection_FalsePositive_ManualGitAmend documents a known
// false positive: pool entries accumulated for a commit are treated as stale
// when the user manually runs 'git commit --amend' (e.g. to fix a typo in
// the commit message) before running 'scribe amend'.
//
// Timeline that triggers this:
//
//	1. make commit (HEAD = A)
//	2. use AI tools → entries in pool, pool-head = A
//	3. git commit --amend --no-edit   ← only fixes something unrelated; HEAD = A'
//	4. scribe amend → detects A ≠ A', clears the pool → Assisted-By never added
func TestAmend_StaleDetection_FalsePositive_ManualGitAmend(t *testing.T) {
	_, headPath := headFile(t, "hash-before-manual-amend")
	p := &mockEditPool{entries: []store.Entry{entry("anthropic", "claude-sonnet-4-6")}}
	g := &mockGit{}

	// HEAD is now A' because the user ran 'git commit --amend' themselves.
	cmd := newAmendCmd(p, g, nil, headPath, hashFn("hash-after-manual-amend"))
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Current behaviour: legitimate entries are cleared (false positive).
	if !p.cleared {
		t.Error("current behaviour: pool cleared despite entries being legitimate")
	}
	if g.key != "" {
		t.Error("current behaviour: git amend not called — Assisted-By trailer lost")
	}
}
