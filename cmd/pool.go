package cmd

import (
	"fmt"

	"github.com/jordi-jordi/scribe/internal/pool"
	"github.com/spf13/cobra"
)

func newPoolCmd(p pool.Pool, poolPath string) *cobra.Command {
	return &cobra.Command{
		Use:   "pool",
		Short: "Show the current pool contents",
		Long:  `pool prints all AI tool usage events accumulated since the last amend.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if poolPath == "" {
				noRepo()
			}

			entries, err := p.Peek()
			if err != nil {
				return fmt.Errorf("reading pool: %w", err)
			}
			if len(entries) == 0 {
				fmt.Println("Pool is empty.")
				return nil
			}

			for _, e := range entries {
				fmt.Printf("%s  %s:%s\n",
					e.Timestamp.Format("2006-01-02T15:04:05Z"),
					e.Vendor, e.Model,
				)
			}
			return nil
		},
	}
}
