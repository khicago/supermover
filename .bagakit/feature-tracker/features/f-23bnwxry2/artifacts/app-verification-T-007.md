# T-007 Verification Notes

## App Checks

- `swift build --package-path macos`: pass
- `swift test --package-path macos`: pass, 8 XCTest tests

The Swift tests cover:

- sync queue and sync run/network/discover command construction
- source/target/observer role gates for sync tasks
- independent foreground slots for local loop, OS watch, and network loop
- queue JSON decoding
- discovery-gated no-match JSON decoding as review evidence

## Repository Checks

Full repository gate evidence is recorded in
`artifacts/gate-T-007-r10-0001.log` after `run-task-gate` passes.

## Manual Review Points

- Sync execution remains source-owned. Target and Observer can only inspect
  queue status/list/ready through read-only commands.
- Foreground loops/watchers are explicitly supervised child processes and are
  not represented as detached background sync.
- Discovery-gated network sync renders the LAN candidate as untrusted
  availability evidence; trust remains pairing plus profile-pinned mTLS.
