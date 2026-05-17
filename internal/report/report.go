package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/health"
	"github.com/khicago/supermover/internal/verify"
)

const ScopeLocalMigrationTarget = "local_migration_target"

type Status string

const (
	StatusEmpty     Status = "local_target_empty"
	StatusVerified  Status = "local_target_verified"
	StatusAttention Status = "local_target_attention"
	StatusFailed    Status = "local_target_verification_failed"
	StatusUnhealthy Status = "local_target_unhealthy"
)

type CompletenessStatus string

const (
	CompletenessNoPublishedSession CompletenessStatus = "no_published_session"
	CompletenessVerified           CompletenessStatus = "verified_at_report_time"
	CompletenessNeedsAttention     CompletenessStatus = "needs_attention"
	CompletenessFailed             CompletenessStatus = "verification_failed"
)

type Options struct {
	TargetRoot string
	ProfileID  string
	TargetID   string
	SessionID  string
}

type Report struct {
	Scope                string               `json:"scope"`
	TargetRoot           string               `json:"target_root"`
	ProfileID            string               `json:"profile_id,omitempty"`
	TargetID             string               `json:"target_id,omitempty"`
	SessionID            string               `json:"session_id,omitempty"`
	Overall              Overall              `json:"overall"`
	Summary              Summary              `json:"summary"`
	LatestSession        LatestSession        `json:"latest_session"`
	Warnings             []control.Warning    `json:"warnings,omitempty"`
	ProfileSuggestions   []ProfileSuggestion  `json:"profile_suggestions,omitempty"`
	SoftDeletes          []control.SoftDelete `json:"soft_deletes,omitempty"`
	Health               Health               `json:"health"`
	ArtifactProblems     []ArtifactProblem    `json:"artifact_problems,omitempty"`
	ProfileSnapshots     []ProfileSnapshot    `json:"profile_snapshots,omitempty"`
	VerificationFindings []verify.Finding     `json:"verification_findings,omitempty"`
}

type Overall struct {
	Scope  string   `json:"scope"`
	Status Status   `json:"status"`
	Issues []string `json:"issues,omitempty"`
}

type Summary struct {
	ManifestCount        int `json:"manifest_count"`
	ManifestEntries      int `json:"manifest_entries"`
	FilesExpected        int `json:"files_expected"`
	FilesVerified        int `json:"files_verified"`
	VerificationErrors   int `json:"verification_errors"`
	VerificationWarnings int `json:"verification_warnings"`
	Warnings             int `json:"warnings"`
	ProfileSuggestions   int `json:"profile_suggestions"`
	SoftDeletes          int `json:"soft_deletes"`
	RecoveryIssues       int `json:"recovery_issues"`
	InvalidHealthRecords int `json:"invalid_health_records"`
	ArtifactProblems     int `json:"artifact_problems"`
}

type LatestSession struct {
	ID           string       `json:"id,omitempty"`
	ManifestID   string       `json:"manifest_id,omitempty"`
	CreatedAt    string       `json:"created_at,omitempty"`
	Entries      int          `json:"entries"`
	Files        int          `json:"files"`
	Completeness Completeness `json:"completeness"`
}

type Completeness struct {
	Status               CompletenessStatus `json:"status"`
	FilesExpected        int                `json:"files_expected"`
	FilesVerified        int                `json:"files_verified"`
	VerificationErrors   int                `json:"verification_errors"`
	VerificationWarnings int                `json:"verification_warnings"`
}

type ProfileSuggestion struct {
	WarningID             string            `json:"warning_id"`
	SessionID             string            `json:"session_id,omitempty"`
	Code                  string            `json:"code"`
	Severity              string            `json:"severity,omitempty"`
	Message               string            `json:"message"`
	Paths                 []string          `json:"paths,omitempty"`
	TargetPath            string            `json:"target_path,omitempty"`
	SuggestedProfilePatch map[string]string `json:"suggested_profile_patch,omitempty"`
	SuggestedConfig       map[string]string `json:"suggested_config,omitempty"`
	CreatedAt             string            `json:"created_at,omitempty"`
}

type Health struct {
	Healthy        bool              `json:"healthy"`
	Summary        HealthSummary     `json:"summary"`
	RecoveryIssues []RecoveryIssue   `json:"recovery_issues,omitempty"`
	InvalidRecords []InvalidRecord   `json:"invalid_records,omitempty"`
	ArtifactIssues []ArtifactProblem `json:"artifact_issues,omitempty"`
}

type HealthSummary struct {
	IncompleteSessions int `json:"incomplete_sessions"`
	InvalidRecords     int `json:"invalid_records"`
	ArtifactProblems   int `json:"artifact_problems"`
}

type RecoveryIssue struct {
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

type ArtifactProblem struct {
	Source    string `json:"source,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}

type ProfileSnapshot struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id,omitempty"`
	ProfileID  string `json:"profile_id"`
	CapturedAt string `json:"captured_at"`
	Path       string `json:"path"`
}

func BuildReport(opts Options) (Report, error) {
	targetRoot := strings.TrimSpace(opts.TargetRoot)
	if targetRoot == "" {
		return Report{}, errors.New("target root is required")
	}
	absRoot, err := filepath.Abs(targetRoot)
	if err != nil {
		return Report{}, err
	}

	healthReport, err := health.BuildReport(health.Options{TargetRoot: absRoot})
	if err != nil {
		return Report{}, err
	}
	verifyReport, err := verify.BuildReport(verify.Options{
		TargetRoot: absRoot,
		SessionID:  opts.SessionID,
		ProfileID:  opts.ProfileID,
		TargetID:   opts.TargetID,
	})
	if err != nil {
		return Report{}, err
	}

	filterSessionID := opts.SessionID
	displaySessionID := opts.SessionID
	if displaySessionID == "" {
		displaySessionID = verifyReport.Manifest.SessionID
	}

	healthView := summarizeHealth(healthReport, filterSessionID)
	snapshots, snapshotProblems := readProfileSnapshots(absRoot, verifyReport.Manifests, opts.ProfileID, opts.TargetID, filterSessionID)
	scopeProblems := readForeignPublishedReceipts(absRoot, opts.ProfileID, opts.TargetID, opts.SessionID, verifyReport.Summary.ManifestCount)
	artifactProblems := mergeArtifactProblems(filterSessionID, healthView.ArtifactIssues, verifyReport.ArtifactProblems, snapshotProblems, scopeProblems)
	profileSuggestions := collectProfileSuggestions(verifyReport.Warnings)

	report := Report{
		Scope:                ScopeLocalMigrationTarget,
		TargetRoot:           verifyReport.TargetRoot,
		ProfileID:            opts.ProfileID,
		TargetID:             opts.TargetID,
		SessionID:            displaySessionID,
		Overall:              Overall{Scope: ScopeLocalMigrationTarget},
		Summary:              summarizeVerify(verifyReport, healthView, len(profileSuggestions), len(artifactProblems)),
		LatestSession:        summarizeLatestSession(verifyReport),
		Warnings:             copyWarnings(verifyReport.Warnings),
		ProfileSuggestions:   profileSuggestions,
		SoftDeletes:          append([]control.SoftDelete(nil), verifyReport.SoftDeletes...),
		Health:               healthView,
		ArtifactProblems:     artifactProblems,
		ProfileSnapshots:     snapshots,
		VerificationFindings: append([]verify.Finding(nil), verifyReport.Findings...),
	}
	report.Overall.Status = classify(report)
	report.Overall.Issues = summarizeIssues(report)
	return report, nil
}

func summarizeVerify(report verify.Report, health Health, suggestionCount int, artifactProblems int) Summary {
	return Summary{
		ManifestCount:        report.Summary.ManifestCount,
		ManifestEntries:      report.Summary.ManifestEntries,
		FilesExpected:        report.Summary.FilesExpected,
		FilesVerified:        report.Summary.FilesVerified,
		VerificationErrors:   report.Summary.ErrorFindings,
		VerificationWarnings: report.Summary.WarningFindings,
		Warnings:             report.Summary.Warnings,
		ProfileSuggestions:   suggestionCount,
		SoftDeletes:          report.Summary.SoftDeletes,
		RecoveryIssues:       len(health.RecoveryIssues),
		InvalidHealthRecords: len(health.InvalidRecords),
		ArtifactProblems:     artifactProblems,
	}
}

func summarizeLatestSession(report verify.Report) LatestSession {
	latest := LatestSession{
		ID:         report.Manifest.SessionID,
		ManifestID: report.Manifest.ID,
		CreatedAt:  report.Manifest.CreatedAt,
		Entries:    report.Manifest.Entries,
		Files:      report.Manifest.Files,
		Completeness: Completeness{
			FilesExpected:        report.Summary.FilesExpected,
			FilesVerified:        report.Summary.FilesVerified,
			VerificationErrors:   report.Summary.ErrorFindings,
			VerificationWarnings: report.Summary.WarningFindings,
		},
	}
	switch {
	case latest.ID == "":
		latest.Completeness.Status = CompletenessNoPublishedSession
	case report.Summary.ErrorFindings > 0:
		latest.Completeness.Status = CompletenessFailed
	case report.Summary.WarningFindings > 0:
		latest.Completeness.Status = CompletenessNeedsAttention
	default:
		latest.Completeness.Status = CompletenessVerified
	}
	return latest
}

func summarizeHealth(report health.Report, sessionID string) Health {
	out := Health{
		Summary: HealthSummary{
			ArtifactProblems: report.Summary.ArtifactProblems,
		},
	}
	for _, item := range report.Items {
		if sessionID != "" && item.SessionID != sessionID {
			continue
		}
		out.RecoveryIssues = append(out.RecoveryIssues, RecoveryIssue{
			SessionID: item.SessionID,
			State:     item.State,
			Action:    item.Action,
			Reason:    item.Reason,
			Path:      filepath.ToSlash(item.Path),
			UpdatedAt: item.UpdatedAt,
		})
	}
	for _, invalid := range report.Invalid {
		if sessionID != "" && invalid.SessionID != sessionID {
			continue
		}
		out.InvalidRecords = append(out.InvalidRecords, InvalidRecord{
			SessionID: invalid.SessionID,
			Path:      filepath.ToSlash(invalid.Path),
			Error:     invalid.Error,
		})
	}
	for _, issue := range report.Artifacts {
		if sessionID != "" && issue.SessionID != "" && issue.SessionID != sessionID {
			continue
		}
		out.ArtifactIssues = append(out.ArtifactIssues, ArtifactProblem{
			Source:    "health",
			SessionID: issue.SessionID,
			Path:      filepath.ToSlash(issue.Path),
			Error:     issue.Error,
		})
	}
	out.Summary.IncompleteSessions = len(out.RecoveryIssues)
	out.Summary.InvalidRecords = len(out.InvalidRecords)
	out.Summary.ArtifactProblems = len(out.ArtifactIssues)
	out.Healthy = out.Summary.IncompleteSessions == 0 && out.Summary.InvalidRecords == 0 && out.Summary.ArtifactProblems == 0
	return out
}

func readProfileSnapshots(targetRoot string, manifests []verify.ManifestSummary, profileID string, targetID string, sessionID string) ([]ProfileSnapshot, []ArtifactProblem) {
	var snapshots []ProfileSnapshot
	var problems []ArtifactProblem
	for _, manifest := range manifests {
		if sessionID != "" && manifest.SessionID != sessionID {
			continue
		}
		id := "profile-" + manifest.SessionID
		path, err := control.Path(targetRoot, control.ArtifactProfileSnapshot, id)
		if err != nil {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Error: err.Error()})
			continue
		}
		snapshot, err := control.ReadFile[control.ProfileSnapshot](path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				err = fmt.Errorf("profile snapshot %q is missing", id)
			}
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: err.Error()})
			continue
		}
		if snapshot.ID != id {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: fmt.Sprintf("profile snapshot id %q does not match expected %q", snapshot.ID, id)})
			continue
		}
		if snapshot.SessionID != manifest.SessionID {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: fmt.Sprintf("profile snapshot session_id %q does not match manifest session %q", snapshot.SessionID, manifest.SessionID)})
			continue
		}
		if strings.TrimSpace(profileID) != "" && snapshot.ProfileID != profileID {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: fmt.Sprintf("profile snapshot profile_id %q does not match requested profile %q", snapshot.ProfileID, profileID)})
			continue
		}
		embedded, err := decodeProfileSnapshot(snapshot, profileID, targetID, manifest.RootID)
		if err != nil {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: err.Error()})
			continue
		}
		snapshots = append(snapshots, ProfileSnapshot{
			ID:         snapshot.ID,
			SessionID:  snapshot.SessionID,
			ProfileID:  embedded.ProfileID,
			CapturedAt: snapshot.CapturedAt,
			Path:       filepath.ToSlash(path),
		})
	}
	return snapshots, problems
}

type snapshotProfile struct {
	ProfileID string `json:"profile_id"`
	Roots     []struct {
		ID string `json:"id"`
	} `json:"roots"`
	Target struct {
		TargetID string `json:"target_id"`
	} `json:"target"`
}

func decodeProfileSnapshot(snapshot control.ProfileSnapshot, expectedProfileID string, expectedTargetID string, manifestRootID string) (snapshotProfile, error) {
	var embedded snapshotProfile
	if err := json.Unmarshal(snapshot.Profile, &embedded); err != nil {
		return embedded, fmt.Errorf("decode embedded profile snapshot: %w", err)
	}
	if embedded.ProfileID != snapshot.ProfileID {
		return embedded, fmt.Errorf("embedded profile_id %q does not match snapshot profile_id %q", embedded.ProfileID, snapshot.ProfileID)
	}
	if strings.TrimSpace(expectedProfileID) != "" && embedded.ProfileID != expectedProfileID {
		return embedded, fmt.Errorf("embedded profile_id %q does not match requested profile %q", embedded.ProfileID, expectedProfileID)
	}
	if strings.TrimSpace(embedded.Target.TargetID) == "" {
		return embedded, errors.New("embedded target.target_id is required")
	}
	if strings.TrimSpace(expectedTargetID) != "" && embedded.Target.TargetID != expectedTargetID {
		return embedded, fmt.Errorf("embedded target_id %q does not match requested target %q", embedded.Target.TargetID, expectedTargetID)
	}
	if strings.TrimSpace(manifestRootID) != "" && !snapshotContainsRoot(embedded.Roots, manifestRootID) {
		return embedded, fmt.Errorf("manifest root_id %q is not present in embedded profile snapshot", manifestRootID)
	}
	return embedded, nil
}

func snapshotContainsRoot(roots []struct {
	ID string `json:"id"`
}, rootID string) bool {
	for _, root := range roots {
		if root.ID == rootID {
			return true
		}
	}
	return false
}

func readForeignPublishedReceipts(targetRoot string, profileID string, targetID string, sessionID string, manifestCount int) []ArtifactProblem {
	if sessionID != "" || manifestCount > 0 || (strings.TrimSpace(profileID) == "" && strings.TrimSpace(targetID) == "") {
		return nil
	}
	sessionsDir := filepath.Join(control.ControlDir(targetRoot), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return []ArtifactProblem{{Source: "scope", Path: sessionsDir, Error: err.Error()}}
	}

	var problems []ArtifactProblem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		receiptPath := filepath.Join(sessionsDir, entry.Name(), "receipt.json")
		receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
		if err != nil || receipt.Status != "published" || receipt.ID != entry.Name() {
			continue
		}
		if strings.TrimSpace(profileID) != "" && receipt.ProfileID == profileID && strings.TrimSpace(targetID) != "" && receipt.TargetID == targetID {
			continue
		}
		if strings.TrimSpace(profileID) != "" && receipt.ProfileID == profileID && strings.TrimSpace(targetID) == "" {
			continue
		}
		if strings.TrimSpace(targetID) != "" && receipt.TargetID == targetID && strings.TrimSpace(profileID) == "" {
			continue
		}
		problems = append(problems, ArtifactProblem{
			Source:    "scope",
			SessionID: receipt.ID,
			Path:      receiptPath,
			Error:     fmt.Sprintf("published session belongs to profile_id/target_id (%q/%q), not requested profile_id/target_id (%q/%q)", receipt.ProfileID, receipt.TargetID, profileID, targetID),
		})
	}
	return problems
}

func collectProfileSuggestions(warnings []control.Warning) []ProfileSuggestion {
	var suggestions []ProfileSuggestion
	for _, warning := range warnings {
		if len(warning.SuggestedProfilePatch) == 0 && len(warning.SuggestedConfig) == 0 {
			continue
		}
		suggestions = append(suggestions, ProfileSuggestion{
			WarningID:             warning.ID,
			SessionID:             warning.SessionID,
			Code:                  warning.Code,
			Severity:              warning.Severity,
			Message:               warning.Message,
			Paths:                 append([]string(nil), warning.Paths...),
			TargetPath:            warning.TargetPath,
			SuggestedProfilePatch: copyStringMap(warning.SuggestedProfilePatch),
			SuggestedConfig:       copyStringMap(warning.SuggestedConfig),
			CreatedAt:             warning.CreatedAt,
		})
	}
	return suggestions
}

func mergeArtifactProblems(sessionID string, healthIssues []ArtifactProblem, verifyProblems []verify.ArtifactProblem, extraProblemSets ...[]ArtifactProblem) []ArtifactProblem {
	var out []ArtifactProblem
	seen := map[string]struct{}{}
	add := func(problem ArtifactProblem) {
		if sessionID != "" && problem.SessionID != "" && problem.SessionID != sessionID {
			return
		}
		problem.Path = filepath.ToSlash(problem.Path)
		key := problem.Source + "\x00" + problem.SessionID + "\x00" + problem.Error
		if problem.Path != "" {
			key = "path\x00" + problem.Path
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, problem)
	}
	for _, issue := range healthIssues {
		add(issue)
	}
	for _, problem := range verifyProblems {
		add(ArtifactProblem{
			Source:    "verify",
			SessionID: problem.SessionID,
			Path:      problem.Path,
			Error:     problem.Err,
		})
	}
	for _, problems := range extraProblemSets {
		for _, problem := range problems {
			add(problem)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			if out[i].SessionID == out[j].SessionID {
				return out[i].Error < out[j].Error
			}
			return out[i].SessionID < out[j].SessionID
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func classify(report Report) Status {
	switch {
	case !report.Health.Healthy || report.Summary.ArtifactProblems > 0:
		return StatusUnhealthy
	case report.Summary.ManifestCount == 0:
		return StatusEmpty
	case report.Summary.VerificationErrors > 0:
		return StatusFailed
	case report.Summary.VerificationWarnings > 0 || report.Summary.Warnings > 0 || report.Summary.SoftDeletes > 0:
		return StatusAttention
	default:
		return StatusVerified
	}
}

func (r Report) NeedsReview() bool {
	return r.Overall.Status != StatusVerified
}

func summarizeIssues(report Report) []string {
	var issues []string
	if report.Summary.ArtifactProblems > 0 {
		issues = append(issues, "artifact_problems")
	}
	if report.Summary.RecoveryIssues > 0 {
		issues = append(issues, "recovery_issues")
	}
	if report.Summary.InvalidHealthRecords > 0 {
		issues = append(issues, "invalid_health_records")
	}
	if report.Summary.VerificationErrors > 0 {
		issues = append(issues, "verification_errors")
	}
	if report.Summary.VerificationWarnings > 0 {
		issues = append(issues, "verification_warnings")
	}
	if report.Summary.Warnings > 0 {
		issues = append(issues, "warnings")
	}
	if report.Summary.ProfileSuggestions > 0 {
		issues = append(issues, "profile_suggestions")
	}
	if report.Summary.SoftDeletes > 0 {
		issues = append(issues, "soft_deletes")
	}
	return issues
}

func copyWarnings(warnings []control.Warning) []control.Warning {
	out := append([]control.Warning(nil), warnings...)
	for i := range out {
		out[i].Paths = append([]string(nil), out[i].Paths...)
		out[i].Detected = copyStringMap(out[i].Detected)
		out[i].SuggestedProfilePatch = copyStringMap(out[i].SuggestedProfilePatch)
		out[i].SuggestedConfig = copyStringMap(out[i].SuggestedConfig)
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
