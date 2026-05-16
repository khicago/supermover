# Network Protocol

Supermover v1 is intended to run behind an authenticated, encrypted transport
boundary. This slice defines only the stable receiver protocol and a testable
server library. CLI wiring, pairing lookup, and TLS or Noise setup remain
integration work.

## Transport Contract

- Wire format: JSON over HTTP for the minimal library implementation.
- Protocol version: `supermover/1`.
- Trust boundary: callers must run the handler behind a transport that
  authenticates the pinned source and target device IDs from the profile SSOT.
- Target control plane: the receiver writes session records, manifests, and
  receipts under `<target>/.supermover`.

## Endpoints

| Method | Path | Meaning |
| --- | --- | --- |
| `POST` | `/v1/sessions` | Begin or resume a session with a manifest. |
| `GET` | `/v1/sessions/{session_id}/status` | Return state and committed file offsets. |
| `POST` | `/v1/chunks` | Append one file chunk at the current offset. |
| `POST` | `/v1/commit` | Verify staged payloads and publish the session. |

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
offset to resume after interruption.

## Manifest and Receipt Adaptation

The protocol manifest is intentionally close to `control.Manifest`:

- file entries require `size` and `sha256:` digest;
- directory entries create target directories at commit;
- symlink entries require `symlink_target`;
- `target_path` is optional and defaults to `path`.

On begin, the receiver writes:

```text
<target>/.supermover/sessions/<session_id>/network-session.json
<target>/.supermover/sessions/<session_id>/session.json
<target>/.supermover/sessions/<session_id>/manifest.json
```

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

## Mainline Integration Points

- CLI command to start a receiver server from a profile.
- Profile-to-server adapter that supplies target root, expected target ID, and
  accepted source pairing receipt.
- Encrypted authenticated transport setup before exposing the HTTP handler.
- Source-side client that chunks large files and uses status offsets for resume.
