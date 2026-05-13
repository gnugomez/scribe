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

	cmd := &cobra.Command{
		Use:   "amend",
		Short: "Drain the pool and amend HEAD with an Assisted-By trailer",
		Long: `amend reads all entries from the pool, shows an interactive picker to select
which models to include in the trailer, amends HEAD, and drains only the
selected entries from the pool. Unselected entries remain for future commits.

Use --all (-y) to skip the picker and include everything.
Use --dry-run to preview without modifying the commit or clearing the pool.`,
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

			if dryRun {
				fmt.Fprintf(out, "Assisted-By: %s  [dry-run]\n", trailerValue)
				return nil
			}

			if err := g.AmendTrailer("Assisted-By", trailerValue); err != nil {
				return err
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
