# Recovery Foundation

Supermover records local transfer progress as durable session records under the
target control directory. The current foundation is local-only: it defines file
layout, state transitions, atomic same-filesystem promotion, and recovery
classification. It does not perform network synchronization.

## Control Directory Layout

Each target has a control directory managed by higher-level packages. Transaction
helpers use this structure inside it:

```text
<control>/
  sessions/
    <session-id>/
      session.json
      manifest.json
      stage/
```

`session.json` is the durable transaction record. `manifest.json` is reserved for
the transfer manifest. `stage/` holds temporary payload files before they are
published into their final locations.

Session IDs are deliberately conservative. They may contain letters, numbers,
dot, underscore, and dash, but no path separators or `..` segments.

## Session States

The transaction package recognizes these states:

| State | Meaning |
| --- | --- |
| `received` | A session was accepted, but not yet validated or staged. |
| `validated` | Metadata passed validation, but payload files are not durably staged. |
| `staged` | Payload files and metadata reached the staging area. |
| `published` | Staged files were promoted to final paths. This is terminal. |
| `rolled_back` | Local staged work was discarded. This is terminal. |
| `needs_repair` | The session cannot be safely automated and needs operator or future repair logic. |

Recovery classification is intentionally simple:

| State | Action |
| --- | --- |
| `received` | `rollback` |
| `validated` | `rollback` |
| `staged` | `recover` |
| `published` | `none` |
| `rolled_back` | `none` |
| `needs_repair` | `repair` |

Invalid or unreadable records are reported separately by the scanner. They are
not silently rolled back because the safe action depends on why the record is
invalid.

## Durable Promotion

`internal/durable.PromoteFile` promotes one temporary file to one final path on
the same filesystem:

1. Sync the temporary file.
2. Create the final parent directory if needed.
3. Rename the temporary file to the final path with `os.Rename`.
4. Best-effort sync the final parent directory on Unix-like platforms.

The rename step is the atomic boundary. Callers are responsible for creating the
temporary file in the same filesystem as the final path; cross-device renames are
reported as I/O errors.

Directory sync is best-effort because platform support differs. Unix-like builds
attempt to sync the parent directory and ignore unsupported directory-sync
failures. Portable fallback builds no-op the directory sync.

## Error Statuses

Durable helpers classify common local failure modes:

| Status | Meaning |
| --- | --- |
| `ok` | No error. |
| `disk_full` | `ENOSPC`, quota exhaustion, or wrapped disk-full sentinel. |
| `interrupted` | Interrupted syscall or wrapped interrupted sentinel. |
| `validation_failure` | Caller passed invalid input. |
| `io_error` | Other filesystem failure. |

The status is exposed through `internal/durable.Error` and
`internal/durable.ClassifyError`.

## Recovery Scanner

`internal/transaction.ScanRecovery` reads `<control>/sessions/*/session.json`,
validates each record, filters terminal states, and returns incomplete sessions
with their recommended action. Results are sorted by session ID for stable
operator output and tests.

Current open policy choices for later workers:

- Whether `staged` recovery should replay manifest entries directly or first
  verify staged payload hashes.
- Whether invalid session records should move to a quarantine directory.
- Whether promotion should refuse to overwrite an existing final path or allow
  replace semantics for retry idempotence.
