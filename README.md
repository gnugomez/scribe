# scribe

> Tool usage tracking for your git commits.

`scribe` maintains a **per-repo pool** of tool usage events. Each time the harness invokes a tool, a hook adds an entry to the pool.

Run `scribe amend` to drain the pool and annotate the current commit with:

```
Assisted-By: anthropic:claude-sonnet-4-6, github:gpt-4o
```

The pool is stored at `.git/scribe/pool.jsonl` — local to the repo, inside `.git/` so it is never committed and requires no `.gitignore` entry.

## Installation

```bash
go install github.com/gnugomez/scribe@latest
```

## Usage

```bash
# See what's in the pool right now
scribe pool

# Show full hook payload details for debugging
scribe pool --debug

# Preview the Assisted-By trailer without modifying anything
scribe amend --dry-run

# Amend HEAD with the trailer and clear the pool
scribe amend

# Discard pool without amending (e.g. after a commit you don't want to annotate)
scribe clear
```

---

## Tool Configuration

  `scribe` no longer includes a `setup` command. Configuration is intentionally manual and copy-pasteable.

### Claude Code CLI

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
    ],
    "SessionStart": [
      {
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

  `scribe` reads the model from the payload. If a `PostToolUse` payload doesn't include a model, `scribe` will reuse the latest known model from that same `session_id`.

---

### GitHub Copilot Chat

VS Code Copilot Chat supports **Agent Hooks** (preview) that fire when the agent uses tools.

Add to your user or workspace `settings.json`:

```json
{
  "github.copilot.agent.hooks": {
    "postToolUse": {
      "command": "scribe hook --vendor github --format copilot"
    },
    "sessionStart": {
      "command": "scribe hook --vendor github --format copilot"
    }
  }
}
```

`scribe` reads the model from the payload. If a `postToolUse` payload doesn't include a model, `scribe` will reuse the latest known model from that same `session_id`.

> **Warning:** Copilot hook payloads may not always include model metadata. To improve attribution, configure a `sessionStart` hook so `scribe` can reuse the model seen at session start when later tool events omit it.

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

Adding a new hook format requires only a new vendor hook package:

1. Create `vendors/<toolname>/hook/parser.go`
2. Implement the `hook.Parser` interface (`Name()` + `Parse()`)
3. Self-register via `func init() { hook.Register(&Parser{}) }`
4. Add a blank import in `cmd/root.go` for `github.com/gnugomez/scribe/vendors/<toolname>/hook`

The core (`cmd/amend.go`, `cmd/hook.go`) never needs to change.

---

## Pool file

  `.git/scribe/pool.jsonl` — one JSON object per line:

```json
{"timestamp":"2026-04-13T10:00:00Z","vendor":"anthropic","model":"claude-sonnet-4-6"}
{"timestamp":"2026-04-13T10:01:00Z","vendor":"github","model":"gpt-4o"}
```

  With `scribe pool --debug`, each entry also shows `model source` so you can see where attribution came from: `payload`, `session`, `flag`, or `default`.
