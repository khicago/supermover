# T-008 Verification Comparator: JSON Evidence Foundation

## Scope

This artifact records the first implementation slice for T-008.

Implemented in this slice:

- `verify` app launches now request `--format json`.
- `VerifySnapshot` decodes the wired `internal/verify.Report` JSON surface:
  manifest summary, counts, findings, warnings, soft deletes, persisted target
  drift, and artifact problems.
- Non-zero verify review exits with structured stdout are promotable evidence,
  not artifact decode failures.
- The Verification screen now shows typed target-vs-published-manifest evidence
  and no longer treats a successful process plus unrelated aggregate evidence as
  sufficient proof of alignment.
- Post-review gate semantics now keep verification evidence out of target
  preflight readiness. A clean `verify` result can pass the integrity gate, but
  cannot make the broader target preflight gate green without `status`,
  `report`, or `health` evidence.
- Warnings, soft deletes, and target drift are now inspectable as verification
  comparator detail lists rather than only as counts.
- Root evidence is explicit:
  - `manifest.root_id` is shown as a profile root identity only.
  - Merkle/root proof is shown as unavailable.
  - Current-source comparison is shown as unavailable.

Not implemented in this slice:

- Merkle tree construction or root-hash comparison.
- Current source tree comparison after publish.
- Embedded dashboard API consumption.
- Full evidence vault browsing and safe next-action flows.

## Review Inputs

Three read-only subagents informed the slice:

- CLI/control-plane inventory: `verify --format json` and `report --format json`
  are wired; no Merkle/tree/root evidence exists.
- macOS UI/state review: add a first-class `VerifySnapshot`; keep comparator
  truth typed and avoid green states from exit code alone.
- Quiet-room architecture review: avoid overclaiming `root_id`; keep root proof
  availability explicit and negative states visible.

## Tests Added

- `testVerifyCommandRequestsJSONAndOptionalSession`
- `testVerifySnapshotDecodesManifestAndReviewEvidence`
- `testVerifySnapshotKeepsRootIdentitySeparateFromMerkleProof`
- `testVerifySnapshotNoManifestRequiresReviewAndRootUnavailable`
- `testVerifySnapshotClearsOnSetupContextChange`
- `testEvidenceGateEvaluationKeepsVerifyOutOfTargetPreflight`

## Validation

- `swift build --package-path macos`
- `swift test --package-path macos`
- `go mod tidy -diff`
- `git diff --check`
- `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go test -p=1 -count=1 ./...`
- `TMPDIR=$PWD/.tmp/testtmp GOTMPDIR=$PWD/.tmp/go-build GOCACHE=$PWD/.tmp/gocache go vet ./...`
- `go run ./cmd/supermover help`
- `go run ./cmd/supermover version`
- `feature-tracker validate-tracker --root .` through the installed Bagakit
  feature tracker script
- `feature-tracker run-task-gate --root . --feature f-23bnwxry2 --task T-008`
  passed after the post-review gate-semantics fix and wrote
  `artifacts/gate-T-008-r13-0001.log`. Earlier `gate-T-008-r12-0001.log`
  passed the initial comparator slice before the gate-semantics fix; the prior
  `gate-T-008-r11-0001.log` failed at the same Go test command without captured
  package output, and the exact command passed manually immediately afterward.
- Manual CLI smoke: publish a temporary profile containing a regular file and a
  hidden file, then run `verify --format json`; `jq` confirmed
  `manifest.session_id`, `manifest.root_id`, `files_expected == 2`,
  `files_verified == 2`, and zero findings.
- Manual CLI negative smoke: run `verify --format json` before any manifest is
  published; exit status was `1` while stdout remained machine-readable with
  `manifest.id == ""` and `summary.manifest_count == 0`.
- Post-commit subagent review of `c4627d8` found no P0 and one P1 gate-semantics
  issue. The P1 is fixed by `EvidenceGateEvaluation`, which separates target
  preflight evidence from verification evidence.
