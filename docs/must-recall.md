# Supermover Recall

This page defines the default recall workflow for shared knowledge.

## Workflow

1. Search first.
2. Read only the needed lines.
3. Quote only the needed lines.
4. Answer with references when useful.

## Commands

- search:
  - `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" recall search --root . '<query>'`
- get:
  - `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" recall get --root . <path> --from <line> --lines <n>`

## Scope

Default recall searches:

- the configured shared root
- root and path-applicable `AGENTS.md`

Default recall does not search other runtime systems unless the task asks for
them explicitly.

## Common Queries

Use these as starting points:

- current capability: `release audit current completed slice`
- implemented commands: `Current Slice Versus Planned Surface`
- local migration workflow: `publish local push verify recover`
- warning records: `warning auditability suggested_config`
- soft delete: `soft-delete review prune`
- recovery: `recover needs_repair staged manifest`
- network plan: `serve discover pair encrypted transport`
- privacy limits: `traffic metadata reduction anonymity`
- agent knowledge: `agent influence manifest`

## Runtime Recall

Use `.bagakit/researcher` for preserved research evidence and `.bagakit/feature-tracker`
for feature lifecycle state only when explicitly needed. Research and tracker
state are not shared checked-in truth until reviewed and normalized into `docs`.
