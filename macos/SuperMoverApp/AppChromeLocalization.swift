import Foundation

struct AppChromeLocalization {
    enum Key: String {
        case sidebarHomeTitle = "sidebar.home.title"
        case sidebarSetupTitle = "sidebar.setup.title"
        case sidebarDevicesTitle = "sidebar.devices.title"
        case sidebarTransferTitle = "sidebar.transfer.title"
        case sidebarVerificationTitle = "sidebar.verification.title"
        case sidebarEvidenceTitle = "sidebar.evidence.title"
        case sidebarTaskDispatchTitle = "sidebar.taskDispatch.title"
        case settingsTitle = "settings.title"
        case sidebarWorkflowGroupTitle = "sidebar.group.workflow.title"
        case sidebarEvidenceGroupTitle = "sidebar.group.evidence.title"
        case sidebarSystemGroupTitle = "sidebar.group.system.title"
        case settingsSubtitle = "settings.subtitle"
        case commandInputsTitle = "settings.commandInputs.title"
        case commandInputsSubtitle = "settings.commandInputs.subtitle"
        case displayPreferencesTitle = "settings.displayPreferences.title"
        case displayPreferencesSubtitle = "settings.displayPreferences.subtitle"
        case displayOnlyNoticeTitle = "settings.displayOnlyNotice.title"
        case displayOnlyNoticeDetail = "settings.displayOnlyNotice.detail"
        case appearancePickerTitle = "settings.appearance.title"
        case languagePickerTitle = "settings.language.title"
        case sidebarTagline = "sidebar.tagline"
        case sidebarWorkstationTitle = "sidebar.workstation.title"
        case sidebarRoleLabel = "sidebar.role.label"
        case sidebarConfigLabel = "sidebar.config.label"
        case sidebarSurfaceLabel = "sidebar.surface.label"
        case safetyPostureTitle = "sidebar.safety.title"
        case safetyConfigSSOT = "sidebar.safety.configSSOT"
        case safetyExplicitTargetMutations = "sidebar.safety.explicitTargetMutations"
        case safetyDurableEvidence = "sidebar.safety.durableEvidence"
        case appearanceSystemTitle = "preference.appearance.system"
        case appearanceLightTitle = "preference.appearance.light"
        case appearanceDarkTitle = "preference.appearance.dark"
        case languageSystemTitle = "preference.language.system"
        case languageEnglishTitle = "preference.language.english"
        case languageSimplifiedChineseTitle = "preference.language.simplifiedChinese"
        case setupHeaderTitle = "setup.header.title"
        case setupHeaderSubtitle = "setup.header.subtitle"
        case setupRoleCardTitle = "setup.role.card.title"
        case setupRoleCardSubtitle = "setup.role.card.subtitle"
        case setupRoleFieldTitle = "setup.role.field.title"
        case setupConfigCardTitle = "setup.config.card.title"
        case setupConfigCardSubtitle = "setup.config.card.subtitle"
        case setupConfigSelectedFileTitle = "setup.config.selectedFile.title"
        case setupRootInputsCardTitle = "setup.rootInputs.card.title"
        case setupRootInputsCardSubtitle = "setup.rootInputs.card.subtitle"
        case setupChecksCardTitle = "setup.checks.card.title"
        case setupChecksCardSubtitle = "setup.checks.card.subtitle"
        case setupSourceRootFieldTitle = "setup.sourceRoot.field.title"
        case setupSourceRootPlaceholder = "setup.sourceRoot.placeholder"
        case setupTargetRootFieldTitle = "setup.targetRoot.field.title"
        case setupTargetRootPlaceholder = "setup.targetRoot.placeholder"
        case setupObserverRootInputsNotice = "setup.observer.rootInputs.notice"
        case setupActionOpenExistingConfig = "setup.action.openExistingConfig"
        case setupActionCreateRecommendedConfig = "setup.action.createRecommendedConfig"
        case setupActionChooseCustomConfigLocation = "setup.action.chooseCustomConfigLocation"
        case setupActionClearConfig = "setup.action.clearConfig"
        case setupActionUpdateExistingConfigTarget = "setup.action.updateExistingConfigTarget"
        case setupActionCreateNewConfigFile = "setup.action.createNewConfigFile"
        case setupActionChooseFolder = "setup.action.chooseFolder"
        case setupActionBrowseSourceRoot = "setup.action.browseSourceRoot"
        case setupActionBrowseTargetRoot = "setup.action.browseTargetRoot"
        case setupActionLintExistingConfig = "setup.action.lintExistingConfig"
        case setupActionLintConfig = "setup.action.lintConfig"
        case setupActionReadStatus = "setup.action.readStatus"
        case setupProfileAdvancedTitle = "setup.profile.advanced.title"
        case setupProfileRawPathLabel = "setup.profile.rawPath.label"
        case setupProfileRawPathEmpty = "setup.profile.rawPath.empty"
        case setupProfileNoConfigTitle = "setup.profile.noConfig.title"
        case setupProfileNoConfigDetailSource = "setup.profile.noConfig.detail.source"
        case setupProfileNoConfigDetailExistingOnly = "setup.profile.noConfig.detail.existingOnly"
        case setupProfileRecommendedSourceConfigTitle = "setup.profile.recommendedSourceConfig.title"
        case setupProfileRecommendedConfigReadyMetadata = "setup.profile.recommendedConfig.readyMetadata"
        case setupProfileNewSourceConfigTitle = "setup.profile.newSourceConfig.title"
        case setupProfileNewSourceConfigDetail = "setup.profile.newSourceConfig.detail"
        case setupProfileNewConfigReadyMetadata = "setup.profile.newConfig.readyMetadata"
        case setupProfileSelectedConfigFileTitle = "setup.profile.selectedConfigFile.title"
        case setupProfileSelectedConfigTitle = "setup.profile.selectedConfig.title"
        case setupProfileSelectedFolderTitle = "setup.profile.selectedFolder.title"
        case setupProfileDirectoryMetadata = "setup.profile.directory.metadata"
        case setupProfileMissingMetadata = "setup.profile.missing.metadata"
        case setupChecklistTitle = "setup.checklist.title"
        case setupChecklistConfigFile = "setup.checklist.configFile"
        case setupChecklistLintLatestConfig = "setup.checklist.lintLatestConfig"
        case setupChecklistDetail = "setup.checklist.detail"
        case setupGuideSubtitle = "setup.guide.subtitle"
        case setupGuideSourceTitle = "setup.guide.source.title"
        case setupGuideTargetTitle = "setup.guide.target.title"
        case setupGuideObserverTitle = "setup.guide.observer.title"
        case setupConfigStepTitle = "setup.guide.config.title"
        case setupConfigDetailNoneSource = "setup.guide.config.detail.none.source"
        case setupConfigDetailNoneExistingOnly = "setup.guide.config.detail.none.existingOnly"
        case setupConfigDetailExistingFile = "setup.guide.config.detail.existingFile"
        case setupConfigDetailNewDestinationSource = "setup.guide.config.detail.newDestination.source"
        case setupConfigDetailNewDestinationExistingOnly = "setup.guide.config.detail.newDestination.existingOnly"
        case setupConfigDetailMissingFileSource = "setup.guide.config.detail.missingFile.source"
        case setupConfigDetailMissingFile = "setup.guide.config.detail.missingFile"
        case setupConfigDetailDirectory = "setup.guide.config.detail.directory"
        case setupFoldersTitleSource = "setup.guide.folders.title.source"
        case setupFoldersTitleTarget = "setup.guide.folders.title.target"
        case setupFoldersTitleObserver = "setup.guide.folders.title.observer"
        case setupFoldersDetailSource = "setup.guide.folders.detail.source"
        case setupFoldersDetailTarget = "setup.guide.folders.detail.target"
        case setupFoldersDetailObserver = "setup.guide.folders.detail.observer"
        case setupValidationTitle = "setup.guide.validation.title"
        case setupValidationDetailSource = "setup.guide.validation.detail.source"
        case setupValidationDetailTarget = "setup.guide.validation.detail.target"
        case setupValidationDetailObserver = "setup.guide.validation.detail.observer"
        case setupStatusNotSelected = "setup.status.notSelected"
        case setupStatusExistingConfigFile = "setup.status.existingConfigFile"
        case setupStatusNewConfigDestination = "setup.status.newConfigDestination"
        case setupStatusConfigFileMissing = "setup.status.configFileMissing"
        case setupStatusFolderSelected = "setup.status.folderSelected"
        case setupStatusOptionalCreatingUpdating = "setup.status.optionalCreatingUpdating"
        case setupStatusOptionalUpdating = "setup.status.optionalUpdating"
        case setupStatusNotRequired = "setup.status.notRequired"
        case setupStatusNotValidated = "setup.status.notValidated"
        case setupStatusLintPassed = "setup.status.lintPassed"
        case setupStatusStatusRead = "setup.status.statusRead"
        case setupStatusTargetPathSet = "setup.status.targetPathSet"
        case setupStatusTargetPathMissing = "setup.status.targetPathMissing"
        case setupDirectoryReadable = "setup.directory.readable"
        case setupDirectoryNotReadable = "setup.directory.notReadable"
        case setupDirectoryWritable = "setup.directory.writable"
        case setupDirectoryNotWritable = "setup.directory.notWritable"
        case setupDirectoryNotSelected = "setup.directory.notSelected"
        case setupRootStatusSourceLabel = "setup.rootStatus.source"
        case setupRootStatusTargetLabel = "setup.rootStatus.target"
        case workbenchRoleSourceTitle = "workbench.role.source.title"
        case workbenchRoleTargetTitle = "workbench.role.target.title"
        case workbenchRoleObserverTitle = "workbench.role.observer.title"
        case workbenchRoleSourceSummary = "workbench.role.source.summary"
        case workbenchRoleTargetSummary = "workbench.role.target.summary"
        case workbenchRoleObserverSummary = "workbench.role.observer.summary"
        case workbenchRoleSourceAllowedSetup = "workbench.role.source.allowedSetup"
        case workbenchRoleTargetAllowedSetup = "workbench.role.target.allowedSetup"
        case workbenchRoleObserverAllowedSetup = "workbench.role.observer.allowedSetup"
    }

    private let language: UILanguagePreference
    private let resourceDirectoryProviders: [() -> URL?]
    private let preferredLanguagesProvider: () -> [String]

    init(
        language: UILanguagePreference,
        resourceDirectoryProviders: [() -> URL?]? = nil,
        preferredLanguagesProvider: @escaping () -> [String] = { Locale.preferredLanguages }
    ) {
        self.language = language
        self.resourceDirectoryProviders = resourceDirectoryProviders ?? Self.defaultResourceDirectoryProviders()
        self.preferredLanguagesProvider = preferredLanguagesProvider
    }

    func text(_ key: Key) -> String {
        text(rawKey: key.rawValue, englishFallback: englishFallback(for: key))
    }

    func text(rawKey key: String, englishFallback: String) -> String {
        for languageCode in resolvedLanguageCodes() {
            if let localized = localizedString(forKey: key, languageCode: languageCode) {
                return localized
            }
        }
        return localizedString(forKey: key, languageCode: "en") ?? englishFallback
    }

    private func resolvedLanguageCodes() -> [String] {
        switch language {
        case .system:
            return preferredLanguagesProvider().flatMap(Self.supportedLanguageCodes)
        case .english:
            return ["en"]
        case .simplifiedChinese:
            return ["zh-Hans"]
        }
    }

    private static func supportedLanguageCodes(for preferredLanguage: String) -> [String] {
        let normalized = preferredLanguage.replacingOccurrences(of: "_", with: "-").lowercased()
        if normalized == "zh-hans" || normalized.hasPrefix("zh-hans-") || normalized.hasPrefix("zh-cn") || normalized.hasPrefix("zh-sg") {
            return ["zh-Hans"]
        }
        if normalized == "en" || normalized.hasPrefix("en-") {
            return ["en"]
        }
        return []
    }

    private func localizedString(forKey key: String, languageCode: String) -> String? {
        for resourceDirectoryProvider in resourceDirectoryProviders {
            guard let resourceDirectory = resourceDirectoryProvider() else {
                continue
            }
            if let localized = localizedString(forKey: key, languageCode: languageCode, in: resourceDirectory) {
                return localized
            }
        }
        return nil
    }

    private static func defaultResourceDirectoryProviders() -> [() -> URL?] {
        [
            { Bundle.main.resourceURL },
            {
                guard Bundle.main.bundleURL.pathExtension != "app" else {
                    return nil
                }
                return Bundle.module.bundleURL
            },
        ]
    }

    private func localizedString(forKey key: String, languageCode: String, in resourceDirectory: URL) -> String? {
        for candidateLanguageCode in [languageCode, languageCode.lowercased()] {
            let languageDirectoryURL = resourceDirectory.appendingPathComponent("\(candidateLanguageCode).lproj", isDirectory: true)
            guard let languageBundle = Bundle(url: languageDirectoryURL) else {
                continue
            }
            let value = languageBundle.localizedString(forKey: key, value: nil, table: "Localizable")
            if value != key {
                return value
            }
        }
        return nil
    }

    private func englishFallback(for key: Key) -> String {
        switch key {
        case .sidebarHomeTitle:
            return "Home"
        case .sidebarSetupTitle:
            return "Prepare"
        case .sidebarDevicesTitle:
            return "Connect"
        case .sidebarTransferTitle:
            return "Move"
        case .sidebarVerificationTitle:
            return "Verify & Repair"
        case .sidebarEvidenceTitle:
            return "Evidence Vault"
        case .sidebarTaskDispatchTitle:
            return "Task Dispatch"
        case .settingsTitle:
            return "Settings"
        case .sidebarWorkflowGroupTitle:
            return "Workflow"
        case .sidebarEvidenceGroupTitle:
            return "Evidence"
        case .sidebarSystemGroupTitle:
            return "System"
        case .settingsSubtitle:
            return "Choose display preferences and command inputs. Migration config files remain the source of truth."
        case .commandInputsTitle:
            return "Command Inputs"
        case .commandInputsSubtitle:
            return "These values build CLI commands. They do not override saved migration config policy."
        case .displayPreferencesTitle:
            return "Display Preferences"
        case .displayPreferencesSubtitle:
            return "Local language and appearance for this app window."
        case .displayOnlyNoticeTitle:
            return "Display only"
        case .displayOnlyNoticeDetail:
            return "Does not change command previews, migration config files, evidence bundles, or CLI output."
        case .appearancePickerTitle:
            return "Appearance"
        case .languagePickerTitle:
            return "Interface Language"
        case .sidebarTagline:
            return "Auditable migration"
        case .sidebarWorkstationTitle:
            return "Workstation"
        case .sidebarRoleLabel:
            return "Role"
        case .sidebarConfigLabel:
            return "Config"
        case .sidebarSurfaceLabel:
            return "Surface"
        case .safetyPostureTitle:
            return "Safety posture"
        case .safetyConfigSSOT:
            return "Config file SSOT"
        case .safetyExplicitTargetMutations:
            return "Explicit target mutations"
        case .safetyDurableEvidence:
            return "Durable evidence"
        case .appearanceSystemTitle:
            return "System"
        case .appearanceLightTitle:
            return "Light"
        case .appearanceDarkTitle:
            return "Dark"
        case .languageSystemTitle:
            return "System"
        case .languageEnglishTitle:
            return "English"
        case .languageSimplifiedChineseTitle:
            return "简体中文"
        case .setupHeaderTitle:
            return "Prepare"
        case .setupHeaderSubtitle:
            return "Prepare this Mac's role and migration config file before running explicit commands."
        case .setupRoleCardTitle:
            return "Role"
        case .setupRoleCardSubtitle:
            return "Choose what this Mac is doing before selecting config and folder evidence."
        case .setupRoleFieldTitle:
            return "Role"
        case .setupConfigCardTitle:
            return "Migration Setup"
        case .setupConfigCardSubtitle:
            return "SuperMover saves this setup in a recommended config file. Existing or custom files are advanced options."
        case .setupConfigSelectedFileTitle:
            return "Current Setup"
        case .setupRootInputsCardTitle:
            return "Choose Folders"
        case .setupRootInputsCardSubtitle:
            return "On Source, choose this Mac's folder. On Target, choose where files land."
        case .setupChecksCardTitle:
            return "Config Checks"
        case .setupChecksCardSubtitle:
            return "Validate the selected config before moving to Connect or Move."
        case .setupSourceRootFieldTitle:
            return "Folder to Move From This Mac"
        case .setupSourceRootPlaceholder:
            return "Choose the folder to move from this Mac"
        case .setupTargetRootFieldTitle:
            return "Destination on Target Mac"
        case .setupTargetRootPlaceholder:
            return "Enter the folder path on the target Mac"
        case .setupObserverRootInputsNotice:
            return "Observer mode reads existing config and target evidence. Folder authoring is disabled for this role."
        case .setupActionOpenExistingConfig:
            return "Open Existing Config"
        case .setupActionCreateRecommendedConfig:
            return "Create Migration Setup"
        case .setupActionChooseCustomConfigLocation:
            return "Choose Custom Location"
        case .setupActionClearConfig:
            return "Clear Config"
        case .setupActionUpdateExistingConfigTarget:
            return "Update Existing Config Target"
        case .setupActionCreateNewConfigFile:
            return "Create New Config File"
        case .setupActionChooseFolder:
            return "Choose Folder"
        case .setupActionBrowseSourceRoot:
            return "Choose Folder"
        case .setupActionBrowseTargetRoot:
            return "Choose Folder"
        case .setupActionLintExistingConfig:
            return "Lint Existing Config"
        case .setupActionLintConfig:
            return "Lint Config"
        case .setupActionReadStatus:
            return "Read Status"
        case .setupProfileAdvancedTitle:
            return "Advanced Options"
        case .setupProfileRawPathLabel:
            return "File location"
        case .setupProfileRawPathEmpty:
            return "No file location selected"
        case .setupProfileNoConfigTitle:
            return "Recommended setup"
        case .setupProfileNoConfigDetailSource:
            return "No file picking needed. Choose folders, then create the setup."
        case .setupProfileNoConfigDetailExistingOnly:
            return "Open an existing migration config file to load roots, pairing, network pins, and evidence links."
        case .setupProfileRecommendedSourceConfigTitle:
            return "Recommended setup ready"
        case .setupProfileRecommendedConfigReadyMetadata:
            return "Recommended location selected."
        case .setupProfileNewSourceConfigTitle:
            return "Custom setup location"
        case .setupProfileNewSourceConfigDetail:
            return "Choose folders, then create the setup."
        case .setupProfileNewConfigReadyMetadata:
            return "Ready to create through the selected file."
        case .setupProfileSelectedConfigFileTitle:
            return "Selected migration config file"
        case .setupProfileSelectedConfigTitle:
            return "Selected migration config"
        case .setupProfileSelectedFolderTitle:
            return "Selected folder"
        case .setupProfileDirectoryMetadata:
            return "Choose a .json migration config file."
        case .setupProfileMissingMetadata:
            return "Config file missing"
        case .setupChecklistTitle:
            return "Setup Checklist"
        case .setupChecklistConfigFile:
            return "Migration config file"
        case .setupChecklistLintLatestConfig:
            return "Lint latest selected config"
        case .setupChecklistDetail:
            return "These checks only cover config-file and path preparation. Live transfer state, evidence health, and supervised process status belong in Control Room and Evidence."
        case .setupGuideSubtitle:
            return "Pick a role, choose folders, then create or validate the setup."
        case .setupGuideSourceTitle:
            return "Prepare this Source"
        case .setupGuideTargetTitle:
            return "Prepare this Target"
        case .setupGuideObserverTitle:
            return "Prepare this Observer"
        case .setupConfigStepTitle:
            return "Migration config file"
        case .setupConfigDetailNoneSource:
            return "Choose folders, then create the recommended setup. Existing and custom config files live in Advanced."
        case .setupConfigDetailNoneExistingOnly:
            return "Open an existing migration config file before reading evidence or running role tasks."
        case .setupConfigDetailExistingFile:
            return "Existing migration config selected. It remains the source of truth for CLI execution."
        case .setupConfigDetailNewDestinationSource:
            return "New destination selected. Write the migration config through the CLI before other tasks use it."
        case .setupConfigDetailNewDestinationExistingOnly:
            return "Targets and observers need an existing migration config file."
        case .setupConfigDetailMissingFileSource:
            return "Selected config file is missing. Create the recommended setup instead, or open an existing file from Advanced."
        case .setupConfigDetailMissingFile:
            return "Selected config file is missing. Open an existing migration config file."
        case .setupConfigDetailDirectory:
            return "A folder is selected. Choose a .json migration config file."
        case .setupFoldersTitleSource:
            return "Choose folders"
        case .setupFoldersTitleTarget:
            return "Destination folder"
        case .setupFoldersTitleObserver:
            return "Evidence target"
        case .setupFoldersDetailSource:
            return "Choose this Mac's folder to move, then enter the destination path that the target Mac will own."
        case .setupFoldersDetailTarget:
            return "Use this folder only when explicitly updating the selected setup target. Lint and Status read the saved setup."
        case .setupFoldersDetailObserver:
            return "Observer mode reads existing target evidence; it does not author source or target folders."
        case .setupValidationTitle:
            return "Validate before moving"
        case .setupValidationDetailSource:
            return "Create or open the config, then run Lint Config before treating setup as ready."
        case .setupValidationDetailTarget:
            return "Run Lint Config or Read Status to confirm the selected config still matches durable evidence."
        case .setupValidationDetailObserver:
            return "Run Read Status to load evidence from the selected config without mutating target state."
        case .setupStatusNotSelected:
            return "not selected"
        case .setupStatusExistingConfigFile:
            return "Existing config file"
        case .setupStatusNewConfigDestination:
            return "New config destination"
        case .setupStatusConfigFileMissing:
            return "Config file missing"
        case .setupStatusFolderSelected:
            return "Folder selected"
        case .setupStatusOptionalCreatingUpdating:
            return "optional unless creating/updating"
        case .setupStatusOptionalUpdating:
            return "optional unless updating"
        case .setupStatusNotRequired:
            return "not required"
        case .setupStatusNotValidated:
            return "not validated"
        case .setupStatusLintPassed:
            return "lint passed"
        case .setupStatusStatusRead:
            return "status read"
        case .setupStatusTargetPathSet:
            return "target path set"
        case .setupStatusTargetPathMissing:
            return "target path missing"
        case .setupDirectoryReadable:
            return "readable"
        case .setupDirectoryNotReadable:
            return "not readable"
        case .setupDirectoryWritable:
            return "writable"
        case .setupDirectoryNotWritable:
            return "not writable"
        case .setupDirectoryNotSelected:
            return "not selected"
        case .setupRootStatusSourceLabel:
            return "source"
        case .setupRootStatusTargetLabel:
            return "target"
        case .workbenchRoleSourceTitle:
            return "Source"
        case .workbenchRoleTargetTitle:
            return "Target"
        case .workbenchRoleObserverTitle:
            return "Observer"
        case .workbenchRoleSourceSummary:
            return "Prepare source roots, config identity, pairing inputs, and bounded transfer inputs."
        case .workbenchRoleTargetSummary:
            return "Prepare target root, config evidence, listen inputs, and read-only evidence access."
        case .workbenchRoleObserverSummary:
            return "Inspect selected target evidence without mutation or long-running services."
        case .workbenchRoleSourceAllowedSetup:
            return "create config, lint config, update target, dry-run preparation"
        case .workbenchRoleTargetAllowedSetup:
            return "target root selection, config lint, listen readiness preparation"
        case .workbenchRoleObserverAllowedSetup:
            return "config selection, status/report/health evidence reads"
        }
    }
}
