# Feature Proposal: f-237nwzbyq

## Title

Broad repair scan inventory

## Why

`f-233nwduwz` can plan or apply selected persisted drift records and can record
current live detector findings through explicit gates. It still does not run a
general background inventory that searches for repair candidates across the
profile-selected target.

This feature keeps scan inventory separate from mutation. A broad scanner must
produce durable review evidence first; it must not quietly apply repair,
rewrite manifests, or authorize prune.

## Goal

Define and implement an explicit non-mutating broad repair scan inventory that
turns profile-scoped target evidence into durable review inputs without
applying repair, rewriting manifests, or pruning target files.

## Principle Layer

- What: a read-only broad inventory over profile-selected source, target, and
  target control-plane artifacts.
- Why: broad repair needs a complete review input set before any automatic
  mutation can be justified.
- Intended generalization: scan output becomes durable operator evidence usable
  by later retry, manifest rewrite, prune handoff, and UX surfaces.
- Failure boundary: scan artifact problems, unsafe paths, symlinks, scope
  mismatch, and missing evidence are recorded as review findings, not repaired.
- Behavior examples: record scan findings for missing regular files, target
  conflicts, unsupported drift classes, and artifact integrity problems without
  mutating the target.
- Evidence refs: existing `drift record`, `reconcile review`, and status/report
  target-drift evidence.

## Scope

- In scope: profile-backed scan configuration, read-only scanner, durable scan
  artifacts, report/status aggregation, path safety, and docs.
- Out of scope: applying repair, automatic retry, manifest rewrite, prune
  approval, operator batch-apply UX, LAN/network discovery, and ongoing sync
  transport.

## Acceptance Criteria

- Scan output is durable, reviewable, and tied to the profile snapshot and
  target control-plane state.
- Scans include hidden files and dot-directories as normal migration data.
- `.supermover` reserved control-plane protections remain enforced.
- The scanner never mutates target data or control-plane mutation records
  except its own scan evidence.
- Docs state that scan inventory is not repair completion.

## Transfer Checks

- Do not count scan findings as resolved drift.
- Do not let scan inventory bypass persisted drift review state.
- Do not infer prune approval from scan output.

## Impact

- Code paths: live drift detector, drift store, scan artifact schema,
  report/status, health/verify if scan evidence is surfaced there.
- Tests: path safety, hidden-file coverage, artifact durability, read-only
  behavior, and stale profile/target refusal.
- Rollout notes: this feature creates evidence for later broad repair slices;
  it is not broad automatic repair by itself.
