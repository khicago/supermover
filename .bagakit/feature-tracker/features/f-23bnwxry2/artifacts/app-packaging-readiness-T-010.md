# T-010 macOS Packaging, Permissions, And Daemon Controls

## Scope

This artifact records the macOS packaging/readiness implementation slice for
T-010.

Implemented in this slice:

- The app exposes CLI provenance/readiness in Settings, distinguishing bundled,
  development build-and-exec, and unavailable CLI modes.
- The app can run `supermover version` from the currently resolved CLI path as a
  manual provenance check.
- The packaged app build script writes
  `Contents/Resources/supermover-provenance.json` with build profile, git
  commit, dirty-worktree flag including non-ignored untracked files, CLI
  version, bundled relative path, build time, and signing mode.
- Packaged app readiness requires an executable bundled CLI plus a readable,
  complete provenance manifest. Missing, malformed, or incomplete packaged
  provenance is blocked; unsigned, ad-hoc signed, or dirty packaged provenance
  remains review-only local evidence.
- Packaged apps do not fall back to the development build-and-exec launcher when
  `Contents/Resources/bin/supermover` is missing.
- The build script supports optional signing through
  `SUPERMOVER_CODESIGN_IDENTITY` and verifies the signed app. Leaving the
  variable empty produces an explicitly unsigned local app.
- `Info.plist` contains a Local Network usage description for explicit
  discovery, pairing, serve, and profile-pinned migration commands.
- `SuperMover.entitlements` records signing intent for network client/server
  and user-selected read-write file access.
- Settings now exposes foreground daemon install/run/restart/stop/status/logs
  controls. `daemon run` is supervised in a dedicated foreground process slot;
  restart and stop write CLI intent artifacts with an explicit operator reason.
  Source and target Settings both expose the reason input required for those
  intent-writing daemon controls.
- Docs state that daemon controls do not install an OS-managed detached service
  and that notarization remains an external release step.

Not implemented in this slice:

- Automatic notarization, stapling, Apple credential management, or a CI release
  pipeline.
- launchd/SMAppService detached daemon installation.
- Automatic Local Network/firewall approval detection.
- New key material storage in the app. Pairing and transport secrets remain
  profile/control-plane owned.
- Final two-machine acceptance evidence. That remains T-011.

## Validation

- `swift test --package-path macos --filter 'AppStoreTests/testVersionAndForegroundDaemonCommandsUseExplicitBoundary|AppStoreTests/testCLIProvenanceReportsRunnableMode'`
- `swift test --package-path macos --filter 'AppStoreTests/testPackagedCLIProvenanceBlocksMissingCLIWithoutDevelopmentFallback|AppStoreTests/testPackagedCLIProvenanceGatesMalformedUnsignedAdHocAndDirtyBundles|AppStoreTests/testCLIProvenanceReportsRunnableMode'`
- `swift test --package-path macos`
- `sh -n macos/script/build-app.sh`
- `macos/script/build-app.sh`
- `jq . macos/dist/SuperMover.app/Contents/Resources/supermover-provenance.json`
- `test -x macos/dist/SuperMover.app/Contents/Resources/bin/supermover && macos/dist/SuperMover.app/Contents/Resources/bin/supermover version`
- `plutil -lint macos/script/Info.plist macos/script/SuperMover.entitlements`
- `feature-tracker validate-tracker --root .`
- `git diff --check`

The local packaging run produced an unsigned app at
`macos/dist/SuperMover.app`. Its provenance manifest used schema
`supermover.macos.provenance.v1`, build profile `local-release`, signing
`unsigned`, and bundled CLI version `supermover 0.1.0-dev`. `git_dirty` was
`true` because this verification ran in an active working tree with unrelated
dirty files still present.

Full Swift package testing also caught and fixed an Evidence Vault refusal
ordering issue: when a confirmed review-metadata action no longer matched
current app inputs, the app now reports the stale preview before generic missing
loaded evidence. If inputs still match but the loaded evidence is gone, the app
continues to report missing loaded evidence.

Stage review found and the implementation fixed four packaging/readiness issues:
packaged executable readiness now requires usable provenance, unsigned or dirty
bundles no longer render as pass, broken packaged apps do not fall back to the
development launcher, target daemon stop/restart has a visible reason input, and
the dirty bit includes non-ignored untracked files.

Re-review also found that ad-hoc signing (`SUPERMOVER_CODESIGN_IDENTITY=-`)
could still render as pass and that packaged failure cases were not deterministic
unit tests. The resolver now treats ad-hoc signing as review-only local evidence
and the Swift test suite covers missing bundled CLI/no fallback, malformed and
incomplete provenance, unsigned, ad-hoc, dirty, and signed-clean provenance.

## Review Notes

- Local development runs may report `development launcher` rather than
  `bundled`; this is expected before `script/build-app.sh` creates the app
  bundle.
- A signed app is still not a notarized release. Do not present local signing or
  ad-hoc signing as source/target install readiness for non-developer machines.
