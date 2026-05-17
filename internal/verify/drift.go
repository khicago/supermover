package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
)

type DriftOptions struct {
	TargetRoot string
	SessionID  string
	ProfileID  string
	TargetID   string
	Now        time.Time
}

type DriftReport struct {
	TargetRoot       string                `json:"target_root"`
	SessionID        string                `json:"session_id,omitempty"`
	Manifest         ManifestSummary       `json:"manifest"`
	Summary          DriftSummary          `json:"summary"`
	Drifts           []control.TargetDrift `json:"target_drifts,omitempty"`
	ArtifactProblems []ArtifactProblem     `json:"artifact_problems,omitempty"`
	Manifests        []ManifestSummary     `json:"manifests,omitempty"`
}

type DriftSummary struct {
	ManifestCount    int `json:"manifest_count"`
	ManifestEntries  int `json:"manifest_entries"`
	TargetDrifts     int `json:"target_drifts"`
	ArtifactProblems int `json:"artifact_problems"`
}

func (r DriftReport) NeedsReview() bool {
	return r.Summary.TargetDrifts > 0 ||
		r.Summary.ArtifactProblems > 0 ||
		r.Summary.ManifestCount == 0
}

func DetectTargetDrift(opts DriftOptions) (DriftReport, error) {
	if strings.TrimSpace(opts.TargetRoot) == "" {
		return DriftReport{}, errors.New("target root is required")
	}
	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return DriftReport{}, err
	}
	scope := identityScope{ProfileID: opts.ProfileID, TargetID: opts.TargetID}
	artifacts, err := loadArtifacts(targetRoot, scope)
	if err != nil {
		return DriftReport{}, err
	}

	manifests := filterManifests(artifacts.Manifests, opts.SessionID)
	report := DriftReport{
		TargetRoot:       filepath.ToSlash(targetRoot),
		SessionID:        opts.SessionID,
		ArtifactProblems: filterArtifactProblems(artifacts.ArtifactProblems, opts.SessionID),
	}
	for _, manifest := range manifests {
		report.Manifests = append(report.Manifests, summarizeManifest(manifest))
	}
	report.Summary.ManifestCount = len(manifests)
	report.Summary.ArtifactProblems = len(report.ArtifactProblems)

	if len(manifests) == 0 {
		if opts.SessionID != "" {
			if err := scopedSessionMismatch(targetRoot, opts.SessionID, scope); err != nil {
				return report, err
			}
			if hasSessionArtifactProblem(report.ArtifactProblems, opts.SessionID) {
				return report, nil
			}
			return report, fmt.Errorf("manifest for session %q not found", opts.SessionID)
		}
		return report, nil
	}

	manifest := manifests[len(manifests)-1]
	report.SessionID = manifest.SessionID
	report.Manifest = summarizeManifest(manifest)
	report.Summary.ManifestEntries = len(manifest.Entries)

	receipt, ok := artifacts.PublishedReceipts[manifest.SessionID]
	if !ok {
		report.ArtifactProblems = appendProblem(report.ArtifactProblems, manifest.SessionID, targetRoot, errors.New("published receipt for selected manifest is missing"))
		report.Summary.ArtifactProblems = len(report.ArtifactProblems)
		return report, nil
	}
	if strings.TrimSpace(manifest.RootID) == "" {
		report.ArtifactProblems = appendProblem(report.ArtifactProblems, manifest.SessionID, targetRoot, errors.New("manifest root_id is required for target drift detection"))
		report.Summary.ArtifactProblems = len(report.ArtifactProblems)
		return report, nil
	}

	detectedAt := opts.Now
	if detectedAt.IsZero() {
		detectedAt = time.Now()
	}
	detectedAt = detectedAt.UTC()

	expectedPaths := make(map[string]control.ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		rel := targetPath(entry)
		if _, ok := expectedPaths[rel]; !ok {
			expectedPaths[rel] = entry
		}
		drifts, problems := detectEntryDrift(targetRoot, manifest, receipt, entry, detectedAt)
		report.Drifts = append(report.Drifts, drifts...)
		report.ArtifactProblems = append(report.ArtifactProblems, problems...)
	}

	if opts.SessionID == "" || isLatestManifest(artifacts.Manifests, manifest) {
		reviewedPaths := reviewedTargetPaths(artifacts.SoftDeletes, manifest.SessionID)
		drifts, problems := detectExtraTargetPaths(targetRoot, manifest, receipt, expectedPaths, reviewedPaths, detectedAt)
		report.Drifts = append(report.Drifts, drifts...)
		report.ArtifactProblems = append(report.ArtifactProblems, problems...)
	}
	sortTargetDrifts(report.Drifts)
	report.Summary.TargetDrifts = len(report.Drifts)
	report.Summary.ArtifactProblems = len(report.ArtifactProblems)
	return report, nil
}

func detectEntryDrift(targetRoot string, manifest control.Manifest, receipt control.SessionReceipt, entry control.ManifestEntry, detectedAt time.Time) ([]control.TargetDrift, []ArtifactProblem) {
	targetRel := targetPath(entry)
	fullPath, err := safeTargetPath(targetRoot, targetRel)
	if err != nil {
		return nil, []ArtifactProblem{{
			SessionID: manifest.SessionID,
			Path:      filepath.ToSlash(filepath.Join(targetRoot, targetRel)),
			Err:       fmt.Errorf("manifest target path is unsafe: %w", err).Error(),
		}}
	}
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
		return singleGeneratedDrift(manifest, receipt, entry, targetRel, "unsafe_parent", detectedAt, control.TargetDriftObservedState{
			Present: boolPtr(true),
			Kind:    "other",
			Path:    targetRel,
		}, []string{
			fmt.Sprintf("target parent is unsafe: %v", err),
			"detector did not follow the parent path",
		})
	}

	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return singleGeneratedDrift(manifest, receipt, entry, targetRel, "missing", detectedAt, missingObservedState(targetRel), []string{"target path is missing"})
		}
		return singleGeneratedDrift(manifest, receipt, entry, targetRel, "read_error", detectedAt, control.TargetDriftObservedState{
			Present: boolPtr(false),
			Kind:    "missing",
			Path:    targetRel,
		}, []string{fmt.Sprintf("could not inspect target path: %v", err)})
	}

	observedKind := driftKindFromInfo(info)
	if observedKind != entry.Kind {
		observed, evidence := observedState(fullPath, targetRel, info, false)
		evidence = append(evidence, fmt.Sprintf("expected kind %q but found %q", entry.Kind, observedKind))
		return singleGeneratedDrift(manifest, receipt, entry, targetRel, "type_mismatch", detectedAt, observed, evidence)
	}

	switch entry.Kind {
	case "file":
		return detectFileEntryDrift(fullPath, targetRel, manifest, receipt, entry, info, detectedAt)
	case "symlink":
		return detectSymlinkEntryDrift(fullPath, targetRel, manifest, receipt, entry, info, detectedAt)
	default:
		return nil, nil
	}
}

func detectFileEntryDrift(fullPath, targetRel string, manifest control.Manifest, receipt control.SessionReceipt, entry control.ManifestEntry, info fs.FileInfo, detectedAt time.Time) ([]control.TargetDrift, []ArtifactProblem) {
	var evidence []string
	if entry.HasSizeEvidence() && info.Size() != entry.Size {
		evidence = append(evidence, fmt.Sprintf("size mismatch: expected %d, observed %d", entry.Size, info.Size()))
	}
	if expectedMode := os.FileMode(entry.Mode).Perm(); entry.HasModeEvidence() && info.Mode().Perm() != expectedMode {
		evidence = append(evidence, fmt.Sprintf("mode mismatch: expected %04o, observed %04o", expectedMode, info.Mode().Perm()))
	}
	if modTime, ok := parseManifestTime(entry.ModTime); ok && !info.ModTime().Equal(modTime) {
		evidence = append(evidence, fmt.Sprintf("mtime mismatch: expected %s, observed %s", modTime.UTC().Format(time.RFC3339Nano), info.ModTime().UTC().Format(time.RFC3339Nano)))
	}
	digestEvidence := false
	if strings.TrimSpace(entry.Digest) != "" && strings.HasPrefix(entry.Digest, "sha256:") {
		actualDigest, err := sha256ObservedRegular(fullPath, info)
		if err != nil {
			return singleGeneratedDrift(manifest, receipt, entry, targetRel, "read_error", detectedAt, control.TargetDriftObservedState{
				Present: boolPtr(true),
				Kind:    "file",
				Path:    targetRel,
			}, []string{fmt.Sprintf("could not compute target digest: %v", err)})
		}
		if actualDigest != entry.Digest {
			digestEvidence = true
			evidence = append(evidence, fmt.Sprintf("digest mismatch: expected %s, observed %s", entry.Digest, actualDigest))
		}
	}
	if len(evidence) == 0 {
		return nil, nil
	}
	observed, observedEvidence := observedState(fullPath, targetRel, info, true)
	evidence = append(evidence, observedEvidence...)
	change := "metadata_mismatch"
	if digestEvidence || (entry.HasSizeEvidence() && info.Size() != entry.Size) {
		change = "content_mismatch"
	}
	return singleGeneratedDrift(manifest, receipt, entry, targetRel, change, detectedAt, observed, evidence)
}

func detectSymlinkEntryDrift(fullPath, targetRel string, manifest control.Manifest, receipt control.SessionReceipt, entry control.ManifestEntry, info fs.FileInfo, detectedAt time.Time) ([]control.TargetDrift, []ArtifactProblem) {
	got, err := os.Readlink(fullPath)
	if err != nil {
		observed, evidence := observedState(fullPath, targetRel, info, false)
		evidence = append(evidence, fmt.Sprintf("could not read symlink target: %v", err))
		return singleGeneratedDrift(manifest, receipt, entry, targetRel, "read_error", detectedAt, observed, evidence)
	}
	if got == entry.SymlinkTarget {
		return nil, nil
	}
	observed, evidence := observedState(fullPath, targetRel, info, false)
	evidence = append(evidence, fmt.Sprintf("symlink target mismatch: expected %q, observed %q", entry.SymlinkTarget, got))
	return singleGeneratedDrift(manifest, receipt, entry, targetRel, "symlink_mismatch", detectedAt, observed, evidence)
}

func detectExtraTargetPaths(targetRoot string, manifest control.Manifest, receipt control.SessionReceipt, expectedPaths map[string]control.ManifestEntry, reviewedPaths map[string]struct{}, detectedAt time.Time) ([]control.TargetDrift, []ArtifactProblem) {
	var drifts []control.TargetDrift
	var problems []ArtifactProblem
	err := filepath.WalkDir(targetRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			problems = appendProblem(problems, manifest.SessionID, path, walkErr)
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == targetRoot {
			return nil
		}
		rel, err := filepath.Rel(targetRoot, path)
		if err != nil {
			problems = appendProblem(problems, manifest.SessionID, path, err)
			return nil
		}
		targetRel := filepath.ToSlash(rel)
		if pathguard.IsReservedControlPath(targetRel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := expectedPaths[targetRel]; ok {
			return nil
		}
		if hasExpectedDescendant(expectedPaths, targetRel) {
			return nil
		}
		if _, ok := reviewedPaths[targetRel]; ok {
			return nil
		}
		if err := pathguard.ValidateSlashRelativePath(targetRel, 0); err != nil {
			problems = appendProblem(problems, manifest.SessionID, path, err)
			return nil
		}
		if entry.IsDir() {
			children, err := os.ReadDir(path)
			if err != nil {
				problems = appendProblem(problems, manifest.SessionID, path, err)
				return nil
			}
			if len(children) > 0 {
				return nil
			}
		}
		info, err := os.Lstat(path)
		if err != nil {
			problems = appendProblem(problems, manifest.SessionID, path, err)
			return nil
		}
		observed, evidence := observedState(path, targetRel, info, true)
		evidence = append(evidence, "target path is not present in the selected manifest")
		drift, err := generatedTargetDrift(manifest, receipt, control.ManifestEntry{
			Path:       targetRel,
			Kind:       "missing",
			TargetPath: targetRel,
		}, targetRel, "extra", detectedAt, observed, evidence)
		if err != nil {
			problems = appendProblem(problems, manifest.SessionID, path, err)
			return nil
		}
		drifts = append(drifts, drift)
		return nil
	})
	if err != nil {
		problems = appendProblem(problems, manifest.SessionID, targetRoot, err)
	}
	return drifts, problems
}

func hasExpectedDescendant(expectedPaths map[string]control.ManifestEntry, targetRel string) bool {
	prefix := strings.TrimSuffix(targetRel, "/") + "/"
	for rel := range expectedPaths {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func isLatestManifest(manifests []control.Manifest, manifest control.Manifest) bool {
	if len(manifests) == 0 {
		return false
	}
	latest := manifests[len(manifests)-1]
	return latest.SessionID == manifest.SessionID && latest.ID == manifest.ID
}

func reviewedTargetPaths(records []control.SoftDelete, sessionID string) map[string]struct{} {
	paths := map[string]struct{}{}
	for _, record := range filterSoftDeletes(records, sessionID) {
		targetRel := filepath.ToSlash(record.TargetPath)
		if strings.TrimSpace(targetRel) == "" {
			continue
		}
		paths[targetRel] = struct{}{}
	}
	return paths
}

func singleGeneratedDrift(manifest control.Manifest, receipt control.SessionReceipt, entry control.ManifestEntry, targetRel string, change string, detectedAt time.Time, observed control.TargetDriftObservedState, evidence []string) ([]control.TargetDrift, []ArtifactProblem) {
	drift, err := generatedTargetDrift(manifest, receipt, entry, targetRel, change, detectedAt, observed, evidence)
	if err != nil {
		return nil, []ArtifactProblem{{
			SessionID: manifest.SessionID,
			Path:      targetRel,
			Err:       err.Error(),
		}}
	}
	return []control.TargetDrift{drift}, nil
}

func generatedTargetDrift(manifest control.Manifest, receipt control.SessionReceipt, entry control.ManifestEntry, targetRel string, change string, detectedAt time.Time, observed control.TargetDriftObservedState, evidence []string) (control.TargetDrift, error) {
	expected := expectedState(manifest, entry, targetRel)
	id, err := detectedTargetDriftID(receipt, manifest, targetRel, change, expected, observed, evidence)
	if err != nil {
		return control.TargetDrift{}, err
	}
	drift := control.TargetDrift{
		Version:     control.CurrentVersion,
		ID:          id,
		SessionID:   manifest.SessionID,
		ProfileID:   receipt.ProfileID,
		TargetID:    receipt.TargetID,
		RootID:      manifest.RootID,
		Path:        targetRel,
		DetectedAt:  detectedAt.Format(time.RFC3339Nano),
		Change:      change,
		Expected:    expected,
		Observed:    observed,
		ReviewState: "needs_review",
		Evidence:    evidence,
	}
	if err := drift.Validate(); err != nil {
		return control.TargetDrift{}, fmt.Errorf("generated target drift is invalid: %w", err)
	}
	return drift, nil
}

func expectedState(manifest control.Manifest, entry control.ManifestEntry, targetRel string) control.TargetDriftExpectedState {
	if entry.Kind == "missing" {
		return control.TargetDriftExpectedState{
			SessionID:  manifest.SessionID,
			ManifestID: manifest.ID,
			Kind:       "missing",
			Path:       targetRel,
		}
	}
	state := control.TargetDriftExpectedState{
		SessionID:     manifest.SessionID,
		ManifestID:    manifest.ID,
		Kind:          entry.Kind,
		Path:          targetRel,
		Digest:        entry.Digest,
		ModTime:       entry.ModTime,
		SymlinkTarget: entry.SymlinkTarget,
	}
	if entry.HasSizeEvidence() {
		state.SetSizeEvidence(entry.Size)
	}
	if entry.HasModeEvidence() {
		state.SetModeEvidence(entry.Mode)
	}
	return state
}

func observedState(path, targetRel string, info fs.FileInfo, includeDigest bool) (control.TargetDriftObservedState, []string) {
	state := control.TargetDriftObservedState{
		Present: boolPtr(true),
		Kind:    driftKindFromInfo(info),
		Path:    targetRel,
		ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	state.SetModeEvidence(uint32(info.Mode().Perm()))
	var evidence []string
	switch state.Kind {
	case "file":
		state.SetSizeEvidence(info.Size())
		if includeDigest {
			digest, err := sha256ObservedRegular(path, info)
			if err != nil {
				evidence = append(evidence, fmt.Sprintf("could not compute observed digest: %v", err))
			} else {
				state.Digest = digest
			}
		}
	case "symlink":
		target, err := os.Readlink(path)
		if err != nil {
			evidence = append(evidence, fmt.Sprintf("could not read observed symlink target: %v", err))
			break
		}
		if err := pathguard.ValidateRelativeSymlinkTarget(target); err != nil {
			evidence = append(evidence, fmt.Sprintf("observed symlink target %q is unsafe: %v", target, err))
			break
		}
		state.SymlinkTarget = target
	}
	return state, evidence
}

func missingObservedState(targetRel string) control.TargetDriftObservedState {
	return control.TargetDriftObservedState{
		Present: boolPtr(false),
		Kind:    "missing",
		Path:    targetRel,
	}
}

func sha256ObservedRegular(path string, observed fs.FileInfo) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(observed, opened) {
		return "", errors.New("regular file changed before digest")
	}
	digest, err := sha256FileReader(file)
	if err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !after.Mode().IsRegular() || !os.SameFile(observed, after) || after.Size() != observed.Size() || after.Mode().Perm() != observed.Mode().Perm() || !after.ModTime().Equal(observed.ModTime()) {
		return "", errors.New("regular file changed during digest")
	}
	return digest, nil
}

func driftKindFromInfo(info fs.FileInfo) string {
	if info.Mode()&os.ModeSymlink != 0 {
		return "symlink"
	}
	if info.Mode().IsRegular() {
		return "file"
	}
	if info.IsDir() {
		return "dir"
	}
	return "special"
}

func detectedTargetDriftID(receipt control.SessionReceipt, manifest control.Manifest, targetRel string, change string, expected control.TargetDriftExpectedState, observed control.TargetDriftObservedState, evidence []string) (string, error) {
	payload := struct {
		SessionID string                           `json:"session_id"`
		ProfileID string                           `json:"profile_id"`
		TargetID  string                           `json:"target_id"`
		RootID    string                           `json:"root_id"`
		Path      string                           `json:"path"`
		Change    string                           `json:"change"`
		Expected  control.TargetDriftExpectedState `json:"expected"`
		Observed  control.TargetDriftObservedState `json:"observed"`
		Evidence  []string                         `json:"evidence,omitempty"`
	}{
		SessionID: manifest.SessionID,
		ProfileID: receipt.ProfileID,
		TargetID:  receipt.TargetID,
		RootID:    manifest.RootID,
		Path:      targetRel,
		Change:    change,
		Expected:  expected,
		Observed:  observed,
		Evidence:  evidence,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal detected target drift identity: %w", err)
	}
	sum := sha256.Sum256(data)
	return "detected_" + hex.EncodeToString(sum[:])[:24], nil
}

func boolPtr(value bool) *bool {
	return &value
}

func sortTargetDrifts(drifts []control.TargetDrift) {
	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].Path == drifts[j].Path {
			if drifts[i].Change == drifts[j].Change {
				return drifts[i].ID < drifts[j].ID
			}
			return drifts[i].Change < drifts[j].Change
		}
		return drifts[i].Path < drifts[j].Path
	})
}
