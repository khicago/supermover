package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLayoutEnsureSessionDirs(t *testing.T) {
	layout := NewLayout(t.TempDir())

	if err := layout.EnsureSessionDirs("session-1"); err != nil {
		t.Fatalf(`Layout.EnsureSessionDirs("session-1") error = %v, want nil`, err)
	}

	if info, err := os.Stat(layout.StagingDir("session-1")); err != nil || !info.IsDir() {
		t.Fatalf("os.Stat(%q) = (%v, %v), want directory", layout.StagingDir("session-1"), info, err)
	}
	if got, want := layout.RecordPath("session-1"), filepath.Join(layout.ControlDir, "sessions", "session-1", "session.json"); got != want {
		t.Errorf(`Layout.RecordPath("session-1") = %q, want %q`, got, want)
	}
}

func TestLayoutEnsureSessionDirsRejectsControlSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".supermover")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	layout := NewLayout(filepath.Join(root, ".supermover"))

	if err := layout.EnsureSessionDirs("session-1"); err == nil {
		t.Fatalf(`Layout.EnsureSessionDirs("session-1") error = nil, want symlink directory error`)
	}
	if _, err := os.Stat(filepath.Join(outside, "sessions", "session-1", "stage")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(outside stage) error = %v, want os.ErrNotExist", err)
	}
}

func TestSessionRecordRoundTrip(t *testing.T) {
	layout := NewLayout(t.TempDir())
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)

	record, err := NewSessionRecord("session-1", now)
	if err != nil {
		t.Fatalf(`NewSessionRecord("session-1", %v) error = %v, want nil`, now, err)
	}
	record, err = record.WithState(StateStaged, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(%q, %v) error = %v, want nil", StateStaged, now.Add(time.Minute), err)
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(%+v) error = %v, want nil", record, err)
	}

	got, err := ReadSessionRecord(layout.RecordPath(record.ID))
	if err != nil {
		t.Fatalf("ReadSessionRecord(%q) error = %v, want nil", layout.RecordPath(record.ID), err)
	}
	if got.ID != record.ID || got.State != record.State || !got.CreatedAt.Equal(record.CreatedAt) || !got.UpdatedAt.Equal(record.UpdatedAt) {
		t.Errorf("ReadSessionRecord(%q) = %+v, want %+v", layout.RecordPath(record.ID), got, record)
	}
}

func TestValidateSessionIDRejectsUnsafeValues(t *testing.T) {
	tests := []string{"", "../x", "a/b", `a\b`, "bad id"}
	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			err := ValidateSessionID(id)
			if !errors.Is(err, ErrValidation) {
				t.Errorf("ValidateSessionID(%q) error = %v, want ErrValidation", id, err)
			}
		})
	}
}
