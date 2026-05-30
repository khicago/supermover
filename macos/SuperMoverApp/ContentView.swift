import Foundation
import SwiftUI

private enum ConnectSurface {
  case deviceState
  case pairing
}

private enum MoveSurface {
  case transfer
  case sync
}

private enum VerifyRepairSurface {
  case verification
  case driftReview
}

private extension SelectedProfilePathState {
  func localizedBadge(using localization: AppChromeLocalization) -> StatusBadgeItem {
    switch self {
    case .none:
      return .init(icon: "exclamationmark.circle", label: localization.text(.setupStatusNotSelected), tint: SMColor.amber)
    case .existingFile:
      return .init(icon: "checkmark.circle.fill", label: localization.text(.setupStatusExistingConfigFile), tint: SMColor.green)
    case .newDestination:
      return .init(icon: "doc.badge.plus", label: localization.text(.setupStatusNewConfigDestination), tint: SMColor.cyan)
    case .missingFile:
      return .init(icon: "exclamationmark.triangle", label: localization.text(.setupStatusConfigFileMissing), tint: SMColor.amber)
    case .directory:
      return .init(icon: "folder.fill", label: localization.text(.setupStatusFolderSelected), tint: SMColor.amber)
    }
  }
}

private enum NetworkEvidenceSurfaceState: Equatable {
  case notChecked
  case noEvidence
  case reviewRequired

  init(status: String?, hasReportTransfers: Bool) {
    let normalized = status?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() ?? ""
    switch normalized {
    case "":
      self = hasReportTransfers ? .reviewRequired : .notChecked
    case "no_evidence":
      self = .noEvidence
    case "review_required":
      self = .reviewRequired
    default:
      self = .reviewRequired
    }
  }

  var label: String {
    switch self {
    case .notChecked:
      return "Not checked"
    case .noEvidence:
      return "No evidence"
    case .reviewRequired:
      return "Review required"
    }
  }

  var pairingVisualState: PairingVisualState {
    switch self {
    case .notChecked, .noEvidence:
      return .neutral
    case .reviewRequired:
      return .warning
    }
  }

  var receiverPresence: DeviceCardModel.Presence {
    switch self {
    case .reviewRequired:
      return .online
    case .notChecked, .noEvidence:
      return .offline
    }
  }

  var receiverTrust: DeviceCardModel.Trust {
    switch self {
    case .reviewRequired:
      return .review
    case .notChecked, .noEvidence:
      return .untrusted
    }
  }
}

struct ContentView: View {
  @EnvironmentObject private var store: AppStore
  @EnvironmentObject private var uiPreferences: UIPreferencesStore
  @State private var selectedSection: AppSection = .controlRoom
  @State private var selectedConnectSurface: ConnectSurface = .deviceState
  @State private var selectedMoveSurface: MoveSurface = .transfer
  @State private var selectedVerifyRepairSurface: VerifyRepairSurface = .verification
  @State private var selectedDeviceFilterID = "all"
  @State private var deviceSearchText = ""
  @State private var selectedDeviceID = "source"
  @State private var selectedTaskCategory: SuperMoverTaskCategory = .local
  @State private var showProfileAdvanced = false

  private var appChromeLocalization: AppChromeLocalization {
    AppChromeLocalization(language: uiPreferences.language)
  }

  var body: some View {
    HStack(spacing: 0) {
      sidebar
      Divider()
        .overlay(SMColor.divider)
      mainContent
    }
    .background(SMColor.appBackground)
    .frame(
      minWidth: WorkbenchWindowMetrics.minimumContentSize.width,
      minHeight: WorkbenchWindowMetrics.minimumContentSize.height
    )
  }

  private var sidebar: some View {
    VStack(alignment: .leading, spacing: 20) {
      sidebarHeader

      ScrollView {
        sidebarNavigation
          .padding(.horizontal, 20)
          .padding(.vertical, 2)
      }
      .scrollIndicators(.hidden)

      sidebarFooter
        .padding(.horizontal, 20)
        .padding(.bottom, 24)
    }
    .frame(width: WorkbenchLayoutMetrics.sidebarWidth)
    .background(SMColor.sidebar)
  }

  private var sidebarNavigation: some View {
    VStack(alignment: .leading, spacing: 12) {
      SidebarRow(
        section: AppSection.homeSection,
        availability: AppSection.homeSection.availability(for: store.selectedRole),
        isSelected: selectedSection.ownerSection == AppSection.homeSection,
        localization: appChromeLocalization
      ) {
        selectSection(AppSection.homeSection)
      }

      ForEach(AppSection.localizedSidebarGroups(using: appChromeLocalization)) { group in
        VStack(alignment: .leading, spacing: 10) {
          Text(group.title.uppercased())
            .font(.system(size: 11, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)

          VStack(spacing: 6) {
            ForEach(group.sections) { section in
              SidebarRow(
                section: section,
                availability: section.availability(for: store.selectedRole),
                isSelected: selectedSection.ownerSection == section,
                localization: appChromeLocalization
              ) {
                selectSection(section)
              }
            }
          }
        }
        .panelSurface(.sidebarCard)
      }
    }
  }

  private func selectSection(_ section: AppSection) {
    selectedSection = section.ownerSection
    switch selectedSection {
    case .devices:
      selectedConnectSurface = .deviceState
    case .transfer:
      selectedMoveSurface = .transfer
    case .verification:
      selectedVerifyRepairSurface = .verification
    case .setup, .controlRoom, .pairing, .sync, .evidence, .driftReview, .taskDispatch, .settings:
      break
    }
  }

  private func showConnect(_ surface: ConnectSurface) {
    selectedConnectSurface = surface
    selectedSection = .devices
  }

  private func showMove(_ surface: MoveSurface) {
    selectedMoveSurface = surface
    selectedSection = .transfer
  }

  private func showVerifyRepair(_ surface: VerifyRepairSurface) {
    selectedVerifyRepairSurface = surface
    selectedSection = .verification
  }

  private func showEvidenceVault() {
    selectedSection = .evidence
  }

  private func showTaskDispatch() {
    selectedTaskCategory = store.selectedTask.taskCategory
    selectedSection = .taskDispatch
  }

  private func ownerModeButton(
    _ title: String,
    systemImage: String,
    isSelected: Bool,
    action: @escaping () -> Void
  ) -> some View {
    Button(action: action) {
      Label(title, systemImage: systemImage)
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(isSelected ? SMColor.primaryText : SMColor.secondaryText)
        .lineLimit(1)
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(isSelected ? SMColor.cardElevated : SMColor.input)
        .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
        .overlay(
          RoundedRectangle(cornerRadius: 8, style: .continuous)
            .stroke(isSelected ? SMColor.blue.opacity(0.5) : SMColor.hairline)
        )
    }
    .buttonStyle(.plain)
    .help(title)
  }

  private var sidebarHeader: some View {
    HStack(alignment: .top, spacing: 12) {
      Image(systemName: "shippingbox.fill")
        .font(.system(size: 15, weight: .semibold))
        .foregroundStyle(SMColor.blue)
        .frame(width: 30, height: 30)
        .background(SMColor.card.opacity(0.88))
        .clipShape(RoundedRectangle(cornerRadius: 9, style: .continuous))

      VStack(alignment: .leading, spacing: 3) {
        Text("SuperMover")
          .font(.system(size: 19, weight: .bold))
          .foregroundStyle(SMColor.primaryText)
        Text(appChromeLocalization.text(.sidebarTagline))
          .font(.system(size: 11, weight: .medium))
          .foregroundStyle(SMColor.secondaryText)
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .padding(.top, 34)
    .padding(.horizontal, 20)
  }

  private var sidebarStatusStrip: some View {
    VStack(alignment: .leading, spacing: 10) {
      Text(appChromeLocalization.text(.sidebarWorkstationTitle))
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      VStack(alignment: .leading, spacing: 8) {
        sidebarStatusRow(label: appChromeLocalization.text(.sidebarRoleLabel), value: store.selectedRole.localizedTitle(using: appChromeLocalization), tint: SMColor.blue)
        sidebarStatusRow(label: appChromeLocalization.text(.sidebarConfigLabel), value: profileStatus, tint: profilePathIsSet ? SMColor.green : SMColor.amber)
        sidebarStatusRow(label: appChromeLocalization.text(.sidebarSurfaceLabel), value: selectedSection.localizedTitle(using: appChromeLocalization), tint: selectedSection.availability(for: store.selectedRole).tint)
      }
    }
    .panelSurface(.statusStrip, padding: EdgeInsets(top: 12, leading: 12, bottom: 12, trailing: 12))
  }

  private var safetyPostureStrip: some View {
    VStack(alignment: .leading, spacing: 10) {
      HStack(spacing: 8) {
        StatusDot(color: SMColor.green)
        Text(appChromeLocalization.text(.safetyPostureTitle))
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
      }
      VStack(alignment: .leading, spacing: 8) {
        sidebarSafetyRow(appChromeLocalization.text(.safetyConfigSSOT), tint: SMColor.green)
        sidebarSafetyRow(appChromeLocalization.text(.safetyExplicitTargetMutations), tint: SMColor.blue)
        sidebarSafetyRow(appChromeLocalization.text(.safetyDurableEvidence), tint: SMColor.cyan)
      }
    }
    .panelSurface(.statusStrip, padding: EdgeInsets(top: 12, leading: 12, bottom: 12, trailing: 12))
  }

  private var sidebarFooter: some View {
    ViewThatFits(in: .vertical) {
      VStack(alignment: .leading, spacing: 14) {
        sidebarStatusStrip
        safetyPostureStrip
      }

      sidebarStatusStrip
    }
  }

  private var mainContent: some View {
    VStack(alignment: .leading, spacing: 0) {
      if hasFixedOwnerModeStrip {
        fixedOwnerModeStrip
          .padding(.horizontal, WorkbenchLayoutMetrics.mainContentHorizontalPadding)
          .padding(.top, WorkbenchLayoutMetrics.mainContentVerticalPadding)
          .padding(.bottom, WorkbenchLayoutMetrics.fixedOwnerModeStripBottomPadding)
          .background(SMColor.appBackground)
          .zIndex(2)
      }

      ScrollView {
        VStack(alignment: .leading, spacing: selectedSection == .controlRoom ? 18 : 16) {
          sectionContent
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, WorkbenchLayoutMetrics.mainContentHorizontalPadding)
        .padding(.top, scrollContentTopPadding)
        .padding(.bottom, WorkbenchLayoutMetrics.mainContentVerticalPadding)
      }
      .coordinateSpace(name: WorkbenchLayoutMetrics.mainContentScrollSpace)
    }
    .background(SMColor.appBackground)
  }

  private var hasFixedOwnerModeStrip: Bool {
    selectedSection.showsFixedOwnerModeStrip
  }

  private var scrollContentTopPadding: CGFloat {
    hasFixedOwnerModeStrip
      ? WorkbenchLayoutMetrics.fixedOwnerModeStripBodyGap
      : WorkbenchLayoutMetrics.mainContentVerticalPadding
  }

  @ViewBuilder
  private var fixedOwnerModeStrip: some View {
    switch selectedSection {
    case .devices, .pairing:
      connectModeStrip
    case .transfer, .sync:
      moveModeStrip
    case .verification, .driftReview:
      verifyRepairModeStrip
    case .controlRoom, .setup, .evidence, .taskDispatch, .settings:
      EmptyView()
    }
  }

  private var connectModeStrip: some View {
    WorkbenchToolbarStrip {
      WorkbenchResponsiveBar(alignment: .center, spacing: 12) {
        PanelHeader(
          title: "Connect",
          subtitle: "Device state, pairing, receiver readiness.",
          titleSize: 17,
          subtitleSize: 11,
          spacing: 4
        )
      } trailing: {
        HStack(spacing: 10) {
          ownerModeButton(
            "Device State",
            systemImage: "desktopcomputer",
            isSelected: selectedConnectSurface == .deviceState
          ) {
            selectedConnectSurface = .deviceState
          }
          ownerModeButton(
            "Pairing",
            systemImage: "link",
            isSelected: selectedConnectSurface == .pairing
          ) {
            selectedConnectSurface = .pairing
          }
        }
      }
    }
  }

  private var moveModeStrip: some View {
    WorkbenchToolbarStrip {
      WorkbenchResponsiveBar(alignment: .center, spacing: 12) {
        PanelHeader(
          title: "Move",
          subtitle: "Transfer and sync modes.",
          titleSize: 17,
          subtitleSize: 11,
          spacing: 4
        )
      } trailing: {
        HStack(spacing: 10) {
          ownerModeButton(
            "Transfer",
            systemImage: "arrow.right.arrow.left.square",
            isSelected: selectedMoveSurface == .transfer
          ) {
            selectedMoveSurface = .transfer
          }
          ownerModeButton(
            "Sync",
            systemImage: "arrow.triangle.2.circlepath",
            isSelected: selectedMoveSurface == .sync
          ) {
            selectedMoveSurface = .sync
          }
        }
      }
    }
  }

  private var verifyRepairModeStrip: some View {
    WorkbenchToolbarStrip {
      WorkbenchResponsiveBar(alignment: .center, spacing: 12) {
        PanelHeader(
          title: "Verify & Repair",
          subtitle: "Verify, drift, reconcile, prune.",
          titleSize: 17,
          subtitleSize: 11,
          spacing: 4
        )
      } trailing: {
        HStack(spacing: 10) {
          ownerModeButton(
            "Verify",
            systemImage: "checkmark.seal",
            isSelected: selectedVerifyRepairSurface == .verification
          ) {
            selectedVerifyRepairSurface = .verification
          }
          ownerModeButton(
            "Drift Review",
            systemImage: "exclamationmark.arrow.triangle.2.circlepath",
            isSelected: selectedVerifyRepairSurface == .driftReview
          ) {
            selectedVerifyRepairSurface = .driftReview
          }
        }
      }
    }
  }

  @ViewBuilder
  private var connectView: some View {
    switch selectedConnectSurface {
    case .deviceState:
      devicesView
    case .pairing:
      pairingView
    }
  }

  @ViewBuilder
  private var moveView: some View {
    switch selectedMoveSurface {
    case .transfer:
      transferView
    case .sync:
      syncView
    }
  }

  @ViewBuilder
  private var verifyRepairView: some View {
    switch selectedVerifyRepairSurface {
    case .verification:
      verificationView
    case .driftReview:
      driftReviewView
    }
  }

  @ViewBuilder
  private var sectionContent: some View {
    switch selectedSection {
    case .controlRoom:
      controlRoom
    case .setup:
      setupView
    case .devices, .pairing:
      connectView
    case .transfer, .sync:
      moveView
    case .verification, .driftReview:
      verifyRepairView
    case .evidence:
      evidenceView
    case .taskDispatch:
      taskDispatchView
    case .settings:
      settingsView
    }
  }

  private var setupView: some View {
    DetailPageHost(
      header: .init(
        title: appChromeLocalization.text(.setupHeaderTitle),
        subtitle: appChromeLocalization.text(.setupHeaderSubtitle),
        prominence: .hero
      ),
      headerAccessoryPlacement: .top,
      headerAccessory: {
        detailPageAccessoryBar(showPairing: false, showNetwork: false)
      },
      primary: {
        let guide = store.localizedSetupGuide(using: appChromeLocalization)
        VStack(alignment: .leading, spacing: 18) {
          ScreenCard(
            title: appChromeLocalization.text(.setupRoleCardTitle),
            subtitle: appChromeLocalization.text(.setupRoleCardSubtitle)
          ) {
            setupRoleSelector
          }

          ScreenCard(
            title: appChromeLocalization.text(.setupConfigCardTitle),
            subtitle: appChromeLocalization.text(.setupConfigCardSubtitle)
          ) {
            setupGuideStepHeader(guide.steps[0])
            profilePicker
          }

          ScreenCard(
            title: appChromeLocalization.text(.setupRootInputsCardTitle),
            subtitle: appChromeLocalization.text(.setupRootInputsCardSubtitle)
          ) {
            setupGuideStepHeader(guide.steps[1])
            setupRootFields
            setupConfigCreationAction(for: guide.steps[0])
          }

          ScreenCard(
            title: appChromeLocalization.text(.setupChecksCardTitle),
            subtitle: appChromeLocalization.text(.setupChecksCardSubtitle)
          ) {
            setupGuideStepHeader(guide.steps[2])
            setupReadiness
            commandPreview(for: setupPreviewTask)
            setupValidationActions(for: guide.steps[2])
          }
        }
      }
    )
  }

  private var setupRoleSelector: some View {
    VStack(alignment: .leading, spacing: 12) {
      Text(appChromeLocalization.text(.setupRoleFieldTitle))
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)

      HStack(spacing: 10) {
        ForEach(WorkbenchRole.allCases) { role in
          compactRoleButton(role)
        }
      }

      VStack(alignment: .leading, spacing: 6) {
        Text(store.selectedRole.localizedSummary(using: appChromeLocalization))
          .font(.system(size: 12))
          .foregroundStyle(SMColor.primaryText)
          .fixedSize(horizontal: false, vertical: true)
        Text(store.selectedRole.localizedAllowedSetup(using: appChromeLocalization))
          .font(.system(size: 11, design: .monospaced))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
      .padding(12)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(SMColor.input)
      .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
      .overlay(
        RoundedRectangle(cornerRadius: 12, style: .continuous)
          .stroke(SMColor.hairline, lineWidth: 1)
      )
    }
  }

  @ViewBuilder
  private var setupRootFields: some View {
    switch store.selectedRole {
    case .source:
      pathField(
        appChromeLocalization.text(.setupSourceRootFieldTitle),
        text: $store.sourceRootPath,
        placeholder: appChromeLocalization.text(.setupSourceRootPlaceholder),
        readiness: sourceRootReadiness,
        browseTitle: appChromeLocalization.text(.setupActionBrowseSourceRoot)
      ) {
        store.browseSourceRoot()
      }
    case .target:
      pathField(
        appChromeLocalization.text(.setupTargetRootFieldTitle),
        text: $store.targetRootPath,
        placeholder: appChromeLocalization.text(.setupTargetRootPlaceholder),
        readiness: targetRootReadiness,
        browseTitle: appChromeLocalization.text(.setupActionBrowseTargetRoot)
      ) {
        store.browseTargetRoot()
      }
    case .observer:
      Text(
        appChromeLocalization.text(.setupObserverRootInputsNotice)
      )
      .font(.system(size: 12))
      .foregroundStyle(SMColor.secondaryText)
      .fixedSize(horizontal: false, vertical: true)
    }
  }

  private func setupGuideStepHeader(_ step: SetupGuide.Step) -> some View {
    HStack(alignment: .top, spacing: 12) {
      Text("\(step.index)")
        .font(.system(size: 12, weight: .bold))
        .foregroundStyle(step.state.color)
        .frame(width: 26, height: 26)
        .background(step.state.color.opacity(0.10))
        .clipShape(Circle())
      VStack(alignment: .leading, spacing: 5) {
        HStack(spacing: 8) {
          Text(step.title)
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(SMColor.primaryText)
          StatusBadge(
            item: .init(icon: "circle.fill", label: step.statusLabel, tint: step.state.color),
            prominence: .plain,
            compact: true
          )
        }
        Text(step.detail)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  @ViewBuilder
  private func setupConfigCreationAction(for step: SetupGuide.Step) -> some View {
    if step.primaryTask == .profileInit, let title = step.primaryActionTitle {
      let canCreate = sourceRootReadiness == "readable"
      PrimaryActionButton(title, systemImage: "doc.badge.plus", isEnabled: canCreate) {
        if store.selectedRole == .source && (profileSelectionState == .none || profileSelectionState == .missingFile) {
          store.useRecommendedProfileDestination()
        }
        run(.profileInit)
      }
    }
  }

  @ViewBuilder
  private func setupValidationActions(for step: SetupGuide.Step) -> some View {
    WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
      if let primaryTitle = step.primaryActionTitle {
        if let task = step.primaryTask {
          ActionButton(primaryTitle, systemImage: setupGuideIcon(for: task)) {
            run(task)
          }
        }
      }
      if let secondaryTitle = step.secondaryActionTitle {
        ActionButton(secondaryTitle, systemImage: "waveform.path.ecg") {
          run(.status)
        }
      }
      if step.id == "validate", store.selectedRole != .observer, profileSelectionState == .existingFile {
        ActionButton(appChromeLocalization.text(.setupActionUpdateExistingConfigTarget), systemImage: "scope") {
          run(.profileSetTarget)
        }
      }
    }
  }

  private func setupGuideIcon(for task: SuperMoverTaskKind) -> String {
    switch task {
    case .profileInit:
      return "doc.badge.plus"
    case .profileSetTarget:
      return "scope"
    case .status:
      return "waveform.path.ecg"
    case .lintProfile:
      return "checklist"
    default:
      return "play.fill"
    }
  }

  private var controlRoom: some View {
    ControlRoomView(
      migrationSurface: { migrationSurface },
      safetyGatesPanel: { safetyGatesPanel },
      recentRunsPanel: { controlRoomRecentRunsPanel },
      metricStrip: { controlRoomMetricStrip },
      stageRail: { stageRail(activeIndex: activeStageIndex) },
      runConsolePanel: { controlRoomRunConsolePanel },
      actionStrip: { controlRoomActionStrip },
      cliPanel: { controlRoomCLIPanel }
    )
  }

  private var migrationSurface: some View {
    VStack(alignment: .leading, spacing: 18) {
      WorkbenchHeaderBar(
        pageHeader: .init(
          eyebrow: "Control Room",
          title: "Migration workbench",
          subtitle: "Foreground transfer, durable evidence, and operator checkpoints presented as one native desk.",
          prominence: .hero,
          badge: .init(
            title: store.activeRun == nil ? "Ready" : "Live",
            tint: store.activeRun == nil ? SMColor.green : SMColor.blue
          )
        ),
        accessoryPlacement: .top
      ) {
        VStack(alignment: .trailing, spacing: 10) {
          HStack(spacing: 8) {
            ControlRoomStatusItem(
              model: .init(
                id: "focus",
                icon: "circle.fill",
                label: "Focus",
                value: controlRoomFocus.label,
                tint: controlRoomFocus.tint
              ))
            ControlRoomStatusItem(
              model: .init(
                id: "evidence",
                icon: "doc.badge.gearshape",
                label: "Evidence",
                value: hasLoadedEvidence ? "Loaded" : "Pending",
                tint: hasLoadedEvidence ? SMColor.green : SMColor.amber
              ))
          }
          Text(store.note)
            .font(.system(size: 11))
            .foregroundStyle(SMColor.secondaryText)
            .lineLimit(2)
            .multilineTextAlignment(.trailing)
        }
      }

      HStack(alignment: .top, spacing: 18) {
        endpointStatusPane(
          roleLabel: "source",
          title: sourceTitle,
          subtitle: sourceSubtitle,
          detail: sourceEndpointDetail,
          systemImage: "laptopcomputer",
          tint: SMColor.cyan,
          progress: verificationProgress
        )

        VStack(alignment: .leading, spacing: 14) {
          controlRoomTransferArtwork

          VStack(alignment: .leading, spacing: 6) {
            Text(controlRoomFocus.value)
              .font(.system(size: 46, weight: .bold))
              .foregroundStyle(SMColor.primaryText)
              .monospacedDigit()
              .lineLimit(1)
              .minimumScaleFactor(0.72)
            Text(controlRoomFocus.label)
              .font(.system(size: 11, weight: .semibold))
              .foregroundStyle(controlRoomFocus.tint)
            Text(controlRoomFocus.detail)
              .font(.system(size: 12))
              .foregroundStyle(SMColor.secondaryText)
              .lineLimit(3)
              .fixedSize(horizontal: false, vertical: true)
          }

          ProgressRail(progress: controlRoomFocus.progress, tint: controlRoomFocus.tint)
            .frame(height: 7)

          HStack(spacing: 18) {
            ControlRoomInlineStat(
              title: "Files", value: filesVerifiedValue, tint: SMColor.primaryText)
            ControlRoomInlineStat(
              title: "Warnings",
              value: warningMetricValue,
              tint: countMetricTint(warningCountEvidence)
            )
            ControlRoomInlineStat(
              title: "Transfers", value: networkTransferValue, tint: SMColor.cyan)
          }
        }
        .frame(width: 260, alignment: .leading)
        .padding(.top, 4)

        endpointStatusPane(
          roleLabel: "target",
          title: targetTitle,
          subtitle: targetSubtitle,
          detail: targetEndpointDetail,
          systemImage: "externaldrive.fill",
          tint: SMColor.green,
          progress: verificationProgress
        )
      }

      HStack(alignment: .top, spacing: 12) {
        ForEach(controlRoomContextTiles) { tile in
          ControlRoomContextTile(model: tile)
        }
      }
    }
    .padding(24)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(
      RoundedRectangle(cornerRadius: 18, style: .continuous)
        .fill(
          LinearGradient(
            colors: [SMColor.card, SMColor.cardElevated, SMColor.input.opacity(0.88)],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
          )
        )
        .overlay(alignment: .topTrailing) {
          controlRoomSurfaceWatermark
            .padding(.top, 12)
            .padding(.trailing, 18)
        }
    )
    .clipShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
  }

  private var safetyGatesPanel: some View {
    VStack(alignment: .leading, spacing: 16) {
      HStack(alignment: .top, spacing: 12) {
        ZStack {
          RoundedRectangle(cornerRadius: 10, style: .continuous)
            .fill(SMColor.green.opacity(0.12))
          Image(systemName: "checkmark.shield.fill")
            .font(.system(size: 17, weight: .semibold))
            .foregroundStyle(SMColor.green)
        }
        .frame(width: 38, height: 38)

        VStack(alignment: .leading, spacing: 5) {
          Text("Safety Gates")
            .font(.system(size: 21, weight: .bold))
            .foregroundStyle(SMColor.primaryText)
          Text("Every target mutation stays explainable, reversible, and evidence-backed.")
            .font(.system(size: 12))
            .foregroundStyle(SMColor.secondaryText)
            .fixedSize(horizontal: false, vertical: true)
        }
      }

      VStack(spacing: 0) {
        controlRoomGateListRow("Config file selected", state: profilePathIsSet ? .pass : .pending)
        Divider()
        controlRoomGateListRow(
          "Dry-run passed",
          state: hasSuccessfulRecent(.dryRun) || hasSuccessfulRecent(.networkDryRun)
            ? .pass : .pending)
        Divider()
        controlRoomGateListRow("Target preflight clean", state: targetPreflightGateState)
        Divider()
        controlRoomGateListRow("Warnings durable", state: warningGateState)
        Divider()
        controlRoomGateListRow("Integrity check current", state: verificationRunwayState)
        Divider()
        controlRoomGateListRow("Reconcile manual", state: .neutral)
      }
      .padding(.vertical, 4)
      .background(SMColor.input.opacity(0.5))
      .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))

      VStack(alignment: .leading, spacing: 6) {
        Text("Policy")
          .font(.system(size: 11, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
        Text(
          "Completion still depends on durable target evidence. The app console does not replace receipts, reports, or verify output."
        )
        .font(.system(size: 11))
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
      }

      HStack(spacing: 10) {
        ActionButton("Connect", systemImage: "point.3.connected.trianglepath.dotted") {
          showConnect(.deviceState)
        }
        ActionButton("Evidence", systemImage: "doc.text.magnifyingglass") {
          showEvidenceVault()
        }
      }
    }
    .padding(22)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(SMColor.card)
    .clipShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
    .overlay(
      RoundedRectangle(cornerRadius: 18, style: .continuous)
        .stroke(SMColor.hairline, lineWidth: 1)
    )
  }

  private var controlRoomMetricStrip: some View {
    ControlRoomMetricStripView(model: controlRoomMetricStripModel)
  }

  private var controlRoomActionStrip: some View {
    VStack(alignment: .leading, spacing: 14) {
      PanelHeader(
        title: "Operator Controls",
        subtitle: "Focused commands only. Existing CLI wiring and role gates stay in force.",
        titleSize: 17,
        subtitleSize: 11,
        spacing: 4
      )

      PrimaryActionButton("Stop Focused Slot", systemImage: "stop.fill") {
        store.stopActiveTask()
      }
      ActionButton("Run Verification", systemImage: "checkmark.seal") {
        run(.verify)
      }
      ActionButton("Task Dispatch", systemImage: "terminal.fill") {
        showTaskDispatch()
      }
      ActionButton("Open Evidence", systemImage: "doc.text.magnifyingglass") {
        showEvidenceVault()
        run(.status)
      }
      if store.selectedRole.allows(task: .dashboard) {
        ActionButton("Dashboard", systemImage: "safari") {
          run(.dashboard)
        }
      } else {
        ActionButton("Open Connect", systemImage: "point.3.connected.trianglepath.dotted") {
          showConnect(.deviceState)
        }
      }

      availabilityNotice(
        title: "Operator note",
        detail:
          "Use Verify and Evidence when you need durable proof. This control strip does not imply completion by itself.",
        state: .neutral
      )
    }
    .padding(14)
    .background(SMColor.card)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(
      RoundedRectangle(cornerRadius: 14, style: .continuous)
        .stroke(SMColor.hairline, lineWidth: 1)
    )
  }

  private var controlRoomCLIPanel: some View {
    WorkbenchPanel(
      title: "Task Dispatch",
      subtitle: "Open the dedicated command dispatcher for the full wired CLI surface."
    ) {
      VStack(alignment: .leading, spacing: 12) {
        let taskAllowed = store.selectedRole.allows(task: store.selectedTask)
        HStack(spacing: 8) {
          taskDispatchBadge(label: "selected", value: store.selectedTask.displayTitle, tint: taskAllowed ? SMColor.blue : SMColor.amber)
          taskDispatchBadge(label: "category", value: store.selectedTask.category, tint: SMColor.secondaryText)
        }
        Text(store.selectedTask.summary)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .lineLimit(3)
          .fixedSize(horizontal: false, vertical: true)

        if !taskAllowed {
          availabilityNotice(
            title: "Role-gated task",
            detail:
              "\(store.selectedRole.localizedTitle(using: appChromeLocalization)) role cannot run \(store.selectedTask.displayTitle). Open Task Dispatch to pick a role-allowed task.",
            state: .blocked
          )
        }
        WorkbenchWrappingRow(spacing: 10, rowSpacing: 10) {
          PrimaryActionButton("Open Task Dispatch", systemImage: "terminal.fill") {
            showTaskDispatch()
          }
          if !taskAllowed {
            EvidenceChip(label: "task", value: "role-gated", tint: SMColor.amber)
          }
        }
      }
    }
  }

  private var controlRoomRunConsolePanel: some View {
    WorkbenchPanel(title: activeRunTitle, subtitle: store.note) {
      supervisedProcessGrid
      if let run = store.activeRun {
        let runState = store.supervisionStateLabel(for: run.slot)
        HStack {
          EvidenceChip(
            label: "slot", value: run.slot.title,
            tint: run.slot.isLongRunning ? SMColor.cyan : SMColor.blue)
          EvidenceChip(label: "task", value: run.kind.displayTitle, tint: SMColor.blue)
          EvidenceChip(label: "state", value: runState, tint: tint(for: runState))
          if let pid = run.processIdentifier {
            EvidenceChip(label: "pid", value: "\(pid)", tint: SMColor.secondaryText)
          }
          Spacer()
          Text(run.launchedAt, style: .time)
            .font(.system(size: 12, weight: .medium))
            .foregroundStyle(SMColor.secondaryText)
        }
        HStack(alignment: .top, spacing: 12) {
          outputPane(title: "stdout", text: run.stdout)
          outputPane(title: "stderr", text: run.stderr)
        }
      } else {
        Text("No active run. Choose a migration config file, then run a task from this workstation.")
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
      }
      processLifecycleLog
    }
  }

  private var controlRoomRecentRunsPanel: some View {
    WorkbenchPanel(
      title: "Recent Activity",
      subtitle:
        "Latest local app-launched CLI runs for this workstation. Durable target receipts remain under Evidence."
    ) {
      if store.recentRuns.isEmpty {
        Text("No recent runs.")
          .font(.system(size: 13))
          .foregroundStyle(SMColor.secondaryText)
      } else {
        let runs = Array(store.recentRuns.prefix(6))
        VStack(spacing: 0) {
          ForEach(Array(runs.enumerated()), id: \.element.id) { index, run in
            ControlRoomRecentRunRow(model: controlRoomRecentRunModel(for: run))
            if index < runs.count - 1 {
              Divider()
            }
          }
        }
      }
    }
  }

  private var taskDispatchView: some View {
    DetailPageHost(
      header: .init(
        title: "Task Dispatch",
        subtitle: "Run wired CLI tasks through explicit migration config inputs and role gates."
      ),
      asideWidth: 328,
      asideLeading: true,
      primary: {
        taskDispatchDetailPanel
      },
      aside: {
        taskDispatchTaskList
      }
    )
  }

  private var taskDispatchTaskList: some View {
    ScreenCard(
      title: "Tasks",
      subtitle: "Choose a wired command surface. Selection does not bypass run gates."
    ) {
      VStack(alignment: .leading, spacing: 12) {
        taskCategorySelector

        ScrollView {
          LazyVStack(alignment: .leading, spacing: 8) {
            ForEach(SuperMoverTaskKind.tasks(in: selectedTaskCategory)) { task in
              taskDispatchRow(task)
            }
          }
          .padding(.vertical, 2)
        }
        .frame(minHeight: 420, maxHeight: 620)
      }
    }
  }

  private var taskCategorySelector: some View {
    LazyVGrid(columns: [GridItem(.adaptive(minimum: 92), spacing: 8)], spacing: 8) {
      ForEach(SuperMoverTaskCategory.allCases) { category in
        Button {
          selectedTaskCategory = category
        } label: {
          Text(category.title)
            .font(.system(size: 11, weight: .semibold))
            .lineLimit(1)
            .frame(maxWidth: .infinity)
            .padding(.horizontal, 9)
            .padding(.vertical, 7)
            .background(selectedTaskCategory == category ? SMColor.cardElevated : SMColor.input)
            .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
            .overlay(
              RoundedRectangle(cornerRadius: 8, style: .continuous)
                .stroke(selectedTaskCategory == category ? SMColor.blue.opacity(0.58) : SMColor.hairline)
            )
        }
        .buttonStyle(.plain)
        .foregroundStyle(selectedTaskCategory == category ? SMColor.primaryText : SMColor.secondaryText)
      }
    }
  }

  private func taskDispatchRow(_ task: SuperMoverTaskKind) -> some View {
    let isSelected = store.selectedTask == task
    let taskAllowed = store.selectedRole.allows(task: task)
    return Button {
      store.selectedTask = task
    } label: {
      HStack(alignment: .top, spacing: 10) {
        Image(systemName: taskDispatchIcon(for: task))
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(taskAllowed ? SMColor.blue : SMColor.amber)
          .frame(width: 22, height: 22)
          .background((taskAllowed ? SMColor.blue : SMColor.amber).opacity(0.1))
          .clipShape(RoundedRectangle(cornerRadius: 7, style: .continuous))
        VStack(alignment: .leading, spacing: 4) {
          HStack(spacing: 6) {
            Text(task.displayTitle)
              .font(.system(size: 13, weight: .semibold))
              .foregroundStyle(SMColor.primaryText)
              .lineLimit(1)
            if !taskAllowed {
              Text("role-gated")
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(SMColor.amber)
            } else if task.longRunning {
              Text("foreground")
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(SMColor.cyan)
            }
          }
          Text(task.summary)
            .font(.system(size: 11))
            .foregroundStyle(SMColor.secondaryText)
            .lineLimit(2)
            .fixedSize(horizontal: false, vertical: true)
        }
        Spacer(minLength: 0)
      }
      .padding(11)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(isSelected ? SMColor.cardElevated : SMColor.input.opacity(0.74))
      .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
      .overlay(
        RoundedRectangle(cornerRadius: 10, style: .continuous)
          .stroke(isSelected ? SMColor.blue.opacity(0.62) : SMColor.hairline)
      )
    }
    .buttonStyle(.plain)
  }

  private var taskDispatchDetailPanel: some View {
    ScreenCard(
      title: store.selectedTask.displayTitle,
      subtitle: store.selectedTask.summary
    ) {
      VStack(alignment: .leading, spacing: 16) {
        taskDispatchStatusStrip
        commandPreview(for: store.selectedTask)
        taskDispatchGatePanel
        taskDispatchRunControls
      }
    }
  }

  private var taskDispatchStatusStrip: some View {
    let taskAllowed = store.selectedRole.allows(task: store.selectedTask)
    let profileSatisfied = profileRequirementSatisfied(for: store.selectedTask)
    return WorkbenchWrappingRow(spacing: 10, rowSpacing: 10) {
      EvidenceChip(
        label: appChromeLocalization.text(.sidebarRoleLabel),
        value: store.selectedRole.localizedTitle(using: appChromeLocalization),
        tint: taskAllowed ? SMColor.green : SMColor.amber)
      EvidenceChip(label: "category", value: store.selectedTask.category, tint: SMColor.blue)
      EvidenceChip(
        label: "mode",
        value: store.selectedTask.longRunning ? "foreground" : "bounded",
        tint: store.selectedTask.longRunning ? SMColor.cyan : SMColor.secondaryText
      )
      EvidenceChip(
        label: "config",
        value: store.selectedTask.requiresProfile ? profileStatus : "not required",
        tint: store.selectedTask.requiresProfile ? (profileSatisfied ? SMColor.green : SMColor.amber) : SMColor.secondaryText
      )
    }
  }

  @ViewBuilder
  private var taskDispatchGatePanel: some View {
    let taskAllowed = store.selectedRole.allows(task: store.selectedTask)
    let runGate = store.taskRunGate()
    VStack(alignment: .leading, spacing: 10) {
      if !taskAllowed {
        availabilityNotice(
          title: "Role-gated task",
          detail:
            "\(store.selectedRole.localizedTitle(using: appChromeLocalization)) role cannot run \(store.selectedTask.displayTitle). Switch roles or choose a role-allowed evidence task.",
          state: .blocked
        )
      }
      if taskAllowed, !runGate.isRunnable {
        availabilityNotice(
          title: "Run gate blocked",
          detail: runGate.note ?? "Current inputs do not satisfy the task run gate.",
          state: .blocked
        )
      }
      if taskAllowed,
        let preview = store.selectedTaskAcceptanceLaunchPreview
      {
        availabilityNotice(
          title: preview.title,
          detail: preview.detail,
          state: gateState(for: preview.state)
        )
      }
      Text(taskDispatchInputSummary(for: store.selectedTask))
        .font(.system(size: 12))
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
    }
    .padding(12)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
  }

  private var taskDispatchRunControls: some View {
    let runGate = store.taskRunGate()
    return WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
      if store.selectedRole.allows(task: store.selectedTask) {
        PrimaryActionButton(
          store.selectedTask.longRunning ? "Start Supervised Process" : "Run Task",
          systemImage: "play.fill",
          isEnabled: runGate.isRunnable
        ) {
          store.runSelectedTask()
        }
        if store.selectedTask.longRunning {
          ActionButton("Stop", systemImage: "stop.fill") {
            store.stopProcess(in: store.selectedTask.supervisedSlot)
          }
        }
        if store.selectedTask == .dashboard {
          ActionButton("Open Dashboard URL", systemImage: "arrow.up.forward.app") {
            store.revealDashboardURL()
          }
        }
      } else {
        EvidenceChip(label: "task", value: "role-gated", tint: SMColor.amber)
      }
      ActionButton("Open Settings", systemImage: "slider.horizontal.3") {
        selectedSection = .settings
      }
      ActionButton("Open Home", systemImage: "house.fill") {
        selectedSection = .controlRoom
      }
    }
  }

  private var devicesView: some View {
    DevicesSectionView(
      model: devicesSectionModel,
      onRefresh: { run(.status) },
      onMoreActions: nil,
      onSelectFilter: { selectedDeviceFilterID = $0 },
      onSearchChange: { deviceSearchText = $0 },
      onSelectDevice: { selectedDeviceID = $0 },
      onInspectorAction: { showEvidenceVault() }
    )
  }

  private var pairingView: some View {
    PairingSectionView(
      model: pairingSectionModel,
      onRegenerateCode: store.selectedRole == .target ? {
        run(.serve)
      } : nil,
      onCancel: { selectedSection = .controlRoom },
      onBack: { showConnect(.deviceState) },
      onContinue: {
        switch store.selectedRole {
        case .source:
          run(.pair)
        case .target:
          run(.serve)
        case .observer:
          showEvidenceVault()
        }
      }
    )
  }

  private var devicesSectionModel: DevicesSectionModel {
    let allDevices: [DeviceCardModel] = [
      DeviceCardModel(
        id: "source",
        name: sourceTitle,
        detailLine: "Source • \(profilePathIsSet ? "config selected" : "config required")",
        kind: .laptop,
        presence: .online,
        trust: pairingEvidenceState == .pass ? .trusted : (pairingEvidenceState == .review ? .review : .untrusted),
        address: profilePathIsSet ? "config-backed source" : "local workstation",
        primaryMetric: .init(label: "Role", value: store.selectedRole == .source ? "Source" : "Observer"),
        secondaryMetric: .init(label: "Config", value: profileStatus),
        storage: profilePathIsSet ? .init(summary: "Config selected", progress: 0.72, tint: SMColor.cyan) : nil,
        lastSeen: "Current session",
        isSelected: false
      ),
      DeviceCardModel(
        id: "target",
        name: targetTitle,
        detailLine: "Target • \(targetSubtitle)",
        kind: .storage,
        presence: hasLoadedEvidence ? .online : .offline,
        trust: targetPreflightGateState == .pass ? .trusted : (targetPreflightGateState == .review ? .review : .untrusted),
        address: statusTargetRoot == "-" ? "target root unknown" : statusTargetRoot,
        primaryMetric: .init(label: "Root", value: statusTargetRoot == "-" ? "Unknown" : "Ready"),
        secondaryMetric: .init(label: "Integrity", value: integrityValue),
        storage: hasLoadedEvidence ? .init(summary: verifiedFilesMetricValue, progress: max(verificationProgress, 0.18), tint: SMColor.green) : nil,
        lastSeen: hasLoadedEvidence ? "Evidence loaded" : "Not loaded",
        isSelected: false
      ),
      DeviceCardModel(
        id: "receiver",
        name: "Receiver Surface",
        detailLine: "Serve / advertise / import",
        kind: .rack,
        presence: store.serveReadinessSnapshot == nil ? networkEvidenceState.receiverPresence : .online,
        trust: networkEvidenceState.receiverTrust,
        address: store.listenAddress.isEmpty ? "listen address not set" : store.listenAddress,
        primaryMetric: .init(label: "Pairing", value: pairingStatus),
        secondaryMetric: .init(label: "Receiver", value: networkStatus),
        storage: nil,
        lastSeen: store.serveReadinessSnapshot == nil ? "Not served" : "Serve ready",
        isSelected: false
      ),
    ]

    let filteredDevices = allDevices
      .filter { device in
        switch selectedDeviceFilterID {
        case "source":
          return device.id == "source"
        case "target":
          return device.id == "target"
        case "receiver":
          return device.id == "receiver"
        default:
          return true
        }
      }
      .filter { device in
        let query = deviceSearchText.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard !query.isEmpty else { return true }
        let haystack = [
          device.name,
          device.detailLine,
          device.address,
          device.primaryMetric.value,
          device.secondaryMetric.value,
          device.lastSeen,
        ]
        .joined(separator: " ")
        .lowercased()
        return haystack.contains(query)
      }
      .map { device in
        var updated = device
        updated.isSelected = device.id == selectedDeviceID
        return updated
      }

    let trustRows = [
      DevicesInspectorModel.PairRow(id: "trusted", label: "Trusted devices", value: pairingEvidenceState == .pass ? "2" : "0"),
      DevicesInspectorModel.PairRow(id: "review", label: "Needs review", value: warningGateState == .review ? "1" : "0"),
      DevicesInspectorModel.PairRow(id: "change", label: "Last trust change", value: latestPairSummary ?? "No confirmed pairing"),
    ]

    let networkRows = [
      DevicesInspectorModel.PairRow(id: "transport", label: "Transport", value: networkStatus),
      DevicesInspectorModel.PairRow(id: "profile", label: "Config", value: profileStatus),
    ]

    return DevicesSectionModel(
      title: "Device State",
      subtitle: "Inspect source, target, receiver, and trust posture under the Connect owner page.",
      lastUpdatedLabel: hasLoadedEvidence ? "Last updated: just now" : "Last updated: pending",
      headerStatus: DevicesHeaderStatus(
        label: hasLoadedEvidence ? "Available" : "Needs evidence",
        systemImage: hasLoadedEvidence ? "checkmark.circle.fill" : "clock",
        tint: hasLoadedEvidence ? SMColor.green : SMColor.amber
      ),
      searchPlaceholder: "Search devices",
      searchText: deviceSearchText,
      filters: [
        DevicesFilter(id: "all", title: "All", countLabel: "\(allDevices.count)", isSelected: selectedDeviceFilterID == "all"),
        DevicesFilter(id: "source", title: "Sources", countLabel: "1", isSelected: selectedDeviceFilterID == "source"),
        DevicesFilter(id: "target", title: "Targets", countLabel: "1", isSelected: selectedDeviceFilterID == "target"),
        DevicesFilter(id: "receiver", title: "Receivers", countLabel: "1", isSelected: selectedDeviceFilterID == "receiver"),
      ],
      devices: filteredDevices,
      inspector: DevicesInspectorModel(
        title: "Safety posture",
        summary: .init(
          title: "Migration config, target, and warning state are derived from the currently loaded app evidence.",
          value: hasLoadedEvidence ? "Loaded" : "Review",
          tint: hasLoadedEvidence ? SMColor.green : SMColor.amber,
          note: "This panel summarizes local evidence. It is not end-to-end trust proof."
        ),
        checks: [
          .init(id: "profile", title: "Config sealed", value: profilePathIsSet ? "Sealed" : "Missing", state: profilePathIsSet ? .pass : .warning),
          .init(id: "dry-run", title: "Dry-run passed", value: hasSuccessfulRecent(.dryRun) || hasSuccessfulRecent(.networkDryRun) ? "Passed" : "Pending", state: hasSuccessfulRecent(.dryRun) || hasSuccessfulRecent(.networkDryRun) ? .pass : .warning),
          .init(id: "preflight", title: "Target preflight clean", value: targetPreflightGateState.title.capitalized, state: targetPreflightGateState == .pass ? .pass : (targetPreflightGateState == .review ? .warning : .neutral)),
          .init(id: "warnings", title: "Warnings durable", value: warningGateState.title.capitalized, state: warningGateState == .pass ? .pass : (warningGateState == .review ? .warning : .neutral)),
          .init(id: "integrity", title: "Root comparison", value: verificationRunwayState.title.capitalized, state: verificationRunwayState == .pass ? .pass : (verificationRunwayState == .review ? .warning : .neutral)),
        ],
        overviewRows: trustRows,
        networkRows: networkRows,
        actionTitle: "Open Evidence"
      )
    )
  }

  private var pairingSectionModel: PairingSectionModel {
    let sourceState: PairingVisualState = pairingEvidenceState == .pass ? .success : (pairingEvidenceState == .review ? .warning : .neutral)
    let targetState = networkEvidenceState.pairingVisualState
    let sourceAddress = store.pairingTargetAddress.isEmpty ? "target address not set" : store.pairingTargetAddress
    let targetAddress = store.listenAddress.isEmpty ? "listen address not set" : store.listenAddress
    let pairingCode = store.serveReadinessSnapshot?.verification_code?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty == false
      ? store.serveReadinessSnapshot!.verification_code!
      : (store.pairingVerificationCode.isEmpty ? "not generated" : store.pairingVerificationCode)
    let inspectorEvents = [
      PairingInspectorEvent(
        id: "pairing",
        title: "Pairing status",
        detail: latestPairSummary,
        trailingLabel: pairingStatus,
        state: sourceState
      ),
      PairingInspectorEvent(
        id: "receiver",
        title: "Receiver surface",
        detail: store.serveReadinessSnapshot?.summaryLine,
        trailingLabel: networkStatus,
        state: targetState
      ),
      PairingInspectorEvent(
        id: "profile",
        title: "Config state",
        detail: sourceSubtitle,
        trailingLabel: profileStatus,
        state: profilePathIsSet ? .success : .warning
      ),
    ]

    return PairingSectionModel(
      subtitle: "Establish a trusted pairing between source and target. Pairing pins are required before transfer.",
      badge: PairingBadge(
        title: store.selectedRole == .observer ? "Read-only" : (store.activeRun == nil ? "Idle" : "Live"),
        state: store.selectedRole == .observer ? .neutral : (store.activeRun == nil ? .success : .info)
      ),
      lastUpdatedLabel: hasLoadedEvidence ? "Last updated: just now" : nil,
      steps: [
        PairingStep(id: "select", indexLabel: "1", title: "Select devices", state: profilePathIsSet ? .complete : .current, isCurrent: !profilePathIsSet),
        PairingStep(id: "trust", indexLabel: "2", title: "Trust and pins", state: pairingEvidenceState == .pass ? .complete : .current, isCurrent: profilePathIsSet && pairingEvidenceState != .pass),
        PairingStep(id: "summary", indexLabel: "3", title: "Summary", state: pairingEvidenceState == .pass ? .current : .upcoming, isCurrent: pairingEvidenceState == .pass),
      ],
      summaryLine: "Discovery is only a hint; explicit pins and config validation establish trust.",
      source: PairingEndpointSummary(
        name: sourceTitle,
        iconName: "laptopcomputer",
        state: sourceState,
        metadata: [
          .init(id: "source-profile", value: sourceSubtitle, emphasized: true),
          .init(id: "source-address", value: sourceAddress),
        ]
      ),
      target: PairingEndpointSummary(
        name: targetTitle,
        iconName: "externaldrive.connected.to.line.below",
        state: targetState,
        metadata: [
          .init(id: "target-address", value: targetAddress, emphasized: true),
          .init(id: "target-root", value: statusTargetRoot == "-" ? "target root unknown" : statusTargetRoot),
        ]
      ),
      transport: PairingTransportSummary(
        title: networkEvidenceState == .notChecked ? "LAN pending" : networkStatus,
        detail: "Pinned trust ceremony over explicit operator confirmation",
        note: store.selectedRole == .observer ? "Observer can inspect trust evidence only." : nil,
        symbolName: "lock.shield",
        state: targetState
      ),
      trustHighlights: [
        PairingTrustHighlight(
          id: "pairing-status",
          title: "Pairing receipt",
          detail: latestPairSummary ?? "No durable pairing receipt loaded yet",
          state: sourceState
        ),
        PairingTrustHighlight(
          id: "profile-state",
          title: "Config sealed",
          detail: profilePathIsSet ? "Migration config selected for this ceremony" : "Migration config required before transfer",
          state: profilePathIsSet ? .success : .warning
        ),
      ],
      expiry: PairingExpirySummary(
        label: store.serveReadinessSnapshot?.trusted == true ? "Trusted session active" : "Until operator confirms pin",
        detail: store.serveReadinessSnapshot?.mode ?? "Serve readiness not loaded"
      ),
      checklist: [
        PairingChecklistItem(
          id: "discover",
          title: "Select source and target surfaces",
          detail: "Discovery is only a hint; explicit source/target addresses stay auditable.",
          state: profilePathIsSet ? .complete : .current
        ),
        PairingChecklistItem(
          id: "serve",
          title: "Start target serve or advertise",
          detail: "Target operator exposes the verification ceremony surface.",
          state: store.serveReadinessSnapshot == nil ? .current : .complete
        ),
        PairingChecklistItem(
          id: "pin",
          title: "Confirm pairing pin and receipt",
          detail: "Use the current verification code and durable receipt before transfer.",
          state: pairingEvidenceState == .pass ? .complete : .pending
        ),
      ],
      code: PairingCodePanelModel(
        value: pairingCode,
        caption: "Enter this pin on the target to establish trust.",
        helperText: store.selectedRole == .target ? "Refreshing the pin restarts the live serve ceremony on this Mac." : "Source operators should match this code against the target serve surface."
      ),
      inspector: PairingInspectorModel(
        title: store.selectedRole == .target ? "On target: accept pairing" : "Pairing status",
        subtitle: "Use the target device to confirm the request and verify the current pin.",
        instructions: [
          "Open SuperMover on the target device and enter pairing mode.",
          "Confirm the source host, config identity, and current verification pin.",
          "Continue only after a durable receipt is recorded.",
        ],
        events: inspectorEvents,
        notice: PairingInspectorNotice(
          title: pairingEvidenceState == .pass ? "Trusted connection" : "Trust not complete",
          detail: pairingEvidenceState == .pass ? "You can continue to transfer when the target operator is ready." : "Do not treat discovery or serve readiness alone as trust proof.",
          state: pairingEvidenceState == .pass ? .info : .warning
        )
      ),
      cancelTitle: "Control Room",
      backTitle: "Device State",
      continueTitle: store.selectedRole == .observer ? "Open Evidence" : (store.selectedRole == .target ? "Start Target Serve" : "Pair Verified Target")
    )
  }

  private var sourcePairingWorkflow: some View {
    VStack(alignment: .leading, spacing: 16) {
      availabilityNotice(
        title: "Source pairing boundary",
        detail:
          "LAN candidates are address hints only. Use a hint to fill the address if useful, then confirm the verification code shown by target serve before running Pair.",
        state: .neutral
      )
      HStack(spacing: 14) {
        field(
          "Target / Explicit Address", text: $store.pairingTargetAddress,
          placeholder: "host:port or http(s) endpoint")
        field(
          "Verification Code", text: $store.pairingVerificationCode,
          placeholder: "Shown by target serve")
        field("Pair Timeout", text: $store.pairingTimeout, placeholder: "5s")
      }
      HStack(spacing: 14) {
        field("Browse Listen", text: $store.discoveryBrowseListen, placeholder: "0.0.0.0:39394")
        field("Browse Timeout", text: $store.discoveryBrowseTimeout, placeholder: "2s")
        pairingMethodPicker
      }
      WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
        ActionButton("Browse LAN Hints", systemImage: "dot.radiowaves.left.and.right") {
          run(.discoverBrowse)
        }
        ActionButton("Check Address Hint", systemImage: "scope") { run(.discoverAddress) }
        PrimaryActionButton("Pair Verified Target", systemImage: "link.badge.plus") { run(.pair) }
        ActionButton("Read Status", systemImage: "waveform.path.ecg") { run(.status) }
        ActionButton("Network Dry Run", systemImage: "network") { run(.networkDryRun) }
      }
      PairingReceiptPanel(
        draft: $store.pairingReceipt,
        role: store.selectedRole,
        runTask: run,
        chooseExportTarget: store.choosePairingReceiptExportTarget,
        browseImportReceipt: store.browsePairingReceiptImportFile
      )
      ProfileNetworkPanel(
        draft: $store.profileNetwork,
        role: store.selectedRole,
        networkStatus: networkStatus,
        networkTint: tint(for: networkStatus),
        runTask: run
      )
      PairingEvidenceSummaryView(
        statusPairing: store.statusSnapshot?.pairing,
        reportPairing: store.reportSnapshot?.pairing
      )
      commandPreview(for: .pair)
      if let summary = latestPairSummary {
        evidenceLine("latest pair", summary)
      }
      discoveryEvidencePanel
    }
  }

  private var targetPairingWorkflow: some View {
    VStack(alignment: .leading, spacing: 16) {
      availabilityNotice(
        title: "Target pairing surface",
        detail:
          "Advertise emits bounded low-information hints. Serve emits the verification code and remains the foreground trust ceremony endpoint.",
        state: .neutral
      )
      HStack(spacing: 14) {
        field("Listen Address", text: $store.listenAddress, placeholder: "127.0.0.1:0")
        field(
          "Advertise Listen", text: $store.discoveryAdvertiseListen,
          placeholder: "CLI default or config receiver URL")
        field(
          "Advertise Destination", text: $store.discoveryAdvertiseDestination,
          placeholder: "255.255.255.255:39394")
        field("Advertise Duration", text: $store.discoveryAdvertiseDuration, placeholder: "10s")
        field("Advertise Interval", text: $store.discoveryAdvertiseInterval, placeholder: "1s")
      }
      WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
        ActionButton("Advertise Hint", systemImage: "antenna.radiowaves.left.and.right") {
          run(.discoverAdvertise)
        }
        PrimaryActionButton("Start Target Serve", systemImage: "play.fill") { run(.serve) }
        ActionButton("Stop Target Serve", systemImage: "stop.fill") {
          store.stopProcess(in: .targetServe)
        }
        ActionButton("Read Status", systemImage: "waveform.path.ecg") { run(.status) }
      }
      ViewThatFits(in: .horizontal) {
        HStack(alignment: .top, spacing: 16) {
          targetPairingPreviewColumn
            .frame(maxWidth: .infinity, alignment: .leading)
          targetPairingStatusColumn
            .frame(width: 330, alignment: .leading)
        }
        VStack(alignment: .leading, spacing: 16) {
          targetPairingPreviewColumn
          targetPairingStatusColumn
        }
      }
      ServeReadinessPanel(snapshot: store.serveReadinessSnapshot)
      PairingReceiptPanel(
        draft: $store.pairingReceipt,
        role: store.selectedRole,
        runTask: run,
        chooseExportTarget: store.choosePairingReceiptExportTarget,
        browseImportReceipt: store.browsePairingReceiptImportFile
      )
      ProfileNetworkPanel(
        draft: $store.profileNetwork,
        role: store.selectedRole,
        networkStatus: networkStatus,
        networkTint: tint(for: networkStatus),
        runTask: run
      )
      PairingEvidenceSummaryView(
        statusPairing: store.statusSnapshot?.pairing,
        reportPairing: store.reportSnapshot?.pairing
      )
      discoveryEvidencePanel
    }
  }

  private var targetPairingPreviewColumn: some View {
    VStack(alignment: .leading, spacing: 10) {
      commandPreview(for: .discoverAdvertise)
      commandPreview(for: .serve)
    }
  }

  private var targetPairingStatusColumn: some View {
    VStack(alignment: .leading, spacing: 10) {
      EvidenceChip(label: "pairing", value: pairingStatus, tint: tint(for: pairingStatus))
      EvidenceChip(label: "receiver", value: networkStatus, tint: tint(for: networkStatus))
      if let summary = ServeInfoParser.firstSummary(
        in: store.run(in: .targetServe)?.stderr ?? ""
      ) {
        Text(summary)
          .font(.system(size: 12, design: .monospaced))
          .foregroundStyle(SMColor.secondaryText)
          .textSelection(.enabled)
      }
    }
  }

  private var pairingMethodPicker: some View {
    VStack(alignment: .leading, spacing: 8) {
      Text("Method")
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      Picker("Method", selection: $store.pairingMethod) {
        ForEach(["sas", "short_code", "qr", "tofu"], id: \.self) { method in
          Text(method).tag(method)
        }
      }
      .labelsHidden()
      .pickerStyle(.segmented)
      .frame(maxWidth: .infinity, alignment: .leading)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  @ViewBuilder
  private var discoveryEvidencePanel: some View {
    if let browse = store.discoveryBrowseSnapshot {
      browseSnapshotPanel(browse)
    } else if let hints = store.discoveryHintsSnapshot {
      explicitHintsPanel(hints)
    } else if let advertise = store.discoveryAdvertiseSnapshot {
      advertiseSnapshotPanel(advertise)
    } else {
      Text(
        "Run Discover Browse, Discover Address, or Advertise Hint to load low-information discovery evidence."
      )
      .font(.system(size: 13))
      .foregroundStyle(SMColor.secondaryText)
    }
  }

  private func explicitHintsPanel(_ hints: [DiscoveryAddressHintSnapshot]) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile("hints", value: "\(hints.count)", tint: SMColor.blue)
        metricTile(
          "trusted", value: hints.contains(where: { $0.trusted }) ? "unexpected" : "false",
          tint: hints.contains(where: { $0.trusted }) ? SMColor.amber : SMColor.blue)
      }
      ForEach(hints) { hint in
        hintCard(hint: hint, classification: "explicit", duplicateCount: 1, ambiguityReasons: [])
      }
    }
  }

  private func browseSnapshotPanel(_ browse: DiscoveryBrowseSnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile(
          "candidates", value: "\(browse.candidate_count)",
          tint: browse.candidate_count == 0 ? SMColor.secondaryText : SMColor.blue)
        metricTile(
          "invalid packets", value: "\(browse.invalid_packets)",
          tint: browse.invalid_packets == 0 ? SMColor.blue : SMColor.amber)
        metricTile(
          "trusted", value: browse.trusted ? "unexpected" : "false",
          tint: browse.trusted ? SMColor.amber : SMColor.blue)
      }
      evidenceLine("source", browse.source)
      evidenceLine("listen", browse.listen)
      if browse.candidates.isEmpty {
        Text("No candidates were seen in this browse window.")
          .font(.system(size: 13))
          .foregroundStyle(SMColor.secondaryText)
      } else {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 280), spacing: 12)], spacing: 12) {
          ForEach(browse.candidates) { candidate in
            hintCard(
              hint: candidate.hint,
              classification: candidate.classification,
              duplicateCount: candidate.duplicate_count,
              ambiguityReasons: candidate.ambiguity_reasons ?? []
            )
          }
        }
      }
    }
  }

  private func advertiseSnapshotPanel(_ advertise: DiscoveryAdvertiseSnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile("advertise", value: advertise.status, tint: SMColor.blue)
        metricTile(
          "trusted", value: advertise.trusted ? "unexpected" : "false",
          tint: advertise.trusted ? SMColor.amber : SMColor.blue)
        metricTile("duration", value: advertise.duration, tint: SMColor.blue)
      }
      evidenceLine("listen", advertise.listen)
      evidenceLine("destination", advertise.destination)
      evidenceLine("protocol", "\(advertise.service_type) · \(advertise.protocol_version)")
      evidenceLine("caps", advertise.capability_flags.joined(separator: ","))
      evidenceLine("nonce", advertise.ephemeral_nonce)
    }
  }

  private func hintCard(
    hint: DiscoveryAddressHintSnapshot, classification: String, duplicateCount: Int,
    ambiguityReasons: [String]
  ) -> some View {
    VStack(alignment: .leading, spacing: 10) {
      HStack {
        EvidenceChip(
          label: "class", value: classification, tint: discoveryClassTint(classification))
        EvidenceChip(
          label: "trusted", value: hint.trusted ? "unexpected" : "false",
          tint: hint.trusted ? SMColor.amber : SMColor.blue)
        Spacer()
      }
      Text(hint.address)
        .font(.system(size: 14, weight: .semibold, design: .monospaced))
        .textSelection(.enabled)
      Text("\(hint.advertisement.service_type) · \(hint.advertisement.protocol_version)")
        .font(.system(size: 11, design: .monospaced))
        .foregroundStyle(SMColor.secondaryText)
        .textSelection(.enabled)
      evidenceLine("caps", hint.advertisement.capability_flags.joined(separator: ","))
      evidenceLine("nonce", hint.advertisement.ephemeral_nonce)
      evidenceLine("expires", hint.expires_at)
      if duplicateCount > 1 {
        evidenceLine("duplicates", "\(duplicateCount)")
      }
      if !ambiguityReasons.isEmpty {
        Text(ambiguityReasons.joined(separator: "; "))
          .font(.system(size: 12))
          .foregroundStyle(SMColor.amber)
          .fixedSize(horizontal: false, vertical: true)
      }
      ActionButton("Use Address", systemImage: "arrow.turn.down.right") {
        store.usePairingTargetAddress(hint.address)
      }
    }
    .padding(14)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).stroke(SMColor.hairline))
  }

  private var transferView: some View {
    TransferSectionView(
      model: transferSectionModel,
      primaryControl: transferPrimaryControl,
      secondaryControl: nil,
      supportingModel: transferSupportingModel,
      onSelectStage: nil,
      onOpenTerminal: nil,
      onOpenEvidence: {
        showEvidenceVault()
        run(.status)
      },
      onInspectorAction: nil
    )
  }

  private var syncView: some View {
    DetailPageHost(
      header: .init(
        title: "Incremental Sync",
        subtitle:
          "Operate the durable changed-file queue and foreground sync loops without implying background daemon behavior."
      ),
      headerAccessoryPlacement: .top,
      headerAccessory: {
        detailPageAccessoryBar(showPairing: false, showNetwork: true)
      },
      primary: {
        ScreenCard(
          title: "Incremental Sync",
          subtitle:
            "Queue evidence, bounded passes, foreground loops, and discovery-gated network runs are CLI-backed surfaces."
        ) {
          sectionAvailabilityBanner(for: .sync)
          if store.selectedRole == .source {
            availabilityNotice(
              title: "Foreground, not detached",
              detail:
                "Queue commands are durable evidence operations. Run/discover-run are bounded. Loop/watch/network loop are supervised foreground processes and stop when their process is terminated.",
              state: .neutral
            )
            syncQueueControls(mutating: true)
            syncExecutionControls
            syncInputs
            syncSnapshotPanel
          } else {
            availabilityNotice(
              title: "Read-only sync evidence",
              detail:
                "This role can inspect queue status/list/ready from selected config target evidence. Enqueue, cancel/fail, local runs, watchers, and network execution stay source-owned.",
              state: .neutral
            )
            syncQueueControls(mutating: false)
            syncSnapshotPanel
          }
        }
      }
    )
  }

  private func syncQueueControls(mutating: Bool) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      Text("Queue")
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
        if mutating {
          ActionButton("Enqueue Snapshot", systemImage: "tray.and.arrow.down") {
            run(.syncQueueEnqueue)
          }
        }
        ActionButton("Status", systemImage: "waveform.path.ecg") { run(.syncQueueStatus) }
        ActionButton("List", systemImage: "list.bullet.rectangle") { run(.syncQueueList) }
        ActionButton("Ready", systemImage: "checklist") { run(.syncQueueReady) }
        if mutating {
          ActionButton("Cancel Entry", systemImage: "xmark.circle") { run(.syncQueueCancel) }
          ActionButton("Fail Entry", systemImage: "exclamationmark.triangle") {
            run(.syncQueueFail)
          }
        }
      }
    }
  }

  private var syncExecutionControls: some View {
    VStack(alignment: .leading, spacing: 12) {
      Text("Execution")
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
        PrimaryActionButton("Local Run", systemImage: "play.fill") { run(.syncRun) }
        ActionButton("Network Run", systemImage: "network") { run(.syncNetworkRun) }
        ActionButton("Discover-Gated Run", systemImage: "dot.radiowaves.left.and.right") {
          run(.syncNetworkDiscoverRun)
        }
      }
      WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
        ActionButton("Start Local Loop", systemImage: "repeat") { run(.syncLoop) }
        ActionButton("Stop Local Loop", systemImage: "stop.fill") {
          store.stopProcess(in: .sourceSyncLoop)
        }
        ActionButton("Start Watch", systemImage: "eye") { run(.syncWatch) }
        ActionButton("Stop Watch", systemImage: "stop.fill") {
          store.stopProcess(in: .sourceSyncWatch)
        }
        ActionButton("Start Network Loop", systemImage: "network") { run(.syncNetworkLoop) }
        ActionButton("Stop Network Loop", systemImage: "stop.fill") {
          store.stopProcess(in: .sourceNetworkLoop)
        }
      }
    }
  }

  private var syncInputs: some View {
    VStack(alignment: .leading, spacing: 14) {
      HStack(spacing: 14) {
        field("Session ID", text: $store.sessionID, placeholder: "Required for bounded sync run")
        field("Session Prefix", text: $store.sessionPrefix, placeholder: "Required for loop/watch")
        field("Queue Entry ID", text: $store.queueEntryID, placeholder: "Required for cancel/fail")
      }
      HStack(spacing: 14) {
        field("Retry Backoff", text: $store.syncRetryBackoff, placeholder: "1m")
        field("Interval", text: $store.syncInterval, placeholder: "1m")
        field("Max Runs", text: $store.syncMaxRuns, placeholder: "0 = until stopped")
      }
      HStack(spacing: 14) {
        field("Watch Settle", text: $store.syncSettle, placeholder: "250ms")
        field("Max Events", text: $store.syncMaxEvents, placeholder: "0 = until stopped")
        field("Discover Listen", text: $store.syncDiscoveryListen, placeholder: "0.0.0.0:39394")
        field("Discover Timeout", text: $store.syncDiscoveryTimeout, placeholder: "2s")
      }
      reasonInput
    }
  }

  @ViewBuilder
  private var syncSnapshotPanel: some View {
    if let queue = store.syncQueueSnapshot {
      syncQueueSnapshotPanel(queue)
    } else if let run = store.syncRunSnapshot {
      syncRunSnapshotPanel(run, network: nil)
    } else if let networkRun = store.syncNetworkRunSnapshot {
      syncRunSnapshotPanel(
        SyncRunSnapshot(
          operation: networkRun.operation, mode: networkRun.mode, enqueue: networkRun.enqueue,
          run: networkRun.run), network: networkRun.network)
    } else if let discover = store.syncNetworkDiscoverRunSnapshot {
      syncDiscoverSnapshotPanel(discover)
    } else if let loop = store.syncLoopSnapshot {
      syncLoopSnapshotPanel(loop)
    } else if let watch = store.syncWatchSnapshot {
      syncWatchSnapshotPanel(watch)
    } else if let networkLoop = store.syncNetworkLoopSnapshot {
      syncNetworkLoopSnapshotPanel(networkLoop)
    } else {
      Text("Run a sync queue or sync execution command to load structured sync evidence.")
        .font(.system(size: 13))
        .foregroundStyle(SMColor.secondaryText)
    }
  }

  private func syncQueueSnapshotPanel(_ queue: SyncQueueSnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile("operation", value: queue.operation, tint: SMColor.blue)
        metricTile(
          "ready", value: "\(queue.summary.ready)",
          tint: queue.summary.ready == 0 ? SMColor.secondaryText : SMColor.blue)
        metricTile(
          "backoff", value: "\(queue.summary.backoff)",
          tint: queue.summary.backoff == 0 ? SMColor.green : SMColor.amber)
        metricTile(
          "failed", value: "\(queue.summary.failed)",
          tint: queue.summary.failed == 0 ? SMColor.green : SMColor.amber)
      }
      evidenceLine("mode", queue.mode)
      evidenceLine("state", queue.state ?? "present")
      evidenceLine("state path", queue.summary.state_path ?? queue.state_path ?? "-")
      if let reason = queue.reason {
        evidenceLine("reason", reason)
      }
      syncEntryList(queue.entries ?? queue.enqueued ?? queue.entry.map { [$0] } ?? [])
    }
  }

  private func syncRunSnapshotPanel(_ run: SyncRunSnapshot, network: SyncNetworkPlanSnapshot?)
    -> some View
  {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile("run", value: run.run.status, tint: tint(for: run.run.status))
        metricTile("published", value: "\(run.run.published?.count ?? 0)", tint: SMColor.green)
        metricTile(
          "retried", value: "\(run.run.retried?.count ?? 0)",
          tint: (run.run.retried?.isEmpty ?? true) ? SMColor.green : SMColor.amber)
        metricTile("total", value: "\(run.run.summary.total)", tint: SMColor.blue)
      }
      evidenceLine("mode", run.mode)
      evidenceLine("session", run.run.session_id)
      evidenceLine("run path", run.run.run_path ?? "-")
      if let error = run.run.error, !error.isEmpty {
        evidenceLine("error", error)
      }
      if let network {
        evidenceLine("network", "\(network.transfer) · \(network.encrypted_transfer)")
        evidenceLine("stage", network.stage ?? "-")
      }
      syncEntryList(run.run.published ?? [])
    }
  }

  private func syncDiscoverSnapshotPanel(_ discover: SyncNetworkDiscoverRunSnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile(
          "discovery", value: discover.discovery.status,
          tint: discover.discovery.status == "matched" ? SMColor.blue : SMColor.amber)
        metricTile(
          "trusted", value: discover.discovery.trusted ? "unexpected" : "false",
          tint: discover.discovery.trusted ? SMColor.amber : SMColor.blue)
        metricTile("candidates", value: "\(discover.discovery.candidate_count)", tint: SMColor.blue)
        metricTile(
          "invalid", value: "\(discover.discovery.invalid_packets)",
          tint: discover.discovery.invalid_packets == 0 ? SMColor.green : SMColor.amber)
      }
      evidenceLine("profile address", discover.discovery.profile_address)
      evidenceLine("matched address", discover.discovery.matched_address ?? "-")
      evidenceLine("reason", discover.discovery.reason)
      if discover.executedRun, let run = discover.run, let enqueue = discover.enqueue {
        syncRunSnapshotPanel(
          SyncRunSnapshot(
            operation: discover.operation, mode: discover.mode, enqueue: enqueue, run: run),
          network: discover.network)
      } else {
        Text("No sync run executed; discovery gate did not pass.")
          .font(.caption)
          .foregroundStyle(SMColor.amber)
      }
    }
  }

  private func syncLoopSnapshotPanel(_ loop: SyncLoopSnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile("loop", value: loop.status, tint: tint(for: loop.status))
        metricTile("runs", value: "\(loop.completed_runs)", tint: SMColor.blue)
        metricTile("published", value: "\(loop.published_runs)", tint: SMColor.green)
        metricTile(
          "retrying", value: "\(loop.retrying_runs)",
          tint: loop.retrying_runs == 0 ? SMColor.green : SMColor.amber)
      }
      evidenceLine("session prefix", loop.session_prefix)
      evidenceLine("interval", "\(loop.interval), max \(loop.max_runs)")
      syncEntryList(loop.runs?.flatMap { $0.run.published ?? [] } ?? [])
    }
  }

  private func syncWatchSnapshotPanel(_ watch: SyncWatchSnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile("watch", value: watch.status, tint: tint(for: watch.status))
        metricTile("batches", value: "\(watch.event_batches)", tint: SMColor.blue)
        metricTile("events", value: "\(watch.events_seen)", tint: SMColor.blue)
        metricTile(
          "retrying", value: "\(watch.retrying_runs)",
          tint: watch.retrying_runs == 0 ? SMColor.green : SMColor.amber)
      }
      evidenceLine("session prefix", watch.session_prefix)
      evidenceLine("watched dirs", "\(watch.watched_dirs)")
      syncEntryList(watch.runs?.flatMap { $0.run.published ?? [] } ?? [])
    }
  }

  private func syncNetworkLoopSnapshotPanel(_ loop: SyncNetworkLoopSnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 12) {
        metricTile("network loop", value: loop.status, tint: tint(for: loop.status))
        metricTile("runs", value: "\(loop.completed_runs)", tint: SMColor.blue)
        metricTile("attempts", value: "\(loop.network_attempts)", tint: SMColor.blue)
        metricTile(
          "not attempted", value: "\(loop.network_not_attempted_runs)", tint: SMColor.secondaryText)
      }
      evidenceLine("session prefix", loop.session_prefix)
      evidenceLine("interval", "\(loop.interval), max \(loop.max_runs)")
      syncEntryList(loop.runs?.flatMap { $0.run.published ?? [] } ?? [])
    }
  }

  private func syncEntryList(_ entries: [SyncQueueEntrySnapshot]) -> some View {
    VStack(alignment: .leading, spacing: 8) {
      Text("Entries")
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      if entries.isEmpty {
        Text("No entries in this structured result.")
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
      } else {
        ForEach(entries.prefix(8)) { entry in
          HStack(spacing: 8) {
            EvidenceChip(label: "status", value: entry.status, tint: tint(for: entry.status))
            EvidenceChip(label: "kind", value: entry.kind, tint: SMColor.blue)
            Text(entry.path)
              .font(.system(size: 12, design: .monospaced))
              .foregroundStyle(SMColor.primaryText)
              .lineLimit(1)
              .textSelection(.enabled)
            Spacer()
            Text(entry.id)
              .font(.system(size: 11, design: .monospaced))
              .foregroundStyle(SMColor.secondaryText)
              .lineLimit(1)
              .textSelection(.enabled)
          }
          .padding(10)
          .background(SMColor.input)
          .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
          .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
        }
      }
    }
  }

  private var verificationView: some View {
    DetailPageHost(
      header: .init(
        title: "Verify",
        subtitle: "Verify target content before declaring alignment; repair decisions stay in this owner surface."
      ),
      headerAccessoryPlacement: .top,
      headerAccessory: {
        detailPageAccessoryBar(showPairing: true, showNetwork: false, showIntegrity: true)
      },
      primary: {
        ScreenCard(
          title: "Verification Comparator",
          subtitle:
            "Target state is checked against published manifest evidence. Merkle/root proof and current-source comparison are shown only when wired evidence exists."
        ) {
          HStack(spacing: 16) {
            metricTile("status", value: verificationStatus, tint: tint(for: verificationStatus))
            metricTile("files", value: verifiedFilesMetricValue, tint: verificationFilesTint)
            metricTile(
              "warnings", value: warningMetricValue, tint: countMetricTint(warningCountEvidence))
            metricTile(
              "artifact issues", value: artifactProblemMetricValue,
              tint: countMetricTint(artifactProblemCountEvidence))
          }
          WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
            PrimaryActionButton("Run Verify", systemImage: "checkmark.seal") { run(.verify) }
            if store.selectedRole != .source {
              ActionButton("Start Dashboard", systemImage: "safari") { run(.dashboard) }
              ActionButton("Open Dashboard URL", systemImage: "arrow.up.forward.app") {
                store.revealDashboardURL()
              }
            } else {
              EvidenceChip(label: "dashboard", value: "target/observer role", tint: SMColor.amber)
            }
          }
          if let verify = store.verifySnapshot {
            verificationComparatorPanel(verify)
          } else {
            Text(
              "Run Verify to load typed target-vs-manifest evidence. A successful process alone is not treated as alignment proof."
            )
            .font(.caption)
            .foregroundStyle(SMColor.secondaryText)
          }
          rootEvidencePanel(store.verifySnapshot)
        }
      }
    )
  }

  private func verificationComparatorPanel(_ verify: VerifySnapshot) -> some View {
    VStack(alignment: .leading, spacing: 12) {
      Divider().overlay(SMColor.hairline)
      HStack(spacing: 12) {
        metricTile(
          "manifest",
          value: verify.summary.manifest_count == 0 ? "missing" : verify.manifest.manifestID,
          tint: verify.summary.manifest_count == 0 ? SMColor.amber : SMColor.blue)
        metricTile(
          "session",
          value: verify.manifest.session_id.isEmpty
            ? (verify.session_id ?? "-") : verify.manifest.session_id, tint: SMColor.blue)
        metricTile(
          "findings", value: "\(verify.summary.error_findings + verify.summary.warning_findings)",
          tint: (verify.summary.error_findings + verify.summary.warning_findings) == 0
            ? SMColor.green : SMColor.amber)
        metricTile(
          "drift", value: "\(verify.summary.target_drifts)",
          tint: verify.summary.target_drifts == 0 ? SMColor.green : SMColor.amber)
      }
      evidenceLine("target root", verify.target_root)
      evidenceLine("profile root id", verify.manifest.root_id ?? "unavailable")
      evidenceLine("created", verify.manifest.created_at.isEmpty ? "-" : verify.manifest.created_at)
      evidenceLine("entries", "\(verify.summary.manifest_entries)")
      if let findings = verify.findings, !findings.isEmpty {
        verificationFindingList(findings)
      }
      if let warnings = verify.warnings, !warnings.isEmpty {
        verificationWarningList(warnings)
      }
      if let softDeletes = verify.soft_deletes, !softDeletes.isEmpty {
        verificationSoftDeleteList(softDeletes)
      }
      if let drifts = verify.target_drifts, !drifts.isEmpty {
        verificationTargetDriftList(drifts)
      }
      if let problems = verify.artifact_problems, !problems.isEmpty {
        artifactProblemList(problems)
      }
    }
  }

  private func rootEvidencePanel(_ verify: VerifySnapshot?) -> some View {
    VStack(alignment: .leading, spacing: 8) {
      Divider().overlay(SMColor.hairline)
      Text("Root Evidence Availability")
        .font(.caption.weight(.semibold))
        .foregroundStyle(SMColor.primaryText)
      let rootIdentity =
        verify?.profileRootIdentity ?? .unavailable("No verify evidence is loaded.")
      availabilityLine("profile root identity", availability: rootIdentity)
      availabilityLine(
        "Merkle/root proof",
        availability: verify?.merkleRootProof ?? .unavailable("No verify evidence is loaded."))
      availabilityLine(
        "current source comparison",
        availability: verify?.currentSourceComparison
          ?? .unavailable("No verify evidence is loaded."))
    }
  }

  private func availabilityLine(_ label: String, availability: EvidenceAvailability) -> some View {
    HStack(alignment: .firstTextBaseline, spacing: 8) {
      EvidenceChip(
        label: label, value: availability.label,
        tint: availability.label == "available" ? SMColor.blue : SMColor.amber)
      Text(availability.detail)
        .font(.caption)
        .foregroundStyle(SMColor.secondaryText)
    }
  }

  private func verificationFindingList(_ findings: [VerifySnapshot.Finding]) -> some View {
    VStack(alignment: .leading, spacing: 6) {
      Text("Verification Findings")
        .font(.caption.weight(.semibold))
        .foregroundStyle(SMColor.primaryText)
      ForEach(Array(findings.prefix(5))) { finding in
        VStack(alignment: .leading, spacing: 2) {
          Text("\(finding.severity) · \(finding.kind)")
            .font(.caption.weight(.semibold))
            .foregroundStyle(tint(for: finding.severity))
          Text("\(finding.path) -> \(finding.target_path)")
            .font(.caption2)
            .foregroundStyle(SMColor.secondaryText)
          Text(finding.message)
            .font(.caption2)
            .foregroundStyle(SMColor.secondaryText)
        }
      }
    }
  }

  private func verificationWarningList(_ warnings: [VerifySnapshot.WarningRecord]) -> some View {
    VStack(alignment: .leading, spacing: 6) {
      Text("Warnings")
        .font(.caption.weight(.semibold))
        .foregroundStyle(SMColor.primaryText)
      ForEach(Array(warnings.prefix(5))) { warning in
        VStack(alignment: .leading, spacing: 2) {
          Text("\(warning.severity ?? "warning") · \(warning.code)")
            .font(.caption.weight(.semibold))
            .foregroundStyle(tint(for: warning.severity ?? "warning"))
          Text(warning.message)
            .font(.caption2)
            .foregroundStyle(SMColor.secondaryText)
          evidenceLine("paths", warningPathSummary(warning))
        }
      }
    }
  }

  private func verificationSoftDeleteList(_ softDeletes: [VerifySnapshot.SoftDeleteRecord])
    -> some View
  {
    VStack(alignment: .leading, spacing: 6) {
      Text("Soft Deletes")
        .font(.caption.weight(.semibold))
        .foregroundStyle(SMColor.primaryText)
      ForEach(Array(softDeletes.prefix(5))) { record in
        VStack(alignment: .leading, spacing: 2) {
          Text(record.target_path)
            .font(.caption.weight(.semibold))
            .foregroundStyle(SMColor.amber)
          evidenceLine("source", record.source_path)
          evidenceLine("reason", record.reason ?? "review before prune")
        }
      }
    }
  }

  private func verificationTargetDriftList(_ drifts: [VerifySnapshot.TargetDriftRecord])
    -> some View
  {
    VStack(alignment: .leading, spacing: 6) {
      Text("Target Drift")
        .font(.caption.weight(.semibold))
        .foregroundStyle(SMColor.primaryText)
      ForEach(Array(drifts.prefix(5))) { drift in
        VStack(alignment: .leading, spacing: 2) {
          Text("\(drift.change) · \(drift.path)")
            .font(.caption.weight(.semibold))
            .foregroundStyle(SMColor.amber)
          evidenceLine("review state", drift.review_state ?? "needs review")
          if let detectedAt = drift.detected_at, !detectedAt.isEmpty {
            evidenceLine("detected", detectedAt)
          }
        }
      }
    }
  }

  private func warningPathSummary(_ warning: VerifySnapshot.WarningRecord) -> String {
    if let paths = warning.paths, !paths.isEmpty {
      return paths.prefix(3).joined(separator: ", ")
    }
    if let targetPath = warning.target_path, !targetPath.isEmpty {
      return targetPath
    }
    return "-"
  }

  private func artifactProblemList(_ problems: [VerifySnapshot.ArtifactProblem]) -> some View {
    VStack(alignment: .leading, spacing: 6) {
      Text("Artifact Problems")
        .font(.caption.weight(.semibold))
        .foregroundStyle(SMColor.primaryText)
      ForEach(Array(problems.prefix(5))) { problem in
        evidenceLine(problem.path, problem.error)
      }
    }
  }

  private var evidenceView: some View {
    EvidenceScreen()
  }

  private var driftReviewView: some View {
    DetailPageHost(
      header: .init(
        title: "Drift Review",
        subtitle: "Persist, review, reconcile and prune target drift explicitly."
      ),
      headerAccessoryPlacement: .top,
      headerAccessory: {
        detailPageAccessoryBar(showPairing: false, showNetwork: false, showIntegrity: true)
      },
      primary: {
        ScreenCard(
          title: "Review Operations",
          subtitle: "Persist, review, reconcile and prune only through explicit review surfaces."
        ) {
          sectionAvailabilityBanner(for: .driftReview)
          if store.selectedRole == .source {
            HStack(spacing: 16) {
              actionCard(
                title: "Record drift",
                subtitle: "Persist current live drift findings as review records.", task: .driftRecord,
                primary: false)
              actionCard(
                title: "Reconcile plan",
                subtitle: "Build a narrow persisted-drift repair plan without mutation.",
                task: .reconcilePlan, primary: false)
              actionCard(
                title: "Reconcile apply",
                subtitle: "Apply selected persisted-drift repair with reason and optional reviewer.",
                task: .reconcileApply, primary: true)
            }
            reviewInputs
          } else {
            availabilityNotice(
              title: "Read-only review mode",
              detail:
                "This role can inspect drift and prune evidence, but mutating drift record, reconcile apply, and approval authoring stay source/operator controlled.",
              state: .neutral
            )
            WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
              ActionButton("Drift List", systemImage: "list.bullet.rectangle") { run(.driftList) }
              ActionButton("Prune Review", systemImage: "doc.text.magnifyingglass") {
                run(.pruneReview)
              }
              ActionButton("Prune Approvals", systemImage: "checklist") { run(.pruneApprovals) }
            }
          }
        }
      }
    )
  }

  private var settingsView: some View {
    DetailPageHost(
      header: .init(
        title: appChromeLocalization.text(.settingsTitle),
        subtitle: appChromeLocalization.text(.settingsSubtitle)
      ),
      headerAccessoryPlacement: .top,
      asideWidth: 300,
      headerAccessory: {
        detailPageAccessoryBar(showPairing: false, showNetwork: true)
      },
      primary: {
        ScreenCard(
          title: appChromeLocalization.text(.commandInputsTitle),
          subtitle: appChromeLocalization.text(.commandInputsSubtitle)
        ) {
          profilePicker
          installReadinessPanel
          switch store.selectedRole {
          case .source:
            inputRow
            syncInputs
            reviewInputs
            daemonControls
          case .target:
            inputRow
            reasonInput
            daemonControls
          case .observer:
            availabilityNotice(
              title: "Observer mutation inputs are hidden",
              detail:
                "Observer mode can choose migration config and read-evidence inputs, but repair, prune approval, reconcile, publish, serve, and pair mutation inputs stay source or target owned.",
              state: .blocked
            )
            inputRow
          }
          cliTaskPanel
        }
      },
      aside: {
        uiPreferencesPanel
      }
    )
  }

  private var uiPreferencesPanel: some View {
    WorkbenchPanel(
      title: appChromeLocalization.text(.displayPreferencesTitle),
      subtitle: appChromeLocalization.text(.displayPreferencesSubtitle)
    ) {
      VStack(alignment: .leading, spacing: 14) {
        WorkbenchNotice(
          title: appChromeLocalization.text(.displayOnlyNoticeTitle),
          detail: appChromeLocalization.text(.displayOnlyNoticeDetail),
          state: .neutral
        )
        appearancePreferencePicker
        languagePreferencePicker
      }
    }
  }

  private var appearancePreferencePicker: some View {
    VStack(alignment: .leading, spacing: 8) {
      Text(appChromeLocalization.text(.appearancePickerTitle))
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      Picker(appChromeLocalization.text(.appearancePickerTitle), selection: $uiPreferences.appearance) {
        ForEach(UIAppearancePreference.allCases) { preference in
          Text(preference.localizedTitle(using: appChromeLocalization)).tag(preference)
        }
      }
      .labelsHidden()
      .pickerStyle(.segmented)
      .frame(maxWidth: .infinity, alignment: .leading)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var languagePreferencePicker: some View {
    VStack(alignment: .leading, spacing: 8) {
      Text(appChromeLocalization.text(.languagePickerTitle))
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      Picker(appChromeLocalization.text(.languagePickerTitle), selection: $uiPreferences.language) {
        ForEach(UILanguagePreference.allCases) { preference in
          Text(preference.localizedTitle(using: appChromeLocalization)).tag(preference)
        }
      }
      .labelsHidden()
      .pickerStyle(.menu)
      .frame(maxWidth: .infinity, alignment: .leading)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var installReadinessPanel: some View {
    VStack(alignment: .leading, spacing: 12) {
      WorkbenchResponsiveBar(alignment: .center, spacing: 12, compactSpacing: 10) {
        sectionLabel("Install Readiness")
      } trailing: {
        WorkbenchWrappingRow(spacing: 12, rowSpacing: 10) {
          ActionButton("Refresh Provenance", systemImage: "arrow.clockwise") {
            store.refreshCLIProvenance()
          }
          ActionButton("Run Version", systemImage: "terminal") {
            run(.version)
          }
        }
      }
      Text(
        "Readiness is evidence and guidance, not a hidden runtime override. Migration config files stay the SSOT; Local Network/firewall prompts are operator-facing macOS state."
      )
      .font(.caption)
      .foregroundStyle(SMColor.secondaryText)
      .fixedSize(horizontal: false, vertical: true)
      LazyVGrid(columns: [GridItem(.adaptive(minimum: 190), spacing: 10)], spacing: 10) {
        metricTile("CLI mode", value: store.cliProvenance.mode.title, tint: cliProvenanceTint)
        metricTile("CLI readiness", value: store.cliProvenance.readiness, tint: cliProvenanceTint)
        metricTile("app version", value: store.cliProvenance.appVersion, tint: SMColor.blue)
        metricTile(
          "bundle id", value: store.cliProvenance.bundleIdentifier, tint: SMColor.secondaryText)
      }
      evidenceLine("cli path", store.cliProvenance.executablePath)
      evidenceLine("working dir", store.cliProvenance.workingDirectoryPath)
      if let version = store.cliProvenance.bundledCLIVersion, !version.isEmpty {
        evidenceLine("bundled cli", version)
      }
      evidenceLine("provenance status", store.cliProvenance.provenanceStatus)
      if let commit = store.cliProvenance.bundleCommit, !commit.isEmpty {
        evidenceLine("bundle commit", commit)
      }
      if let buildProfile = store.cliProvenance.buildProfile, !buildProfile.isEmpty {
        evidenceLine("build profile", buildProfile)
      }
      if let signing = store.cliProvenance.signing, !signing.isEmpty {
        evidenceLine("signing", signing)
      }
      if let gitDirty = store.cliProvenance.gitDirty {
        evidenceLine("git dirty", String(gitDirty))
      }
      if let builtAt = store.cliProvenance.builtAt, !builtAt.isEmpty {
        evidenceLine("built at", builtAt)
      }
      if let provenancePath = store.cliProvenance.provenancePath {
        evidenceLine("provenance", provenancePath)
      }
      Text(store.cliProvenance.detail)
        .font(.caption)
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
      VStack(alignment: .leading, spacing: 6) {
        readinessBullet(
          "File access",
          "Select config/source/target paths explicitly. Sandboxed distribution must include user-selected read/write file access."
        )
        readinessBullet(
          "Local Network",
          "First LAN browse/advertise/serve may trigger macOS Local Network or firewall prompts; refusal is an operator readiness issue, not sync evidence."
        )
        readinessBullet(
          "Distribution",
          "Developer ID signing and notarization are packaging evidence. A local development build remains a development artifact."
        )
        readinessBullet(
          "Foreground daemon",
          "Daemon controls write foreground lifecycle evidence and intents; they do not install a detached OS service."
        )
      }
    }
    .padding(12)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
  }

  private var daemonControls: some View {
    VStack(alignment: .leading, spacing: 12) {
      sectionLabel("Foreground Daemon Controls")
      Text(
        "These commands use `daemon run --foreground` plus scoped stop/restart intents. They are supervised foreground processes, not launchd installation or detached background sync."
      )
      .font(.caption)
      .foregroundStyle(SMColor.secondaryText)
      .fixedSize(horizontal: false, vertical: true)
      WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
        ActionButton("Install Evidence", systemImage: "doc.badge.gearshape") { run(.daemonInstall) }
        PrimaryActionButton("Start Foreground Daemon", systemImage: "play.fill") { run(.daemonRun) }
        ActionButton("Request Restart", systemImage: "arrow.clockwise") { run(.daemonRestart) }
        ActionButton("Request Stop", systemImage: "stop.circle") { run(.daemonStop) }
        ActionButton("Status", systemImage: "waveform.path.ecg") { run(.daemonStatus) }
        ActionButton("Logs", systemImage: "list.bullet.rectangle") { run(.daemonLogs) }
      }
      ActionButton("Terminate App Process", systemImage: "xmark.circle") {
        store.stopProcess(in: .foregroundDaemon)
      }
      commandPreview(for: .daemonRun)
    }
  }

  private var cliProvenanceTint: Color {
    switch store.cliProvenance.mode {
    case .bundled:
      return store.cliProvenance.readiness == "ready" ? SMColor.green : SMColor.amber
    case .development:
      return SMColor.amber
    case .unavailable:
      return SMColor.red
    }
  }

  private func readinessBullet(_ title: String, _ detail: String) -> some View {
    HStack(alignment: .top, spacing: 8) {
      StatusDot(color: SMColor.blue)
      VStack(alignment: .leading, spacing: 2) {
        Text(title)
          .font(.caption.weight(.semibold))
          .foregroundStyle(SMColor.primaryText)
        Text(detail)
          .font(.caption2)
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
  }

  @ViewBuilder
  private func sectionAvailabilityBanner(for section: AppSection) -> some View {
    let availability = section.availability(for: store.selectedRole)
    if availability != .available {
      availabilityNotice(
        title: availability.label, detail: availability.detail,
        state: gateState(for: availability))
    }
  }

  private func availabilityNotice(title: String, detail: String, state: GateState) -> some View {
    HStack(alignment: .top, spacing: 10) {
      StatusDot(color: state.color)
      VStack(alignment: .leading, spacing: 4) {
        Text(title)
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        Text(detail)
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
    .panelSurface(.notice)
  }

  @ViewBuilder
  private func detailPageAccessoryBar(
    showPairing: Bool = true,
    showNetwork: Bool = true,
    showIntegrity: Bool = false
  ) -> some View {
    VStack(alignment: .trailing, spacing: 8) {
      WorkbenchWrappingRow(alignment: .trailing, spacing: 8, rowSpacing: 8) {
        WorkbenchPageHeaderValuePill(
          icon: "person.crop.circle.fill",
          label: appChromeLocalization.text(.sidebarRoleLabel),
          value: store.selectedRole.localizedTitle(using: appChromeLocalization),
          tint: SMColor.blue
        )
        if showPairing {
          WorkbenchPageHeaderValuePill(
            icon: "link.circle.fill",
            label: "pairing",
            value: pairingStatus,
            tint: tint(for: pairingStatus)
          )
        }
        if showNetwork {
          WorkbenchPageHeaderValuePill(
            icon: "dot.radiowaves.left.and.right",
            label: "network",
            value: networkStatus,
            tint: tint(for: networkStatus)
          )
        }
        WorkbenchPageHeaderValuePill(
          icon: "doc.text.fill",
          label: "config",
          value: profileStatus,
          tint: profilePathIsSet ? SMColor.green : SMColor.amber
        )
        if showIntegrity {
          WorkbenchPageHeaderValuePill(
            icon: "checkmark.shield.fill",
            label: "integrity",
            value: integrityValue,
            tint: tint(for: integrityValue)
          )
        }
      }
    }
    .frame(maxWidth: .infinity, alignment: .trailing)
  }

  private func gateState(for state: AcceptanceInstalledAppLaunchPreview.State) -> GateState {
    switch state {
    case .pass:
      return .pass
    case .review:
      return .review
    case .blocked:
      return .blocked
    }
  }

  private func gateState(for availability: SectionAvailability) -> GateState {
    switch availability {
    case .available:
      return .pass
    case .planned:
      return .planned
    case .readOnly:
      return .neutral
    case .roleGated:
      return .blocked
    }
  }

  private var profilePicker: some View {
    let displayContext = store.localizedProfileSelectionContext(using: appChromeLocalization)
    return VStack(alignment: .leading, spacing: 10) {
      WorkbenchResponsiveBar(alignment: .top, spacing: 12, compactSpacing: 12) {
        HStack(alignment: .top, spacing: 12) {
          Image(systemName: profileSelectionIcon)
            .font(.system(size: 16, weight: .semibold))
            .foregroundStyle(profileSelectionTint)
            .frame(width: 34, height: 34)
            .background(profileSelectionTint.opacity(0.12))
            .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))

          VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
              Text(appChromeLocalization.text(.setupConfigSelectedFileTitle))
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(SMColor.secondaryText)
              if profileSelectionState != .none {
                StatusBadge(
                  item: profileSelectionContext.pathState.localizedBadge(using: appChromeLocalization),
                  prominence: .plain,
                  compact: true
                )
              }
            }
            Text(displayContext.title)
              .font(.system(size: 13, weight: .semibold))
              .foregroundStyle(hasSelectedProfilePath ? SMColor.primaryText : SMColor.secondaryText)
              .lineLimit(1)
              .truncationMode(.middle)
            if let detail = displayContext.detail {
              Text(detail)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(SMColor.secondaryText)
                .lineLimit(1)
                .truncationMode(.tail)
            }
            if let metadata = displayContext.metadata {
              Text(metadata)
                .font(.system(size: 11))
                .foregroundStyle(SMColor.secondaryText.opacity(0.9))
                .lineLimit(1)
                .truncationMode(.middle)
            }
          }
          .frame(maxWidth: .infinity, alignment: .leading)
        }
      } trailing: {
        if hasSelectedProfilePath {
          CompactActionButton(appChromeLocalization.text(.setupActionClearConfig), systemImage: "xmark") {
            store.profilePath = ""
          }
        }
      }
      .padding(12)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(SMColor.input)
      .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
      .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))

      VStack(alignment: .leading, spacing: 8) {
        Button {
          withAnimation(.easeInOut(duration: 0.18)) {
            showProfileAdvanced.toggle()
          }
        } label: {
          HStack(spacing: 8) {
            Text(appChromeLocalization.text(.setupProfileAdvancedTitle))
              .font(.system(size: 11, weight: .semibold))
              .foregroundStyle(SMColor.secondaryText)
            Spacer()
            Image(systemName: showProfileAdvanced ? "chevron.up" : "chevron.down")
              .font(.system(size: 11, weight: .semibold))
              .foregroundStyle(SMColor.secondaryText)
          }
        }
        .buttonStyle(.plain)

        if showProfileAdvanced {
          profileAdvancedFields
        }
      }
    }
  }

  private var profileSelectionIcon: String {
    switch profileSelectionState {
    case .existingFile:
      return "checkmark.seal.fill"
    case .newDestination:
      return "sparkles"
    case .missingFile:
      return "exclamationmark.triangle.fill"
    case .directory:
      return "folder.fill.badge.questionmark"
    case .none:
      return "sparkles"
    }
  }

  private var profileSelectionTint: Color {
    switch profileSelectionState {
    case .existingFile:
      return SMColor.green
    case .newDestination:
      return SMColor.cyan
    case .missingFile, .directory:
      return SMColor.amber
    case .none:
      return SMColor.blue
    }
  }

  private var profileAdvancedFields: some View {
    VStack(alignment: .leading, spacing: 14) {
      WorkbenchWrappingRow(spacing: 10, rowSpacing: 10) {
        CompactActionButton(appChromeLocalization.text(.setupActionOpenExistingConfig), systemImage: "folder") {
          store.browseProfile()
        }
        if store.selectedRole == .source {
          CompactActionButton(appChromeLocalization.text(.setupActionChooseCustomConfigLocation), systemImage: "folder.badge.plus") {
            store.chooseProfileDestination()
          }
        }
      }

      if let profileID = profileSelectionContext.evidenceID {
        VStack(alignment: .leading, spacing: 6) {
          Text("Evidence Config ID")
            .font(.system(size: 11, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)
          Text(profileID)
            .font(.system(size: 12, design: .monospaced))
            .foregroundStyle(SMColor.primaryText)
            .textSelection(.enabled)
        }
      }
      VStack(alignment: .leading, spacing: 6) {
        Text(store.localizedProfileSelectionContext(using: appChromeLocalization).rawPathLabel)
          .font(.system(size: 11, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
        Text(profileSelectionContext.rawPath ?? appChromeLocalization.text(.setupProfileRawPathEmpty))
          .font(.system(size: 12, design: .monospaced))
          .foregroundStyle(profileSelectionContext.rawPath == nil ? SMColor.secondaryText : SMColor.primaryText)
          .textSelection(.enabled)
          .lineLimit(2)
          .truncationMode(.middle)
          .frame(maxWidth: .infinity, alignment: .leading)
          .padding(10)
          .background(SMColor.input)
          .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
          .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
      }

      if profileSelectionContext.showsSourceIdentityFields {
        HStack(spacing: 14) {
          field("Config ID", text: $store.profileID, placeholder: "profile-local")
          field("Config Name", text: $store.profileName, placeholder: "Local migration config")
        }
      }
      if profileSelectionContext.showsTargetIdentityFields {
        HStack(spacing: 14) {
          field(
            "Target ID", text: $store.targetID, placeholder: "Optional stable target identity")
          field(
            "Target Name", text: $store.targetName,
            placeholder: "Optional display name for target")
        }
      }
    }
  }

  private var inputRow: some View {
    HStack(spacing: 14) {
      field(
        "Session ID", text: $store.sessionID, placeholder: "Required for publish / network push")
      field(
        "Listen Address", text: $store.listenAddress,
        placeholder: "Serve / dashboard / daemon bind address")
    }
  }

  private var reviewInputs: some View {
    VStack(alignment: .leading, spacing: 14) {
      HStack(spacing: 14) {
        field(
          "Persisted Drift IDs", text: $store.driftIDsInput, placeholder: "Comma or space separated"
        )
        field("Reviewer", text: $store.reviewer, placeholder: "Optional reviewer id")
      }
      HStack(spacing: 14) {
        field(
          "Approval ID", text: $store.approvalID,
          placeholder: "Required for prune approve / supersede")
        field(
          "Soft-Delete IDs", text: $store.softDeleteIDsInput,
          placeholder: "Required for prune approve")
      }
      field("Expires At", text: $store.expiresAt, placeholder: "Optional RFC3339 expiry")
    }
  }

  private var reasonInput: some View {
    field(
      "Reason", text: $store.reason,
      placeholder: "Required for drift, reconcile, and prune mutations")
  }

  private var cliTaskPanel: some View {
    ScreenCard(
      title: "Task Dispatch", subtitle: "Full command dispatch lives in its own owner surface."
    ) {
      VStack(alignment: .leading, spacing: 12) {
        let taskAllowed = store.selectedRole.allows(task: store.selectedTask)
        WorkbenchWrappingRow(spacing: 10, rowSpacing: 10) {
          EvidenceChip(label: "selected", value: store.selectedTask.displayTitle, tint: taskAllowed ? SMColor.blue : SMColor.amber)
          EvidenceChip(label: "category", value: store.selectedTask.category, tint: SMColor.secondaryText)
        }
        Text(store.selectedTask.summary)
          .font(.system(size: 13))
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
        WorkbenchWrappingRow(spacing: 12, rowSpacing: 12) {
          PrimaryActionButton("Open Task Dispatch", systemImage: "terminal.fill") {
            showTaskDispatch()
          }
        }
      }
    }
  }

  private var supervisedProcessGrid: some View {
    LazyVGrid(columns: [GridItem(.adaptive(minimum: 240), spacing: 12)], spacing: 12) {
      ForEach(SupervisedProcessSlot.allCases) { slot in
        processSlotCard(slot)
      }
    }
  }

  private func processSlotCard(_ slot: SupervisedProcessSlot) -> some View {
    let run = store.run(in: slot)
    let state = store.supervisionStateLabel(for: slot)
    return VStack(alignment: .leading, spacing: 9) {
      HStack {
        StatusDot(
          color: store.isStaleRunning(slot)
            ? SMColor.amber : (store.isRunning(slot) ? SMColor.green : SMColor.secondaryText))
        Text(slot.title)
          .font(.system(size: 14, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        Spacer()
        Text(state)
          .font(.system(size: 11, weight: .medium))
          .foregroundStyle(tint(for: state))
      }
      Text(slot.summary)
        .font(.system(size: 11))
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
      if let run {
        Text(run.kind.displayTitle)
          .font(.system(size: 11, design: .monospaced))
          .foregroundStyle(SMColor.secondaryText)
          .lineLimit(1)
      }
      if store.isProcessAlive(slot) {
        ActionButton(slot.stopLabel, systemImage: "stop.fill") {
          store.stopProcess(in: slot)
        }
      }
    }
    .padding(13)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(store.focusedProcessSlot == slot ? SMColor.cardElevated : SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(
      RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(
        store.focusedProcessSlot == slot ? SMColor.blue.opacity(0.6) : SMColor.hairline)
    )
    .contentShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .onTapGesture {
      store.focusedProcessSlot = slot
    }
  }

  private var processLifecycleLog: some View {
    VStack(alignment: .leading, spacing: 8) {
      Text("Lifecycle Log")
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      if store.processEvents.isEmpty {
        Text("No process lifecycle events yet.")
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
      } else {
        ForEach(store.processEvents.prefix(6)) { event in
          HStack(alignment: .top, spacing: 8) {
            Text(event.occurredAt, style: .time)
              .font(.system(size: 11, design: .monospaced))
              .foregroundStyle(SMColor.secondaryText)
            Text(event.slot.title)
              .font(.system(size: 11, weight: .semibold))
              .foregroundStyle(SMColor.primaryText)
            Text(event.message)
              .font(.system(size: 11))
              .foregroundStyle(SMColor.secondaryText)
              .fixedSize(horizontal: false, vertical: true)
          }
        }
      }
    }
  }


  private func endpointStatusPane(
    roleLabel: String, title: String, subtitle: String, detail: String, systemImage: String,
    tint: Color, progress: Double
  ) -> some View {
    VStack(alignment: .leading, spacing: 14) {
      HStack(alignment: .top, spacing: 10) {
        VStack(alignment: .leading, spacing: 5) {
          Text(roleLabel.uppercased())
            .font(.system(size: 10, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)
          Text(title)
            .font(.system(size: 18, weight: .bold))
            .foregroundStyle(SMColor.primaryText)
            .lineLimit(2)
            .minimumScaleFactor(0.85)
          Text(subtitle)
            .font(.system(size: 12))
            .foregroundStyle(SMColor.secondaryText)
            .lineLimit(2)
        }
        Spacer()
        StatusDot(color: tint)
      }

      ZStack {
        RoundedRectangle(cornerRadius: 18, style: .continuous)
          .fill(
            LinearGradient(
              colors: [SMColor.input.opacity(0.95), SMColor.card.opacity(0.85)],
              startPoint: .topLeading,
              endPoint: .bottomTrailing
            )
          )
        Circle()
          .fill(tint.opacity(0.15))
          .frame(width: 108, height: 108)
          .blur(radius: 14)
        Image(systemName: systemImage)
          .font(.system(size: 52, weight: .regular))
          .foregroundStyle(tint)
      }
      .frame(height: 122)

      Text(detail)
        .font(.system(size: 11, design: .monospaced))
        .foregroundStyle(SMColor.secondaryText)
        .lineLimit(3)
        .textSelection(.enabled)

      ProgressRail(progress: progress, tint: tint)
    }
    .frame(maxWidth: .infinity, alignment: .topLeading)
  }

  private func deviceCard(title: String, subtitle: String, tint: Color, progress: Double)
    -> some View
  {
    VStack(alignment: .leading, spacing: 18) {
      HStack(alignment: .top) {
        VStack(alignment: .leading, spacing: 5) {
          Text(title)
            .font(.system(size: 18, weight: .bold))
            .foregroundStyle(SMColor.primaryText)
          Text(subtitle)
            .font(.system(size: 12))
            .foregroundStyle(SMColor.secondaryText)
            .lineLimit(2)
        }
        Spacer()
        StatusDot(color: tint)
      }
      ProgressRail(progress: progress, tint: tint)
    }
    .padding(20)
    .frame(width: 300, height: 160, alignment: .topLeading)
    .background(SMColor.cardElevated)
    .clipShape(RoundedRectangle(cornerRadius: 22, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 22, style: .continuous).stroke(SMColor.hairline))
  }

  private func deviceDetailCard(title: String, value: String, detail: String, tint: Color)
    -> some View
  {
    VStack(alignment: .leading, spacing: 14) {
      HStack {
        StatusDot(color: tint)
        Text(title)
          .font(.system(size: 13, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
      }
      Text(value)
        .font(.system(size: 20, weight: .bold))
        .foregroundStyle(SMColor.primaryText)
      Text(detail.isEmpty ? "-" : detail)
        .font(.system(size: 12, design: .monospaced))
        .foregroundStyle(SMColor.secondaryText)
        .lineLimit(3)
        .textSelection(.enabled)
    }
    .padding(18)
    .frame(maxWidth: .infinity, minHeight: 160, alignment: .topLeading)
    .background(SMColor.cardElevated)
    .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 20, style: .continuous).stroke(SMColor.hairline))
  }

  private func metricTile(_ label: String, value: String, tint: Color) -> some View {
    VStack(alignment: .leading, spacing: 6) {
      Text(value)
        .font(.system(size: 22, weight: .bold))
        .foregroundStyle(tint)
        .lineLimit(1)
        .minimumScaleFactor(0.72)
        .monospacedDigit()
      Text(label)
        .font(.system(size: 12))
        .foregroundStyle(SMColor.secondaryText)
    }
    .panelSurface(.metricTile, minHeight: 88)
  }

  private func sectionLabel(_ title: String) -> some View {
    Text(title)
      .font(.system(size: 11, weight: .semibold))
      .foregroundStyle(SMColor.secondaryText)
  }

  private func gateRow(_ label: String, state: GateState) -> some View {
    HStack(spacing: 10) {
      StatusDot(color: state.color)
      Text(label)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(SMColor.primaryText)
      Spacer()
      Text(state.title)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
    }
    .panelSurface(.gateRow)
  }

  private func stageRail(activeIndex: Int) -> some View {
    let stages = ["Prepare", "Connect", "Move", "Verify & Repair", "Evidence"]
    return VStack(alignment: .leading, spacing: 14) {
      HStack(alignment: .firstTextBaseline) {
        Text("Migration Stage")
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
        Spacer()
        Text(controlRoomStageSummary(activeIndex: activeIndex))
          .font(.system(size: 11))
          .foregroundStyle(SMColor.secondaryText)
      }
      HStack(spacing: 0) {
        ForEach(Array(stages.enumerated()), id: \.offset) { index, stage in
          VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 0) {
              if index > 0 {
                Rectangle()
                  .fill(index <= activeIndex ? SMColor.green : SMColor.hairline)
                  .frame(height: 1)
              }
              ZStack {
                Circle()
                  .fill(
                    index <= activeIndex
                      ? (index == activeIndex ? SMColor.blue : SMColor.green) : SMColor.card)
                Circle()
                  .stroke(
                    index <= activeIndex
                      ? (index == activeIndex ? SMColor.blue : SMColor.green) : SMColor.hairline,
                    lineWidth: 1.5
                  )
                if index < activeIndex {
                  Image(systemName: "checkmark")
                    .font(.system(size: 8, weight: .bold))
                    .foregroundStyle(SMColor.inverseText)
                } else if index == activeIndex {
                  Circle()
                    .fill(SMColor.inverseText)
                    .frame(width: 4, height: 4)
                }
              }
              .frame(width: 18, height: 18)
              if index < stages.count - 1 {
                Rectangle()
                  .fill(index < activeIndex ? SMColor.green : SMColor.hairline)
                  .frame(height: 1)
              }
            }
            VStack(alignment: .leading, spacing: 2) {
              Text(stage)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(
                  index <= activeIndex ? SMColor.primaryText : SMColor.secondaryText)
              Text(controlRoomStageStatus(index: index, activeIndex: activeIndex))
                .font(.system(size: 10))
                .foregroundStyle(index == activeIndex ? SMColor.blue : SMColor.secondaryText)
            }
          }
          .frame(maxWidth: .infinity)
        }
      }
    }
    .panelSurface(.runway)
  }

  private func actionCard(title: String, subtitle: String, task: SuperMoverTaskKind, primary: Bool)
    -> some View
  {
    VStack(alignment: .leading, spacing: 14) {
      Text(title)
        .font(.system(size: 17, weight: .bold))
        .foregroundStyle(SMColor.primaryText)
      Text(subtitle)
        .font(.system(size: 12))
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
      Spacer()
      if primary {
        PrimaryActionButton("Run", systemImage: "play.fill") { run(task) }
      } else {
        ActionButton("Run", systemImage: "play.fill") { run(task) }
      }
    }
    .padding(18)
    .frame(maxWidth: .infinity, minHeight: 170, alignment: .leading)
    .background(SMColor.cardElevated)
    .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 20, style: .continuous).stroke(SMColor.hairline))
  }

  private func compactRoleButton(_ role: WorkbenchRole) -> some View {
    Button {
      store.selectedRole = role
    } label: {
      HStack(spacing: 8) {
        StatusDot(color: store.selectedRole == role ? SMColor.blue : SMColor.secondaryText)
        Text(role.localizedTitle(using: appChromeLocalization))
          .font(.system(size: 13, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
          .lineLimit(1)
      }
      .padding(.horizontal, 12)
      .padding(.vertical, 10)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(store.selectedRole == role ? SMColor.cardElevated : SMColor.input)
      .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
      .overlay(
        RoundedRectangle(cornerRadius: 20, style: .continuous)
          .stroke(store.selectedRole == role ? SMColor.blue.opacity(0.7) : SMColor.hairline)
      )
    }
    .buttonStyle(.plain)
  }

  private var setupReadiness: some View {
    let guide = store.localizedSetupGuide(using: appChromeLocalization)
    return VStack(alignment: .leading, spacing: 10) {
      Text(appChromeLocalization.text(.setupChecklistTitle))
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      LazyVGrid(columns: [GridItem(.adaptive(minimum: 220), spacing: 10)], spacing: 10) {
        gateRow(appChromeLocalization.text(.setupChecklistConfigFile), state: guide.steps[0].state)
        gateRow(guide.steps[1].title, state: guide.steps[1].state)
        gateRow(appChromeLocalization.text(.setupChecklistLintLatestConfig), state: guide.steps[2].state)
      }
      Text(
        appChromeLocalization.text(.setupChecklistDetail)
      )
      .font(.system(size: 12))
      .foregroundStyle(SMColor.secondaryText)
      .fixedSize(horizontal: false, vertical: true)
    }
  }

  private func field(_ label: String, text: Binding<String>, placeholder: String) -> some View {
    VStack(alignment: .leading, spacing: 8) {
      Text(label)
        .font(.system(size: 12, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      TextField(placeholder, text: text)
        .textFieldStyle(.plain)
        .font(.system(size: 13))
        .padding(10)
        .background(SMColor.input)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private func pathField(
    _ label: String, text: Binding<String>, placeholder: String, readiness: String,
    browseTitle: String,
    browse: @escaping () -> Void
  ) -> some View {
    let hasPath = !text.wrappedValue.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    return VStack(alignment: .leading, spacing: 8) {
      HStack {
        Text(label)
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
        Spacer()
        if hasPath {
          EvidenceChip(label: "access", value: localizedDirectoryReadiness(readiness), tint: tint(for: readiness))
        }
      }
      HStack(spacing: 10) {
        TextField(placeholder, text: text)
          .textFieldStyle(.plain)
          .font(.system(size: 13))
          .padding(10)
          .background(SMColor.input)
          .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
          .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
        CompactActionButton(browseTitle, systemImage: "folder", action: browse)
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private func localizedDirectoryReadiness(_ readiness: String) -> String {
    switch readiness {
    case "readable":
      return appChromeLocalization.text(.setupDirectoryReadable)
    case "not readable":
      return appChromeLocalization.text(.setupDirectoryNotReadable)
    case "writable":
      return appChromeLocalization.text(.setupDirectoryWritable)
    case "not writable":
      return appChromeLocalization.text(.setupDirectoryNotWritable)
    default:
      return appChromeLocalization.text(.setupDirectoryNotSelected)
    }
  }

  private func commandPreview(for task: SuperMoverTaskKind) -> some View {
    let args = store.commandPreviewArguments(for: task)
    return VStack(alignment: .leading, spacing: 6) {
      Text("CLI Preview")
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      ScrollView(.horizontal, showsIndicators: false) {
        Text(shellPreview(["supermover"] + args))
          .font(.system(size: 12, design: .monospaced))
          .foregroundStyle(SMColor.primaryText)
          .textSelection(.enabled)
      }
      .padding(10)
      .background(SMColor.input)
      .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
      .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
    }
  }

  private func taskDispatchBadge(label: String, value: String, tint: Color) -> some View {
    HStack(spacing: 7) {
      StatusDot(color: tint)
      Text(label)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
        .lineLimit(1)
      Text(value)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(1)
        .minimumScaleFactor(0.78)
    }
    .padding(.horizontal, 10)
    .padding(.vertical, 7)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(tint.opacity(0.08))
    .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
  }

  private func taskDispatchIcon(for task: SuperMoverTaskKind) -> String {
    switch task.taskCategory {
    case .all:
      return "terminal"
    case .runtime:
      return "info.circle"
    case .profile:
      return "person.crop.circle.badge.checkmark"
    case .local:
      return "externaldrive.fill"
    case .review:
      return "checklist"
    case .pairing:
      return "link"
    case .network:
      return "network"
    case .sync:
      return "arrow.triangle.2.circlepath"
    case .foreground:
      return "play.rectangle.fill"
    }
  }

  private func taskDispatchInputSummary(for task: SuperMoverTaskKind) -> String {
    var inputs: [String] = []
    if task.requiresProfile {
      inputs.append(profileRequirementLabel(for: task))
    }
    switch task {
    case .profileInit:
      inputs.append("source folder")
      inputs.append("config identity")
    case .profileSetTarget:
      inputs.append("target destination root")
    case .publish, .networkPush, .syncRun, .syncNetworkRun, .syncNetworkDiscoverRun:
      inputs.append("session id")
    case .syncLoop, .syncWatch, .syncNetworkLoop:
      inputs.append("session prefix")
    case .syncQueueCancel, .syncQueueFail:
      inputs.append("queue entry id")
      inputs.append("reason")
    case .discoverAddress:
      inputs.append("target address hint")
    case .pair:
      inputs.append("target address")
      inputs.append("verification code")
    case .profileAdoptPairing:
      inputs.append("pairing receipt file")
    case .driftAcknowledge, .driftResolve, .driftExpire:
      inputs.append("one persisted drift id")
      inputs.append("reason")
    case .driftRecord, .reconcileApply:
      inputs.append("reason")
    case .pruneApprove:
      inputs.append("approval id")
      inputs.append("soft-delete ids")
      inputs.append("reviewer")
    case .pruneSupersede:
      inputs.append("approval id")
      inputs.append("reason")
    case .daemonRestart, .daemonStop:
      inputs.append("reason")
    default:
      break
    }
    let inputText = inputs.isEmpty ? "No additional app inputs are required for this task." : "Inputs: \(inputs.joined(separator: ", "))."
    return "\(inputText) Execution still runs through the existing command preflight and role gate."
  }

  private func profileRequirementSatisfied(for task: SuperMoverTaskKind) -> Bool {
    guard task.requiresProfile else {
      return true
    }
    if task == .profileInit {
      return profileSelectionState == .newDestination
    }
    return profilePathIsSet
  }

  private func profileRequirementLabel(for task: SuperMoverTaskKind) -> String {
    if task == .profileInit {
      return profileSelectionState == .newDestination ? "new config destination" : "config destination required"
    }
    return profilePathIsSet ? "existing config" : "existing config required"
  }

  private func shellPreview(_ parts: [String]) -> String {
    parts.map { part in
      let safeScalars = CharacterSet(
        charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_./:-=@%+,")
      if !part.isEmpty, part.unicodeScalars.allSatisfy({ safeScalars.contains($0) }) {
        return part
      }
      return "'" + part.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }
    .joined(separator: " ")
  }

  private func outputPane(title: String, text: String) -> some View {
    VStack(alignment: .leading, spacing: 8) {
      Text(title)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      ScrollView {
        Text(text.isEmpty ? "No output yet." : text)
          .font(.system(size: 12, design: .monospaced))
          .foregroundStyle(SMColor.primaryText)
          .frame(maxWidth: .infinity, alignment: .leading)
          .textSelection(.enabled)
      }
      .frame(minHeight: 110, maxHeight: 190)
      .padding(12)
      .background(SMColor.input)
      .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
      .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
    }
  }

  private func evidenceLine(_ label: String, _ value: String) -> some View {
    HStack(alignment: .firstTextBaseline, spacing: 10) {
      Text(label)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
        .frame(width: 110, alignment: .leading)
      Text(value)
        .font(.system(size: 12, design: .monospaced))
        .foregroundStyle(SMColor.primaryText)
        .textSelection(.enabled)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private func run(_ task: SuperMoverTaskKind) {
    store.selectedTask = task
    store.runSelectedTask()
  }

  private func hasSuccessfulRecent(_ kind: SuperMoverTaskKind) -> Bool {
    store.hasSuccessfulRun(kind)
  }

  private func tint(for value: String) -> Color {
    let normalized = value.lowercased()
    if normalized.contains("error") || normalized.contains("failed")
      || normalized.contains("review") || normalized.contains("drift")
      || normalized.contains("warning") || normalized.contains("attention")
      || normalized.contains("pending") || normalized.contains("missing")
      || normalized.contains("invalid") || normalized.contains("mismatch")
      || normalized.contains("unavailable") || normalized.contains("unpaired")
      || normalized.contains("unhealthy") || normalized.contains("artifact")
      || normalized.contains("recovery") || normalized.contains("not selected")
      || normalized.contains("not writable") || normalized.contains("not readable")
      || normalized.contains("blocked") || normalized.contains("stale")
      || normalized.contains("not checked")
    {
      return SMColor.amber
    }
    if normalized.contains("clean") || normalized.contains("healthy")
      || normalized.contains("aligned") || normalized.contains("verified")
      || normalized.contains("paired") || normalized.contains("complete")
      || normalized.contains("published") || normalized.contains("readable")
      || normalized.contains("writable") || normalized.contains("ok")
      || normalized.contains("exit 0") || normalized.contains("running")
    {
      return SMColor.green
    }
    return SMColor.blue
  }

  private func discoveryClassTint(_ value: String) -> Color {
    switch value.lowercased() {
    case "unique", "explicit":
      return SMColor.blue
    default:
      return SMColor.amber
    }
  }

  private var hasSelectedProfilePath: Bool {
    store.isProfileSelected
  }

  private var profilePathIsSet: Bool {
    profileSelectionState == .existingFile
  }

  private var profileSelectionState: SelectedProfilePathState {
    profileSelectionContext.pathState
  }

  private var profileSelectionContext: ProfileSelectionContext {
    store.profileSelectionContext
  }

  private var profileStatus: String {
    switch profileSelectionState {
    case .existingFile:
      return "selected"
    case .newDestination:
      return "create pending"
    case .missingFile, .directory:
      return "review"
    case .none:
      return "required"
    }
  }

  private var sourceRootReadiness: String {
    if store.selectedRole != .source {
      return "not required"
    }
    return store.directoryReadiness(path: store.sourceRootPath, requiresWrite: false)
  }

  private var targetRootReadiness: String {
    if store.selectedRole == .observer {
      return "not required"
    }
    return store.directoryReadiness(path: store.targetRootPath, requiresWrite: true)
  }

  private var sourceRootGateState: GateState {
    if store.selectedRole != .source {
      return .neutral
    }
    return sourceRootReadiness == "readable" ? .pass : .pending
  }

  private var targetRootGateState: GateState {
    if store.selectedRole == .observer {
      return .neutral
    }
    return targetRootReadiness == "writable" ? .pass : .pending
  }

  private var pairingEvidenceState: GateState {
    let normalized = pairingStatus.lowercased()
    if normalized == "not checked" {
      return .pending
    }
    if normalized == "paired_receipt_valid" || normalized == "paired" {
      return .pass
    }
    return .review
  }

  private var transferRunwayState: GateState {
    if hasSuccessfulRecent(.networkPush) || hasSuccessfulRecent(.publish) {
      return .pass
    }
    if store.activeRun?.kind == .networkPush || store.activeRun?.kind == .publish {
      return .review
    }
    if hasSuccessfulRecent(.networkDryRun) || hasSuccessfulRecent(.dryRun) {
      return .review
    }
    return .pending
  }

  private var syncRunwayState: GateState {
    if store.isStaleRunning(.sourceSyncLoop) || store.isStaleRunning(.sourceSyncWatch)
      || store.isStaleRunning(.sourceNetworkLoop)
    {
      return .review
    }
    if store.isRunning(.sourceSyncLoop) || store.isRunning(.sourceSyncWatch)
      || store.isRunning(.sourceNetworkLoop)
    {
      return .pass
    }
    if hasSuccessfulRecent(.syncRun) || hasSuccessfulRecent(.syncNetworkRun)
      || hasSuccessfulRecent(.syncNetworkDiscoverRun)
    {
      return .pass
    }
    if store.syncQueueSnapshot != nil || store.syncLoopSnapshot != nil
      || store.syncWatchSnapshot != nil || store.syncNetworkLoopSnapshot != nil
    {
      return .review
    }
    return .pending
  }

  private var verificationRunwayState: GateState {
    evidenceGateEvaluation.verificationState.gateState
  }

  private var targetPreflightGateState: GateState {
    evidenceGateEvaluation.targetPreflightState.gateState
  }

  private var aggregateEvidenceGateState: GateState {
    evidenceGateEvaluation.aggregateEvidenceState.gateState
  }

  private var serveRunwayState: GateState {
    if store.isStaleRunning(.targetServe) {
      return .review
    }
    if store.isRunning(.targetServe) {
      return .pass
    }
    return .pending
  }

  private var dashboardRunwayState: GateState {
    if store.isStaleRunning(.targetDashboard) {
      return .review
    }
    if store.isRunning(.targetDashboard) {
      return .pass
    }
    return .pending
  }

  private var installReadinessState: GateState {
    switch store.cliProvenance.readinessLevel {
    case .pass:
      return .pass
    case .review:
      return .review
    case .blocked:
      return .blocked
    }
  }

  private var evidenceRunwayState: GateState {
    aggregateEvidenceGateState
  }

  private var setupPreviewTask: SuperMoverTaskKind {
    switch store.selectedRole {
    case .source:
      return .profileInit
    case .target:
      return .profileSetTarget
    case .observer:
      return .lintProfile
    }
  }

  private var pairingStatus: String {
    store.statusSnapshot?.pairing.status ?? store.reportSnapshot?.pairing.status ?? "not checked"
  }

  private var networkEvidenceState: NetworkEvidenceSurfaceState {
    NetworkEvidenceSurfaceState(
      status: store.statusSnapshot?.network.status,
      hasReportTransfers: !(store.reportSnapshot?.network_transfers?.isEmpty ?? true)
    )
  }

  private var networkStatus: String {
    networkEvidenceState.label
  }

  private var latestPairSummary: String? {
    store.recentRuns
      .first(where: { run in
        guard run.kind == .pair else { return false }
        if case .finished(0) = run.state {
          return store.isCurrentContext(run)
        }
        return false
      })
      .flatMap { PairInfoParser.firstSummary(in: $0.stdout) }
  }

  private var activeRunTitle: String {
    store.activeRun == nil ? "Run Console" : "Active Run"
  }

  private var transferPercent: String {
    if store.activeRun?.kind == .networkPush {
      return "Task"
    }
    if let status = store.statusSnapshot {
      return status.latest_session.completeness_status
    }
    if let report = store.reportSnapshot {
      return report.latest_session.completeness.status
    }
    return "Idle"
  }

  private var transferCaption: String {
    store.activeRun?.kind == .networkPush ? "foreground process running" : "evidence-backed state"
  }

  private var filesVerifiedValue: String {
    if let verify = store.verifySnapshot {
      return "\(verify.summary.files_verified) / \(verify.summary.files_expected)"
    }
    if let status = store.statusSnapshot {
      return "\(status.latest_session.files_verified) / \(status.latest_session.files_expected)"
    }
    if let report = store.reportSnapshot {
      return
        "\(report.latest_session.completeness.files_verified) / \(report.latest_session.completeness.files_expected)"
    }
    return "not checked"
  }

  private var networkTransferValue: String {
    if let status = store.statusSnapshot {
      return "\(status.counts.network_transfers)"
    }
    if let report = store.reportSnapshot {
      return "\(report.summary.network_transfers)"
    }
    return store.activeRun?.kind == .networkPush ? "foreground" : "not checked"
  }

  private var integrityValue: String {
    if let verify = store.verifySnapshot {
      return verify.statusLabel
    }
    if let status = store.statusSnapshot {
      return status.overall.status
    }
    if let report = store.reportSnapshot {
      return report.overall.status
    }
    return "not checked"
  }

  private var warningMetricValue: String {
    countMetricValue(warningCountEvidence)
  }

  private var artifactProblemMetricValue: String {
    countMetricValue(artifactProblemCountEvidence)
  }

  private var verifiedFilesMetricValue: String {
    guard let verify = store.verifySnapshot else {
      return "not checked"
    }
    return "\(verify.summary.files_verified)/\(verify.summary.files_expected)"
  }

  private var verificationFilesTint: Color {
    guard let verify = store.verifySnapshot else {
      return SMColor.amber
    }
    if verify.summary.files_expected == 0 {
      return SMColor.amber
    }
    return verify.summary.files_verified == verify.summary.files_expected
      ? SMColor.green : SMColor.amber
  }

  private var verificationProgress: Double {
    if let verify = store.verifySnapshot,
      verify.summary.files_expected > 0
    {
      return Double(verify.summary.files_verified) / Double(verify.summary.files_expected)
    }
    if let status = store.statusSnapshot,
      status.latest_session.files_expected > 0
    {
      return Double(status.latest_session.files_verified)
        / Double(status.latest_session.files_expected)
    }
    if let report = store.reportSnapshot,
      report.latest_session.completeness.files_expected > 0
    {
      return Double(report.latest_session.completeness.files_verified)
        / Double(report.latest_session.completeness.files_expected)
    }
    return 0
  }

  private var targetSubtitle: String {
    if let status = store.statusSnapshot {
      return status.overall.target_status
    }
    if let report = store.reportSnapshot {
      return report.overall.status
    }
    return "Target evidence not loaded"
  }

  private var sourceTitle: String {
    if let profileID = store.statusSnapshot?.profile_id {
      return profileID
    }
    if let profileID = store.reportSnapshot?.profile_id {
      return profileID
    }
    return "Source Config"
  }

  private var sourceSubtitle: String {
    store.selectedProfileDisplayTitle
  }

  private var targetTitle: String {
    if let targetID = store.statusSnapshot?.target_id {
      return targetID
    }
    if let targetID = store.reportSnapshot?.target_id {
      return targetID
    }
    return "Target Evidence"
  }

  private var sourceEndpointDetail: String {
    "pairing \(pairingStatus) • network \(networkStatus)"
  }

  private var targetEndpointDetail: String {
    let root = statusTargetRoot == "-" ? "target root unknown" : statusTargetRoot
    return "\(root) • integrity \(integrityValue)"
  }

  private var statusTargetRoot: String {
    store.verifySnapshot?.target_root ?? store.statusSnapshot?.target_root ?? store.reportSnapshot?
      .target_root ?? store.healthSnapshot?.target_root ?? "-"
  }

  private var verificationStatus: String {
    if let verify = store.verifySnapshot {
      return verify.statusLabel
    }
    return store.statusSnapshot?.overall.status ?? store.reportSnapshot?.overall.status
      ?? "not checked"
  }

  private var statusHasProblems: Bool {
    evidenceHasReviewProblems
  }

  private var evidenceGateEvaluation: EvidenceGateEvaluation {
    EvidenceGateEvaluation(
      hasStatusEvidence: store.statusSnapshot != nil,
      statusNeedsReview: store.statusSnapshot.map(statusIssuesNeedReview) ?? false,
      hasReportEvidence: store.reportSnapshot != nil,
      reportNeedsReview: store.reportSnapshot.map(reportIssuesNeedReview) ?? false,
      hasHealthEvidence: store.healthSnapshot != nil,
      healthNeedsReview: store.healthSnapshot.map(healthIssuesNeedReview) ?? false,
      hasVerifyEvidence: store.verifySnapshot != nil,
      verifyNeedsReview: store.verifySnapshot?.reviewRequired ?? false
    )
  }

  private var warningCountEvidence: Int? {
    maxEvidenceCount([
      store.verifySnapshot.map { $0.summary.warnings + $0.summary.warning_findings },
      store.statusSnapshot?.counts.warnings,
      store.reportSnapshot?.summary.warnings,
    ])
  }

  private var artifactProblemCountEvidence: Int? {
    maxEvidenceCount([
      store.verifySnapshot?.summary.artifact_problems,
      store.statusSnapshot?.counts.artifact_problems,
      store.reportSnapshot?.summary.artifact_problems,
      store.healthSnapshot?.summary.artifact_problems,
    ])
  }

  private var warningGateState: GateState {
    guard let count = warningCountEvidence else {
      return .pending
    }
    return count == 0 ? .pass : .review
  }

  private var evidenceHasReviewProblems: Bool {
    if let verify = store.verifySnapshot, verify.reviewRequired {
      return true
    }
    if let status = store.statusSnapshot, statusIssuesNeedReview(status) {
      return true
    }
    if let report = store.reportSnapshot, reportIssuesNeedReview(report) {
      return true
    }
    if let health = store.healthSnapshot, healthIssuesNeedReview(health) {
      return true
    }
    return false
  }

  private var hasLoadedEvidence: Bool {
    evidenceGateEvaluation.hasAnyEvidence
  }

  private func countMetricValue(_ count: Int?) -> String {
    guard let count else {
      return "not checked"
    }
    return "\(count)"
  }

  private func countMetricTint(_ count: Int?) -> Color {
    guard let count else {
      return SMColor.amber
    }
    return count == 0 ? SMColor.green : SMColor.amber
  }

  private func maxEvidenceCount(_ counts: [Int?]) -> Int? {
    let available = counts.compactMap { $0 }
    return available.max()
  }

  private func statusIssuesNeedReview(_ status: StatusSnapshot) -> Bool {
    if statusTextNeedsReview(status.overall.status)
      || statusTextNeedsReview(status.overall.target_status)
    {
      return true
    }
    if let issues = status.issues, !issues.isEmpty {
      return true
    }
    if status.latest_session.verification_errors > 0
      || status.latest_session.verification_warnings > 0
    {
      return true
    }
    if statusTextNeedsReview(status.latest_session.completeness_status) {
      return true
    }
    if status.counts.warnings > 0 || status.counts.target_drifts > 0
      || status.counts.live_target_drifts > 0
      || status.counts.live_target_drift_artifact_problems > 0
      || status.counts.prune_unapplied_approvals > 0 || status.counts.prune_stale_approvals > 0
      || status.counts.prune_expired_approvals > 0 || status.counts.prune_receipt_issues > 0
      || status.counts.recovery_issues > 0 || status.counts.artifact_problems > 0
      || status.network.artifact_problems > 0
    {
      return true
    }
    return false
  }

  private func reportIssuesNeedReview(_ report: ReportSnapshot) -> Bool {
    if statusTextNeedsReview(report.overall.status) {
      return true
    }
    if let issues = report.overall.issues, !issues.isEmpty {
      return true
    }
    if report.latest_session.completeness.verification_errors > 0
      || report.latest_session.completeness.verification_warnings > 0
      || statusTextNeedsReview(report.latest_session.completeness.status)
    {
      return true
    }
    if statusTextNeedsReview(report.prune_review.status) || report.prune_review.approval_required
      || report.prune_review.summary.candidates > 0 || report.prune_review.summary.refusals > 0
      || report.prune_review.summary.unapplied_approvals > 0
      || report.prune_review.summary.receipt_issues > 0
    {
      return true
    }
    if report.summary.warnings > 0 || report.summary.target_drifts > 0
      || report.summary.live_target_drifts > 0 || report.summary.artifact_problems > 0
      || report.health.summary.incomplete_sessions > 0 || report.health.summary.invalid_records > 0
      || report.health.summary.artifact_problems > 0 || report.health.summary.target_drifts > 0
      || !report.health.healthy
    {
      return true
    }
    return false
  }

  private func healthIssuesNeedReview(_ health: HealthSnapshot) -> Bool {
    if !health.healthy {
      return true
    }
    return health.summary.incomplete_sessions > 0 || health.summary.invalid_records > 0
      || health.summary.artifact_problems > 0 || health.summary.target_drifts > 0
  }

  private func statusTextNeedsReview(_ value: String) -> Bool {
    let normalized = value.lowercased()
    return normalized.contains("error") || normalized.contains("failed")
      || normalized.contains("review") || normalized.contains("drift")
      || normalized.contains("warning") || normalized.contains("attention")
      || normalized.contains("pending") || normalized.contains("missing")
      || normalized.contains("invalid") || normalized.contains("mismatch")
      || normalized.contains("unavailable") || normalized.contains("unpaired")
      || normalized.contains("unhealthy") || normalized.contains("artifact")
      || normalized.contains("recovery") || normalized.contains("blocked")
      || normalized.contains("stale") || normalized.contains("incomplete")
  }

  private var activeStageIndex: Int {
    if store.verifySnapshot != nil || hasSuccessfulRecent(.verify)
      || hasSuccessfulRecent(.reconcileApply) || hasSuccessfulRecent(.pruneApprove)
    {
      return 3
    }
    if hasSuccessfulRecent(.networkPush) || hasSuccessfulRecent(.publish)
      || hasSuccessfulRecent(.networkDryRun) || hasSuccessfulRecent(.dryRun)
      || hasSuccessfulRecent(.syncRun) || hasSuccessfulRecent(.syncNetworkRun)
      || hasSuccessfulRecent(.syncNetworkDiscoverRun)
    {
      return 2
    }
    if profilePathIsSet || pairingEvidenceState != .pending || networkEvidenceState != .notChecked {
      return 1
    }
    return 0
  }

  private var transferSectionModel: TransferSectionModel {
    let recentTransferRun = store.recentRuns.first(where: { run in
      (run.kind == .networkPush || run.kind == .networkDryRun || run.kind == .publish)
        && store.isCurrentContext(run)
    })

    return TransferSectionModel(
      title: "Transfer",
      subtitle:
        "Run bounded config-backed network transfer. Dry-run first keeps target mutation explicit.",
      headerBadge: TransferStatusBadge(
        label: transferRunwayState.title.capitalized,
        systemImage: transferStatusSymbol(for: transferRunwayState),
        tint: transferRunwayState.color
      ),
      lastUpdatedLabel: transferLastUpdatedLabel,
      headerNote: store.selectedRole == .source
        ? "Foreground runs are supervised here; durable completion still depends on evidence."
        : "Read-only transfer evidence for this role.",
      route: TransferRouteModel(
        summaryLine: "pairing \(pairingStatus) • network \(networkStatus) • integrity \(integrityValue)",
        source: TransferEndpointModel(
          name: sourceTitle,
          address: sourceSubtitle,
          symbolName: "laptopcomputer",
          statusTint: pairingEvidenceState.color,
          metadata: [
            .init(
              id: "source-role",
              value: "\(appChromeLocalization.text(.sidebarRoleLabel)) • \(store.selectedRole.localizedTitle(using: appChromeLocalization))",
              emphasized: true),
            .init(id: "source-detail", value: sourceEndpointDetail)
          ]
        ),
        target: TransferEndpointModel(
          name: targetTitle,
          address: statusTargetRoot == "-" ? "target root unknown" : statusTargetRoot,
          symbolName: "externaldrive.connected.to.line.below",
          statusTint: targetPreflightGateState.color,
          metadata: [
            .init(id: "target-summary", value: targetSubtitle, emphasized: true),
            .init(id: "target-detail", value: targetEndpointDetail)
          ]
        )
      ),
      overview: TransferOverviewModel(
        progress: transferOverviewProgress,
        progressLabel: transferOverviewLabel,
        stateLabel: store.activeRun?.kind == .networkPush ? "Foreground" : transferRunwayState.title.capitalized,
        detail: transferCaption,
        tint: transferOverviewTint,
        highlights: [
          .init(id: "pairing", label: "pairing", value: pairingStatus, tint: pairingEvidenceState.color),
          .init(id: "network", label: "network", value: networkStatus, tint: tint(for: networkStatus)),
          .init(id: "warnings", label: "warnings", value: warningMetricValue, tint: countMetricTint(warningCountEvidence))
        ]
      ),
      metrics: [
        TransferMetricModel(
          id: "files",
          title: "Files verified",
          value: filesVerifiedValue,
          detail: verificationProgress > 0 ? "\(Int((verificationProgress * 100).rounded()))% of expected surface" : "verification pending",
          progress: verificationProgress > 0 ? verificationProgress : nil,
          sparkline: nil,
          tint: verificationFilesTint
        ),
        TransferMetricModel(
          id: "network",
          title: "Network transfers",
          value: networkTransferValue,
          detail: networkStatus,
          progress: nil,
          sparkline: nil,
          tint: SMColor.cyan
        ),
        TransferMetricModel(
          id: "integrity",
          title: "Integrity",
          value: integrityValue,
          detail: verificationStatus,
          progress: nil,
          sparkline: nil,
          tint: tint(for: integrityValue)
        ),
        TransferMetricModel(
          id: "activity",
          title: "Run state",
          value: recentTransferRun.map { stateLabel($0.state) } ?? transferPercent,
          detail: recentTransferRun?.kind.displayTitle ?? transferCaption,
          progress: nil,
          sparkline: transferRunSparkline,
          tint: transferOverviewTint
        )
      ],
      activity: TransferActivityModel(
        subtitle: recentTransferRun.map { "\($0.kind.displayTitle) • \(stateLabel($0.state))" }
          ?? "Latest transfer task and evidence-backed progression.",
        currentFile: .init(
          path: transferCurrentPath,
          progressLabel: transferCurrentProgressLabel,
          progress: transferOverviewProgress,
          tint: transferOverviewTint,
          startedAt: recentTransferRun.map { timeString($0.launchedAt) } ?? "Not started",
          receiptID: transferReceiptID
        ),
        stageSummary: controlRoomStageSummary(activeIndex: activeStageIndex),
        stages: transferStages
      ),
      log: TransferLogModel(
        subtitle: "Recent transfer-relevant task output retained in-app.",
        entries: transferLogEntries,
        footerNote: transferLogEntries.isEmpty ? nil : "Use durable evidence under Evidence for completion claims."
      ),
      inspector: TransferInspectorModel(
        title: "Safety posture",
        subtitle: "Transfer remains config-backed and evidence-gated.",
        actionTitle: nil,
        summaryRows: [
          .init(id: "profile", label: "Config", value: profileStatus, tint: profilePathIsSet ? SMColor.green : SMColor.amber),
          .init(id: "pairing", label: "Pairing", value: pairingStatus, tint: pairingEvidenceState.color),
          .init(id: "preflight", label: "Preflight", value: targetPreflightGateState.title, tint: targetPreflightGateState.color),
          .init(id: "integrity", label: "Integrity", value: integrityValue, tint: tint(for: integrityValue))
        ],
        notes: [
          .init(
            id: "profile-backed",
            title: "Config-backed route",
            detail: "Receiver address and TLS identity stay in migration config SSOT material.",
            style: .neutral
          ),
          .init(
            id: "mutation",
            title: "Target mutation stays explicit",
            detail: "Dry-run and verify remain the clearest preflight and completion evidence.",
            style: targetPreflightGateState == .pass ? .good : .warning
          ),
          .init(
            id: "warnings",
            title: "Warnings remain durable",
            detail: warningCountEvidence == 0
              ? "No warning evidence is currently loaded for this context."
              : "Review warning and artifact evidence before treating transfer as complete.",
            style: warningGateState == .pass ? .good : .warning
          )
        ]
      )
    )
  }

  private var transferPrimaryControl: TransferSectionControl? {
    guard store.selectedRole == .source else {
      return nil
    }
    if store.activeRun?.kind == .networkPush {
      return TransferSectionControl(
        title: "Stop Transfer",
        systemImage: "stop.fill",
        prominence: .primary,
        action: { store.stopActiveTask() }
      )
    }
    return TransferSectionControl(
      title: "Run Network Push",
      systemImage: "play.fill",
      prominence: .primary,
      action: { run(.networkPush) }
    )
  }

  private var transferSupportingModel: TransferSupportingModel {
    if store.selectedRole == .source {
      return TransferSupportingModel(
        gateNotice: nil,
        actionCards: [
          .init(
            id: "network-dry-run",
            title: "Network dry-run",
            subtitle: "Validate config-backed network transfer without contacting receiver.",
            primary: false,
            action: { run(.networkDryRun) }
          ),
          .init(
            id: "network-push",
            title: "Network push",
            subtitle: "Run the current bounded network push path with an explicit session id.",
            primary: true,
            action: { run(.networkPush) }
          ),
        ],
        profileNetworkPanel: AnyView(
          ProfileNetworkPanel(
            draft: $store.profileNetwork,
            role: store.selectedRole,
            networkStatus: networkStatus,
            networkTint: tint(for: networkStatus),
            runTask: run
          )
        ),
        sessionID: $store.sessionID,
        listenAddress: $store.listenAddress
      )
    }

    return TransferSupportingModel(
      gateNotice: .init(
        title: "Source role required",
        detail:
          "Target and observer roles can inspect evidence, serve, and open dashboard surfaces. Bounded transfer execution stays source-owned until a future role-specific workflow says otherwise.",
        state: .blocked
      ),
      actionCards: [],
      profileNetworkPanel: nil,
      sessionID: nil,
      listenAddress: nil
    )
  }

  private var transferOverviewProgress: Double {
    max(verificationProgress, store.activeRun?.kind == .networkPush ? 0.42 : 0)
  }

  private var transferOverviewLabel: String {
    if store.activeRun?.kind == .networkPush {
      return "LIVE"
    }
    if verificationProgress > 0 {
      return "\(Int((verificationProgress * 100).rounded()))%"
    }
    return transferPercent.uppercased()
  }

  private var transferOverviewTint: Color {
    store.activeRun?.kind == .networkPush ? SMColor.blue : transferRunwayState.color
  }

  private var transferLastUpdatedLabel: String? {
    if let run = store.activeRun {
      return "Updated \(timeString(run.launchedAt))"
    }
    if let run = store.recentRuns.first(where: { store.isCurrentContext($0) }) {
      return "Updated \(timeString(run.launchedAt))"
    }
    return hasLoadedEvidence ? "Evidence loaded" : nil
  }

  private var transferCurrentPath: String {
    if let root = store.verifySnapshot?.target_root, !root.isEmpty {
      return root
    }
    if !store.targetRootPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
      return store.targetRootPath
    }
    return "No transfer artifact path loaded."
  }

  private var transferCurrentProgressLabel: String {
    verificationProgress > 0 ? "\(filesVerifiedValue) verified" : transferPercent
  }

  private var transferReceiptID: String {
    let verifySession = store.verifySnapshot?.session_id?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    if !verifySession.isEmpty {
      return verifySession
    }
    let inputSession = store.sessionID.trimmingCharacters(in: .whitespacesAndNewlines)
    return inputSession.isEmpty ? "none" : inputSession
  }

  private var transferStages: [TransferStageModel] {
    let stages = ["Prepare", "Connect", "Move", "Verify & Repair", "Evidence"]
    return stages.enumerated().map { index, title in
      let state: TransferStageModel.State
      if index < activeStageIndex {
        state = .complete
      } else if index == activeStageIndex {
        state = transferRunwayState == .review ? .warning : .current
      } else {
        state = .pending
      }
      return TransferStageModel(
        id: title.lowercased(),
        title: title,
        timeLabel: transferStageTimeLabel(index),
        statusLabel: controlRoomStageStatus(index: index, activeIndex: activeStageIndex),
        state: state
      )
    }
  }

  private var transferLogEntries: [TransferLogModel.Entry] {
    let runs = store.recentRuns.filter { run in
      (run.kind == .networkPush || run.kind == .networkDryRun || run.kind == .publish)
        && store.isCurrentContext(run)
    }
    return Array(runs.prefix(4)).enumerated().map { index, run in
      TransferLogModel.Entry(
        id: "transfer-log-\(index)",
        timestamp: timeString(run.launchedAt),
        message: transferLogMessage(for: run),
        tint: tint(for: stateLabel(run.state))
      )
    }
  }

  private var transferRunSparkline: [Double]? {
    guard let warnings = warningCountEvidence else {
      return nil
    }
    return [
      max(verificationProgress, 0.06),
      max(0.08, min(verificationProgress + 0.12, 1)),
      max(0.08, 0.6 - (Double(warnings) * 0.05)),
      max(0.08, transferOverviewProgress)
    ]
  }

  private func transferStatusSymbol(for state: GateState) -> String {
    switch state {
    case .pass:
      return "checkmark.circle.fill"
    case .pending:
      return "clock.fill"
    case .review:
      return "exclamationmark.triangle.fill"
    case .blocked:
      return "xmark.octagon.fill"
    case .planned:
      return "sparkles"
    case .neutral:
      return "info.circle.fill"
    }
  }

  private func transferStageTimeLabel(_ index: Int) -> String {
    guard index <= activeStageIndex else {
      return "pending"
    }
    if let run = store.activeRun {
      return timeString(run.launchedAt)
    }
    if let recent = store.recentRuns.first(where: { store.isCurrentContext($0) }) {
      return timeString(recent.launchedAt)
    }
    return hasLoadedEvidence ? "loaded" : "pending"
  }

  private func transferLogMessage(for run: TaskRun) -> String {
    let stdout = run.stdout.trimmingCharacters(in: .whitespacesAndNewlines)
    if let line = stdout.split(separator: "\n").first, !line.isEmpty {
      return String(line)
    }
    let stderr = run.stderr.trimmingCharacters(in: .whitespacesAndNewlines)
    if let line = stderr.split(separator: "\n").first, !line.isEmpty {
      return String(line)
    }
    return "\(run.kind.displayTitle) \(stateLabel(run.state))"
  }

  private func evidenceSemanticState(for gate: GateState) -> EvidenceSemanticState {
    switch gate {
    case .pass:
      return .success
    case .pending:
      return .pending
    case .review:
      return .warning
    case .blocked:
      return .danger
    case .planned:
      return .neutral
    case .neutral:
      return .info
    }
  }

  private func evidenceSemanticState(for severity: EvidenceArtifactIssueSeverity) -> EvidenceSemanticState {
    switch severity {
    case .ok:
      return .success
    case .warning:
      return .warning
    case .critical:
      return .danger
    }
  }

  private func evidenceSemanticState(for status: EvidenceArtifactJSONStatus) -> EvidenceSemanticState {
    switch status {
    case .valid:
      return .valid
    case .notJSON, .notCheckedLargeArtifact:
      return .neutral
    case .malformed, .unreadable, .symlink:
      return .warning
    }
  }

  private func artifactSeverityLabel(_ severity: EvidenceArtifactIssueSeverity) -> String {
    switch severity {
    case .ok:
      return "clean"
    case .warning:
      return "review"
    case .critical:
      return "critical"
    }
  }

  private func evidenceStageLabel(for family: EvidenceArtifactFamily) -> String {
    switch family {
    case .profile:
      return "Config"
    case .pairing:
      return "Pairing"
    case .session, .networkTransfer:
      return "Transfer"
    case .warning:
      return "Warning"
    case .deleted, .pruneApproval, .pruneReceipt:
      return "Prune"
    case .drift, .reconcileReceipt:
      return "Review"
    case .daemon, .daemonEvent:
      return "Daemon"
    case .incrementalSyncQueue, .incrementalSyncRun:
      return "Sync"
    case .agentInfluence, .historyIndex, .recoveryState, .unknownControl:
      return "Control"
    }
  }

  private func evidenceIconName(for family: EvidenceArtifactFamily) -> String {
    switch family {
    case .profile:
      return "doc.text"
    case .pairing:
      return "link.circle"
    case .session, .networkTransfer:
      return "shippingbox"
    case .warning:
      return "doc.badge.exclamationmark"
    case .deleted:
      return "trash"
    case .drift:
      return "arrow.triangle.branch"
    case .pruneApproval, .pruneReceipt:
      return "checklist"
    case .reconcileReceipt:
      return "arrow.triangle.2.circlepath"
    case .daemon, .daemonEvent:
      return "gearshape.2"
    case .incrementalSyncQueue, .incrementalSyncRun:
      return "repeat"
    case .agentInfluence, .historyIndex, .recoveryState, .unknownControl:
      return "archivebox"
    }
  }

  private func timeString(_ date: Date) -> String {
    date.formatted(date: .omitted, time: .shortened)
  }

  private func dateTimeString(_ date: Date) -> String {
    date.formatted(date: .abbreviated, time: .shortened)
  }

  private func timeAgoString(_ date: Date) -> String {
    let formatter = RelativeDateTimeFormatter()
    formatter.unitsStyle = .short
    return formatter.localizedString(for: date, relativeTo: Date())
  }

  private func stateLabel(_ state: TaskRun.State) -> String {
    switch state {
    case .idle:
      return "idle"
    case .running:
      return "running"
    case let .finished(code):
      return "exit \(code)"
    case let .failedToLaunch(message):
      return "launch failed: \(message)"
    case .cancelled:
      return "cancelled"
    }
  }

  private func sidebarSafetyRow(_ label: String, tint: Color) -> some View {
    HStack(spacing: 8) {
      StatusDot(color: tint)
      Text(label)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
    }
  }

  private func sidebarStatusRow(label: String, value: String, tint: Color) -> some View {
    HStack(spacing: 8) {
      StatusDot(color: tint)
      Text(label)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(SMColor.secondaryText)
      Spacer(minLength: 12)
      Text(value)
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(1)
    }
  }

  private var controlRoomFocus: ControlRoomFocusModel {
    if let run = store.activeRun {
      return ControlRoomFocusModel(
        value: "LIVE",
        label: "FOREGROUND SLOT",
        detail: "\(run.kind.displayTitle) • \(store.supervisionStateLabel(for: run.slot))",
        tint: SMColor.blue,
        progress: 0.42
      )
    }

    let detail =
      hasLoadedEvidence ? transferCaption : "Run status, report, or verify to load target evidence."
    if verificationProgress > 0 {
      return ControlRoomFocusModel(
        value: "\(Int((verificationProgress * 100).rounded()))%",
        label: "VERIFIED SURFACE",
        detail: detail,
        tint: tint(for: verificationStatus),
        progress: verificationProgress
      )
    }
    if networkTransferValue != "not checked" {
      return ControlRoomFocusModel(
        value: networkTransferValue,
        label: "NETWORK TRANSFERS",
        detail: detail,
        tint: tint(for: integrityValue),
        progress: hasLoadedEvidence ? 0.16 : 0.06
      )
    }
    return ControlRoomFocusModel(
      value: integrityValue.capitalized,
      label: "INTEGRITY",
      detail: detail,
      tint: tint(for: integrityValue),
      progress: hasLoadedEvidence ? 0.16 : 0.06
    )
  }

  private var controlRoomMetricStripModel: ControlRoomMetricStripModel {
    let verificationDetail =
      verificationProgress > 0
      ? "\(Int((verificationProgress * 100).rounded()))% of expected surface"
      : "verification pending"
    let warningDetail =
      warningCountEvidence.map { $0 == 0 ? "durable warning log clean" : "review required" }
      ?? "warning evidence not loaded"

    return ControlRoomMetricStripModel(
      metrics: [
        .init(
          id: "files-verified",
          icon: "checklist",
          title: "Files Verified",
          value: filesVerifiedValue,
          detail: verificationDetail,
          tint: SMColor.blue,
          progress: verificationProgress
        ),
        .init(
          id: "network-transfers",
          icon: "arrow.left.arrow.right",
          title: "Network Transfers",
          value: networkTransferValue,
          detail: networkStatus,
          tint: SMColor.cyan,
          progress: nil
        ),
        .init(
          id: "integrity",
          icon: "checkmark.shield",
          title: "Integrity",
          value: integrityValue,
          detail: verificationStatus,
          tint: tint(for: integrityValue),
          progress: nil
        ),
        .init(
          id: "warnings",
          icon: "exclamationmark.triangle",
          title: "Warnings",
          value: warningMetricValue,
          detail: warningDetail,
          tint: countMetricTint(warningCountEvidence),
          progress: nil
        ),
      ])
  }

  private var controlRoomContextTiles: [ControlRoomContextTileModel] {
    [
      .init(
        id: "profile-path",
        title: "Migration Config",
        value: store.selectedProfileDisplayTitle,
        detail: store.selectedProfileDisplayDetail ?? "config file required",
        tint: profilePathIsSet ? SMColor.green : SMColor.amber
      ),
      .init(
        id: "target-root",
        title: "Target Root",
        value: statusTargetRoot == "-" ? "target root unknown" : statusTargetRoot,
        detail: targetSubtitle,
        tint: tint(for: integrityValue)
      ),
      .init(
        id: "role-surface",
        title: "Role Surface",
        value: store.selectedRole.localizedTitle(using: appChromeLocalization),
        detail: selectedSection.title,
        tint: SMColor.blue
      ),
    ]
  }

  private func controlRoomRecentRunModel(for run: TaskRun) -> ControlRoomRecentRunModel {
    let state = stateLabel(run.state)
    return ControlRoomRecentRunModel(
      id: run.id,
      title: run.kind.displayTitle,
      launchedAt: run.launchedAt,
      state: state,
      stateTint: tint(for: state),
      commandLine: run.commandLine.joined(separator: " ")
    )
  }

  private var controlRoomTransferArtwork: some View {
    ZStack {
      RoundedRectangle(cornerRadius: 18, style: .continuous)
        .fill(SMColor.input.opacity(0.7))
      HStack(spacing: 16) {
        ZStack {
          RoundedRectangle(cornerRadius: 16, style: .continuous)
            .fill(SMColor.graphite.opacity(0.88))
          Image(systemName: "shippingbox.fill")
            .font(.system(size: 26, weight: .regular))
            .foregroundStyle(SMColor.inverseText)
        }
        .frame(width: 72, height: 72)

        ZStack {
          Capsule()
            .fill(
              LinearGradient(
                colors: [SMColor.cyan.opacity(0.6), SMColor.blue, SMColor.green.opacity(0.65)],
                startPoint: .leading,
                endPoint: .trailing
              )
            )
            .frame(width: 80, height: 10)
          Image(systemName: "arrow.right")
            .font(.system(size: 19, weight: .bold))
            .foregroundStyle(SMColor.inverseText)
        }

        ZStack {
          RoundedRectangle(cornerRadius: 16, style: .continuous)
            .fill(SMColor.graphite.opacity(0.9))
          Image(systemName: "checkmark.shield.fill")
            .font(.system(size: 24, weight: .regular))
            .foregroundStyle(SMColor.blue)
        }
        .frame(width: 72, height: 72)
      }
    }
    .frame(height: 108)
  }

  private var controlRoomSurfaceWatermark: some View {
    ZStack {
      Circle()
        .fill(SMColor.blue.opacity(0.05))
        .frame(width: 124, height: 124)
      Image(systemName: "shippingbox.circle.fill")
        .font(.system(size: 72, weight: .regular))
        .foregroundStyle(SMColor.blue.opacity(0.08))
    }
  }

  private func controlRoomGateListRow(_ label: String, state: GateState) -> some View {
    HStack(spacing: 10) {
      StatusDot(color: state.color)
      Text(label)
        .font(.system(size: 12, weight: .medium))
        .foregroundStyle(SMColor.primaryText)
      Spacer()
      Text(state.title)
        .font(.system(size: 11, weight: .medium))
        .foregroundStyle(state.color)
    }
    .padding(.horizontal, 12)
    .padding(.vertical, 11)
  }
  private func controlRoomStageStatus(index: Int, activeIndex: Int) -> String {
    if index < activeIndex {
      return "complete"
    }
    if index == activeIndex {
      return "current"
    }
    return "pending"
  }

  private func controlRoomStageSummary(activeIndex: Int) -> String {
    let stages = ["prepare", "connect", "move", "verify & repair", "evidence"]
    guard stages.indices.contains(activeIndex) else {
      return "awaiting config"
    }
    return "\(stages[activeIndex]) stage"
  }

}

private struct RawEvidencePreview {
  let text: String
  let summary: String
  let truncated: Bool
}
