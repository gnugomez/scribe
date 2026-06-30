<!--
  ~ Copyright (c) 2026 Eclipse Foundation AISBL
  ~ 
  ~ This program and the accompanying materials are made available under the
  ~ terms of the Eclipse Public License 2.0 which is available at
  ~ http://www.eclipse.org/legal/epl-2.0.
  ~ 
  ~ SPDX-License-Identifier: EPL-2.0
-->

# scribe 🪶

`scribe` maintains a **per-repo pool** of tool usage events. Each time the harness invokes a tool, a hook adds an entry to the pool.

Run `scribe amend` to drain the pool and annotate one or more commits with:

```
Assisted-By: anthropic:claude-sonnet-4-6, github:gpt-4o
```

You'll be prompted to select which models to include and which commits to annotate. If a commit already has an `Assisted-By` trailer, new models are merged with existing ones to avoid duplicates.

The pool is stored at `.git/scribe/pool.jsonl` — a single file shared across all branches, local to the repo, inside `.git/` so it is never committed and requires no `.gitignore` entry.

## Installation

```bash
go install github.com/gnugomez/scribe@latest
```

## Usage

In order to capture tool usage, you need to set up hooks for your AI tools (see below). Once that's done, the typical workflow is:

Run `scribe amend` to annotate your commits with the tools you used. This is a manual step so you can choose which models to include and which commits to annotate. You'll see interactive pickers for both selections.

```bash
scribe amend
```

By default, unpushed commits (or commits since the fork point on new branches) are offered for selection. Use `scribe amend --help` to see options for controlling the commit range, skipping the interactive pickers, or previewing changes before applying them.

Other useful commands:

```bash
scribe pool   # View captured tool events
scribe clear  # Discard pool without amending
scribe amend --help  # See all options (dry-run, since, all, etc.)
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

## Pool design

  The pool is a **single file** at `.git/scribe/pool.jsonl` shared across all branches — just like git's staging area. When you switch branches carrying uncommitted work, the pool entries follow you. The pool is cleared explicitly by `scribe amend` (after annotating) or `scribe clear` (to discard).

### Trailer merging

  When you run `scribe amend`, the new models are merged with any existing `Assisted-By` trailers on the target commits. Duplicate vendor:model pairs are automatically deduplicated. This means you can safely re-run `scribe amend` on the same commits without creating duplicate entries.

  > [!TIP]
  > If you discard your work (e.g. `git reset --hard`) without committing, run `scribe clear` to remove the now-irrelevant pool entries.

---

## Workflow

```
# Work session with your AI tools:
# → hooks fire automatically as each tool writes files

git add -A
git commit -m "feat: implement user auth"

scribe amend  # interactively select models and commits to annotate
```

---

## Adding support for a new AI tool

Adding a new hook format requires only a new vendor hook package:

1. Create `hook/<vendor>/parser.go`
2. Implement the `hook.Parser` interface (`Name()` + `Parse()`)
3. Self-register via `func init() { hook.Register(&Parser{}) }`
4. Add a blank import in `cmd/root.go` for `github.com/gnugomez/scribe/hook/<vendor>`

The core (`cmd/amend.go`, `cmd/hook.go`) never needs to change.

---

## Pool file

`.git/scribe/pool.jsonl` — one JSON object per line:

```json
{"timestamp":"2026-04-13T10:00:00Z","vendor":"anthropic","model":"claude-sonnet-4-6"}
{"timestamp":"2026-04-13T10:01:00Z","vendor":"github","model":"gpt-4o"}
```

With `scribe pool --debug`, each entry also shows `model source` so you can see where attribution came from: `payload`, `session`, `flag`, or `default`.

---

Copyright 2026 the Eclipse Foundation AISBL
