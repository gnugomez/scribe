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

	cmd := &cobra.Command{
		Use:   "amend",
		Short: "Drain the pool and amend HEAD with an Assisted-By trailer",
		Long: `amend reads all entries from the pool, builds an Assisted-By trailer,
amends HEAD, and clears the pool.

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
			trailerValue := strings.Join(deduplicatePairs(entries), ", ")

			if dryRun {
				fmt.Fprintf(out, "Assisted-By: %s  [dry-run]\n", trailerValue)
				return nil
			}

			if err := g.AmendTrailer("Assisted-By", trailerValue); err != nil {
				return err
			}
			if _, err := p.Drain(); err != nil {
				return fmt.Errorf("clearing pool: %w", err)
			}
			fmt.Fprintf(out, "Assisted-By: %s  %s\n", trailerValue, green("done"))
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the trailer without amending or clearing")
	return cmd
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
