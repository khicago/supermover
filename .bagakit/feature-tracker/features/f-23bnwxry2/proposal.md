# Feature Proposal: f-23bnwxry2

## Title

Native macOS app-first LAN migration workbench

## Why

Supermover now has a native macOS operator shell and a CLI-backed network
migration stack, but the app is still primarily a command launcher. Operators
who install the app on source and target machines still need to understand the
CLI surfaces for discovery, pairing, serve, network push, sync queue/run/loop,
verification, and evidence review.

The product goal requires an app-first workflow that can guide a high-stakes
cross-device migration without hiding the underlying safety model. The app
should feel professional and elegant, but the implementation plan must start
from profile creation, current CLI command coverage, process supervision,
structured event/artifact contracts, and macOS deployment constraints. Every
green state must be backed by profile-selected configuration, successful command
output, or durable `.supermover` evidence.

## Goal

Design and implement a complete native macOS app-first LAN migration workbench
that guides source and target operators through discovery, pairing, bounded
network sync, verification, evidence review, and safe follow-up actions while
preserving profile and target evidence as the source of truth.

## Current Implementation Checkpoint

As of the T-004 gate, this feature is in execution but not complete.

Implemented:

- T-001: capability contract and command coverage matrix.
- T-002: app-guided setup/onboarding for profile path, source root, target root,
  profile init/lint/set-target, and role selection.
- T-003: role-scoped foreground process supervision for ordinary actions,
  target serve, and target dashboard, including stale setup-context handling.
- T-004: structured app events plus visible artifact-reader problem reporting
  for expected JSON command output.

Not yet implemented:

- native discovery browse/advertise and pairing workflow screens
- sync queue/run/loop/watch/network controls
- verification comparator with Merkle/root evidence availability rules
- evidence browser with safe next-action flows
- macOS packaging, permission, signing/notarization, Local Network/firewall,
  and bundled CLI provenance hardening
- final acceptance matrix, docs closeout, and two-machine or loopback LAN
  evidence

Current product truth: the macOS app can be used as a CLI-backed operator
workbench, but it is not yet sufficient as the only interface for a large
cross-device LAN migration.

## Principle Layer

- What: a native app-first workflow for source, target, and observer operation.
- Why: large LAN migrations need a guided control room rather than a raw command
  matrix, but migration safety depends on evidence-backed truth.
- Intended generalization: app UI states are projections of profile, CLI, and
  target control-plane artifacts.
- Failure boundary: missing, stale, failed, or unsupported evidence appears as
  not-run, review-required, blocked, or unavailable, never as success.
- Behavior examples: target app starts `serve` and shows pairing code; source
  app browses low-information candidates, pairs with verification code, runs a
  network dry-run, executes a bounded sync, verifies target evidence, and opens
  receipts/warnings for review.
- Evidence refs: current CLI behavior, `f-235nwsp4y` app shell,
  `f-226nwy2vy` discovery/pairing, `f-227nw2p2n` secure transport,
  `f-232nwu2nw` ongoing sync surfaces, and target `.supermover` artifacts.

## Scope

- In scope: app-guided profile init/lint/set-target, source/target/observer
  role flows, native discovery and pairing, network sync queue/run/loop/watch/
  discover-run controls, command coverage tests, role-scoped process
  supervision, structured progress/run-state and artifact-reader APIs,
  verification comparator, evidence browser, macOS permissions/signing/
  local-network packaging/runbook, and app copy that accurately reflects wired
  behavior.
- Out of scope: bidirectional sync, automatic endpoint trust, app-only runtime
  policy overrides, physical prune bypasses, broad automatic repair, and
  claiming OS-managed detached daemon behavior before it is implemented.

## Acceptance Criteria

- The app can be installed on source and target machines and guide an operator
  through profile creation or selection, role setup, LAN readiness, discovery,
  pairing, bounded sync, verification, and evidence review without requiring raw
  CLI knowledge for ordinary paths.
- A command coverage matrix maps each claimed app action to a current CLI/API
  command, required arguments, structured outputs, disabled states, tests, and
  unsupported states.
- Discovery candidates are shown as untrusted hints until pairing evidence and
  profile pins validate.
- Pairing, receiver serving, network dry-run, bounded transfer, verification,
  and evidence review are all available through native screens.
- The app can supervise the foreground tasks each role actually needs. Starting
  one long-running task must not silently terminate another required target or
  source role task.
- Transfer progress, throughput, ETA, comparator summaries, and green safety
  states appear only when backed by structured events or durable evidence.
- Merkle/root comparison is either implemented in CLI/control-plane evidence
  with tests, or the app explicitly shows it as unavailable rather than
  pending/successful.
- Evidence browser exposes receipts, warnings, profile snapshots, network
  transfer artifacts, drift/reconcile records, prune approvals, malformed
  artifact states, schema/version problems, and raw JSON.
- Mutating flows require explicit operator intent, reason, reviewer, approval,
  or session inputs matching current CLI safety gates.
- macOS install/runtime readiness covers file permissions, Local Network/
  firewall/listen behavior, signing or local developer-build provenance,
  bundled CLI version display, and failure copy.
- Docs and README state which app-first behaviors are implemented, foreground
  only, detached/background, or still planned.
- Verification includes Swift build, Go tests for touched CLI/API surfaces,
  contract tests, app smoke, negative/fault-path tests, subagent review gates,
  and a two-machine or loopback LAN runbook with captured evidence.

## Adjusted Execution Plan

The next execution phases should stay in this order unless a blocker is found:

1. T-005: stabilize the SwiftUI information architecture and design system
   against the current structured-event/artifact-reader contract.
2. T-006: implement native discovery and pairing workflow with explicit trust
   boundaries.
3. T-007: wire sync queue/run/loop/watch/network execution controls without
   claiming detached background behavior.
4. T-008: build verification comparator views and show Merkle/root comparison
   only when backed by wired CLI/control-plane evidence.
5. T-009: build the evidence browser and safe next-action flows.
6. T-010: harden macOS packaging, permissions, bundled CLI provenance, and
   foreground daemon controls.
7. T-011: run acceptance matrix, docs closeout, and independent review gates.

## Transfer Checks

- Do not count the existing macOS operator shell as completion of this feature.
- Do not display mock progress, mock Merkle roots, or example device names as
  live truth.
- Do not infer trust from LAN discovery.
- Do not mark warnings, drift, or verification as clean when evidence has not
  been loaded.
- Do not add app-specific runtime overrides that bypass profile SSOT.
- Do not describe foreground loops as detached background services.

## Impact

- Code paths: macOS SwiftUI app, app state/process orchestration, CLI command/
  event surfaces, profile init/lint/set-target, discovery/pairing, sync queue/
  run/loop/watch/network, network transfer evidence, status/report/health,
  dashboard/operator UI, control-plane artifact readers, packaging scripts,
  permission/readiness surfaces, and docs.
- Tests: Swift build, app view/model tests if introduced, CLI contract tests,
  progress/event contract tests, pairing/discovery smoke, sync network
  acceptance, verification/evidence fixtures, duplicate/ambiguous discovery,
  unpaired and wrong-code refusal, interruption/resume, hidden files and
  dot-directories, `.supermover` target guards, stale/missing evidence refusal,
  and docs/help honesty checks.
- Rollout notes: implement in slices. Start from the current Control Room UI,
  but advance only evidence-backed behavior. Keep the archived app-shell feat
  as a prerequisite, not as proof this app-first feature is complete.
