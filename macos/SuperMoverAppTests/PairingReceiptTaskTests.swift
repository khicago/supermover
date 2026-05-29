import XCTest
@testable import SuperMoverApp

final class PairingReceiptTaskTests: XCTestCase {
    func testPairBuildsOptionalReceiptExportArgument() {
        let input = TaskInput(
            profilePath: "/tmp/source.profile.json",
            sourceRootPath: "/tmp/source",
            targetRootPath: "/tmp/target",
            profileID: "profile-local",
            profileName: "Local profile",
            targetID: "",
            targetName: "",
            sessionID: "",
            sessionPrefix: "",
            queueEntryID: "",
            syncRetryBackoff: "1m",
            syncInterval: "1m",
            syncMaxRuns: "0",
            syncSettle: "250ms",
            syncMaxEvents: "0",
            syncDiscoveryListen: "0.0.0.0:39394",
            syncDiscoveryTimeout: "2s",
            listenAddress: "127.0.0.1:4000",
            pairingTargetAddress: "10.0.0.20:39395",
            pairingVerificationCode: "123456",
            pairingMethod: "sas",
            pairingTimeout: "7s",
            pairingReceipt: PairingReceiptDraft(exportTarget: " /tmp/exported-receipts ", importReceiptFile: ""),
            discoveryBrowseListen: "0.0.0.0:39394",
            discoveryBrowseTimeout: "3s",
            discoveryAdvertiseListen: "",
            discoveryAdvertiseDestination: "255.255.255.255:39394",
            discoveryAdvertiseDuration: "11s",
            discoveryAdvertiseInterval: "2s",
            driftIDsInput: "",
            approvalID: "",
            softDeleteIDsInput: "",
            expiresAt: "",
            reason: "",
            reviewer: ""
        )

        XCTAssertEqual(
            SuperMoverTaskKind.pair.buildArguments(using: input),
            [
                "pair",
                "--profile", "/tmp/source.profile.json",
                "--target", "10.0.0.20:39395",
                "--verification-code", "123456",
                "--method", "sas",
                "--timeout", "7s",
                "--receipt-out", "/tmp/exported-receipts",
            ]
        )
    }

    func testProfileAdoptPairingBuildsReceiptFileArgument() {
        let input = TaskInput(
            profilePath: "/tmp/target.profile.json",
            sourceRootPath: "",
            targetRootPath: "/tmp/target",
            profileID: "profile-local",
            profileName: "Local profile",
            targetID: "",
            targetName: "",
            sessionID: "",
            sessionPrefix: "",
            queueEntryID: "",
            syncRetryBackoff: "1m",
            syncInterval: "1m",
            syncMaxRuns: "0",
            syncSettle: "250ms",
            syncMaxEvents: "0",
            syncDiscoveryListen: "0.0.0.0:39394",
            syncDiscoveryTimeout: "2s",
            listenAddress: "127.0.0.1:4000",
            pairingTargetAddress: "",
            pairingVerificationCode: "",
            pairingMethod: "sas",
            pairingTimeout: "5s",
            pairingReceipt: PairingReceiptDraft(exportTarget: "", importReceiptFile: " /tmp/exported/pairing-1.json "),
            discoveryBrowseListen: "0.0.0.0:39394",
            discoveryBrowseTimeout: "2s",
            discoveryAdvertiseListen: "",
            discoveryAdvertiseDestination: "255.255.255.255:39394",
            discoveryAdvertiseDuration: "10s",
            discoveryAdvertiseInterval: "1s",
            driftIDsInput: "",
            approvalID: "",
            softDeleteIDsInput: "",
            expiresAt: "",
            reason: "",
            reviewer: ""
        )

        XCTAssertEqual(
            SuperMoverTaskKind.profileAdoptPairing.buildArguments(using: input),
            [
                "profile", "adopt-pairing",
                "--profile", "/tmp/target.profile.json",
                "--receipt-file", "/tmp/exported/pairing-1.json",
            ]
        )
    }

    func testProfileAdoptPairingIsRoleGatedToTarget() {
        XCTAssertFalse(WorkbenchRole.source.allows(task: .profileAdoptPairing))
        XCTAssertTrue(WorkbenchRole.target.allows(task: .profileAdoptPairing))
        XCTAssertFalse(WorkbenchRole.observer.allows(task: .profileAdoptPairing))
    }

    @MainActor
    func testPairingReceiptContextTracksExportAndImportInputs() {
        let store = AppStore()
        store.profilePath = "/tmp/profile.json"

        let firstPair = store.currentContextSignature(for: .pair)
        store.pairingReceipt.exportTarget = "/tmp/exported-receipts"
        XCTAssertNotEqual(firstPair, store.currentContextSignature(for: .pair))

        let firstAdopt = store.currentContextSignature(for: .profileAdoptPairing)
        store.pairingReceipt.importReceiptFile = "/tmp/exported-receipts/pairing-1.json"
        XCTAssertNotEqual(firstAdopt, store.currentContextSignature(for: .profileAdoptPairing))
    }

    func testPairingReceiptDraftBuildsOptionalArguments() {
        var draft = PairingReceiptDraft()
        XCTAssertEqual(draft.exportArguments, [])
        XCTAssertEqual(draft.adoptArguments, [])

        draft.exportTarget = " /tmp/out "
        XCTAssertEqual(draft.exportArguments, ["--receipt-out", "/tmp/out"])

        draft.importReceiptFile = " /tmp/pairing.json "
        XCTAssertEqual(draft.adoptArguments, ["--receipt-file", "/tmp/pairing.json"])
    }
}
