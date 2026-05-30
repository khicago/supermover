# T-011 App Chrome, DRY, And Localization Closeout

## Scope

This pass closes the recent page-classification, shared-chrome, and mixed
language defects in the native macOS workbench.

Checked surfaces:

1. shared page chrome and owner-mode strips
2. Prepare, Connect, Move, Verify/Repair, Evidence, Task Dispatch, and Settings
   visible page chrome
3. auxiliary panels embedded inside owner pages
4. global UI preference ownership and localized resource coverage for app chrome
5. repository operating rules for root-cause sweeps, DRY, page contracts, and
   localization boundaries

## Root Cause

The app had already introduced reusable page shells, but not every visible
surface consumed them equally. Some owner pages used `DetailPageHost` and
shared localized section models, while embedded panels and secondary owner
surfaces kept direct SwiftUI strings and direct action titles. That created
three defects from one boundary mistake:

- UI fixes could land on one page while sibling pages drifted.
- Chinese UI could still show English buttons or panel titles.
- Future agents had no repository-level rule forcing root-cause analysis,
  sibling sweeps, DRY ownership, and localization coverage before closeout.
- The interface-language control was made global but then placed beside the
  sidebar brand title. That mixed app identity with settings chrome and caused
  the title to wrap in narrow sidebar widths.

The fix is to make app chrome ownership explicit: shared page chrome,
navigation labels, owner-mode strips, task display names, and auxiliary panel
chrome must flow through shared helpers or `AppChromeLocalization`; raw
evidence, CLI output, paths, JSON/protocol fields, profile IDs, and
operator-entered text remain unlocalized.

## Implementation

- `AGENTS.md` now requires root-cause/sibling-sweep closeout and adds explicit
  DRY, page-contract, and localization discipline.
- Owner-mode strips for Connect, Move, and Verify/Repair now use one generic
  `ownerModeStrip` implementation instead of cloned per-page functions.
- Sidebar, page headings, task category/display/summary strings, supervised
  process titles, major page sections, and visible action labels route through
  `AppChromeLocalization` or localized section helpers.
- Auxiliary panels now accept an injected localization owner:
  `ProfileNetworkPanel`, `PairingReceiptPanel`, `AcceptanceBundlePanel`, and
  `AcceptanceOperatorEvidencePanel`.
- Evidence Vault identity is unified with sidebar identity, avoiding a parallel
  page title.
- English and Simplified Chinese resource files now cover the newly localized
  page chrome, panel chrome, action labels, placeholders, and empty states.
- Follow-up coverage gap: resource coverage alone was not sufficient because
  some visible display-model values never called `AppChromeLocalization`.
  Control Room/homepage strings, transfer state labels, task input summaries,
  evidence artifact family/stage titles, evidence detail labels, JSON status
  labels, and count/empty-state messages now route through localization. Shared
  `GateState` and `EvidenceArtifactFamily` helpers replace scattered
  `.title.capitalized` and duplicate evidence-stage switches.
- Follow-up ownership gap: interface language switching remains one global
  preference binding, but the control now lives in Settings > Display
  Preferences instead of the sidebar brand header. The sidebar header is
  brand-only and the title is constrained to one line.

## Sweep Results

- Raw SwiftUI chrome scan now leaves only the product brand name as a direct
  visible string in app source. Preview-only action labels were also routed
  through localization to prevent copy-forward drift.
- The auxiliary panel scan finds no direct `Text("...")`, `TextField("...")`,
  `Toggle("...")`, `Picker("...")`, `ActionButton("...")`,
  `PrimaryActionButton("...")`, or `CompactActionButton("...")` chrome in the
  four audited panels.
- Resource scans found no missing `AppChromeLocalization.text("...")` keys and
  no duplicate resource keys.
- Follow-up source guards now explicitly reject the sibling bypasses that were
  found later: raw Control Room model chrome, `GateState.title.capitalized`,
  direct evidence `artifact.family.title`, and interpolated English evidence
  count messages.
- Follow-up sidebar sweep found no remaining product-code `globalLanguageMenu`,
  old `appChrome.language.title` owner key, or "language lives in global app
  chrome" copy. The only language preference binding in `ContentView` is the
  Settings display-preference picker.

## Validation

- `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|WorkbenchChromeTests'`
  passed with 31 tests.
- `plutil -lint macos/SuperMoverApp/Resources/en.lproj/Localizable.strings macos/SuperMoverApp/Resources/zh-Hans.lproj/Localizable.strings`
  passed.
- Missing-key and duplicate-key resource scans passed.
- Follow-up validation:
  `swift test --package-path macos --filter 'UIPreferencesTests|WorkbenchNavigationTests|WorkbenchChromeTests'`
  passed with 38 selected tests; `swift build --package-path macos --product
  SuperMoverApp` passed; `plutil -lint` passed; literal localization key scan
  found 0 missing keys and 0 duplicate resource keys for `en` and `zh-Hans`.

## Boundary

This pass does not claim that raw evidence content is translated. That is
intentional: CLI output, JSON/protocol fields, artifact names, paths, profile
IDs, receipt IDs, and operator-authored text are audit material and must remain
literal unless a later product decision creates a separate presentation layer.

This pass also does not replace real visual QA across every window size or the
real two-Mac installed-app validation. It closes the shared chrome/i18n/DRY
class of defects and adds guardrails so future UI work does not regress into
page-by-page patches.
