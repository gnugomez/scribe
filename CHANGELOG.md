# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
