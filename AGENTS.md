# AGENTS.md

This file is the repository-level operating guide for AI agents working on
Supermover.

## Project Truth

Supermover is a Go CLI for one-way, auditable file migration. The long-term
goal includes LAN agents, discovery, pairing, encrypted transport, resumable
sync, bounded traffic-shape protection, and ongoing incremental synchronization.

The current implementation may lag the design docs. Before reporting status or
claiming completion, check all three:

1. original/user-stated requirement
2. current wired CLI/code behavior
3. validation evidence from tests or manual smoke runs

Do not treat a strong vertical slice as the completed minimum product. A prior
mistake in this repo was calling the local/mounted migration slice effectively
done while LAN agent, encrypted communication, traffic-shape protection,
changed-file incremental sync, richer audit/report UX, and migration runbook
surfaces were still unwired. This is a standing lesson: **implemented**,
**planned**, and **originally requested** are separate statuses.

When docs mention future behavior, verify whether the command or package is
actually wired before presenting it as available.

## Required Skills

Prefer mature external skills as the engineering base, then layer the
Supermover-local project overlay for repository facts and stricter product
invariants.

Project-local external skills are installed under `.codex/skills` and
`.agents/skills`:

- Go base: `samber/cc-skills-golang` skills such as `$golang-cli`,
  `$golang-code-style`, `$golang-concurrency`, `$golang-context`,
  `$golang-error-handling`, `$golang-lint`, `$golang-modernize`,
  `$golang-naming`, `$golang-observability`, `$golang-project-layout`,
  `$golang-safety`, `$golang-security`, `$golang-structs-interfaces`,
  `$golang-testing`.
- TDD/debug/review base: `$tdd`, `$test-driven-development`, `$diagnose`,
  `$systematic-debugging`, `$requesting-code-review`,
  `$receiving-code-review`, `$verification-before-completion`,
  `$subagent-driven-development`.
- Architecture/DDD base: `$improve-codebase-architecture`,
  `$clean-ddd-hexagonal`.
- Experience/design lenses: `$karpathy-guidelines`,
  `$andrej-karpathy-perspective`, `$software-design-philosophy`,
  `$pragmatic-programmer`, `$refactoring-patterns`, `$release-it`,
  `$ddia-systems`, `$grill-with-docs`, `$zoom-out`.
- Supermover overlay: `$supermover-project-rules`.

Do not load every skill. Use the smallest useful set, normally one external
base skill plus `$supermover-project-rules`; add one lens skill only when the
task clearly needs a stronger perspective.

### Skill Selection Matrix

| Task | Load |
| --- | --- |
| CLI behavior, flags, exit codes | `$golang-cli` + `$supermover-project-rules` |
| Ordinary Go implementation | `$golang-code-style` or `$golang-safety` + `$supermover-project-rules` |
| Tests or bug fixes | `$golang-testing` or `$tdd` + `$supermover-project-rules` |
| Crash/recovery/debugging | `$systematic-debugging` or `$diagnose` + `$supermover-project-rules` |
| Architecture, DDD, package boundaries | `$improve-codebase-architecture` or `$clean-ddd-hexagonal` + `$supermover-project-rules` |
| Concurrent workers, cancellation, long-running agents | `$golang-concurrency` + `$golang-context` + `$supermover-project-rules` |
| Security, path safety, crypto, pairing | `$golang-security` + `$golang-safety` + `$supermover-project-rules` |
| Observability, audit/report/status | `$golang-observability` + `$supermover-project-rules` |
| Pre-completion review | `$verification-before-completion` or `$requesting-code-review` + `$supermover-project-rules` |
| Subagent implementation team | `$subagent-driven-development` + `$supermover-project-rules` |

### Lens Skill Rules

Use lens skills to challenge judgment, not to replace implementation skills.

| Situation | Add |
| --- | --- |
| Any non-trivial coding/refactor task where LLM overreach is likely | `$karpathy-guidelines` |
| AI/agent reliability, vibe-coding, capability-boundary, or product judgment discussion | `$andrej-karpathy-perspective` |
| Module/interface depth, complexity budget, shallow abstractions | `$software-design-philosophy` |
| Shipping strategy, tracer bullets, DRY/orthogonality, reversible decisions | `$pragmatic-programmer` |
| Behavior-preserving cleanup with named transformations | `$refactoring-patterns` |
| Resilience, failure containment, timeout/retry/bulkhead/operability thinking | `$release-it` |
| Distributed/data consistency, replication, transactions, sync protocol reasoning | `$ddia-systems` |
| Stress-testing a plan against docs/domain language before implementation | `$grill-with-docs` |
| Need broader map before editing unfamiliar code | `$zoom-out` |

For complex work, choose at most one primary lens for the first pass. Add a
second lens only during review if it exposes a different class of risk.

If a requested domain has no credible external skill, record that search result
before creating or extending a local skill.

Continue using global skills when they apply, especially `go-testing`,
`karpathy-guidelines`, `bagakit-researcher`, `bagakit-git-message-craft`,
`find-skills`, and `skill-vetter`.

## Engineering Rules

- Profile files are the SSOT. Do not add casual runtime overrides that make
  behavior impossible to audit.
- Hidden files and dot-directories are first-class migration data.
- `.supermover` is reserved control-plane space on targets; guard normalized
  paths, symlinks, and traversal attempts.
- Warning records must be durable, auditable, and usable as "needs additional
  migration config" input.
- Soft deletes must remain easy for users to inspect before physical pruning.
- Before target mutation, preflight the whole publish plan where the code has
  enough information to know conflicts.
- For safety-critical flows, add a behavior test before or with the fix.
- Keep docs and README honest about wired behavior versus roadmap.
- Never delete, prune, clean, rewrite, or otherwise remove `.codex` session
  state, including any session/runtime/history material under `.codex`. Treat
  these sessions as user-owned operational evidence unless the user gives an
  explicit, path-specific deletion instruction in the current conversation.

## Validation

Use targeted tests during development. Before claiming a non-trivial change is
done, run the relevant subset of:

```bash
go mod tidy -diff
go test -count=1 ./...
go test -race -count=1 ./...
go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go vet ./...
staticcheck ./...
golangci-lint run ./...
git diff --check
go run ./cmd/supermover help
go run ./cmd/supermover version
```

If a command cannot be run, state that explicitly in the final response.

## Research And Decisions

Use `$bagakit-researcher` for durable research. Existing evidence lives under
`.bagakit/researcher`; inspect it before opening a new topic.

When using researcher:

- refresh or inspect the frontdoor first
- create/update a topic charter and pass for broad research
- keep source cards and summaries source-bound
- record claims before turning evidence into recommendations
- run researcher doctor checks and refresh indexes before closeout

## Commits

When asked to commit, use `bagakit-git-message-craft` and write audit-grade
messages with context, key facts, validation, risks deliberately avoided, and
known next work. Do not hide unimplemented originally requested behavior behind
optimistic wording.
<!-- BAGAKIT:LIVING-KNOWLEDGE:START -->
This is a managed block for `bagakit-living-knowledge`. Do not hand-edit the
managed region directly; refresh it through the skill operator instead.

Resolve the installed skill dir before using the operator directly:

- `export BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR="<repo-relative-installed-skill-dir>"`

Boot layer:

- Read the resolved `must-guidebook.md` before relying on memory.
- If a task needs shared knowledge rules, read `must-authority.md`.
- If a task needs maintenance-route guidance or shared directives, read `must-sop.md`.
- If a task needs prior decisions or facts, follow `must-recall.md`.
- `AGENTS.md` is only the bootstrap layer; the shared checked-in knowledge root
  is configured in `.bagakit/knowledge_conf.toml`.

Recall discipline:

- Search first:
  - `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" recall search --root . '<query>'`
- Then inspect only the needed lines:
  - `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" recall get --root . <path> --from <line> --lines <n>`
- Prefer quoting only needed lines over paraphrasing from memory.

Substrate discipline:

- Shared knowledge belongs under the configured shared root.
- Durable examples and managed bootstrap text must stay repo-relative; never
  record absolute filesystem paths in shared knowledge or AGENTS guidance.
- When imported material needs one durable handle, prefer a short opaque id
  such as `k-2ab7qxk9` instead of a timestamped capture name.
- Research runtime belongs to `bagakit-researcher`.
- Task-level composition/runtime belongs to `bagakit-skill-selector`.
- Repository evolution memory belongs to `bagakit-skill-evolver`.
- `living-knowledge` owns path protocol, normalization, indexing, and recall.
- `living-knowledge` also owns generated `must-sop.md` and reusable-items
  governance inside the shared knowledge root.

Inspection helpers:

- View the resolved path protocol:
  - `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" paths --root .`
- Refresh the guidebook and helper map:
  - `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" index --root .`
- Run non-destructive diagnostics:
  - `sh "$BAGAKIT_LIVING_KNOWLEDGE_SKILL_DIR/scripts/bagakit-living-knowledge.sh" doctor --root .`

If the surrounding workflow explicitly asks for `living-knowledge` task
reporting, the response footer may use:

- `[[BAGAKIT]]`
- `- LivingKnowledge: Surface=<updated shared surfaces or none>; Evidence=<commands/checks>; Next=<one deterministic next action>`
<!-- BAGAKIT:LIVING-KNOWLEDGE:END -->
