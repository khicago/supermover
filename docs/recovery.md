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
2. Rename the temporary file to the final path with `os.Rename`.
3. Best-effort sync the final parent directory on Unix-like platforms.

The rename step is the atomic boundary. Callers are responsible for creating the
temporary file in the same filesystem as the final path and for creating and
validating the final parent directory before promotion. Cross-device renames are
reported as I/O errors.

`internal/durable.PromoteFileNoReplace` is the safer publish primitive for data
paths that must never overwrite existing target content. It syncs the staged
file and tries an atomic hard-link publish. For cross-device link failures, it
copies to a temporary file in the final directory, syncs that file, then
hard-links the temporary file into the final path before cleanup. Filesystems
without hard-link support are rejected so a crash cannot leave a partial final
file. A crash during the fallback copy can leave a `.supermover-promote-*`
temporary file in the final directory; operators may delete that orphan after
confirming it is not referenced by a manifest or session stage. Callers still
own parent-directory creation and path safety.

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
  publication unless the final file already exists and matches the manifest.
  New final files still use no-replace semantics. Existing final files are
  accepted when size and digest already match the manifest. Changed managed
  regular files may be atomically replaced only when the manifest carries
  previous-session evidence and the current target still matches that previous
  size, `sha256:` digest, mode, and modification time. If the final path is
  missing for a changed-file replacement, recovery treats it as repair-needed
  because the previous target evidence can no longer be verified automatically.
  Recovery does not create or consume automatic backup sidecars for managed
  replacements.
  Directory and symlink entries are also checked before recovery publish so an
  unsafe or conflicting non-file entry cannot create a partial target update.
- `received` and `validated` sessions are not silently discarded. `recover
  --dry-run` reports the rollback action; `recover --rollback-incomplete`
  explicitly marks them `rolled_back` when the operator decides they never
  reached durable staging.
- sessions that cannot be automated, such as missing manifests or divergent
  final target content, are marked `needs_repair` with the failure note retained
  in `session.json`.

Open recovery work:

- invalid session records may need a quarantine directory.
- broader automatic repair may eventually reconcile more target drift cases
  after preserving the existing audit evidence.
