package cmd

import (
	"fmt"
	"os"

	"github.com/jordi-jordi/scribe/internal/git"
	"github.com/jordi-jordi/scribe/internal/pool/jsonl"
	"github.com/spf13/cobra"

	// Side-effect imports: register hook parsers.
	_ "github.com/jordi-jordi/scribe/internal/hook/claudecode"
	_ "github.com/jordi-jordi/scribe/internal/hook/copilot"
)

// Execute builds the command tree and runs it.
func Execute() error {
	root := &cobra.Command{
		Use:   "scribe",
		Short: "Track AI tool usage and annotate git commits with Assisted-By trailers",
		Long: `scribe maintains a per-repo pool of AI tool usage events.
Each time an AI tool (Claude Code, Copilot Chat, ...) invokes the LLM, a hook
adds an entry to the pool. Run 'scribe amend' to drain the pool and annotate
the current commit with an Assisted-By trailer.`,
	}

	// Resolve git repo root and pool path. Commands that need git context
	// will fail gracefully if not in a repo.
	repoRoot, repoErr := git.RepoRoot()
	poolPath := ""
	if repoErr == nil {
		poolPath = git.PoolPath(repoRoot)
	}

	p := jsonl.NewPool(poolPath)
	gitClient := git.NewClient()

	root.AddCommand(
		newHookCmd(p, poolPath),
		newAmendCmd(p, gitClient, repoErr),
		newPoolCmd(p, poolPath),
		newClearCmd(p, poolPath),
		newSetupCmd(),
	)

	return root.Execute()
}

// noRepo prints an error and exits when a command requires a git repository.
func noRepo() {
	fmt.Fprintln(os.Stderr, "error: must be run inside a git repository")
	os.Exit(1)
}
