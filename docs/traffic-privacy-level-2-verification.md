# Traffic Privacy Level 2 Verification

This document records the current f-22vnwgwjj evidence boundary.
Operator-facing non-dry-run `push --network` is wired, but level 2 remains a
bounded metadata-reduction claim, not anonymity or broad sync/resume
completion.

## Current Runnable Gate

- `profile lint` is the wired profile/privacy SSOT gate. It rejects traffic
  level 2 profiles that omit padding, batching, jitter, or low-information
  discovery settings, and prints the configured level 2 contract with
  profile-owned network/privacy fields. It does not contact a receiver.
- `health --profile` is a wired read-only control-plane health surface. JSON
  output can include `network_transfers` when a `networkrun` artifact exists,
  including artifacts from non-dry-run `push --network`.
  Published level 2 network-transfer artifacts are invalid without applied
  privacy overhead evidence.
- `report --profile` is the wired read-only operator aggregation surface. Text
  and JSON output expose configured privacy policy, residual leakage, pairing
  state, and review-required network-transfer artifacts when they are not clean
  published evidence. Clean published network-transfer privacy overhead is
  verified from `.supermover/sessions/<session>/network-transfer.json`; a clean
  `health`/`status`/`report` run may not print a `network_transfer` issue line.
- `status --profile` and `report --profile` expose
  `traffic_privacy_acceptance`. A `passed` result requires the profile SSOT to
  be traffic level 2, profile-backed mTLS to be configured, the transfer
  artifact policy to match the profile privacy policy, source/target device IDs
  to match the pinned pairing receipt, and applied padding, batching, and jitter
  counters to be present in a clean published `network-transfer.json`.
  Otherwise the field reports blockers such as `privacy_policy_mismatch`,
  `network_transfer_identity_mismatch`, or missing applied overhead. The field
  always carries `anonymity_claim=not_claimed`.
- `push --network --profile ... --dry-run` is a contract preflight. It requires
  profile-backed network material and valid local TLS identity files/pins
  before emitting a `transfer=dry_run`,
  `encrypted_transfer=profile_backed_mtls_validated`, `resume=not_attempted`,
  `resume_authority=not_attempted`, `resume_outcome=not_attempted` binding; it
  writes no session artifact, contacts no receiver, and sends no files.
- Non-dry-run `push --network --profile ... --session <id>` connects to the
  profile-selected pinned TLS 1.3 mTLS receiver and writes receiver-side
  `.supermover/sessions/<session>/network-transfer.json` only after receiver
  begin stores a session. Current operator `push --network` supports traffic
  level 2 only. Zero-byte regular files publish through explicit final empty
  completion evidence on this profile-backed path and should have normal
  receipt plus network-transfer evidence when otherwise clean. Transport setup
  failures and begin-auth refusal can still leave no network-transfer artifact.
- Same-session non-dry-run output includes `resume_authority` and
  `resume_outcome`. `resume_outcome=resumed` plus nonzero `resumed_bytes`
  indicates a receiver-status partial upload resume only when preserved
  artifacts show matching prior payload-overhead evidence. `published_retry` is
  a zero-payload idempotent commit retry; it preserves prior published payload
  overhead evidence and records `needs_repair/payload_overhead_missing` when
  needed prior evidence is unavailable or not auditable.
- The bounded automated interruption-rerun gate uses the same profile-backed
  level 2 path: receiver-accepted payload bytes, simulated transport failure,
  same profile/session CLI/Runner rerun, `resume_authority=receiver_status`,
  `resume_outcome=resumed`, nonzero `resumed_bytes`, clean
  `health`/`status`/`report`, and preserved or merged privacy overhead in
  `network-transfer.json`.

## Evidence Refs

- `internal/profile/profile.go`: validates level 2 padding, batching, jitter,
  and low-information discovery fields from the profile.
- `internal/cli/cli.go`: `profile lint`, `status`, and `report` print privacy status,
  configured reductions, residual leakage, and network-transfer evidence when
  artifacts exist. `status` and `report` also print
  `traffic_privacy_acceptance`. `push --network` separates dry-run preflight
  from non-dry-run transfer.
- `internal/cli/cli_test.go`: covers no runtime privacy override flags for
  `push --network`, dry-run no-session-artifact mutation, non-dry-run mTLS
  publish, public-protocol receiver-status resume, bounded same-session
  CLI/Runner interruption-rerun recovery, report text privacy lines, and report
  JSON network-transfer overhead.
- `internal/networkrun/run.go`: requires caller-supplied profile privacy policy
  to match the transfer request policy before writing network-transfer
  artifacts, then records outcome artifacts and privacy overhead for network
  runs. Receiver-status retries that imply previously accepted payload bytes
  require auditable prior transfer evidence; published zero-payload retries read
  prior receiver-side transfer evidence and preserve existing payload overhead.
  Missing, corrupt, mismatched, non-published where a published retry is
  required, or jitter-only prior evidence blocks the release claim with
  `payload_overhead_missing`.
- `internal/control/control.go`: rejects published level 2 `network-transfer`
  artifacts that omit applied privacy overhead. The release acceptance layer
  further requires the overhead to prove each configured level 2 reduction,
  not just any nonzero overhead field.
- `internal/protocolclient/client.go`: applies level 2 padding, batching, and
  bounded jitter to the protocol client path from its transfer request policy.
- `docs/runbook.md`, `docs/troubleshooting.md`, and `docs/release-audit.md`:
  define the current T-006 release boundary and deferred network product gates.

## Deferred Acceptance

The remaining acceptance work is not "can the CLI send a file over mTLS" or
"can the narrow interruption seam rerun." Those paths are wired. Deferred
surfaces are LAN browsing, daemon behavior, ongoing incremental sync, broad
resume acceptance and runbook UX, receiver-side recovery UX, arbitrary
process-kill recovery, and broader release smoke evidence beyond simulated
transport failure. Future gates must preserve the profile, command output,
`health` output, `report` output, `network-transfer.json` when a transfer
attempt reaches receiver begin and stores a session, receiver-side session
evidence when present, and interruption/resume evidence. They must also show
applied padding, batching, and bounded jitter overhead without claiming
anonymity. Total bytes, transfer duration, peer IP addresses, LAN presence, and
Supermover use remain observable.
