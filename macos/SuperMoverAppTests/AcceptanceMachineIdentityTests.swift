import XCTest
@testable import SuperMoverApp

final class AcceptanceMachineIdentityTests: XCTestCase {
    func testResolvedMachineIdentityPrefersPlatformUUIDHashAndLocalHostName() {
        let identity = AcceptanceMachineIdentityResolver.resolvedMachineIdentity(
            platformUUID: "ABC-123",
            localHostName: "supermover-source",
            hostName: "fallback-host"
        )

        XCTAssertTrue(identity.machineID.hasPrefix("macos-platformuuid-"))
        XCTAssertEqual(identity.machineLabel, "supermover-source")
    }

    func testResolvedMachineIdentityFallsBackToHostNameHash() {
        let identity = AcceptanceMachineIdentityResolver.resolvedMachineIdentity(
            platformUUID: nil,
            localHostName: nil,
            hostName: "target-host"
        )

        XCTAssertTrue(identity.machineID.hasPrefix("macos-hostname-"))
        XCTAssertEqual(identity.machineLabel, "target-host")
    }

    func testResolvedMachineIdentityFallsBackToUnknownWhenNoInputsExist() {
        let identity = AcceptanceMachineIdentityResolver.resolvedMachineIdentity(
            platformUUID: nil,
            localHostName: nil,
            hostName: nil
        )

        XCTAssertEqual(identity.machineID, "macos-machine-unknown")
        XCTAssertNil(identity.machineLabel)
    }
}
