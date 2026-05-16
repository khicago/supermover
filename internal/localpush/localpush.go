package localpush

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	unlock, err := lockTargetSession(opts.TargetDir, sessionID)
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	if err := ensureSessionUnused(opts.TargetDir, sessionID); err != nil {
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
	influences := agentkb.Detect(scanResult.Entries, agentKnowledgeCategories(opts.Profile.AgentKnowledge))
	softDeletes, err := softDeletesForRun(opts.Profile, opts.TargetDir, scanResult, sessionID, now)
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
			manifestEntries = append(manifestEntries, manifestEntry(entry, "file", digest))
		case scan.KindSymlink:
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, entry.Path, audit.SeverityWarning, "symlink_not_copied", "symlink copy is not implemented in local push"),
				map[string]string{"target": entry.SymlinkTarget},
			))
			manifestEntries = append(manifestEntries, manifestEntry(entry, "symlink", ""))
		default:
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, entry.Path, audit.SeverityWarning, "special_not_copied", "special file copy is not supported"),
				map[string]string{"mode": entry.Mode.String()},
			))
		}
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
	if err := publishStaged(layout, opts.TargetDir, sessionID, manifestEntries, existingDirs); err != nil {
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
	softDeletes, err := softDeletesForRun(opts.Profile, opts.TargetDir, scanResult, sessionID, now)
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
			if err := preflightRegularTarget(sourcePath, targetPath); err != nil {
				return Result{}, err
			}
			entries++
			copied++
		case scan.KindSymlink:
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, entry.Path, audit.SeverityWarning, "symlink_not_copied", "symlink copy is not implemented in local push"),
				map[string]string{"target": entry.SymlinkTarget},
			))
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
	layout := transaction.NewLayout(control.ControlDir(opts.TargetDir))
	scan, err := transaction.ScanRecovery(layout)
	if err != nil {
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
			if opts.DryRun {
				item.Status = "would_recover"
				item.Message = scanned.Reason
				result.Skipped++
				result.Items = append(result.Items, item)
				continue
			}
			if err := recoverStagedSession(layout, opts.Profile, opts.TargetDir, scanned.Record, now); err != nil {
				marked := markSessionNeedsRepair(layout, scanned.Record, now, err)
				item.Status = "needs_repair"
				item.Message = err.Error()
				result.RepairNeeded++
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
	if manifest.RootID != "" && manifest.RootID != p.Roots[0].ID {
		return fmt.Errorf("recover manifest root_id %q does not match profile root %q", manifest.RootID, p.Roots[0].ID)
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
	if err := publishStaged(layout, targetDir, record.ID, manifest.Entries, existingDirs); err != nil {
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

func validateRecoverableStagedFiles(layout transaction.Layout, targetDir string, sessionID string, entries []control.ManifestEntry) error {
	for _, entry := range entries {
		if entry.Kind != "file" {
			continue
		}
		if strings.TrimSpace(entry.Digest) == "" {
			return fmt.Errorf("recover file %q is missing digest", entry.Path)
		}
		finalPath, err := targetPathForManifestEntry(targetDir, entry)
		if err != nil {
			return err
		}
		same, exists, err := targetFileState(finalPath, entry.Size, entry.Digest)
		if err != nil {
			return err
		}
		if exists {
			if same {
				continue
			}
			return fmt.Errorf("target file %q already exists with different content; refusing to overwrite", finalPath)
		}
		stagePath, err := pathguard.SafeJoinParent(layout.StagingDir(sessionID), entry.Path)
		if err != nil {
			return err
		}
		info, err := os.Stat(stagePath)
		if err != nil {
			return fmt.Errorf("stat staged file %q for recovery: %w", entry.Path, err)
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
	}
	return nil
}

func recoverWarningsForUnsupportedEntries(entries []control.ManifestEntry) []audit.Record {
	var warnings []audit.Record
	for _, entry := range entries {
		switch entry.Kind {
		case "dir", "file":
			continue
		case "symlink":
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, targetPath(entry), audit.SeverityWarning, "symlink_not_copied", "symlink copy is not implemented in local recover"),
				map[string]string{"target": entry.SymlinkTarget},
			))
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
	key := target + "\x00session\x00" + sessionID
	value, _ := localPushLocks.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock, nil
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

func softDeletesForRun(p profile.Profile, targetDir string, scanResult scan.Result, sessionID string, now time.Time) ([]control.SoftDelete, error) {
	if p.DeletePolicy.Mode == profile.DeleteModeIgnore || len(scanResult.Entries) == 0 {
		return nil, nil
	}
	previous, ok, err := latestPublishedManifest(p, targetDir)
	if err != nil {
		return nil, err
	}
	if !ok {
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

func preflightRegularTarget(sourcePath, targetPath string) error {
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
	same, exists, err := targetFileState(targetPath, sourceInfo.Size(), sourceDigest)
	if err != nil {
		return err
	}
	if exists && !same {
		return fmt.Errorf("target file %q already exists with different content; refusing to overwrite", targetPath)
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
	same, exists, err := targetFileState(targetPath, info.Size(), digest)
	if err != nil {
		return "", err
	}
	if exists {
		if same {
			return digest, nil
		}
		return "", fmt.Errorf("target file %q already exists with different content; refusing to overwrite", targetPath)
	}
	if err := durable.PromoteFileNoReplace(stagePath, targetPath); err != nil {
		return "", err
	}
	if err := applyFileMetadata(targetPath, mode, modTime); err != nil {
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
		case "file":
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

func publishStaged(layout transaction.Layout, targetDir string, sessionID string, entries []control.ManifestEntry, existingDirs map[string]existingDirMeta) error {
	defer restoreExistingDirs(existingDirs)
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
			if mode == 0 {
				mode = 0o755
			}
			existed, err := directoryExists(targetPath)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(targetPath, mode); err != nil {
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
			same, exists, err := targetFileState(targetPath, entry.Size, entry.Digest)
			if err != nil {
				return err
			}
			if exists {
				if same {
					mode := os.FileMode(entry.Mode)
					if mode == 0 {
						mode = 0o644
					}
					if err := applyFileMetadata(targetPath, mode, parseManifestModTime(entry.ModTime)); err != nil {
						return err
					}
					if err := removeStagedIfPresent(stagePath, entry.Path); err != nil {
						return err
					}
					continue
				}
				return fmt.Errorf("target file %q already exists with different content; refusing to overwrite", targetPath)
			}
			if err := durable.PromoteFileNoReplace(stagePath, targetPath); err != nil {
				return err
			}
			mode := os.FileMode(entry.Mode)
			if mode == 0 {
				mode = 0o644
			}
			if err := applyFileMetadata(targetPath, mode, parseManifestModTime(entry.ModTime)); err != nil {
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

func restoreExistingDirs(dirs map[string]existingDirMeta) {
	for path, meta := range dirs {
		_ = os.Chmod(path, meta.Mode)
		_ = os.Chtimes(path, meta.ModTime, meta.ModTime)
	}
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
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
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
	if err := durable.PromoteFileNoReplace(tempName, targetPath); err != nil {
		return "", err
	}
	cleanup = false
	return digest, nil
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

func targetFileState(path string, size int64, digest string) (same bool, exists bool, err error) {
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

func manifestEntry(entry scan.Entry, kind string, digest string) control.ManifestEntry {
	return control.ManifestEntry{
		Path:          entry.Path,
		Kind:          kind,
		Mode:          uint32(entry.Mode.Perm()),
		Size:          entry.Size,
		ModTime:       entry.ModTime.UTC().Format(time.RFC3339Nano),
		Digest:        digest,
		TargetPath:    entry.Path,
		SymlinkTarget: entry.SymlinkTarget,
	}
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
	if err := pathguard.EnsureDirectory(control.ControlDir(targetDir), filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
