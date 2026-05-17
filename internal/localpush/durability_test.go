package localpush

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/scan"
)

func TestRestoreExistingDirsReportsFailures(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	err := restoreExistingDirs(map[string]existingDirMeta{
		missing: {
			Mode:    0o700,
			ModTime: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC),
		},
	})
	if err == nil {
		t.Fatalf("restoreExistingDirs(missing) error = nil, want visibility failure")
	}
	if !strings.Contains(err.Error(), "restore directory permissions") {
		t.Fatalf("restoreExistingDirs(missing) error = %q, want permissions restore context", err.Error())
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restoreExistingDirs(missing) error = %v, want os.ErrNotExist", err)
	}
}

func TestCopyRegularToStageRejectsSymlinkParentBeforeWritingOutside(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	stageParent := filepath.Join(dir, "stage", "nested")
	outside := filepath.Join(dir, "outside")
	stamp := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	mustWriteFile(t, source, "payload", 0o640)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(stageParent), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(stage root) error = %v, want nil", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(outside) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, stageParent); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	entry := scan.Entry{
		Path:    "nested/file.txt",
		Kind:    scan.KindRegular,
		Size:    int64(len("payload")),
		Mode:    0o640,
		ModTime: stamp,
	}

	_, err := copyRegularToStage(source, filepath.Join(stageParent, "file.txt"), entry)
	if !errors.Is(err, pathguard.ErrUnsafePath) {
		t.Fatalf("copyRegularToStage(symlink parent) error = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(outside staged file) error = %v, want os.ErrNotExist", err)
	}
}
