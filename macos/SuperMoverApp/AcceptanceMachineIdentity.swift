import CryptoKit
import Foundation
import IOKit
import SystemConfiguration

struct AcceptanceMachineIdentity: Equatable {
    let machineID: String
    let machineLabel: String?
}

struct AcceptanceMachineIdentityResolver {
    private let resolveCurrentMachine: () -> AcceptanceMachineIdentity

    init(
        platformUUIDProvider: @escaping () -> String? = Self.livePlatformUUID,
        localHostNameProvider: @escaping () -> String? = Self.liveLocalHostName,
        hostNameProvider: @escaping () -> String? = { ProcessInfo.processInfo.hostName }
    ) {
        resolveCurrentMachine = {
            Self.resolvedMachineIdentity(
                platformUUID: platformUUIDProvider(),
                localHostName: localHostNameProvider(),
                hostName: hostNameProvider()
            )
        }
    }

    init(resolveCurrentMachine: @escaping () -> AcceptanceMachineIdentity) {
        self.resolveCurrentMachine = resolveCurrentMachine
    }

    func resolve() -> AcceptanceMachineIdentity {
        resolveCurrentMachine()
    }

    static func resolvedMachineIdentity(
        platformUUID: String?,
        localHostName: String?,
        hostName: String?
    ) -> AcceptanceMachineIdentity {
        let cleanedPlatformUUID = cleaned(platformUUID)
        let cleanedHostName = cleaned(hostName)
        let machineID: String

        if let cleanedPlatformUUID {
            machineID = "macos-platformuuid-\(sha256Hex(cleanedPlatformUUID))"
        } else if let cleanedHostName {
            machineID = "macos-hostname-\(sha256Hex(cleanedHostName))"
        } else {
            machineID = "macos-machine-unknown"
        }

        return AcceptanceMachineIdentity(
            machineID: machineID,
            machineLabel: cleaned(localHostName) ?? cleanedHostName
        )
    }

    private static func cleaned(_ value: String?) -> String? {
        guard let value else {
            return nil
        }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    private static func sha256Hex(_ value: String) -> String {
        SHA256.hash(data: Data(value.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    private static func livePlatformUUID() -> String? {
        let service = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching("IOPlatformExpertDevice"))
        guard service != 0 else {
            return nil
        }
        defer {
            IOObjectRelease(service)
        }
        guard let unmanaged = IORegistryEntryCreateCFProperty(
            service,
            "IOPlatformUUID" as CFString,
            kCFAllocatorDefault,
            0
        ) else {
            return nil
        }
        return unmanaged.takeRetainedValue() as? String
    }

    private static func liveLocalHostName() -> String? {
        SCDynamicStoreCopyLocalHostName(nil) as String?
    }
}
