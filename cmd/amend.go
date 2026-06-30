// Copyright (c) 2026 Jordi Gómez Hidalgo
// 
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
// 
// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/gnugomez/scribe/git"
	"github.com/gnugomez/scribe/store"
	"github.com/spf13/cobra"
)

func newAmendCmd(p store.EditPool, g git.Git, repoErr error) *cobra.Command {
	var dryRun bool
	var all bool
	var since string

	cmd := &cobra.Command{
		Use:   "amend",
		Short: "Drain the pool and amend commits with an Assisted-By trailer",
		Long: `amend reads all entries from the pool, shows an interactive picker to select
which models to include in the trailer, then shows a commit picker to choose
which commits to annotate.

By default, unpushed commits are shown (or commits since the fork point on
new branches). Use --since (-s) with any git ref (HEAD~5, a commit hash,
a branch name) to show all commits between that ref and HEAD.

The latest commit is pre-selected; all others are unchecked by default.

Use --all (-y) to skip the model picker and include everything.
Use --dry-run to preview without modifying commits or clearing the pool.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repoErr != nil {
				noRepo()
			}

			out := cmd.OutOrStdout()

			entries, err := p.Peek()
			if err != nil {
				return fmt.Errorf("reading pool: %w", err)
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "pool empty")
				return nil
			}

			// Deduplicate vendor:model pairs preserving insertion order.
			pairs := deduplicatePairs(entries)

			// Interactive selection (unless --all).
			selected := pairs
			if !all && !dryRun && len(pairs) > 1 {
				chosen, err := pickModels(pairs)
				if err != nil {
					return fmt.Errorf("picker: %w", err)
				}
				if chosen == nil {
					// User cancelled.
					fmt.Fprintln(out, "cancelled")
					return nil
				}
				selected = chosen
			}

			trailerValue := strings.Join(selected, ", ")

			// Determine which commits to offer.
			var commits []git.Commit
			if since != "" {
				commits, err = g.CommitsSince(since)
			} else {
				commits, err = g.UnpushedCommits()
			}
			if err != nil {
				return fmt.Errorf("listing commits: %w", err)
			}

			// Pick target commits.
			var targetHashes []string
			if len(commits) == 0 || dryRun {
				// No commits to choose from or dry-run — target HEAD implicitly.
				targetHashes = nil
			} else if len(commits) == 1 {
				// Only one commit — no picker needed.
				targetHashes = []string{commits[0].Hash}
			} else {
				chosen, err := pickCommits(commits)
				if err != nil {
					return fmt.Errorf("commit picker: %w", err)
				}
				if chosen == nil {
					fmt.Fprintln(out, "cancelled")
					return nil
				}
				targetHashes = chosen
			}

			if dryRun {
				fmt.Fprintf(out, "Assisted-By: %s  [dry-run]\n", trailerValue)
				return nil
			}

			// Amend the selected commits.
			if len(targetHashes) == 0 {
				// Default: amend HEAD only.
				if err := g.AmendTrailer("Assisted-By", trailerValue); err != nil {
					return err
				}
			} else {
			if err := g.AmendTrailerOnCommits(out, targetHashes, "Assisted-By", trailerValue); err != nil {
				}
			}

			// Drain only selected pairs from the pool; keep the rest.
			pairSet := make(map[string]struct{}, len(selected))
			for _, s := range selected {
				pairSet[s] = struct{}{}
			}
			if _, err := p.DrainMatching(pairSet); err != nil {
				return fmt.Errorf("draining pool: %w", err)
			}

			fmt.Fprintf(out, "Assisted-By: %s  %s\n", trailerValue, success("done"))
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the trailer without amending or clearing")
	cmd.Flags().BoolVarP(&all, "all", "y", false, "Skip interactive selection and include all models")
	cmd.Flags().StringVarP(&since, "since", "s", "", "Show commits since a git ref (hash, HEAD~5, branch, etc.)")
	return cmd
}

// pickModels shows an interactive picker and returns the selected pairs.
// Returns nil if the user cancelled.
func pickModels(pairs []string) ([]string, error) {
	pk := newPicker(pairs)
	indices, err := pk.Run()
	if err != nil {
		return nil, err
	}
	if indices == nil {
		return nil, nil
	}
	selected := make([]string, len(indices))
	for i, idx := range indices {
		selected[i] = pairs[idx]
	}
	return selected, nil
}

// pickCommits shows an interactive picker for commits and returns the
// selected hashes (newest-first order preserved). Only the latest commit
// (index 0) is pre-selected.
func pickCommits(commits []git.Commit) ([]string, error) {
	labels := make([]string, len(commits))
	defaults := make([]bool, len(commits))
	for i, c := range commits {
		short := c.Hash
		if len(short) > 7 {
			short = short[:7]
		}
		labels[i] = fmt.Sprintf("%s %s", short, c.Subject)
		defaults[i] = (i == 0) // only latest commit selected
	}
	pk := newPickerWithDefaults(labels, defaults)
	pk.header = "Select commits to annotate (space=toggle, a=all, enter=confirm):"
	indices, err := pk.Run()
	if err != nil {
		return nil, err
	}
	if indices == nil {
		return nil, nil
	}
	hashes := make([]string, len(indices))
	for i, idx := range indices {
		hashes[i] = commits[idx].Hash
	}
	return hashes, nil
}

func deduplicatePairs(entries []store.Entry) []string {
	seen := map[string]struct{}{}
	var pairs []string
	for _, e := range entries {
		key := e.Vendor + ":" + e.Model
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			pairs = append(pairs, key)
		}
	}
	return pairs
}
