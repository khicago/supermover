package prune

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/targetlock"
)

func TestPlanDryRunBuildsCandidateEvidenceWithoutMutatingTarget(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	before := snapshotPruneTree(t, target)

	report, err := PlanDryRun(validOptions(target))
	if err != nil {
		t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
	}

	if !report.NeedsReview() {
		t.Fatalf("PlanDryRun(%q).NeedsReview() = false, want true for candidate review", target)
	}
	if report.Schema != "supermover.prune_dry_run.v1" || !report.DryRun || !report.ApprovalRequired {
		t.Fatalf("PlanDryRun(%q) report metadata = %#v, want dry-run review metadata", target, report)
	}
	if report.ProfileDeletePolicy.Mode != "prune" || !report.ProfileDeletePolicy.RequireReview || !report.ProfileDeletePolicy.AllowPhysicalPrune || report.ProfileDeletePolicy.RetentionDays != 30 {
		t.Fatalf("PlanDryRun(%q) profile policy = %#v, want profile SSOT policy in report", target, report.ProfileDeletePolicy)
	}
	if report.Summary.SoftDeletes != 1 || report.Summary.Candidates != 1 || report.Summary.Refusals != 0 || report.Summary.ArtifactProblems != 0 {
		t.Fatalf("PlanDryRun(%q).Summary = %+v, want one candidate", target, report.Summary)
	}
	candidate := report.Candidates[0]
	if candidate.SoftDeleteID != record.ID || candidate.DetectedSessionID != "session-two" || candidate.PreviousSessionID != "session-one" || candidate.PreviousManifestID != "manifest-session-one" {
		t.Fatalf("candidate soft-delete/manifest evidence = %+v, want session and previous manifest evidence", candidate)
	}
	if candidate.ProfileID != "profile-local" || candidate.TargetID != "target-local" || candidate.RootID != "root" {
		t.Fatalf("candidate scope evidence = %+v, want profile/target/root", candidate)
	}
	if candidate.SourcePath != "gone.txt" || candidate.TargetPath != "gone.txt" || candidate.Kind != "file" || candidate.Size != 4 || candidate.Digest != digest([]byte("gone")) {
		t.Fatalf("candidate file evidence = %+v, want soft-delete path/size/digest", candidate)
	}
	if candidate.PreviousManifestEntry.SessionID != "session-one" || candidate.PreviousManifestEntry.ManifestID != "manifest-session-one" || candidate.PreviousManifestEntry.Digest != digest([]byte("gone")) {
		t.Fatalf("candidate previous manifest entry = %+v, want previous evidence", candidate.PreviousManifestEntry)
	}
	if candidate.CurrentTargetState.Present == nil || !*candidate.CurrentTargetState.Present || candidate.CurrentTargetState.Kind != "file" || candidate.CurrentTargetState.Digest != digest([]byte("gone")) {
		t.Fatalf("candidate current target state = %+v, want present matching file digest", candidate.CurrentTargetState)
	}
	if candidate.IntendedAction != "delete_file" || candidate.PhysicalPruning != "not_applied" || candidate.ApprovalWriting != "not_written_by_dry_run" || candidate.ReceiptWriting != "not_written_by_dry_run" || !candidate.ReviewRequired {
		t.Fatalf("candidate action fields = %+v, want review-only dry-run action", candidate)
	}
	after := snapshotPruneTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("PlanDryRun changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestPlanDryRunRefusesUnsafeTargetStates(t *testing.T) {
	tests := []struct {
		name        string
		prepare     func(t *testing.T, target string)
		wantReason  string
		wantKind    string
		wantPresent bool
	}{
		{
			name:        "missing target",
			prepare:     func(t *testing.T, target string) {},
			wantReason:  ReasonTargetMissing,
			wantKind:    "missing",
			wantPresent: false,
		},
		{
			name: "already-pruned target",
			prepare: func(t *testing.T, target string) {
				writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
				if err := os.Remove(filepath.Join(target, "gone.txt")); err != nil {
					t.Fatalf("Remove(already-pruned target) error = %v, want nil", err)
				}
			},
			wantReason:  ReasonTargetMissing,
			wantKind:    "missing",
			wantPresent: false,
		},
		{
			name: "divergent target",
			prepare: func(t *testing.T, target string) {
				writePruneTargetFile(t, target, "gone.txt", []byte("changed"))
			},
			wantReason:  ReasonTargetContentMismatch,
			wantKind:    "file",
			wantPresent: true,
		},
		{
			name: "symlink parent",
			prepare: func(t *testing.T, target string) {
				outside := filepath.Join(t.TempDir(), "outside")
				if err := os.MkdirAll(outside, 0o755); err != nil {
					t.Fatalf("MkdirAll(outside) error = %v, want nil", err)
				}
				if err := os.Symlink(outside, filepath.Join(target, "parent")); err != nil {
					t.Fatalf("Symlink(parent) error = %v, want nil", err)
				}
			},
			wantReason:  ReasonUnsafeTargetParent,
			wantKind:    "other",
			wantPresent: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			entry := fileEntry("gone.txt", []byte("gone"))
			if tt.name == "symlink parent" {
				entry.Path = "parent/gone.txt"
				entry.TargetPath = "parent/gone.txt"
			}
			writePruneSession(t, target, "session-one", []control.ManifestEntry{entry})
			record := softDelete("session-two", entry.TargetPath, []byte("gone"))
			record.SourcePath = entry.Path
			writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
			tt.prepare(t, target)
			before := snapshotPruneTree(t, target)

			report, err := PlanDryRun(validOptions(target))
			if err != nil {
				t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
			}

			if report.Summary.Candidates != 0 || report.Summary.Refusals != 1 {
				t.Fatalf("PlanDryRun(%q).Summary = %+v, want one refusal", target, report.Summary)
			}
			refusal := report.Refusals[0]
			if refusal.ReasonCode != tt.wantReason {
				t.Fatalf("refusal reason = %q, want %q: %+v", refusal.ReasonCode, tt.wantReason, refusal)
			}
			if refusal.CurrentTargetState.Present == nil || *refusal.CurrentTargetState.Present != tt.wantPresent || refusal.CurrentTargetState.Kind != tt.wantKind {
				t.Fatalf("refusal observed = %+v, want present=%t kind=%q", refusal.CurrentTargetState, tt.wantPresent, tt.wantKind)
			}
			after := snapshotPruneTree(t, target)
			if strings.Join(after, "\n") != strings.Join(before, "\n") {
				t.Fatalf("PlanDryRun changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
			}
		})
	}
}

func TestPlanDryRunHandlesSymlinkAndNestedReservedNameAsData(t *testing.T) {
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, "docs", ".supermover"), 0o755); err != nil {
		t.Fatalf("MkdirAll(nested reserved name) error = %v, want nil", err)
	}
	if err := os.Symlink("target.txt", filepath.Join(target, "docs", ".supermover", "link")); err != nil {
		t.Fatalf("Symlink(link) error = %v, want nil", err)
	}
	entry := control.ManifestEntry{
		Path:          "docs/.supermover/link",
		Kind:          "symlink",
		TargetPath:    "docs/.supermover/link",
		SymlinkTarget: "target.txt",
	}
	writePruneSession(t, target, "session-one", []control.ManifestEntry{entry})
	record := control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-two-del_link",
		SessionID:          "session-two",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "session-one",
		PreviousManifestID: "manifest-session-one",
		SourcePath:         "docs/.supermover/link",
		TargetPath:         "docs/.supermover/link",
		Kind:               "symlink",
		DetectedAt:         "2026-04-16T00:02:00Z",
		Reason:             "source_missing",
	}
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)

	report, err := PlanDryRun(validOptions(target))
	if err != nil {
		t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
	}

	if report.Summary.Candidates != 1 || report.Summary.Refusals != 0 || report.Summary.ArtifactProblems != 0 {
		t.Fatalf("PlanDryRun(%q).Summary = %+v, want symlink candidate", target, report.Summary)
	}
	candidate := report.Candidates[0]
	if candidate.TargetPath != "docs/.supermover/link" || candidate.Kind != "symlink" || candidate.IntendedAction != "delete_symlink" {
		t.Fatalf("symlink candidate = %+v, want nested .supermover symlink data candidate", candidate)
	}
	if candidate.CurrentTargetState.SymlinkTarget != "target.txt" {
		t.Fatalf("symlink observed = %+v, want symlink target evidence", candidate.CurrentTargetState)
	}
}

func TestPlanDryRunEnforcesRetentionWindow(t *testing.T) {
	tests := []struct {
		name          string
		retentionDays int
		now           time.Time
		wantCandidate bool
	}{
		{
			name:          "zero retention is immediately eligible",
			retentionDays: 0,
			now:           time.Date(2026, 5, 16, 0, 2, 0, 0, time.UTC),
			wantCandidate: true,
		},
		{
			name:          "active retention is refused",
			retentionDays: 30,
			now:           time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
			wantCandidate: false,
		},
		{
			name:          "just before boundary is refused",
			retentionDays: 30,
			now:           time.Date(2026, 6, 15, 0, 1, 59, int(time.Second-time.Nanosecond), time.UTC),
			wantCandidate: false,
		},
		{
			name:          "boundary is eligible",
			retentionDays: 30,
			now:           time.Date(2026, 6, 15, 0, 2, 0, 0, time.UTC),
			wantCandidate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
			record := softDelete("session-two", "gone.txt", []byte("gone"))
			record.DetectedAt = "2026-05-16T00:02:00Z"
			writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
			before := snapshotPruneTree(t, target)
			opts := validOptions(target)
			opts.DeletePolicy.RetentionDays = tt.retentionDays
			opts.Now = tt.now

			report, err := PlanDryRun(opts)
			if err != nil {
				t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
			}

			if tt.wantCandidate {
				if report.Summary.Candidates != 1 || report.Summary.Refusals != 0 {
					t.Fatalf("PlanDryRun(%q).Summary = %+v, want eligible candidate", target, report.Summary)
				}
				if report.Candidates[0].SoftDeleteID != record.ID {
					t.Fatalf("candidate = %+v, want soft-delete %q", report.Candidates[0], record.ID)
				}
			} else {
				if report.Summary.Candidates != 0 || report.Summary.Refusals != 1 {
					t.Fatalf("PlanDryRun(%q).Summary = %+v, want retention refusal", target, report.Summary)
				}
				refusal := report.Refusals[0]
				if refusal.ReasonCode != ReasonRetentionWindowActive || !strings.Contains(refusal.Message, "retention window active until") {
					t.Fatalf("refusal = %+v, want retention_window_active", refusal)
				}
				if refusal.CurrentTargetState.Present == nil || !*refusal.CurrentTargetState.Present || refusal.CurrentTargetState.Kind != "file" {
					t.Fatalf("refusal observed = %+v, want current target evidence", refusal.CurrentTargetState)
				}
			}
			after := snapshotPruneTree(t, target)
			if strings.Join(after, "\n") != strings.Join(before, "\n") {
				t.Fatalf("PlanDryRun changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
			}
		})
	}
}

func TestPlanDryRunRefusesInvalidRetentionEvidence(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	opts := validOptions(target)
	opts.DeletePolicy.RetentionDays = -1

	report, err := PlanDryRun(opts)
	if err != nil {
		t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
	}

	if report.Summary.Candidates != 0 || report.Summary.Refusals != 1 {
		t.Fatalf("PlanDryRun(%q).Summary = %+v, want retention refusal", target, report.Summary)
	}
	refusal := report.Refusals[0]
	if refusal.ReasonCode != ReasonRetentionWindowActive || !strings.Contains(refusal.Message, "retention_days cannot be negative") {
		t.Fatalf("refusal = %+v, want retention_window_active for negative retention", refusal)
	}
}

func TestPlanDryRunRefusesStaleSoftDeleteWhenLaterManifestContainsTarget(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	writePruneSession(t, target, "session-three", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	rewritePruneManifestCreatedAt(t, target, "session-three", "2026-05-17T00:00:00Z")
	writePruneSession(t, target, "session-four", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))})
	rewritePruneManifestCreatedAt(t, target, "session-four", "2026-05-18T00:00:00Z")

	report, err := PlanDryRun(validOptions(target))
	if err != nil {
		t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
	}

	if report.Summary.Candidates != 0 || report.Summary.Refusals != 1 {
		t.Fatalf("PlanDryRun(%q).Summary = %+v, want stale soft-delete refusal", target, report.Summary)
	}
	if report.Refusals[0].ReasonCode != ReasonTargetReappeared {
		t.Fatalf("refusal reason = %q, want %q: %+v", report.Refusals[0].ReasonCode, ReasonTargetReappeared, report.Refusals[0])
	}
}

func TestPlanDryRunRefusesWeakPreviousFileEvidence(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	entry := fileEntry("gone.txt", []byte("gone"))
	entry.Digest = ""
	writePruneSession(t, target, "session-one", []control.ManifestEntry{entry})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	record.Digest = ""
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)

	report, err := PlanDryRun(validOptions(target))
	if err != nil {
		t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
	}

	if report.Summary.Candidates != 0 || report.Summary.Refusals != 1 {
		t.Fatalf("PlanDryRun(%q).Summary = %+v, want weak evidence refusal", target, report.Summary)
	}
	if report.Refusals[0].ReasonCode != ReasonWeakPreviousEvidence {
		t.Fatalf("refusal reason = %q, want %q: %+v", report.Refusals[0].ReasonCode, ReasonWeakPreviousEvidence, report.Refusals[0])
	}
}

func TestPlanDryRunReportsPublishedSoftDeleteArtifactProblems(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "corrupt JSON", body: `{"version":1`, want: "unexpected EOF"},
		{name: "reserved target path", body: `{"version":1,"id":"session-two-del_reserved","session_id":"session-two","profile_id":"profile-local","target_id":"target-local","root_id":"root","previous_session_id":"session-one","previous_manifest_id":"manifest-session-one","source_path":"gone.txt","target_path":".supermover/gone.txt","kind":"file","detected_at":"2026-05-16T00:02:00Z"}`, want: "reserved control directory"},
		{name: "invalid detected_at", body: fmt.Sprintf(`{"version":%d,"id":"session-two-del_time","session_id":"session-two","profile_id":"profile-local","target_id":"target-local","root_id":"root","previous_session_id":"session-one","previous_manifest_id":"manifest-session-one","source_path":"gone.txt","target_path":"gone.txt","kind":"file","size":4,"digest":%q,"detected_at":"not-time","reason":"source_missing"}`, control.CurrentVersion, digest([]byte("gone"))), want: "detected_at must be RFC3339"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
			writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))})
			writeRawPruneArtifact(t, target, "deleted", "session-two-del_bad.json", tt.body)

			report, err := PlanDryRun(validOptions(target))
			if err != nil {
				t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
			}

			if report.Summary.SoftDeletes != 0 || report.Summary.Candidates != 0 || report.Summary.Refusals != 0 || report.Summary.ArtifactProblems != 1 {
				t.Fatalf("PlanDryRun(%q).Summary = %+v, want one artifact problem only", target, report.Summary)
			}
			if !strings.Contains(report.ArtifactProblems[0].Error, tt.want) {
				t.Fatalf("artifact problem = %+v, want error containing %q", report.ArtifactProblems[0], tt.want)
			}
		})
	}
}

func TestPlanDryRunRequiresExistingTargetControlPlane(t *testing.T) {
	target := t.TempDir()

	_, err := PlanDryRun(validOptions(target))

	if err == nil || !strings.Contains(err.Error(), "target control plane is missing") {
		t.Fatalf("PlanDryRun(%q) error = %v, want missing control-plane error", target, err)
	}
}

func TestPlanDryRunEmptyListIsReviewNeutral(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "keep.txt", []byte("keep"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))})

	report, err := PlanDryRun(validOptions(target))
	if err != nil {
		t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
	}

	if report.NeedsReview() {
		t.Fatalf("PlanDryRun(%q).NeedsReview() = true, want false for empty clean plan", target)
	}
	if report.Summary != (DryRunSummary{SoftDeletes: 0, Candidates: 0, Refusals: 0, ArtifactProblems: 0}) {
		t.Fatalf("PlanDryRun(%q).Summary = %+v, want empty summary", target, report.Summary)
	}
}

func TestAuthorApprovalWritesApprovalAndSnapshotWithoutMutatingTarget(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	before := snapshotPruneTree(t, target)

	result, err := AuthorApproval(validAuthorApprovalOptions(target, "approval-author", record.ID))
	if err != nil {
		t.Fatalf("AuthorApproval(file) error = %v, want nil", err)
	}

	after := snapshotPruneTree(t, target)
	if !samePruneTreeExcept(after, before, []string{
		filepath.ToSlash(filepath.Join(control.DirName, "locks")),
		filepath.ToSlash(filepath.Join(control.DirName, "locks", "target.lock")),
		filepath.ToSlash(filepath.Join(control.DirName, "profiles")),
		filepath.ToSlash(filepath.Join(control.DirName, "profiles", "profile-approval-author.json")),
		filepath.ToSlash(filepath.Join(control.DirName, "prune")),
		filepath.ToSlash(filepath.Join(control.DirName, "prune", "approvals")),
		filepath.ToSlash(filepath.Join(control.DirName, "prune", "approvals", "approval-author.json")),
	}) {
		t.Fatalf("AuthorApproval changed unexpected target paths\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
	if result.Schema != "supermover.prune_approval_authoring.v1" || result.ApprovalWriting != "written" || result.PhysicalPruning != "not_applied" || result.ReceiptWriting != "not_written_by_approval_authoring" {
		t.Fatalf("AuthorApproval result = %+v, want narrow authoring metadata", result)
	}
	if result.Approval.ID != "approval-author" || result.Approval.ProfileID != "profile-local" || result.Approval.TargetID != "target-local" || result.Approval.RootID != "root" {
		t.Fatalf("approval scope = %+v, want profile/target/root binding", result.Approval)
	}
	if result.Approval.ApprovedBy != "reviewer" || result.Approval.ApprovalReason != "reviewed stale target" || result.Approval.ReviewTool != "supermover prune approve" || result.Approval.Status != "approved" {
		t.Fatalf("approval review fields = %+v, want operator review evidence", result.Approval)
	}
	if len(result.Approval.Items) != 1 || result.Approval.Items[0] != approvalItem(record) {
		t.Fatalf("approval items = %+v, want soft-delete evidence item", result.Approval.Items)
	}
	if result.Approval.ApprovalScopeDigest != ApprovalScopeDigest(result.Approval.ProfileID, result.Approval.TargetID, result.Approval.RootID, result.Approval.ProfileSnapshotID, result.Approval.ProfileSnapshotDigest, validOptions(target).DeletePolicy, result.Approval.Items) {
		t.Fatalf("approval scope digest = %q, want recomputable digest", result.Approval.ApprovalScopeDigest)
	}
	approvalPath, err := control.Path(target, control.ArtifactPruneApproval, "approval-author")
	if err != nil {
		t.Fatalf("control.Path(approval) error = %v, want nil", err)
	}
	approval, err := control.ReadFile[control.PruneApproval](approvalPath)
	if err != nil {
		t.Fatalf("control.ReadFile(%q, approval) error = %v, want nil", approvalPath, err)
	}
	if approval.ApprovalScopeDigest != result.ApprovalScopeDigest {
		t.Fatalf("durable approval digest = %q, want %q", approval.ApprovalScopeDigest, result.ApprovalScopeDigest)
	}
	snapshotPath, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-approval-author")
	if err != nil {
		t.Fatalf("control.Path(snapshot) error = %v, want nil", err)
	}
	snapshot, err := control.ReadFile[control.ProfileSnapshot](snapshotPath)
	if err != nil {
		t.Fatalf("control.ReadFile(%q, snapshot) error = %v, want nil", snapshotPath, err)
	}
	snapshotDigest, err := ProfileSnapshotDigest(snapshot.Profile)
	if err != nil {
		t.Fatalf("ProfileSnapshotDigest(durable snapshot) error = %v, want nil", err)
	}
	if snapshotDigest != result.ProfileSnapshotDigest || snapshot.ProfileID != "profile-local" {
		t.Fatalf("profile snapshot = %+v digest=%s, want approval-bound snapshot", snapshot, snapshotDigest)
	}
	if _, err := os.Lstat(receiptPath(t, target, "approval-author")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(prune receipt) error = %v, want no receipt from authoring", err)
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target file after authoring) error = %v, want target data retained", err)
	}
}

func TestAuthorApprovalRejectsRefusalOrMissingSelectionWithoutWriting(t *testing.T) {
	tests := []struct {
		name     string
		selected string
		prepare  func(t *testing.T, target string)
		want     string
	}{
		{
			name:     "current refusal",
			selected: "session-two-del_gone",
			prepare: func(t *testing.T, target string) {
				mustWritePruneFile(t, filepath.Join(target, "gone.txt"), "changed")
			},
			want: "refusal",
		},
		{
			name:     "unknown selection",
			selected: "session-two-del_missing",
			prepare:  func(t *testing.T, target string) {},
			want:     ReasonCurrentCandidateMissing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
			record := softDelete("session-two", "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
			tt.prepare(t, target)
			before := snapshotPruneTree(t, target)

			_, err := AuthorApproval(validAuthorApprovalOptions(target, "approval-refuse", tt.selected))

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AuthorApproval(%s) error = %v, want containing %q", tt.name, err, tt.want)
			}
			after := snapshotPruneTree(t, target)
			if !samePruneTreeExcept(after, before, []string{
				filepath.ToSlash(filepath.Join(control.DirName, "locks")),
				filepath.ToSlash(filepath.Join(control.DirName, "locks", "target.lock")),
			}) {
				t.Fatalf("AuthorApproval(%s) wrote artifacts on refusal\nbefore:\n%s\nafter:\n%s", tt.name, strings.Join(before, "\n"), strings.Join(after, "\n"))
			}
		})
	}
}

func TestAuthorApprovalRejectsAnyDryRunRefusalBeforeSelection(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneTargetFile(t, target, "young.txt", []byte("young"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{
		fileEntry("gone.txt", []byte("gone")),
		fileEntry("young.txt", []byte("young")),
	})
	eligible := softDelete("session-two", "gone.txt", []byte("gone"))
	young := softDelete("session-two", "young.txt", []byte("young"))
	young.ID = "session-two-del_young"
	young.DetectedAt = "2026-05-16T00:02:00Z"
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, eligible, young)
	before := snapshotPruneTree(t, target)

	_, err := AuthorApproval(validAuthorApprovalOptions(target, "approval-any-refusal", eligible.ID))

	if err == nil || !strings.Contains(err.Error(), "dry-run produced 1 refusal") {
		t.Fatalf("AuthorApproval(unselected refusal) error = %v, want dry-run refusal", err)
	}
	after := snapshotPruneTree(t, target)
	if !samePruneTreeExcept(after, before, []string{
		filepath.ToSlash(filepath.Join(control.DirName, "locks")),
		filepath.ToSlash(filepath.Join(control.DirName, "locks", "target.lock")),
	}) {
		t.Fatalf("AuthorApproval(unselected refusal) wrote artifacts unexpectedly\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestAuthorApprovalRejectsExistingArtifactsNoReplace(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-exists", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)

	_, err := AuthorApproval(validAuthorApprovalOptions(target, "approval-exists", record.ID))

	if err == nil || !errors.Is(err, control.ErrArtifactExists) {
		t.Fatalf("AuthorApproval(existing approval) error = %v, want ErrArtifactExists", err)
	}
	durable, readErr := control.ReadFile[control.PruneApproval](approvalPath(t, target, "approval-exists"))
	if readErr != nil {
		t.Fatalf("ReadFile(existing approval) error = %v, want nil", readErr)
	}
	if durable.ReviewTool != "unit-test" || durable.ApprovalReason != "reviewed" {
		t.Fatalf("existing approval = %+v, want original bytes preserved", durable)
	}
}

func TestAuthorApprovalRejectsInvalidProfileBindingBeforeWriting(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(opts *AuthorApprovalOptions)
		want   string
	}{
		{
			name: "blank approval id",
			mutate: func(opts *AuthorApprovalOptions) {
				opts.ApprovalID = ""
			},
			want: "approval id is required",
		},
		{
			name: "blank soft delete id",
			mutate: func(opts *AuthorApprovalOptions) {
				opts.SoftDeleteIDs = []string{""}
			},
			want: "soft-delete id is required",
		},
		{
			name: "payload mismatch",
			mutate: func(opts *AuthorApprovalOptions) {
				p := opts.Profile
				p.Roots = append([]profile.Root(nil), p.Roots...)
				p.Roots[0].Path = filepath.Join("different", "source")
				opts.ProfilePayload = pruneProfilePayload(p, "approval-mismatch")
			},
			want: "profile payload does not match",
		},
		{
			name: "missing profile root",
			mutate: func(opts *AuthorApprovalOptions) {
				opts.Profile.Roots = []profile.Root{{ID: "other-root", Path: filepath.Join("target", "source")}}
				opts.ProfilePayload = pruneProfilePayload(opts.Profile, "approval-missing-root")
			},
			want: "approval root_id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
			record := softDelete("session-two", "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
			opts := validAuthorApprovalOptions(target, "approval-invalid-binding", record.ID)
			tt.mutate(&opts)
			before := snapshotPruneTree(t, target)

			_, err := AuthorApproval(opts)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AuthorApproval(%s) error = %v, want containing %q", tt.name, err, tt.want)
			}
			after := snapshotPruneTree(t, target)
			if !samePruneTreeExcept(after, before, []string{
				filepath.ToSlash(filepath.Join(control.DirName, "locks")),
				filepath.ToSlash(filepath.Join(control.DirName, "locks", "target.lock")),
			}) {
				t.Fatalf("AuthorApproval(%s) wrote artifacts unexpectedly\nbefore:\n%s\nafter:\n%s", tt.name, strings.Join(before, "\n"), strings.Join(after, "\n"))
			}
		})
	}
}

func TestAuthorApprovalArtifactCanBeAppliedAndApplyStillRevalidates(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	if _, err := AuthorApproval(validAuthorApprovalOptions(target, "approval-apply", record.ID)); err != nil {
		t.Fatalf("AuthorApproval(file) error = %v, want nil", err)
	}
	mustWritePruneFile(t, filepath.Join(target, "gone.txt"), "changed")

	result, err := Apply(validApplyOptions(target, "approval-apply"))

	if err != nil {
		t.Fatalf("Apply(authored approval after target change) error = %v, want nil refusal receipt", err)
	}
	if result.Receipt.Status != control.PruneReceiptFailed || len(result.Receipt.Items) != 1 || result.Receipt.Items[0].Result != "refused" || result.Receipt.Items[0].ErrorCode != ReasonTargetContentMismatch {
		t.Fatalf("Apply(authored approval after target change) receipt = %+v, want fail-closed refusal", result.Receipt)
	}
	if got, err := os.ReadFile(filepath.Join(target, "gone.txt")); err != nil || string(got) != "changed" {
		t.Fatalf("ReadFile(target after refused apply) = %q, %v; want changed file retained", string(got), err)
	}
}

func TestApplyDeletesApprovedFileAndWritesReceipt(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-file", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)

	result, err := Apply(validApplyOptions(target, "approval-file"))
	if err != nil {
		t.Fatalf("Apply(file) error = %v, want nil", err)
	}

	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(pruned file) error = %v, want not exist", err)
	}
	if result.ExistingReceipt {
		t.Fatalf("Apply(file).ExistingReceipt = true, want false")
	}
	if result.Receipt.Status != control.PruneReceiptApplied || result.Receipt.ApprovalID != "approval-file" || result.Receipt.PruneSessionID != "prune-apply" {
		t.Fatalf("receipt metadata = %+v, want applied prune receipt", result.Receipt)
	}
	if len(result.Receipt.Items) != 1 {
		t.Fatalf("receipt items = %d, want 1", len(result.Receipt.Items))
	}
	item := result.Receipt.Items[0]
	if item.Result != "pruned" || item.ErrorCode != "" || item.PrunedAt == "" {
		t.Fatalf("receipt item = %+v, want pruned result", item)
	}
	if item.PrePruneObserved.Present == nil || !*item.PrePruneObserved.Present || item.PrePruneObserved.Kind != "file" || item.PrePruneObserved.Digest != digest([]byte("gone")) {
		t.Fatalf("receipt pre-prune evidence = %+v, want observed file", item.PrePruneObserved)
	}
	receipt, err := control.ReadFile[control.PruneReceipt](receiptPath(t, target, "prune-apply"))
	if err != nil {
		t.Fatalf("ReadFile(prune receipt) error = %v, want nil", err)
	}
	if receipt.ID != result.Receipt.ID || receipt.Items[0].Result != "pruned" {
		t.Fatalf("durable receipt = %+v, want returned receipt", receipt)
	}
}

func TestApplyRejectsApprovalBindingMismatches(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-mismatch", []control.PruneApprovalItem{approvalItem(record)})
	approval.TargetID = "other-target"
	approval.ApprovalScopeDigest = ApprovalScopeDigest(approval.ProfileID, approval.TargetID, approval.RootID, approval.ProfileSnapshotID, approval.ProfileSnapshotDigest, validOptions(target).DeletePolicy, approval.Items)
	writePruneApproval(t, target, approval)

	_, err := Apply(validApplyOptions(target, "approval-mismatch"))
	if err == nil || !strings.Contains(err.Error(), ReasonApprovalScopeMismatch) {
		t.Fatalf("Apply(mismatched approval) error = %v, want scope mismatch", err)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "gone.txt")); statErr != nil {
		t.Fatalf("Lstat(target file after rejected approval) error = %v, want nil", statErr)
	}
	if _, err := os.Lstat(receiptPath(t, target, "prune-apply")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(receipt after rejected approval) error = %v, want not exist", err)
	}
}

func TestApplyRefusesTargetDriftBetweenApprovalAndApply(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("changed"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-drift", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)

	result, err := Apply(validApplyOptions(target, "approval-drift"))
	if err != nil {
		t.Fatalf("Apply(drift) error = %v, want nil receipt with refusal", err)
	}

	if got, err := os.ReadFile(filepath.Join(target, "gone.txt")); err != nil || string(got) != "changed" {
		t.Fatalf("ReadFile(drifted target) = (%q, %v), want changed content", string(got), err)
	}
	if result.Receipt.Status != control.PruneReceiptFailed || len(result.Receipt.Items) != 1 {
		t.Fatalf("receipt = %+v, want failed single-item receipt", result.Receipt)
	}
	item := result.Receipt.Items[0]
	if item.Result != "refused" || item.ErrorCode != ReasonTargetContentMismatch {
		t.Fatalf("receipt item = %+v, want content mismatch refusal", item)
	}
	if len(result.Receipt.Refusals) != 1 || result.Receipt.Refusals[0].ReasonCode != ReasonTargetContentMismatch {
		t.Fatalf("receipt refusals = %+v, want content mismatch refusal", result.Receipt.Refusals)
	}
}

func TestApplyRefusesApprovedItemBeforeRetentionWindow(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	record.DetectedAt = "2026-05-16T00:02:00Z"
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-retention", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)

	result, err := Apply(validApplyOptions(target, "approval-retention"))
	if err != nil {
		t.Fatalf("Apply(retention active) error = %v, want nil failed receipt", err)
	}

	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target after retention refusal) error = %v, want target retained", err)
	}
	if result.Receipt.Status != control.PruneReceiptFailed || len(result.Receipt.Items) != 1 {
		t.Fatalf("receipt = %+v, want failed single-item receipt", result.Receipt)
	}
	item := result.Receipt.Items[0]
	if item.Result != "refused" || item.ErrorCode != ReasonRetentionWindowActive {
		t.Fatalf("receipt item = %+v, want retention refusal", item)
	}
	if len(result.Receipt.Refusals) != 1 || result.Receipt.Refusals[0].ReasonCode != ReasonRetentionWindowActive {
		t.Fatalf("receipt refusals = %+v, want retention refusal", result.Receipt.Refusals)
	}
}

func TestApplyDeletesApprovedSymlink(t *testing.T) {
	target := t.TempDir()
	if err := os.Symlink("target.txt", filepath.Join(target, "gone-link")); err != nil {
		t.Fatalf("Symlink(gone-link) error = %v, want nil", err)
	}
	entry := symlinkEntry("gone-link", "target.txt")
	writePruneSession(t, target, "session-one", []control.ManifestEntry{entry})
	record := symlinkSoftDelete("session-two", "gone-link")
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-link", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)

	result, err := Apply(validApplyOptions(target, "approval-link"))
	if err != nil {
		t.Fatalf("Apply(symlink) error = %v, want nil", err)
	}

	if _, err := os.Lstat(filepath.Join(target, "gone-link")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(pruned symlink) error = %v, want not exist", err)
	}
	if result.Receipt.Status != control.PruneReceiptApplied || result.Receipt.Items[0].PrePruneObserved.Kind != "symlink" || result.Receipt.Items[0].PrePruneObserved.SymlinkTarget != "target.txt" {
		t.Fatalf("receipt = %+v, want applied symlink evidence", result.Receipt)
	}
}

func TestApplyRefusesSymlinkApprovalTargetMismatch(t *testing.T) {
	target := t.TempDir()
	if err := os.Symlink("target.txt", filepath.Join(target, "gone-link")); err != nil {
		t.Fatalf("Symlink(gone-link) error = %v, want nil", err)
	}
	entry := symlinkEntry("gone-link", "target.txt")
	writePruneSession(t, target, "session-one", []control.ManifestEntry{entry})
	record := symlinkSoftDelete("session-two", "gone-link")
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	item := approvalItem(record)
	item.SymlinkTarget = "other.txt"
	approval := approvalForItems("approval-link-mismatch", []control.PruneApprovalItem{item})
	writePruneApproval(t, target, approval)

	result, err := Apply(validApplyOptions(target, "approval-link-mismatch"))
	if err != nil {
		t.Fatalf("Apply(symlink target mismatch) error = %v, want nil failed receipt", err)
	}

	if _, err := os.Lstat(filepath.Join(target, "gone-link")); err != nil {
		t.Fatalf("Lstat(gone-link after refused apply) error = %v, want nil", err)
	}
	if result.Receipt.Status != control.PruneReceiptFailed || result.Receipt.Items[0].Result != "refused" || result.Receipt.Items[0].ErrorCode != ReasonApprovalItemMismatch {
		t.Fatalf("receipt = %+v, want approval-item mismatch refusal", result.Receipt)
	}
}

func TestApplyRefusesUnsupportedDirectory(t *testing.T) {
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, "gone-dir"), 0o755); err != nil {
		t.Fatalf("Mkdir(gone-dir) error = %v, want nil", err)
	}
	entry := dirEntry("gone-dir")
	writePruneSession(t, target, "session-one", []control.ManifestEntry{entry})
	record := dirSoftDelete("session-two", "gone-dir")
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-dir", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)

	result, err := Apply(validApplyOptions(target, "approval-dir"))
	if err != nil {
		t.Fatalf("Apply(directory) error = %v, want nil refused receipt", err)
	}

	if info, err := os.Lstat(filepath.Join(target, "gone-dir")); err != nil || !info.IsDir() {
		t.Fatalf("Lstat(directory after refused apply) = (%v, %v), want directory", info, err)
	}
	if result.Receipt.Status != control.PruneReceiptFailed || result.Receipt.Items[0].Result != "refused" || result.Receipt.Items[0].ErrorCode != ReasonUnsupportedKind {
		t.Fatalf("receipt = %+v, want unsupported-kind refusal", result.Receipt)
	}
}

func TestApplyReturnsExistingReceiptWithoutDeletingAgain(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-idempotent", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)
	opts := validApplyOptions(target, "approval-idempotent")

	first, err := Apply(opts)
	if err != nil {
		t.Fatalf("Apply(first) error = %v, want nil", err)
	}
	writePruneTargetFile(t, target, "gone.txt", []byte("reappeared"))
	second, err := Apply(opts)
	if err != nil {
		t.Fatalf("Apply(second) error = %v, want nil existing receipt", err)
	}

	if !second.ExistingReceipt {
		t.Fatalf("Apply(second).ExistingReceipt = false, want true")
	}
	if second.Receipt.ID != first.Receipt.ID || second.Receipt.Items[0].Result != "pruned" {
		t.Fatalf("second receipt = %+v, want first receipt", second.Receipt)
	}
	if got, err := os.ReadFile(filepath.Join(target, "gone.txt")); err != nil || string(got) != "reappeared" {
		t.Fatalf("ReadFile(reappeared target) = (%q, %v), want untouched reappeared file", string(got), err)
	}
}

func TestApplyLeavesStartedReceiptWhenRemoveFails(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-started", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)
	previous := beforeRemoveApprovedTarget
	beforeRemoveApprovedTarget = func(targetRoot, targetRel string) error {
		return errors.New("injected remove failure")
	}
	t.Cleanup(func() { beforeRemoveApprovedTarget = previous })

	result, err := Apply(validApplyOptions(target, "approval-started"))
	if err != nil {
		t.Fatalf("Apply(remove failure) error = %v, want failed receipt", err)
	}

	if result.Receipt.Status != control.PruneReceiptFailed || result.Receipt.Items[0].Result != "failed" {
		t.Fatalf("receipt = %+v, want failed final receipt", result.Receipt)
	}
	receipt, err := control.ReadFile[control.PruneReceipt](receiptPath(t, target, "prune-apply"))
	if err != nil {
		t.Fatalf("ReadFile(receipt) error = %v, want nil", err)
	}
	if receipt.Status != control.PruneReceiptFailed || receipt.Items[0].ErrorCode != ReasonRemoveFailed {
		t.Fatalf("durable receipt = %+v, want failed final receipt", receipt)
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target after failed remove) error = %v, want nil", err)
	}
}

func TestApplyFailsClosedWhenTargetRootChangesBeforeRemove(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir(target) error = %v, want nil", err)
	}
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-root-swap", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("Mkdir(outside) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "gone.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside/gone.txt) error = %v, want nil", err)
	}
	movedTarget := filepath.Join(parent, "target-moved")
	previous := beforeRemoveApprovedTarget
	beforeRemoveApprovedTarget = func(targetRoot, targetRel string) error {
		if err := os.Rename(targetRoot, movedTarget); err != nil {
			return err
		}
		return os.Symlink(outside, targetRoot)
	}
	t.Cleanup(func() { beforeRemoveApprovedTarget = previous })

	_, err := Apply(validApplyOptions(target, "approval-root-swap"))
	if err == nil || !strings.Contains(err.Error(), "write final prune receipt") {
		t.Fatalf("Apply(root swap) error = %v, want fail-closed final receipt error", err)
	}

	started, readErr := control.ReadFile[control.PruneReceipt](filepath.Join(movedTarget, control.DirName, "prune", "receipts", "prune-apply.json"))
	if readErr != nil {
		t.Fatalf("ReadFile(started receipt in moved target) error = %v, want nil", readErr)
	}
	if started.Status != control.PruneReceiptStarted {
		t.Fatalf("started receipt = %+v, want started receipt preserved in original target root", started)
	}
	if got, err := os.ReadFile(filepath.Join(movedTarget, "gone.txt")); err != nil || string(got) != "gone" {
		t.Fatalf("ReadFile(moved original target) = (%q, %v), want original content", string(got), err)
	}
	if got, err := os.ReadFile(filepath.Join(outside, "gone.txt")); err != nil || string(got) != "outside" {
		t.Fatalf("ReadFile(outside target) = (%q, %v), want untouched outside content", string(got), err)
	}
}

func TestApplyHoldsSharedTargetLockDuringRemove(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-lock", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)
	acquired := make(chan struct{})
	releaseProbe := make(chan struct{})
	previous := beforeRemoveApprovedTarget
	beforeRemoveApprovedTarget = func(targetRoot, targetRel string) error {
		go func() {
			unlock, err := targetlock.LockTarget(targetRoot)
			if err == nil {
				close(acquired)
				<-releaseProbe
				unlock()
			}
		}()
		select {
		case <-acquired:
			return errors.New("target-wide lock was not held during prune remove")
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}
	t.Cleanup(func() {
		beforeRemoveApprovedTarget = previous
		close(releaseProbe)
	})

	result, err := Apply(validApplyOptions(target, "approval-lock"))
	if err != nil {
		t.Fatalf("Apply(lock) error = %v, want nil", err)
	}

	if result.Receipt.Status != control.PruneReceiptApplied {
		t.Fatalf("receipt = %+v, want applied receipt", result.Receipt)
	}
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatalf("shared target lock probe did not acquire after prune completed")
	}
}

func TestApplyRefusesStartedReceiptOnRerun(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-interrupted", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)
	report, err := PlanDryRun(validOptions(target))
	if err != nil {
		t.Fatalf("PlanDryRun(%q) error = %v, want nil", target, err)
	}
	if err := control.WriteNewFile(receiptPath(t, target, "prune-apply"), startedReceipt("prune-apply", approval, report, validOptions(target).Now)); err != nil {
		t.Fatalf("WriteNewFile(started receipt) error = %v, want nil", err)
	}

	_, err = Apply(validApplyOptions(target, "approval-interrupted"))
	if err == nil || !strings.Contains(err.Error(), "is started") {
		t.Fatalf("Apply(started receipt) error = %v, want fail-closed started receipt error", err)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "gone.txt")); statErr != nil {
		t.Fatalf("Lstat(target after started receipt refusal) error = %v, want nil", statErr)
	}
}

func TestApplyRejectsSymlinkApprovalArtifact(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-symlink-artifact", []control.PruneApprovalItem{approvalItem(record)})
	writePruneProfileSnapshot(t, target, approval)
	approvalDir := filepath.Join(target, control.DirName, "prune", "approvals")
	if err := os.MkdirAll(approvalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(approval dir) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(target, "outside-approval.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside approval) error = %v, want nil", err)
	}
	if err := os.Symlink(filepath.Join(target, "outside-approval.json"), filepath.Join(approvalDir, approval.ID+".json")); err != nil {
		t.Fatalf("Symlink(approval artifact) error = %v, want nil", err)
	}

	_, err := Apply(validApplyOptions(target, approval.ID))
	if err == nil || !strings.Contains(err.Error(), "exists as a symlink") {
		t.Fatalf("Apply(symlink approval artifact) error = %v, want symlink artifact refusal", err)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "gone.txt")); statErr != nil {
		t.Fatalf("Lstat(target after symlink approval rejection) error = %v, want nil", statErr)
	}
}

func TestApplyRejectsSymlinkApprovalDirectory(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-symlink-dir", []control.PruneApprovalItem{approvalItem(record)})
	writePruneProfileSnapshot(t, target, approval)

	pruneDir := filepath.Join(target, control.DirName, "prune")
	if err := os.MkdirAll(pruneDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(prune dir) error = %v, want nil", err)
	}
	approvalDir := filepath.Join(pruneDir, "approvals")
	if err := os.RemoveAll(approvalDir); err != nil {
		t.Fatalf("RemoveAll(approval dir) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "approvals"), 0o755); err != nil {
		t.Fatalf("MkdirAll(outside approvals) error = %v, want nil", err)
	}
	outsidePath := filepath.Join(outside, "approvals", approval.ID+".json")
	if err := control.WriteNewFile(outsidePath, approval); err != nil {
		t.Fatalf("WriteNewFile(outside approval) error = %v, want nil", err)
	}
	if err := os.Symlink(filepath.Join(outside, "approvals"), approvalDir); err != nil {
		t.Fatalf("Symlink(approval dir) error = %v, want nil", err)
	}

	_, err := Apply(validApplyOptions(target, approval.ID))
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Apply(symlink approval dir) error = %v, want symlink directory refusal", err)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "gone.txt")); statErr != nil {
		t.Fatalf("Lstat(target after symlink approval dir rejection) error = %v, want nil", statErr)
	}
}

func TestApplyRejectsSymlinkProfileSnapshotDirectory(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-symlink-profile-dir", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)

	profilesDir := filepath.Join(target, control.DirName, "profiles")
	if err := os.RemoveAll(profilesDir); err != nil {
		t.Fatalf("RemoveAll(profiles dir) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "profiles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(outside profiles) error = %v, want nil", err)
	}
	outsideSnapshot := filepath.Join(outside, "profiles", approval.ProfileSnapshotID+".json")
	payload := pruneProfileSnapshotPayloadForApproval(approval.ID)
	if err := control.WriteFile(outsideSnapshot, control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         approval.ProfileSnapshotID,
		ProfileID:  approval.ProfileID,
		CapturedAt: approval.CreatedAt,
		Profile:    payload,
	}); err != nil {
		t.Fatalf("WriteFile(outside profile snapshot) error = %v, want nil", err)
	}
	if err := os.Symlink(filepath.Join(outside, "profiles"), profilesDir); err != nil {
		t.Fatalf("Symlink(profiles dir) error = %v, want nil", err)
	}

	_, err := Apply(validApplyOptions(target, approval.ID))
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Apply(symlink profile dir) error = %v, want symlink directory refusal", err)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "gone.txt")); statErr != nil {
		t.Fatalf("Lstat(target after symlink profile dir rejection) error = %v, want nil", statErr)
	}
}

func TestApplyRejectsSymlinkReceiptDirectory(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-receipt-dir", []control.PruneApprovalItem{approvalItem(record)})
	writePruneApproval(t, target, approval)
	first, err := Apply(validApplyOptions(target, approval.ID))
	if err != nil {
		t.Fatalf("Apply(first) error = %v, want nil", err)
	}
	if first.Receipt.Status != control.PruneReceiptApplied {
		t.Fatalf("Apply(first).Receipt.Status = %q, want applied", first.Receipt.Status)
	}
	writePruneTargetFile(t, target, "gone.txt", []byte("recreated"))

	receiptDir := filepath.Join(target, control.DirName, "prune", "receipts")
	if err := os.RemoveAll(receiptDir); err != nil {
		t.Fatalf("RemoveAll(receipt dir) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "receipts"), 0o755); err != nil {
		t.Fatalf("MkdirAll(outside receipts) error = %v, want nil", err)
	}
	outsideReceipt := filepath.Join(outside, "receipts", "prune-apply.json")
	if err := control.WriteFile(outsideReceipt, first.Receipt); err != nil {
		t.Fatalf("WriteFile(outside receipt) error = %v, want nil", err)
	}
	if err := os.Symlink(filepath.Join(outside, "receipts"), receiptDir); err != nil {
		t.Fatalf("Symlink(receipt dir) error = %v, want nil", err)
	}

	_, err = Apply(validApplyOptions(target, approval.ID))
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Apply(symlink receipt dir) error = %v, want symlink directory refusal", err)
	}
	data, readErr := os.ReadFile(filepath.Join(target, "gone.txt"))
	if readErr != nil {
		t.Fatalf("ReadFile(target after symlink receipt dir rejection) error = %v, want nil", readErr)
	}
	if string(data) != "recreated" {
		t.Fatalf("target content after symlink receipt dir rejection = %q, want recreated", string(data))
	}
}

func TestApplyRejectsApprovalPathIDMismatch(t *testing.T) {
	target := t.TempDir()
	writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
	record := softDelete("session-two", "gone.txt", []byte("gone"))
	writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
	approval := approvalForItems("approval-real", []control.PruneApprovalItem{approvalItem(record)})
	writePruneProfileSnapshot(t, target, approval)
	mismatchPath := filepath.Join(target, control.DirName, "prune", "approvals", "approval-path.json")
	if err := control.WriteNewFile(mismatchPath, approval); err != nil {
		t.Fatalf("WriteNewFile(mismatched approval path) error = %v, want nil", err)
	}
	opts := validApplyOptions(target, "")
	opts.ApprovalID = ""
	opts.ApprovalPath = mismatchPath

	_, err := Apply(opts)
	if err == nil || !strings.Contains(err.Error(), "does not match requested id") {
		t.Fatalf("Apply(approval path id mismatch) error = %v, want id mismatch", err)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "gone.txt")); statErr != nil {
		t.Fatalf("Lstat(target after approval path rejection) error = %v, want nil", statErr)
	}
}

func TestApplyMissingInvalidAndExpiredApproval(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, target string, record control.SoftDelete)
		want    string
	}{
		{
			name:    "missing",
			prepare: func(t *testing.T, target string, record control.SoftDelete) {},
			want:    "read prune approval",
		},
		{
			name: "invalid status",
			prepare: func(t *testing.T, target string, record control.SoftDelete) {
				approval := approvalForItems("approval-check", []control.PruneApprovalItem{approvalItem(record)})
				approval.Status = "refused"
				approval.RefusalReason = "operator refused"
				approval.ApprovedBy = ""
				approval.ApprovedAt = ""
				writePruneApproval(t, target, approval)
			},
			want: "is not approved",
		},
		{
			name: "expired",
			prepare: func(t *testing.T, target string, record control.SoftDelete) {
				approval := approvalForItems("approval-check", []control.PruneApprovalItem{approvalItem(record)})
				approval.ExpiresAt = "2026-05-18T09:59:59Z"
				writePruneApproval(t, target, approval)
			},
			want: "approval expired",
		},
		{
			name: "expires exactly now",
			prepare: func(t *testing.T, target string, record control.SoftDelete) {
				approval := approvalForItems("approval-check", []control.PruneApprovalItem{approvalItem(record)})
				approval.ExpiresAt = "2026-05-18T10:00:00Z"
				writePruneApproval(t, target, approval)
			},
			want: "approval expired",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			writePruneTargetFile(t, target, "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-one", []control.ManifestEntry{fileEntry("gone.txt", []byte("gone"))})
			record := softDelete("session-two", "gone.txt", []byte("gone"))
			writePruneSession(t, target, "session-two", []control.ManifestEntry{fileEntry("keep.txt", []byte("keep"))}, record)
			tt.prepare(t, target, record)

			_, err := Apply(validApplyOptions(target, "approval-check"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Apply(%s) error = %v, want containing %q", tt.name, err, tt.want)
			}
			if _, statErr := os.Lstat(filepath.Join(target, "gone.txt")); statErr != nil {
				t.Fatalf("Lstat(target after %s) error = %v, want nil", tt.name, statErr)
			}
			if _, statErr := os.Lstat(receiptPath(t, target, "prune-apply")); !os.IsNotExist(statErr) {
				t.Fatalf("Lstat(receipt after %s) error = %v, want not exist", tt.name, statErr)
			}
		})
	}
}

func TestApprovalScopeDigestDeterministicAndCanonical(t *testing.T) {
	policy := validOptions("unused").DeletePolicy
	a := approvalItem(softDelete("session-two", "a.txt", []byte("a")))
	a.SoftDeleteID = "session-two-del_a"
	a.SoftDeleteRef = "deleted/session-two-del_a.json"
	b := approvalItem(softDelete("session-two", "b.txt", []byte("b")))
	b.SoftDeleteID = "session-two-del_b"
	b.SoftDeleteRef = "deleted/session-two-del_b.json"
	snapshotDigest := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	first := ApprovalScopeDigest("profile-local", "target-local", "root", "profile-snapshot", snapshotDigest, policy, []control.PruneApprovalItem{a, b})
	second := ApprovalScopeDigest("profile-local", "target-local", "root", "profile-snapshot", snapshotDigest, policy, []control.PruneApprovalItem{a, b})
	reordered := ApprovalScopeDigest("profile-local", "target-local", "root", "profile-snapshot", snapshotDigest, policy, []control.PruneApprovalItem{b, a})
	changedPolicy := policy
	changedPolicy.RetentionDays = 31
	changed := ApprovalScopeDigest("profile-local", "target-local", "root", "profile-snapshot", snapshotDigest, changedPolicy, []control.PruneApprovalItem{a, b})
	changedRoot := ApprovalScopeDigest("profile-local", "target-local", "other-root", "profile-snapshot", snapshotDigest, policy, []control.PruneApprovalItem{a, b})
	changedSnapshot := ApprovalScopeDigest("profile-local", "target-local", "root", "profile-other", snapshotDigest, policy, []control.PruneApprovalItem{a, b})
	changedSnapshotDigest := ApprovalScopeDigest("profile-local", "target-local", "root", "profile-snapshot", digest([]byte("other")), policy, []control.PruneApprovalItem{a, b})

	if first != second {
		t.Fatalf("ApprovalScopeDigest deterministic mismatch: %q != %q", first, second)
	}
	if first != reordered {
		t.Fatalf("ApprovalScopeDigest order-sensitive for canonical approved items: %q != %q", first, reordered)
	}
	if first == changed {
		t.Fatalf("ApprovalScopeDigest policy-insensitive: %q", first)
	}
	if first == changedRoot {
		t.Fatalf("ApprovalScopeDigest root-insensitive: %q", first)
	}
	if first == changedSnapshot {
		t.Fatalf("ApprovalScopeDigest profile snapshot-insensitive: %q", first)
	}
	if first == changedSnapshotDigest {
		t.Fatalf("ApprovalScopeDigest profile snapshot digest-insensitive: %q", first)
	}
}

func validOptions(target string) Options {
	return Options{
		TargetRoot: target,
		ProfileID:  "profile-local",
		TargetID:   "target-local",
		DeletePolicy: profile.DeletePolicy{
			Mode:               profile.DeleteModePrune,
			RequireReview:      true,
			RetentionDays:      30,
			AllowPhysicalPrune: true,
		},
		Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
	}
}

func validApplyOptions(target, approvalID string) ApplyOptions {
	opts := validOptions(target)
	return ApplyOptions{
		TargetRoot:     opts.TargetRoot,
		ProfileID:      opts.ProfileID,
		TargetID:       opts.TargetID,
		ApprovalID:     approvalID,
		PruneSessionID: "prune-apply",
		DeletePolicy:   opts.DeletePolicy,
		Now:            opts.Now,
	}
}

func validAuthorApprovalOptions(target, approvalID string, softDeleteIDs ...string) AuthorApprovalOptions {
	p := pruneProfileForTest()
	p.Target.LocalPath = target
	payload := pruneProfilePayload(p, approvalID)
	return AuthorApprovalOptions{
		Profile:        p,
		ProfilePayload: payload,
		ApprovalID:     approvalID,
		SoftDeleteIDs:  softDeleteIDs,
		ApprovedBy:     "reviewer",
		Reason:         "reviewed stale target",
		ExpiresAt:      "2026-05-19T00:00:00Z",
		Now:            validOptions(target).Now,
	}
}

func fileEntry(rel string, data []byte) control.ManifestEntry {
	entry := control.ManifestEntry{
		Path:       rel,
		Kind:       "file",
		ModTime:    "2026-05-16T00:00:00Z",
		Digest:     digest(data),
		TargetPath: rel,
	}
	entry.SetSizeEvidence(int64(len(data)))
	entry.SetModeEvidence(0o644)
	return entry
}

func symlinkEntry(rel, target string) control.ManifestEntry {
	return control.ManifestEntry{
		Path:          rel,
		Kind:          "symlink",
		ModTime:       "2026-05-16T00:00:00Z",
		TargetPath:    rel,
		SymlinkTarget: target,
	}
}

func dirEntry(rel string) control.ManifestEntry {
	entry := control.ManifestEntry{
		Path:       rel,
		Kind:       "dir",
		ModTime:    "2026-05-16T00:00:00Z",
		TargetPath: rel,
	}
	entry.SetModeEvidence(0o755)
	return entry
}

func softDelete(sessionID, targetPath string, data []byte) control.SoftDelete {
	return control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 sessionID + "-del_gone",
		SessionID:          sessionID,
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "session-one",
		PreviousManifestID: "manifest-session-one",
		SourcePath:         targetPath,
		TargetPath:         targetPath,
		Kind:               "file",
		Size:               int64(len(data)),
		Digest:             digest(data),
		DetectedAt:         "2026-04-16T00:02:00Z",
		Reason:             "source_missing",
	}
}

func symlinkSoftDelete(sessionID, targetPath string) control.SoftDelete {
	record := softDelete(sessionID, targetPath, nil)
	record.Kind = "symlink"
	record.Size = 0
	record.Digest = ""
	record.SymlinkTarget = "target.txt"
	return record
}

func dirSoftDelete(sessionID, targetPath string) control.SoftDelete {
	record := softDelete(sessionID, targetPath, nil)
	record.Kind = "dir"
	record.Size = 0
	record.Digest = ""
	return record
}

func approvalItem(record control.SoftDelete) control.PruneApprovalItem {
	return control.PruneApprovalItem{
		SoftDeleteID:       record.ID,
		SoftDeleteRef:      "deleted/" + record.ID + ".json",
		DetectedSessionID:  record.SessionID,
		PreviousSessionID:  record.PreviousSessionID,
		PreviousManifestID: record.PreviousManifestID,
		RootID:             record.RootID,
		SourcePath:         record.SourcePath,
		TargetPath:         record.TargetPath,
		Kind:               record.Kind,
		Size:               record.Size,
		Digest:             record.Digest,
		SymlinkTarget:      record.SymlinkTarget,
		DetectedAt:         record.DetectedAt,
	}
}

func approvalForItems(id string, items []control.PruneApprovalItem) control.PruneApproval {
	policy := validOptions("unused").DeletePolicy
	snapshotPayload := pruneProfileSnapshotPayloadForApproval(id)
	snapshotDigest, err := ProfileSnapshotDigest(snapshotPayload)
	if err != nil {
		panic(fmt.Sprintf("profile snapshot digest for %s: %v", id, err))
	}
	approval := control.PruneApproval{
		Version:               control.CurrentVersion,
		ID:                    id,
		ProfileID:             "profile-local",
		TargetID:              "target-local",
		RootID:                "root",
		CreatedAt:             "2026-05-18T09:00:00Z",
		ApprovedBy:            "reviewer",
		ApprovedAt:            "2026-05-18T09:30:00Z",
		ReviewTool:            "unit-test",
		ProfileSnapshotID:     "profile-snapshot",
		ProfileSnapshotDigest: snapshotDigest,
		ProfileDeletePolicy: control.PruneDeletePolicy{
			Mode:               string(policy.Mode),
			RequireReview:      policy.RequireReview,
			RetentionDays:      policy.RetentionDays,
			AllowPhysicalPrune: policy.AllowPhysicalPrune,
		},
		Items:          items,
		ExpiresAt:      "2026-05-19T00:00:00Z",
		Status:         "approved",
		ApprovalReason: "reviewed",
	}
	approval.ApprovalScopeDigest = ApprovalScopeDigest(approval.ProfileID, approval.TargetID, approval.RootID, approval.ProfileSnapshotID, approval.ProfileSnapshotDigest, policy, approval.Items)
	return approval
}

func writePruneSession(t *testing.T, target string, sessionID string, entries []control.ManifestEntry, softDeletes ...control.SoftDelete) {
	t.Helper()
	writePruneManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   entries,
	})
	writePublishedReceipt(t, target, sessionID)
	for _, record := range softDeletes {
		writeSoftDelete(t, target, record)
	}
}

func writePruneManifest(t *testing.T, target string, manifest control.Manifest) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, manifest.SessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest %q) error = %v, want nil", manifest.SessionID, err)
	}
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
	}
}

func rewritePruneManifestCreatedAt(t *testing.T, target string, sessionID string, createdAt string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest %q) error = %v, want nil", sessionID, err)
	}
	manifest, err := control.ReadManifestCompatFile(path)
	if err != nil {
		t.Fatalf("control.ReadManifestCompatFile(%q) error = %v, want nil", path, err)
	}
	manifest.CreatedAt = createdAt
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
	}
}

func writePublishedReceipt(t *testing.T, target string, sessionID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
	}
	err = control.WriteFile(path, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: "profile-local",
		TargetID:  "target-local",
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	})
	if err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", path, err)
	}
}

func writeSoftDelete(t *testing.T, target string, record control.SoftDelete) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSoftDelete, record.ID)
	if err != nil {
		t.Fatalf("control.Path(soft delete %q) error = %v, want nil", record.ID, err)
	}
	if err := control.WriteFile(path, record); err != nil {
		t.Fatalf("control.WriteFile(%q, soft delete) error = %v, want nil", path, err)
	}
}

func writeRawPruneArtifact(t *testing.T, target, dir, name, data string) {
	t.Helper()
	path := filepath.Join(target, control.DirName, dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
}

func writePruneApproval(t *testing.T, target string, approval control.PruneApproval) {
	t.Helper()
	writePruneProfileSnapshot(t, target, approval)
	path, err := control.Path(target, control.ArtifactPruneApproval, approval.ID)
	if err != nil {
		t.Fatalf("control.Path(prune approval %q) error = %v, want nil", approval.ID, err)
	}
	if err := control.WriteNewFile(path, approval); err != nil {
		t.Fatalf("control.WriteNewFile(%q, prune approval) error = %v, want nil", path, err)
	}
}

func writePruneProfileSnapshot(t *testing.T, target string, approval control.PruneApproval) {
	t.Helper()
	payload := pruneProfileSnapshotPayloadForApproval(approval.ID)
	snapshotDigest, err := ProfileSnapshotDigest(payload)
	if err != nil {
		t.Fatalf("ProfileSnapshotDigest(%q) error = %v, want nil", approval.ID, err)
	}
	if snapshotDigest != approval.ProfileSnapshotDigest {
		t.Fatalf("profile snapshot digest for %q = %s, want approval digest %s", approval.ID, snapshotDigest, approval.ProfileSnapshotDigest)
	}
	path, err := control.Path(target, control.ArtifactProfileSnapshot, approval.ProfileSnapshotID)
	if err != nil {
		t.Fatalf("control.Path(profile snapshot %q) error = %v, want nil", approval.ProfileSnapshotID, err)
	}
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         approval.ProfileSnapshotID,
		ProfileID:  approval.ProfileID,
		CapturedAt: approval.CreatedAt,
		Profile:    payload,
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, profile snapshot) error = %v, want nil", path, err)
	}
}

func pruneProfileSnapshotPayloadForApproval(approvalID string) []byte {
	p := pruneProfileForTest()
	return pruneProfilePayload(p, approvalID)
}

func pruneProfilePayload(p profile.Profile, approvalID string) []byte {
	payload, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal profile snapshot for %s: %v", approvalID, err))
	}
	return append(payload, '\n')
}

func pruneProfileForTest() profile.Profile {
	policy := validOptions("unused").DeletePolicy
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join("target", "source"), "target")
	p.Target.TargetID = "target-local"
	p.DeletePolicy = policy
	return p
}

func approvalPath(t *testing.T, target, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPruneApproval, id)
	if err != nil {
		t.Fatalf("control.Path(prune approval %q) error = %v, want nil", id, err)
	}
	return path
}

func receiptPath(t *testing.T, target, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPruneReceipt, id)
	if err != nil {
		t.Fatalf("control.Path(prune receipt %q) error = %v, want nil", id, err)
	}
	return path
}

func writePruneTargetFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
}

func mustWritePruneFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
}

func snapshotPruneTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		if entry.IsDir() && entry.Type()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%q) error = %v, want nil", root, err)
	}
	return out
}

func samePruneTreeExcept(after, before, allowedAdded []string) bool {
	allowed := map[string]struct{}{}
	for _, path := range allowedAdded {
		allowed[path] = struct{}{}
	}
	beforeSet := map[string]struct{}{}
	for _, path := range before {
		beforeSet[path] = struct{}{}
	}
	for _, path := range before {
		if !containsPrunePath(after, path) {
			return false
		}
	}
	for _, path := range after {
		if _, ok := beforeSet[path]; ok {
			continue
		}
		if _, ok := allowed[path]; !ok {
			return false
		}
	}
	return true
}

func containsPrunePath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
