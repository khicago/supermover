package driftreview

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/targetlock"
	"github.com/khicago/supermover/internal/verify"
)

type AcknowledgeOptions struct {
	Profile  profile.Profile
	ID       string
	Reason   string
	Reviewer string
	Now      time.Time
}

type AcknowledgeResult struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	PreviousState string `json:"previous_state"`
	ReviewState   string `json:"review_state"`
	ReviewedAt    string `json:"reviewed_at"`
	Reviewer      string `json:"reviewer,omitempty"`
	Reason        string `json:"reason"`
	ProfileID     string `json:"profile_id"`
	TargetID      string `json:"target_id"`
	SessionID     string `json:"session_id"`
}

type ResolveOptions struct {
	Profile  profile.Profile
	ID       string
	Reason   string
	Reviewer string
	Now      time.Time
}

type ResolveResult struct {
	ID              string `json:"id"`
	Path            string `json:"path"`
	PreviousState   string `json:"previous_state"`
	ReviewState     string `json:"review_state"`
	ReviewedAt      string `json:"reviewed_at"`
	Reviewer        string `json:"reviewer,omitempty"`
	Reason          string `json:"reason"`
	ProfileID       string `json:"profile_id"`
	TargetID        string `json:"target_id"`
	SessionID       string `json:"session_id"`
	Repair          string `json:"repair"`
	ManifestRewrite string `json:"manifest_rewrite"`
	Prune           string `json:"prune"`
}

func Acknowledge(opts AcknowledgeOptions) (AcknowledgeResult, error) {
	if strings.TrimSpace(opts.ID) == "" {
		return AcknowledgeResult{}, errors.New("drift id is required")
	}
	if err := control.ValidateArtifactID(opts.ID); err != nil {
		return AcknowledgeResult{}, fmt.Errorf("drift id is unsafe: %w", err)
	}
	reason := strings.TrimSpace(opts.Reason)
	if reason == "" {
		return AcknowledgeResult{}, errors.New("reason is required")
	}
	targetRoot, err := targetRootFromProfile(opts.Profile)
	if err != nil {
		return AcknowledgeResult{}, err
	}
	if err := control.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return AcknowledgeResult{}, err
	}
	if _, _, err := loadReviewableDrift(targetRoot, opts.Profile, opts.ID); err != nil {
		return AcknowledgeResult{}, err
	}

	unlock, err := targetlock.LockTarget(targetRoot)
	if err != nil {
		return AcknowledgeResult{}, err
	}
	defer unlock()

	path, doc, err := loadReviewableDrift(targetRoot, opts.Profile, opts.ID)
	if err != nil {
		return AcknowledgeResult{}, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	reviewedAt := now.UTC().Format(time.RFC3339Nano)
	previousState := doc.ReviewState
	if strings.TrimSpace(previousState) == "" {
		previousState = "needs_review"
	}
	switch previousState {
	case "needs_review":
	case "acknowledged":
		return AcknowledgeResult{}, fmt.Errorf("persisted target drift %q is already acknowledged; re-review overwrite is not supported", opts.ID)
	case "resolved":
		return AcknowledgeResult{}, fmt.Errorf("persisted target drift %q is already resolved; acknowledge would overwrite review evidence", opts.ID)
	default:
		return AcknowledgeResult{}, fmt.Errorf("persisted target drift %q has unsupported review_state %q", opts.ID, previousState)
	}

	doc.ReviewState = "acknowledged"
	doc.ReviewedAt = reviewedAt
	doc.ReviewedBy = strings.TrimSpace(opts.Reviewer)
	doc.ReviewReason = reason
	doc.ReviewAction = "acknowledge"
	if err := control.WriteFile(path, doc); err != nil {
		return AcknowledgeResult{}, err
	}

	return AcknowledgeResult{
		ID:            doc.ID,
		Path:          doc.Path,
		PreviousState: previousState,
		ReviewState:   doc.ReviewState,
		ReviewedAt:    doc.ReviewedAt,
		Reviewer:      doc.ReviewedBy,
		Reason:        doc.ReviewReason,
		ProfileID:     doc.ProfileID,
		TargetID:      doc.TargetID,
		SessionID:     doc.SessionID,
	}, nil
}

func Resolve(opts ResolveOptions) (ResolveResult, error) {
	if strings.TrimSpace(opts.ID) == "" {
		return ResolveResult{}, errors.New("drift id is required")
	}
	if err := control.ValidateArtifactID(opts.ID); err != nil {
		return ResolveResult{}, fmt.Errorf("drift id is unsafe: %w", err)
	}
	reason := strings.TrimSpace(opts.Reason)
	if reason == "" {
		return ResolveResult{}, errors.New("reason is required")
	}
	targetRoot, err := targetRootFromProfile(opts.Profile)
	if err != nil {
		return ResolveResult{}, err
	}
	if err := control.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return ResolveResult{}, err
	}
	_, doc, err := loadReviewableDrift(targetRoot, opts.Profile, opts.ID)
	if err != nil {
		return ResolveResult{}, err
	}
	if err := requireDriftNoLongerDetected(targetRoot, opts.Profile, doc); err != nil {
		return ResolveResult{}, err
	}

	unlock, err := targetlock.LockTarget(targetRoot)
	if err != nil {
		return ResolveResult{}, err
	}
	defer unlock()

	path, doc, err := loadReviewableDrift(targetRoot, opts.Profile, opts.ID)
	if err != nil {
		return ResolveResult{}, err
	}
	if err := requireDriftNoLongerDetected(targetRoot, opts.Profile, doc); err != nil {
		return ResolveResult{}, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	reviewedAt := now.UTC().Format(time.RFC3339Nano)
	previousState := doc.ReviewState
	if strings.TrimSpace(previousState) == "" {
		previousState = "needs_review"
	}
	switch previousState {
	case "needs_review", "acknowledged":
	case "resolved":
		return ResolveResult{}, fmt.Errorf("persisted target drift %q is already resolved; re-resolve overwrite is not supported", opts.ID)
	default:
		return ResolveResult{}, fmt.Errorf("persisted target drift %q has unsupported review_state %q", opts.ID, previousState)
	}

	doc.ReviewState = "resolved"
	doc.ReviewedAt = reviewedAt
	doc.ReviewedBy = strings.TrimSpace(opts.Reviewer)
	doc.ReviewReason = reason
	doc.ReviewAction = "resolve"
	if err := control.WriteFile(path, doc); err != nil {
		return ResolveResult{}, err
	}

	return ResolveResult{
		ID:              doc.ID,
		Path:            doc.Path,
		PreviousState:   previousState,
		ReviewState:     doc.ReviewState,
		ReviewedAt:      doc.ReviewedAt,
		Reviewer:        doc.ReviewedBy,
		Reason:          doc.ReviewReason,
		ProfileID:       doc.ProfileID,
		TargetID:        doc.TargetID,
		SessionID:       doc.SessionID,
		Repair:          "not_applied",
		ManifestRewrite: "not_applied",
		Prune:           "not_authorized",
	}, nil
}

func loadReviewableDrift(targetRoot string, p profile.Profile, id string) (string, control.TargetDrift, error) {
	if err := control.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return "", control.TargetDrift{}, err
	}
	path, err := control.Path(targetRoot, control.ArtifactTargetDrift, id)
	if err != nil {
		return "", control.TargetDrift{}, err
	}
	if err := requireExistingReviewArtifact(targetRoot, path, id); err != nil {
		return "", control.TargetDrift{}, err
	}
	doc, err := control.ReadFileNoSymlinkUnderRoot[control.TargetDrift](control.ControlDir(targetRoot), path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", control.TargetDrift{}, fmt.Errorf("persisted target drift %q not found", id)
		}
		return "", control.TargetDrift{}, err
	}
	if doc.ID != id {
		return "", control.TargetDrift{}, fmt.Errorf("persisted target drift id %q does not match requested id %q", doc.ID, id)
	}
	if doc.ProfileID != p.ProfileID || doc.TargetID != p.Target.TargetID {
		return "", control.TargetDrift{}, fmt.Errorf("persisted target drift %q scope (%q/%q) does not match profile scope (%q/%q)", id, doc.ProfileID, doc.TargetID, p.ProfileID, p.Target.TargetID)
	}
	if !profileHasRootID(p, doc.RootID) {
		return "", control.TargetDrift{}, fmt.Errorf("persisted target drift %q root_id %q does not match profile roots", id, doc.RootID)
	}
	if err := requirePublishedEvidence(targetRoot, p, doc); err != nil {
		return "", control.TargetDrift{}, err
	}
	return path, doc, nil
}

func requireExistingReviewArtifact(targetRoot string, path string, id string) error {
	if _, err := os.Lstat(targetRoot); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("persisted target drift %q not found: target root does not exist", id)
		}
		return fmt.Errorf("inspect target root: %w", err)
	}
	controlDir := control.ControlDir(targetRoot)
	if _, err := os.Lstat(controlDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("persisted target drift %q not found", id)
		}
		return fmt.Errorf("inspect control directory: %w", err)
	}
	driftDir := filepath.Dir(path)
	if _, err := os.Lstat(driftDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("persisted target drift %q not found", id)
		}
		return fmt.Errorf("inspect target drift directory: %w", err)
	}
	if _, err := os.Lstat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("persisted target drift %q not found", id)
		}
		return fmt.Errorf("inspect target drift artifact: %w", err)
	}
	return nil
}

func profileHasRootID(p profile.Profile, rootID string) bool {
	for _, root := range p.Roots {
		if root.ID == rootID {
			return true
		}
	}
	return false
}

func requirePublishedEvidence(targetRoot string, p profile.Profile, doc control.TargetDrift) error {
	artifacts, err := verify.LoadArtifactsForScope(targetRoot, p.ProfileID, p.Target.TargetID)
	if err != nil {
		return fmt.Errorf("validate persisted target drift evidence: %w", err)
	}
	expectedSessionID := strings.TrimSpace(doc.Expected.SessionID)
	if expectedSessionID == "" {
		return fmt.Errorf("persisted target drift %q is not valid for the selected profile/session scope: expected.session_id is required", doc.ID)
	}
	expectedManifestID := strings.TrimSpace(doc.Expected.ManifestID)
	if expectedManifestID == "" {
		return fmt.Errorf("persisted target drift %q is not valid for the selected profile/session scope: expected.manifest_id is required", doc.ID)
	}
	if _, ok := artifacts.PublishedReceipts[expectedSessionID]; !ok {
		return fmt.Errorf("persisted target drift %q is not valid for the selected profile/session scope: published expected session receipt is required", doc.ID)
	}
	manifestRootID := ""
	manifestID := ""
	var expectedManifest control.Manifest
	for _, manifest := range artifacts.Manifests {
		if manifest.SessionID == expectedSessionID {
			manifestRootID = manifest.RootID
			manifestID = manifest.ID
			expectedManifest = manifest
			break
		}
	}
	if manifestRootID == "" {
		return fmt.Errorf("persisted target drift %q is not valid for the selected profile/session scope: published expected session manifest is required", doc.ID)
	}
	if manifestID != expectedManifestID {
		return fmt.Errorf("persisted target drift %q expected manifest_id %q does not match published expected session manifest_id %q", doc.ID, expectedManifestID, manifestID)
	}
	if doc.RootID != manifestRootID {
		return fmt.Errorf("persisted target drift %q root_id %q does not match expected session manifest root_id %q", doc.ID, doc.RootID, manifestRootID)
	}
	if err := requireExpectedManifestEntry(doc, expectedManifest); err != nil {
		return err
	}
	for _, drift := range artifacts.TargetDrifts {
		if drift.ID == doc.ID {
			return nil
		}
	}
	return fmt.Errorf("persisted target drift %q is not valid for the selected profile/session scope", doc.ID)
}

func requireDriftNoLongerDetected(targetRoot string, p profile.Profile, doc control.TargetDrift) error {
	if err := requireExtraNoLongerPresent(targetRoot, doc); err != nil {
		return err
	}
	sessionID := strings.TrimSpace(doc.Expected.SessionID)
	if sessionID == "" {
		sessionID = doc.SessionID
	}
	report, err := verify.DetectTargetDrift(verify.DriftOptions{
		TargetRoot: targetRoot,
		SessionID:  sessionID,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
	})
	if err != nil {
		return fmt.Errorf("check persisted target drift %q against live detector: %w", doc.ID, err)
	}
	for _, live := range report.Drifts {
		if sameExpectedPathFinding(doc, live) {
			return fmt.Errorf("persisted target drift %q still reports drift for path %q against expected session %q", doc.ID, doc.Path, sessionID)
		}
	}
	return nil
}

func requireExtraNoLongerPresent(targetRoot string, doc control.TargetDrift) error {
	if doc.Change != "extra" || doc.Expected.Kind != "missing" {
		return nil
	}
	if err := pathguard.ValidateSlashRelativePath(doc.Path, 0); err != nil {
		return fmt.Errorf("persisted target drift %q path is unsafe: %w", doc.ID, err)
	}
	if pathguard.IsReservedControlPath(doc.Path) {
		return fmt.Errorf("persisted target drift %q path %q uses reserved control-plane space", doc.ID, doc.Path)
	}
	fullPath := filepath.Join(targetRoot, filepath.FromSlash(doc.Path))
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
		return fmt.Errorf("persisted target drift %q extra path %q parent is unsafe: %w", doc.ID, doc.Path, err)
	}
	info, err := os.Lstat(fullPath)
	if err == nil {
		if info.IsDir() {
			entries, readErr := os.ReadDir(fullPath)
			if readErr != nil {
				return fmt.Errorf("persisted target drift %q still reports drift for extra path %q: inspect directory: %w", doc.ID, doc.Path, readErr)
			}
			if len(entries) == 0 {
				return fmt.Errorf("persisted target drift %q still reports drift for extra empty directory %q", doc.ID, doc.Path)
			}
		}
		return fmt.Errorf("persisted target drift %q still reports drift for extra path %q", doc.ID, doc.Path)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("persisted target drift %q still reports drift for extra path %q: inspect target path: %w", doc.ID, doc.Path, err)
}

func sameExpectedPathFinding(persisted control.TargetDrift, live control.TargetDrift) bool {
	return persisted.ProfileID == live.ProfileID &&
		persisted.TargetID == live.TargetID &&
		persisted.RootID == live.RootID &&
		persisted.Path == live.Path &&
		reflect.DeepEqual(persisted.Expected, live.Expected)
}

func requireExpectedManifestEntry(doc control.TargetDrift, manifest control.Manifest) error {
	if doc.Path != doc.Expected.Path {
		return fmt.Errorf("persisted target drift %q path %q does not match expected path %q", doc.ID, doc.Path, doc.Expected.Path)
	}
	if doc.Expected.Kind == "missing" {
		if doc.Change != "extra" {
			return fmt.Errorf("persisted target drift %q expected missing baseline is only valid for extra target paths, got change %q", doc.ID, doc.Change)
		}
		if err := requireExtraObservedEvidence(doc); err != nil {
			return err
		}
		for _, entry := range manifest.Entries {
			if manifestEntryTargetPath(entry) == doc.Expected.Path {
				return fmt.Errorf("persisted target drift %q expected missing path %q is present in published manifest %q", doc.ID, doc.Expected.Path, manifest.ID)
			}
		}
		return nil
	}
	for _, entry := range manifest.Entries {
		if manifestEntryTargetPath(entry) != doc.Expected.Path {
			continue
		}
		return validateExpectedManifestEntry(doc, entry)
	}
	return fmt.Errorf("persisted target drift %q expected path %q is not present in published manifest %q", doc.ID, doc.Expected.Path, manifest.ID)
}

func requireExtraObservedEvidence(doc control.TargetDrift) error {
	if doc.Observed.Path != doc.Path {
		return fmt.Errorf("persisted target drift %q observed path %q does not match drift path %q", doc.ID, doc.Observed.Path, doc.Path)
	}
	if doc.Observed.Present == nil {
		return fmt.Errorf("persisted target drift %q observed.present is required for extra target paths", doc.ID)
	}
	if !*doc.Observed.Present {
		return fmt.Errorf("persisted target drift %q observed path %q must be present for extra target paths", doc.ID, doc.Observed.Path)
	}
	if strings.TrimSpace(doc.Observed.Kind) == "" || doc.Observed.Kind == "missing" {
		return fmt.Errorf("persisted target drift %q observed kind %q is not valid for extra target paths", doc.ID, doc.Observed.Kind)
	}
	return nil
}

func validateExpectedManifestEntry(doc control.TargetDrift, entry control.ManifestEntry) error {
	expected := doc.Expected
	if expected.Kind != entry.Kind {
		return fmt.Errorf("persisted target drift %q expected kind %q does not match manifest entry kind %q", doc.ID, expected.Kind, entry.Kind)
	}
	if expected.Digest != entry.Digest {
		return fmt.Errorf("persisted target drift %q expected digest does not match manifest entry digest", doc.ID)
	}
	if expected.HasSizeEvidence() != entry.HasSizeEvidence() || expected.Size != entry.Size {
		return fmt.Errorf("persisted target drift %q expected size evidence does not match manifest entry size evidence", doc.ID)
	}
	if expected.HasModeEvidence() != entry.HasModeEvidence() || expected.Mode != entry.Mode {
		return fmt.Errorf("persisted target drift %q expected mode evidence does not match manifest entry mode evidence", doc.ID)
	}
	if expected.ModTime != entry.ModTime {
		return fmt.Errorf("persisted target drift %q expected mod_time %q does not match manifest entry mod_time %q", doc.ID, expected.ModTime, entry.ModTime)
	}
	if expected.SymlinkTarget != entry.SymlinkTarget {
		return fmt.Errorf("persisted target drift %q expected symlink target %q does not match manifest entry symlink target %q", doc.ID, expected.SymlinkTarget, entry.SymlinkTarget)
	}
	return nil
}

func manifestEntryTargetPath(entry control.ManifestEntry) string {
	if strings.TrimSpace(entry.TargetPath) != "" {
		return filepath.ToSlash(entry.TargetPath)
	}
	return filepath.ToSlash(entry.Path)
}

func targetRootFromProfile(p profile.Profile) (string, error) {
	if strings.TrimSpace(p.Target.LocalPath) == "" {
		return "", errors.New("target.local_path is required; run profile set-target to persist the trusted target path")
	}
	root, err := filepath.Abs(p.Target.LocalPath)
	if err != nil {
		return "", err
	}
	return root, nil
}
