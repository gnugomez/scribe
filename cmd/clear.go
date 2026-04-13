package cmd

import (
	"fmt"

	"github.com/jordi-jordi/scribe/internal/pool"
	"github.com/spf13/cobra"
)

func newClearCmd(p pool.Pool, poolPath string) *cobra.Command {
	return &cobra.Command{
		Use:          "clear",
		Short:        "Clear the pool without amending",
		Long:         `clear empties the pool. Use this to discard accumulated events without annotating a commit.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if poolPath == "" {
				noRepo()
			}
			if err := p.Clear(); err != nil {
				return fmt.Errorf("clearing pool: %w", err)
			}
			fmt.Println("done")
			return nil
		},
	}
}
