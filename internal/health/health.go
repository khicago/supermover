package health

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
	"github.com/khicago/supermover/internal/verify"
)

type Options struct {
	TargetRoot string
	ProfileID  string
	TargetID   string
}

type Report struct {
	TargetRoot   string                `json:"target_root"`
	Healthy      bool                  `json:"healthy"`
	Summary      Summary               `json:"summary"`
	Items        []RecoveryItem        `json:"items,omitempty"`
	Invalid      []InvalidRecord       `json:"invalid,omitempty"`
	Artifacts    []ArtifactIssue       `json:"artifacts,omitempty"`
	TargetDrifts []control.TargetDrift `json:"target_drifts,omitempty"`
	Transfers    []NetworkTransfer     `json:"network_transfers,omitempty"`
}

type Summary struct {
	IncompleteSessions int `json:"incomplete_sessions"`
	InvalidRecords     int `json:"invalid_records"`
	ArtifactProblems   int `json:"artifact_problems"`
	TargetDrifts       int `json:"target_drifts"`
	NetworkTransfers   int `json:"network_transfers"`
}

type RecoveryItem struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	Path      string `json:"path"`
	UpdatedAt string `json:"updated_at"`
}

type InvalidRecord struct {
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}

type ArtifactIssue struct {
	Source    string `json:"source,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}

type NetworkTransfer struct {
	SessionID string                                  `json:"session_id"`
	ProfileID string                                  `json:"profile_id,omitempty"`
	TargetID  string                                  `json:"target_id,omitempty"`
	Status    string                                  `json:"status"`
	Stage     string                                  `json:"stage,omitempty"`
	ErrorCode string                                  `json:"error_code,omitempty"`
	Error     string                                  `json:"error,omitempty"`
	Path      string                                  `json:"path"`
	UpdatedAt string                                  `json:"updated_at,omitempty"`
	Action    string                                  `json:"action"`
	Privacy   transport.PrivacyPolicy                 `json:"privacy_policy,omitempty"`
	Overhead  *control.NetworkTransferPrivacyOverhead `json:"privacy_overhead,omitempty"`
}

func BuildReport(opts Options) (Report, error) {
	targetRoot := strings.TrimSpace(opts.TargetRoot)
	if targetRoot == "" {
		return Report{}, fmt.Errorf("target root is required")
	}
	info, err := os.Stat(targetRoot)
	if err != nil {
		return Report{}, fmt.Errorf("stat target root %q: %w", targetRoot, err)
	}
	if !info.IsDir() {
		return Report{}, fmt.Errorf("target root %q is not a directory", targetRoot)
	}
	if err := verify.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return Report{}, err
	}

	scan, err := transaction.ScanRecovery(transaction.NewLayout(control.ControlDir(targetRoot)))
	if err != nil {
		return Report{}, err
	}
	report := Report{TargetRoot: targetRoot}
	for _, item := range scan.Items {
		if networkTransferOutOfScope(targetRoot, item.Record.ID, opts.ProfileID, opts.TargetID) {
			continue
		}
		report.Items = append(report.Items, RecoveryItem{
			SessionID: item.Record.ID,
			State:     string(item.Record.State),
			Action:    string(item.Action),
			Reason:    item.Reason,
			Path:      item.Path,
			UpdatedAt: item.Record.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}
	transfers, transferArtifacts, err := loadNetworkTransfers(targetRoot, opts.ProfileID, opts.TargetID)
	if err != nil {
		return Report{}, err
	}
	for _, invalid := range scan.Invalid {
		if invalidNetworkSessionHasTransferArtifact(targetRoot, invalid, opts.ProfileID, opts.TargetID) {
			continue
		}
		report.Invalid = append(report.Invalid, InvalidRecord{
			SessionID: invalid.SessionID,
			Path:      invalid.Path,
			Error:     invalid.Err.Error(),
		})
	}
	artifactIssues, err := scanSessionArtifacts(targetRoot, opts.ProfileID, opts.TargetID)
	if err != nil {
		return Report{}, err
	}
	reviewArtifacts, err := verify.LoadArtifactsForScope(targetRoot, opts.ProfileID, opts.TargetID)
	if err != nil {
		return Report{}, err
	}
	for _, problem := range reviewArtifacts.ArtifactProblems {
		artifactIssues = append(artifactIssues, ArtifactIssue{SessionID: problem.SessionID, Path: problem.Path, Error: problem.Err})
	}
	artifactIssues = append(artifactIssues, transferArtifacts...)
	report.Artifacts = artifactIssues
	report.TargetDrifts = unresolvedTargetDrifts(reviewArtifacts.TargetDrifts)
	report.Transfers = transfers
	report.Summary.IncompleteSessions = len(report.Items)
	report.Summary.InvalidRecords = len(report.Invalid)
	report.Summary.ArtifactProblems = len(report.Artifacts)
	report.Summary.TargetDrifts = len(report.TargetDrifts)
	report.Summary.NetworkTransfers = len(report.Transfers)
	report.Healthy = report.Summary.IncompleteSessions == 0 &&
		report.Summary.InvalidRecords == 0 &&
		report.Summary.ArtifactProblems == 0 &&
		report.Summary.TargetDrifts == 0 &&
		report.Summary.NetworkTransfers == 0
	return report, nil
}

func unresolvedTargetDrifts(records []control.TargetDrift) []control.TargetDrift {
	out := make([]control.TargetDrift, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.ReviewState) == "resolved" {
			continue
		}
		out = append(out, record)
	}
	return out
}

func networkTransferOutOfScope(targetRoot string, sessionID string, profileID string, targetID string) bool {
	return networkTransferReceiptOutOfScope(targetRoot, sessionID, profileID, targetID)
}

func invalidNetworkSessionHasTransferArtifact(targetRoot string, invalid transaction.RecoveryProblem, profileID string, targetID string) bool {
	if strings.TrimSpace(invalid.SessionID) == "" {
		return false
	}
	return networkTransferReceiptOutOfScope(targetRoot, invalid.SessionID, profileID, targetID)
}

func loadNetworkTransfers(targetRoot string, profileID string, targetID string) ([]NetworkTransfer, []ArtifactIssue, error) {
	sessionsDir := filepath.Join(control.ControlDir(targetRoot), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read network transfer artifacts: %w", err)
	}

	var transfers []NetworkTransfer
	var artifacts []ArtifactIssue
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionID := entry.Name()
		path, err := control.Path(targetRoot, control.ArtifactNetworkTransfer, sessionID)
		if err != nil {
			artifacts = append(artifacts, networkTransferArtifactIssue(sessionID, "", err.Error()))
			continue
		}
		doc, err := control.ReadFile[control.NetworkTransfer](path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if networkTransferReceiptOutOfScope(targetRoot, sessionID, profileID, targetID) {
				continue
			}
			artifacts = append(artifacts, networkTransferArtifactIssue(sessionID, path, err.Error()))
			continue
		}
		if networkTransferReceiptOutOfScope(targetRoot, sessionID, profileID, targetID) {
			continue
		}
		if doc.SessionID != sessionID {
			artifacts = append(artifacts, networkTransferArtifactIssue(sessionID, path, fmt.Sprintf("network transfer session_id %q does not match session directory %q", doc.SessionID, sessionID)))
			continue
		}
		inScope, scopeIssues := evaluateNetworkTransferScope(targetRoot, sessionID, doc, path, profileID, targetID)
		artifacts = append(artifacts, scopeIssues...)
		if !inScope {
			continue
		}
		artifacts = append(artifacts, networkTransferStateIssues(targetRoot, path, doc)...)
		if doc.Status == control.NetworkTransferPublished {
			continue
		}
		transfers = append(transfers, summarizeNetworkTransfer(path, doc))
	}
	return transfers, artifacts, nil
}

func evaluateNetworkTransferScope(targetRoot string, sessionID string, doc control.NetworkTransfer, transferPath string, profileID string, targetID string) (bool, []ArtifactIssue) {
	receiptPath, err := control.Path(targetRoot, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return true, []ArtifactIssue{networkTransferArtifactIssue(sessionID, transferPath, err.Error())}
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return true, []ArtifactIssue{networkTransferArtifactIssue(sessionID, receiptPath, err.Error())}
	}
	if receipt.ID != sessionID {
		return true, []ArtifactIssue{networkTransferArtifactIssue(sessionID, receiptPath, fmt.Sprintf("receipt id %q does not match session directory %q", receipt.ID, sessionID))}
	}
	receiptInScope := receiptInScope(receipt, profileID, targetID)
	if !receiptInScope {
		return false, nil
	}
	if receipt.ProfileID != doc.ProfileID || receipt.TargetID != doc.TargetID {
		return true, []ArtifactIssue{{
			Source:    "network_transfer",
			SessionID: doc.SessionID,
			Path:      transferPath,
			Error:     fmt.Sprintf("network transfer profile_id/target_id (%q/%q) does not match session receipt profile_id/target_id (%q/%q)", doc.ProfileID, doc.TargetID, receipt.ProfileID, receipt.TargetID),
		}}
	}
	return true, nil
}

func networkTransferReceiptOutOfScope(targetRoot string, sessionID string, profileID string, targetID string) bool {
	receiptPath, err := control.Path(targetRoot, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return false
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		return false
	}
	if receipt.ID != sessionID {
		return false
	}
	return !receiptInScope(receipt, profileID, targetID)
}

func receiptInScope(receipt control.SessionReceipt, profileID string, targetID string) bool {
	if strings.TrimSpace(profileID) != "" && receipt.ProfileID != profileID {
		return false
	}
	if strings.TrimSpace(targetID) != "" && receipt.TargetID != targetID {
		return false
	}
	return true
}

func networkTransferStateIssues(targetRoot string, path string, doc control.NetworkTransfer) []ArtifactIssue {
	if doc.Status != control.NetworkTransferPublished {
		return nil
	}
	var issues []ArtifactIssue
	recordPath := transaction.NewLayout(control.ControlDir(targetRoot)).RecordPath(doc.SessionID)
	record, err := transaction.ReadSessionRecord(recordPath)
	if err == nil && record.State != transaction.StatePublished {
		issues = append(issues, ArtifactIssue{
			Source:    "network_transfer",
			SessionID: doc.SessionID,
			Path:      path,
			Error:     fmt.Sprintf("network transfer status %q disagrees with session state %q", doc.Status, record.State),
		})
	}
	receiptPath, pathErr := control.Path(targetRoot, control.ArtifactSessionReceipt, doc.SessionID)
	if pathErr != nil {
		issues = append(issues, networkTransferArtifactIssue(doc.SessionID, path, pathErr.Error()))
		return issues
	}
	receipt, receiptErr := control.ReadFile[control.SessionReceipt](receiptPath)
	if receiptErr != nil {
		if !os.IsNotExist(receiptErr) || err == nil {
			issues = append(issues, networkTransferArtifactIssue(doc.SessionID, receiptPath, receiptErr.Error()))
		}
		return issues
	}
	if receipt.Status != "published" {
		issues = append(issues, ArtifactIssue{
			Source:    "network_transfer",
			SessionID: doc.SessionID,
			Path:      receiptPath,
			Error:     fmt.Sprintf("network transfer status %q disagrees with receipt status %q", doc.Status, receipt.Status),
		})
	}
	return issues
}

func networkTransferArtifactIssue(sessionID string, path string, err string) ArtifactIssue {
	return ArtifactIssue{
		Source:    "network_transfer",
		SessionID: sessionID,
		Path:      path,
		Error:     err,
	}
}

func summarizeNetworkTransfer(path string, doc control.NetworkTransfer) NetworkTransfer {
	return NetworkTransfer{
		SessionID: doc.SessionID,
		ProfileID: doc.ProfileID,
		TargetID:  doc.TargetID,
		Status:    string(doc.Status),
		Stage:     doc.Stage,
		ErrorCode: doc.ErrorCode,
		Error:     doc.Error,
		Path:      path,
		UpdatedAt: doc.UpdatedAt,
		Action:    networkTransferAction(doc.Status),
		Privacy:   doc.PrivacyPolicy,
		Overhead:  doc.PrivacyOverhead,
	}
}

func networkTransferAction(status control.NetworkTransferStatus) string {
	switch status {
	case control.NetworkTransferStarted, control.NetworkTransferInterrupted:
		return "retry_network_transfer"
	case control.NetworkTransferAuthRefused:
		return "review_pairing_and_profile_pins"
	case control.NetworkTransferNeedsRepair:
		return "review_receiver_repair_state_before_retry"
	case control.NetworkTransferPublishFailed:
		return "inspect_target_publish_state_before_retry"
	case control.NetworkTransferFailed:
		return "inspect_transfer_error_before_retry"
	default:
		return "inspect_network_transfer"
	}
}

func scanSessionArtifacts(targetRoot string, profileID string, targetID string) ([]ArtifactIssue, error) {
	sessionsDir := filepath.Join(control.ControlDir(targetRoot), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session artifacts: %w", err)
	}

	var issues []ArtifactIssue
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionID := entry.Name()
		if sessionArtifactsOutOfScope(targetRoot, sessionID, profileID, targetID) {
			continue
		}
		recordPath := transaction.NewLayout(control.ControlDir(targetRoot)).RecordPath(sessionID)
		record, err := transaction.ReadSessionRecord(recordPath)
		if err != nil {
			continue
		}
		sessionIssues, err := scanSessionArtifactEvidence(targetRoot, sessionID, record)
		if err != nil {
			return nil, err
		}
		issues = append(issues, sessionIssues...)
	}
	return issues, nil
}

func sessionArtifactsOutOfScope(targetRoot string, sessionID string, profileID string, targetID string) bool {
	if strings.TrimSpace(profileID) == "" && strings.TrimSpace(targetID) == "" {
		return false
	}
	return networkTransferReceiptOutOfScope(targetRoot, sessionID, profileID, targetID)
}

func scanSessionArtifactEvidence(targetRoot string, sessionID string, record transaction.SessionRecord) ([]ArtifactIssue, error) {
	var issues []ArtifactIssue
	manifestPath, err := control.Path(targetRoot, control.ArtifactManifest, sessionID)
	if err != nil {
		return nil, err
	}
	receiptPath, err := control.Path(targetRoot, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return nil, err
	}
	receipt, receiptErr := control.ReadFile[control.SessionReceipt](receiptPath)
	receiptPublished := receiptErr == nil && receipt.Status == "published"
	manifestRequired := record.State == transaction.StatePublished || receiptPublished
	manifestExists := appendManifestArtifactIssues(&issues, sessionID, manifestPath, manifestRequired)

	if receiptPublished && record.State != transaction.StatePublished {
		issues = append(issues, ArtifactIssue{
			SessionID: sessionID,
			Path:      receiptPath,
			Error:     fmt.Sprintf("receipt status %q disagrees with session state %q", receipt.Status, record.State),
		})
	} else if receiptErr == nil && record.State != transaction.StatePublished {
		issues = append(issues, ArtifactIssue{
			SessionID: sessionID,
			Path:      receiptPath,
			Error:     fmt.Sprintf("receipt status %q exists for non-published session state %q", receipt.Status, record.State),
		})
	} else if receiptErr != nil && !os.IsNotExist(receiptErr) && record.State != transaction.StatePublished {
		issues = append(issues, ArtifactIssue{SessionID: sessionID, Path: receiptPath, Error: receiptErr.Error()})
	}
	if manifestExists && (record.State == transaction.StateReceived || record.State == transaction.StateValidated) {
		issues = append(issues, ArtifactIssue{
			SessionID: sessionID,
			Path:      manifestPath,
			Error:     fmt.Sprintf("manifest exists for non-staged session state %q", record.State),
		})
	}
	if record.State != transaction.StatePublished {
		return issues, nil
	}
	if receiptErr != nil {
		issues = append(issues, ArtifactIssue{SessionID: sessionID, Path: receiptPath, Error: receiptErr.Error()})
		return issues, nil
	}
	if receipt.Status != "published" {
		issues = append(issues, ArtifactIssue{SessionID: sessionID, Path: receiptPath, Error: fmt.Sprintf("receipt status %q is not published", receipt.Status)})
	}
	return issues, nil
}

func appendManifestArtifactIssues(issues *[]ArtifactIssue, sessionID string, manifestPath string, required bool) bool {
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			if required {
				*issues = append(*issues, ArtifactIssue{SessionID: sessionID, Path: manifestPath, Error: err.Error()})
			}
			return false
		}
		*issues = append(*issues, ArtifactIssue{SessionID: sessionID, Path: manifestPath, Error: err.Error()})
		return false
	}
	if _, err := control.ReadManifestCompatFile(manifestPath); err != nil {
		*issues = append(*issues, ArtifactIssue{SessionID: sessionID, Path: manifestPath, Error: err.Error()})
	}
	return true
}
