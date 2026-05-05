package cmd

import (
	"fmt"

	"github.com/gnugomez/scribe/store"
	"github.com/spf13/cobra"
)

func newClearCmd(p store.EditPool, storePath, headPath string) *cobra.Command {
	return &cobra.Command{
		Use:          "clear",
		Short:        "Clear the pool without amending",
		Long:         `clear empties the pool. Use this to discard accumulated events without annotating a commit.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if storePath == "" {
				noRepo()
			}
			entries, err := p.Drain()
			if err != nil {
				return fmt.Errorf("clearing pool: %w", err)
			}
			if headPath != "" {
				removePoolHead(headPath)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cleared %s  %s\n", bold(fmt.Sprintf("%d entries", len(entries))), success("done"))
			return nil
		},
	}
}
