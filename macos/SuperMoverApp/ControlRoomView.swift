import SwiftUI

struct ControlRoomStatusModel {
  struct Item: Identifiable {
    let id: String
    let icon: String
    let label: String
    let value: String
    let tint: Color
  }

  let items: [Item]
}

struct ControlRoomFocusModel {
  let value: String
  let label: String
  let detail: String
  let tint: Color
  let progress: Double
}

struct ControlRoomMetricStripModel {
  struct Metric: Identifiable {
    let id: String
    let icon: String
    let title: String
    let value: String
    let detail: String
    let tint: Color
    let progress: Double?
  }

  let metrics: [Metric]
}

struct ControlRoomContextTileModel: Identifiable {
  let id: String
  let title: String
  let value: String
  let detail: String
  let tint: Color
}

struct ControlRoomRecentRunModel: Identifiable {
  let id: UUID
  let title: String
  let launchedAt: Date
  let state: String
  let stateTint: Color
  let commandLine: String
}

struct ControlRoomView<
  MigrationSurface: View,
  SafetyGatesPanel: View,
  RecentRunsPanel: View,
  MetricStrip: View,
  StageRail: View,
  RunConsolePanel: View,
  ActionStrip: View,
  CLIPanel: View
>: View {
  @ViewBuilder let migrationSurface: MigrationSurface
  @ViewBuilder let safetyGatesPanel: SafetyGatesPanel
  @ViewBuilder let recentRunsPanel: RecentRunsPanel
  @ViewBuilder let metricStrip: MetricStrip
  @ViewBuilder let stageRail: StageRail
  @ViewBuilder let runConsolePanel: RunConsolePanel
  @ViewBuilder let actionStrip: ActionStrip
  @ViewBuilder let cliPanel: CLIPanel

  var body: some View {
    VStack(alignment: .leading, spacing: 18) {
      HStack(alignment: .top, spacing: 18) {
        migrationSurface
          .frame(maxWidth: .infinity, alignment: .leading)

        VStack(alignment: .leading, spacing: 18) {
          safetyGatesPanel
          recentRunsPanel
        }
        .frame(width: 344, alignment: .leading)
      }

      metricStrip

      HStack(alignment: .top, spacing: 18) {
        VStack(alignment: .leading, spacing: 18) {
          stageRail
          runConsolePanel
        }
        .frame(maxWidth: .infinity, alignment: .leading)

        VStack(alignment: .leading, spacing: 18) {
          actionStrip
          cliPanel
        }
        .frame(width: 344, alignment: .leading)
      }
    }
  }
}

struct ControlRoomStatusItem: View {
  let model: ControlRoomStatusModel.Item

  var body: some View {
    WorkbenchPageHeaderStatusItem(
      icon: model.icon,
      label: model.label,
      value: model.value,
      tint: model.tint
    )
  }
}

struct ControlRoomStatusPill: View {
  let icon: String
  let label: String
  let value: String
  let tint: Color

  var body: some View {
    WorkbenchPageHeaderValuePill(
      icon: icon,
      label: label,
      value: value,
      tint: tint
    )
  }
}

struct ControlRoomContextTile: View {
  let model: ControlRoomContextTileModel

  var body: some View {
    VStack(alignment: .leading, spacing: 8) {
      Text(model.title.uppercased())
        .font(.system(size: 10, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      Text(model.value)
        .font(.system(size: 12, weight: .semibold, design: .monospaced))
        .foregroundStyle(SMColor.primaryText)
        .lineLimit(2)
        .textSelection(.enabled)
      Text(model.detail)
        .font(.system(size: 11))
        .foregroundStyle(SMColor.secondaryText)
        .lineLimit(2)
        .fixedSize(horizontal: false, vertical: true)
      Capsule()
        .fill(model.tint.opacity(0.8))
        .frame(width: 28, height: 3)
    }
    .padding(14)
    .frame(maxWidth: .infinity, minHeight: 86, alignment: .topLeading)
    .controlRoomInsetSurface(cornerRadius: 12, fill: SMColor.cardElevated.opacity(0.72))
  }
}

struct ControlRoomInlineStat: View {
  let title: String
  let value: String
  let tint: Color

  var body: some View {
    VStack(alignment: .leading, spacing: 4) {
      Text(title.uppercased())
        .font(.system(size: 9, weight: .semibold))
        .foregroundStyle(SMColor.secondaryText)
      Text(value)
        .font(.system(size: 13, weight: .semibold))
        .foregroundStyle(tint)
        .lineLimit(1)
        .minimumScaleFactor(0.7)
        .monospacedDigit()
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }
}

struct ControlRoomMetricStripView: View {
  let model: ControlRoomMetricStripModel

  var body: some View {
    HStack(spacing: 0) {
      ForEach(Array(model.metrics.enumerated()), id: \.element.id) { index, metric in
        if index > 0 {
          Divider()
            .overlay(SMColor.divider.opacity(0.72))
        }
        ControlRoomMetricCell(model: metric)
      }
    }
    .controlRoomInsetSurface(cornerRadius: 16, fill: SMColor.cardElevated.opacity(0.96))
  }
}

struct ControlRoomMetricCell: View {
  let model: ControlRoomMetricStripModel.Metric

  var body: some View {
    VStack(alignment: .leading, spacing: 8) {
      HStack(spacing: 8) {
        Image(systemName: model.icon)
          .font(.system(size: 12, weight: .semibold))
          .foregroundStyle(model.tint)
        Text(model.title.uppercased())
          .font(.system(size: 10, weight: .semibold))
          .foregroundStyle(SMColor.secondaryText)
      }
      Text(model.value)
        .font(.system(size: 20, weight: .bold))
        .foregroundStyle(model.tint)
        .lineLimit(1)
        .minimumScaleFactor(0.72)
        .monospacedDigit()
      Text(model.detail)
        .font(.system(size: 11))
        .foregroundStyle(SMColor.secondaryText)
        .lineLimit(2)
        .fixedSize(horizontal: false, vertical: true)
      if let progress = model.progress {
        ProgressRail(progress: progress, tint: model.tint)
      }
    }
    .padding(.horizontal, 16)
    .padding(.vertical, 16)
    .frame(maxWidth: .infinity, minHeight: 104, alignment: .leading)
  }
}

private extension View {
  func controlRoomInsetSurface(cornerRadius: CGFloat, fill: Color) -> some View {
    self
      .background(
        RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
          .fill(fill)
      )
  }
}

struct ControlRoomRecentRunRow: View {
  let model: ControlRoomRecentRunModel

  var body: some View {
    HStack(alignment: .top, spacing: 10) {
      StatusDot(color: model.stateTint)
      VStack(alignment: .leading, spacing: 4) {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
          Text(model.title)
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(SMColor.primaryText)
            .lineLimit(1)
          Spacer(minLength: 8)
          Text(model.launchedAt, style: .time)
            .font(.system(size: 11, design: .monospaced))
            .foregroundStyle(SMColor.secondaryText)
        }
        Text(model.state)
          .font(.system(size: 11, weight: .medium))
          .foregroundStyle(model.stateTint)
        Text(model.commandLine)
          .font(.system(size: 11, design: .monospaced))
          .foregroundStyle(SMColor.secondaryText)
          .lineLimit(3)
          .textSelection(.enabled)
      }
    }
  }
}
