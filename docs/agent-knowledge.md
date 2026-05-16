# Agent Knowledge Catalog

Supermover preserves agent knowledge files as normal files and catalogs their
influence for downstream tooling. It does not interpret, merge, summarize,
embed, or promote semantic memory.

## Scan Foundation

`internal/scan` performs a read-only local filesystem scan rooted at a
directory. It records:

- relative path
- type: regular file, directory, symlink, or special file
- hidden-path status
- size, mode, modification time, executable bit
- symlink target, without following the link

Special files are not copied or modified by the scanner. They produce warning
audit records so later workflow layers can decide whether to skip, transform,
or require explicit user approval.

## Audit Records

`internal/audit` defines stable, serializable records for findings and policy
decisions:

- stable ID
- source path and optional target path
- severity, kind, reason
- detected metadata
- suggested profile patch or config
- disposition

Record IDs are derived from stable identifying fields, not from enrichment
metadata, so the same finding can be tracked across later report generation.

## Agent Knowledge Detection

`internal/agentkb` classifies known agent-influence paths from scan entries.
Current patterns:

- `AGENTS.md`
- `CLAUDE.md`
- `GEMINI.md`
- `.github/copilot-instructions.md`
- `.github/instructions/**`
- `.cursor/rules/**`
- `.windsurf/rules/**`
- `.continue/**`
- `.codex/**`

Categories are intentionally coarse:

- `repo_rules`: repository-level human-authored agent rules, currently
  `AGENTS.md`.
- `tool_project_rules`: project-local tool instructions for specific agents or
  IDE assistants.
- `home_memories`: user-home memory or settings files when such paths are
  scanned as part of a home migration.
- `generated_state`: generated or runtime state that should be preserved but
  treated differently from authored instructions.

Open decision: `.codex/**` is currently categorized as `generated_state`
because it can include runtime state as well as local skill/config material.
Future profile policy may split generated state from authored Codex project
rules once the expected file layout is fixed.
