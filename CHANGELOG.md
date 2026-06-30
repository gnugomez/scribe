<!--
  ~ Copyright (c) 2026 Jordi Gómez Hidalgo
  ~ 
  ~ This program and the accompanying materials are made available under the
  ~ terms of the Eclipse Public License 2.0 which is available at
  ~ http://www.eclipse.org/legal/epl-2.0.
  ~ 
  ~ SPDX-License-Identifier: EPL-2.0
-->

# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.3.0] - 2026-30-06

### Added

- Multi-commit amendment: `amend` now supports annotating multiple unpushed commits (or commits since fork point on new branches) with an interactive picker
- `--since` (`-s`) flag to control commit range (accepts any git ref: hash, `HEAD~5`, branch, etc.)
- Real-time per-commit progress feedback during batch amendment operations
- Smart trailer merging: new models are merged with existing `Assisted-By` trailers instead of creating duplicates

## [0.2.1] - 2026-21-05

### Fixed

- Payload model now persisted to session pool so it survives amend/clear cycles

## [0.2.0] - 2026-14-05

### Fixed

- Pool reader now handles lines larger than 64 KiB

### Changed

- Single shared pool at `.git/scribe/pool.jsonl` — follows uncommitted work across branches
- `scribe amend` shows an interactive picker when multiple models are in the pool
- Only selected entries are drained; unselected remain for future commits

### Added

- `--all` / `-y` flag on `scribe amend` to skip the picker

### Removed

- Branch-scoped pool isolation
- Stale-pool guard (HEAD sentinel auto-clear)

---

## [0.1.0] — 2026-05-05

### Added

- Initial CLI with `hook`, `amend`, `pool`, and `clear` commands
- Claude Code and GitHub Copilot Chat hook parsers with self-registering vendor architecture
- Session-model fallback: `PostToolUse` events that omit the model are back-filled from the most recent `SessionStart` in the same session
- Coloured terminal output for `amend`, `pool`, and `clear`
- Branch-scoped pool isolation — each branch keeps its own pool at `.git/scribe/<branch>/pool.jsonl`, preventing tool usage from one branch leaking into another
- Stale-pool guard using a HEAD sentinel file (`.git/scribe/<branch>/pool-head`) — the pool is automatically cleared after a `git reset --hard` or unscribed branch switch so stale entries are never applied to the wrong commit
- `--dry-run` flag on `scribe amend` to preview the `Assisted-By` trailer without modifying the commit or clearing the pool
- Version flag (`scribe --version`) populated automatically from the tagged module version when installed via `go install`
