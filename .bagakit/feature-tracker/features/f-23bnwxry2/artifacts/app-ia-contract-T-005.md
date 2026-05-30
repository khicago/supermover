# T-005 App Information Architecture Contract

This contract defines the native workbench information architecture after the
structured app event and artifact-reader base.

## Navigation Model

- The app has one role-aware navigation spine shared by source, target, and
  observer operators.
- Each section declares availability for the selected role:
  - `available`: current app-backed actions or evidence views exist.
  - `planned`: the section names future app-first behavior without showing it as
    live capability.
  - `read-only`: the role can inspect evidence but cannot mutate state.
  - `role-gated`: the role should not run the section's execution actions.
- Sidebar badges and top-bar chips expose this availability so planned or
  role-gated surfaces cannot look equivalent to complete app-first behavior.

## Role Runway

- Source runway: setup, trust, transfer, verification.
- Target runway: target setup, serve, evidence, install readiness.
- Observer runway: profile, evidence, dashboard, blocked mutations.
- Runway state is derived only from profile path/readiness, recent successful
  app-launched commands, active foreground slots, structured snapshots, and
  durable target evidence.
- Runway state must not synthesize transfer progress, throughput, ETA,
  Merkle/root comparison, or broad all-clear states.
- Planned and blocked runway cards are informational and do not navigate into a
  generic execution surface.

## Page Rules

- Pairing:
  - Target role may start supervised foreground `serve`.
  - Source role sees native discovery/pairing as planned for T-006 and can only
    inspect pairing evidence or run network preflight.
  - Observer role is read-only.
- Transfer:
  - Source role may run current wired network dry-run and bounded network push.
  - Target and observer roles see transfer execution as source-owned and
    role-gated.
- Drift review:
  - Source role may access current explicit review/mutation commands.
  - Target and observer roles get read-only drift/prune evidence entry points.
- Settings:
  - Source role may edit full command inputs for current wired source-owned
    flows.
  - Target role shows install readiness as planned and hides source mutation
    inputs.
  - Observer role hides mutation inputs and relies on task-level role gates.
- CLI surface:
  - The task picker may show the complete command catalog, but Run/Stop/Open URL
    controls are only shown when the selected role can execute the selected task.
- Evidence and verification remain read-first and cannot claim root comparison
  unless T-008 wires real CLI/control-plane evidence.

## Remaining Boundaries

- T-005 does not implement native discovery browse, advertise, or pair flows.
- T-005 does not add sync queue/run/loop/watch/network controls.
- T-005 does not add Merkle/root comparison evidence.
- T-005 does not harden packaging, signing, notarization, Local Network
  permission diagnostics, or bundled CLI provenance.
- T-005 does not make the app sufficient as the sole interface for a large
  two-machine migration.
