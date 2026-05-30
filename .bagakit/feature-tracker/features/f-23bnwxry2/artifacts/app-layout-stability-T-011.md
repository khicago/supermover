# T-011 App Layout Stability Follow-up

This follow-up tightens the native macOS workbench shell after manual review
found that page titles and owner-page controls could move out of view during
vertical scrolling or window resizing.

## Scope

- Keep owner-page mode controls for Connect, Move, and Verify/Repair outside
  the page body scroll view so the operator can switch sub-surfaces without
  losing the section context.
- Keep detail-page content inside the body scroll view so long evidence panels
  remain scrollable without dragging the owner toolbar away.
- Let sidebar navigation scroll independently while preserving a compact
  workstation footer; at constrained heights the footer drops the secondary
  safety panel before it pushes navigation or status out of view.
- Record the fixed-owner-mode policy on `AppSection` rather than embedding that
  decision only inside the view tree.
- Use one tighter main-content top inset for all page hosts and fixed owner
  toolbars, while keeping the larger bottom inset for long-page breathing room.
- Pin shared detail-page headers with SwiftUI native pinned section headers so
  Prepare, Task Dispatch, Settings, and other `DetailPageHost` pages keep their
  page title visible while the body scrolls.
- Do not emulate fixed detail headers by measuring scroll position and applying
  a visual offset. That leaves the header in the scroll view's layout flow and
  can still let the title disappear or clip during real scrolling.
- Keep fixed owner-mode strips as compact mode controls only. Connect, Move,
  and Verify/Repair already have child detail pages that own the page title, so
  the owner strip must not reintroduce a second title banner.
- Let pinned detail headers collapse from the expanded banner into a compact
  integrated top bar after scrolling. The scroll marker only changes header
  presentation; it does not position or offset the pinned header.
- Keep Devices and Transfer header accessories to a single row so collapsed
  headers are not re-expanded by page-specific status/action stacks.
- Sweep direct page/card label duplicates: Sync's primary card no longer
  repeats the page title, and Prepare's role field label no longer repeats the
  role card title.

## Non-goals

- This does not redesign the information architecture, add new migration
  behavior, or change CLI command inputs.
- This does not claim final two-machine installed-app acceptance, signed
  distribution readiness, notarization, stapling, Gatekeeper proof, or Local
  Network/firewall prompt evidence.

## Validation

- `swift test --package-path macos --filter 'WorkbenchNavigationTests|WorkbenchChromeTests|AppStoreTests'`:
  pass; 136 tests, 0 failures.
- `swift test --package-path macos --filter 'WorkbenchChromeTests|WorkbenchNavigationTests|UIPreferencesTests'`:
  pass; 31 tests, 0 failures.
- `swift test --package-path macos --filter 'WorkbenchChromeTests|WorkbenchNavigationTests|UIPreferencesTests'`:
  pass; 33 tests, 0 failures after adding compact-header and owner-title
  duplication guards.
- `plutil -lint macos/SuperMoverApp/Resources/en.lproj/Localizable.strings macos/SuperMoverApp/Resources/zh-Hans.lproj/Localizable.strings`:
  pass after the Sync card title was made distinct from the page title.
- `WorkbenchNavigationTests/testOwnerSectionsDeclareFixedModeStripPolicy` pins
  the owner-section policy for fixed mode strips.
- `WorkbenchChromeTests/testFixedOwnerModeStripLeavesDedicatedBodyGap` pins the
  body gap used when the fixed owner toolbar is present.
- `WorkbenchChromeTests/testDetailPageHeaderUsesNativePinnedSectionHeader` pins
  `DetailPageHost` to native pinned section headers and rejects the old
  `GeometryReader` plus offset shim.
- `WorkbenchChromeTests/testDetailPageHeaderCollapseMetricsUseHysteresis` pins
  the compact-header threshold hysteresis.
- `WorkbenchNavigationTests/testOwnerModeStripDoesNotDuplicateOwnerPageTitles`
  prevents owner mode controls from reintroducing a title/subtitle header.
