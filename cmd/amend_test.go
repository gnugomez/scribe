// Copyright (c) 2026 Jordi Gómez Hidalgo
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"io"
	"strings"
	"testing"

	"github.com/gnugomez/scribe/git"
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
func (m *mockEditPool) DrainMatching(pairs map[string]struct{}) ([]store.Entry, error) {
	m.drained = true
	var matched, remaining []store.Entry
	for _, e := range m.entries {
		key := e.Vendor + ":" + e.Model
		if _, ok := pairs[key]; ok {
			matched = append(matched, e)
		} else {
			remaining = append(remaining, e)
		}
	}
	m.entries = remaining
	return matched, m.drainErr
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
	key    string
	value  string
	err    error
	hashes []string // hashes passed to AmendTrailerOnCommits

	unpushedCommits  []git.Commit
	sinceCommits     []git.Commit
}

func (m *mockGit) AmendTrailer(key, value string) error {
	m.key = key
	m.value = value
	return m.err
}

func (m *mockGit) UnpushedCommits() ([]git.Commit, error) {
	return m.unpushedCommits, nil
}

func (m *mockGit) CommitsSince(ref string) ([]git.Commit, error) {
	return m.sinceCommits, nil
}

func (m *mockGit) AmendTrailerOnCommits(out io.Writer, hashes []string, key, value string) error {
	m.hashes = hashes
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
	p := &mockEditPool{entries: []store.Entry{entry("anthropic", "claude-sonnet-4-6")}}
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
	p := &mockEditPool{entries: []store.Entry{
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
	p := &mockEditPool{entries: []store.Entry{entry("anthropic", "claude-sonnet-4-6")}}
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
	p := &mockEditPool{entries: []store.Entry{
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
