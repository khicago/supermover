# Claims

## c001

- kind: `observation`
- status: `open`
- confidence: `low`
- confidence: `medium`

### Statement

The current feature plan has the right evidence-backed product direction, but it is not yet a complete implementation contract because it lacks app-owned profile bootstrap, exact command coverage, early event contracts, multi-process supervision, and concrete acceptance gates.

### Evidence Refs
- summaries/sm01.md; summaries/sm02.md

### Counterevidence Refs
- none recorded

### Bagakit Implication

Revise the feature plan before implementation; treat the current UI redesign as
an input slice, not completion evidence.

## c002

- kind: `observation`
- status: `open`
- confidence: `high`

### Statement

The CLI exposes profile init/lint/set-target, discover, pair, sync queue/run/loop/watch/network, daemon, and serve surfaces, while the current macOS app task enum only covers profile lint, local/review commands, network push, serve, daemon status/logs, and dashboard.

### Evidence Refs
- originals/sm02.md; macos/SuperMoverApp/AppStore.swift:25; internal/cli/cli.go:9045

### Counterevidence Refs
- none recorded

### Bagakit Implication

Create a command coverage matrix and make missing app actions explicit
implementation tasks.

## c003

- kind: `recommendation`
- status: `open`
- confidence: `medium`

### Statement

Structured lifecycle/event and artifact-reader contracts should be implemented before app transfer controls, because current readiness parsing uses stderr text and a single active child process terminates previous long-running tasks.

### Evidence Refs
- summaries/sm02.md; macos/SuperMoverApp/AppStore.swift:1145; macos/SuperMoverApp/AppStore.swift:1452

### Counterevidence Refs
- Current CLI commands already provide JSON for many bounded command results;
  the missing piece is a durable/live app-oriented event and artifact contract,
  not all command output.

### Bagakit Implication

Move structured event/artifact contracts ahead of transfer UI completion.

## c004

- kind: `recommendation`
- status: `open`
- confidence: `medium`

### Statement

macOS file access, Local Network privacy, firewall/listen guidance, signing/notarization, bundled CLI provenance, and key material handling are implementation requirements for two-machine install, not late release polish.

### Evidence Refs
- summaries/ap01.md; summaries/ap02.md; summaries/ap03.md

### Counterevidence Refs
- Developer-local builds can run without App Store distribution, but the user
  asked for source/target installation on two machines, so provenance and
  permission handling still need explicit acceptance criteria.

### Bagakit Implication

Treat macOS deployment and permission readiness as feature tasks, not final
release polish.

## c005

- kind: `recommendation`
- status: `open`
- confidence: `medium`

### Statement

Verification comparison should be scoped to current wired evidence, and Merkle/root comparison must be either separately implemented with tests or shown as unavailable; UI-only root labels would overclaim.

### Evidence Refs
- summaries/pr02.md; .bagakit/feature-tracker/features/f-23bnwxry2/tasks.json

### Counterevidence Refs
- The current verification/dashboard surfaces can still provide useful
  file-level evidence without Merkle roots.

### Bagakit Implication

Make root comparison conditional: implement it with Go tests or show it as
unavailable.

## c006

- kind: `recommendation`
- status: `open`
- confidence: `medium`

### Statement

The implementation plan should add negative and fault-path gates for duplicate discovery, unpaired transfer, wrong verification code, interruption/resume, hidden files, dot-directories, .supermover target guards, stale evidence, and two-machine LAN evidence capture.

### Evidence Refs
- summaries/sm01.md; summaries/pr01.md; summaries/pr02.md

### Counterevidence Refs
- Some cases may already be covered by lower-level Go tests, but the app-first
  feature still needs acceptance gates that prove the native orchestration does
  not mask failures.

### Bagakit Implication

Add negative/fault-path gates to the final task rather than relying on happy
path smoke runs.
