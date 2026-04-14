package cmd

import (
	"fmt"
	"os"

	"github.com/gnugomez/scribe/git"
	"github.com/gnugomez/scribe/store/jsonl"
	"github.com/spf13/cobra"

	// Side-effect imports: register hook parsers.
	_ "github.com/gnugomez/scribe/hook/claude"
	_ "github.com/gnugomez/scribe/hook/copilot"
)

// Execute builds the command tree and runs it.
func Execute() error {
	root := &cobra.Command{
		Use:   "scribe",
		Short: "Track tool usage and annotate git commits with Assisted-By trailers",
		Long: `scribe maintains a per-repo pool of tool usage events.
Each time the harness invokes a tool, a hook
adds an entry to the pool. Run 'scribe amend' to drain the pool and annotate
the current commit with an Assisted-By trailer.`,
	}

	// Resolve git repo root and store paths. Commands that need git context
	// will fail gracefully if not in a repo.
	repoRoot, repoErr := git.RepoRoot()
	poolPath := ""
	sessionPath := ""
	if repoErr == nil {
		poolPath = git.PoolPath(repoRoot)
		sessionPath = git.SessionPath(repoRoot)
	}

	editPool := jsonl.NewPool(poolPath)
	sessionPool := jsonl.NewPool(sessionPath)
	gitClient := git.NewClient()

	root.AddCommand(
		newHookCmd(editPool, sessionPool, poolPath),
		newAmendCmd(editPool, gitClient, repoErr),
		newPoolCmd(editPool, poolPath),
		newClearCmd(editPool, poolPath),
	)

	return root.Execute()
}

// noRepo prints an error and exits when a command requires a git repository.
func noRepo() {
	fmt.Fprintln(os.Stderr, "error: must be run inside a git repository")
	os.Exit(1)
}
