// Copyright (c) 2026 Eclipse Foundation AISBL
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package git

import (
	"fmt"
	"io"
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
	// by their full SHA. Hashes must be ordered newest-first. Progress is written
	// to out (use io.Discard to suppress).
	AmendTrailerOnCommits(out io.Writer, hashes []string, key, value string) error
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
// If the trailer already exists on the commit, the new value is merged with
// the existing one (duplicates removed).
// Requires git 2.32+.
func (c *Client) AmendTrailer(key, value string) error {
	// Check if trailer already exists on HEAD.
	existing, err := c.trailerValue("HEAD", key)
	if err != nil {
		return err
	}
	// Merge with existing if present.
	if existing != "" {
		value = mergeTrailerValues(existing, value)
	}
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
// Progress output is written to out.
func (c *Client) AmendTrailerOnCommits(out io.Writer, hashes []string, key, value string) error {
	if len(hashes) == 0 {
		return nil
	}

	// Fast path: if only HEAD is selected, use simple amend.
	head, err := c.headHash()
	if err != nil {
		return err
	}
	if len(hashes) == 1 && hashes[0] == head {
		fmt.Fprintf(out, "Amending %s...\n", head[:7])
		err := c.AmendTrailer(key, value)
		if err == nil {
			fmt.Fprintf(out, "Amended %s\n", head[:7])
		}
		return err
	}

	// Build a set of short hashes (first 7 chars) and pre-fetch current trailer values.
	hashSet := make(map[string]bool, len(hashes))
	trailersByShort := make(map[string]string) // map short hash -> merged trailer value
	for _, h := range hashes {
		if len(h) >= 7 {
			short := h[:7]
			hashSet[short] = true
			// Get current trailer value for this commit.
			existing, _ := c.trailerValue(h, key)
			merged := value
			if existing != "" {
				merged = mergeTrailerValues(existing, value)
			}
			trailersByShort[short] = merged
		}
	}

	// Find the oldest commit (last in the slice since hashes are newest-first).
	oldest := hashes[len(hashes)-1]

	// Write a temporary awk-based editor script that inserts exec lines.
	script, err := c.writeRebaseScript(hashSet, key, trailersByShort)
	if err != nil {
		return err
	}
	defer os.Remove(script)

	fmt.Fprintf(out, "Amending %d commit(s)...\n", len(hashes))

	// Run interactive rebase with our custom sequence editor.
	cmd := exec.Command("git", "rebase", "-i", oldest+"^")
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR="+script)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		// Attempt to abort the rebase on failure.
		_ = exec.Command("git", "rebase", "--abort").Run()
		return fmt.Errorf("rebase failed: %w", err)
	}
	
	fmt.Fprintf(out, "Successfully amended %d commit(s)\n", len(hashes))
	return nil
}

func (c *Client) headHash() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("resolving HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) writeRebaseScript(hashSet map[string]bool, key string, trailersByShort map[string]string) (string, error) {
	// Build awk patterns that match selected commits.
	var patterns []string
	for short := range hashSet {
		// Get the merged trailer value for this commit.
		value := trailersByShort[short]
		// Escape for embedding inside an awk double-quoted string.
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
		trailer := fmt.Sprintf("%s: %s", key, escaped)
		patterns = append(patterns, fmt.Sprintf(`/^pick %s/ { print; print "exec echo Amending %s..."; print "exec git commit --amend --no-edit --trailer \"%s\""; print "exec echo Amended %s"; next }`, short, short, trailer, short))
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

// trailerValue extracts the value of a trailer from a commit.
// Returns empty string if the trailer doesn't exist.
func (c *Client) trailerValue(ref, key string) (string, error) {
	// Use git log with a custom format to extract trailers.
	// %(trailers) includes all trailers, we parse them to find the key.
	out, err := exec.Command("git", "log", "-1", "--format=%(trailers)", ref).Output()
	if err != nil {
		return "", nil // Ref might not exist, return empty
	}
	trailers := string(out)
	for _, line := range strings.Split(trailers, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1]), nil
		}
	}
	return "", nil
}

// mergeTrailerValues combines two comma-separated trailer values,
// removing duplicates while preserving order.
func mergeTrailerValues(existing, new string) string {
	// Split both on commas and collect unique pairs.
	var seenPairs map[string]bool = make(map[string]bool)
	var result []string

	// Add existing values first (preserving their order).
	for _, pair := range strings.Split(existing, ",") {
		pair = strings.TrimSpace(pair)
		if pair != "" && !seenPairs[pair] {
			seenPairs[pair] = true
			result = append(result, pair)
		}
	}

	// Add new values (only if not already present).
	for _, pair := range strings.Split(new, ",") {
		pair = strings.TrimSpace(pair)
		if pair != "" && !seenPairs[pair] {
			seenPairs[pair] = true
			result = append(result, pair)
		}
	}

	return strings.Join(result, ", ")
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
