import SwiftUI

struct EvidenceViewState {
  var selectedStageFilterID: String = "all"
  var selectedTypeFilterID: String = "all"
  var selectedStatusFilterID: String = "all"
  var artifactFamilyFilterID: String = "all"
  var searchQuery: String = ""
  var selectedRecordID: String? = nil
  var pageIndex: Int = 0
}

struct EvidenceSectionView: View {
  let model: EvidenceSectionModel
  var localization: AppChromeLocalization = AppChromeLocalization(language: .english)
  let actions: EvidenceSectionActions

  var body: some View {
    if let posture = model.safetyPosture {
      DetailPageHost(
        header: .init(title: model.title, subtitle: model.subtitle),
        asideWidth: WorkbenchLayoutMetrics.evidenceAsideWidth,
        asideLeading: true,
        spacing: 16,
        primary: {
          contentBody
        },
        aside: {
          EvidenceSafetyPostureCard(
            posture: posture,
            localization: localization,
            action: actions.openSafetyPosture
          )
        }
      )
    } else {
      DetailPageHost(
        header: .init(title: model.title, subtitle: model.subtitle),
        spacing: 16,
        primary: {
          contentBody
        }
      )
    }
  }

  private var contentBody: some View {
    VStack(alignment: .leading, spacing: 16) {
      primaryDesk

      if let supportingContent = model.supportingContent {
        ScreenCard(
          title: localization.text("Evidence Supporting Surfaces"),
          subtitle: localization.text("Acceptance, raw vault, and advanced review panels that still sit outside the primary evidence desk.")
        ) {
          VStack(alignment: .leading, spacing: 16) {
            if actions.hasSupportingToolbar {
              EvidenceSupportingToolbar(localization: localization, actions: actions)
            }
            supportingContent
          }
        }
      }
    }
  }

  private var primaryDesk: some View {
    VStack(alignment: .leading, spacing: 16) {
      EvidenceToolbar(model: model, localization: localization, actions: actions)
      EvidenceArtifactTable(
        model: model,
        localization: localization,
        selectRecord: actions.selectRecord
      )
      EvidenceTableFooter(
        selectionSummary: model.selectionSummary,
        pagination: model.pagination,
        previousPage: actions.previousPage,
        nextPage: actions.nextPage
      )

      if let detail = model.selectedDetail {
        EvidenceInspector(detail: detail, localization: localization, actions: actions)
      } else {
        EvidenceEmptyInspector(localization: localization)
      }
    }
  }
}

struct EvidenceSectionModel {
  var title: String = "Evidence Vault"
  var subtitle: String = "Browse immutable, evidence-backed records for this migration."
  var updatedAt: String
  var stageFilter: EvidenceFilterState
  var typeFilter: EvidenceFilterState
  var statusFilter: EvidenceFilterState
  var searchQuery: String
  var records: [EvidenceRecord]
  var selectionSummary: String
  var pagination: EvidencePagination
  var safetyPosture: EvidenceSafetyPosture?
  var selectedDetail: EvidenceRecordDetail?
  var supportingContent: AnyView?
}

struct EvidenceSectionActions {
  var selectStage: @MainActor @Sendable (String) -> Void = { _ in }
  var selectType: @MainActor @Sendable (String) -> Void = { _ in }
  var selectStatus: @MainActor @Sendable (String) -> Void = { _ in }
  var updateSearch: @MainActor @Sendable (String) -> Void = { _ in }
  var exportList: (() -> Void)? = nil
  var selectRecord: (String) -> Void = { _ in }
  var previousPage: (() -> Void)? = nil
  var nextPage: (() -> Void)? = nil
  var viewSelectedJSON: (() -> Void)? = nil
  var openSafetyPosture: (() -> Void)? = nil
  var runStatus: (() -> Void)? = nil
  var runReport: (() -> Void)? = nil
  var runHealth: (() -> Void)? = nil
  var runVerify: (() -> Void)? = nil
  var refreshArtifacts: (() -> Void)? = nil
  var runDaemonLogs: (() -> Void)? = nil

  var hasSupportingToolbar: Bool {
    runStatus != nil || runReport != nil || runHealth != nil || runVerify != nil
      || refreshArtifacts != nil || runDaemonLogs != nil
  }
}

struct EvidenceFilterState {
  var selectedID: String
  var options: [EvidenceFilterOption]
}

struct EvidenceFilterOption: Identifiable, Hashable {
  let id: String
  let title: String
}

struct EvidencePagination {
  var pageLabel: String
  var canGoBackward: Bool
  var canGoForward: Bool
}

struct EvidenceSafetyPosture {
  var title: String
  var state: EvidenceSemanticState
  var summary: String
  var details: [String]
  var actionTitle: String
}

struct EvidenceRecord: Identifiable, Hashable {
  let id: String
  var type: String
  var stage: String
  var source: String
  var target: String
  var created: String
  var status: EvidenceBadge
  var signature: EvidenceBadge
  var size: String
  var iconSystemName: String = "doc.text"
  var accent: EvidenceSemanticState = .neutral
  var isSelected: Bool = false
}

struct EvidenceRecordDetail {
  let id: String
  var title: String
  var titleBadge: EvidenceBadge?
  var signatureBadge: EvidenceBadge?
  var iconSystemName: String
  var facts: [EvidenceDetailFact]
  var tags: [String]
  var summaryMetrics: [EvidenceMetric]
  var signatureDetails: [EvidenceDetailFact]
  var verificationChecks: [EvidenceVerificationCheck]
  var timeline: [EvidenceTimelineEntry]
  var notes: String
}

struct EvidenceDetailFact: Identifiable, Hashable {
  let id: String
  var label: String
  var value: String
}

struct EvidenceMetric: Identifiable, Hashable {
  let id: String
  var label: String
  var value: String
}

struct EvidenceVerificationCheck: Identifiable, Hashable {
  let id: String
  var label: String
  var value: String
  var state: EvidenceSemanticState
}

struct EvidenceTimelineEntry: Identifiable, Hashable {
  let id: String
  var title: String
  var timestamp: String
  var detail: String
  var state: EvidenceSemanticState
}

struct EvidenceBadge: Hashable {
  var text: String
  var state: EvidenceSemanticState
}

enum EvidenceSemanticState: String, Hashable {
  case neutral
  case info
  case success
  case warning
  case danger
  case sealed
  case valid
  case trusted
  case durable
  case pending
  case verified
  case passed

  var color: Color {
    switch self {
    case .neutral:
      return SMColor.secondaryText
    case .info:
      return SMColor.blue
    case .success, .sealed, .valid, .trusted, .verified, .passed:
      return SMColor.green
    case .warning, .durable, .pending:
      return SMColor.amber
    case .danger:
      return SMColor.red
    }
  }

  var symbolName: String {
    switch self {
    case .neutral:
      return "minus.circle.fill"
    case .info:
      return "info.circle.fill"
    case .success, .trusted, .verified, .passed:
      return "checkmark.circle.fill"
    case .warning, .durable:
      return "exclamationmark.shield.fill"
    case .danger:
      return "xmark.octagon.fill"
    case .sealed:
      return "checkmark.shield.fill"
    case .valid:
      return "checkmark.seal.fill"
    case .pending:
      return "clock.fill"
    }
  }
}

private struct EvidenceToolbar: View {
  let model: EvidenceSectionModel
  let localization: AppChromeLocalization
  let actions: EvidenceSectionActions

  var body: some View {
    WorkbenchToolbarStrip {
      ViewThatFits(in: .horizontal) {
        wideToolbar
        compactToolbar
      }
    }
  }

  private var wideToolbar: some View {
    HStack(alignment: .center, spacing: 12) {
      filterControls

      searchField

      Spacer(minLength: 12)

      toolbarMeta
    }
  }

  private var compactToolbar: some View {
    VStack(alignment: .leading, spacing: 12) {
      if hasFilterControls {
        HStack(alignment: .center, spacing: 12) {
          filterControls
          Spacer(minLength: 0)
        }
      }

      HStack(alignment: .center, spacing: 12) {
        searchField
          .frame(minWidth: 220, maxWidth: .infinity, alignment: .leading)

        toolbarMeta
      }
    }
  }

  @ViewBuilder
  private var filterControls: some View {
    if model.stageFilter.options.count > 1 {
      EvidenceFilterPicker(
        title: localization.text("Stage"),
        state: model.stageFilter,
        selection: actions.selectStage
      )
    }
    if model.typeFilter.options.count > 1 {
      EvidenceFilterPicker(
        title: localization.text("Type"),
        state: model.typeFilter,
        selection: actions.selectType
      )
    }
    if model.statusFilter.options.count > 1 {
      EvidenceFilterPicker(
        title: localization.text("Status"),
        state: model.statusFilter,
        selection: actions.selectStatus
      )
    }
  }

  private var hasFilterControls: Bool {
    model.stageFilter.options.count > 1
      || model.typeFilter.options.count > 1
      || model.statusFilter.options.count > 1
  }

  @ViewBuilder
  private var toolbarMeta: some View {
    Text("\(localization.text("Last updated")): \(model.updatedAt)")
      .font(.system(size: 12))
      .foregroundStyle(SMColor.secondaryText)
      .lineLimit(1)

    if let exportList = actions.exportList {
      ActionButton(localization.text("Export…"), systemImage: "square.and.arrow.up", action: exportList)
    }
  }

  private var searchField: some View {
    WorkbenchSearchField(
      text: model.searchQuery,
      placeholder: localization.text("Search evidence…"),
      onChange: actions.updateSearch
    )
    .frame(minWidth: 260, maxWidth: 390)
  }
}

private struct EvidenceSupportingToolbar: View {
  let localization: AppChromeLocalization
  let actions: EvidenceSectionActions

  var body: some View {
    FlowLayout(horizontalSpacing: 12, verticalSpacing: 12) {
      if let runStatus = actions.runStatus {
        ActionButton(localization.text("Status"), systemImage: "waveform.path.ecg", action: runStatus)
      }
      if let runReport = actions.runReport {
        ActionButton(localization.text("Report"), systemImage: "doc.text", action: runReport)
      }
      if let runHealth = actions.runHealth {
        ActionButton(localization.text("Health"), systemImage: "cross.case", action: runHealth)
      }
      if let runVerify = actions.runVerify {
        ActionButton(localization.text("Verify"), systemImage: "checkmark.seal", action: runVerify)
      }
      if let refreshArtifacts = actions.refreshArtifacts {
        ActionButton(localization.text("Artifacts"), systemImage: "archivebox", action: refreshArtifacts)
      }
      if let runDaemonLogs = actions.runDaemonLogs {
        ActionButton(localization.text("Daemon Logs"), systemImage: "list.bullet.rectangle", action: runDaemonLogs)
      }
    }
  }
}

private struct EvidenceFilterPicker: View {
  let title: String
  let state: EvidenceFilterState
  let selection: @MainActor @Sendable (String) -> Void

  var body: some View {
    Picker(
      title,
      selection: Binding(
        get: { state.selectedID },
        set: { newValue in
          Task { @MainActor in
            selection(newValue)
          }
        }
      )
    ) {
      ForEach(state.options) { option in
        Text(option.title).tag(option.id)
      }
    }
    .pickerStyle(.menu)
    .frame(width: 138)
  }
}

private struct EvidenceArtifactTable: View {
  let model: EvidenceSectionModel
  let localization: AppChromeLocalization
  let selectRecord: (String) -> Void

  private var columns: [EvidenceColumn] {
    [
      EvidenceColumn(title: localization.text("ID"), width: 154, alignment: .leading),
      EvidenceColumn(title: localization.text("Type"), width: 118, alignment: .leading),
      EvidenceColumn(title: localization.text("Stage"), width: 82, alignment: .leading),
      EvidenceColumn(title: localization.text("Source"), width: 180, alignment: .leading),
      EvidenceColumn(title: localization.text("Target"), width: 180, alignment: .leading),
      EvidenceColumn(title: localization.text("Created"), width: 178, alignment: .leading),
      EvidenceColumn(title: localization.text("Status"), width: 106, alignment: .leading),
      EvidenceColumn(title: localization.text("Signature"), width: 96, alignment: .leading),
      EvidenceColumn(title: localization.text("Size"), width: 66, alignment: .trailing)
    ]
  }

  private var tableWidth: CGFloat {
    columns.reduce(CGFloat(28)) { partial, column in
      partial + column.width
    }
  }

  var body: some View {
    ScrollView(.horizontal, showsIndicators: true) {
      VStack(spacing: 0) {
        header

        Divider()
          .overlay(SMColor.hairline)

        ScrollView {
          LazyVStack(spacing: 0) {
            ForEach(model.records) { record in
              Button {
                selectRecord(record.id)
              } label: {
                EvidenceArtifactRow(record: record, columns: columns)
              }
              .buttonStyle(.plain)

              if record.id != model.records.last?.id {
                Divider()
                  .overlay(SMColor.hairline.opacity(0.72))
                  .padding(.leading, 12)
              }
            }
          }
        }
        .frame(minHeight: 220, idealHeight: 280, maxHeight: 320)
      }
      .frame(width: tableWidth, alignment: .leading)
    }
    .panelSurface(.panel, padding: EdgeInsets(top: 0, leading: 0, bottom: 0, trailing: 0))
  }

  private var header: some View {
    HStack(spacing: 0) {
      ForEach(columns) { column in
        Text(column.title)
          .font(.system(size: 11, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
          .frame(width: column.width, alignment: column.alignment)
      }
    }
    .padding(.horizontal, 14)
    .padding(.vertical, 11)
    .background(SMColor.cardElevated)
  }
}

private struct EvidenceArtifactRow: View {
  let record: EvidenceRecord
  let columns: [EvidenceColumn]

  var body: some View {
    HStack(spacing: 0) {
      leadingIDCell
        .frame(width: columns[0].width, alignment: .leading)
      evidenceCell(record.type, width: columns[1].width)
      evidenceCell(record.stage, width: columns[2].width)
      evidenceCell(record.source, width: columns[3].width)
      evidenceCell(record.target, width: columns[4].width)
      evidenceCell(record.created, width: columns[5].width)
      badgeCell(record.status, width: columns[6].width)
      badgeCell(record.signature, width: columns[7].width)
      evidenceCell(record.size, width: columns[8].width, alignment: .trailing)
    }
    .padding(.horizontal, 14)
    .padding(.vertical, 9)
    .background(record.isSelected ? SMColor.blue.opacity(0.08) : SMColor.card)
  }

  private var leadingIDCell: some View {
    HStack(spacing: 10) {
      Image(systemName: record.iconSystemName)
        .font(.system(size: 13, weight: .semibold))
        .foregroundStyle(record.accent.color)
        .frame(width: 16)
      Text(record.id)
        .font(.system(size: 12.5, weight: .medium))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(1)
    }
  }

  private func evidenceCell(
    _ text: String,
    width: CGFloat,
    alignment: Alignment = .leading
  ) -> some View {
    Text(text)
      .font(.system(size: 12.5))
      .foregroundStyle(SMColor.primaryText)
      .lineLimit(1)
      .truncationMode(.tail)
      .frame(width: width, alignment: alignment)
  }

  private func badgeCell(_ badge: EvidenceBadge, width: CGFloat) -> some View {
    HStack {
      EvidenceStatusBadge(badge: badge, compact: true)
      Spacer(minLength: 0)
    }
    .frame(width: width, alignment: .leading)
  }
}

private struct EvidenceTableFooter: View {
  let selectionSummary: String
  let pagination: EvidencePagination
  let previousPage: (() -> Void)?
  let nextPage: (() -> Void)?

  var body: some View {
    HStack {
      Text(selectionSummary)
        .font(.system(size: 12))
        .foregroundStyle(SMColor.secondaryText)

      Spacer()

      HStack(spacing: 8) {
        if pagination.canGoBackward, let previousPage {
          IconActionButton(systemImage: "chevron.left", action: previousPage, size: 24)
        }

        Text(pagination.pageLabel)
          .font(.system(size: 12, weight: .medium))
          .foregroundStyle(SMColor.primaryText)
          .frame(minWidth: 32)

        if pagination.canGoForward, let nextPage {
          IconActionButton(systemImage: "chevron.right", action: nextPage, size: 24)
        }
      }
      .foregroundStyle(SMColor.secondaryText)
      .opacity((pagination.canGoBackward || pagination.canGoForward) ? 1 : 0.75)
    }
  }
}

private struct EvidenceInspector: View {
  let detail: EvidenceRecordDetail
  let localization: AppChromeLocalization
  let actions: EvidenceSectionActions

  var body: some View {
    ViewThatFits(in: .horizontal) {
      wideLayout
      compactLayout
    }
  }

  private var wideLayout: some View {
    HStack(alignment: .top, spacing: 16) {
      EvidenceDetailOverview(detail: detail, localization: localization, actions: actions)
        .frame(minWidth: 290, idealWidth: 318, maxWidth: 330)

      VStack(spacing: 16) {
        EvidenceMetricCard(title: localization.text("Summary"), metrics: detail.summaryMetrics)
        if !detail.timeline.isEmpty {
          EvidenceTimelineCard(entries: detail.timeline, localization: localization)
        } else {
          EvidenceNotesCard(notes: detail.notes, localization: localization)
        }
      }
      .frame(maxWidth: .infinity, alignment: .leading)

      VStack(spacing: 16) {
        EvidenceFactsCard(
          title: localization.text("Digital Signature"),
          badge: detail.signatureBadge,
          facts: detail.signatureDetails
        )
        EvidenceVerificationCard(
          checks: detail.verificationChecks,
          localization: localization,
          viewRawJSON: actions.viewSelectedJSON
        )
        if !detail.timeline.isEmpty {
          EvidenceNotesCard(notes: detail.notes, localization: localization)
        }
      }
      .frame(minWidth: 310, idealWidth: 330, maxWidth: 360, alignment: .leading)
    }
  }

  private var compactLayout: some View {
    VStack(alignment: .leading, spacing: 16) {
      EvidenceDetailOverview(detail: detail, localization: localization, actions: actions)

      EvidenceMetricCard(title: localization.text("Summary"), metrics: detail.summaryMetrics)

      EvidenceFactsCard(
        title: localization.text("Digital Signature"),
        badge: detail.signatureBadge,
        facts: detail.signatureDetails
      )

      EvidenceVerificationCard(
        checks: detail.verificationChecks,
        localization: localization,
        viewRawJSON: actions.viewSelectedJSON
      )

      if !detail.timeline.isEmpty {
        EvidenceTimelineCard(entries: detail.timeline, localization: localization)
      }

      EvidenceNotesCard(notes: detail.notes, localization: localization)
    }
  }
}

private struct EvidenceDetailOverview: View {
  let detail: EvidenceRecordDetail
  let localization: AppChromeLocalization
  let actions: EvidenceSectionActions

  var body: some View {
    VStack(alignment: .leading, spacing: 16) {
      HStack(alignment: .top, spacing: 12) {
        Image(systemName: detail.iconSystemName)
          .font(.system(size: 28, weight: .semibold))
          .foregroundStyle(SMColor.blue)
          .frame(width: 36)

        VStack(alignment: .leading, spacing: 6) {
          HStack(spacing: 10) {
            Text(detail.title)
              .font(.system(size: 16, weight: .bold))
              .foregroundStyle(SMColor.primaryText)
            if let badge = detail.titleBadge {
              EvidenceStatusBadge(badge: badge, compact: false)
            }
          }

          Text(detail.id)
            .font(.system(size: 12, design: .monospaced))
            .foregroundStyle(SMColor.secondaryText)
            .textSelection(.enabled)
        }

        Spacer(minLength: 0)
      }

      EvidenceFactGrid(facts: detail.facts)

      if !detail.tags.isEmpty {
        VStack(alignment: .leading, spacing: 8) {
          Text(localization.text("Tags"))
            .font(.system(size: 11, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)
          FlexibleTagRow(tags: detail.tags)
        }
      }
    }
    .panelSurface(.panel)
  }
}

private struct EvidenceMetricCard: View {
  let title: String
  let metrics: [EvidenceMetric]

  var body: some View {
    VStack(alignment: .leading, spacing: 10) {
      Text(title)
        .font(.system(size: 14, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)

      VStack(spacing: 9) {
        ForEach(metrics) { metric in
          HStack(alignment: .firstTextBaseline, spacing: 12) {
            Text(metric.label)
              .font(.system(size: 12))
              .foregroundStyle(SMColor.secondaryText)
            Spacer(minLength: 12)
            Text(metric.value)
              .font(.system(size: 12, design: .monospaced))
              .foregroundStyle(SMColor.primaryText)
              .textSelection(.enabled)
              .multilineTextAlignment(.trailing)
          }
        }
      }
    }
    .panelSurface(.panel)
  }
}

private struct EvidenceFactsCard: View {
  let title: String
  let badge: EvidenceBadge?
  let facts: [EvidenceDetailFact]

  var body: some View {
    VStack(alignment: .leading, spacing: 12) {
      HStack(spacing: 10) {
        Text(title)
          .font(.system(size: 14, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        if let badge {
          EvidenceStatusBadge(badge: badge, compact: false)
        }
      }

      EvidenceFactGrid(facts: facts)
    }
    .panelSurface(.panel)
  }
}

private struct EvidenceVerificationCard: View {
  let checks: [EvidenceVerificationCheck]
  let localization: AppChromeLocalization
  let viewRawJSON: (() -> Void)?

  var body: some View {
    VStack(alignment: .leading, spacing: 10) {
      Text(localization.text("Verification"))
        .font(.system(size: 14, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)

      VStack(spacing: 10) {
        ForEach(checks) { check in
          HStack(alignment: .top, spacing: 10) {
            Image(systemName: check.state.symbolName)
              .font(.system(size: 12, weight: .semibold))
              .foregroundStyle(check.state.color)
              .frame(width: 14)
            Text(check.label)
              .font(.system(size: 12))
              .foregroundStyle(SMColor.primaryText)
            Spacer(minLength: 10)
            Text(check.value)
              .font(.system(size: 12, weight: .medium))
              .foregroundStyle(check.state.color)
              .multilineTextAlignment(.trailing)
          }
        }
      }

      if let viewRawJSON {
        ActionButton(localization.text("View raw JSON"), systemImage: "curlybraces", action: viewRawJSON)
      }
    }
    .panelSurface(.panel)
  }
}

private struct EvidenceTimelineCard: View {
  let entries: [EvidenceTimelineEntry]
  let localization: AppChromeLocalization

  var body: some View {
    VStack(alignment: .leading, spacing: 12) {
      Text(localization.text("Timeline"))
        .font(.system(size: 14, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)

      VStack(alignment: .leading, spacing: 0) {
        ForEach(Array(entries.enumerated()), id: \.element.id) { index, entry in
          HStack(alignment: .top, spacing: 12) {
            VStack(spacing: 4) {
              Circle()
                .fill(entry.state.color)
                .frame(width: 10, height: 10)
              if index != entries.count - 1 {
                Rectangle()
                  .fill(SMColor.hairline)
                  .frame(width: 2)
                  .frame(maxHeight: .infinity)
              }
            }
            .frame(width: 12)

            VStack(alignment: .leading, spacing: 4) {
              HStack {
                Text(entry.title)
                  .font(.system(size: 12.5, weight: .semibold))
                  .foregroundStyle(SMColor.primaryText)
                Spacer(minLength: 10)
                Text(entry.timestamp)
                  .font(.system(size: 11.5))
                  .foregroundStyle(SMColor.secondaryText)
              }
              Text(entry.detail)
                .font(.system(size: 12))
                .foregroundStyle(SMColor.secondaryText)
                .fixedSize(horizontal: false, vertical: true)
            }
          }
          .padding(.vertical, 7)
        }
      }
    }
    .panelSurface(.panel)
  }
}

private struct EvidenceNotesCard: View {
  let notes: String
  let localization: AppChromeLocalization

  var body: some View {
    VStack(alignment: .leading, spacing: 10) {
      Text(localization.text("Notes"))
        .font(.system(size: 14, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)

      Text(notes.isEmpty ? localization.text("No operator notes recorded.") : notes)
        .font(.system(size: 12))
        .foregroundStyle(notes.isEmpty ? SMColor.secondaryText : SMColor.primaryText)
        .frame(maxWidth: .infinity, minHeight: 78, alignment: .topLeading)
        .textSelection(.enabled)
    }
    .panelSurface(.panel)
  }
}

private struct EvidenceSafetyPostureCard: View {
  let posture: EvidenceSafetyPosture
  let localization: AppChromeLocalization
  let action: (() -> Void)?

  var body: some View {
    VStack(alignment: .leading, spacing: 14) {
      Text(localization.text("Safety Posture"))
        .font(.system(size: 11, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
        .textCase(.uppercase)

      HStack(spacing: 10) {
        Image(systemName: posture.state.symbolName)
          .font(.system(size: 16, weight: .semibold))
          .foregroundStyle(posture.state.color)
        VStack(alignment: .leading, spacing: 4) {
          Text(posture.title)
            .font(.system(size: 15, weight: .semibold))
            .foregroundStyle(posture.state.color)
          Text(posture.summary)
            .font(.system(size: 12))
            .foregroundStyle(SMColor.secondaryText)
            .fixedSize(horizontal: false, vertical: true)
        }
      }

      VStack(alignment: .leading, spacing: 8) {
        ForEach(Array(posture.details.enumerated()), id: \.offset) { _, detail in
          Text(detail)
            .font(.system(size: 12))
            .foregroundStyle(SMColor.secondaryText)
        }
      }

      if let action {
        Button(action: action) {
          Text(posture.actionTitle)
            .font(.system(size: 12.5, weight: .semibold))
            .foregroundStyle(SMColor.blue)
        }
        .buttonStyle(.plain)
      }
    }
    .panelSurface(.panel)
  }
}

private struct EvidenceEmptyInspector: View {
  let localization: AppChromeLocalization

  var body: some View {
    VStack(alignment: .leading, spacing: 8) {
      Text(localization.text("Select an evidence record"))
        .font(.system(size: 14, weight: .semibold))
        .foregroundStyle(SMColor.primaryText)
      Text(localization.text("The inspector shows signed metadata, integrity checks, and operator actions for the current selection."))
        .font(.system(size: 12))
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
    }
    .panelSurface(.notice)
  }
}

private struct EvidenceStatusBadge: View {
  let badge: EvidenceBadge
  let compact: Bool

  var body: some View {
    HStack(spacing: compact ? 5 : 6) {
      Image(systemName: badge.state.symbolName)
        .font(.system(size: compact ? 10 : 11, weight: .semibold))
      Text(badge.text)
        .font(.system(size: compact ? 11 : 12, weight: .medium))
        .lineLimit(1)
    }
    .foregroundStyle(badge.state.color)
    .padding(.horizontal, compact ? 0 : 8)
    .padding(.vertical, compact ? 0 : 4)
    .background(compact ? .clear : badge.state.color.opacity(0.08))
    .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
  }
}

private struct EvidenceFactGrid: View {
  let facts: [EvidenceDetailFact]

  var body: some View {
    VStack(alignment: .leading, spacing: 9) {
      ForEach(facts) { fact in
        HStack(alignment: .top, spacing: 12) {
          Text(fact.label)
            .font(.system(size: 12))
            .foregroundStyle(SMColor.secondaryText)
            .frame(width: 110, alignment: .leading)
          Text(fact.value)
            .font(.system(size: 12, design: .monospaced))
            .foregroundStyle(SMColor.primaryText)
            .textSelection(.enabled)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
      }
    }
  }
}

private struct FlexibleTagRow: View {
  let tags: [String]

  var body: some View {
    FlowLayout(horizontalSpacing: 8, verticalSpacing: 8) {
      ForEach(tags, id: \.self) { tag in
        StatusBadge(
          item: .init(icon: "number", label: tag, tint: SMColor.secondaryText),
          prominence: .softFill,
          compact: false
        )
      }
    }
  }
}

private struct FlowLayout: Layout {
  var horizontalSpacing: CGFloat
  var verticalSpacing: CGFloat

  func sizeThatFits(
    proposal: ProposedViewSize,
    subviews: Subviews,
    cache: inout ()
  ) -> CGSize {
    let containerWidth = proposal.width ?? .greatestFiniteMagnitude
    let rows = arrangedRows(subviews: subviews, containerWidth: containerWidth)
    let width = rows.map(\.width).max() ?? 0
    let height = rows.reduce(CGFloat.zero) { partial, row in
      partial + row.height
    } + CGFloat(max(0, rows.count - 1)) * verticalSpacing
    return CGSize(width: width, height: height)
  }

  func placeSubviews(
    in bounds: CGRect,
    proposal: ProposedViewSize,
    subviews: Subviews,
    cache: inout ()
  ) {
    let rows = arrangedRows(subviews: subviews, containerWidth: bounds.width)
    var y = bounds.minY
    for row in rows {
      var x = bounds.minX
      for element in row.elements {
        let size = element.size
        element.subview.place(
          at: CGPoint(x: x, y: y),
          proposal: ProposedViewSize(width: size.width, height: size.height)
        )
        x += size.width + horizontalSpacing
      }
      y += row.height + verticalSpacing
    }
  }

  private func arrangedRows(subviews: Subviews, containerWidth: CGFloat) -> [FlowRow] {
    guard !subviews.isEmpty else { return [] }
    let maxWidth = containerWidth.isFinite ? max(containerWidth, 1) : .greatestFiniteMagnitude
    var rows: [FlowRow] = []
    var current = FlowRow()

    for subview in subviews {
      let size = subview.sizeThatFits(.unspecified)
      let itemWidth = min(size.width, maxWidth)
      let proposedWidth = current.elements.isEmpty
        ? itemWidth
        : current.width + horizontalSpacing + itemWidth

      if proposedWidth > maxWidth && !current.elements.isEmpty {
        rows.append(current)
        current = FlowRow()
      }

      current.elements.append(.init(subview: subview, size: CGSize(width: itemWidth, height: size.height)))
      current.width = current.elements.reduce(CGFloat.zero) { partial, element in
        partial + element.size.width
      } + CGFloat(max(0, current.elements.count - 1)) * horizontalSpacing
      current.height = max(current.height, size.height)
    }

    if !current.elements.isEmpty {
      rows.append(current)
    }
    return rows
  }

  private struct FlowRow {
    var elements: [FlowElement] = []
    var width: CGFloat = 0
    var height: CGFloat = 0
  }

  private struct FlowElement {
    let subview: LayoutSubview
    let size: CGSize
  }
}

private struct EvidenceColumn: Identifiable {
  let id = UUID()
  let title: String
  let width: CGFloat
  let alignment: Alignment
}

struct EvidenceScreen: View {
  @EnvironmentObject private var store: AppStore
  let localization: AppChromeLocalization
  @State private var viewState = EvidenceViewState()

  var body: some View {
    EvidenceSectionView(
      model: evidenceSectionModel,
      localization: localization,
      actions: evidenceSectionActions
    )
  }

  private var evidenceSectionModel: EvidenceSectionModel {
    let records = evidenceSectionRecords
    let selectionSummary: String
    if records.isEmpty {
      selectionSummary = localization.text("No records loaded")
    } else if let selected = records.first(where: \.isSelected) {
      selectionSummary = "\(selected.id) \(localization.text("selected")) · \(records.count) \(localization.text("total"))"
    } else {
      selectionSummary = "\(records.count) \(localization.text("record(s)"))"
    }
    return EvidenceSectionModel(
      title: AppSection.evidence.localizedTitle(using: localization),
      subtitle: localization.text("Browse immutable, evidence-backed records for this migration."),
      updatedAt: evidenceUpdatedAtLabel,
      stageFilter: .init(
        selectedID: viewState.selectedStageFilterID,
        options: [.init(id: "all", title: localization.text("All Stages"))]
      ),
      typeFilter: .init(
        selectedID: viewState.selectedTypeFilterID,
        options: [.init(id: "all", title: localization.text("All Types"))]
      ),
      statusFilter: .init(
        selectedID: viewState.selectedStatusFilterID,
        options: [.init(id: "all", title: localization.text("All Statuses"))]
      ),
      searchQuery: viewState.searchQuery,
      records: records,
      selectionSummary: selectionSummary,
      pagination: .init(pageLabel: "1", canGoBackward: false, canGoForward: false),
      safetyPosture: EvidenceSafetyPosture(
        title: localization.text(aggregateEvidenceGateState.title.capitalized),
        state: evidenceSemanticState(for: aggregateEvidenceGateState),
        summary: hasLoadedEvidence
          ? localization.text("Loaded evidence surfaces are available for this context.")
          : localization.text("Read status, report, health, or verify to populate evidence-backed review surfaces."),
        details: [
          "\(localization.text("Aggregate evidence gate")): \(aggregateEvidenceGateState.title)",
          "\(localization.text("Warnings")): \(warningMetricValue)",
          "\(localization.text("Artifact problems")): \(artifactProblemMetricValue)",
        ],
        actionTitle: localization.text("Review next actions")
      ),
      selectedDetail: evidenceSelectedDetail,
      supportingContent: AnyView(
        VStack(alignment: .leading, spacing: 16) {
          AcceptanceBundlePanel(
            bundlePath: store.acceptanceBundlePath,
            snapshot: store.acceptanceBundleSnapshot,
            loadError: store.acceptanceBundleLoadError,
            browseBundle: store.browseAcceptanceBundle,
            refreshBundle: store.refreshAcceptanceBundle,
            servePhase: $store.acceptanceServePhase,
            requireOperatorEvidence: Binding(
                get: { store.acceptanceEvaluationMode.requireOperatorEvidence },
                set: { store.setAcceptanceEvaluationRequireOperatorEvidence($0) }
            ),
            requireOperatorEvidenceLocked: store.acceptanceEvaluationMode.isLockedForTwoMachineCollection,
            role: store.selectedRole,
            recordBrowse: store.recordAcceptanceDiscoveryBrowseArtifact,
            recordAdvertise: store.recordAcceptanceDiscoveryAdvertiseArtifact,
            recordServePhase: store.recordAcceptanceServePhaseArtifact,
            recordSourcePair: store.recordAcceptanceSourcePairArtifact,
            recordSourceTransfer: store.recordAcceptanceSourceTransferArtifact,
            recordTargetImport: store.recordAcceptanceTargetImportArtifact,
            recordEvaluation: store.recordAcceptanceEvaluationArtifact,
            recordPackagingEvidence: store.recordAcceptancePackagingEvidence,
            localization: localization
          )
          AcceptanceOperatorEvidencePanel(
            draft: $store.acceptanceOperatorEvidence,
            role: store.selectedRole,
            bundlePath: store.acceptanceBundlePath,
            snapshot: store.acceptanceBundleSnapshot,
            requireOperatorEvidence: store.acceptanceEvaluationMode.requireOperatorEvidence,
            requireOperatorEvidenceLocked: store.acceptanceEvaluationMode.isLockedForTwoMachineCollection,
            chooseArtifact: store.browseAcceptanceOperatorArtifact,
            clearArtifact: store.clearAcceptanceOperatorArtifact,
            recordEvidence: store.recordAcceptanceOperatorEvidence,
            refreshBundle: store.refreshAcceptanceBundle,
            localization: localization
          )
          alignmentScopePanel
          evidenceSnapshot
          evidenceArtifactCatalogPanel
          rawEvidenceSurfaces
          nextActionPreviewPanel
          artifactReaderProblems
          appEventLog
        }
      )
    )
  }

  private var evidenceSectionActions: EvidenceSectionActions {
    EvidenceSectionActions(
      selectStage: { viewState.selectedStageFilterID = $0 },
      selectType: { viewState.selectedTypeFilterID = $0 },
      selectStatus: { viewState.selectedStatusFilterID = $0 },
      updateSearch: { viewState.searchQuery = $0 },
      selectRecord: { viewState.selectedRecordID = $0 },
      previousPage: nil,
      nextPage: nil,
      viewSelectedJSON: nil,
      openSafetyPosture: nil,
      runStatus: { run(.status) },
      runReport: { run(.report) },
      runHealth: { run(.health) },
      runVerify: { run(.verify) },
      refreshArtifacts: { store.refreshEvidenceArtifactCatalog() },
      runDaemonLogs: { run(.daemonLogs) }
    )
  }

  private var evidenceSectionRecords: [EvidenceRecord] {
    let artifacts = Array(filteredEvidenceArtifacts.prefix(8))
    if !artifacts.isEmpty {
      let firstRecordID = artifacts.first.map { $0.artifactID ?? $0.relativePath }
      return artifacts.map { artifact in
        let recordID = artifact.artifactID ?? artifact.relativePath
        return EvidenceRecord(
          id: recordID,
          type: artifact.family.title,
          stage: evidenceStageLabel(for: artifact.family),
          source: sourceTitle,
          target: artifact.relativePath,
          created: artifact.modifiedAt.map(dateTimeString) ?? "unknown",
          status: EvidenceBadge(
            text: artifactJSONSeverityLabel(artifact.issueSeverity),
            state: evidenceSemanticState(for: artifact.issueSeverity)
          ),
          signature: EvidenceBadge(
            text: artifactJSONStatusLabel(artifact.jsonStatus),
            state: evidenceSemanticState(for: artifact.jsonStatus)
          ),
          size: artifact.size.map(formattedArtifactSize) ?? "-",
          iconSystemName: evidenceIconName(for: artifact.family),
          accent: evidenceSemanticState(for: artifact.issueSeverity),
          isSelected: viewState.selectedRecordID == recordID
            || (viewState.selectedRecordID == nil && recordID == firstRecordID)
        )
      }
    }

    let envelopes = Array(rawEvidenceEnvelopes.prefix(8))
    return envelopes.enumerated().map { index, envelope in
      let recordID = "\(envelope.artifactKind.rawValue)-\(index)"
      return EvidenceRecord(
        id: recordID,
        type: envelope.artifactKind.title,
        stage: "Envelope",
        source: store.selectedRole.localizedTitle(using: localization),
        target: envelope.task.rawValue,
        created: dateTimeString(envelope.loadedAt),
        status: EvidenceBadge(
          text: envelope.freshness.rawValue,
          state: envelope.freshness == .current ? .trusted : .pending
        ),
        signature: EvidenceBadge(
          text: "exit \(envelope.exitCode)",
          state: envelope.exitCode == 0 ? .valid : .warning
        ),
        size: "\(envelope.rawStdout.lengthOfBytes(using: .utf8)) B",
        iconSystemName: "curlybraces",
        accent: envelope.freshness == .current ? .info : .warning,
        isSelected: viewState.selectedRecordID == recordID
          || (viewState.selectedRecordID == nil && index == 0)
      )
    }
  }

  private var evidenceSelectedDetail: EvidenceRecordDetail? {
    if let artifact = selectedEvidenceArtifact {
      let problems = store.evidenceArtifactCatalog?.problems(for: artifact) ?? []
      return EvidenceRecordDetail(
        id: artifact.artifactID ?? artifact.relativePath,
        title: artifact.fileName,
        titleBadge: EvidenceBadge(
          text: artifact.family.title,
          state: evidenceSemanticState(for: artifact.issueSeverity)
        ),
        signatureBadge: EvidenceBadge(
          text: artifactJSONStatusLabel(artifact.jsonStatus),
          state: evidenceSemanticState(for: artifact.jsonStatus)
        ),
        iconSystemName: evidenceIconName(for: artifact.family),
        facts: [
          .init(id: "path", label: "Path", value: artifact.relativePath),
          .init(id: "id", label: "Artifact ID", value: artifact.artifactID ?? "none"),
          .init(id: "family", label: "Family", value: artifact.family.title),
          .init(
            id: "modified",
            label: "Modified",
            value: artifact.modifiedAt.map(dateTimeString) ?? "unknown"
          ),
          .init(
            id: "size",
            label: "Size",
            value: artifact.size.map(formattedArtifactSize) ?? "unknown"
          ),
        ],
        tags: [artifact.family.rawValue, artifactJSONStatusLabel(artifact.jsonStatus)],
        summaryMetrics: [
          .init(
            id: "severity",
            label: "Issue severity",
            value: artifactJSONSeverityLabel(artifact.issueSeverity)
          ),
          .init(
            id: "json",
            label: "JSON status",
            value: artifactJSONStatusLabel(artifact.jsonStatus)
          ),
          .init(id: "problems", label: "Catalog problems", value: "\(problems.count)"),
        ],
        signatureDetails: [
          .init(id: "json", label: "JSON", value: artifactJSONStatusLabel(artifact.jsonStatus)),
          .init(id: "search", label: "Search text", value: artifact.searchText),
        ],
        verificationChecks: [
          .init(
            id: "catalog",
            label: "Catalog review",
            value: problems.isEmpty ? "Clean" : "\(problems.count) issue(s)",
            state: problems.isEmpty ? .passed : .warning
          ),
          .init(
            id: "json",
            label: "JSON parse",
            value: artifactJSONStatusLabel(artifact.jsonStatus),
            state: evidenceSemanticState(for: artifact.jsonStatus)
          ),
        ],
        timeline: [],
        notes: artifact.previewText.isEmpty ? "No preview retained for this artifact." : artifact.previewText
      )
    }

    guard let envelope = selectedEvidenceEnvelope else {
      return nil
    }

    let preview = rawEvidencePreview(envelope.rawStdout, maxCharacters: 1600)
    return EvidenceRecordDetail(
      id: envelope.artifactKind.rawValue,
      title: envelope.task.rawValue,
      titleBadge: EvidenceBadge(
        text: envelope.freshness.rawValue,
        state: envelope.freshness == .current ? .trusted : .pending
      ),
      signatureBadge: EvidenceBadge(
        text: "exit \(envelope.exitCode)",
        state: envelope.exitCode == 0 ? .valid : .warning
      ),
      iconSystemName: "curlybraces",
      facts: [
        .init(id: "artifact", label: "Artifact", value: envelope.artifactKind.title),
        .init(id: "task", label: "Task", value: envelope.task.rawValue),
        .init(id: "loaded", label: "Loaded", value: dateTimeString(envelope.loadedAt)),
        .init(id: "context", label: "Context", value: envelope.contextSignature),
      ],
      tags: [envelope.artifactKind.rawValue, envelope.freshness.rawValue],
      summaryMetrics: [
        .init(id: "exit", label: "Exit", value: "\(envelope.exitCode)"),
        .init(
          id: "bytes",
          label: "Stdout size",
          value: "\(envelope.rawStdout.lengthOfBytes(using: .utf8)) B"
        ),
      ],
      signatureDetails: [
        .init(
          id: "stderr",
          label: "stderr sample",
          value: envelope.stderrSample.isEmpty ? "none" : envelope.stderrSample
        )
      ],
      verificationChecks: [
        .init(
          id: "freshness",
          label: "Freshness",
          value: envelope.freshness.rawValue,
          state: envelope.freshness == .current ? .passed : .pending
        )
      ],
      timeline: [],
      notes: preview.text
    )
  }

  @ViewBuilder
  private var artifactReaderProblems: some View {
    VStack(alignment: .leading, spacing: 8) {
      evidenceSectionLabel(localization.text("Artifact Reader Problems"))
      if store.artifactReadProblems.isEmpty {
        Text(localization.text("No app-side artifact decode problems in the current setup context."))
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
      } else {
        ForEach(store.artifactReadProblems.prefix(6)) { problem in
          VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 8) {
              EvidenceChip(
                label: "artifact",
                value: problem.artifactKind.title,
                tint: SMColor.amber
              )
              EvidenceChip(label: "task", value: problem.task.rawValue, tint: SMColor.blue)
              Text(problem.occurredAt, style: .time)
                .font(.system(size: 11, design: .monospaced))
                .foregroundStyle(SMColor.secondaryText)
            }
            Text(problem.problem)
              .font(.system(size: 12))
              .foregroundStyle(SMColor.primaryText)
              .textSelection(.enabled)
            if !problem.rawSample.isEmpty {
              Text(problem.rawSample)
                .font(.system(size: 11, design: .monospaced))
                .foregroundStyle(SMColor.secondaryText)
                .lineLimit(4)
                .textSelection(.enabled)
            }
          }
          .padding(10)
          .background(SMColor.input)
          .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
          .overlay(
            RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline)
          )
        }
      }
    }
  }

  @ViewBuilder
  private var appEventLog: some View {
    VStack(alignment: .leading, spacing: 8) {
      evidenceSectionLabel(localization.text("Structured App Events"))
      if store.appEvents.isEmpty {
        Text(localization.text("No structured app events yet."))
          .font(.system(size: 12))
          .foregroundStyle(SMColor.secondaryText)
      } else {
        ForEach(store.appEvents.prefix(8)) { event in
          HStack(alignment: .top, spacing: 8) {
            StatusDot(color: tint(for: event.severity.rawValue))
            Text(event.occurredAt, style: .time)
              .font(.system(size: 11, design: .monospaced))
              .foregroundStyle(SMColor.secondaryText)
            VStack(alignment: .leading, spacing: 3) {
              Text(event.title)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(SMColor.primaryText)
              Text(event.detail)
                .font(.system(size: 11))
                .foregroundStyle(SMColor.secondaryText)
                .fixedSize(horizontal: false, vertical: true)
            }
          }
        }
      }
    }
  }

  @ViewBuilder
  private var evidenceSnapshot: some View {
    VStack(alignment: .leading, spacing: 10) {
      evidenceSectionLabel(localization.text("Evidence Cards"))
      LazyVGrid(columns: [GridItem(.adaptive(minimum: 280), spacing: 12)], spacing: 12) {
        ForEach(evidenceCards) { card in
          evidenceCardView(card)
        }
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var evidenceCards: [EvidenceCard] {
    EvidenceVaultBuilder(
      verify: store.verifySnapshot,
      sourceConsistency: store.effectiveSourceConsistencySnapshot,
      status: store.statusSnapshot,
      report: store.reportSnapshot,
      health: store.healthSnapshot,
      envelopes: store.evidenceEnvelopes
    ).cards()
  }

  @ViewBuilder
  private var alignmentScopePanel: some View {
    let availability = currentSourceAvailability
    VStack(alignment: .leading, spacing: 10) {
      evidenceSectionLabel(localization.text("Alignment Scope"))
      LazyVGrid(columns: [GridItem(.adaptive(minimum: 260), spacing: 12)], spacing: 12) {
        evidenceMetricTile("comparison", value: "target vs manifest", tint: SMColor.blue)
        evidenceMetricTile(
          "current source",
          value: availability.label,
          tint: availability.label == "available" ? SMColor.green : SMColor.amber
        )
        evidenceMetricTile("Merkle/root proof", value: "unavailable", tint: SMColor.amber)
      }
      VStack(alignment: .leading, spacing: 6) {
        if let verify = store.verifySnapshot {
          evidenceLine(
            "manifest",
            verify.summary.manifest_count == 0 ? "none" : verify.manifest.manifestID
          )
          let sessionID = verify.session_id?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
          evidenceLine("session", sessionID.isEmpty ? "none" : sessionID)
          evidenceLine("target root", verify.target_root)
          evidenceLine(
            "file check",
            "\(verify.summary.files_verified) verified / \(verify.summary.files_expected) expected"
          )
          evidenceLine("source compare", availability.detail)
          evidenceLine("root proof", verify.merkleRootProof.detail)
        } else {
          evidenceLine("verify evidence", "not checked")
          evidenceLine("source compare", availability.detail)
          evidenceLine(
            "root proof",
            "Merkle/root proof is unavailable because no Merkle tree or content-root artifact is wired."
          )
        }
      }
      .padding(12)
      .background(SMColor.input)
      .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
      .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
    }
  }

  private var currentSourceAvailability: EvidenceAvailability {
    guard let consistency = store.effectiveSourceConsistencySnapshot else {
      return .unavailable(
        "Current source proof has not been captured for the current app-first acceptance transfer context."
      )
    }
    let status = consistency.status.trimmingCharacters(in: .whitespacesAndNewlines)
    let mode = consistency.mode.trimmingCharacters(in: .whitespacesAndNewlines)
    let detail =
      consistency.detail?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty == false
      ? consistency.detail!.trimmingCharacters(in: .whitespacesAndNewlines)
      : "Current source proof is present, but no detail string was emitted."
    if status == "pass" && mode == "current_source_verified" {
      return .available(detail)
    }
    return .unavailable(detail)
  }

  private func evidenceCardView(_ card: EvidenceCard) -> some View {
    VStack(alignment: .leading, spacing: 10) {
      HStack(alignment: .firstTextBaseline) {
        StatusDot(color: evidenceSeverityTint(card.severity))
        Text(card.title)
          .font(.system(size: 14, weight: .semibold))
          .foregroundStyle(SMColor.primaryText)
        Spacer()
        EvidenceChip(
          label: "state",
          value: card.status,
          tint: evidenceSeverityTint(card.severity)
        )
      }
      Text(card.detail)
        .font(.caption)
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
      ForEach(card.displayFacts(maxCount: 7)) { fact in
        HStack(alignment: .firstTextBaseline, spacing: 8) {
          Text(fact.label)
            .font(.caption2.weight(.semibold))
            .foregroundStyle(SMColor.secondaryText)
            .frame(width: 112, alignment: .leading)
          Text(fact.value)
            .font(.caption2.monospaced())
            .foregroundStyle(evidenceSeverityTint(fact.severity))
            .textSelection(.enabled)
        }
      }
      let hidden = card.hiddenFactCount(maxCount: 7)
      if hidden > 0 {
        Text("\(hidden) more facts retained in raw evidence.")
          .font(.caption2)
          .foregroundStyle(SMColor.secondaryText)
      }
      if let action = card.nextAction {
        EvidenceChip(label: "vault action", value: action.label, tint: SMColor.blue)
      }
    }
    .padding(13)
    .frame(maxWidth: .infinity, alignment: .topLeading)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
  }

  @ViewBuilder
  private var evidenceArtifactCatalogPanel: some View {
    VStack(alignment: .leading, spacing: 10) {
      HStack {
        evidenceSectionLabel(localization.text("Artifact Catalog"))
        Spacer()
        ActionButton(localization.text("Refresh"), systemImage: "arrow.clockwise") {
          store.refreshEvidenceArtifactCatalog()
        }
      }
      Text(
        localization.text("Manual read of the selected Target Root field, not a config-derived target proof. Symlinks, malformed JSON, and unknown `.supermover` control artifacts stay visible as review evidence.")
      )
      .font(.caption)
      .foregroundStyle(SMColor.secondaryText)
      HStack(spacing: 10) {
        WorkbenchSearchField(
          text: viewState.searchQuery,
          placeholder: localization.text("Search path, id, family, preview, or problem text"),
          onChange: { viewState.searchQuery = $0 }
        )
        Picker(localization.text("Family"), selection: $viewState.artifactFamilyFilterID) {
          Text(localization.text("All families")).tag("all")
          ForEach(evidenceArtifactFamilyOptions) { family in
            Text(family.title).tag(family.rawValue)
          }
        }
        .labelsHidden()
        .frame(width: 210)
      }
      if let catalog = store.evidenceArtifactCatalog {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 210), spacing: 10)], spacing: 10) {
          evidenceMetricTile(
            "cataloged artifacts",
            value: "\(catalog.artifacts.count)",
            tint: SMColor.blue
          )
          evidenceMetricTile(
            "catalog problems",
            value: "\(catalog.problems.count)",
            tint: catalog.hasProblems ? SMColor.amber : SMColor.green
          )
          evidenceMetricTile(
            "visible families",
            value: "\(catalog.familiesWithArtifacts.count)",
            tint: SMColor.cyan
          )
        }
        if !catalog.problems.isEmpty {
          artifactCatalogProblemList(catalog.problems)
        }
        let artifacts = filteredEvidenceArtifacts
        if artifacts.isEmpty {
          Text(localization.text("No artifacts match the current filter."))
            .font(.caption)
            .foregroundStyle(SMColor.secondaryText)
        } else {
          ForEach(artifacts.prefix(12)) { artifact in
            artifactCatalogRow(artifact, problems: catalog.problems(for: artifact))
          }
          if artifacts.count > 12 {
            Text(
              "\(artifacts.count - 12) \(localization.text("more artifacts match. Narrow the query or family filter to inspect them."))"
            )
            .font(.caption2)
            .foregroundStyle(SMColor.secondaryText)
          }
        }
      } else {
        Text(localization.text("Artifact catalog not loaded. Select a target root, then click Artifacts or Refresh."))
          .font(.caption)
          .foregroundStyle(SMColor.secondaryText)
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private var evidenceArtifactFamilyOptions: [EvidenceArtifactFamily] {
    store.evidenceArtifactCatalog?.familiesWithArtifacts ?? EvidenceArtifactFamily.allCases
  }

  private var selectedEvidenceArtifactFamilies: Set<EvidenceArtifactFamily> {
    guard let family = EvidenceArtifactFamily(rawValue: viewState.artifactFamilyFilterID) else {
      return []
    }
    return [family]
  }

  private var filteredEvidenceArtifacts: [EvidenceArtifactRecord] {
    store.evidenceArtifactCatalog?.filtered(
      families: selectedEvidenceArtifactFamilies,
      query: viewState.searchQuery
    ) ?? []
  }

  private func artifactCatalogProblemList(_ problems: [EvidenceArtifactCatalogProblem]) -> some View {
    VStack(alignment: .leading, spacing: 6) {
      Text(localization.text("Catalog Problems"))
        .font(.caption.weight(.semibold))
        .foregroundStyle(SMColor.primaryText)
      ForEach(problems.prefix(5)) { problem in
        HStack(alignment: .top, spacing: 8) {
          StatusDot(color: artifactSeverityTint(problem.severity))
          VStack(alignment: .leading, spacing: 2) {
            Text(problem.relativePath)
              .font(.caption2.monospaced())
              .foregroundStyle(SMColor.primaryText)
              .textSelection(.enabled)
            Text(problem.message)
              .font(.caption2)
              .foregroundStyle(SMColor.secondaryText)
          }
        }
      }
      if problems.count > 5 {
        Text("\(problems.count - 5) more catalog problems retained in the filterable artifact list.")
          .font(.caption2)
          .foregroundStyle(SMColor.secondaryText)
      }
    }
    .padding(12)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
  }

  private func artifactCatalogRow(
    _ artifact: EvidenceArtifactRecord,
    problems: [EvidenceArtifactCatalogProblem]
  ) -> some View {
    VStack(alignment: .leading, spacing: 8) {
      HStack(spacing: 8) {
        EvidenceChip(
          label: "family",
          value: artifact.family.title,
          tint: artifactSeverityTint(artifact.issueSeverity)
        )
        EvidenceChip(
          label: "json",
          value: artifactJSONStatusLabel(artifact.jsonStatus),
          tint: artifactJSONStatusTint(artifact.jsonStatus)
        )
        if let size = artifact.size {
          EvidenceChip(
            label: "size",
            value: formattedArtifactSize(size),
            tint: SMColor.secondaryText
          )
        }
        Spacer()
        if let modifiedAt = artifact.modifiedAt {
          Text(modifiedAt, style: .time)
            .font(.caption2.monospaced())
            .foregroundStyle(SMColor.secondaryText)
        }
      }
      Text(artifact.relativePath)
        .font(.caption2.monospaced())
        .foregroundStyle(SMColor.primaryText)
        .textSelection(.enabled)
      if let artifactID = artifact.artifactID, !artifactID.isEmpty {
        evidenceLine("id", artifactID)
      }
      if !artifact.previewText.isEmpty {
        Text(artifact.previewText)
          .font(.caption2.monospaced())
          .foregroundStyle(SMColor.secondaryText)
          .lineLimit(3)
          .textSelection(.enabled)
      }
      ForEach(problems.prefix(2)) { problem in
        evidenceLine(artifactProblemKindLabel(problem.kind), problem.message)
      }
    }
    .padding(12)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
  }

  @ViewBuilder
  private var rawEvidenceSurfaces: some View {
    VStack(alignment: .leading, spacing: 10) {
      evidenceSectionLabel(localization.text("Raw JSON Envelopes"))
      if rawEvidenceEnvelopes.isEmpty {
        Text(
          "No structured command stdout has been retained yet. Run a JSON-backed task to populate raw evidence."
        )
        .font(.caption)
        .foregroundStyle(SMColor.secondaryText)
      } else {
        ForEach(rawEvidenceEnvelopes.prefix(8)) { envelope in
          rawEvidenceEnvelopeView(envelope)
        }
      }
    }
  }

  private var rawEvidenceEnvelopes: [StructuredEvidenceEnvelope] {
    store.evidenceEnvelopeHistory.sorted { lhs, rhs in
      if lhs.freshness != rhs.freshness {
        return lhs.freshness == .current
      }
      return lhs.loadedAt > rhs.loadedAt
    }
  }

  private func rawEvidenceEnvelopeView(_ envelope: StructuredEvidenceEnvelope) -> some View {
    let preview = rawEvidencePreview(envelope.rawStdout)
    return VStack(alignment: .leading, spacing: 8) {
      HStack(spacing: 8) {
        EvidenceChip(
          label: "artifact",
          value: envelope.artifactKind.title,
          tint: envelope.freshness == .current ? SMColor.blue : SMColor.amber
        )
        EvidenceChip(label: "task", value: envelope.task.rawValue, tint: SMColor.secondaryText)
        EvidenceChip(
          label: "exit",
          value: "\(envelope.exitCode)",
          tint: envelope.exitCode == 0 ? SMColor.green : SMColor.amber
        )
        EvidenceChip(
          label: "freshness",
          value: envelope.freshness.rawValue,
          tint: envelope.freshness == .current ? SMColor.green : SMColor.amber
        )
        EvidenceChip(
          label: "raw",
          value: preview.summary,
          tint: preview.truncated ? SMColor.amber : SMColor.secondaryText
        )
        Spacer()
        Text(envelope.loadedAt, style: .time)
          .font(.caption2.monospaced())
          .foregroundStyle(SMColor.secondaryText)
      }
      ScrollView {
        Text(preview.text)
          .font(.caption2.monospaced())
          .foregroundStyle(SMColor.primaryText)
          .frame(maxWidth: .infinity, alignment: .leading)
          .textSelection(.enabled)
      }
      .frame(maxHeight: 180)
      if !envelope.stderrSample.isEmpty {
        Text(envelope.stderrSample)
          .font(.caption2.monospaced())
          .foregroundStyle(SMColor.secondaryText)
          .lineLimit(3)
          .textSelection(.enabled)
      }
    }
    .padding(12)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
  }

  @ViewBuilder
  private var nextActionPreviewPanel: some View {
    VStack(alignment: .leading, spacing: 10) {
      evidenceSectionLabel(localization.text("Evidence-Bound Next Actions"))
      Text(
        "The vault can run only review-metadata commands whose IDs resolve from loaded evidence. Target-content mutation, transfer, pairing, publish, prune apply, and reconcile apply remain excluded here."
      )
      .font(.caption)
      .foregroundStyle(SMColor.secondaryText)
      LazyVGrid(columns: [GridItem(.adaptive(minimum: 290), spacing: 12)], spacing: 12) {
        ForEach(nextActionPreviews) { action in
          nextActionPreviewCard(action)
        }
      }
    }
  }

  private var nextActionPreviews: [EvidenceNextAction] {
    let previewKinds: [EvidenceNextAction.Kind] = [
      .driftRecord,
      .driftAcknowledge,
      .driftResolve,
      .driftExpire,
      .syncQueueCancel,
      .syncQueueFail,
      .pruneApprove,
      .pruneSupersede,
      .reconcileApply,
    ]
    return store.evidenceNextActions(for: previewKinds)
  }

  private func nextActionPreviewCard(_ action: EvidenceNextAction) -> some View {
    VStack(alignment: .leading, spacing: 9) {
      HStack {
        StatusDot(color: nextActionTint(action))
        Text(action.title)
          .font(.caption.weight(.semibold))
          .foregroundStyle(SMColor.primaryText)
        Spacer()
        EvidenceChip(
          label: "safety",
          value: nextActionSafetyLabel(action),
          tint: nextActionTint(action)
        )
      }
      Text(action.summary)
        .font(.caption2)
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
      if let preview = action.commandPreview {
        Text(evidenceShellPreview([preview.executableName] + preview.arguments))
          .font(.caption2.monospaced())
          .foregroundStyle(SMColor.primaryText)
          .lineLimit(4)
          .textSelection(.enabled)
      } else if let disabled = action.disabledReason {
        Text(disabled)
          .font(.caption2)
          .foregroundStyle(SMColor.secondaryText)
          .fixedSize(horizontal: false, vertical: true)
      }
      Text(action.safety.explanation)
        .font(.caption2)
        .foregroundStyle(SMColor.secondaryText)
        .fixedSize(horizontal: false, vertical: true)
      if let task = executableTask(for: action.kind) {
        if action.allowsExecution, store.selectedRole.allows(task: task) {
          PrimaryActionButton(localization.text("Run Review Metadata"), systemImage: "play.fill") {
            store.runEvidenceReviewMetadataAction(action, task: task)
          }
        } else if action.allowsExecution {
          Text("\(store.selectedRole.localizedTitle(using: localization)) \(localization.text("role cannot run")) \(localization.text("this metadata action")).")
            .font(.caption2)
            .foregroundStyle(SMColor.secondaryText)
        }
      }
    }
    .padding(12)
    .frame(maxWidth: .infinity, alignment: .topLeading)
    .background(SMColor.input)
    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(SMColor.hairline))
  }

  private func executableTask(for kind: EvidenceNextAction.Kind) -> SuperMoverTaskKind? {
    switch kind {
    case .driftRecord:
      return .driftRecord
    case .driftAcknowledge:
      return .driftAcknowledge
    case .driftResolve:
      return .driftResolve
    case .driftExpire:
      return .driftExpire
    case .syncQueueCancel:
      return .syncQueueCancel
    case .syncQueueFail:
      return .syncQueueFail
    case .pruneApprove:
      return .pruneApprove
    case .pruneSupersede:
      return .pruneSupersede
    default:
      return nil
    }
  }

  private var evidenceUpdatedAtLabel: String {
    if let envelope = rawEvidenceEnvelopes.first {
      return timeAgoString(envelope.loadedAt)
    }
    if let run = store.recentRuns.first(where: { store.isCurrentContext($0) }) {
      return timeAgoString(run.launchedAt)
    }
    return "not loaded"
  }

  private var selectedEvidenceArtifact: EvidenceArtifactRecord? {
    let artifacts = Array(filteredEvidenceArtifacts.prefix(8))
    guard !artifacts.isEmpty else {
      return nil
    }
    if let selectedRecordID = viewState.selectedRecordID {
      return artifacts.first { ($0.artifactID ?? $0.relativePath) == selectedRecordID }
    }
    return artifacts.first
  }

  private var selectedEvidenceEnvelope: StructuredEvidenceEnvelope? {
    let envelopes = Array(rawEvidenceEnvelopes.prefix(8))
    guard !envelopes.isEmpty else {
      return nil
    }
    if let selectedRecordID = viewState.selectedRecordID {
      return envelopes.enumerated().first { index, envelope in
        "\(envelope.artifactKind.rawValue)-\(index)" == selectedRecordID
      }?.element
    }
    return envelopes.first
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

  private var sourceTitle: String {
    if let profileID = store.statusSnapshot?.profile_id {
      return profileID
    }
    if let profileID = store.reportSnapshot?.profile_id {
      return profileID
    }
    return "Source Config"
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

  private var verificationStatus: String {
    if let verify = store.verifySnapshot {
      return verify.statusLabel
    }
    return store.statusSnapshot?.overall.status ?? store.reportSnapshot?.overall.status
      ?? "not checked"
  }

  private var warningMetricValue: String {
    countMetricValue(warningCountEvidence)
  }

  private var artifactProblemMetricValue: String {
    countMetricValue(artifactProblemCountEvidence)
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

  private var aggregateEvidenceGateState: GateState {
    evidenceGateEvaluation.aggregateEvidenceState.gateState
  }

  private var hasLoadedEvidence: Bool {
    evidenceGateEvaluation.hasAnyEvidence
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

  private func countMetricValue(_ count: Int?) -> String {
    guard let count else {
      return "not checked"
    }
    return "\(count)"
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

  private func run(_ task: SuperMoverTaskKind) {
    store.selectedTask = task
    store.runSelectedTask()
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

  private func evidenceSemanticState(
    for severity: EvidenceArtifactIssueSeverity
  ) -> EvidenceSemanticState {
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

  private func evidenceSeverityTint(_ severity: EvidenceSeverity) -> Color {
    switch severity {
    case .ok:
      return SMColor.green
    case .unavailable, .warning, .review:
      return SMColor.amber
    case .critical:
      return SMColor.red
    }
  }

  private func artifactSeverityTint(_ severity: EvidenceArtifactIssueSeverity) -> Color {
    switch severity {
    case .ok:
      return SMColor.green
    case .warning:
      return SMColor.amber
    case .critical:
      return SMColor.red
    }
  }

  private func artifactJSONStatusTint(_ status: EvidenceArtifactJSONStatus) -> Color {
    switch status {
    case .valid:
      return SMColor.green
    case .notJSON, .notCheckedLargeArtifact:
      return SMColor.secondaryText
    case .malformed, .unreadable, .symlink:
      return SMColor.red
    }
  }

  private func artifactJSONStatusLabel(_ status: EvidenceArtifactJSONStatus) -> String {
    switch status {
    case .valid:
      return "valid"
    case .malformed:
      return "malformed"
    case .notJSON:
      return "not JSON"
    case .notCheckedLargeArtifact:
      return "large, not checked"
    case .unreadable:
      return "unreadable"
    case .symlink:
      return "symlink refused"
    }
  }

  private func artifactJSONSeverityLabel(_ severity: EvidenceArtifactIssueSeverity) -> String {
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

  private func artifactProblemKindLabel(_ kind: EvidenceArtifactCatalogProblem.Kind) -> String {
    switch kind {
    case .unsafePath:
      return "unsafe path"
    case .symlink:
      return "symlink refused"
    case .malformedJSON:
      return "malformed JSON"
    case .readError:
      return "read error"
    case .unsupportedFileType:
      return "unsupported file"
    }
  }

  private func formattedArtifactSize(_ size: Int64) -> String {
    if size < 1024 {
      return "\(size) B"
    }
    if size < 1024 * 1024 {
      return "\(size / 1024) KiB"
    }
    return "\(size / (1024 * 1024)) MiB"
  }

  private func nextActionTint(_ action: EvidenceNextAction) -> Color {
    if action.allowsExecution {
      return SMColor.green
    }
    switch action.safety.boundary {
    case .firstSliceMetadataPreview:
      return action.commandPreview == nil ? SMColor.amber : SMColor.blue
    case .plannedPreviewDisabled:
      return SMColor.amber
    case .excludedFromFirstSlice:
      return SMColor.secondaryText
    }
  }

  private func nextActionSafetyLabel(_ action: EvidenceNextAction) -> String {
    if action.allowsExecution {
      return "ready metadata"
    }
    switch action.safety.boundary {
    case .firstSliceMetadataPreview:
      return action.commandPreview == nil ? "missing evidence" : "preview only"
    case .plannedPreviewDisabled:
      return "planned, disabled"
    case .excludedFromFirstSlice:
      return "excluded"
    }
  }

  private func evidenceSectionLabel(_ title: String) -> some View {
    Text(title)
      .font(.system(size: 11, weight: .semibold))
      .foregroundStyle(SMColor.secondaryText)
  }

  private func evidenceMetricTile(_ label: String, value: String, tint: Color) -> some View {
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

  private func evidenceShellPreview(_ parts: [String]) -> String {
    let safeScalars = CharacterSet(
      charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_./:-=@%+,"
    )
    return parts.map { part in
      if !part.isEmpty, part.unicodeScalars.allSatisfy({ safeScalars.contains($0) }) {
        return part
      }
      return "'" + part.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }
    .joined(separator: " ")
  }

  private func rawEvidencePreview(
    _ raw: String,
    maxCharacters: Int = 4000
  ) -> EvidenceRawPreview {
    let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else {
      return EvidenceRawPreview(text: "No stdout retained.", summary: "0 bytes", truncated: false)
    }
    let pretty = prettyPrintedJSON(trimmed) ?? trimmed
    let truncated = pretty.count > maxCharacters
    let text =
      truncated
      ? String(pretty.prefix(maxCharacters))
        + "\n... truncated; inspect the CLI run output or target artifact for the full JSON."
      : pretty
    let bytes = raw.lengthOfBytes(using: .utf8)
    let lines = max(1, pretty.components(separatedBy: .newlines).count)
    let summary = "\(bytes) bytes / \(lines) lines" + (truncated ? " / truncated" : "")
    return EvidenceRawPreview(text: text, summary: summary, truncated: truncated)
  }

  private func prettyPrintedJSON(_ text: String) -> String? {
    guard let data = text.data(using: .utf8),
      let object = try? JSONSerialization.jsonObject(with: data),
      JSONSerialization.isValidJSONObject(object),
      let prettyData = try? JSONSerialization.data(
        withJSONObject: object,
        options: [.prettyPrinted, .sortedKeys]
      )
    else {
      return nil
    }
    return String(data: prettyData, encoding: .utf8)
  }

  private func timeAgoString(_ date: Date) -> String {
    let formatter = RelativeDateTimeFormatter()
    formatter.unitsStyle = .short
    return formatter.localizedString(for: date, relativeTo: Date())
  }

  private func dateTimeString(_ date: Date) -> String {
    date.formatted(date: .abbreviated, time: .shortened)
  }
}

private struct EvidenceRawPreview {
  let text: String
  let summary: String
  let truncated: Bool
}

#Preview {
  let previewLocalization = AppChromeLocalization(language: .english)
  EvidenceSectionView(
    model: EvidenceSectionModel(
      updatedAt: "2 min ago",
      stageFilter: EvidenceFilterState(
        selectedID: "all",
        options: [
          EvidenceFilterOption(id: "all", title: "All Stages"),
          EvidenceFilterOption(id: "transfer", title: "Transfer")
        ]
      ),
      typeFilter: EvidenceFilterState(
        selectedID: "all",
        options: [
          EvidenceFilterOption(id: "all", title: "All Types"),
          EvidenceFilterOption(id: "receipt", title: "Transfer Receipt")
        ]
      ),
      statusFilter: EvidenceFilterState(
        selectedID: "all",
        options: [
          EvidenceFilterOption(id: "all", title: "All Statuses"),
          EvidenceFilterOption(id: "sealed", title: "Sealed")
        ]
      ),
      searchQuery: "",
      records: [
        EvidenceRecord(
          id: "EV-2025-05-29-0008",
          type: "Transfer Run",
          stage: "Transfer",
          source: "MacBook Pro (10.0.0.12)",
          target: "Studio Storage (10.0.0.20)",
          created: "May 29, 2025, 2:46 PM",
          status: EvidenceBadge(text: "Sealed", state: .sealed),
          signature: EvidenceBadge(text: "Valid", state: .valid),
          size: "156 KB",
          iconSystemName: "doc.text",
          accent: .info
        ),
        EvidenceRecord(
          id: "EV-2025-05-29-0007",
          type: "Transfer Receipt",
          stage: "Transfer",
          source: "MacBook Pro (10.0.0.12)",
          target: "Studio Storage (10.0.0.20)",
          created: "May 29, 2025, 2:46 PM",
          status: EvidenceBadge(text: "Sealed", state: .sealed),
          signature: EvidenceBadge(text: "Valid", state: .valid),
          size: "98 KB",
          iconSystemName: "doc.text",
          accent: .info,
          isSelected: true
        ),
        EvidenceRecord(
          id: "EV-2025-05-29-0006",
          type: "Chunk Map",
          stage: "Transfer",
          source: "MacBook Pro (10.0.0.12)",
          target: "Studio Storage (10.0.0.20)",
          created: "May 29, 2025, 2:46 PM",
          status: EvidenceBadge(text: "Sealed", state: .sealed),
          signature: EvidenceBadge(text: "Valid", state: .valid),
          size: "512 KB",
          iconSystemName: "doc.plaintext",
          accent: .success
        ),
        EvidenceRecord(
          id: "EV-2025-05-29-0002",
          type: "Warning Summary",
          stage: "Verify",
          source: "MacBook Pro (10.0.0.12)",
          target: "Studio Storage (10.0.0.20)",
          created: "May 29, 2025, 2:57 PM",
          status: EvidenceBadge(text: "Durable", state: .durable),
          signature: EvidenceBadge(text: "Valid", state: .valid),
          size: "41 KB",
          iconSystemName: "doc.badge.exclamationmark",
          accent: .warning
        )
      ],
      selectionSummary: "1 of 8 selected",
      pagination: EvidencePagination(pageLabel: "1", canGoBackward: false, canGoForward: true),
      safetyPosture: EvidenceSafetyPosture(
        title: "Strong",
        state: .success,
        summary: "Config sealed. Evidence durable.",
        details: [
          "Config sealed",
          "Evidence durable"
        ],
        actionTitle: "View details ->"
      ),
      selectedDetail: EvidenceRecordDetail(
        id: "EV-2025-05-29-0007",
        title: "Transfer Receipt",
        titleBadge: EvidenceBadge(text: "Sealed", state: .sealed),
        signatureBadge: EvidenceBadge(text: "Valid", state: .valid),
        iconSystemName: "doc.text",
        facts: [
          EvidenceDetailFact(id: "stage", label: "Stage", value: "Transfer"),
          EvidenceDetailFact(id: "type", label: "Type", value: "Transfer Receipt"),
          EvidenceDetailFact(id: "created", label: "Created", value: "May 29, 2025, 2:46 PM"),
          EvidenceDetailFact(id: "source", label: "Source", value: "MacBook Pro (10.0.0.12)"),
          EvidenceDetailFact(id: "target", label: "Target", value: "Studio Storage (10.0.0.20)"),
          EvidenceDetailFact(id: "transport", label: "Transport", value: "LAN (10 GbE) - Jumbo frames"),
          EvidenceDetailFact(id: "engine", label: "Engine", value: "SuperMover Engine 1.2.3 (142)"),
          EvidenceDetailFact(id: "profile", label: "Config", value: "Baseline Migration Config (sealed)"),
          EvidenceDetailFact(id: "size", label: "Size", value: "98 KB")
        ],
        tags: ["production", "finance-data"],
        summaryMetrics: [
          EvidenceMetric(id: "files", label: "Files transferred", value: "128,420"),
          EvidenceMetric(id: "bytes", label: "Bytes transferred", value: "912.4 GB"),
          EvidenceMetric(id: "fingerprint", label: "Data fingerprint", value: "SHA-256"),
          EvidenceMetric(id: "sourcefp", label: "Fingerprint (source)", value: "9f3b6e43...a8d7c2e1"),
          EvidenceMetric(id: "targetfp", label: "Fingerprint (target)", value: "9f3b6e43...a8d7c2e1"),
          EvidenceMetric(id: "start", label: "Start time", value: "May 29, 2025, 2:45:58 PM"),
          EvidenceMetric(id: "end", label: "End time", value: "May 29, 2025, 2:46:35 PM"),
          EvidenceMetric(id: "duration", label: "Duration", value: "00:00:37"),
          EvidenceMetric(id: "warnings", label: "Warnings", value: "2"),
          EvidenceMetric(id: "errors", label: "Errors", value: "0")
        ],
        signatureDetails: [
          EvidenceDetailFact(id: "signedBy", label: "Signed by", value: "SuperMover Engine"),
          EvidenceDetailFact(id: "key", label: "Key ID", value: "7F3A...C91D"),
          EvidenceDetailFact(id: "signature", label: "Signature", value: "3045022100F3A6...C91D2203B42..."),
          EvidenceDetailFact(id: "algorithm", label: "Algorithm", value: "ECDSA P-256 (SHA-256)"),
          EvidenceDetailFact(id: "signedAt", label: "Signed", value: "May 29, 2025, 2:46:35 PM")
        ],
        verificationChecks: [
          EvidenceVerificationCheck(id: "sig", label: "Signature valid", value: "Verified", state: .verified),
          EvidenceVerificationCheck(id: "cert", label: "Certificate trusted", value: "Trusted", state: .trusted),
          EvidenceVerificationCheck(id: "stamp", label: "Timestamp valid", value: "2:46 PM", state: .success),
          EvidenceVerificationCheck(id: "sealed", label: "Record sealed", value: "Sealed", state: .sealed),
          EvidenceVerificationCheck(id: "integrity", label: "Integrity verified", value: "Passed", state: .passed)
        ],
        timeline: [
          EvidenceTimelineEntry(id: "created", title: "Receipt created", timestamp: "2:45:58 PM", detail: "Transfer session opened and initial manifest hash captured.", state: .info),
          EvidenceTimelineEntry(id: "sealed", title: "Receipt sealed", timestamp: "2:46:35 PM", detail: "Final digest matched source and target before signature was attached.", state: .sealed),
          EvidenceTimelineEntry(id: "verified", title: "Integrity verified", timestamp: "2:46:35 PM", detail: "Verification chain completed with no unsigned segments.", state: .verified)
        ],
        notes: "Warnings were limited to optional ACL remaps; no file-content divergence was recorded."
      ),
      supportingContent: AnyView(
        LazyVGrid(
          columns: [GridItem(.adaptive(minimum: 260), alignment: .top)],
          alignment: .leading,
          spacing: 16
        ) {
          WorkbenchPanel(
            title: "Warning Summary",
            subtitle: "Durable follow-up items captured during verification."
          ) {
            VStack(alignment: .leading, spacing: 10) {
              EvidenceStatusBadge(badge: EvidenceBadge(text: "2 warnings", state: .warning), compact: false)
              EvidenceFactGrid(
                facts: [
                  EvidenceDetailFact(id: "acl", label: "ACL remaps", value: "2 paths need review"),
                  EvidenceDetailFact(id: "owner", label: "Owner mapping", value: "Deferred to operator profile"),
                  EvidenceDetailFact(id: "receipt", label: "Linked receipt", value: "EV-2025-05-29-0007")
                ]
              )
              ActionButton(previewLocalization.text("Open warning summary"), systemImage: "doc.badge.exclamationmark") {}
            }
          }

          WorkbenchPanel(
            title: "Migration Runbook",
            subtitle: "Operator-facing checklist for exporting or attesting this run."
          ) {
            VStack(alignment: .leading, spacing: 10) {
              EvidenceStatusBadge(badge: EvidenceBadge(text: "Ready", state: .success), compact: false)
              EvidenceFactGrid(
                facts: [
                  EvidenceDetailFact(id: "audit", label: "Audit package", value: "Complete"),
                  EvidenceDetailFact(id: "retention", label: "Retention policy", value: "90 days"),
                  EvidenceDetailFact(id: "handoff", label: "Handoff notes", value: "No unresolved blockers")
                ]
              )
              ActionButton(previewLocalization.text("Open runbook"), systemImage: "list.bullet.clipboard") {}
            }
          }
        }
      )
    ),
    localization: previewLocalization,
    actions: EvidenceSectionActions()
  )
  .padding()
  .background(SMColor.appBackground)
}
