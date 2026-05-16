# Operation Review

Supermover keeps migration review data in the target `.supermover` control
plane. Profiles remain the SSOT for configuration; review commands consume
artifacts and do not invent migration policy outside the profile.

## Verify Review

The `internal/verify` package builds an operator review report from:

- `.supermover/sessions/<session>/manifest.json`
- `.supermover/warnings/*.json`
- `.supermover/deleted/*.json`

Default review only consumes sessions with a valid
`.supermover/sessions/<session>/receipt.json` whose status is `published`.
Manifests, warnings, and soft-delete records from sessions without a published
receipt are treated as recovery evidence rather than review/prune input. Use
`health` and `recover` first to bring the session to a terminal state or to mark
it `needs_repair`.

For the selected manifest, it verifies target files by safe relative target
path, size, and `sha256:` digest. Non-file entries are included in manifest
summaries but are not hashed. Invalid JSON or unreadable artifacts are retained
as report problems instead of aborting the whole review.

`supermover verify --profile <path>` calls this package through the profile
target and renders either text or JSON. A non-zero exit means the selected
manifest has error findings, artifact problems, or no manifest at all.

## Soft Delete Review

The `internal/deleted` package compares the previous manifest with the current
source scan and emits `control.SoftDelete` records for paths that disappeared
from the source. It records intent only:

- no target files are physically deleted
- directory entries are skipped
- record IDs are deterministic from session and paths
- records include profile, target, root, previous session, previous manifest,
  kind, size, and digest evidence when known
- timestamps are caller supplied for reproducible tests

The local push flow now:

1. Read the latest trusted manifest for the same profile ID, target ID, and
   root.
2. Scan the current source root from the profile.
3. Call `deleted.Generate`.
4. Persist records under `.supermover/deleted/<id>.json`.
5. Let a future explicit prune/delete command perform physical deletion after
   user review.

## Mainline Integration Points

Remaining integration:

- `deleted` still needs an explicit reviewed physical prune command.
- `verify` should eventually include directory, symlink, and metadata checks
  beyond regular-file size and digest.
