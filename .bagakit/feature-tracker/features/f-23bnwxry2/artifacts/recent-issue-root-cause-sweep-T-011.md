# T-011 Recent Issue Root-Cause Sweep

## Scope

This pass applies the repository rule that a discovered issue requires a root
cause, a sibling sweep, and explicit evidence before closeout.

Recent issues checked:

1. Source Prepare appeared to ask the source Mac for a target-owned destination.
2. File actions such as Browse/New previously leaked default-looking native
   button styling.
3. Workbench pages drifted under scroll/resize, with headers too far from the
   window top or capable of moving out of view.

## Findings

### Source/Target Setup Ownership

- Root cause: the early Prepare model treated source-side profile creation as
  if both local roots belonged to the same app session. That contradicted the
  product model: each Mac owns and validates only its own local filesystem,
  while the profile remains the shared SSOT.
- Sweep: checked app role rendering, task argument construction, run gates,
  source/target root requirements, profile destination planning, setup-guide
  tests, command-preview tests, and CLI profile tests.
- Result: current code keeps Source creation source-only. Source Prepare
  renders only the source root selector, `profile init` builds
  `--source-only`, profile-init gates require only a readable source root, and
  Target remains the only setup path that requires a writable target root
  before `profile set-target`.
- Boundary: a source-only profile is not migration-ready until Target updates
  the same profile with its target-owned root. This is expected pending state,
  not a completed transfer setup.
- Key refs:
  - `macos/SuperMoverApp/ContentView.swift`
  - `macos/SuperMoverApp/AppStore.swift`
  - `macos/SuperMoverAppTests/AppStoreTests.swift`
  - `internal/profile`
  - `internal/cli`

### Button Chrome

- Root cause: earlier file actions exposed default AppKit/SwiftUI button chrome
  instead of the workbench's icon-first action components.
- Sweep: searched app code for direct `Button` construction, `ActionButton`,
  `CompactActionButton`, `IconActionButton`, file browse actions, profile
  destination actions, acceptance-bundle browsing, operator-artifact browsing,
  sidebar rows, evidence rows, and task/category chips.
- Result: no current raw Browse/New file-action leak was found. File-style
  actions now route through `CompactActionButton`; command actions use
  `ActionButton` or `PrimaryActionButton`; the remaining direct `Button` sites
  are locally styled selectors, rows, chips, or collapsers with
  `.buttonStyle(.plain)` and custom surfaces.
- Boundary: this was an inspection result only. No additional code change was
  needed in this pass.
- Key refs:
  - `macos/SuperMoverApp/WorkbenchChrome.swift`
  - `macos/SuperMoverApp/ContentView.swift`
  - `macos/SuperMoverApp/EvidenceSectionView.swift`
  - `macos/SuperMoverApp/SidebarNavigation.swift`
  - `macos/SuperMoverApp/AcceptanceBundlePanel.swift`
  - `macos/SuperMoverApp/AcceptanceOperatorEvidencePanel.swift`

### Workbench Layout And Top Inset

- Root cause: owner toolbars and page bodies previously lived in the same body
  scroll path, and the workbench used one vertical padding metric for both top
  and bottom spacing. That allowed title/header context to move with content
  and made compact windows feel overly pushed down.
- Follow-up root cause: `DetailPageHost` tried to keep page headers visible by
  measuring scroll position and applying an offset, but the header still
  belonged to the scroll view's layout flow. Real scrolling can clip or cull
  that original layout frame, so Prepare and sibling detail pages could still
  lose their title.
- Sweep: checked the shared workbench chrome metrics, fixed owner-mode toolbar
  path, sidebar navigation, `DetailPageHost` sticky-header behavior, and every
  top-level/merged section route. Control Room remains a custom page, but it
  still shares the outer main-content top inset.
- Result: Connect, Move, and Verify/Repair owner toolbars are fixed outside
  the page body scroll; sidebar navigation scrolls independently; page-top
  spacing now uses a dedicated top inset while retaining bottom breathing room.
  `DetailPageHost` now uses native pinned section headers, so Prepare, Task
  Dispatch, Settings, and other shared detail hosts keep their page header
  anchored during body scroll.
- Boundary: this is layout-stability evidence only. It does not replace a
  future full visual QA pass across real window sizes.
- Key refs:
  - `macos/SuperMoverApp/WorkbenchChrome.swift`
  - `macos/SuperMoverApp/ContentView.swift`
  - `macos/SuperMoverApp/DevicesSectionView.swift`
  - `macos/SuperMoverApp/PairingSectionView.swift`
  - `macos/SuperMoverApp/TransferSectionView.swift`
  - `macos/SuperMoverApp/EvidenceSectionView.swift`
  - `macos/SuperMoverAppTests/WorkbenchChromeTests.swift`
  - `macos/SuperMoverAppTests/WorkbenchNavigationTests.swift`

## Validation

- `swift test --package-path macos --filter 'AppStoreTests/test(ProfileDestinationPlanInitializesNewSourceProfileWhenSourceIsReady|ProfileInitCommandPreviewUsesSourceOnlyAndIgnoresTargetPathInput|TaskRunGateAllowsProfileInitWithReadableSourceOnly|SetupGuideExplainsEmptySourcePreparationInUserOrder|LocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide)|WorkbenchChromeTests|WorkbenchNavigationTests'`
  passed with 21 tests.
- `go test -count=1 ./internal/profile ./internal/cli -run 'Test(NewDefault|NewSourceOnly|ProfileInit|ProfileSetTarget)'`
  passed.
- `bash "$BAGAKIT_FEATURE_TRACKER_SKILL_DIR/scripts/feature-tracker.sh" validate-tracker --root .`
  passed.
- `git diff --check` passed.
- `swift test --package-path macos --filter 'WorkbenchChromeTests|WorkbenchNavigationTests|UIPreferencesTests'`
  passed with 31 tests after replacing the old offset shim with a native pinned
  `DetailPageHost` header.

## Closeout Boundary

No new code defect was found by this sweep beyond the already-corrected source
ownership and layout issues. The current pass adds process rules and durable
evidence; it does not claim real two-Mac installed-app transfer evidence,
signed/notarized distribution readiness, Local Network/firewall prompt
evidence, or Merkle/current-source proof.
