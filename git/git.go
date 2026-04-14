package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git performs the VCS operations scribe needs.
type Git interface {
	// AmendTrailer appends "key: value" as a trailer to HEAD without editing
	// the commit message.
	AmendTrailer(key, value string) error
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

// PoolPath returns the path for per-commit tool usage events for the repo
// rooted at root. Cleared on each amend/clear.
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
