// Copyright (c) 2026 Jordi Gómez Hidalgo
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/gnugomez/scribe/store"
	"github.com/spf13/cobra"
)

var (
	stagedStyle    = color.New(color.FgGreen)
	unstagedStyle  = color.New(color.FgRed)
	untrackedStyle = color.New(color.FgCyan, color.Faint)
	scribeStyle    = color.New(color.FgYellow)
)

type parsedStatus struct {
	branch    string
	upstream  string
	ahead     int
	behind    int
	staged    []fileEntry
	unstaged  []fileEntry
	untracked []string
}

type fileEntry struct {
	code string
	path string
}

func newStatusCmd(p store.EditPool, repoErr error) *cobra.Command {
	return &cobra.Command{
		Use:          "status",
		Short:        "Show working tree status with pending scribe annotation",
		Long:         `status combines git status with the current scribe pool, showing staged, unstaged, and untracked changes alongside any pending Assisted-By annotation.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repoErr != nil {
				noRepo()
			}

			out := cmd.OutOrStdout()

			gs, err := parseGitStatus()
			if err != nil {
				return fmt.Errorf("git status: %w", err)
			}

			// Branch line
			branchLabel := bold(gs.branch)
			if gs.upstream != "" {
				branchLabel += fmt.Sprintf("  %s  %s", dim("→"), dim(gs.upstream))
				if gs.ahead > 0 || gs.behind > 0 {
					branchLabel += fmt.Sprintf("  %s", dim(fmt.Sprintf("(+%d -%d)", gs.ahead, gs.behind)))
				}
			}
			fmt.Fprintf(out, "%s  %s\n", dim("branch"), branchLabel)

			anyChanges := len(gs.staged) > 0 || len(gs.unstaged) > 0 || len(gs.untracked) > 0

			if len(gs.staged) > 0 {
				fmt.Fprintf(out, "\n%s\n", dim("staged"))
				for _, e := range gs.staged {
					fmt.Fprintf(out, "  %s  %s\n", stagedStyle.Sprint(expandCode(e.code)), e.path)
				}
			}

			if len(gs.unstaged) > 0 {
				fmt.Fprintf(out, "\n%s\n", dim("not staged"))
				for _, e := range gs.unstaged {
					fmt.Fprintf(out, "  %s  %s\n", unstagedStyle.Sprint(expandCode(e.code)), e.path)
				}
			}

			if len(gs.untracked) > 0 {
				fmt.Fprintf(out, "\n%s\n", dim("untracked"))
				for _, f := range gs.untracked {
					fmt.Fprintf(out, "  %s\n", untrackedStyle.Sprint(f))
				}
			}

			entries, err := p.Peek()
			if err != nil {
				return fmt.Errorf("reading pool: %w", err)
			}

			if len(entries) > 0 {
				fmt.Fprintf(out, "\n%s\n", dim("scribe"))
				for _, pair := range deduplicatePairs(entries) {
					fmt.Fprintf(out, "  %s\n", scribeStyle.Sprint(pair))
				}
			} else if !anyChanges {
				fmt.Fprintf(out, "  %s\n", dim("clean"))
			}

			return nil
		},
	}
}

func parseGitStatus() (*parsedStatus, error) {
	out, err := exec.Command("git", "status", "--porcelain", "--branch").Output()
	if err != nil {
		return nil, err
	}

	gs := &parsedStatus{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}

		if strings.HasPrefix(line, "## ") {
			parseBranchLine(line[3:], gs)
			continue
		}

		if len(line) < 4 {
			continue
		}

		x := string(line[0])
		y := string(line[1])
		path := line[3:]

		// Renames: "old -> new" — show the new name only.
		if idx := strings.Index(path, " -> "); idx != -1 {
			path = path[idx+4:]
		}

		if x == "?" && y == "?" {
			gs.untracked = append(gs.untracked, path)
			continue
		}

		if x != " " && x != "?" {
			gs.staged = append(gs.staged, fileEntry{code: x, path: path})
		}
		if y != " " && y != "?" {
			gs.unstaged = append(gs.unstaged, fileEntry{code: y, path: path})
		}
	}

	return gs, scanner.Err()
}

// parseBranchLine parses the branch header produced by git status --porcelain --branch.
// Examples:
//
//	"main"
//	"main...origin/main"
//	"main...origin/main [ahead 1]"
//	"main...origin/main [behind 2]"
//	"main...origin/main [ahead 1, behind 2]"
//	"No commits yet on main"
//	"HEAD (no commits yet)"
func parseBranchLine(s string, gs *parsedStatus) {
	// "No commits yet on <branch>"
	if after, ok := strings.CutPrefix(s, "No commits yet on "); ok {
		gs.branch = after
		return
	}

	// Strip trailing divergence info: " [ahead N, behind M]"
	var info string
	if idx := strings.Index(s, " ["); idx != -1 {
		info = s[idx+2 : len(s)-1]
		s = s[:idx]
	}

	head, upstream, hasUpstream := strings.Cut(s, "...")
	gs.branch = head
	if hasUpstream {
		gs.upstream = upstream
	}

	if info == "" {
		return
	}
	for part := range strings.SplitSeq(info, ", ") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "ahead "):
			gs.ahead, _ = strconv.Atoi(strings.TrimPrefix(part, "ahead "))
		case strings.HasPrefix(part, "behind "):
			gs.behind, _ = strconv.Atoi(strings.TrimPrefix(part, "behind "))
		}
	}
}

// expandCode converts a single-letter git porcelain status code to a
// padded human-readable word so columns align across entries.
func expandCode(code string) string {
	words := map[string]string{
		"M": "modified  ",
		"A": "added     ",
		"D": "deleted   ",
		"R": "renamed   ",
		"C": "copied    ",
		"U": "unmerged  ",
		"T": "typechange",
	}
	if w, ok := words[code]; ok {
		return w
	}
	return code
}
