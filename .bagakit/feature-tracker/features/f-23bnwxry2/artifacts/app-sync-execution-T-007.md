# T-007 App Sync Execution Surface

## Implemented Surface

- Added native app tasks for:
  - `sync queue enqueue|status|list|ready|cancel|fail`
  - `sync run`
  - `sync loop`
  - `sync watch`
  - `sync network run`
  - `sync network discover-run`
  - `sync network loop`
- Added a dedicated Sync section in the macOS app.
- Source role can enqueue, mutate queue review state, run bounded local/network
  sync passes, run discovery-gated network sync, and start/stop foreground
  local/watch/network loops.
- Target and Observer roles can inspect read-only queue evidence through
  status/list/ready, but cannot enqueue, cancel/fail, or execute sync.
- Foreground sync loops use separate supervised slots:
  - Source Sync Loop
  - Source Sync Watch
  - Source Network Loop
- App command construction uses the existing CLI flags only. It does not add
  runtime policy overrides outside the profile SSOT.

## CLI Contract Used

- `sync queue` is queue-only durable evidence; it does not watch, copy, run a
  daemon, or perform ongoing sync.
- `sync run` and `sync network run` are bounded runs requiring explicit
  `--session`.
- `sync loop`, `sync watch`, and `sync network loop` are foreground processes
  requiring explicit `--session-prefix`; `--max-runs` / `--max-events` can bound
  smoke runs.
- `sync network discover-run` first requires a low-information LAN candidate
  matching profile-selected `network.receiver_url`. Discovery is not trust and
  no queue/run receipt is written on no-match.

## Structured Evidence

The app now decodes and displays:

- queue summary counters, state path, selected entries, and operator reasons
- run status, session id, run path, published/retried entries, and queue totals
- network transfer status, stage, mTLS mode, and transfer outcome
- discovery gate status, matched/profile address, candidate count, invalid
  packets, and `trusted=false`
- foreground loop/watch counters and bounded run summaries

Non-zero sync JSON exits are accepted as review evidence when stdout contains a
structured result. This is required for retrying runs and no-match
`discover-run` evidence, but those runs are still not counted as successful
recent runs by the existing `finished(0)` success gate.

## Boundaries

- This does not implement detached background sync or OS service-manager
  lifecycle.
- This does not make discovery an endpoint selector or trust mechanism.
- This does not implement Merkle/root-comparison verification.
- This does not make the app the final two-machine acceptance surface; T-008
  through T-011 still cover verification comparator, evidence browser, packaging,
  permissions, and acceptance closeout.
