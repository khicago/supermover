# Troubleshooting Matrix

Use this matrix to choose safe operator actions. Prefer collecting control-plane
evidence before rerunning or deleting files.

| Symptom | Likely cause | Evidence to collect | Safe action |
| --- | --- | --- | --- |
| `profile init: ... already exists` | A profile already exists at the chosen path. | Existing profile path and desired source/target. | Edit the existing profile deliberately, then run `profile lint`. Do not overwrite it to change policy silently. |
| `profile init: --profile, --source, and --target are required` | Missing required initialization flag. | Exact command line. | Rerun with all three flags. |
| `profile set-target: --profile and --target are required` | Missing target update flag. | Exact command line. | Rerun with both flags, then lint the profile. |
| `profile lint` fails with validation errors | Profile violates schema or safety policy. | Full profile file and stderr. | Fix the profile. Do not bypass with runtime flags; the profile is the SSOT. |
| `delete_policy.require_review must be true when mode is prune` | Physical prune was enabled without review. | Profile delete policy section. | Set `require_review: true`, review retention and prune policy, then lint again. |
| `plaintext privacy mode without explicit plaintext restore approval` or equivalent validation failure | Target plaintext restore was not explicitly accepted. | Profile privacy policy section. | Set `allow_plaintext_restore` only if the target is trusted; otherwise do not use v1 plaintext restore. |
| Traffic level 2 validation fails | Required padding, batching, jitter, or low-information discovery fields are missing. | Profile privacy policy section. | Restore level 2 fields or choose an explicitly different privacy posture in the profile. |
| `scan: scan root "...": not a directory` | Source root is missing, mistyped, or not mounted. | Profile roots and `test -d` result. | Restore or mount the source path, or edit the profile root and lint. |
| Dry run reports unexpected entry count | Include/exclude rules or source root are wrong, or source changed. | Profile, dry-run output, `scan` output. | Stop. Inspect the source and profile before publishing. |
| Dry run reports warnings | Scanner found audit-relevant conditions expected to become warning records. | Dry-run output and later warning JSON if published. | Decide whether each warning is acceptable. Rerun after profile/source changes if needed. |
| Published run reports warnings | Local push completed but wrote reviewable warning artifacts. | `.supermover/warnings/*.json`, session receipt, manifest. | Review every warning. Accept, rerun with changed profile/source, or block release. |
| Symlinks are not copied | Current local push slice does not implement symlink copy. | Warning with code similar to `symlink_not_copied`; manifest entry for the path. | Decide whether the target tree is acceptable. If symlink fidelity is required, block until mainline implements it. |
| Target lacks `.supermover` after dry run | `push --dry-run` intentionally does not write target state. | Command line containing `--dry-run`. | Run non-dry-run push when ready to publish. |
| Target lacks `.supermover` after non-dry-run push | Push failed before control artifacts, wrong target path, or permission issue. | stdout/stderr, exit code, target path, filesystem permissions. | Fix the failure and rerun with a new session ID or cleanly documented retry decision. |
| Session receipt status is not `published` | Interrupted or incomplete transaction state. | `.supermover/sessions/<session>/`, receipt/session files, command logs, `health --profile` output. | Preserve evidence. Use `health` for read-only classification. Use planned `recover` once available; for now, triage manually before rerun. |
| Manifest exists but restored files are missing | Target was modified after publish, publish failed, or wrong target was inspected. | Manifest, session receipt, target path listing, drift notes if any. | Preserve `.supermover`; compare manifest paths to target; do not prune or delete evidence. |
| Warning files are deleted to "clean up" the target | Audit trail was damaged. | Backups, session output showing warning count. | Restore warning files from backup if possible. Treat the run as not fully auditable if warnings cannot be reconstructed. |
| Agent influence count is unexpected | Agent rule/state files were detected or omitted by profile/source rules. | `.supermover/agent/<session>-influence.json`, profile `agent_knowledge`, source tree. | Review whether these files should be migrated and cataloged. Edit profile rules if needed. |
| Discovery shows an unknown target | LAN discovery is unauthenticated address hinting. | Discovery advertisement fields and network context. | Do not trust or push to it. Pair only after explicit verification and identity pinning. |
| Discovery advertisement contains hostname, username, path, profile name, or inventory size | Discovery implementation leaked more than low-information hints. | Raw advertisement/TXT fields. | Block release of discovery. Advertisements must be limited to service, protocol, nonce, and minimal capabilities. |
| Pairing target identity differs from profile target | Device identity pinning caught a target mismatch. | Pairing receipt, profile target section, target device key/fingerprint. | Stop. Verify the physical/logical target. Do not update the profile unless the target change is intentional and reviewed. |
| Disk full or quota error during publish | Target filesystem cannot accept staged or final files. | stderr, free-space output, session state under `.supermover`. | Free space, preserve session evidence, then retry or recover according to session state. |
| Cross-device rename or promotion failure | Temporary and final files were not on the same filesystem. | Error text, source/target mount information, session state. | Keep staging on the same target filesystem; rerun after layout fix. |
| Operator wants to change delete, privacy, or metadata behavior for one run | Runtime override would break auditability. | Requested change and profile diff. | Change the profile, lint it, and keep the profile snapshot. |
| Operator wants to change the local target for one run | Target selection is profile-owned. | Requested target path and current profile target section. | Run `profile set-target --profile <path> --target <target>`, lint, then push. Do not change `target.target_id` unless the target identity changes intentionally. |

## Evidence Commands

```bash
go run ./cmd/supermover profile lint --profile ./supermover.profile.json
go run ./cmd/supermover profile set-target --profile ./supermover.profile.json --target /path/to/target
go run ./cmd/supermover scan --profile ./supermover.profile.json
go run ./cmd/supermover health --profile ./supermover.profile.json
find /path/to/target/.supermover -maxdepth 4 -type f | sort
sed -n '1,160p' /path/to/target/.supermover/sessions/<session>/receipt.json
find /path/to/target/.supermover/warnings -type f -name '*.json' -maxdepth 1 2>/dev/null | sort
```

## Mainline Commands Needed To Close The Matrix

The matrix references current commands and the planned operational surface:

```bash
go run ./cmd/supermover recover --profile ./supermover.profile.json --session <session-id>
go run ./cmd/supermover verify --profile ./supermover.profile.json --session <session-id>
go run ./cmd/supermover deleted list --profile ./supermover.profile.json
go run ./cmd/supermover drift list --target /path/to/target --profile ./supermover.profile.json
go run ./cmd/supermover prune --target /path/to/target --profile ./supermover.profile.json --dry-run
```
