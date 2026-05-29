import XCTest
@testable import SuperMoverApp

final class ProfileNetworkTaskTests: XCTestCase {
    func testProfileSetNetworkBuildsReviewedProfileArguments() {
        let draft = ProfileNetworkDraft(
            receiverURL: " https://127.0.0.1:9443 ",
            tlsCertificatePath: " /tmp/device.crt ",
            tlsPrivateKeyPath: " /tmp/device.key ",
            clearReceiverURL: false,
            clearTLSIdentity: false
        )
        let input = TaskInput(
            profilePath: "/tmp/profile.json",
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
            pairingTargetAddress: "",
            pairingVerificationCode: "",
            pairingMethod: "sas",
            pairingTimeout: "5s",
            profileNetwork: draft,
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
            SuperMoverTaskKind.profileSetNetwork.buildArguments(using: input),
            [
                "profile", "set-network",
                "--profile", "/tmp/profile.json",
                "--receiver-url", "https://127.0.0.1:9443",
                "--tls-cert", "/tmp/device.crt",
                "--tls-key", "/tmp/device.key",
            ]
        )
    }

    func testProfileSetNetworkSupportsClearFlags() {
        let draft = ProfileNetworkDraft(
            receiverURL: "",
            tlsCertificatePath: "",
            tlsPrivateKeyPath: "",
            clearReceiverURL: true,
            clearTLSIdentity: true
        )
        let input = TaskInput(
            profilePath: "/tmp/profile.json",
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
            pairingTargetAddress: "",
            pairingVerificationCode: "",
            pairingMethod: "sas",
            pairingTimeout: "5s",
            profileNetwork: draft,
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
            SuperMoverTaskKind.profileSetNetwork.buildArguments(using: input),
            ["profile", "set-network", "--profile", "/tmp/profile.json", "--clear-receiver-url", "--clear-tls-identity"]
        )
    }

    func testProfileSetNetworkIsRoleGatedToSourceAndTarget() {
        XCTAssertTrue(WorkbenchRole.source.allows(task: .profileSetNetwork))
        XCTAssertTrue(WorkbenchRole.target.allows(task: .profileSetNetwork))
        XCTAssertFalse(WorkbenchRole.observer.allows(task: .profileSetNetwork))
    }

    @MainActor
    func testProfileSetNetworkContextTracksReviewedInputs() {
        let store = AppStore()
        store.profilePath = "/tmp/profile.json"
        store.profileNetwork.receiverURL = "https://127.0.0.1:9443"

        let first = store.currentContextSignature(for: .profileSetNetwork)

        store.profileNetwork.tlsCertificatePath = "/tmp/device.crt"
        XCTAssertNotEqual(first, store.currentContextSignature(for: .profileSetNetwork))

        let second = store.currentContextSignature(for: .profileSetNetwork)
        store.profileNetwork.tlsPrivateKeyPath = "/tmp/device.key"
        XCTAssertNotEqual(second, store.currentContextSignature(for: .profileSetNetwork))
    }

    func testProfileNetworkDraftRequiresWholeTLSIdentity() {
        var draft = ProfileNetworkDraft()
        XCTAssertFalse(draft.hasRequestedChange)
        XCTAssertFalse(draft.hasPartialTLSIdentity)

        draft.tlsCertificatePath = "/tmp/device.crt"
        XCTAssertTrue(draft.hasRequestedChange)
        XCTAssertTrue(draft.hasPartialTLSIdentity)

        draft.tlsPrivateKeyPath = "/tmp/device.key"
        XCTAssertFalse(draft.hasPartialTLSIdentity)
    }
}
