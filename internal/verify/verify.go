package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/transaction"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type FindingKind string

const (
	FindingMissingFile       FindingKind = "missing_file"
	FindingNotRegular        FindingKind = "not_regular"
	FindingUnsafeTargetPath  FindingKind = "unsafe_target_path"
	FindingSizeMismatch      FindingKind = "size_mismatch"
	FindingDigestMismatch    FindingKind = "digest_mismatch"
	FindingUnsupportedDigest FindingKind = "unsupported_digest"
	FindingDigestMissing     FindingKind = "digest_missing"
	FindingModeMismatch      FindingKind = "mode_mismatch"
	FindingModTimeMismatch   FindingKind = "mtime_mismatch"
	FindingMissingDirectory  FindingKind = "missing_directory"
	FindingNotDirectory      FindingKind = "not_directory"
	FindingMissingSymlink    FindingKind = "missing_symlink"
	FindingNotSymlink        FindingKind = "not_symlink"
	FindingSymlinkMismatch   FindingKind = "symlink_mismatch"
	FindingUnsupportedKind   FindingKind = "unsupported_kind"
	FindingReadError         FindingKind = "read_error"
)

type Options struct {
	TargetRoot string
	SessionID  string
	ProfileID  string
	TargetID   string
}

type Report struct {
	TargetRoot       string                `json:"target_root"`
	SessionID        string                `json:"session_id,omitempty"`
	Manifest         ManifestSummary       `json:"manifest"`
	Summary          Summary               `json:"summary"`
	Findings         []Finding             `json:"findings,omitempty"`
	Warnings         []control.Warning     `json:"warnings,omitempty"`
	SoftDeletes      []control.SoftDelete  `json:"soft_deletes,omitempty"`
	TargetDrifts     []control.TargetDrift `json:"target_drifts,omitempty"`
	ArtifactProblems []ArtifactProblem     `json:"artifact_problems,omitempty"`
	Manifests        []ManifestSummary     `json:"manifests,omitempty"`
}

type Summary struct {
	ManifestCount    int `json:"manifest_count"`
	ManifestEntries  int `json:"manifest_entries"`
	FilesExpected    int `json:"files_expected"`
	FilesVerified    int `json:"files_verified"`
	Warnings         int `json:"warnings"`
	SoftDeletes      int `json:"soft_deletes"`
	TargetDrifts     int `json:"target_drifts"`
	ArtifactProblems int `json:"artifact_problems"`
	ErrorFindings    int `json:"error_findings"`
	WarningFindings  int `json:"warning_findings"`
	SkippedDigest    int `json:"skipped_digest"`
}

type ManifestSummary struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	RootID    string `json:"root_id,omitempty"`
	CreatedAt string `json:"created_at"`
	Entries   int    `json:"entries"`
	Files     int    `json:"files"`
}

type Finding struct {
	Kind            FindingKind `json:"kind"`
	Severity        Severity    `json:"severity"`
	SessionID       string      `json:"session_id"`
	Path            string      `json:"path"`
	TargetPath      string      `json:"target_path"`
	Message         string      `json:"message"`
	ExpectedSize    int64       `json:"expected_size,omitempty"`
	ActualSize      int64       `json:"actual_size,omitempty"`
	ExpectedDigest  string      `json:"expected_digest,omitempty"`
	ActualDigest    string      `json:"actual_digest,omitempty"`
	ExpectedMode    uint32      `json:"expected_mode,omitempty"`
	ActualMode      uint32      `json:"actual_mode,omitempty"`
	ExpectedModTime string      `json:"expected_mtime,omitempty"`
	ActualModTime   string      `json:"actual_mtime,omitempty"`
	Err             string      `json:"error,omitempty"`
}

type ArtifactProblem struct {
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Err       string `json:"error"`
}

type Artifacts struct {
	Manifests         []control.Manifest
	Warnings          []control.Warning
	SoftDeletes       []control.SoftDelete
	TargetDrifts      []control.TargetDrift
	ArtifactProblems  []ArtifactProblem
	KnownSessions     map[string]struct{}
	PublishedSessions map[string]struct{}
	PublishedReceipts map[string]control.SessionReceipt
}

type publishedDocument[T control.Document] struct {
	Doc       T
	Path      string
	SessionID string
}

func BuildReport(opts Options) (Report, error) {
	if strings.TrimSpace(opts.TargetRoot) == "" {
		return Report{}, errors.New("target root is required")
	}
	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return Report{}, err
	}
	scope := identityScope{ProfileID: opts.ProfileID, TargetID: opts.TargetID}
	artifacts, err := loadArtifacts(targetRoot, scope)
	if err != nil {
		return Report{}, err
	}

	manifests := filterManifests(artifacts.Manifests, opts.SessionID)
	report := Report{
		TargetRoot:       filepath.ToSlash(targetRoot),
		SessionID:        opts.SessionID,
		Warnings:         filterWarnings(artifacts.Warnings, opts.SessionID),
		SoftDeletes:      filterSoftDeletes(artifacts.SoftDeletes, opts.SessionID),
		TargetDrifts:     filterTargetDrifts(artifacts.TargetDrifts, opts.SessionID),
		ArtifactProblems: filterArtifactProblems(artifacts.ArtifactProblems, opts.SessionID),
	}
	for _, manifest := range manifests {
		report.Manifests = append(report.Manifests, summarizeManifest(manifest))
	}
	report.Summary.ManifestCount = len(manifests)
	report.Summary.Warnings = len(report.Warnings)
	report.Summary.SoftDeletes = len(report.SoftDeletes)
	report.Summary.TargetDrifts = len(report.TargetDrifts)
	report.Summary.ArtifactProblems = len(report.ArtifactProblems)

	if len(manifests) == 0 {
		if opts.SessionID != "" {
			if err := scopedSessionMismatch(targetRoot, opts.SessionID, scope); err != nil {
				return report, err
			}
			if hasSessionArtifactProblem(report.ArtifactProblems, opts.SessionID) {
				return report, nil
			}
			if len(report.TargetDrifts) > 0 {
				return report, nil
			}
			return report, fmt.Errorf("manifest for session %q not found", opts.SessionID)
		}
		return report, nil
	}

	manifest := manifests[len(manifests)-1]
	report.Manifest = summarizeManifest(manifest)
	report.Summary.ManifestEntries = len(manifest.Entries)
	for _, entry := range manifest.Entries {
		findings := verifyEntry(targetRoot, manifest.SessionID, entry)
		if entry.Kind == "file" {
			report.Summary.FilesExpected++
			if len(findings) == 0 {
				report.Summary.FilesVerified++
				continue
			}
		} else if len(findings) == 0 {
			continue
		}
		for _, finding := range findings {
			switch finding.Severity {
			case SeverityError:
				report.Summary.ErrorFindings++
			case SeverityWarning:
				report.Summary.WarningFindings++
			}
			if finding.Kind == FindingDigestMissing || finding.Kind == FindingUnsupportedDigest {
				report.Summary.SkippedDigest++
			}
			report.Findings = append(report.Findings, finding)
		}
	}
	sortFindings(report.Findings)
	return report, nil
}

func verifyEntry(targetRoot, sessionID string, entry control.ManifestEntry) []Finding {
	switch entry.Kind {
	case "file":
		return verifyFile(targetRoot, sessionID, entry)
	case "dir":
		return verifyDirectory(targetRoot, sessionID, entry)
	case "symlink":
		return verifySymlink(targetRoot, sessionID, entry)
	default:
		return []Finding{{
			Kind:       FindingUnsupportedKind,
			Severity:   SeverityError,
			SessionID:  sessionID,
			Path:       entry.Path,
			TargetPath: targetPath(entry),
			Message:    "manifest entry uses an unsupported kind",
			Err:        entry.Kind,
		}}
	}
}

func LoadArtifacts(targetRoot string) (Artifacts, error) {
	return loadArtifacts(targetRoot, identityScope{})
}

func LoadArtifactsForScope(targetRoot string, profileID string, targetID string) (Artifacts, error) {
	return loadArtifacts(targetRoot, identityScope{ProfileID: profileID, TargetID: targetID})
}

func ValidateArtifactLoadBoundary(targetRoot string) error {
	return control.ValidateArtifactLoadBoundary(targetRoot)
}

type identityScope struct {
	ProfileID string
	TargetID  string
}

func (s identityScope) empty() bool {
	return strings.TrimSpace(s.ProfileID) == "" && strings.TrimSpace(s.TargetID) == ""
}

func (s identityScope) matches(receipt control.SessionReceipt) bool {
	if strings.TrimSpace(s.ProfileID) != "" && receipt.ProfileID != s.ProfileID {
		return false
	}
	if strings.TrimSpace(s.TargetID) != "" && receipt.TargetID != s.TargetID {
		return false
	}
	return true
}

func loadArtifacts(targetRoot string, scope identityScope) (Artifacts, error) {
	var artifacts Artifacts
	if err := control.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return artifacts, err
	}
	controlDir := control.ControlDir(targetRoot)
	if _, err := os.Lstat(controlDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return artifacts, nil
		}
		return artifacts, fmt.Errorf("inspect control directory: %w", err)
	}

	artifacts.KnownSessions, artifacts.PublishedSessions, artifacts.PublishedReceipts, artifacts.ArtifactProblems = readSessionReceipts(controlDir, artifacts.ArtifactProblems, scope)
	artifacts.Manifests, artifacts.ArtifactProblems = readManifests(controlDir, artifacts.PublishedSessions, artifacts.ArtifactProblems)
	artifacts.Warnings, artifacts.ArtifactProblems = readPublishedDocuments[control.Warning](filepath.Join(controlDir, "warnings"), artifacts.KnownSessions, artifacts.PublishedSessions, artifacts.ArtifactProblems)
	manifestRoots := manifestRootIDs(artifacts.Manifests)
	artifacts.SoftDeletes, artifacts.ArtifactProblems = readSoftDeletes(filepath.Join(controlDir, "deleted"), artifacts.KnownSessions, artifacts.PublishedSessions, artifacts.PublishedReceipts, manifestRoots, artifacts.ArtifactProblems)
	artifacts.TargetDrifts, artifacts.ArtifactProblems = readTargetDrifts(filepath.Join(controlDir, "drift"), scope, manifestRoots, artifacts.ArtifactProblems)

	sort.Slice(artifacts.Manifests, func(i, j int) bool {
		left := manifestCreatedAt(artifacts.Manifests[i])
		right := manifestCreatedAt(artifacts.Manifests[j])
		if left.Equal(right) {
			return artifacts.Manifests[i].SessionID < artifacts.Manifests[j].SessionID
		}
		return left.Before(right)
	})
	sort.Slice(artifacts.Warnings, func(i, j int) bool { return artifacts.Warnings[i].ID < artifacts.Warnings[j].ID })
	sort.Slice(artifacts.SoftDeletes, func(i, j int) bool { return artifacts.SoftDeletes[i].ID < artifacts.SoftDeletes[j].ID })
	sort.Slice(artifacts.TargetDrifts, func(i, j int) bool { return artifacts.TargetDrifts[i].ID < artifacts.TargetDrifts[j].ID })
	sort.Slice(artifacts.ArtifactProblems, func(i, j int) bool { return artifacts.ArtifactProblems[i].Path < artifacts.ArtifactProblems[j].Path })
	return artifacts, nil
}

func readSessionReceipts(controlDir string, problems []ArtifactProblem, scope identityScope) (map[string]struct{}, map[string]struct{}, map[string]control.SessionReceipt, []ArtifactProblem) {
	known := map[string]struct{}{}
	published := map[string]struct{}{}
	receipts := map[string]control.SessionReceipt{}
	sessionsDir := filepath.Join(controlDir, "sessions")
	sessions, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return known, published, receipts, problems
		}
		return known, published, receipts, appendProblem(problems, "", sessionsDir, err)
	}

	for _, session := range sessions {
		if !session.IsDir() {
			continue
		}
		known[session.Name()] = struct{}{}
		receiptPath := filepath.Join(sessionsDir, session.Name(), "receipt.json")
		receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			problems = appendProblem(problems, session.Name(), receiptPath, err)
			continue
		}
		if receipt.Status != "published" {
			continue
		}
		if receipt.ID != session.Name() {
			problems = appendProblem(problems, session.Name(), receiptPath, fmt.Errorf("receipt id %q does not match session directory %q", receipt.ID, session.Name()))
			continue
		}
		if !scope.matches(receipt) {
			continue
		}
		published[session.Name()] = struct{}{}
		receipts[session.Name()] = receipt
	}
	return known, published, receipts, problems
}

func scopedSessionMismatch(targetRoot string, sessionID string, scope identityScope) error {
	if scope.empty() {
		return nil
	}
	receiptPath, err := control.Path(targetRoot, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return err
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		return nil
	}
	if receipt.Status != "published" || receipt.ID != sessionID || scope.matches(receipt) {
		return nil
	}
	return fmt.Errorf("session %q receipt profile_id/target_id (%q/%q) does not match requested profile/target (%q/%q)", sessionID, receipt.ProfileID, receipt.TargetID, scope.ProfileID, scope.TargetID)
}

func readManifests(controlDir string, published map[string]struct{}, problems []ArtifactProblem) ([]control.Manifest, []ArtifactProblem) {
	sessionsDir := filepath.Join(controlDir, "sessions")
	sessions, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, problems
		}
		return nil, appendProblem(problems, "", sessionsDir, err)
	}

	var manifests []control.Manifest
	for _, session := range sessions {
		if !session.IsDir() {
			continue
		}
		if _, ok := published[session.Name()]; !ok {
			continue
		}
		path := filepath.Join(sessionsDir, session.Name(), "manifest.json")
		manifest, err := control.ReadManifestCompatFile(path)
		if err != nil {
			problems = appendProblem(problems, session.Name(), path, err)
			continue
		}
		if manifest.SessionID != session.Name() {
			problems = appendProblem(problems, session.Name(), path, fmt.Errorf("manifest session_id %q does not match session directory %q", manifest.SessionID, session.Name()))
			continue
		}
		manifests = append(manifests, manifest)
	}
	return manifests, problems
}

func readPublishedDocuments[T control.Document](dir string, known map[string]struct{}, published map[string]struct{}, problems []ArtifactProblem) ([]T, []ArtifactProblem) {
	documents, problems := readPublishedDocumentFiles[T](dir, known, published, problems)
	docs := make([]T, 0, len(documents))
	for _, document := range documents {
		docs = append(docs, document.Doc)
	}
	return docs, problems
}

func readPublishedDocumentFiles[T control.Document](dir string, known map[string]struct{}, published map[string]struct{}, problems []ArtifactProblem) ([]publishedDocument[T], []ArtifactProblem) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, problems
		}
		return nil, appendProblem(problems, "", dir, err)
	}
	var docs []publishedDocument[T]
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		sessionID := ""
		if detectedSessionID, ok := sessionIDFromArtifactFilename(entry.Name(), known); ok {
			sessionID = detectedSessionID
			if _, published := published[sessionID]; !published {
				continue
			}
		}
		path := filepath.Join(dir, entry.Name())
		doc, err := control.ReadFile[T](path)
		if err != nil {
			problems = appendProblem(problems, sessionID, path, err)
			continue
		}
		if _, ok := published[documentSessionID(doc)]; ok {
			docs = append(docs, publishedDocument[T]{
				Doc:       doc,
				Path:      path,
				SessionID: documentSessionID(doc),
			})
		}
	}
	return docs, problems
}

func readSoftDeletes(dir string, known map[string]struct{}, published map[string]struct{}, receipts map[string]control.SessionReceipt, manifestRoots map[string]string, problems []ArtifactProblem) ([]control.SoftDelete, []ArtifactProblem) {
	records, problems := readPublishedDocumentFiles[control.SoftDelete](dir, known, published, problems)
	out := make([]control.SoftDelete, 0, len(records))
	for _, document := range records {
		record := document.Doc
		receipt, ok := receipts[document.SessionID]
		if !ok {
			out = append(out, record)
			continue
		}
		if record.ProfileID != receipt.ProfileID {
			problems = appendProblem(problems, document.SessionID, document.Path, fmt.Errorf("soft delete profile_id %q does not match session receipt profile_id %q", record.ProfileID, receipt.ProfileID))
			continue
		}
		if record.TargetID != receipt.TargetID {
			problems = appendProblem(problems, document.SessionID, document.Path, fmt.Errorf("soft delete target_id %q does not match session receipt target_id %q", record.TargetID, receipt.TargetID))
			continue
		}
		if strings.TrimSpace(record.RootID) == "" {
			problems = appendProblem(problems, document.SessionID, document.Path, errors.New("soft delete root_id is required"))
			continue
		}
		if rootID := manifestRoots[document.SessionID]; rootID != "" && record.RootID != rootID {
			problems = appendProblem(problems, document.SessionID, document.Path, fmt.Errorf("soft delete root_id %q does not match session manifest root_id %q", record.RootID, rootID))
			continue
		}
		out = append(out, record)
	}
	return out, problems
}

func readTargetDrifts(dir string, scope identityScope, manifestRoots map[string]string, problems []ArtifactProblem) ([]control.TargetDrift, []ArtifactProblem) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, problems
		}
		return nil, appendProblem(problems, "", dir, err)
	}
	controlDir := filepath.Dir(dir)
	var out []control.TargetDrift
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		doc, err := control.ReadFile[control.TargetDrift](path)
		if err != nil {
			scopeHint := readTargetDriftScopeHint(path)
			if err := scopeHint.validate(); err != nil {
				problems = appendProblem(problems, "", path, err)
				continue
			}
			if !scope.empty() && !malformedTargetDriftMatchesScope(controlDir, scopeHint, scope) {
				continue
			}
			problems = appendProblem(problems, scopeHint.SessionID, path, err)
			continue
		}
		receipt, receiptOK, receiptErr := readPublishedReceiptForDrift(controlDir, doc.SessionID)
		if receiptErr != nil {
			problems = appendProblem(problems, doc.SessionID, path, receiptErr)
			continue
		}
		if receiptOK {
			if doc.ProfileID != receipt.ProfileID {
				problems = appendProblem(problems, doc.SessionID, path, fmt.Errorf("target drift profile_id %q does not match session receipt profile_id %q", doc.ProfileID, receipt.ProfileID))
				continue
			}
			if doc.TargetID != receipt.TargetID {
				problems = appendProblem(problems, doc.SessionID, path, fmt.Errorf("target drift target_id %q does not match session receipt target_id %q", doc.TargetID, receipt.TargetID))
				continue
			}
			if rootID := manifestRoots[doc.SessionID]; rootID != "" && doc.RootID != rootID {
				problems = appendProblem(problems, doc.SessionID, path, fmt.Errorf("target drift root_id %q does not match session manifest root_id %q", doc.RootID, rootID))
				continue
			}
		}
		if !scope.empty() {
			if doc.ProfileID != scope.ProfileID || doc.TargetID != scope.TargetID {
				if receiptOK && scope.matches(receipt) {
					problems = appendProblem(problems, doc.SessionID, path, fmt.Errorf("target drift scope (%q/%q) does not match scoped session receipt (%q/%q)", doc.ProfileID, doc.TargetID, receipt.ProfileID, receipt.TargetID))
				}
				continue
			}
			if receiptOK && !scope.matches(receipt) {
				continue
			}
		}
		if strings.TrimSpace(doc.ReviewState) == "" {
			doc.ReviewState = "needs_review"
		}
		out = append(out, doc)
	}
	return out, problems
}

type targetDriftScopeHint struct {
	SessionID string `json:"session_id"`
	ProfileID string `json:"profile_id"`
	TargetID  string `json:"target_id"`
}

func (h targetDriftScopeHint) empty() bool {
	return h.SessionID == "" && h.ProfileID == "" && h.TargetID == ""
}

func (h targetDriftScopeHint) validate() error {
	if h.SessionID == "" {
		return nil
	}
	return transaction.ValidateSessionID(h.SessionID)
}

func (h targetDriftScopeHint) matches(scope identityScope) bool {
	profileMatches := h.ProfileID == "" || h.ProfileID == scope.ProfileID
	targetMatches := h.TargetID == "" || h.TargetID == scope.TargetID
	return profileMatches && targetMatches
}

func malformedTargetDriftMatchesScope(controlDir string, hint targetDriftScopeHint, scope identityScope) bool {
	if scope.empty() || hint.empty() {
		return true
	}
	if hint.SessionID != "" {
		receipt, ok, err := readSessionReceiptForDriftScope(controlDir, hint.SessionID)
		if err == nil && ok {
			return scope.matches(receipt)
		}
	}
	return hint.matches(scope)
}

func readTargetDriftScopeHint(path string) targetDriftScopeHint {
	file, err := os.Open(path)
	if err != nil {
		return targetDriftScopeHint{}
	}
	defer file.Close()

	var hint targetDriftScopeHint
	decoder := json.NewDecoder(file)
	token, err := decoder.Token()
	if err != nil {
		return targetDriftScopeHint{}
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return targetDriftScopeHint{}
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return hint
		}
		key, ok := token.(string)
		if !ok {
			return hint
		}
		value, ok, err := readJSONStringToken(decoder)
		if err != nil {
			return hint
		}
		if !ok {
			continue
		}
		switch key {
		case "session_id":
			hint.SessionID = value
		case "profile_id":
			hint.ProfileID = value
		case "target_id":
			hint.TargetID = value
		}
	}
	return hint
}

func readJSONStringToken(decoder *json.Decoder) (string, bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", false, err
	}
	if delim, ok := token.(json.Delim); ok {
		if err := skipJSONDelimited(decoder, delim); err != nil {
			return "", false, err
		}
		return "", false, nil
	}
	value, ok := token.(string)
	return value, ok, nil
}

func skipJSONDelimited(decoder *json.Decoder, start json.Delim) error {
	var end json.Delim
	switch start {
	case '{':
		end = '}'
	case '[':
		end = ']'
	default:
		return nil
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			continue
		}
		if delim == end {
			return nil
		}
		if delim == '{' || delim == '[' {
			if err := skipJSONDelimited(decoder, delim); err != nil {
				return err
			}
		}
	}
}

func readPublishedReceiptForDrift(controlDir string, sessionID string) (control.SessionReceipt, bool, error) {
	receiptPath := filepath.Join(controlDir, "sessions", sessionID, "receipt.json")
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return control.SessionReceipt{}, false, nil
		}
		return control.SessionReceipt{}, false, fmt.Errorf("read target drift session receipt: %w", err)
	}
	if receipt.Status != "published" {
		return control.SessionReceipt{}, false, nil
	}
	return receipt, true, nil
}

func readSessionReceiptForDriftScope(controlDir string, sessionID string) (control.SessionReceipt, bool, error) {
	receiptPath := filepath.Join(controlDir, "sessions", sessionID, "receipt.json")
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return control.SessionReceipt{}, false, nil
		}
		return control.SessionReceipt{}, false, fmt.Errorf("read target drift session receipt: %w", err)
	}
	if receipt.ID != sessionID {
		return control.SessionReceipt{}, false, fmt.Errorf("receipt id %q does not match session directory %q", receipt.ID, sessionID)
	}
	return receipt, true, nil
}

func manifestRootIDs(manifests []control.Manifest) map[string]string {
	roots := make(map[string]string, len(manifests))
	for _, manifest := range manifests {
		roots[manifest.SessionID] = manifest.RootID
	}
	return roots
}

func sessionIDFromArtifactFilename(name string, known map[string]struct{}) (string, bool) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	best := ""
	for sessionID := range known {
		suffix, ok := strings.CutPrefix(base, sessionID)
		if !ok {
			continue
		}
		if strings.HasPrefix(suffix, "-del_") || hasWarningSequencePrefix(suffix) {
			if len(sessionID) > len(best) {
				best = sessionID
			}
		}
	}
	return best, best != ""
}

func hasWarningSequencePrefix(suffix string) bool {
	return len(suffix) > 5 &&
		suffix[0] == '-' &&
		suffix[4] == '-' &&
		suffix[1] >= '0' && suffix[1] <= '9' &&
		suffix[2] >= '0' && suffix[2] <= '9' &&
		suffix[3] >= '0' && suffix[3] <= '9'
}

func documentSessionID[T control.Document](doc T) string {
	switch value := any(doc).(type) {
	case control.Warning:
		return value.SessionID
	case control.SoftDelete:
		return value.SessionID
	default:
		return ""
	}
}

func manifestCreatedAt(manifest control.Manifest) time.Time {
	ts, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
	if err == nil {
		return ts.UTC()
	}
	return time.Time{}
}

func verifyFile(targetRoot, sessionID string, entry control.ManifestEntry) []Finding {
	targetRel := targetPath(entry)
	fullPath, err := safeTargetPath(targetRoot, targetRel)
	if err != nil {
		return []Finding{{
			Kind:       FindingUnsafeTargetPath,
			Severity:   SeverityError,
			SessionID:  sessionID,
			Path:       entry.Path,
			TargetPath: targetRel,
			Message:    "manifest target path escapes the target root",
			Err:        err.Error(),
		}}
	}
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
		return []Finding{{
			Kind:       FindingUnsafeTargetPath,
			Severity:   SeverityError,
			SessionID:  sessionID,
			Path:       entry.Path,
			TargetPath: targetRel,
			Message:    "manifest file target parent is unsafe",
			Err:        err.Error(),
		}}
	}

	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Finding{{
				Kind:         FindingMissingFile,
				Severity:     SeverityError,
				SessionID:    sessionID,
				Path:         entry.Path,
				TargetPath:   targetRel,
				Message:      "manifest file is missing from the target",
				ExpectedSize: entry.Size,
			}}
		}
		return []Finding{readErrorFinding(sessionID, entry, targetRel, err)}
	}
	if !info.Mode().IsRegular() {
		return []Finding{{
			Kind:       FindingNotRegular,
			Severity:   SeverityError,
			SessionID:  sessionID,
			Path:       entry.Path,
			TargetPath: targetRel,
			Message:    "manifest file target is not a regular file",
			Err:        info.Mode().String(),
		}}
	}

	var findings []Finding
	if expectedMode := os.FileMode(entry.Mode).Perm(); expectedMode != 0 && info.Mode().Perm() != expectedMode {
		findings = append(findings, Finding{
			Kind:         FindingModeMismatch,
			Severity:     SeverityError,
			SessionID:    sessionID,
			Path:         entry.Path,
			TargetPath:   targetRel,
			Message:      "target file permissions do not match the manifest",
			ExpectedMode: uint32(expectedMode),
			ActualMode:   uint32(info.Mode().Perm()),
		})
	}
	if modTime, ok := parseManifestTime(entry.ModTime); ok && !info.ModTime().Equal(modTime) {
		findings = append(findings, Finding{
			Kind:            FindingModTimeMismatch,
			Severity:        SeverityError,
			SessionID:       sessionID,
			Path:            entry.Path,
			TargetPath:      targetRel,
			Message:         "target file modification time does not match the manifest",
			ExpectedModTime: modTime.UTC().Format(time.RFC3339Nano),
			ActualModTime:   info.ModTime().UTC().Format(time.RFC3339Nano),
		})
	}
	if info.Size() != entry.Size {
		findings = append(findings, Finding{
			Kind:         FindingSizeMismatch,
			Severity:     SeverityError,
			SessionID:    sessionID,
			Path:         entry.Path,
			TargetPath:   targetRel,
			Message:      "target file size does not match the manifest",
			ExpectedSize: entry.Size,
			ActualSize:   info.Size(),
		})
	}
	if strings.TrimSpace(entry.Digest) == "" {
		findings = append(findings, Finding{
			Kind:         FindingDigestMissing,
			Severity:     SeverityWarning,
			SessionID:    sessionID,
			Path:         entry.Path,
			TargetPath:   targetRel,
			Message:      "manifest entry has no digest to verify",
			ExpectedSize: entry.Size,
			ActualSize:   info.Size(),
		})
		return findings
	}
	if !strings.HasPrefix(entry.Digest, "sha256:") {
		findings = append(findings, Finding{
			Kind:           FindingUnsupportedDigest,
			Severity:       SeverityWarning,
			SessionID:      sessionID,
			Path:           entry.Path,
			TargetPath:     targetRel,
			Message:        "manifest entry uses an unsupported digest algorithm",
			ExpectedDigest: entry.Digest,
		})
		return findings
	}

	actualDigest, err := sha256File(fullPath)
	if err != nil {
		findings = append(findings, readErrorFinding(sessionID, entry, targetRel, err))
		return findings
	}
	if actualDigest != entry.Digest {
		findings = append(findings, Finding{
			Kind:           FindingDigestMismatch,
			Severity:       SeverityError,
			SessionID:      sessionID,
			Path:           entry.Path,
			TargetPath:     targetRel,
			Message:        "target file digest does not match the manifest",
			ExpectedDigest: entry.Digest,
			ActualDigest:   actualDigest,
		})
	}
	return findings
}

func verifyDirectory(targetRoot, sessionID string, entry control.ManifestEntry) []Finding {
	targetRel := targetPath(entry)
	fullPath, err := safeTargetPath(targetRoot, targetRel)
	if err != nil {
		return []Finding{unsafeEntryFinding(sessionID, entry, targetRel, "manifest directory target path escapes the target root", err)}
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
				return []Finding{unsafeEntryFinding(sessionID, entry, targetRel, "manifest directory target parent is unsafe", err)}
			}
			return []Finding{{
				Kind:       FindingMissingDirectory,
				Severity:   SeverityError,
				SessionID:  sessionID,
				Path:       entry.Path,
				TargetPath: targetRel,
				Message:    "manifest directory is missing from the target",
			}}
		}
		return []Finding{readErrorFinding(sessionID, entry, targetRel, err)}
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return []Finding{{
			Kind:       FindingNotDirectory,
			Severity:   SeverityError,
			SessionID:  sessionID,
			Path:       entry.Path,
			TargetPath: targetRel,
			Message:    "manifest directory target is not a plain directory",
			Err:        info.Mode().String(),
		}}
	}
	return nil
}

func verifySymlink(targetRoot, sessionID string, entry control.ManifestEntry) []Finding {
	targetRel := targetPath(entry)
	fullPath, err := safeTargetPath(targetRoot, targetRel)
	if err != nil {
		return []Finding{unsafeEntryFinding(sessionID, entry, targetRel, "manifest symlink target path escapes the target root", err)}
	}
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
		return []Finding{unsafeEntryFinding(sessionID, entry, targetRel, "manifest symlink target parent is unsafe", err)}
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Finding{{
				Kind:       FindingMissingSymlink,
				Severity:   SeverityError,
				SessionID:  sessionID,
				Path:       entry.Path,
				TargetPath: targetRel,
				Message:    "manifest symlink is missing from the target",
			}}
		}
		return []Finding{readErrorFinding(sessionID, entry, targetRel, err)}
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return []Finding{{
			Kind:       FindingNotSymlink,
			Severity:   SeverityError,
			SessionID:  sessionID,
			Path:       entry.Path,
			TargetPath: targetRel,
			Message:    "manifest symlink target is not a symlink",
			Err:        info.Mode().String(),
		}}
	}
	got, err := os.Readlink(fullPath)
	if err != nil {
		return []Finding{readErrorFinding(sessionID, entry, targetRel, err)}
	}
	if got != entry.SymlinkTarget {
		return []Finding{{
			Kind:       FindingSymlinkMismatch,
			Severity:   SeverityError,
			SessionID:  sessionID,
			Path:       entry.Path,
			TargetPath: targetRel,
			Message:    "target symlink destination does not match the manifest",
			Err:        got,
		}}
	}
	return nil
}

func unsafeEntryFinding(sessionID string, entry control.ManifestEntry, targetRel string, message string, err error) Finding {
	return Finding{
		Kind:       FindingUnsafeTargetPath,
		Severity:   SeverityError,
		SessionID:  sessionID,
		Path:       entry.Path,
		TargetPath: targetRel,
		Message:    message,
		Err:        err.Error(),
	}
}

func parseManifestTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

func readErrorFinding(sessionID string, entry control.ManifestEntry, targetRel string, err error) Finding {
	return Finding{
		Kind:       FindingReadError,
		Severity:   SeverityError,
		SessionID:  sessionID,
		Path:       entry.Path,
		TargetPath: targetRel,
		Message:    "could not read target file for verification",
		Err:        err.Error(),
	}
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return sha256FileReader(file)
}

func sha256FileReader(file *os.File) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func filterManifests(manifests []control.Manifest, sessionID string) []control.Manifest {
	if sessionID == "" {
		return append([]control.Manifest(nil), manifests...)
	}
	var out []control.Manifest
	for _, manifest := range manifests {
		if manifest.SessionID == sessionID {
			out = append(out, manifest)
		}
	}
	return out
}

func filterWarnings(warnings []control.Warning, sessionID string) []control.Warning {
	if sessionID == "" {
		return append([]control.Warning(nil), warnings...)
	}
	var out []control.Warning
	for _, warning := range warnings {
		if warning.SessionID == "" || warning.SessionID == sessionID {
			out = append(out, warning)
		}
	}
	return out
}

func filterSoftDeletes(records []control.SoftDelete, sessionID string) []control.SoftDelete {
	if sessionID == "" {
		return append([]control.SoftDelete(nil), records...)
	}
	var out []control.SoftDelete
	for _, record := range records {
		if record.SessionID == "" || record.SessionID == sessionID {
			out = append(out, record)
		}
	}
	return out
}

func filterTargetDrifts(records []control.TargetDrift, sessionID string) []control.TargetDrift {
	if sessionID == "" {
		out := make([]control.TargetDrift, 0, len(records))
		for _, record := range records {
			if strings.TrimSpace(record.ReviewState) == "resolved" {
				continue
			}
			out = append(out, record)
		}
		return out
	}
	var out []control.TargetDrift
	for _, record := range records {
		if strings.TrimSpace(record.ReviewState) == "resolved" {
			continue
		}
		if record.SessionID == "" || record.SessionID == sessionID {
			out = append(out, record)
		}
	}
	return out
}

func filterArtifactProblems(problems []ArtifactProblem, sessionID string) []ArtifactProblem {
	if sessionID == "" {
		return append([]ArtifactProblem(nil), problems...)
	}
	var out []ArtifactProblem
	for _, problem := range problems {
		if problem.SessionID == "" || problem.SessionID == sessionID {
			out = append(out, problem)
		}
	}
	return out
}

func hasSessionArtifactProblem(problems []ArtifactProblem, sessionID string) bool {
	for _, problem := range problems {
		if problem.SessionID == sessionID {
			return true
		}
	}
	return false
}

func summarizeManifest(manifest control.Manifest) ManifestSummary {
	summary := ManifestSummary{
		ID:        manifest.ID,
		SessionID: manifest.SessionID,
		RootID:    manifest.RootID,
		CreatedAt: manifest.CreatedAt,
		Entries:   len(manifest.Entries),
	}
	for _, entry := range manifest.Entries {
		if entry.Kind == "file" {
			summary.Files++
		}
	}
	return summary
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path == findings[j].Path {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Path < findings[j].Path
	})
}

func targetPath(entry control.ManifestEntry) string {
	if strings.TrimSpace(entry.TargetPath) != "" {
		return filepath.ToSlash(entry.TargetPath)
	}
	return filepath.ToSlash(entry.Path)
}

func safeTargetPath(root, rel string) (string, error) {
	if err := pathguard.ValidateSlashRelativePath(rel, 0); err != nil {
		return "", err
	}
	if pathguard.IsReservedControlPath(rel) {
		return "", fmt.Errorf("reserved control-plane target path %q", rel)
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func appendProblem(problems []ArtifactProblem, sessionID string, path string, err error) []ArtifactProblem {
	return append(problems, ArtifactProblem{
		SessionID: sessionID,
		Path:      filepath.ToSlash(path),
		Err:       err.Error(),
	})
}
