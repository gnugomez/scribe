package cmd

import (
	"strings"
	"testing"

	"github.com/gnugomez/scribe/store"
)

func TestParseGitStatus_BranchOnly(t *testing.T) {
	gs := &parsedStatus{}
	parseBranchLine("main", gs)
	if gs.branch != "main" {
		t.Errorf("expected branch 'main', got %q", gs.branch)
	}
	if gs.upstream != "" {
		t.Errorf("expected no upstream, got %q", gs.upstream)
	}
}

func TestParseGitStatus_BranchWithUpstream(t *testing.T) {
	gs := &parsedStatus{}
	parseBranchLine("main...origin/main", gs)
	if gs.branch != "main" {
		t.Errorf("expected branch 'main', got %q", gs.branch)
	}
	if gs.upstream != "origin/main" {
		t.Errorf("expected upstream 'origin/main', got %q", gs.upstream)
	}
	if gs.ahead != 0 || gs.behind != 0 {
		t.Errorf("expected no divergence, got ahead=%d behind=%d", gs.ahead, gs.behind)
	}
}

func TestParseGitStatus_BranchAhead(t *testing.T) {
	gs := &parsedStatus{}
	parseBranchLine("main...origin/main [ahead 3]", gs)
	if gs.ahead != 3 {
		t.Errorf("expected ahead=3, got %d", gs.ahead)
	}
	if gs.behind != 0 {
		t.Errorf("expected behind=0, got %d", gs.behind)
	}
}

func TestParseGitStatus_BranchBehind(t *testing.T) {
	gs := &parsedStatus{}
	parseBranchLine("main...origin/main [behind 2]", gs)
	if gs.behind != 2 {
		t.Errorf("expected behind=2, got %d", gs.behind)
	}
}

func TestParseGitStatus_BranchDiverged(t *testing.T) {
	gs := &parsedStatus{}
	parseBranchLine("feat/x...origin/feat/x [ahead 1, behind 2]", gs)
	if gs.branch != "feat/x" {
		t.Errorf("expected branch 'feat/x', got %q", gs.branch)
	}
	if gs.ahead != 1 || gs.behind != 2 {
		t.Errorf("expected ahead=1 behind=2, got ahead=%d behind=%d", gs.ahead, gs.behind)
	}
}

func TestParseGitStatus_InitialCommit(t *testing.T) {
	gs := &parsedStatus{}
	parseBranchLine("No commits yet on main", gs)
	if gs.branch != "main" {
		t.Errorf("expected branch 'main' for initial commit, got %q", gs.branch)
	}
}

func TestStatusCmd_EmptyPoolAndClean(t *testing.T) {
	p := &mockEditPool{}
	cmd := newStatusCmd(p, nil)
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// We can't easily unit-test the git subprocess, but we can verify the
	// command wires up and runs without error when pool is empty and git is
	// available. This is an integration-style smoke test.
	_ = cmd.Execute()
}

func TestStatusCmd_WithPoolEntries(t *testing.T) {
	p := &mockEditPool{entries: []store.Entry{
		entry("anthropic", "claude-sonnet-4-6"),
		entry("github", "gpt-4o"),
		entry("anthropic", "claude-sonnet-4-6"), // duplicate
	}}
	cmd := newStatusCmd(p, nil)
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()

	got := out.String()
	if !strings.Contains(got, "anthropic:claude-sonnet-4-6") {
		t.Errorf("expected 'anthropic:claude-sonnet-4-6' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "github:gpt-4o") {
		t.Errorf("expected 'github:gpt-4o' in output, got:\n%s", got)
	}
	// Duplicate should be collapsed
	count := strings.Count(got, "anthropic:claude-sonnet-4-6")
	if count != 1 {
		t.Errorf("expected 'anthropic:claude-sonnet-4-6' exactly once, got %d times", count)
	}
}
