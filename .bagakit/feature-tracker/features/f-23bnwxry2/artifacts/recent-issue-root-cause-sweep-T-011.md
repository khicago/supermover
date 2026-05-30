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
4. i18n appeared complete by resource-key scan but still left visible English
   in Control Room, transfer state, and Evidence interpretation panels.
5. The global language selector sat beside the sidebar brand title, causing
   "SuperMover" to wrap and mixing settings chrome with product identity.

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
- Follow-up root cause: after switching to native pinned headers, the shared
  header still had only one visual state. Scrolling pinned the full expanded
  banner forever instead of collapsing into top chrome. Owner pages also kept a
  fixed owner title above a child detail page title, creating a two-title stack
  on Connect, Move, and Verify/Repair.
- Follow-up root cause: the first compact-header pass changed typography and
  padding, but the compact state still read like an in-page strip rather than a
  fused top chrome surface.
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
- Result: fixed owner-mode strips now render only compact mode controls, and
  `DetailPageHost` switches from expanded banner to compact integrated header
  chrome after scroll threshold hysteresis. The Sync page's first card title is
  also distinct from its page title.
- Result: compact detail headers now use a shared full-bleed material treatment
  with edge divider, shadow, and top-in transition so the scroll state visibly
  merges into top chrome.
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

### Language Control Ownership

- Root cause: `UIPreferencesStore.language` was a single global state, but its
  control was rendered inside `sidebarHeader` next to the product brand. That
  made a settings action compete with identity chrome and created the observed
  title wrapping in constrained sidebar widths.
- Sweep: checked `ContentView`, Settings display preferences, UI preference
  resources, sidebar navigation tests, app localization tests, and resource
  keys for the old `globalLanguageMenu`, `appChrome.language.title`, and
  "language lives in global app chrome" wording.
- Result: the sidebar header is brand-only, the product title is constrained to
  one line, and the single language binding now lives in Settings > Display
  Preferences beside appearance. No per-page language switchers were added.
- Boundary: this is the owner-placement fix for the global preference control.
  It does not change the underlying locale persistence model or translate raw
  audit evidence.
- Key refs:
  - `macos/SuperMoverApp/ContentView.swift`
  - `macos/SuperMoverApp/AppChromeLocalization.swift`
  - `macos/SuperMoverApp/Resources/en.lproj/Localizable.strings`
  - `macos/SuperMoverApp/Resources/zh-Hans.lproj/Localizable.strings`
  - `macos/SuperMoverAppTests/UIPreferencesTests.swift`
  - `macos/SuperMoverAppTests/WorkbenchNavigationTests.swift`

### Visible I18n Coverage

- Root cause: the previous localization guard proved literal
  `AppChromeLocalization.text("...")` calls had packaged `en` and `zh-Hans`
  resources, but it did not prove every visible display-model string used the
  localization owner. Control Room and Evidence still had strings built from
  raw `title/subtitle/label/value` literals, `.title.capitalized`, direct
  `artifact.family.title`, and interpolated English count messages.
- Sweep: checked Control Room/homepage model chrome, transfer status models,
  task input summaries, Evidence record/detail models, artifact catalog cards,
  raw-evidence empty states, JSON status/severity labels, localized resources,
  and source-guard tests. Raw evidence values and identifiers were separated
  from interpretation-layer labels.
- Result: visible interpretation-layer chrome now uses
  `AppChromeLocalization`, `GateState.localizedTitle(using:)`, and
  `EvidenceArtifactFamily` localized helpers. Resource scans report 0 missing
  keys and 0 duplicate keys in both language packs. Source guards now pin the
  specific bypass patterns that caused this defect class.
- Boundary: raw CLI output, stdout/stderr payloads, JSON/protocol field values,
  file paths, artifact raw identifiers, receipt IDs, task raw values, and
  operator-entered text remain literal audit material.
- Key refs:
  - `macos/SuperMoverApp/ContentView.swift`
  - `macos/SuperMoverApp/EvidenceSectionView.swift`
  - `macos/SuperMoverApp/EvidenceArtifactLocalization.swift`
  - `macos/SuperMoverApp/WorkbenchSupport.swift`
  - `macos/SuperMoverApp/Resources/en.lproj/Localizable.strings`
  - `macos/SuperMoverApp/Resources/zh-Hans.lproj/Localizable.strings`
  - `macos/SuperMoverAppTests/UIPreferencesTests.swift`
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
- `swift test --package-path macos --filter 'WorkbenchChromeTests|WorkbenchNavigationTests|UIPreferencesTests'`
  passed with 33 tests after owner-title deduplication and compact-header
  guards.
- `swift test --package-path macos --filter 'WorkbenchChromeTests|WorkbenchNavigationTests|UIPreferencesTests'`
  passed after pinning full-bleed compact header chrome and global language-menu
  ownership.
- `plutil -lint macos/SuperMoverApp/Resources/en.lproj/Localizable.strings macos/SuperMoverApp/Resources/zh-Hans.lproj/Localizable.strings`
  passed.
- `swift test --package-path macos --filter 'WorkbenchChromeTests|WorkbenchNavigationTests|UIPreferencesTests'`
  passed with 38 selected tests after the Control Room/Evidence i18n coverage
  follow-up.
- `swift build --package-path macos --product SuperMoverApp` passed.
- Literal localization key/resource scan passed with 0 missing keys and 0
  duplicate keys for `en` and `zh-Hans`.

## Closeout Boundary

The current pass adds process rules, durable evidence, and app-chrome/i18n
guardrails; it does not claim real two-Mac installed-app transfer evidence,
signed/notarized distribution readiness, Local Network/firewall prompt
evidence, or Merkle/current-source proof.
