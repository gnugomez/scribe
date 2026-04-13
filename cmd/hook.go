package cmd

import (
	"fmt"
	"os"

	"github.com/jordi-jordi/scribe/internal/hook"
	"github.com/jordi-jordi/scribe/internal/pool"
	"github.com/spf13/cobra"
)

func newHookCmd(p pool.Pool, poolPath string) *cobra.Command {
	var vendor, model, format string

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Receive an AI tool hook event and add it to the pool",
		Long: `hook reads a JSON payload from stdin (sent by Claude Code, Copilot Chat,
or another supported tool) and adds an entry to the repo-local pool.

The model name is extracted from the payload when available, with --model
as a fallback. This command always exits 0 so it never blocks the calling tool.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if poolPath == "" {
				// Not in a git repo — silently exit so we don't disrupt the tool.
				return nil
			}

			parser, ok := hook.Get(format)
			if !ok {
				fmt.Fprintf(os.Stderr, "scribe hook: %v\n", hook.ErrUnknownFormat(format))
				return nil
			}

			entries, err := parser.Parse(os.Stdin, vendor, model)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scribe hook: parse error: %v\n", err)
				return nil
			}
			if len(entries) == 0 {
				return nil
			}

			if err := p.Add(entries...); err != nil {
				fmt.Fprintf(os.Stderr, "scribe hook: pool error: %v\n", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&vendor, "vendor", "", "AI vendor — used as fallback if not in payload (e.g. anthropic, github)")
	cmd.Flags().StringVar(&model, "model", "", "Model name — used as fallback if not in payload or env")
	cmd.Flags().StringVar(&format, "format", "claude-code", fmt.Sprintf("Hook payload format (available: %v)", hook.Names()))

	return cmd
}
