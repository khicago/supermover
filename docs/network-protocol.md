# Network Protocol

Supermover v1 is intended to run behind an authenticated, encrypted transport
boundary. This slice defines the stable receiver protocol, a testable server
library, a source protocol client, TLS/mTLS adapters, a profile-backed `serve`
receiver listener for paired targets, and the non-dry-run `push --network`
source path. `push --network --dry-run` remains preflight-only and does not
contact the receiver or write target artifacts.

The f-227 transport decision is TLS 1.3 over the existing HTTP receiver
protocol, with mutual certificate authentication and profile-pinned peer
identity checks. Library helpers now implement TLS 1.3 mTLS config, leaf SPKI
pin checks, certificate validity checks, and TLS-derived receiver peer context.
The receiver side is wired through `serve` only after pairing and complete
profile network material. Source-side non-dry-run `push --network` loads the
profile-selected local TLS identity, validates the paired target pin, connects
to the profile-selected receiver URL over pinned TLS 1.3 mTLS, and drives the
protocol client through `networkpush` and `networkrun`. QUIC/TLS and Noise
remain deferred adapter options.

The source-side CLI contract is wired as `supermover push --network`. It
validates the selected profile, pairing receipt, and profile-backed network
material (`network.receiver_url` plus `network.local_tls_identity`). Without
`--dry-run`, it transfers through the pinned mTLS receiver and records
receiver-side `network-transfer.json` outcome evidence after receiver begin
stores a session. With
`--dry-run`, it emits a `transfer=dry_run` binding and exits after profile,
pairing, network material, local TLS identity, scan, and manifest preflight; it
does not contact the receiver and does not write session artifacts.

## Transport Contract

- Wire format: JSON over HTTP for the minimal library implementation.
- Protocol version: `supermover/1`.
- Trust boundary: callers must run the handler behind TLS 1.3 mutual
  authentication that validates the peer's live leaf SPKI SHA-256 hash against
  pairing receipt/profile pins from the profile SSOT.
- Application binding: begin requests must still carry and validate profile ID,
  target ID, source device ID, target device ID, optional root ID, and protocol
  version. The serving profile supplies the expected target ID and target root;
  the request value is an application-scope binding, not proof of trust. TLS
  authenticates the channel, while receiver state remains the resume truth.
- Target control plane: the receiver writes session records, manifests, and
  receipts under `<target>/.supermover`.

## Endpoints

| Method | Path | Meaning |
| --- | --- | --- |
| `POST` | `/v1/sessions` | Begin or resume a session with a manifest. |
| `GET` | `/v1/sessions/{session_id}/status` | Return state and committed file offsets. |
| `POST` | `/v1/chunks` | Append one file chunk at the current offset. |
| `POST` | `/v1/commit` | Verify staged payloads and publish the session. |

`POST /v1/sessions` requires at least `protocol_version`, `session_id`,
`profile_id`, `target_id`, `source_device_id`, `target_device_id`, `created_at`,
and a manifest. Receiver session metadata and published receipts persist the
profile `target_id`, not the target device ID.

## Session State Semantics

The network receiver maps onto `internal/transaction` states:

| State | Receiver meaning |
| --- | --- |
| `validated` | Begin request and manifest were accepted and durably recorded. |
| `staged` | All payloads passed size and SHA-256 verification. |
| `published` | Staged files were promoted into the target tree and receipt was written. |
| `needs_repair` | Integrity or publish failure requires operator or future repair logic. |

Begin is idempotent when the same session metadata is replayed. If the session
exists with different metadata, the receiver returns a conflict.

## Chunk Upload Semantics

Each chunk request contains `session_id`, manifest `path`, byte `offset`, base64
JSON `data`, optional `sha256:` digest for that chunk, and an optional `final`
flag.

The protocol library limits decoded chunk payloads to 4 MiB. The HTTP handler
also wraps request bodies with a bounded reader slightly above that size to
account for JSON/base64 framing overhead. Larger transfer windows belong in a
future streaming transport, not unbounded JSON bodies.

The receiver accepts append-only writes:

- `offset == committed_size`: append and fsync the staged file.
- chunk fully inside committed bytes with identical content: return
  `duplicate` for retry idempotence.
- gaps, overlapping different bytes, unknown paths, oversized chunks, and
  chunks for non-file manifest entries: return conflict.

Status returns one `FileStatus` per file with `committed_size`; senders use that
offset to resume after interruption. The source must treat `committed_size` as
an authenticated receiver claim bounded by the manifest entry size, not as
permission to skip final source stability checks, manifest digest validation, or
receiver commit verification.

## Manifest and Receipt Adaptation

The protocol manifest is intentionally close to `control.Manifest`:

- file entries require `size` and `sha256:` digest;
- directory entries create target directories at commit;
- symlink entries require `symlink_target`;
- `target_path` is optional and defaults to `path`;
- entries may not appear below another entry that is a symlink, either in the
  source `path` tree or the effective target tree after applying `target_path`.

On begin, the receiver writes:

```text
<target>/.supermover/sessions/<session_id>/network-session.json
<target>/.supermover/sessions/<session_id>/session.json
<target>/.supermover/sessions/<session_id>/manifest.json
```

When `networkrun` has artifact access for a stored receiver session, including
through non-dry-run `push --network` after receiver begin accepts the session,
it also writes:

```text
<target>/.supermover/sessions/<session_id>/network-transfer.json
```

That document is an operator outcome artifact, not transport progress
authority. It records a validated session/profile/target/device binding,
attempt timestamps, current stage, and one of `started`, `published`,
`interrupted`, `auth_refused`, `needs_repair`, `publish_failed`, or `failed`.
Stages are bounded to `begin`, `status`, `chunk`, `commit`,
`warning_artifacts`, `network_transfer_artifact`, or `transport`. Begin-auth
refusal and transport setup failure can leave no receiver-side
`network-transfer.json`; preserve command output and target listing as evidence
for those cases. Zero-byte regular files are no longer a pre-begin rejection:
the profile-backed `push --network` path sends an explicit final empty
completion record and a clean publish should leave the usual receipt and
network-transfer outcome evidence. The wrapper persists protocol-client
warning records before it can mark the transfer `published`, and
`health`/`report` cross-check the transfer scope against receipts and session
state before surfacing it.
For profile-backed non-dry-run `push --network` attempts that are interrupted
after receiver begin and accepted payload bytes, `networkrun` may persist
in-flight `network-transfer.json` evidence for a same-profile, same-session
rerun. That artifact is bounded operator evidence only; receiver status remains
the resume authority, the profile remains the network SSOT, and this does not
add endpoint, certificate, privacy, or recovery runtime overrides.
For same-session published retries that upload no payload chunks, `networkrun`
reads the prior receiver-side `network-transfer.json` before writing a final
artifact. It may append a retry attempt while preserving the prior published
payload padding/batching overhead. It must not create fresh payload privacy
evidence from a zero-upload retry; if the prior published artifact is missing,
corrupt, mismatched, or lacks payload overhead counters, the retry records
`needs_repair` with `error_code=payload_overhead_missing` and blocks release
claims until reviewed.

On commit, the receiver verifies staged file size and digest, publishes into the
target tree, updates `session.json` to `published`, and writes:

```text
<target>/.supermover/sessions/<session_id>/receipt.json
```

Publishing is conservative. The receiver refuses to overwrite an existing file
or symlink unless the existing target object is content-identical to the
manifest entry. This keeps resume/idempotent commit safe while avoiding
destructive replacement of unrelated target data. Manifests also reject duplicate
effective target paths before begin.

## Authenticated Transport Integration Points

- `serve` starts a transfer receiver server from a paired profile only when the
  profile has complete `network.receiver_url` and `network.local_tls_identity`.
  The low-information pairing listener stays separate from the mTLS receiver
  listener.
- Internal profile-to-server adapter:
  `receiver.NewAuthenticatedHandlerFromProfile` validates
  `pairing.ValidateProfileTrust`, supplies target root, expected target ID,
  source device ID, and target device ID, and rejects unpaired or mismatched
  profiles. It is mounted by `serve` through the TLS receiver adapter only in
  paired receiver mode.
- Profile-bound TLS receiver adapter: `receiver.NewTLSReceiverFromProfile`
  validates profile/receipt trust, verifies the serving certificate's leaf SPKI
  hash matches the profile-pinned target device ID, configures TLS 1.3 with
  required client certificates, and wraps the receiver handler with TLS-derived
  peer context.
- Receiver authentication adapter: `receiver.NewAuthenticatedHandler` requires
  a transport-authenticated peer context before begin/status/chunk/commit
  routes can reach the store, and then checks begin request scope plus stored
  session metadata against the authenticated profile/device pins.
- TLS 1.3 server setup with required client certificates and peer leaf SPKI
  SHA-256 pin verification before exposing the HTTP handler.
- Source-side TLS client setup validates the target certificate pin and sends
  the source certificate for mTLS. Non-dry-run `push --network` loads the
  profile-selected local TLS identity and uses it for this source-side client
  path.
- Source-side protocol client: `internal/protocolclient` can build a begin
  request from `scan.Result`, skip reserved `.supermover` and unsupported
  special entries with warnings, stream regular files from disk in bounded JSON
  chunks, resume from receiver `GET /v1/sessions/{session_id}/status`
  committed offsets, retry commit for staged/published sessions, and commit
  through the receiver protocol. It treats status, not source-local progress or
  transport acknowledgments, as the resume authority. It is used by non-dry-run
  `push --network` through the profile-backed TLS/mTLS adapter and
  `networkrun`, which persists returned warnings before marking a transfer
  published. Even staged/published commit retries revalidate source evidence
  before commit; warning persistence failure prevents a published
  network-transfer outcome.
- Network-runner wrapper: `internal/networkrun` wraps the protocol client for
  non-dry-run `push --network` and persists `network-transfer.json` plus
  warning artifacts before exposing a `published` outcome to operators.
  Receiver-status retries that imply previously accepted payload bytes require
  auditable prior payload-overhead proof. Already published zero-payload
  retries preserve prior published proof; missing, unreadable, mismatched, or
  payload-empty prior proof fails closed with `payload_overhead_missing`.
- Receiver-offset resume loop: implemented inside the library source client for
  JSON/HTTP receiver protocol runs. Current `push --network` can resume from
  receiver status offsets during a same-session retry only when
  `networkrun` can preserve or merge auditable prior payload-overhead evidence.
  Automated evidence now includes the bounded profile-backed CLI/Runner case
  where receiver-accepted payload bytes are followed by a source-side network
  interruption or simulated transport failure and a same-profile, same-session
  rerun recovers from receiver status. It also includes a command-level
  receiver listener restart fixture over preserved target control-plane state,
  commit-only and published-session retry evidence at the `networkpush` layer,
  missing/corrupt/mismatched prior payload-overhead evidence that fails closed
  with `payload_overhead_missing`, and deterministic internal `networkrun`
  evidence for a source stop immediately after durable in-flight chunk progress
  evidence, followed by same-session receiver-status resume and merged
  privacy-overhead evidence. Broad arbitrary interruption recovery, network
  `recover`, broad resume acceptance, receiver-side recovery UX, arbitrary
  process-kill recovery, daemon restart recovery, power-loss recovery, and
  general interruption/restart release acceptance remain planned or unwired.
- Zero-byte regular files: the receiver protocol now uses an explicit final
  completion record with `offset=0`, empty `data`, and the empty-file SHA-256
  digest when present. The receiver marks a zero-byte file complete only after
  staged evidence exists and commit refuses to infer completion from a missing
  staged file or a pre-existing target file. The profile-backed
  operator-facing `push --network` path now sends that completion through
  `protocolclient`, `networkpush`, and the CLI end to end. This is file
  transfer support only; LAN browsing, daemon workflow, ongoing sync, broad
  arbitrary interruption recovery, broad resume acceptance, and arbitrary
  process-kill recovery remain planned. Anonymity is not claimed; strong
  anonymity is out of scope.

0-RTT or replayable early data must not be used for `POST /v1/sessions`,
`POST /v1/chunks`, or `POST /v1/commit`; any future QUIC/HTTP3 adapter inherits
the same rule. A transport acknowledgment is not progress evidence; only
receiver `committed_size` status, staged file digest verification, and published
receipts drive resume and completion.

## Network Recovery Acceptance Matrix

The current acceptance matrix is intentionally bounded to profile-backed,
same-profile, same-session `push --network` reruns. Receiver status is the
resume authority; `network-transfer.json` is auditable payload-overhead
evidence and release evidence, not an independent permission to resume.

| Mode | Current acceptance |
| --- | --- |
| Pre-begin profile, TLS, or auth failure | Fails closed before receiver payload state. Evidence is command output and target control-plane inspection. |
| Partial payload accepted, source/network interrupted | Supported for the bounded same-session CLI/Runner path. Rerun resumes from receiver status, publishes matching target content, and leaves clean `health`, `status`, and `report` review. |
| Receiver listener restarted after partial payload | Supported when the receiver is restarted over the same profile-selected target/control-plane state and the same session is retried with prior auditable payload-overhead evidence. |
| All payload accepted, interruption before commit | Supported at the `networkpush` layer as commit-only retry: no payload reupload, one commit retry, and preserved prior overhead evidence. |
| Already published same-session retry | Supported as `published_retry`: no chunks uploaded and prior published payload-overhead evidence is preserved. |
| Missing, corrupt, mismatched, non-published, or payload-empty prior transfer evidence | Blocked. The retry records `needs_repair` with `error_code=payload_overhead_missing` and does not fabricate overhead or claim recovery. |
| Changed source/profile/manifest/session scope | Blocked by source evidence validation, receiver metadata conflict, or prior-evidence scope checks. |
| Network `recover`, automatic retry/reconcile, daemon or OS-service restart recovery, arbitrary process kill, power loss, or receiver crash with unsupervised write boundaries | Future work. These modes stay outside the current release claim until they have command/process-level fixtures and documented evidence. |
