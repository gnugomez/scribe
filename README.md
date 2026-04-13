# scribe

> AI attribution tracking for your git commits.

`scribe` maintains a **per-repo pool** of AI tool usage events. Every time an AI tool invokes the LLM, a hook adds an entry to the pool. When you're ready to commit or after committing, run `scribe amend` to drain the pool and annotate the commit with:

```
Assisted-By: anthropic:claude-sonnet-4-6, github:gpt-4o
```

The pool is stored at `.git/scribe/pool.jsonl` — local to the repo, inside `.git/` so it is never committed and requires no `.gitignore` entry.

## Installation

```bash
git clone https://github.com/jordi-jordi/scribe ~/Projects/Repos/scribe
cd ~/Projects/Repos/scribe
go install .
```

Ensure `~/go/bin` is in your `PATH`:

```bash
export PATH="$HOME/go/bin:$PATH"
```

## Usage

```bash
# See what's in the pool right now
scribe pool

# Preview the Assisted-By trailer without modifying anything
scribe amend --dry-run

# Amend HEAD with the trailer and clear the pool
scribe amend

# Discard pool without amending (e.g. after a commit you don't want to annotate)
scribe clear
```

---

## Tool Configuration

### Claude Code CLI (recommended — automatic)

Claude Code fires `PostToolUse` hooks after every file write. Add the following to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "scribe hook --vendor anthropic"
          }
        ]
      }
    ]
  }
}
```

**No `--model` flag needed.** `scribe` reads the model from the hook payload automatically. If the Claude Code version you're using doesn't include the model in the payload, it falls back to the `CLAUDE_MODEL` environment variable.

---

### GitHub Copilot Chat — VS Code Agent Hooks (automatic)

VS Code Copilot Chat supports **Agent Hooks** (preview) that fire when the agent uses tools.

Add to your user or workspace `settings.json`:

```json
{
  "github.copilot.agent.hooks": {
    "postToolUse": {
      "command": "scribe hook --vendor github --format copilot"
    }
  }
}
```

**No `--model` flag needed.** `scribe` reads the model from the hook payload. Falls back to the `COPILOT_MODEL` or `GITHUB_COPILOT_MODEL` environment variable if the payload doesn't include it.

> **Note:** The VS Code Agent Hooks API is in preview. See the [VS Code Copilot hooks documentation](https://code.visualstudio.com/docs/copilot/customization/hooks) for the latest config format and payload schema.

---

## Workflow

```
# Work session with Claude Code and Copilot Chat:
# → hooks fire automatically as each tool writes files

git add -A
git commit -m "feat: implement user auth"

scribe amend --dry-run   # preview: Assisted-By: anthropic:claude-sonnet-4-6, github:gpt-4o
scribe amend             # apply and clear pool
```

---

## Adding support for a new AI tool

Adding a new hook format requires only a new package:

1. Create `internal/hook/<toolname>/parser.go`
2. Implement the `hook.Parser` interface (`Name()` + `Parse()`)
3. Self-register via `func init() { hook.Register(&Parser{}) }`
4. Add a blank import in `cmd/root.go`

The core (`cmd/amend.go`, `cmd/hook.go`) never needs to change.

---

## Pool file

`.git/scribe/pool.jsonl` — one JSON object per line:

```json
{"timestamp":"2026-04-13T10:00:00Z","vendor":"anthropic","model":"claude-sonnet-4-6"}
{"timestamp":"2026-04-13T10:01:00Z","vendor":"github","model":"gpt-4o"}
```
