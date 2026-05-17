package localpush

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/verify"
)

func TestSourceChangeDuringCopyBlocksStageAndStableRerunSucceeds(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourcePath := filepath.Join(source, "file.txt")
	stagePath := filepath.Join(dir, "stage", "file.txt")
	stamp := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	nextStamp := stamp.Add(time.Second)
	mustWriteFile(t, sourcePath, "before", 0o640)
	if err := os.Chtimes(sourcePath, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(source before) error = %v, want nil", err)
	}
	scanned, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	entry := scanEntryByPath(t, scanned, "file.txt")

	_, err = copyRegularToStageWithPostCopy(sourcePath, stagePath, entry, func() error {
		if err := os.WriteFile(sourcePath, []byte("after stable rerun payload"), 0o640); err != nil {
			return err
		}
		return os.Chtimes(sourcePath, nextStamp, nextStamp)
	})
	if err == nil || !strings.Contains(err.Error(), "changed during copy") {
		t.Fatalf("copyRegularToStageWithPostCopy(churned source) error = %v, want changed during copy", err)
	}
	if _, err := os.Stat(stagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(stage after churn) error = %v, want os.ErrNotExist", err)
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(stagePath), ".supermover-*.tmp"))
	if err != nil {
		t.Fatalf("filepath.Glob(stage temps) error = %v, want nil", err)
	}
	if len(temps) != 0 {
		t.Fatalf("stage temps after churn = %#v, want none", temps)
	}

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-stable", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("Run(stable rerun after source churn) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "after stable rerun payload" {
		t.Fatalf("target after stable rerun = (%q, %v), want changed payload", string(got), err)
	}
}

func TestRepeatedRerunAfterSourceChangeCarriesPreviousEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	oldTime := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes(old source) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: oldTime}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	previous := manifestEntryByPath(t, readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-one"), "file.txt")

	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new", 0o600)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(new source) error = %v, want nil", err)
	}
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: newTime.Add(time.Hour)}); err != nil {
		t.Fatalf("second Run(changed source) error = %v, want nil", err)
	}
	entry := manifestEntryByPath(t, readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-two"), "file.txt")
	if entry.PreviousSessionID != "session-one" || entry.PreviousManifestID != "manifest-session-one" {
		t.Fatalf("session-two previous ids = (%q, %q), want session-one/manifest-session-one", entry.PreviousSessionID, entry.PreviousManifestID)
	}
	if entry.PreviousSize != previous.Size || entry.PreviousDigest != previous.Digest || entry.PreviousMode != previous.Mode || entry.PreviousModTime != previous.ModTime {
		t.Fatalf("session-two previous evidence = %#v, want previous entry %#v", entry, previous)
	}
}

func TestChangedFileUpdateRefusesWithoutPreviousSupermoverEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, "session-one")
	if err != nil {
		t.Fatalf("control.Path(previous receipt) error = %v, want nil", err)
	}
	if err := os.Remove(receiptPath); err != nil {
		t.Fatalf("os.Remove(previous receipt) error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new", 0o644)

	_, err = Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("Run(changed source without previous receipt) error = %v, want overwrite refusal", err)
	}
	var drift targetDriftCause
	if errors.As(err, &drift) {
		t.Fatalf("Run(changed source without previous receipt) error = %v, want ordinary overwrite refusal without target drift cause", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("target after missing-evidence refusal = (%q, %v), want old", string(got), err)
	}
	assertNoReceipt(t, target, "session-two")
}

func TestTargetDriftRefusesChangedFileUpdate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(target, "file.txt"), "manual target drift", 0o644)
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("Run(changed source with target drift) error = %v, want overwrite refusal", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "manual target drift" {
		t.Fatalf("target after drift refusal = (%q, %v), want manual target drift", string(got), err)
	}
	assertNoReceipt(t, target, "session-two")
}

func TestLargeFileManifestAndVerify(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourcePath := filepath.Join(source, "large.bin")
	wantDigest, wantSize := writePatternFile(t, sourcePath, 12*1024*1024)
	stamp := time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC)
	if err := os.Chtimes(sourcePath, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(large source) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-large", Now: time.Date(2026, 5, 16, 4, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("Run(large file) error = %v, want nil", err)
	}
	entry := manifestEntryByPath(t, readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-large"), "large.bin")
	if entry.Size != wantSize || entry.Digest != wantDigest {
		t.Fatalf("large manifest entry = size %d digest %q, want size %d digest %q", entry.Size, entry.Digest, wantSize, wantDigest)
	}
	report, err := verify.BuildReport(verify.Options{TargetRoot: target, SessionID: "session-large", ProfileID: p.ProfileID, TargetID: p.Target.TargetID})
	if err != nil {
		t.Fatalf("verify.BuildReport(large file) error = %v, want nil", err)
	}
	if len(report.Findings) != 0 || report.Summary.FilesVerified != 1 {
		t.Fatalf("verify.BuildReport(large file) findings=%#v summary=%+v, want one verified file", report.Findings, report.Summary)
	}
}

func assertNoReceipt(t *testing.T, target string, sessionID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt %q) error = %v, want os.ErrNotExist", sessionID, err)
	}
}

func writePatternFile(t *testing.T, path string, size int64) (string, int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		t.Fatalf("os.OpenFile(%q) error = %v, want nil", path, err)
	}
	defer file.Close()
	hasher := sha256.New()
	chunk := make([]byte, 64*1024)
	for i := range chunk {
		chunk[i] = byte((i*31 + 17) % 251)
	}
	written := int64(0)
	for written < size {
		n := int64(len(chunk))
		if remaining := size - written; remaining < n {
			n = remaining
		}
		writeChunk(t, file, hasher, chunk[:n])
		written += n
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %q error = %v, want nil", path, err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), written
}

func writeChunk(t *testing.T, file *os.File, hasher hash.Hash, chunk []byte) {
	t.Helper()
	if _, err := file.Write(chunk); err != nil {
		t.Fatalf("write %q error = %v, want nil", file.Name(), err)
	}
	if _, err := hasher.Write(chunk); err != nil {
		t.Fatalf("hash chunk for %q error = %v, want nil", file.Name(), err)
	}
}

func TestWritePatternFileDigestUsesAllBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pattern.bin")
	gotDigest, gotSize := writePatternFile(t, path, 1024)
	if gotSize != 1024 {
		t.Fatalf("writePatternFile size = %d, want 1024", gotSize)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	sum := sha256.Sum256(bytes)
	wantDigest := "sha256:" + fmt.Sprintf("%x", sum[:])
	if gotDigest != wantDigest {
		t.Fatalf("writePatternFile digest = %q, want %q", gotDigest, wantDigest)
	}
}
