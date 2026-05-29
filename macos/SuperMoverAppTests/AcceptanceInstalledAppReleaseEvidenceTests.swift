import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppReleaseEvidenceTests: XCTestCase {
    func testAcceptedNotaryLogRequiresAcceptedStatusAndMatchingJobID() {
        let data = Data(
            AcceptanceReleaseEvidenceFixtures.notaryLogJSON(
                submissionID: "11111111-1111-1111-1111-111111111111"
            ).utf8
        )

        XCTAssertTrue(
            AcceptanceInstalledAppReleaseEvidenceSummary.acceptedNotaryLog(
                data: data,
                submissionID: "11111111-1111-1111-1111-111111111111"
            )
        )
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.acceptedNotaryLog(
                data: data,
                submissionID: "22222222-2222-2222-2222-222222222222"
            )
        )
    }

    func testAcceptedNotaryLogRejectsAcceptedLogWithoutJobID() {
        let data = Data(#"{"status":"Accepted","issues":[]}"#.utf8)

        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.acceptedNotaryLog(
                data: data,
                submissionID: "11111111-1111-1111-1111-111111111111"
            )
        )
    }

    func testNotarizationIsReleaseReadyAcceptsReleaseReadySubmissionAndAudit() {
        XCTAssertTrue(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: "11111111-1111-1111-1111-111111111111",
                submissionStatus: "Accepted",
                authMode: "keychain_profile",
                failurePresent: false,
                notaryLogPath: "/tmp/notary/notary-log.json",
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }

    func testNotarizationIsReleaseReadyRejectsMissingAuthMode() {
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: "11111111-1111-1111-1111-111111111111",
                submissionStatus: "Accepted",
                authMode: nil,
                failurePresent: false,
                notaryLogPath: "/tmp/notary/notary-log.json",
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }

    func testNotarizationIsReleaseReadyRejectsUnknownAuthMode() {
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: "11111111-1111-1111-1111-111111111111",
                submissionStatus: "Accepted",
                authMode: "manual",
                failurePresent: false,
                notaryLogPath: "/tmp/notary/notary-log.json",
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }

    func testNotarizationIsReleaseReadyRejectsFailureRecord() {
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: "11111111-1111-1111-1111-111111111111",
                submissionStatus: "Accepted",
                authMode: "keychain_profile",
                failurePresent: true,
                notaryLogPath: "/tmp/notary/notary-log.json",
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }

    func testNotarizationIsReleaseReadyRejectsNonAcceptedSubmission() {
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: "11111111-1111-1111-1111-111111111111",
                submissionStatus: "In Progress",
                authMode: "keychain_profile",
                failurePresent: false,
                notaryLogPath: "/tmp/notary/notary-log.json",
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }

    func testNotarizationIsReleaseReadyRejectsMissingSubmissionID() {
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: nil,
                submissionStatus: "Accepted",
                authMode: "keychain_profile",
                failurePresent: false,
                notaryLogPath: "/tmp/notary/notary-log.json",
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }

    func testNotarizationIsReleaseReadyRejectsMalformedSubmissionID() {
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: "manual-pass",
                submissionStatus: "Accepted",
                authMode: "keychain_profile",
                failurePresent: false,
                notaryLogPath: "/tmp/notary/notary-log.json",
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }

    func testNotarizationIsReleaseReadyRejectsMissingNotaryLog() {
        XCTAssertFalse(
            AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: "pass",
                submissionID: "11111111-1111-1111-1111-111111111111",
                submissionStatus: "Accepted",
                authMode: "keychain_profile",
                failurePresent: false,
                notaryLogPath: nil,
                auditStatus: "pass",
                auditReadiness: "distribution_ready",
                auditPassReady: true
            )
        )
    }
}
