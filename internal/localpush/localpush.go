package localpush

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/deleted"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/filelock"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/transaction"
)

type Options struct {
	Profile   profile.Profile
	TargetDir string
	SessionID string
	Now       time.Time
}

type Result struct {
	SessionID  string
	Entries    int
	Copied     int
	Warnings   int
	Influences int
	Deleted    int
}

var localPushLocks sync.Map

var errSymlinkTargetConflict = errors.New("target symlink conflict")
var errManagedReplaceTargetChanged = errors.New("managed replace target changed")

var beforeReadStableSymlink func(sourcePath string, entry scan.Entry) error
var beforePublishStaged func(entry control.ManifestEntry, targetPath string) error
var beforeManagedReplacePromote func(entry control.ManifestEntry, targetPath string) error
var beforeManagedReplaceCurrentHold func(entry control.ManifestEntry, targetPath string, holdPath string) error
var afterManagedReplaceHold func(entry control.ManifestEntry, targetPath string, holdPath string) error

type publishMode int

const (
	publishModeRun publishMode = iota
	publishModeRecover
)

type RecoverOptions struct {
	Profile            profile.Profile
	TargetDir          string
	SessionID          string
	DryRun             bool
	RollbackIncomplete bool
	Now                time.Time
}

type RecoverResult struct {
	TargetDir    string        `json:"target_dir"`
	SessionID    string        `json:"session_id,omitempty"`
	DryRun       bool          `json:"dry_run"`
	Inspected    int           `json:"inspected"`
	Recovered    int           `json:"recovered"`
	Skipped      int           `json:"skipped"`
	RepairNeeded int           `json:"repair_needed"`
	Items        []RecoverItem `json:"items,omitempty"`
}

type RecoverItem struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

type previousFileEvidence struct {
	SessionID  string
	ManifestID string
	Size       int64
	Digest     string
	Mode       uint32
	ModTime    string
	HasSize    bool
	HasMode    bool
}

type replacementHolds struct {
	previousPath string
	currentPath  string
}

func Run(opts Options) (Result, error) {
	if err := opts.Profile.Validate(); err != nil {
		return Result{}, err
	}
	if err := ValidateProfileForLocalPush(opts.Profile); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(opts.TargetDir) == "" {
		return Result{}, fmt.Errorf("target directory is required")
	}
	if err := ValidateSourceTargetSeparation(opts.Profile.Roots[0].Path, opts.TargetDir); err != nil {
		return Result{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sessionID := opts.SessionID
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "session-" + now.UTC().Format("20060102T150405Z")
	}
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		return Result{}, err
	}
	unlock, err := lockTargetSession(opts.TargetDir, sessionID)
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	if err := ensureSessionUnused(opts.TargetDir, sessionID); err != nil {
		return Result{}, err
	}
	if err := refuseUnhealthyRecoveryState(opts.Profile, opts.TargetDir); err != nil {
		return Result{}, err
	}

	root := opts.Profile.Roots[0]
	scanResult, err := scan.Scan(root.Path)
	if err != nil {
		return Result{}, err
	}
	scanResult = dropControlPlaneEntries(scanResult)
	if err := rejectScanErrors(scanResult); err != nil {
		return Result{}, err
	}
	previous, hasPrevious, err := latestPublishedManifest(opts.Profile, opts.TargetDir)
	if err != nil {
		return Result{}, err
	}
	previousEntries := previousManifestEntries(previous, hasPrevious)
	influences := agentkb.Detect(scanResult.Entries, agentKnowledgeCategories(opts.Profile.AgentKnowledge))
	softDeletes, err := softDeletesForRun(opts.Profile, previous, hasPrevious, scanResult, sessionID, now)
	if err != nil {
		return Result{}, err
	}
	controlDir := control.ControlDir(opts.TargetDir)
	layout := transaction.NewLayout(controlDir)
	record, err := transaction.NewSessionRecord(sessionID, now)
	if err != nil {
		return Result{}, err
	}
	if err := layout.EnsureSessionDirs(sessionID); err != nil {
		return Result{}, err
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		return Result{}, err
	}

	var manifestEntries []control.ManifestEntry
	warnings := append([]audit.Record(nil), scanResult.Audit...)
	copied := 0
	for _, entry := range scanResult.Entries {
		if entry.Path == "." {
			continue
		}
		sourcePath := filepath.Join(root.Path, filepath.FromSlash(entry.Path))
		switch entry.Kind {
		case scan.KindDir:
			manifestEntries = append(manifestEntries, manifestEntry(entry, "dir", ""))
		case scan.KindRegular:
			stagePath, err := pathguard.SafeJoinParent(layout.StagingDir(sessionID), entry.Path)
			if err != nil {
				return Result{}, err
			}
			digest, err := copyRegularToStage(sourcePath, stagePath, entry)
			if err != nil {
				return Result{}, err
			}
			copied++
			manifestEntries = append(manifestEntries, manifestEntryWithPrevious(entry, "file", digest, previousEntries[cleanManifestTarget(entry.Path)]))
		case scan.KindSymlink:
			if beforeReadStableSymlink != nil {
				if err := beforeReadStableSymlink(sourcePath, entry); err != nil {
					return Result{}, err
				}
			}
			currentTarget, err := readStableSymlink(sourcePath, entry)
			if err != nil {
				return Result{}, err
			}
			entry.SymlinkTarget = currentTarget
			targetPath, err := targetPathForEntry(opts.TargetDir, entry)
			if err != nil {
				return Result{}, err
			}
			if err := preflightSymlinkTarget(targetPath, entry.SymlinkTarget); err != nil {
				if errors.Is(err, errSymlinkTargetConflict) || errors.Is(err, pathguard.ErrUnsafePath) {
					warnings = append(warnings, audit.WithDetected(
						audit.New(entry.Path, entry.Path, audit.SeverityWarning, "symlink_not_published", err.Error()),
						map[string]string{"target": entry.SymlinkTarget},
					))
					continue
				}
				return Result{}, err
			}
			manifestEntries = append(manifestEntries, manifestEntry(entry, "symlink", ""))
		default:
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, entry.Path, audit.SeverityWarning, "special_not_copied", "special file copy is not supported"),
				map[string]string{"mode": entry.Mode.String()},
			))
		}
	}

	if err := preflightPublishPlan(layout, opts.TargetDir, sessionID, manifestEntries, publishModeRun); err != nil {
		return Result{}, err
	}
	existingDirs, err := captureExistingPublishDirs(opts.TargetDir, manifestEntries)
	if err != nil {
		return Result{}, err
	}
	if err := writeControlArtifacts(opts.TargetDir, opts.Profile, sessionID, now, manifestEntries, warnings, influences, softDeletes); err != nil {
		return Result{}, err
	}
	record, err = record.WithState(transaction.StateStaged, now)
	if err != nil {
		return Result{}, err
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		return Result{}, err
	}
	if err := publishStaged(layout, opts.TargetDir, sessionID, manifestEntries, existingDirs, publishModeRun); err != nil {
		return Result{}, err
	}
	if err := writeSessionReceipt(opts.TargetDir, opts.Profile, sessionID, now); err != nil {
		return Result{}, err
	}
	record, err = record.WithState(transaction.StatePublished, now)
	if err != nil {
		return Result{}, err
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		return Result{}, err
	}
	return Result{
		SessionID:  sessionID,
		Entries:    len(manifestEntries),
		Copied:     copied,
		Warnings:   len(warnings),
		Influences: len(influences),
		Deleted:    len(softDeletes),
	}, nil
}

func Preflight(opts Options) (Result, error) {
	if err := opts.Profile.Validate(); err != nil {
		return Result{}, err
	}
	if err := ValidateProfileForLocalPush(opts.Profile); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(opts.TargetDir) == "" {
		return Result{}, fmt.Errorf("target directory is required")
	}
	if err := ValidateSourceTargetSeparation(opts.Profile.Roots[0].Path, opts.TargetDir); err != nil {
		return Result{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		sessionID = "dry-run-" + now.UTC().Format("20060102T150405Z")
	}
	root := opts.Profile.Roots[0]
	scanResult, err := scan.Scan(root.Path)
	if err != nil {
		return Result{}, err
	}
	scanResult = dropControlPlaneEntries(scanResult)
	if err := rejectScanErrors(scanResult); err != nil {
		return Result{}, err
	}
	warnings := append([]audit.Record(nil), scanResult.Audit...)
	influences := agentkb.Detect(scanResult.Entries, agentKnowledgeCategories(opts.Profile.AgentKnowledge))
	previous, hasPrevious, err := latestPublishedManifest(opts.Profile, opts.TargetDir)
	if err != nil {
		return Result{}, err
	}
	previousEntries := previousManifestEntries(previous, hasPrevious)
	softDeletes, err := softDeletesForRun(opts.Profile, previous, hasPrevious, scanResult, sessionID, now)
	if err != nil {
		return Result{}, err
	}
	entries := 0
	copied := 0
	for _, entry := range scanResult.Entries {
		if entry.Path == "." {
			continue
		}
		targetPath, err := targetPathForEntry(opts.TargetDir, entry)
		if err != nil {
			return Result{}, err
		}
		switch entry.Kind {
		case scan.KindDir:
			if err := ensureDirectoryPreflight(targetPath); err != nil {
				return Result{}, err
			}
			entries++
		case scan.KindRegular:
			sourcePath := filepath.Join(root.Path, filepath.FromSlash(entry.Path))
			if err := preflightRegularTarget(sourcePath, targetPath, previousEntries[cleanManifestTarget(entry.Path)]); err != nil {
				return Result{}, err
			}
			entries++
			copied++
		case scan.KindSymlink:
			if err := preflightSymlinkTarget(targetPath, entry.SymlinkTarget); err != nil {
				if errors.Is(err, errSymlinkTargetConflict) || errors.Is(err, pathguard.ErrUnsafePath) {
					warnings = append(warnings, audit.WithDetected(
						audit.New(entry.Path, entry.Path, audit.SeverityWarning, "symlink_not_published", err.Error()),
						map[string]string{"target": entry.SymlinkTarget},
					))
					continue
				}
				return Result{}, err
			}
			entries++
		default:
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, entry.Path, audit.SeverityWarning, "special_not_copied", "special file copy is not supported"),
				map[string]string{"mode": entry.Mode.String()},
			))
		}
	}
	return Result{SessionID: sessionID, Entries: entries, Copied: copied, Warnings: len(warnings), Influences: len(influences), Deleted: len(softDeletes)}, nil
}

func Recover(opts RecoverOptions) (RecoverResult, error) {
	if err := opts.Profile.Validate(); err != nil {
		return RecoverResult{}, err
	}
	if err := ValidateProfileForLocalPush(opts.Profile); err != nil {
		return RecoverResult{}, err
	}
	if strings.TrimSpace(opts.TargetDir) == "" {
		return RecoverResult{}, fmt.Errorf("target directory is required")
	}
	if err := ValidateSourceTargetSeparation(opts.Profile.Roots[0].Path, opts.TargetDir); err != nil {
		return RecoverResult{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	wantSession := strings.TrimSpace(opts.SessionID)
	if wantSession != "" {
		if err := transaction.ValidateSessionID(wantSession); err != nil {
			return RecoverResult{}, err
		}
	}
	lockSession := wantSession
	if lockSession == "" {
		lockSession = "recover"
	}
	unlock, err := lockTargetSession(opts.TargetDir, lockSession)
	if err != nil {
		return RecoverResult{}, err
	}
	defer unlock()
	layout := transaction.NewLayout(control.ControlDir(opts.TargetDir))
	scan, err := transaction.ScanRecovery(layout)
	if err != nil {
		return RecoverResult{}, err
	}

	result := RecoverResult{TargetDir: opts.TargetDir, SessionID: wantSession, DryRun: opts.DryRun}
	matched := false
	for _, invalid := range scan.Invalid {
		if wantSession != "" && sessionIDFromRecordPath(invalid.Path) != wantSession {
			continue
		}
		matched = true
		result.Inspected++
		result.RepairNeeded++
		result.Items = append(result.Items, RecoverItem{
			SessionID: sessionIDFromRecordPath(invalid.Path),
			State:     "invalid",
			Action:    string(transaction.ActionRepair),
			Status:    "needs_repair",
			Message:   invalid.Err.Error(),
		})
	}
	for _, scanned := range scan.Items {
		if wantSession != "" && scanned.Record.ID != wantSession {
			continue
		}
		matched = true
		result.Inspected++
		item := RecoverItem{
			SessionID: scanned.Record.ID,
			State:     string(scanned.Record.State),
			Action:    string(scanned.Action),
		}
		switch scanned.Action {
		case transaction.ActionRecover:
			if receiptState, err := inspectRecoveryReceipt(opts.TargetDir, scanned.Record.ID); err != nil {
				var marked error
				item, marked = addRecoverRepairItem(&result, layout, item, scanned.Record, now, err, opts.DryRun)
				result.Items = append(result.Items, item)
				if marked != nil {
					return result, marked
				}
				continue
			} else if receiptState == recoveryReceiptPublished {
				if opts.DryRun {
					if err := validatePublishedReceiptEvidence(opts.Profile, opts.TargetDir, scanned.Record); err != nil {
						item.Status = "would_mark_needs_repair"
						item.Message = err.Error()
					} else {
						item.Status = "would_complete_state"
						item.Message = "receipt and target already prove published session; session state would be marked published"
					}
					result.Skipped++
					result.Items = append(result.Items, item)
					continue
				}
				if err := completePublishedStateFromReceipt(layout, opts.Profile, opts.TargetDir, scanned.Record, now); err != nil {
					var marked error
					item, marked = addRecoverRepairItem(&result, layout, item, scanned.Record, now, err, false)
					if marked != nil {
						return result, marked
					}
				} else {
					item.Status = "completed_state"
					item.Message = "receipt and target already prove published session; session state marked published"
					result.Recovered++
				}
				result.Items = append(result.Items, item)
				continue
			}
			if opts.DryRun {
				item.Status = "would_recover"
				item.Message = scanned.Reason
				result.Skipped++
				result.Items = append(result.Items, item)
				continue
			}
			if err := recoverStagedSession(layout, opts.Profile, opts.TargetDir, scanned.Record, now); err != nil {
				var marked error
				item, marked = addRecoverRepairItem(&result, layout, item, scanned.Record, now, err, false)
				if marked != nil {
					return result, marked
				}
			} else {
				item.Status = "recovered"
				item.Message = "published staged payload and wrote receipt"
				result.Recovered++
			}
		case transaction.ActionRollback:
			if opts.DryRun {
				item.Status = "would_rollback"
				item.Message = scanned.Reason
				result.Skipped++
			} else if opts.RollbackIncomplete {
				rolledBack, err := scanned.Record.WithState(transaction.StateRolledBack, now)
				if err != nil {
					return result, err
				}
				rolledBack.Note = "rolled back by recover: " + scanned.Reason
				if err := layout.WriteSessionRecord(rolledBack); err != nil {
					return result, err
				}
				item.Status = "rolled_back"
				item.Message = "session did not reach durable staging; final target files were not published"
				result.Recovered++
			} else {
				item.Status = "skipped"
				item.Message = "session did not reach durable staging; rerun with --rollback-incomplete to mark terminal"
				result.Skipped++
			}
		case transaction.ActionRepair:
			item.Status = "needs_repair"
			item.Message = scanned.Reason
			result.RepairNeeded++
		default:
			item.Status = "skipped"
			item.Message = scanned.Reason
			result.Skipped++
		}
		result.Items = append(result.Items, item)
	}
	if wantSession != "" && !matched {
		if _, err := transaction.ReadSessionRecord(layout.RecordPath(wantSession)); err != nil {
			return result, fmt.Errorf("session %q not found in recovery scan: %w", wantSession, err)
		}
	}
	return result, nil
}

type recoveryReceiptState int

const (
	recoveryReceiptMissing recoveryReceiptState = iota
	recoveryReceiptPublished
)

func inspectRecoveryReceipt(targetDir string, sessionID string) (recoveryReceiptState, error) {
	receiptPath, err := control.Path(targetDir, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return recoveryReceiptMissing, err
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return recoveryReceiptMissing, nil
		}
		return recoveryReceiptMissing, fmt.Errorf("read recovery receipt %q: %w", sessionID, err)
	}
	if receipt.ID != sessionID {
		return recoveryReceiptMissing, fmt.Errorf("recovery receipt id %q does not match session %q; refusing to overwrite existing receipt", receipt.ID, sessionID)
	}
	if receipt.Status != "published" {
		return recoveryReceiptMissing, fmt.Errorf("recovery receipt status %q is not published; refusing to overwrite existing receipt", receipt.Status)
	}
	return recoveryReceiptPublished, nil
}

func addRecoverRepairItem(result *RecoverResult, layout transaction.Layout, item RecoverItem, record transaction.SessionRecord, now time.Time, cause error, dryRun bool) (RecoverItem, error) {
	item.Message = cause.Error()
	if dryRun {
		item.Status = "would_mark_needs_repair"
		result.Skipped++
		return item, nil
	}
	item.Status = "needs_repair"
	result.RepairNeeded++
	return item, markSessionNeedsRepair(layout, record, now, cause)
}

func completePublishedStateFromReceipt(layout transaction.Layout, p profile.Profile, targetDir string, record transaction.SessionRecord, now time.Time) error {
	if err := validatePublishedReceiptEvidence(p, targetDir, record); err != nil {
		return err
	}
	published, err := record.WithState(transaction.StatePublished, now)
	if err != nil {
		return err
	}
	published.Note = "completed after receipt already published"
	return layout.WriteSessionRecord(published)
}

func validatePublishedReceiptEvidence(p profile.Profile, targetDir string, record transaction.SessionRecord) error {
	if record.State != transaction.StateStaged {
		return fmt.Errorf("session %q state is %q, want staged", record.ID, record.State)
	}
	receiptPath, err := control.Path(targetDir, control.ArtifactSessionReceipt, record.ID)
	if err != nil {
		return err
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		return err
	}
	if receipt.ID != record.ID {
		return fmt.Errorf("receipt id %q does not match session %q", receipt.ID, record.ID)
	}
	if receipt.Status != "published" {
		return fmt.Errorf("receipt status %q is not published", receipt.Status)
	}
	if receipt.ProfileID != p.ProfileID || receipt.TargetID != p.Target.TargetID {
		return fmt.Errorf("receipt scope does not match current profile/target")
	}
	manifestPath, err := control.Path(targetDir, control.ArtifactManifest, record.ID)
	if err != nil {
		return err
	}
	manifest, err := control.ReadManifestCompatFile(manifestPath)
	if err != nil {
		return err
	}
	if manifest.SessionID != record.ID {
		return fmt.Errorf("manifest session_id %q does not match session %q", manifest.SessionID, record.ID)
	}
	if manifest.ID != "manifest-"+record.ID {
		return fmt.Errorf("manifest id %q does not match session %q", manifest.ID, record.ID)
	}
	if manifest.CreatedAt != record.CreatedAt.UTC().Format(time.RFC3339Nano) && manifest.CreatedAt != record.CreatedAt.UTC().Format(time.RFC3339) {
		return fmt.Errorf("manifest created_at %q does not match session %q created_at %q", manifest.CreatedAt, record.ID, record.CreatedAt.UTC().Format(time.RFC3339Nano))
	}
	if manifest.RootID != "" && manifest.RootID != p.Roots[0].ID {
		return fmt.Errorf("manifest root_id %q does not match profile root %q", manifest.RootID, p.Roots[0].ID)
	}
	if err := validateRecoverProfileSnapshot(targetDir, p, manifest); err != nil {
		return err
	}
	if err := validatePreviousEvidenceForRecover(p, targetDir, manifest); err != nil {
		return err
	}
	if err := validateRecoverablePublishedTargets(targetDir, manifest.Entries); err != nil {
		return err
	}
	return nil
}

func validateRecoverablePublishedTargets(targetDir string, entries []control.ManifestEntry) error {
	if err := validateTargetPlan(entries, func(entry control.ManifestEntry) (string, string) {
		return targetPath(entry), entry.Kind
	}); err != nil {
		return err
	}
	for _, entry := range entries {
		entryTarget := targetPath(entry)
		if pathguard.IsReservedControlPath(entryTarget) {
			return fmt.Errorf("manifest target path %q uses reserved control directory", entryTarget)
		}
		finalPath, err := targetPathForManifestEntry(targetDir, entry)
		if err != nil {
			return err
		}
		switch entry.Kind {
		case "file":
			same, exists, err := targetFileContentState(finalPath, entry.Size, entry.Digest)
			if err != nil {
				return err
			}
			if !exists || !same {
				return fmt.Errorf("target file %q does not match published receipt manifest", finalPath)
			}
			manifestSame, err := targetMatchesManifestFile(finalPath, entry)
			if err != nil {
				return err
			}
			if !manifestSame {
				return fmt.Errorf("target file %q does not match published receipt manifest metadata", finalPath)
			}
		case "dir":
			exists, err := directoryExists(finalPath)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("target directory %q is missing for published receipt manifest", finalPath)
			}
		case "symlink":
			same, exists, err := symlinkTargetState(finalPath, entry.SymlinkTarget)
			if err != nil {
				return err
			}
			if !exists || !same {
				return fmt.Errorf("target symlink %q does not match published receipt manifest", finalPath)
			}
		default:
			return fmt.Errorf("recover manifest entry %q uses unsupported kind %q", entry.Path, entry.Kind)
		}
	}
	return nil
}

func sessionIDFromRecordPath(path string) string {
	sessionDir := filepath.Dir(path)
	if filepath.Base(path) != "session.json" {
		return ""
	}
	return filepath.Base(sessionDir)
}

func recoverStagedSession(layout transaction.Layout, p profile.Profile, targetDir string, record transaction.SessionRecord, now time.Time) error {
	if record.State != transaction.StateStaged {
		return fmt.Errorf("session %q state is %q, want staged", record.ID, record.State)
	}
	manifestPath, err := control.Path(targetDir, control.ArtifactManifest, record.ID)
	if err != nil {
		return err
	}
	manifest, err := control.ReadManifestCompatFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read recover manifest %q: %w", record.ID, err)
	}
	if manifest.SessionID != record.ID {
		return fmt.Errorf("recover manifest session_id %q does not match session %q", manifest.SessionID, record.ID)
	}
	if manifest.ID != "manifest-"+record.ID {
		return fmt.Errorf("recover manifest id %q does not match session %q", manifest.ID, record.ID)
	}
	if manifest.CreatedAt != record.CreatedAt.UTC().Format(time.RFC3339Nano) && manifest.CreatedAt != record.CreatedAt.UTC().Format(time.RFC3339) {
		return fmt.Errorf("recover manifest created_at %q does not match session %q created_at %q", manifest.CreatedAt, record.ID, record.CreatedAt.UTC().Format(time.RFC3339Nano))
	}
	if manifest.RootID != "" && manifest.RootID != p.Roots[0].ID {
		return fmt.Errorf("recover manifest root_id %q does not match profile root %q", manifest.RootID, p.Roots[0].ID)
	}
	if err := validateRecoverProfileSnapshot(targetDir, p, manifest); err != nil {
		return err
	}
	if err := validatePreviousEvidenceForRecover(p, targetDir, manifest); err != nil {
		return err
	}
	if err := validateRecoverableStagedFiles(layout, targetDir, record.ID, manifest.Entries); err != nil {
		return err
	}
	recoveryWarnings := recoverWarningsForUnsupportedEntries(manifest.Entries)
	if len(recoveryWarnings) > 0 {
		if err := writeWarningArtifacts(targetDir, record.ID, now, recoveryWarnings); err != nil {
			return err
		}
	}
	existingDirs, err := captureExistingPublishDirs(targetDir, manifest.Entries)
	if err != nil {
		return err
	}
	if err := publishStaged(layout, targetDir, record.ID, manifest.Entries, existingDirs, publishModeRecover); err != nil {
		return err
	}
	if err := writeSessionReceiptWithTimes(targetDir, p, record.ID, record.CreatedAt, now); err != nil {
		return err
	}
	published, err := record.WithState(transaction.StatePublished, now)
	if err != nil {
		return err
	}
	return layout.WriteSessionRecord(published)
}

func validateRecoverProfileSnapshot(targetDir string, p profile.Profile, manifest control.Manifest) error {
	snapshotPath, err := control.Path(targetDir, control.ArtifactProfileSnapshot, "profile-"+manifest.SessionID)
	if err != nil {
		return err
	}
	snapshot, err := control.ReadFile[control.ProfileSnapshot](snapshotPath)
	if err != nil {
		return fmt.Errorf("read recover profile snapshot %q: %w", manifest.SessionID, err)
	}
	if snapshot.ID != "profile-"+manifest.SessionID {
		return fmt.Errorf("recover profile snapshot id %q does not match session %q", snapshot.ID, manifest.SessionID)
	}
	if snapshot.SessionID != "" && snapshot.SessionID != manifest.SessionID {
		return fmt.Errorf("recover profile snapshot session_id %q does not match session %q", snapshot.SessionID, manifest.SessionID)
	}
	if snapshot.ProfileID != p.ProfileID {
		return fmt.Errorf("recover profile snapshot profile_id %q does not match current profile %q", snapshot.ProfileID, p.ProfileID)
	}
	var snapProfile profile.Profile
	if err := json.Unmarshal(snapshot.Profile, &snapProfile); err != nil {
		return fmt.Errorf("decode recover profile snapshot %q: %w", manifest.SessionID, err)
	}
	if snapProfile.ProfileID != p.ProfileID {
		return fmt.Errorf("recover embedded profile_id %q does not match current profile %q", snapProfile.ProfileID, p.ProfileID)
	}
	if snapProfile.Target.TargetID != p.Target.TargetID {
		return fmt.Errorf("recover target_id %q does not match current target %q", snapProfile.Target.TargetID, p.Target.TargetID)
	}
	if manifest.RootID != "" {
		foundRoot := false
		for _, root := range snapProfile.Roots {
			if root.ID == manifest.RootID {
				foundRoot = true
				break
			}
		}
		if !foundRoot {
			return fmt.Errorf("recover manifest root_id %q is not present in profile snapshot", manifest.RootID)
		}
	}
	return nil
}

func validatePreviousEvidenceForRecover(p profile.Profile, targetDir string, manifest control.Manifest) error {
	for _, entry := range manifest.Entries {
		if entry.Kind != "file" || !previousFileEvidenceComplete(previousEvidenceFromManifestEntry(entry)) {
			continue
		}
		previousReceiptPath, err := control.Path(targetDir, control.ArtifactSessionReceipt, entry.PreviousSessionID)
		if err != nil {
			return err
		}
		previousReceipt, err := control.ReadFile[control.SessionReceipt](previousReceiptPath)
		if err != nil {
			return fmt.Errorf("read previous receipt %q for recover evidence: %w", entry.PreviousSessionID, err)
		}
		if previousReceipt.Status != "published" {
			return fmt.Errorf("previous receipt %q status = %q, want published", previousReceipt.ID, previousReceipt.Status)
		}
		if previousReceipt.ID != entry.PreviousSessionID {
			return fmt.Errorf("previous receipt %q id = %q", entry.PreviousSessionID, previousReceipt.ID)
		}
		if previousReceipt.ProfileID != p.ProfileID || previousReceipt.TargetID != p.Target.TargetID {
			return fmt.Errorf("previous receipt %q scope does not match current profile/target", previousReceipt.ID)
		}
		previousManifestPath, err := control.Path(targetDir, control.ArtifactManifest, entry.PreviousSessionID)
		if err != nil {
			return err
		}
		previousManifest, err := control.ReadManifestCompatFile(previousManifestPath)
		if err != nil {
			return fmt.Errorf("read previous manifest %q for recover evidence: %w", entry.PreviousSessionID, err)
		}
		if previousManifest.ID != entry.PreviousManifestID {
			return fmt.Errorf("previous manifest %q id = %q, want %q", entry.PreviousSessionID, previousManifest.ID, entry.PreviousManifestID)
		}
		if previousManifest.SessionID != entry.PreviousSessionID {
			return fmt.Errorf("previous manifest %q session_id = %q", entry.PreviousSessionID, previousManifest.SessionID)
		}
		if previousManifest.RootID != p.Roots[0].ID && !(previousManifest.RootID == "" && len(p.Roots) == 1) {
			return fmt.Errorf("previous manifest %q root_id %q does not match current root %q", previousManifest.ID, previousManifest.RootID, p.Roots[0].ID)
		}
		previousEntry, ok := manifestFileEntryByTarget(previousManifest, targetPath(entry))
		if !ok {
			return fmt.Errorf("previous manifest %q does not contain target path %q", previousManifest.ID, targetPath(entry))
		}
		if previousEntry.Size != entry.PreviousSize || previousEntry.Digest != entry.PreviousDigest {
			return fmt.Errorf("previous manifest %q evidence for %q does not match staged manifest", previousManifest.ID, targetPath(entry))
		}
		if previousEntry.Mode != entry.PreviousMode || previousEntry.ModTime != entry.PreviousModTime {
			return fmt.Errorf("previous manifest %q metadata for %q does not match staged manifest", previousManifest.ID, targetPath(entry))
		}
	}
	return nil
}

func manifestFileEntryByTarget(manifest control.Manifest, target string) (control.ManifestEntry, bool) {
	cleanTarget := cleanManifestTarget(target)
	for _, entry := range manifest.Entries {
		if entry.Kind != "file" {
			continue
		}
		if cleanManifestTarget(targetPath(entry)) == cleanTarget {
			return entry, true
		}
	}
	return control.ManifestEntry{}, false
}

func validateRecoverableStagedFiles(layout transaction.Layout, targetDir string, sessionID string, entries []control.ManifestEntry) error {
	return preflightPublishPlan(layout, targetDir, sessionID, entries, publishModeRecover)
}

func preflightPublishPlan(layout transaction.Layout, targetDir string, sessionID string, entries []control.ManifestEntry, mode publishMode) error {
	if err := validateTargetPlan(entries, func(entry control.ManifestEntry) (string, string) {
		return targetPath(entry), entry.Kind
	}); err != nil {
		return err
	}
	for _, entry := range entries {
		entryTarget := targetPath(entry)
		if pathguard.IsReservedControlPath(entryTarget) {
			return fmt.Errorf("manifest target path %q uses reserved control directory", entryTarget)
		}
		finalPath, err := targetPathForManifestEntry(targetDir, entry)
		if err != nil {
			return err
		}
		switch entry.Kind {
		case "file":
			same, exists, err := targetFileContentState(finalPath, entry.Size, entry.Digest)
			if err != nil {
				return err
			}
			previous := previousEvidenceFromManifestEntry(entry)
			if exists {
				if same {
					if mode == publishModeRecover {
						manifestSame, err := targetMatchesManifestFile(finalPath, entry)
						if err != nil {
							return err
						}
						if !manifestSame {
							return fmt.Errorf("target file %q already matches new content but not staged manifest metadata; refusing to complete recovery", finalPath)
						}
					}
					if previousFileEvidenceComplete(previous) && mode != publishModeRecover {
						previousSame, err := targetMatchesPreviousFile(finalPath, previous)
						if err != nil {
							return err
						}
						if !previousSame {
							return fmt.Errorf("target file %q already matches new content but not previous manifest evidence; refusing to accept external replacement", finalPath)
						}
					}
					continue
				}
				previousSame, err := targetMatchesPreviousFile(finalPath, previous)
				if err != nil {
					return err
				}
				if previousSame {
					if err := validateStagedManifestFile(layout, sessionID, entry); err != nil {
						return err
					}
					continue
				}
				return fmt.Errorf("target file %q already exists with different content; refusing to overwrite", finalPath)
			}
			if previousFileEvidenceComplete(previous) {
				holdSame, err := replacementHoldMatchesPrevious(targetDir, sessionID, entryTarget, previous)
				if err != nil {
					return err
				}
				if !holdSame {
					return fmt.Errorf("target file %q is missing for managed replacement; refusing to publish without previous target evidence", finalPath)
				}
			}
			if err := validateStagedManifestFile(layout, sessionID, entry); err != nil {
				return err
			}
		case "dir":
			if _, err := directoryExists(finalPath); err != nil {
				return err
			}
		case "symlink":
			if err := pathguard.ValidateRelativeSymlinkTarget(entry.SymlinkTarget); err != nil {
				return fmt.Errorf("recover symlink %q: %w", entry.Path, err)
			}
			same, exists, err := symlinkTargetState(finalPath, entry.SymlinkTarget)
			if err != nil {
				return err
			}
			if exists && !same {
				return fmt.Errorf("target symlink %q already exists with different target; refusing to overwrite", finalPath)
			}
		default:
			return fmt.Errorf("recover manifest entry %q uses unsupported kind %q", entry.Path, entry.Kind)
		}
	}
	return nil
}

func validateTargetPlan(entries []control.ManifestEntry, targetAndKind func(control.ManifestEntry) (string, string)) error {
	seen := map[string]string{}
	blockingLeaf := map[string]string{}
	for _, entry := range entries {
		if entry.Kind != "file" && entry.Kind != "dir" && entry.Kind != "symlink" {
			return fmt.Errorf("manifest entry %q uses unsupported kind %q", entry.Path, entry.Kind)
		}
		target, kind := targetAndKind(entry)
		clean := cleanManifestTarget(target)
		if previous, ok := seen[clean]; ok {
			return fmt.Errorf("manifest target path %q is used by both %q and %q", clean, previous, entry.Path)
		}
		for parent := filepath.Dir(clean); parent != "." && parent != "/"; parent = filepath.Dir(parent) {
			if previous, ok := blockingLeaf[parent]; ok {
				return fmt.Errorf("manifest target path %q is below non-directory target %q from %q", clean, parent, previous)
			}
		}
		seen[clean] = entry.Path
		if kind != "dir" {
			blockingLeaf[clean] = entry.Path
		}
	}
	for _, entry := range entries {
		target, kind := targetAndKind(entry)
		if kind == "dir" {
			continue
		}
		clean := cleanManifestTarget(target)
		for other := range seen {
			if other != clean && strings.HasPrefix(other, clean+"/") {
				return fmt.Errorf("manifest target path %q has descendant target %q but is not a directory", clean, other)
			}
		}
	}
	return nil
}

func cleanManifestTarget(target string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(target)))
}

func validateStagedManifestFile(layout transaction.Layout, sessionID string, entry control.ManifestEntry) error {
	if strings.TrimSpace(entry.Digest) == "" {
		return fmt.Errorf("staged file %q is missing digest", entry.Path)
	}
	if !entry.HasSizeEvidence() {
		return fmt.Errorf("staged file %q is missing size evidence", entry.Path)
	}
	if !entry.HasModeEvidence() {
		return fmt.Errorf("staged file %q is missing mode evidence", entry.Path)
	}
	if strings.TrimSpace(entry.ModTime) == "" {
		return fmt.Errorf("staged file %q is missing mod_time evidence", entry.Path)
	}
	stagePath, err := pathguard.SafeJoinParent(layout.StagingDir(sessionID), entry.Path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(stagePath)
	if err != nil {
		return fmt.Errorf("stat staged file %q: %w", entry.Path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("staged file %q is not a regular file", entry.Path)
	}
	if info.Size() != entry.Size {
		return fmt.Errorf("staged file %q size = %d, want %d", entry.Path, info.Size(), entry.Size)
	}
	got, err := digestFile(stagePath)
	if err != nil {
		return err
	}
	if got != entry.Digest {
		return fmt.Errorf("staged file %q digest = %s, want %s", entry.Path, got, entry.Digest)
	}
	return nil
}

func recoverWarningsForUnsupportedEntries(entries []control.ManifestEntry) []audit.Record {
	var warnings []audit.Record
	for _, entry := range entries {
		switch entry.Kind {
		case "dir", "file", "symlink":
			continue
		default:
			warnings = append(warnings, audit.New(entry.Path, targetPath(entry), audit.SeverityWarning, "unsupported_manifest_entry_not_published", "manifest entry kind is not published by local recover"))
		}
	}
	return warnings
}

func markSessionNeedsRepair(layout transaction.Layout, record transaction.SessionRecord, now time.Time, cause error) error {
	repair, err := record.WithState(transaction.StateNeedsRepair, now)
	if err != nil {
		return err
	}
	repair.Note = cause.Error()
	if err := layout.WriteSessionRecord(repair); err != nil {
		return err
	}
	return nil
}

func targetPathForEntry(targetDir string, entry scan.Entry) (string, error) {
	switch entry.Kind {
	case scan.KindDir:
		return pathguard.SafeJoinDirectory(targetDir, entry.Path)
	default:
		return pathguard.SafeJoinParent(targetDir, entry.Path)
	}
}

func ValidateSupportedRules(p profile.Profile) error {
	if len(p.Exclude) > 0 {
		return fmt.Errorf("exclude rules are not implemented in local push yet")
	}
	if len(p.Include) == 0 {
		return nil
	}
	if len(p.Include) == 1 && p.Include[0].Pattern == "**" {
		return nil
	}
	return fmt.Errorf("custom include rules are not implemented in local push yet")
}

func ValidateProfileForLocalPush(p profile.Profile) error {
	if len(p.Roots) != 1 {
		return fmt.Errorf("local push requires exactly one root for now")
	}
	if err := ValidateSupportedRules(p); err != nil {
		return err
	}
	if p.Consistency != profile.ConsistencyStrict {
		return fmt.Errorf("consistency=%q is not implemented in local push; only strict is supported", p.Consistency)
	}
	if p.DeletePolicy.Mode == profile.DeleteModePrune || p.DeletePolicy.AllowPhysicalPrune {
		return fmt.Errorf("physical prune is not implemented in local push; use delete_policy.mode=record or ignore")
	}
	if p.PrivacyPolicy.Mode != profile.PrivacyModePlaintext {
		return fmt.Errorf("privacy_policy.mode=%q is not implemented in local push; target files are restored as plaintext", p.PrivacyPolicy.Mode)
	}
	if !p.PrivacyPolicy.AllowHiddenFiles {
		return fmt.Errorf("privacy_policy.allow_hidden_files=false is not implemented in local push; hidden files are always included")
	}
	if !p.PrivacyPolicy.AllowSensitiveFilenames {
		return fmt.Errorf("privacy_policy.allow_sensitive_filenames=false is not implemented in local push; sensitive filenames are always included")
	}
	if !reflect.DeepEqual(p.PrivacyPolicy, profile.DefaultPrivacyPolicy()) {
		return fmt.Errorf("custom privacy_policy transport settings are not implemented in local push yet")
	}
	if p.MetadataPolicy.PreserveExtendedAttr {
		return fmt.Errorf("metadata_policy.preserve_extended_attr=true is not implemented in local push")
	}
	if !p.MetadataPolicy.PreservePermissions {
		return fmt.Errorf("metadata_policy.preserve_permissions=false is not implemented in local push; permissions are always preserved")
	}
	if !p.MetadataPolicy.PreserveModTime {
		return fmt.Errorf("metadata_policy.preserve_mod_time=false is not implemented in local push; modification times are always preserved")
	}
	if !reflect.DeepEqual(p.AgentKnowledge, profile.DefaultAgentKnowledge()) {
		return fmt.Errorf("custom agent_knowledge categories are not implemented in local push yet")
	}
	return nil
}

func agentKnowledgeCategories(config profile.AgentKnowledge) []agentkb.KnowledgeCategory {
	categories := make([]agentkb.KnowledgeCategory, 0, len(config.Categories))
	for _, category := range config.Categories {
		categories = append(categories, agentkb.KnowledgeCategory{
			Name:     agentkb.Category(category.Name),
			Paths:    append([]string(nil), category.Paths...),
			Manifest: category.Manifest,
		})
	}
	return categories
}

func ValidateSourceTargetSeparation(sourceRoot, targetDir string) error {
	sourceAbs, err := filepath.Abs(sourceRoot)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	if sourceAbs == targetAbs {
		return fmt.Errorf("source root and target directory must be different")
	}
	if inside(sourceAbs, targetAbs) {
		return fmt.Errorf("target directory must not be inside the source root")
	}
	if inside(targetAbs, sourceAbs) {
		return fmt.Errorf("source root must not be inside the target directory")
	}
	sourceReal, sourceErr := pathguard.CanonicalPath(sourceAbs)
	targetReal, targetErr := pathguard.CanonicalPath(targetAbs)
	if sourceErr != nil || targetErr != nil {
		return nil
	}
	if sourceReal == sourceAbs && targetReal == targetAbs {
		return nil
	}
	return validateSeparatedAbs(sourceReal, targetReal)
}

func validateSeparatedAbs(sourceAbs, targetAbs string) error {
	if sourceAbs == "" || targetAbs == "" {
		return nil
	}
	if sourceAbs == targetAbs {
		return fmt.Errorf("source root and target directory must be different")
	}
	if inside(sourceAbs, targetAbs) {
		return fmt.Errorf("target directory must not be inside the source root")
	}
	if inside(targetAbs, sourceAbs) {
		return fmt.Errorf("source root must not be inside the target directory")
	}
	return nil
}

func lockTargetSession(targetDir, sessionID string) (func(), error) {
	target, err := pathguard.CanonicalPath(targetDir)
	if err != nil {
		return nil, err
	}
	locksDir := filepath.Join(control.ControlDir(targetDir), "locks")
	if err := pathguard.EnsurePlainDirectory(targetDir, locksDir, 0o700); err != nil {
		return nil, err
	}

	targetValue, _ := localPushLocks.LoadOrStore(target+"\x00target", &sync.Mutex{})
	targetMu := targetValue.(*sync.Mutex)
	targetMu.Lock()
	unlockTargetFile, err := filelock.LockInDir(locksDir, "target.lock")
	if err != nil {
		targetMu.Unlock()
		return nil, err
	}

	sessionValue, _ := localPushLocks.LoadOrStore(target+"\x00session\x00"+sessionID, &sync.Mutex{})
	sessionMu := sessionValue.(*sync.Mutex)
	sessionMu.Lock()
	unlockSessionFile, err := filelock.LockInDir(locksDir, sessionID+".lock")
	if err != nil {
		sessionMu.Unlock()
		unlockTargetFile()
		targetMu.Unlock()
		return nil, err
	}
	return func() {
		unlockSessionFile()
		sessionMu.Unlock()
		unlockTargetFile()
		targetMu.Unlock()
	}, nil
}

func inside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func ensureSessionUnused(targetDir, sessionID string) error {
	receiptPath, err := control.Path(targetDir, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(receiptPath); err == nil {
		return fmt.Errorf("session %q is already published", sessionID)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat session receipt: %w", err)
	}
	recordPath := transaction.NewLayout(control.ControlDir(targetDir)).RecordPath(sessionID)
	if _, err := os.Stat(recordPath); err == nil {
		return fmt.Errorf("session %q already has local state; recovery/resume is required before reuse", sessionID)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat session record: %w", err)
	}
	return nil
}

func refuseUnhealthyRecoveryState(p profile.Profile, targetDir string) error {
	scan, err := transaction.ScanRecovery(transaction.NewLayout(control.ControlDir(targetDir)))
	if err != nil {
		return err
	}
	for _, invalid := range scan.Invalid {
		if isPublishedLegacySession(p, targetDir, invalid.SessionID, invalid.Err) {
			continue
		}
		return fmt.Errorf("target has invalid recovery state for session %q; run health or recover before starting a new push", invalid.SessionID)
	}
	if len(scan.Items) > 0 {
		item := scan.Items[0]
		return fmt.Errorf("target has nonterminal recovery state for session %q (%s); run health or recover before starting a new push", item.Record.ID, item.Record.State)
	}
	return nil
}

func isPublishedLegacySession(p profile.Profile, targetDir, sessionID string, recordErr error) bool {
	if !errors.Is(recordErr, os.ErrNotExist) {
		return false
	}
	receiptPath, err := control.Path(targetDir, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return false
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		return false
	}
	if receipt.ID != sessionID || receipt.Status != "published" {
		return false
	}
	if receipt.ProfileID != p.ProfileID || receipt.TargetID != p.Target.TargetID {
		return false
	}
	manifestPath, err := control.Path(targetDir, control.ArtifactManifest, sessionID)
	if err != nil {
		return false
	}
	manifest, err := control.ReadManifestCompatFile(manifestPath)
	if err != nil {
		return false
	}
	if manifest.ID != "manifest-"+sessionID || manifest.SessionID != sessionID {
		return false
	}
	if manifest.RootID != p.Roots[0].ID && !(manifest.RootID == "" && len(p.Roots) == 1) {
		return false
	}
	stamp := manifest.CreatedAt
	if stamp == "" {
		stamp = receipt.StartedAt
	}
	if _, err := parseArtifactTime(stamp); err != nil {
		return false
	}
	if err := validateLegacyPublishedManifestTargets(targetDir, manifest); err != nil {
		return false
	}
	return true
}

func validateLegacyPublishedManifestTargets(targetDir string, manifest control.Manifest) error {
	if err := validateTargetPlan(manifest.Entries, func(entry control.ManifestEntry) (string, string) {
		return targetPath(entry), entry.Kind
	}); err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		entryTarget := targetPath(entry)
		if pathguard.IsReservedControlPath(entryTarget) {
			return fmt.Errorf("manifest target path %q uses reserved control directory", entryTarget)
		}
		if _, err := targetPathForManifestEntry(targetDir, entry); err != nil {
			return err
		}
		if entry.Kind == "file" && isSHA256Digest(entry.Digest) && entry.HasSizeEvidence() {
			finalPath, err := targetPathForManifestEntry(targetDir, entry)
			if err != nil {
				return err
			}
			same, exists, err := targetFileContentState(finalPath, entry.Size, entry.Digest)
			if err != nil {
				return err
			}
			if exists && !same {
				return fmt.Errorf("legacy target file %q does not match manifest evidence", finalPath)
			}
		}
	}
	return nil
}

func softDeletesForRun(p profile.Profile, previous control.Manifest, hasPrevious bool, scanResult scan.Result, sessionID string, now time.Time) ([]control.SoftDelete, error) {
	if p.DeletePolicy.Mode == profile.DeleteModeIgnore || len(scanResult.Entries) == 0 {
		return nil, nil
	}
	if !hasPrevious {
		return nil, nil
	}
	result, err := deleted.Generate(deleted.Options{
		PreviousManifest: previous,
		CurrentScan:      scanResult,
		SessionID:        sessionID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		RootID:           p.Roots[0].ID,
		DetectedAt:       now,
	})
	if err != nil {
		return nil, err
	}
	return result.Records, nil
}

func previousManifestEntries(previous control.Manifest, ok bool) map[string]previousFileEvidence {
	out := map[string]previousFileEvidence{}
	if !ok {
		return out
	}
	for _, entry := range previous.Entries {
		if entry.Kind != "file" || !isSHA256Digest(entry.Digest) {
			continue
		}
		evidence := previousFileEvidence{
			SessionID:  previous.SessionID,
			ManifestID: previous.ID,
			Size:       entry.Size,
			Digest:     entry.Digest,
			Mode:       entry.Mode,
			ModTime:    entry.ModTime,
			HasSize:    entry.HasSizeEvidence(),
			HasMode:    entry.HasModeEvidence(),
		}
		if previousFileEvidenceComplete(evidence) {
			out[cleanManifestTarget(targetPath(entry))] = evidence
		}
	}
	return out
}

func latestPublishedManifest(p profile.Profile, targetDir string) (control.Manifest, bool, error) {
	sessionsDir := filepath.Join(control.ControlDir(targetDir), "sessions")
	sessionDirs, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return control.Manifest{}, false, nil
		}
		return control.Manifest{}, false, fmt.Errorf("read previous sessions: %w", err)
	}

	var latest control.Manifest
	var latestStamp time.Time
	found := false
	for _, sessionDir := range sessionDirs {
		if !sessionDir.IsDir() {
			continue
		}
		sessionID := sessionDir.Name()
		receiptPath, err := control.Path(targetDir, control.ArtifactSessionReceipt, sessionID)
		if err != nil {
			return control.Manifest{}, false, err
		}
		receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return control.Manifest{}, false, fmt.Errorf("read previous receipt %q: %w", sessionID, err)
		}
		if receipt.Status != "published" {
			continue
		}
		if receipt.ID != sessionID {
			return control.Manifest{}, false, fmt.Errorf("published receipt %q id = %q", sessionID, receipt.ID)
		}
		if receipt.ProfileID != p.ProfileID || receipt.TargetID != p.Target.TargetID {
			continue
		}
		manifestPath, err := control.Path(targetDir, control.ArtifactManifest, sessionID)
		if err != nil {
			return control.Manifest{}, false, err
		}
		manifest, err := control.ReadManifestCompatFile(manifestPath)
		if err != nil {
			return control.Manifest{}, false, fmt.Errorf("read previous manifest %q: %w", sessionID, err)
		}
		if manifest.SessionID != sessionID {
			return control.Manifest{}, false, fmt.Errorf("published manifest %q session_id = %q", sessionID, manifest.SessionID)
		}
		rootID := p.Roots[0].ID
		if manifest.RootID != rootID && !(manifest.RootID == "" && len(p.Roots) == 1) {
			continue
		}
		stamp := manifest.CreatedAt
		if stamp == "" {
			stamp = receipt.StartedAt
		}
		parsedStamp, err := parseArtifactTime(stamp)
		if err != nil {
			return control.Manifest{}, false, fmt.Errorf("parse previous manifest time %q for session %q: %w", stamp, sessionID, err)
		}
		if !found || parsedStamp.After(latestStamp) || (parsedStamp.Equal(latestStamp) && manifest.SessionID > latest.SessionID) {
			latest = manifest
			latestStamp = parsedStamp
			found = true
		}
	}
	return latest, found, nil
}

func dropControlPlaneEntries(result scan.Result) scan.Result {
	entries := result.Entries[:0]
	skipped := 0
	firstPath := ""
	for _, entry := range result.Entries {
		if isControlPlanePath(entry.Path) {
			skipped++
			if firstPath == "" {
				firstPath = entry.Path
			}
			continue
		}
		entries = append(entries, entry)
	}
	result.Entries = entries
	if skipped > 0 {
		result.Audit = append(result.Audit, audit.WithSuggestedConfig(
			audit.WithDetected(
				audit.New(control.DirName, "", audit.SeverityWarning, "reserved_control_plane_skipped", "source .supermover is reserved for target control artifacts and was not copied"),
				map[string]string{"entries": fmt.Sprintf("%d", skipped), "first_path": firstPath},
			),
			map[string]string{
				"append_migration_path": control.DirName,
				"reason":                "review whether the source .supermover directory is application data that should be migrated separately",
			},
		))
	}
	return result
}

func isControlPlanePath(path string) bool {
	return pathguard.IsReservedControlPath(path)
}

func rejectScanErrors(result scan.Result) error {
	for _, record := range result.Audit {
		if record.Kind == "scan_error" {
			return fmt.Errorf("source scan error at %q; rerun after the source is readable before publishing or recording soft deletes", record.Path)
		}
	}
	return nil
}

func parseArtifactTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("timestamp is required")
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts.UTC(), nil
	}
	return time.Parse(time.RFC3339, value)
}

func copyRegularToStage(sourcePath, stagePath string, entry scan.Entry) (string, error) {
	return copyRegularToStageWithPostCopy(sourcePath, stagePath, entry, nil)
}

func preflightRegularTarget(sourcePath, targetPath string, previous previousFileEvidence) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source file %q: %w", sourcePath, err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("source file %q is no longer regular", sourcePath)
	}
	sourceDigest, err := digestFile(sourcePath)
	if err != nil {
		return err
	}
	same, exists, err := targetFileContentState(targetPath, sourceInfo.Size(), sourceDigest)
	if err != nil {
		return err
	}
	if exists && same && previousFileEvidenceComplete(previous) {
		previousSame, err := targetMatchesPreviousFile(targetPath, previous)
		if err != nil {
			return err
		}
		if !previousSame {
			return fmt.Errorf("target file %q already matches new content but not previous manifest evidence; refusing to accept external replacement", targetPath)
		}
	}
	if exists && !same {
		previousSame, err := targetMatchesPreviousFile(targetPath, previous)
		if err != nil {
			return err
		}
		if previousSame {
			return nil
		}
		return fmt.Errorf("target file %q already exists with different content; refusing to overwrite", targetPath)
	}
	if !exists && previousFileEvidenceComplete(previous) {
		return fmt.Errorf("target file %q is missing for managed replacement; refusing to publish without previous target evidence", targetPath)
	}
	return nil
}

func preflightSymlinkTarget(targetPath string, symlinkTarget string) error {
	if err := pathguard.ValidateRelativeSymlinkTarget(symlinkTarget); err != nil {
		return fmt.Errorf("symlink target for %q is unsafe: %w", targetPath, err)
	}
	same, exists, err := symlinkTargetState(targetPath, symlinkTarget)
	if err != nil {
		return err
	}
	if exists && !same {
		return fmt.Errorf("%w: target symlink %q already exists with different target; refusing to overwrite", errSymlinkTargetConflict, targetPath)
	}
	return nil
}

func ensureDirectoryPreflight(targetPath string) error {
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat target directory %q: %w", targetPath, err)
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	return fmt.Errorf("target directory %q already exists as non-directory; refusing to overwrite", targetPath)
}

func directoryExists(targetPath string) (bool, error) {
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat target directory %q: %w", targetPath, err)
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return true, nil
	}
	return false, fmt.Errorf("target directory %q already exists as non-directory; refusing to overwrite", targetPath)
}

func symlinkTargetState(path string, target string) (same bool, exists bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("stat target symlink %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, true, fmt.Errorf("%w: target symlink %q already exists as non-symlink; refusing to overwrite", errSymlinkTargetConflict, path)
	}
	got, err := os.Readlink(path)
	if err != nil {
		return false, true, fmt.Errorf("read target symlink %q: %w", path, err)
	}
	return got == target, true, nil
}

func copyRegularWithPostCopy(sourcePath, targetPath string, mode os.FileMode, modTime time.Time, postCopy func() error) (string, error) {
	stagePath := targetPath + ".stage"
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("stat source file %q before copy: %w", sourcePath, err)
	}
	entry := scan.Entry{
		Path:    filepath.Base(sourcePath),
		Kind:    scan.KindRegular,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
	}
	digest, err := copyRegularToStageWithPostCopy(sourcePath, stagePath, entry, postCopy)
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(stagePath)
		}
	}()
	info, err = os.Stat(stagePath)
	if err != nil {
		return "", fmt.Errorf("stat staged file before publish: %w", err)
	}
	same, exists, err := targetFileContentState(targetPath, info.Size(), digest)
	if err != nil {
		return "", err
	}
	if exists {
		if same {
			return digest, nil
		}
		return "", fmt.Errorf("target file %q already exists with different content; refusing to overwrite", targetPath)
	}
	if err := applyFileMetadata(stagePath, mode, modTime); err != nil {
		return "", err
	}
	if err := durable.PromoteFileNoReplace(stagePath, targetPath); err != nil {
		return "", err
	}
	cleanup = false
	return digest, nil
}

type existingDirMeta struct {
	Mode    os.FileMode
	ModTime time.Time
}

func captureExistingPublishDirs(targetDir string, entries []control.ManifestEntry) (map[string]existingDirMeta, error) {
	out := map[string]existingDirMeta{}
	for _, entry := range entries {
		var dirPath string
		var err error
		switch entry.Kind {
		case "dir":
			dirPath, err = targetPathForManifestEntry(targetDir, entry)
		case "file", "symlink":
			filePath, err := targetPathForManifestEntry(targetDir, entry)
			if err == nil {
				dirPath = filepath.Dir(filePath)
			}
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, dir := range ancestorDirs(targetDir, dirPath) {
			if _, ok := out[dir]; ok {
				continue
			}
			info, err := os.Lstat(dir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("stat target directory %q: %w", dir, err)
			}
			if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				out[dir] = existingDirMeta{Mode: info.Mode().Perm(), ModTime: info.ModTime()}
			}
		}
	}
	return out, nil
}

func ancestorDirs(root, dir string) []string {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}
	var dirs []string
	current := filepath.Clean(dirAbs)
	for {
		rel, err := filepath.Rel(rootAbs, current)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			break
		}
		dirs = append(dirs, current)
		if current == rootAbs {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return dirs
}

func publishStaged(layout transaction.Layout, targetDir string, sessionID string, entries []control.ManifestEntry, existingDirs map[string]existingDirMeta, mode publishMode) (err error) {
	defer func() {
		if restoreErr := restoreExistingDirs(existingDirs); restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
	}()
	for _, entry := range entries {
		entryTarget := targetPath(entry)
		targetPath, err := targetPathForManifestEntry(targetDir, entry)
		if err != nil {
			return err
		}
		if pathguard.IsReservedControlPath(entryTarget) {
			return fmt.Errorf("manifest target path %q uses reserved control directory", entryTarget)
		}
		switch entry.Kind {
		case "dir":
			mode := os.FileMode(entry.Mode)
			if !entry.HasModeEvidence() {
				mode = 0o755
			}
			existed, err := directoryExists(targetPath)
			if err != nil {
				return err
			}
			if err := pathguard.EnsurePlainDirectory(targetDir, targetPath, mode); err != nil {
				return fmt.Errorf("publish directory %q: %w", entry.Path, err)
			}
			if !existed {
				if err := applyFileMetadata(targetPath, mode, parseManifestModTime(entry.ModTime)); err != nil {
					return err
				}
			}
		case "file":
			stagePath, err := pathguard.SafeJoinParent(layout.StagingDir(sessionID), entry.Path)
			if err != nil {
				return err
			}
			if beforePublishStaged != nil {
				if err := beforePublishStaged(entry, targetPath); err != nil {
					return err
				}
			}
			previous := previousEvidenceFromManifestEntry(entry)
			same, exists, err := targetFileContentState(targetPath, entry.Size, entry.Digest)
			if err != nil {
				return err
			}
			if exists {
				if same {
					if previousFileEvidenceComplete(previous) && mode != publishModeRecover {
						previousSame, err := targetMatchesPreviousFile(targetPath, previous)
						if err != nil {
							return err
						}
						manifestSame, err := targetMatchesManifestFile(targetPath, entry)
						if err != nil {
							return err
						}
						if !previousSame {
							return fmt.Errorf("target file %q already matches new content but not previous manifest evidence; refusing to accept external replacement", targetPath)
						}
						if !manifestSame {
							if err := publishManagedReplacement(stagePath, targetPath, targetDir, sessionID, entry, previous); err != nil {
								return err
							}
							continue
						}
					}
					if previousFileEvidenceComplete(previous) && mode == publishModeRecover {
						manifestSame, err := targetMatchesManifestFile(targetPath, entry)
						if err != nil {
							return err
						}
						if !manifestSame {
							return fmt.Errorf("target file %q already matches new content but not staged manifest metadata; refusing to complete managed replacement", targetPath)
						}
						if err := removeMatchingReplacementHoldsIfPresent(targetDir, sessionID, entry, previous); err != nil {
							return err
						}
					}
					if err := removeStagedIfPresent(stagePath, entry.Path); err != nil {
						return err
					}
					continue
				}
				previousSame, err := targetMatchesPreviousFile(targetPath, previous)
				if err != nil {
					return err
				}
				if !previousSame {
					return fmt.Errorf("target file %q already exists with different content; refusing to overwrite", targetPath)
				}
				if err := publishManagedReplacement(stagePath, targetPath, targetDir, sessionID, entry, previous); err != nil {
					return err
				}
				continue
			}
			if !previousFileEvidenceComplete(previous) {
				if err := publishNewStagedFile(stagePath, targetDir, targetPath, entry); err != nil {
					return err
				}
				continue
			}
			holdSame, err := replacementHoldMatchesPrevious(targetDir, sessionID, entryTarget, previous)
			if err != nil {
				return err
			}
			if !holdSame {
				return fmt.Errorf("target file %q is missing for managed replacement; refusing to publish without previous target evidence", targetPath)
			}
			if err := publishHeldManagedReplacement(stagePath, targetPath, targetDir, sessionID, entry); err != nil {
				return err
			}
			continue
		case "symlink":
			if err := pathguard.ValidateRelativeSymlinkTarget(entry.SymlinkTarget); err != nil {
				return fmt.Errorf("publish symlink %q: %w", entry.Path, err)
			}
			same, exists, err := symlinkTargetState(targetPath, entry.SymlinkTarget)
			if err != nil {
				return err
			}
			if exists {
				if same {
					continue
				}
				return fmt.Errorf("target symlink %q already exists with different target; refusing to overwrite", targetPath)
			}
			if err := pathguard.EnsurePlainDirectory(targetDir, filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("publish symlink parent %q: %w", entry.Path, err)
			}
			if err := os.Symlink(entry.SymlinkTarget, targetPath); err != nil {
				if os.IsExist(err) {
					return fmt.Errorf("target symlink %q appeared before publish; refusing to overwrite", targetPath)
				}
				return fmt.Errorf("publish symlink %q: %w", entry.Path, err)
			}
			if err := durable.SyncDirBestEffort(filepath.Dir(targetPath)); err != nil {
				return err
			}
		}
	}
	return nil
}

func removeStagedIfPresent(stagePath string, entryPath string) error {
	if err := os.Remove(stagePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove duplicate staged file %q: %w", entryPath, err)
	}
	return nil
}

func publishNewStagedFile(stagePath, targetDir, targetPath string, entry control.ManifestEntry) error {
	if err := pathguard.EnsurePlainDirectory(targetDir, filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("publish file parent %q: %w", entry.Path, err)
	}
	mode := os.FileMode(entry.Mode)
	if !entry.HasModeEvidence() {
		mode = 0o644
	}
	if err := applyFileMetadata(stagePath, mode, parseManifestModTime(entry.ModTime)); err != nil {
		return err
	}
	return durable.PromoteFileNoReplace(stagePath, targetPath)
}

func restoreExistingDirs(dirs map[string]existingDirMeta) error {
	var errs []error
	for path, meta := range dirs {
		if err := os.Chmod(path, meta.Mode); err != nil {
			errs = append(errs, fmt.Errorf("restore directory permissions for %q: %w", path, err))
		}
		if err := os.Chtimes(path, meta.ModTime, meta.ModTime); err != nil {
			errs = append(errs, fmt.Errorf("restore directory modification time for %q: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func targetPathForManifestEntry(targetDir string, entry control.ManifestEntry) (string, error) {
	switch entry.Kind {
	case "dir":
		return pathguard.SafeJoinDirectory(targetDir, targetPath(entry))
	default:
		return pathguard.SafeJoinParent(targetDir, targetPath(entry))
	}
}

func targetPath(entry control.ManifestEntry) string {
	if entry.TargetPath != "" {
		return entry.TargetPath
	}
	return entry.Path
}

func parseManifestModTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func copyRegularToStageWithPostCopy(sourcePath, targetPath string, entry scan.Entry, postCopy func() error) (string, error) {
	stageRoot, err := stageRootForEntryPath(targetPath, entry.Path)
	if err != nil {
		return "", err
	}
	if err := pathguard.EnsurePlainDirectory(stageRoot, filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create staged parent %q: %w", filepath.Dir(targetPath), err)
	}
	before, err := os.Lstat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("stat source file %q before copy: %w", sourcePath, err)
	}
	if !entry.MatchesObservedRegular(before) {
		return "", fmt.Errorf("source file %q changed since scan; rerun after the source is stable", sourcePath)
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer in.Close()
	openedInfo, err := in.Stat()
	if err != nil {
		return "", fmt.Errorf("stat opened source file %q: %w", sourcePath, err)
	}
	if !entry.MatchesObservedRegular(openedInfo) || !sameSourceFile(before, openedInfo) {
		return "", fmt.Errorf("source file %q changed before copy opened; rerun after the source is stable", sourcePath)
	}
	beforeDigest, err := digestOpenFile(in)
	if err != nil {
		return "", fmt.Errorf("digest source file %q before copy: %w", sourcePath, err)
	}
	if entry.Digest != "" && beforeDigest != entry.Digest {
		return "", fmt.Errorf("source file %q changed since scan; rerun after the source is stable", sourcePath)
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind source file %q: %w", sourcePath, err)
	}

	temp, err := os.CreateTemp(filepath.Dir(targetPath), ".supermover-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create staged temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temp, hasher), in); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("copy %q to temp file: %w", sourcePath, err)
	}
	if err := temp.Chmod(entry.Mode.Perm()); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if postCopy != nil {
		if err := postCopy(); err != nil {
			return "", err
		}
	}
	after, err := os.Lstat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("stat source file %q after copy: %w", sourcePath, err)
	}
	if !entry.MatchesObservedRegular(after) || !sameSourceFile(before, after) {
		return "", fmt.Errorf("source file %q changed during copy; rerun after the source is stable", sourcePath)
	}
	digest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	afterDigest, err := digestFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("digest source file %q after copy: %w", sourcePath, err)
	}
	if beforeDigest != afterDigest || digest != afterDigest {
		return "", fmt.Errorf("source file %q changed during copy; rerun after the source is stable", sourcePath)
	}
	if err := durable.PromoteFileNoReplace(tempName, targetPath); err != nil {
		return "", err
	}
	cleanup = false
	return digest, nil
}

func stageRootForEntryPath(targetPath, entryPath string) (string, error) {
	if err := pathguard.ValidateSlashRelativePath(entryPath, 0); err != nil {
		return "", err
	}
	root := filepath.Clean(targetPath)
	for range strings.Split(entryPath, "/") {
		root = filepath.Dir(root)
	}
	return root, nil
}

func digestOpenFile(file *os.File) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func applyFileMetadata(path string, mode os.FileMode, modTime time.Time) error {
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("preserve permissions for %q: %w", path, err)
	}
	if !modTime.IsZero() {
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			return fmt.Errorf("preserve modification time for %q: %w", path, err)
		}
	}
	return nil
}

func targetFileContentState(path string, size int64, digest string) (same bool, exists bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("stat target file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return false, true, fmt.Errorf("target path %q already exists and is not a regular file; refusing to overwrite", path)
	}
	if info.Size() != size {
		return false, true, nil
	}
	got, err := digestFile(path)
	if err != nil {
		return false, true, err
	}
	return got == digest, true, nil
}

func targetMatchesPreviousFile(path string, previous previousFileEvidence) (bool, error) {
	if !previousFileEvidenceComplete(previous) {
		return false, nil
	}
	return targetMatchesFileEvidence(path, previous.Size, previous.Digest, previous.Mode, previous.ModTime, "previous target")
}

func targetMatchesManifestFile(path string, entry control.ManifestEntry) (bool, error) {
	if entry.Kind != "file" || !isSHA256Digest(entry.Digest) || !entry.HasModeEvidence() || strings.TrimSpace(entry.ModTime) == "" {
		return false, nil
	}
	if parseManifestModTime(entry.ModTime).IsZero() {
		return false, nil
	}
	return targetMatchesFileEvidence(path, entry.Size, entry.Digest, entry.Mode, entry.ModTime, "manifest target")
}

func targetMatchesFileEvidence(path string, size int64, digest string, mode uint32, modTime string, description string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s file %q: %w", description, path, err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("target path %q already exists and is not a regular file; refusing to overwrite", path)
	}
	if info.Size() != size {
		return false, nil
	}
	if uint32(info.Mode().Perm()) != mode {
		return false, nil
	}
	wantModTime := parseManifestModTime(modTime)
	if wantModTime.IsZero() || !info.ModTime().Equal(wantModTime) {
		return false, nil
	}
	got, err := digestFile(path)
	if err != nil {
		return false, err
	}
	return got == digest, nil
}

func previousFileEvidenceComplete(previous previousFileEvidence) bool {
	return strings.TrimSpace(previous.SessionID) != "" &&
		strings.TrimSpace(previous.ManifestID) != "" &&
		isSHA256Digest(previous.Digest) &&
		previous.HasSize &&
		previous.HasMode &&
		previous.Size >= 0 &&
		strings.TrimSpace(previous.ModTime) != "" &&
		!parseManifestModTime(previous.ModTime).IsZero()
}

func isSHA256Digest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	hexPart := strings.TrimPrefix(value, prefix)
	if len(hexPart) != 64 {
		return false
	}
	for _, r := range hexPart {
		if ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F') {
			continue
		}
		return false
	}
	return true
}

func previousEvidenceFromManifestEntry(entry control.ManifestEntry) previousFileEvidence {
	return previousFileEvidence{
		SessionID:  entry.PreviousSessionID,
		ManifestID: entry.PreviousManifestID,
		Size:       entry.PreviousSize,
		Digest:     entry.PreviousDigest,
		Mode:       entry.PreviousMode,
		ModTime:    entry.PreviousModTime,
		HasSize:    entry.HasPreviousSizeEvidence(),
		HasMode:    entry.HasPreviousModeEvidence(),
	}
}

func publishManagedReplacement(stagePath, finalPath, targetDir, sessionID string, entry control.ManifestEntry, previous previousFileEvidence) error {
	previousSame, err := targetMatchesPreviousFile(finalPath, previous)
	if err != nil {
		return err
	}
	if !previousSame {
		return fmt.Errorf("%w: target file %q no longer matches previous manifest evidence", errManagedReplaceTargetChanged, finalPath)
	}
	mode := os.FileMode(entry.Mode)
	if !entry.HasModeEvidence() {
		mode = 0o644
	}
	if err := applyFileMetadata(stagePath, mode, parseManifestModTime(entry.ModTime)); err != nil {
		return err
	}
	if beforeManagedReplacePromote != nil {
		if err := beforeManagedReplacePromote(entry, finalPath); err != nil {
			return err
		}
	}
	previousSame, err = targetMatchesPreviousFile(finalPath, previous)
	if err != nil {
		return err
	}
	if !previousSame {
		return fmt.Errorf("%w: target file %q changed before managed replacement", errManagedReplaceTargetChanged, finalPath)
	}
	holds, err := holdTargetForManagedReplacement(targetDir, sessionID, entry, finalPath, previous)
	if err != nil {
		return err
	}
	if afterManagedReplaceHold != nil {
		if err := afterManagedReplaceHold(entry, finalPath, holds.previousPath); err != nil {
			return err
		}
	}
	if err := durable.PromoteFileNoReplace(stagePath, finalPath); err != nil {
		return errors.Join(err, restoreReplacementHolds(holds, finalPath, previous))
	}
	return removeReplacementHolds(holds, finalPath, previous)
}

func publishHeldManagedReplacement(stagePath, finalPath, targetDir, sessionID string, entry control.ManifestEntry) error {
	previous := previousEvidenceFromManifestEntry(entry)
	holdPath, err := replacementHoldPath(targetDir, sessionID, "previous", targetPath(entry))
	if err != nil {
		return err
	}
	currentHoldPath, err := replacementHoldPath(targetDir, sessionID, "current", targetPath(entry))
	if err != nil {
		return err
	}
	holdSame, err := targetMatchesPreviousFile(holdPath, previous)
	if err != nil {
		return err
	}
	if !holdSame {
		return fmt.Errorf("replacement hold for %q no longer matches previous manifest evidence", finalPath)
	}
	currentSame, err := targetMatchesPreviousFile(currentHoldPath, previous)
	if err != nil {
		return err
	}
	if !currentSame {
		return fmt.Errorf("current replacement hold for %q no longer matches previous manifest evidence", finalPath)
	}
	mode := os.FileMode(entry.Mode)
	if !entry.HasModeEvidence() {
		mode = 0o644
	}
	if err := applyFileMetadata(stagePath, mode, parseManifestModTime(entry.ModTime)); err != nil {
		return err
	}
	if err := durable.PromoteFileNoReplace(stagePath, finalPath); err != nil {
		return err
	}
	return removeMatchingReplacementHoldsIfPresent(targetDir, sessionID, entry, previous)
}

func holdTargetForManagedReplacement(targetDir, sessionID string, entry control.ManifestEntry, finalPath string, previous previousFileEvidence) (replacementHolds, error) {
	entryTarget := targetPath(entry)
	previousHoldPath, err := replacementHoldPath(targetDir, sessionID, "previous", entryTarget)
	if err != nil {
		return replacementHolds{}, err
	}
	currentHoldPath, err := replacementHoldPath(targetDir, sessionID, "current", entryTarget)
	if err != nil {
		return replacementHolds{}, err
	}
	holds := replacementHolds{previousPath: previousHoldPath, currentPath: currentHoldPath}
	createdHold := false
	if _, err := os.Lstat(previousHoldPath); err == nil {
		holdSame, err := targetMatchesPreviousFile(previousHoldPath, previous)
		if err != nil {
			return replacementHolds{}, err
		}
		if !holdSame {
			return replacementHolds{}, fmt.Errorf("replacement hold for %q no longer matches previous manifest evidence", finalPath)
		}
	} else if !os.IsNotExist(err) {
		return replacementHolds{}, fmt.Errorf("stat replacement hold %q: %w", previousHoldPath, err)
	} else {
		if err := pathguard.EnsurePlainDirectory(control.ControlDir(targetDir), filepath.Dir(previousHoldPath), 0o700); err != nil {
			return replacementHolds{}, fmt.Errorf("create replacement hold parent %q: %w", filepath.Dir(previousHoldPath), err)
		}
		if err := createReplacementHold(finalPath, previousHoldPath); err != nil {
			return replacementHolds{}, fmt.Errorf("create replacement hold %q for %q: %w", previousHoldPath, finalPath, err)
		}
		createdHold = true
		if err := durable.SyncDirBestEffort(filepath.Dir(previousHoldPath)); err != nil {
			return replacementHolds{}, err
		}
		holdSame, err := targetMatchesPreviousFile(previousHoldPath, previous)
		if err != nil {
			return replacementHolds{}, err
		}
		if !holdSame {
			removeErr := removeReplacementHold(previousHoldPath)
			return replacementHolds{}, errors.Join(fmt.Errorf("%w: held target %q does not match previous manifest evidence", errManagedReplaceTargetChanged, previousHoldPath), removeErr)
		}
	}

	finalSame, err := targetMatchesPreviousFile(finalPath, previous)
	if err != nil {
		return replacementHolds{}, err
	}
	if !finalSame {
		return replacementHolds{}, fmt.Errorf("%w: target file %q changed before managed replacement; replacement hold retained at %q", errManagedReplaceTargetChanged, finalPath, previousHoldPath)
	}
	if _, err := os.Lstat(currentHoldPath); err == nil {
		return replacementHolds{}, fmt.Errorf("current replacement hold %q already exists; recovery is required before replacing %q", currentHoldPath, finalPath)
	} else if !os.IsNotExist(err) {
		return replacementHolds{}, fmt.Errorf("stat current replacement hold %q: %w", currentHoldPath, err)
	}
	if err := pathguard.EnsurePlainDirectory(control.ControlDir(targetDir), filepath.Dir(currentHoldPath), 0o700); err != nil {
		return replacementHolds{}, fmt.Errorf("create current replacement hold parent %q: %w", filepath.Dir(currentHoldPath), err)
	}
	if beforeManagedReplaceCurrentHold != nil {
		if err := beforeManagedReplaceCurrentHold(entry, finalPath, currentHoldPath); err != nil {
			return replacementHolds{}, err
		}
	}
	if err := durable.MoveFileNoReplace(finalPath, currentHoldPath); err != nil {
		var cleanupErr error
		if createdHold {
			cleanupErr = removeReplacementHold(previousHoldPath)
		}
		return replacementHolds{}, errors.Join(fmt.Errorf("move current target %q to replacement hold: %w", finalPath, err), cleanupErr)
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(finalPath)); err != nil {
		return replacementHolds{}, err
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(currentHoldPath)); err != nil {
		return replacementHolds{}, err
	}
	holdSame, err := targetMatchesPreviousFile(previousHoldPath, previous)
	if err != nil {
		return replacementHolds{}, err
	}
	currentSame, currentErr := targetMatchesPreviousFile(currentHoldPath, previous)
	if currentErr != nil {
		return replacementHolds{}, currentErr
	}
	if holdSame && currentSame {
		return holds, nil
	}
	restoreErr := restoreCurrentReplacementHold(holds, finalPath)
	return replacementHolds{}, errors.Join(fmt.Errorf("%w: replacement hold for %q no longer matches previous manifest evidence", errManagedReplaceTargetChanged, finalPath), restoreErr)
}

func replacementHoldMatchesPrevious(targetDir, sessionID, entryTarget string, previous previousFileEvidence) (bool, error) {
	holdPath, err := replacementHoldPath(targetDir, sessionID, "previous", entryTarget)
	if err != nil {
		return false, err
	}
	previousSame, err := targetMatchesPreviousFile(holdPath, previous)
	if err != nil || !previousSame {
		return previousSame, err
	}
	currentHoldPath, err := replacementHoldPath(targetDir, sessionID, "current", entryTarget)
	if err != nil {
		return false, err
	}
	return targetMatchesPreviousFile(currentHoldPath, previous)
}

func createReplacementHold(finalPath, holdPath string) error {
	info, err := os.Lstat(finalPath)
	if err != nil {
		return fmt.Errorf("stat previous target %q: %w", finalPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("previous target %q is not a regular file", finalPath)
	}
	in, err := os.Open(finalPath)
	if err != nil {
		return fmt.Errorf("open previous target %q: %w", finalPath, err)
	}
	defer in.Close()
	openedInfo, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat opened previous target %q: %w", finalPath, err)
	}
	if !sameSourceFile(info, openedInfo) {
		return fmt.Errorf("previous target %q changed before replacement hold copy", finalPath)
	}

	temp, err := os.CreateTemp(filepath.Dir(holdPath), ".replacement-hold-*.tmp")
	if err != nil {
		return fmt.Errorf("create replacement hold temp: %w", err)
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := io.Copy(temp, in); err != nil {
		_ = temp.Close()
		return fmt.Errorf("copy previous target %q to replacement hold temp: %w", finalPath, err)
	}
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		_ = temp.Close()
		return fmt.Errorf("chmod replacement hold temp: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close replacement hold temp: %w", err)
	}
	after, err := os.Lstat(finalPath)
	if err != nil {
		return fmt.Errorf("stat previous target %q after hold copy: %w", finalPath, err)
	}
	if !sameSourceFile(info, after) {
		return fmt.Errorf("previous target %q changed during replacement hold copy", finalPath)
	}
	if err := os.Chtimes(tempName, info.ModTime(), info.ModTime()); err != nil {
		return fmt.Errorf("preserve replacement hold temp modtime: %w", err)
	}
	if err := durable.PromoteFileNoReplace(tempName, holdPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func replacementHoldPath(targetDir, sessionID, holdKind, entryTarget string) (string, error) {
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("replacement hold session id is required")
	}
	switch holdKind {
	case "previous", "current":
	default:
		return "", fmt.Errorf("replacement hold kind %q is invalid", holdKind)
	}
	holdRoot := filepath.Join(control.ControlDir(targetDir), "replacement-holds", sessionID, holdKind)
	holdPath, err := pathguard.SafeJoin(holdRoot, entryTarget)
	if err != nil {
		return "", err
	}
	if err := validateReplacementHoldParent(targetDir, filepath.Dir(holdPath)); err != nil {
		return "", err
	}
	return holdPath, nil
}

func validateReplacementHoldParent(targetDir, dir string) error {
	controlDir := control.ControlDir(targetDir)
	if err := pathguard.EnsureDirectory(filepath.Dir(controlDir), controlDir); err != nil {
		return fmt.Errorf("validate control directory %q: %w", controlDir, err)
	}
	if err := pathguard.EnsureDirectory(controlDir, dir); err != nil {
		return fmt.Errorf("validate replacement hold parent %q: %w", dir, err)
	}
	return nil
}

func restoreReplacementHold(holdPath, finalPath string, previous previousFileEvidence) error {
	if _, err := os.Lstat(holdPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat replacement hold %q: %w", holdPath, err)
	}
	holdSame, err := targetMatchesPreviousFile(holdPath, previous)
	if err != nil {
		return err
	}
	if !holdSame {
		return fmt.Errorf("%w: replacement hold for %q changed before restore", errManagedReplaceTargetChanged, finalPath)
	}
	if _, err := os.Lstat(finalPath); err == nil {
		return fmt.Errorf("target path %q exists while restoring replacement hold %q", finalPath, holdPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat target path %q before replacement hold restore: %w", finalPath, err)
	}
	if err := durable.PromoteFileNoReplace(holdPath, finalPath); err != nil {
		return fmt.Errorf("restore replacement hold %q: %w", holdPath, err)
	}
	return nil
}

func restoreCurrentReplacementHold(holds replacementHolds, finalPath string) error {
	if _, err := os.Lstat(holds.currentPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat current replacement hold %q: %w", holds.currentPath, err)
	}
	if _, err := os.Lstat(finalPath); err == nil {
		return fmt.Errorf("target path %q exists while restoring current replacement hold %q", finalPath, holds.currentPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat target path %q before current hold restore: %w", finalPath, err)
	}
	if err := durable.PromoteFileNoReplace(holds.currentPath, finalPath); err != nil {
		return fmt.Errorf("restore current replacement hold %q: %w", holds.currentPath, err)
	}
	return nil
}

func restoreReplacementHolds(holds replacementHolds, finalPath string, previous previousFileEvidence) error {
	if err := restoreReplacementHold(holds.previousPath, finalPath, previous); err != nil {
		return err
	}
	if _, err := os.Lstat(holds.currentPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat current replacement hold %q: %w", holds.currentPath, err)
	}
	currentSame, err := targetMatchesPreviousFile(holds.currentPath, previous)
	if err != nil {
		return err
	}
	if !currentSame {
		return fmt.Errorf("%w: current replacement hold for %q changed before cleanup", errManagedReplaceTargetChanged, finalPath)
	}
	return removeReplacementHold(holds.currentPath)
}

func removeReplacementHold(holdPath string) error {
	if err := os.Remove(holdPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove replacement hold %q: %w", holdPath, err)
	}
	return durable.SyncDirBestEffort(filepath.Dir(holdPath))
}

func removeReplacementHolds(holds replacementHolds, finalPath string, previous previousFileEvidence) error {
	if err := validateReplacementHoldMatches(holds.previousPath, finalPath, previous); err != nil {
		return err
	}
	if err := validateReplacementHoldMatches(holds.currentPath, finalPath, previous); err != nil {
		return err
	}
	if err := removeReplacementHold(holds.currentPath); err != nil {
		return err
	}
	if err := removeReplacementHold(holds.previousPath); err != nil {
		return err
	}
	return nil
}

func removeMatchingReplacementHoldsIfPresent(targetDir, sessionID string, entry control.ManifestEntry, previous previousFileEvidence) error {
	type presentHold struct {
		kind string
		path string
	}
	holds := make([]presentHold, 0, 2)
	for _, holdKind := range []string{"previous", "current"} {
		holdPath, err := replacementHoldPath(targetDir, sessionID, holdKind, targetPath(entry))
		if err != nil {
			return err
		}
		if _, err := os.Lstat(holdPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat replacement hold %q: %w", holdPath, err)
		}
		if err := validateReplacementHoldMatches(holdPath, entry.Path, previous); err != nil {
			return err
		}
		holds = append(holds, presentHold{kind: holdKind, path: holdPath})
	}
	for _, hold := range holds {
		if err := removeReplacementHold(hold.path); err != nil {
			return err
		}
	}
	return nil
}

func validateReplacementHoldMatches(holdPath, finalPath string, previous previousFileEvidence) error {
	holdSame, err := targetMatchesPreviousFile(holdPath, previous)
	if err != nil {
		return err
	}
	if !holdSame {
		return fmt.Errorf("%w: replacement hold for %q changed during managed replacement", errManagedReplaceTargetChanged, finalPath)
	}
	return nil
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open target file %q: %w", path, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash target file %q: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func sameSourceFile(before, after os.FileInfo) bool {
	return before.Mode().IsRegular() &&
		after.Mode().IsRegular() &&
		os.SameFile(before, after) &&
		before.Size() == after.Size() &&
		before.Mode().Perm() == after.Mode().Perm() &&
		before.ModTime().Equal(after.ModTime())
}

func readStableSymlink(sourcePath string, entry scan.Entry) (string, error) {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("stat source symlink %q before publish: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("source path %q changed from symlink to %s before publish", sourcePath, info.Mode().Type())
	}
	if !info.ModTime().Equal(entry.ModTime) {
		return "", fmt.Errorf("source symlink %q changed modtime before publish", sourcePath)
	}
	target, err := os.Readlink(sourcePath)
	if err != nil {
		return "", fmt.Errorf("read source symlink %q before publish: %w", sourcePath, err)
	}
	if target != entry.SymlinkTarget {
		return "", fmt.Errorf("source symlink %q changed target from %q to %q before publish", sourcePath, entry.SymlinkTarget, target)
	}
	return target, nil
}

func manifestEntry(entry scan.Entry, kind string, digest string) control.ManifestEntry {
	out := control.ManifestEntry{
		Path:          entry.Path,
		Kind:          kind,
		ModTime:       entry.ModTime.UTC().Format(time.RFC3339Nano),
		Digest:        digest,
		TargetPath:    entry.Path,
		SymlinkTarget: entry.SymlinkTarget,
	}
	out.SetModeEvidence(uint32(entry.Mode.Perm()))
	out.SetSizeEvidence(entry.Size)
	return out
}

func manifestEntryWithPrevious(entry scan.Entry, kind string, digest string, previous previousFileEvidence) control.ManifestEntry {
	out := manifestEntry(entry, kind, digest)
	if previousFileEvidenceComplete(previous) {
		out.PreviousSessionID = previous.SessionID
		out.PreviousManifestID = previous.ManifestID
		out.SetPreviousSizeEvidence(previous.Size)
		out.PreviousDigest = previous.Digest
		out.SetPreviousModeEvidence(previous.Mode)
		out.PreviousModTime = previous.ModTime
	}
	return out
}

func writeWarningArtifacts(targetDir string, sessionID string, now time.Time, warnings []audit.Record) error {
	stamp := now.UTC().Format(time.RFC3339Nano)
	for i, warning := range warnings {
		doc := control.Warning{
			Version:               control.CurrentVersion,
			ID:                    fmt.Sprintf("%s-%03d-%s", sessionID, i+1, warning.ID),
			SessionID:             sessionID,
			Code:                  warning.Kind,
			Message:               warning.Reason,
			Severity:              string(warning.Severity),
			Paths:                 []string{warning.Path},
			TargetPath:            warning.TargetPath,
			Detected:              warning.Detected,
			SuggestedProfilePatch: warning.SuggestedProfilePatch,
			SuggestedConfig:       warning.SuggestedConfig,
			CreatedAt:             stamp,
		}
		if path, err := control.Path(targetDir, control.ArtifactWarning, doc.ID); err != nil {
			return err
		} else if err := control.WriteFile(path, doc); err != nil {
			return err
		}
	}
	return nil
}

func writeControlArtifacts(targetDir string, p profile.Profile, sessionID string, now time.Time, entries []control.ManifestEntry, warnings []audit.Record, influences []agentkb.Influence, softDeletes []control.SoftDelete) error {
	profilePayload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal profile snapshot: %w", err)
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  p.ProfileID,
		SessionID:  sessionID,
		CapturedAt: stamp,
		Profile:    profilePayload,
	}
	if path, err := control.Path(targetDir, control.ArtifactProfileSnapshot, snapshot.ID); err != nil {
		return err
	} else if err := control.WriteFile(path, snapshot); err != nil {
		return err
	}
	manifest := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    p.Roots[0].ID,
		CreatedAt: stamp,
		Entries:   entries,
	}
	if path, err := control.Path(targetDir, control.ArtifactManifest, sessionID); err != nil {
		return err
	} else if err := control.WriteFile(path, manifest); err != nil {
		return err
	}
	if err := writeWarningArtifacts(targetDir, sessionID, now, warnings); err != nil {
		return err
	}
	for _, softDelete := range softDeletes {
		if path, err := control.Path(targetDir, control.ArtifactSoftDelete, softDelete.ID); err != nil {
			return err
		} else if err := control.WriteFile(path, softDelete); err != nil {
			return err
		}
	}
	if len(influences) > 0 {
		if err := writeAgentInfluence(targetDir, sessionID, stamp, influences); err != nil {
			return err
		}
	}
	return nil
}

func writeSessionReceipt(targetDir string, p profile.Profile, sessionID string, now time.Time) error {
	return writeSessionReceiptWithTimes(targetDir, p, sessionID, now, now)
}

func writeSessionReceiptWithTimes(targetDir string, p profile.Profile, sessionID string, startedAt time.Time, endedAt time.Time) error {
	started := startedAt.UTC().Format(time.RFC3339Nano)
	ended := endedAt.UTC().Format(time.RFC3339Nano)
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: p.ProfileID,
		TargetID:  p.Target.TargetID,
		StartedAt: started,
		EndedAt:   ended,
		Status:    "published",
	}
	if path, err := control.Path(targetDir, control.ArtifactSessionReceipt, sessionID); err != nil {
		return err
	} else if err := control.WriteFile(path, receipt); err != nil {
		return err
	}
	return nil
}

func writeAgentInfluence(targetDir, sessionID, stamp string, influences []agentkb.Influence) error {
	type document struct {
		Version   int                 `json:"version"`
		SessionID string              `json:"session_id"`
		CreatedAt string              `json:"created_at"`
		Influence []agentkb.Influence `json:"influence"`
	}
	path := filepath.Join(control.ControlDir(targetDir), "agent", sessionID+"-influence.json")
	if err := control.EnsureControlDir(targetDir); err != nil {
		return err
	}
	if err := pathguard.EnsurePlainDirectory(control.ControlDir(targetDir), filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(document{Version: 1, SessionID: sessionID, CreatedAt: stamp, Influence: influences}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeBytesAtomic(path, data)
}

func writeBytesAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".control-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(path)); err != nil {
		return err
	}
	cleanup = false
	return nil
}
