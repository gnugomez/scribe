# scribe — Agent Guidelines

scribe is a Go CLI that captures AI tool-use events via hooks and annotates git commits with `Assisted-By:` trailers. See [README.md](README.md) for usage and hook setup.

## Build and Test

```bash
go build ./...
go test ./...
```

Requires Go 1.25+. No Makefile.

## Architecture

```
main.go → cmd/          # cobra commands; composition root in root.go
          git/          # thin git wrapper (RepoRoot, PoolPath, CurrentBranch, HeadHash, AmendTrailer)
          hook/         # Parser interface + self-registering vendor parsers
            claude/     # Anthropic parser (init() registers itself)
            copilot/    # GitHub Copilot parser (init() registers itself)
          store/        # Entry type + EditPool/SessionPool interfaces
            jsonl/      # JSONL file-backed implementation
```

`cmd/root.go` is the composition root — it resolves git paths, creates `jsonl.Pool` instances, and injects them into each command constructor.

## Key Conventions

**Two pools, different lifecycles.**
- *Edit pool* (`.git/scribe/<branch>/pool.jsonl`): drained on every `amend`/`clear`. Scoped to the current branch — checking out a different branch uses a separate pool automatically.
- *Session pool* (`.git/scribe/sessions.jsonl`): append-only, never cleared; used to back-fill the model when a `PostToolUse` payload lacks it.

Both files live inside `.git/` — local to the repo, never committed, no `.gitignore` needed.

**Self-registering parsers.** Each vendor parser calls `hook.Register()` in its `init()`. `cmd/root.go` blank-imports them as side effects. To add a new vendor, create `hook/<vendor>/parser.go` that self-registers and blank-import it in `cmd/root.go`.

**Dependency injection via constructors.** Commands receive dependencies as arguments (`newHookCmd(editPool, sessionPool, ...)`). No global mutable state beyond the parser registry.

**Interface segregation.** Accept the narrowest interface needed: `store.EditPool` (Add/Peek/Drain/Clear) or `store.SessionPool` (Add/Peek). `jsonl.Pool` satisfies both.

**Stale-pool guard.** A sentinel file `.git/scribe/<branch>/pool-head` records the HEAD commit hash whenever entries are written. On `hook` (before adding) and `amend` (before reading), `poolIsStale(headPath, hashFn)` compares the stored hash to current HEAD. A mismatch auto-clears the pool so a `git reset --hard` or branch switch can't drag old entries onto the wrong commit. `hashFn` is injected so the guard is fully unit-testable without git.

**Known false positive.** A manual `git commit --amend` (e.g. to fix a commit message typo) changes HEAD and triggers a false stale detection. Workaround: run `scribe amend` before any manual amend, or accept that the pool will be cleared.

 Parse failures and pool errors are printed to stderr but not returned, so they never interrupt the calling AI tool.

**Error wrapping.** Use `fmt.Errorf("context: %w", err)`. Commands use `cobra.Command.RunE` (not `Run`).
