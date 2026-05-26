package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// logFormat is the --pretty format used to list commits as "<hash> <subject>".
const logFormat = "--pretty=format:%H %s"

// Commit represents a single git commit.
type Commit struct {
	Hash    string // full SHA
	Subject string // first line of commit message
}

// Git performs the VCS operations scribe needs.
type Git interface {
	// AmendTrailer appends "key: value" as a trailer to HEAD without editing
	// the commit message.
	AmendTrailer(key, value string) error

	// UnpushedCommits returns commits ahead of the remote tracking branch,
	// newest first. For branches without an upstream, returns commits since
	// the fork point from the default branch.
	UnpushedCommits() ([]Commit, error)

	// CommitsSince returns commits between ref (exclusive) and HEAD, newest
	// first. ref can be any valid git revision (hash, HEAD~5, branch, etc.).
	CommitsSince(ref string) ([]Commit, error)

	// AmendTrailerOnCommits adds the trailer to one or more commits identified
	// by their full SHA. Hashes must be ordered newest-first.
	AmendTrailerOnCommits(hashes []string, key, value string) error
}

// Client is the exec-based Git implementation.
type Client struct{}

// NewClient returns a new Client.
func NewClient() *Client { return &Client{} }

// RepoRoot returns the absolute path of the nearest git repository root,
// or an error if the working directory is not inside a git repo.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// PoolPath returns the path for tool usage events in the repo rooted at root.
// Cleared on each amend/clear.
func PoolPath(root string) string {
	return filepath.Join(root, ".git", "scribe", "pool.jsonl")
}

// SessionPath returns the path for session model data. This file is never
// cleared so session model lookups survive amend/clear cycles.
func SessionPath(root string) string {
	return filepath.Join(root, ".git", "scribe", "sessions.jsonl")
}

// AmendTrailer runs git commit --amend --no-edit --trailer "key: value".
// Requires git 2.32+.
func (c *Client) AmendTrailer(key, value string) error {
	trailer := fmt.Sprintf("%s: %s", key, value)
	cmd := exec.Command("git", "commit", "--amend", "--no-edit", "--trailer", trailer)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git amend failed: %w\n%s", err, out)
	}
	return nil
}

// UnpushedCommits returns commits between the upstream tracking ref and HEAD,
// newest first. If no upstream is configured (e.g. new branch), it falls back
// to commits since the fork point from main/master.
//
// Uses: git rev-parse --abbrev-ref @{upstream}  (check upstream exists)
//       git log @{upstream}..HEAD               (list commits ahead)
func (c *Client) UnpushedCommits() ([]Commit, error) {
	// Check if there is an upstream configured (exit 0 = yes).
	hasUpstream := exec.Command("git", "rev-parse", "--abbrev-ref", "@{upstream}").Run() == nil

	if hasUpstream {
		// Upstream exists — list all commits between upstream and HEAD.
		out, err := exec.Command("git", "log", "@{upstream}..HEAD", logFormat).Output()
		if err != nil {
			return nil, fmt.Errorf("listing unpushed commits: %w", err)
		}
		return parseCommitLog(string(out)), nil
	}

	// No upstream — find fork point from default branch.
	base := c.forkBase()
	if base == "" {
		return nil, nil
	}
	out, err := exec.Command("git", "log", base+"..HEAD", logFormat).Output()
	if err != nil {
		return nil, fmt.Errorf("listing commits since fork: %w", err)
	}
	return parseCommitLog(string(out)), nil
}

// CommitsSince returns all commits between ref (exclusive) and HEAD (inclusive),
// newest first. ref can be any valid git revision: a SHA, HEAD~5, a branch, etc.
//
// Uses: git log <ref>..HEAD
func (c *Client) CommitsSince(ref string) ([]Commit, error) {
	out, err := exec.Command("git", "log", ref+"..HEAD", logFormat).Output()
	if err != nil {
		return nil, fmt.Errorf("listing commits since %s: %w", ref, err)
	}
	return parseCommitLog(string(out)), nil
}

// forkBase returns the merge-base of HEAD with main or master.
// Returns empty string if neither exists.
func (c *Client) forkBase() string {
	for _, branch := range []string{"main", "master"} {
		if out, err := exec.Command("git", "merge-base", "HEAD", branch).Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// AmendTrailerOnCommits adds the trailer to the given commits using an
// interactive rebase with exec commands. Hashes must be ordered newest-first.
func (c *Client) AmendTrailerOnCommits(hashes []string, key, value string) error {
	if len(hashes) == 0 {
		return nil
	}

	// Fast path: if only HEAD is selected, use simple amend.
	head, err := c.headHash()
	if err != nil {
		return err
	}
	if len(hashes) == 1 && hashes[0] == head {
		return c.AmendTrailer(key, value)
	}

	// Build a set of short hashes (first 7 chars) for matching in the rebase todo.
	hashSet := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		if len(h) >= 7 {
			hashSet[h[:7]] = true
		}
	}

	trailer := fmt.Sprintf("%s: %s", key, value)

	// Find the oldest commit (last in the slice since hashes are newest-first).
	oldest := hashes[len(hashes)-1]

	// Write a temporary awk-based editor script that inserts exec lines.
	script, err := c.writeRebaseScript(hashSet, trailer)
	if err != nil {
		return err
	}
	defer os.Remove(script)

	// Run interactive rebase with our custom sequence editor.
	cmd := exec.Command("git", "rebase", "-i", oldest+"^")
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR="+script)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Attempt to abort the rebase on failure.
		_ = exec.Command("git", "rebase", "--abort").Run()
		return fmt.Errorf("rebase failed: %w\n%s", err, out)
	}
	return nil
}

func (c *Client) headHash() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("resolving HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) writeRebaseScript(hashSet map[string]bool, trailer string) (string, error) {
	// Escape trailer for embedding inside an awk double-quoted string.
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(trailer)

	// Build awk patterns that match selected commits.
	var patterns []string
	for short := range hashSet {
		patterns = append(patterns, fmt.Sprintf(`/^pick %s/ { print; print "exec git commit --amend --no-edit --trailer \"%s\""; next }`, short, escaped))
	}
	awkBody := strings.Join(patterns, "\n") + "\n{ print }"
	scriptContent := fmt.Sprintf("#!/bin/sh\nawk '%s' \"$1\" > \"$1.tmp\" && mv \"$1.tmp\" \"$1\"\n", awkBody)

	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	f, err := os.CreateTemp(tmpDir, "scribe-rebase-*.sh")
	if err != nil {
		return "", fmt.Errorf("creating rebase script: %w", err)
	}
	if _, err := f.WriteString(scriptContent); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0o700); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func parseCommitLog(output string) []Commit {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	commits := make([]Commit, 0, len(lines))
	for _, line := range lines {
		if hash, subject, ok := strings.Cut(line, " "); ok {
			commits = append(commits, Commit{Hash: hash, Subject: subject})
		}
	}
	return commits
}
