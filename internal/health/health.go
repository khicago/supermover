package health

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/verify"
)

type Options struct {
	TargetRoot string
}

type Report struct {
	TargetRoot string          `json:"target_root"`
	Healthy    bool            `json:"healthy"`
	Summary    Summary         `json:"summary"`
	Items      []RecoveryItem  `json:"items,omitempty"`
	Invalid    []InvalidRecord `json:"invalid,omitempty"`
	Artifacts  []ArtifactIssue `json:"artifacts,omitempty"`
}

type Summary struct {
	IncompleteSessions int `json:"incomplete_sessions"`
	InvalidRecords     int `json:"invalid_records"`
	ArtifactProblems   int `json:"artifact_problems"`
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
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
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

	scan, err := transaction.ScanRecovery(transaction.NewLayout(control.ControlDir(targetRoot)))
	if err != nil {
		return Report{}, err
	}
	report := Report{TargetRoot: targetRoot}
	for _, item := range scan.Items {
		report.Items = append(report.Items, RecoveryItem{
			SessionID: item.Record.ID,
			State:     string(item.Record.State),
			Action:    string(item.Action),
			Reason:    item.Reason,
			Path:      item.Path,
			UpdatedAt: item.Record.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}
	for _, invalid := range scan.Invalid {
		report.Invalid = append(report.Invalid, InvalidRecord{
			SessionID: invalid.SessionID,
			Path:      invalid.Path,
			Error:     invalid.Err.Error(),
		})
	}
	artifactIssues, err := scanSessionArtifacts(targetRoot)
	if err != nil {
		return Report{}, err
	}
	reviewArtifacts, err := verify.LoadArtifacts(targetRoot)
	if err != nil {
		return Report{}, err
	}
	for _, problem := range reviewArtifacts.ArtifactProblems {
		artifactIssues = append(artifactIssues, ArtifactIssue{SessionID: problem.SessionID, Path: problem.Path, Error: problem.Err})
	}
	report.Artifacts = artifactIssues
	report.Summary.IncompleteSessions = len(report.Items)
	report.Summary.InvalidRecords = len(report.Invalid)
	report.Summary.ArtifactProblems = len(report.Artifacts)
	report.Healthy = report.Summary.IncompleteSessions == 0 && report.Summary.InvalidRecords == 0 && report.Summary.ArtifactProblems == 0
	return report, nil
}

func scanSessionArtifacts(targetRoot string) ([]ArtifactIssue, error) {
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
