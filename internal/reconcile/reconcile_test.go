package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
)

func TestPlanClassifiesMissingFileRepairFromManifestAndSourceEvidence(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-missing", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)

	receipt, err := Plan(Options{Profile: p, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan() error = %v, want nil", err)
	}

	if receipt.Schema != SchemaPlanReceipt || receipt.ApplyIntent {
		t.Fatalf("Plan() metadata = %+v, want dry-run plan receipt", receipt)
	}
	if receipt.Summary.Records != 1 || receipt.Summary.Planned != 1 || receipt.Summary.Refused != 0 {
		t.Fatalf("Plan().Summary = %+v, want one planned action", receipt.Summary)
	}
	action := receipt.Actions[0]
	if action.DriftID != drift.ID || action.Action != ActionRestoreFile || action.Result != ResultPlanned || action.Path != "file.txt" {
		t.Fatalf("Plan() action = %+v, want missing-file restore plan", action)
	}
	if action.Expected.SessionID != drift.Expected.SessionID || action.Expected.ManifestID != drift.Expected.ManifestID || action.Expected.Digest != digest([]byte("payload")) {
		t.Fatalf("Plan() expected evidence = %+v, want drift expected manifest evidence", action.Expected)
	}
	if action.ObservedBefore.Present == nil || *action.ObservedBefore.Present || action.ObservedBefore.Kind != "missing" {
		t.Fatalf("Plan() observed = %+v, want missing target", action.ObservedBefore)
	}
	if action.SourceEvidence == nil || action.SourceEvidence.Digest != digest([]byte("payload")) || action.SourceEvidence.Size != 7 {
		t.Fatalf("Plan() source evidence = %+v, want matching source digest/size", action.SourceEvidence)
	}
}

func TestPlanRefusesScopeMismatchWithoutMutation(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-scope", "file.txt", []byte("payload"))
	drift.ProfileID = "other-profile"
	writePublishedSession(t, target, drift.SessionID, "other-profile", p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	before := snapshotTree(t, target)

	receipt, err := Plan(Options{Profile: p, IDs: []string{drift.ID}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan() error = %v, want nil", err)
	}

	if receipt.Summary.Planned != 0 || receipt.Summary.Refused != 1 {
		t.Fatalf("Plan().Summary = %+v, want one refusal", receipt.Summary)
	}
	if receipt.Refusals[0].ReasonCode != ReasonProfileScopeMismatch {
		t.Fatalf("Plan() refusal = %+v, want scope mismatch", receipt.Refusals[0])
	}
	assertTreeUnchanged(t, target, before)
}

func TestPlanDryRunDoesNotMutateTarget(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-dry-run", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	before := snapshotTree(t, target)

	receipt, err := Plan(Options{Profile: p, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan() error = %v, want nil", err)
	}
	if receipt.Summary.Planned != 1 {
		t.Fatalf("Plan().Summary = %+v, want one planned dry-run action", receipt.Summary)
	}
	assertTreeUnchanged(t, target, before)
	if _, err := os.Lstat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("target file after dry-run err = %v, want missing", err)
	}
}

func TestApplyRequiresIntentAndPreflightRefusesChangedTarget(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-apply", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	before := snapshotTree(t, target)

	withoutIntent, err := Apply(ApplyOptions{Profile: p, IDs: []string{drift.ID}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Apply(no intent) error = %v, want nil receipt refusal", err)
	}
	if withoutIntent.Summary.Planned != 0 || withoutIntent.Summary.Refused != 1 || withoutIntent.Refusals[0].ReasonCode != ReasonMissingApplyIntent {
		t.Fatalf("Apply(no intent) = %+v, want missing-intent refusal", withoutIntent)
	}
	assertTreeUnchanged(t, target, before)

	writeTargetFile(t, target, "file.txt", []byte("operator"), 0o644)
	changedBeforeFile := readFileDigest(t, filepath.Join(target, "file.txt"))
	changedBeforeDrift := readRawFile(t, targetDriftPath(t, target, drift.ID))
	withIntent, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "repair missing target file",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply(changed target) error = %v, want nil receipt refusal", err)
	}
	if withIntent.Summary.Applied != 0 || withIntent.Summary.Refused != 1 || withIntent.Refusals[0].ReasonCode != ReasonAmbiguousState {
		t.Fatalf("Apply(changed target) = %+v, want preflight refusal", withIntent)
	}
	if got := readFileDigest(t, filepath.Join(target, "file.txt")); got != changedBeforeFile {
		t.Fatalf("target file digest changed after refused apply: got %s want %s", got, changedBeforeFile)
	}
	if got := readRawFile(t, targetDriftPath(t, target, drift.ID)); string(got) != string(changedBeforeDrift) {
		t.Fatalf("target drift changed after refused apply\nbefore=%s\nafter=%s", changedBeforeDrift, got)
	}
}

func TestApplyRestoresFileAndMarksDriftResolved(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-restore", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)

	receipt, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "restore from source evidence",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	if receipt.Summary.Applied != 1 || receipt.Summary.Refused != 0 {
		t.Fatalf("Apply().Summary = %+v, want one applied action", receipt.Summary)
	}
	got, err := os.ReadFile(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile(restored target) error = %v, want nil", err)
	}
	if string(got) != "payload" {
		t.Fatalf("restored target content = %q, want payload", got)
	}
	persisted := readTargetDrift(t, target, drift.ID)
	if persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" || persisted.ReviewedAt != fixedNow().Format(time.RFC3339Nano) || persisted.ReviewedBy != "ops@example.com" || persisted.ReviewReason != "restore from source evidence" {
		t.Fatalf("persisted drift review = %+v, want resolved apply evidence", persisted)
	}
}

func TestApplyRequiresExplicitIDsForMutation(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-require-id", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	before := snapshotTree(t, target)

	_, err := Apply(ApplyOptions{
		Profile:  p,
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "must select drift ids",
		Now:      fixedNow(),
	})

	if err == nil || !strings.Contains(err.Error(), "at least one persisted target drift id is required") {
		t.Fatalf("Apply(no ids) error = %v, want explicit id requirement", err)
	}
	assertTreeUnchanged(t, target, before)
}

func TestPlanRefusesRestoreWhenManifestSourceAndTargetPathsDiffer(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "source-name.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-source-target-diverge", "target-name.txt", []byte("payload"))
	entry := manifestEntry("source-name.txt", []byte("payload"))
	entry.TargetPath = "target-name.txt"
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{entry})
	writeTargetDrift(t, target, drift)

	receipt, err := Plan(Options{Profile: p, IDs: []string{drift.ID}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan(diverged source/target path) error = %v, want nil", err)
	}

	if receipt.Summary.Planned != 0 || receipt.Summary.Refused != 1 {
		t.Fatalf("Plan(diverged source/target path).Summary = %+v, want one refusal", receipt.Summary)
	}
	if receipt.Refusals[0].ReasonCode != ReasonPublishedEvidence {
		t.Fatalf("Plan(diverged source/target path).Refusal = %+v, want published evidence refusal", receipt.Refusals[0])
	}
}

func TestApplyRefusesWhenSourceChangesBeforeStagedPublish(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-source-race", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	beforeDrift := readRawFile(t, targetDriftPath(t, target, drift.ID))

	originalAfterSourcePreflight := afterSourcePreflight
	t.Cleanup(func() { afterSourcePreflight = originalAfterSourcePreflight })
	afterSourcePreflight = func(path string) {
		if strings.HasSuffix(filepath.ToSlash(path), "file.txt") {
			if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
				t.Fatalf("os.WriteFile(source race) error = %v, want nil", err)
			}
		}
	}

	receipt, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "restore from source evidence",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply(source race) error = %v, want nil receipt refusal", err)
	}

	if receipt.Summary.Applied != 0 || receipt.Summary.Refused != 1 {
		t.Fatalf("Apply(source race).Summary = %+v, want one refusal", receipt.Summary)
	}
	if receipt.Refusals[0].ReasonCode != ReasonSourceMismatch || !strings.Contains(receipt.Refusals[0].Message, "staged file does not match expected evidence") {
		t.Fatalf("Apply(source race).Refusal = %+v, want staged mismatch refusal", receipt.Refusals[0])
	}
	if _, err := os.Lstat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("target file after refused source race err = %v, want missing", err)
	}
	if got := readRawFile(t, targetDriftPath(t, target, drift.ID)); string(got) != string(beforeDrift) {
		t.Fatalf("target drift changed after refused source race\nbefore=%s\nafter=%s", beforeDrift, got)
	}
}

func TestApplyRefusesWhenSourcePathIsReplacedBeforePublish(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-source-replace", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	beforeDrift := readRawFile(t, targetDriftPath(t, target, drift.ID))

	originalAfterSourcePreflight := afterSourcePreflight
	t.Cleanup(func() { afterSourcePreflight = originalAfterSourcePreflight })
	afterSourcePreflight = func(path string) {
		if strings.HasSuffix(filepath.ToSlash(path), "file.txt") {
			tmp := path + ".replacement"
			if err := os.WriteFile(tmp, []byte("changed"), 0o644); err != nil {
				t.Fatalf("os.WriteFile(source replacement) error = %v, want nil", err)
			}
			if err := os.Rename(tmp, path); err != nil {
				t.Fatalf("os.Rename(source replacement) error = %v, want nil", err)
			}
		}
	}

	receipt, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "restore from source evidence",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply(source replacement) error = %v, want nil receipt refusal", err)
	}

	if receipt.Summary.Applied != 0 || receipt.Summary.Refused != 1 {
		t.Fatalf("Apply(source replacement).Summary = %+v, want one refusal", receipt.Summary)
	}
	if receipt.Refusals[0].ReasonCode != ReasonSourceMismatch || !strings.Contains(receipt.Refusals[0].Message, "current source file changed before publish") {
		t.Fatalf("Apply(source replacement).Refusal = %+v, want current-source mismatch refusal", receipt.Refusals[0])
	}
	if _, err := os.Lstat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("target file after refused source replacement err = %v, want missing", err)
	}
	if got := readRawFile(t, targetDriftPath(t, target, drift.ID)); string(got) != string(beforeDrift) {
		t.Fatalf("target drift changed after refused source replacement\nbefore=%s\nafter=%s", beforeDrift, got)
	}
}

func TestApplyRefusesDuplicateSelectedTargetPathsBeforeMutation(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	driftA := missingFileDrift("drift-duplicate-a", "file.txt", []byte("payload"))
	driftB := missingFileDrift("drift-duplicate-b", "file.txt", []byte("payload"))
	writePublishedSession(t, target, driftA.SessionID, p.ProfileID, p.Target.TargetID, driftA.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, driftA)
	writeTargetDrift(t, target, driftB)
	before := snapshotTree(t, target)

	receipt, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{driftA.ID, driftB.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "restore from source evidence",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply(duplicate target paths) error = %v, want nil receipt refusal", err)
	}

	if receipt.Summary.Applied != 0 || receipt.Summary.Refused != 2 {
		t.Fatalf("Apply(duplicate target paths).Summary = %+v, want two refusals", receipt.Summary)
	}
	for _, refusal := range receipt.Refusals {
		if refusal.ReasonCode != ReasonAmbiguousState || !strings.Contains(refusal.Message, "multiple selected drift records target the same path") {
			t.Fatalf("Apply(duplicate target paths).Refusal = %+v, want duplicate target path refusal", refusal)
		}
	}
	assertTreeUnchanged(t, target, before)
	if _, err := os.Lstat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("target file after duplicate-target refusal err = %v, want missing", err)
	}
}

func TestPlanTreatsAlreadyRestoredMissingFileAsResolveNoop(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	writeTargetFile(t, target, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-restored-noop", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)

	receipt, err := Plan(Options{Profile: p, IDs: []string{drift.ID}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan(already restored) error = %v, want nil", err)
	}

	if receipt.Summary.Planned != 1 || receipt.Summary.Refused != 0 {
		t.Fatalf("Plan(already restored).Summary = %+v, want one planned noop", receipt.Summary)
	}
	action := receipt.Actions[0]
	if action.Action != ActionResolveNoop || action.Result != ResultPlanned || action.Reason != "target already matches expected evidence" {
		t.Fatalf("Plan(already restored).Action = %+v, want resolve noop", action)
	}
	if action.ObservedBefore.Present == nil || !*action.ObservedBefore.Present || action.ObservedBefore.Digest != digest([]byte("payload")) {
		t.Fatalf("Plan(already restored).ObservedBefore = %+v, want restored file evidence", action.ObservedBefore)
	}
}

func TestPlanRefusesAlreadyRestoredMissingFileWithoutExpectedDigestAndSize(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeTargetFile(t, target, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-restored-weak-evidence", "file.txt", []byte("payload"))
	drift.Expected = control.TargetDriftExpectedState{
		SessionID:  drift.SessionID,
		ManifestID: "manifest-" + drift.SessionID,
		Kind:       "file",
		Path:       "file.txt",
	}
	writeTargetDrift(t, target, drift)

	receipt, err := Plan(Options{Profile: p, IDs: []string{drift.ID}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan(weak restored evidence) error = %v, want nil", err)
	}

	if receipt.Summary.Planned != 0 || receipt.Summary.Refused != 1 {
		t.Fatalf("Plan(weak restored evidence).Summary = %+v, want one refusal", receipt.Summary)
	}
	if receipt.Refusals[0].ReasonCode != ReasonPublishedEvidence {
		t.Fatalf("Plan(weak restored evidence).Refusal = %+v, want published evidence refusal", receipt.Refusals[0])
	}
}

func TestApplyResolvesAlreadyRestoredMissingFileDrift(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	writeTargetFile(t, target, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-stranded-restore", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)

	receipt, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "target already restored from source evidence",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply(already restored) error = %v, want nil", err)
	}

	if receipt.Summary.Applied != 1 || receipt.Summary.Refused != 0 {
		t.Fatalf("Apply(already restored).Summary = %+v, want one applied resolve", receipt.Summary)
	}
	action := receipt.Actions[0]
	if action.Action != ActionResolveNoop || action.Result != ResultApplied {
		t.Fatalf("Apply(already restored).Action = %+v, want applied resolve noop", action)
	}
	persisted := readTargetDrift(t, target, drift.ID)
	if persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" || persisted.ReviewReason != "target already restored from source evidence" {
		t.Fatalf("persisted drift = %+v, want resolved stranded restore", persisted)
	}
}

func TestApplyRefusesPublishErrorThenAllowsExplicitResolveRerun(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-post-publish", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)

	originalPromote := promoteFileNoReplace
	t.Cleanup(func() { promoteFileNoReplace = originalPromote })
	promoteFileNoReplace = func(tempPath string, finalPath string) error {
		if err := originalPromote(tempPath, finalPath); err != nil {
			return err
		}
		return errors.New("injected post-link failure")
	}

	refused, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "restore missing target file",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply(post-publish error) error = %v, want nil", err)
	}

	if refused.Summary.Applied != 0 || refused.Summary.Refused != 1 {
		t.Fatalf("Apply(post-publish error).Summary = %+v, want one refusal", refused.Summary)
	}
	if refusal := refused.Refusals[0]; refusal.ReasonCode != ReasonMutationFailed || !strings.Contains(refusal.Message, "target now matches expected evidence") {
		t.Fatalf("Apply(post-publish error).Refusal = %+v, want mutation refusal with rerun guidance", refusal)
	}
	if got := readFileDigest(t, filepath.Join(target, "file.txt")); got != digest([]byte("payload")) {
		t.Fatalf("target file digest after refusal = %s, want restored payload", got)
	}
	persisted := readTargetDrift(t, target, drift.ID)
	if persisted.ReviewState != "needs_review" {
		t.Fatalf("persisted drift after refused publish = %+v, want unresolved", persisted)
	}

	promoteFileNoReplace = originalPromote
	resolved, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "target already restored after previous publish attempt",
		Now:      fixedNow().Add(time.Second),
	})
	if err != nil {
		t.Fatalf("Apply(resolve rerun) error = %v, want nil", err)
	}
	if resolved.Summary.Applied != 1 || resolved.Summary.Refused != 0 {
		t.Fatalf("Apply(resolve rerun).Summary = %+v, want one applied resolution", resolved.Summary)
	}
	action := resolved.Actions[0]
	if action.Action != ActionResolveNoop || action.Result != ResultApplied {
		t.Fatalf("Apply(resolve rerun).Action = %+v, want applied resolve noop", action)
	}
	persisted = readTargetDrift(t, target, drift.ID)
	if persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" || persisted.ReviewReason != "target already restored after previous publish attempt" {
		t.Fatalf("persisted drift after resolve rerun = %+v, want resolved stranded restore", persisted)
	}
}

func TestApplyRestoreFileRefusesWhenStagedModTimeCannotBeSet(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-restore-ctime", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	beforeDrift := readRawFile(t, targetDriftPath(t, target, drift.ID))

	originalChtimes := chtimes
	t.Cleanup(func() { chtimes = originalChtimes })
	chtimes = func(name string, atime time.Time, mtime time.Time) error {
		if strings.Contains(filepath.Base(name), ".reconcile-") {
			return errors.New("injected chtimes failure")
		}
		return originalChtimes(name, atime, mtime)
	}

	receipt, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "restore from source evidence",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply() error = %v, want nil receipt refusal", err)
	}

	if receipt.Summary.Applied != 0 || receipt.Summary.Refused != 1 {
		t.Fatalf("Apply().Summary = %+v, want one refusal", receipt.Summary)
	}
	if receipt.Refusals[0].ReasonCode != ReasonMutationFailed || !strings.Contains(receipt.Refusals[0].Message, "set staged mod time") {
		t.Fatalf("Apply() refusal = %+v, want staged modtime mutation failure", receipt.Refusals[0])
	}
	if _, err := os.Lstat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("target file after refused apply err = %v, want missing", err)
	}
	if got := readRawFile(t, targetDriftPath(t, target, drift.ID)); string(got) != string(beforeDrift) {
		t.Fatalf("target drift changed after refused apply\nbefore=%s\nafter=%s", beforeDrift, got)
	}
}

func TestApplyRestoresFileWithExpectedMetadataEvidence(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o600)
	drift := missingFileDrift("drift-restore-metadata", "file.txt", []byte("payload"))
	drift.Expected.SetModeEvidence(0o600)
	entry := manifestEntry("file.txt", []byte("payload"))
	entry.SetModeEvidence(0o600)
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{entry})
	writeTargetDrift(t, target, drift)

	receipt, err := Apply(ApplyOptions{
		Profile:  p,
		IDs:      []string{drift.ID},
		Apply:    true,
		Reviewer: "ops@example.com",
		Reason:   "restore from source evidence",
		Now:      fixedNow(),
	})
	if err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	if receipt.Summary.Applied != 1 || receipt.Summary.Refused != 0 {
		t.Fatalf("Apply().Summary = %+v, want one applied action", receipt.Summary)
	}
	info, err := os.Lstat(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("Lstat(restored target) error = %v, want nil", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("restored target mode = %04o, want 0600", got)
	}
	if got := info.ModTime().UTC().Format(time.RFC3339Nano); got != drift.Expected.ModTime {
		t.Fatalf("restored target modtime = %q, want %q", got, drift.Expected.ModTime)
	}
}

func TestPlanExplicitIDIgnoresUnselectedForeignMalformedArtifact(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "file.txt", []byte("payload"), 0o644)
	drift := missingFileDrift("drift-selected", "file.txt", []byte("payload"))
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID, []control.ManifestEntry{manifestEntry("file.txt", []byte("payload"))})
	writeTargetDrift(t, target, drift)
	writePublishedSession(t, target, "foreign-session", "foreign-profile", "foreign-target", drift.RootID, nil)
	writeRawArtifact(t, target, "drift", "foreign-drift.json", `{"version":1,"id":"foreign-drift","session_id":"foreign-session","profile_id":"foreign-profile","target_id":"foreign-target","root_id":"root","path":"file.txt",`)

	receipt, err := Plan(Options{Profile: p, IDs: []string{drift.ID}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan() error = %v, want nil", err)
	}

	if receipt.Summary.ArtifactProblems != 0 || len(receipt.ArtifactProblems) != 0 {
		t.Fatalf("Plan().ArtifactProblems = %#v summary=%+v, want unselected foreign problem ignored", receipt.ArtifactProblems, receipt.Summary)
	}
	if receipt.Summary.Planned != 1 || receipt.Summary.Refused != 0 {
		t.Fatalf("Plan().Summary = %+v, want selected drift planned", receipt.Summary)
	}
}

func TestPlanExplicitIDKeepsSelectedMalformedArtifactProblem(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeRawArtifact(t, target, "drift", "drift-selected-bad.json", `{"version":1,"id":"drift-selected-bad","session_id":"session-one","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	receipt, err := Plan(Options{Profile: p, IDs: []string{"drift-selected-bad"}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan() error = %v, want nil receipt refusal", err)
	}

	if receipt.Summary.ArtifactProblems != 1 || receipt.Summary.Refused != 1 {
		t.Fatalf("Plan().Summary = %+v, want selected artifact problem refusal", receipt.Summary)
	}
	if len(receipt.ArtifactProblems) != 1 || !strings.Contains(receipt.ArtifactProblems[0].Path, "drift-selected-bad.json") {
		t.Fatalf("Plan().ArtifactProblems = %#v, want selected malformed drift problem", receipt.ArtifactProblems)
	}
	if receipt.Refusals[0].ReasonCode != ReasonArtifactProblems {
		t.Fatalf("Plan() refusal = %+v, want artifact-problems refusal", receipt.Refusals[0])
	}
}

func TestReceiptDeterminism(t *testing.T) {
	p, target := setupReconcileFixture(t)
	writeSourceFile(t, p, "b.txt", []byte("bbb"), 0o644)
	writeSourceFile(t, p, "a.txt", []byte("aaa"), 0o644)
	driftB := missingFileDrift("drift-b", "b.txt", []byte("bbb"))
	driftA := missingFileDrift("drift-a", "a.txt", []byte("aaa"))
	writePublishedSession(t, target, driftA.SessionID, p.ProfileID, p.Target.TargetID, driftA.RootID, []control.ManifestEntry{
		manifestEntry("b.txt", []byte("bbb")),
		manifestEntry("a.txt", []byte("aaa")),
	})
	writeTargetDrift(t, target, driftB)
	writeTargetDrift(t, target, driftA)

	first, err := Plan(Options{Profile: p, IDs: []string{"drift-b", "drift-a"}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan(first) error = %v, want nil", err)
	}
	second, err := Plan(Options{Profile: p, IDs: []string{"drift-a", "drift-b"}, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Plan(second) error = %v, want nil", err)
	}
	firstJSON, err := CanonicalJSON(first)
	if err != nil {
		t.Fatalf("CanonicalJSON(first) error = %v, want nil", err)
	}
	secondJSON, err := CanonicalJSON(second)
	if err != nil {
		t.Fatalf("CanonicalJSON(second) error = %v, want nil", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("plan receipts are not deterministic\nfirst:\n%s\nsecond:\n%s", firstJSON, secondJSON)
	}
	if first.Actions[0].Path != "a.txt" || first.Actions[1].Path != "b.txt" {
		t.Fatalf("action order = %+v, want path order", first.Actions)
	}
}

func TestClassifyRefusesControlPlaneTargetPath(t *testing.T) {
	p, target := setupReconcileFixture(t)
	drift := missingFileDrift("drift-control", ".supermover/sessions/forged/receipt.json", []byte("payload"))

	item := classify(target, p, drift, nil, fixedNow(), false)

	if !item.refused {
		t.Fatalf("classify(control path) refused = false, want refusal")
	}
	if item.refusal.ReasonCode != ReasonControlPlanePath {
		t.Fatalf("classify(control path) refusal = %+v, want control-plane path refusal", item.refusal)
	}
}

func setupReconcileFixture(t *testing.T) (profile.Profile, string) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v, want nil", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.TargetID = "target-local"
	return p, target
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
}

func missingFileDrift(id string, targetPath string, data []byte) control.TargetDrift {
	expected := control.TargetDriftExpectedState{
		SessionID:  "session-one",
		ManifestID: "manifest-session-one",
		Kind:       "file",
		Path:       filepath.ToSlash(targetPath),
		Digest:     digest(data),
		ModTime:    "2026-05-18T00:00:00Z",
	}
	expected.SetSizeEvidence(int64(len(data)))
	expected.SetModeEvidence(0o644)
	missing := false
	return control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         id,
		SessionID:  "session-one",
		ProfileID:  "profile-local",
		TargetID:   "target-local",
		RootID:     "root",
		Path:       filepath.ToSlash(targetPath),
		DetectedAt: "2026-05-20T00:00:00Z",
		Change:     "missing",
		Expected:   expected,
		Observed: control.TargetDriftObservedState{
			Present: &missing,
			Kind:    "missing",
			Path:    filepath.ToSlash(targetPath),
		},
		ReviewState: "needs_review",
		Evidence:    []string{"target path is missing"},
	}
}

func manifestEntry(targetPath string, data []byte) control.ManifestEntry {
	entry := control.ManifestEntry{
		Path:       filepath.ToSlash(targetPath),
		TargetPath: filepath.ToSlash(targetPath),
		Kind:       "file",
		Digest:     digest(data),
		ModTime:    "2026-05-18T00:00:00Z",
	}
	entry.SetSizeEvidence(int64(len(data)))
	entry.SetModeEvidence(0o644)
	return entry
}

func writePublishedSession(t *testing.T, target string, sessionID string, profileID string, targetID string, rootID string, entries []control.ManifestEntry) {
	t.Helper()
	manifestPath, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest %q) error = %v, want nil", sessionID, err)
	}
	if err := control.WriteFile(manifestPath, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    rootID,
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   entries,
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", manifestPath, err)
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
	}
	if err := control.WriteFile(receiptPath, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: profileID,
		TargetID:  targetID,
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", receiptPath, err)
	}
}

func writeTargetDrift(t *testing.T, target string, drift control.TargetDrift) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, drift.ID)
	if err != nil {
		t.Fatalf("control.Path(target drift %q) error = %v, want nil", drift.ID, err)
	}
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("control.WriteFile(%q, drift) error = %v, want nil", path, err)
	}
}

func writeRawArtifact(t *testing.T, target string, dir string, name string, data string) {
	t.Helper()
	path := filepath.Join(target, control.DirName, dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
}

func readTargetDrift(t *testing.T, target string, id string) control.TargetDrift {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, id)
	if err != nil {
		t.Fatalf("control.Path(target drift %q) error = %v, want nil", id, err)
	}
	drift, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return drift
}

func targetDriftPath(t *testing.T, target string, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, id)
	if err != nil {
		t.Fatalf("control.Path(target drift %q) error = %v, want nil", id, err)
	}
	return path
}

func writeSourceFile(t *testing.T, p profile.Profile, rel string, data []byte, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(p.Roots[0].Path, filepath.FromSlash(rel))
	writeFileWithModTime(t, path, data, mode)
}

func writeTargetFile(t *testing.T, target string, rel string, data []byte, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(target, filepath.FromSlash(rel))
	writeFileWithModTime(t, path, data, mode)
}

func writeFileWithModTime(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
	modTime, err := time.Parse(time.RFC3339Nano, "2026-05-18T00:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse(modtime) error = %v, want nil", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%q) error = %v, want nil", path, err)
	}
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		line := filepath.ToSlash(rel) + "|" + info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			line += "|" + digest(data)
		}
		entries = append(entries, line)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%q) error = %v, want nil", root, err)
	}
	return strings.Join(entries, "\n")
}

func assertTreeUnchanged(t *testing.T, root string, before string) {
	t.Helper()
	after := snapshotTree(t, root)
	if after != before {
		t.Fatalf("target tree changed\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func readFileDigest(t *testing.T, path string) string {
	t.Helper()
	data := readRawFile(t, path)
	return digest(data)
}

func readRawFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	return data
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestCanonicalJSONUsesStableObjectEncoding(t *testing.T) {
	receipt := Receipt{Schema: SchemaPlanReceipt, TargetRoot: "/target", ProfileID: "p", TargetID: "t", GeneratedAt: fixedNow().Format(time.RFC3339Nano)}
	got, err := CanonicalJSON(receipt)
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v, want nil", err)
	}
	var decoded Receipt
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(CanonicalJSON()) error = %v, want nil", err)
	}
	if decoded.Schema != receipt.Schema || decoded.GeneratedAt != receipt.GeneratedAt {
		t.Fatalf("CanonicalJSON decoded = %+v, want %+v", decoded, receipt)
	}
}
