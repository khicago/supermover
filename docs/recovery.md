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

The public `health --profile <path>` command exposes this scanner as a read-only
operator check. It uses the profile SSOT to find `target.local_path`, reports
incomplete or invalid session records, and returns non-zero when follow-up is
needed.

The public `recover --profile <path>` command executes the conservative local
recovery subset:

- `staged` sessions are replayed from the durable manifest and stage directory.
  Staged file size and SHA-256 digest must match the manifest before
  publication. File publication still uses no-replace semantics. Existing final
  files are accepted only when size and digest match the manifest.
- `received` and `validated` sessions are not silently discarded. `recover
  --dry-run` reports the rollback action; `recover --rollback-incomplete`
  explicitly marks them `rolled_back` when the operator decides they never
  reached durable staging.
- sessions that cannot be automated, such as missing manifests or divergent
  final target content, are marked `needs_repair` with the failure note retained
  in `session.json`.

Open recovery work:

- invalid session records may need a quarantine directory.
- publish reconciliation should eventually verify the full receipt, manifest,
  and target state matrix before changing session state.
