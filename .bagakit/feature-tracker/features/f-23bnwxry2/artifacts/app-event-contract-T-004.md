# T-004 App Event And Artifact Reader Contract

This contract defines the app-owned structured surface used before final
transfer screens are wired.

## Event Surface

- `process event`
  - Source: app-owned foreground process supervisor.
  - Required fields: timestamp, severity, slot, task, message.
  - Current coverage: process start, stop, launch failure, operator stop intent.
- `artifact decoded`
  - Source: successful app decode of command JSON stdout.
  - Required fields: timestamp, artifact kind, task.
  - Current coverage: status, health, report, drift list, daemon status/logs,
    drift mutations, prune review/approvals/authoring/supersede, reconcile.
- `artifact read problem`
  - Source: missing or malformed JSON stdout where structured output is
    expected.
  - Required fields: timestamp, artifact kind, task, problem, raw sample.
  - Current coverage: empty stdout from successful structured commands and
    JSON/schema decode failure.
- `stale structured output`
  - Source: command completion after setup context changed.
  - Required fields: timestamp, task, stale-context detail.
- `structured output skipped`
  - Source: expected-structured command completion that is cancelled, has no
    structured stdout on a non-zero exit, or exits non-zero from a mutating
    command surface.
  - Required fields: timestamp, task, exit code, reason, stderr/stdout sample
    when available.

## UI Rules

- Text/stderr compatibility parsing may show neutral process facts such as a
  printed URL or serve summary.
- Text/stderr compatibility parsing must not produce green trust, pairing,
  verification, transfer, or root-comparison state.
- App-side artifact read problems must remain visible in the Evidence view with
  a raw sample for inspection.
- Failed or cancelled commands must not be converted into artifact read
  problems unless a successful or review-required structured command produced
  malformed or missing structured output.
- Process liveness is app-owned and foreground-only. Durable daemon status/logs
  remain evidence, not active process liveness.
- Read-only review surfaces may promote valid JSON from review-required exits,
  but the app must mark that completion as review evidence, not success.

## Remaining Boundaries

- T-004 does not implement protocol-level transfer progress, throughput, ETA, or
  Merkle/root comparison.
- T-004 does not turn serve/dashboard stderr readiness into trusted readiness.
- T-004 does not preserve raw non-UTF-8 stdout bytes; current structured command
  contracts are JSON/UTF-8 surfaces.
- Future task surfaces must replace compatibility parsing with structured CLI
  events or durable control-plane evidence before showing green state.
