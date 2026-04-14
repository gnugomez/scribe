# scribe 🪶

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

In order to capture tool usage, you need to set up hooks for your AI tools (see below). Once that's done, the typical workflow is:

Just run `scribe amend` after your commit to annotate it with the tools you used. This is a manual step so you can choose which commits to annotate and when.

```bash
scribe amend
```
This could be paired with pre-commit hooks.


Some other useful commands:

```bash
scribe amend --dry-run   # Preview the annotation without clearing the pool
scribe pool # View the list of captured tool events in the pool
scribe clear # Discard pool without amending
```

---

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

> [!WARNING]
> Right now there's an open issue where SessionStart hooks aren't following the specification, not including the `model`, [details here](https://github.com/microsoft/vscode/issues/309510)

VS Code Copilot Chat supports **Agent Hooks** (preview) that fire when the agent uses tools.

Create `.copilot/hooks/scribe.json` in your repo or globally at `~/.copilot/hooks/scribe.json` with the following content:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "command": "scribe hook --vendor github --format copilot",
        "type": "command"
      }
    ],
    "SessionStart": [
      {
        "command": "scribe hook --vendor github --format copilot",
        "type": "command"
      }
    ]
  }
}
```

`scribe` reads the model from the payload. If a `PostToolUse` payload doesn't include a model, `scribe` will reuse the latest known model from that same `session_id`.

> [!WARNING]
> Copilot hook payloads may not always include model metadata. The `SessionStart` hook lets `scribe` capture the model at session start so it can be reused when later tool events omit it.

> [!NOTE]
> The VS Code Agent Hooks API is in preview. See the [VS Code Copilot hooks documentation](https://code.visualstudio.com/docs/copilot/customization/hooks) for the latest config format and payload schema.

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
