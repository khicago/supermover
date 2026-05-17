package durable

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPromoteFileRenamesSyncedTempToFinal(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "nested", "file.txt")

	if err := os.WriteFile(tempPath, []byte("durable payload"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(finalPath), err)
	}

	if err := PromoteFile(tempPath, finalPath); err != nil {
		t.Fatalf("PromoteFile(%q, %q) error = %v, want nil", tempPath, finalPath, err)
	}

	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", finalPath, err)
	}
	if string(got) != "durable payload" {
		t.Errorf("PromoteFile(%q, %q) content = %q, want %q", tempPath, finalPath, string(got), "durable payload")
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", tempPath, err)
	}
}

func TestPromoteFileErrorPathsRetainTemp(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string) (tempPath string, finalPath string)
		wantOp    string
		wantPath  func(tempPath string, finalPath string) string
		wantExist bool
	}{
		{
			name: "missing parent",
			setup: func(t *testing.T, dir string) (string, string) {
				tempPath := filepath.Join(dir, "file.tmp")
				if err := os.WriteFile(tempPath, []byte("payload"), 0o644); err != nil {
					t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
				}
				return tempPath, filepath.Join(dir, "missing", "file.txt")
			},
			wantOp:    "rename",
			wantPath:  func(_ string, finalPath string) string { return finalPath },
			wantExist: true,
		},
		{
			name: "missing temp",
			setup: func(t *testing.T, dir string) (string, string) {
				return filepath.Join(dir, "missing.tmp"), filepath.Join(dir, "file.txt")
			},
			wantOp:    "open for sync",
			wantPath:  func(tempPath string, _ string) string { return tempPath },
			wantExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tempPath, finalPath := tt.setup(t, dir)

			err := PromoteFile(tempPath, finalPath)
			if err == nil {
				t.Fatalf("PromoteFile(%q, %q) error = nil, want error", tempPath, finalPath)
			}
			var durableErr *Error
			if !errors.As(err, &durableErr) {
				t.Fatalf("PromoteFile(%q, %q) error = %T, want *Error", tempPath, finalPath, err)
			}
			if durableErr.Op != tt.wantOp {
				t.Fatalf("PromoteFile(%q, %q) op = %q, want %q", tempPath, finalPath, durableErr.Op, tt.wantOp)
			}
			if wantPath := tt.wantPath(tempPath, finalPath); durableErr.Path != wantPath {
				t.Fatalf("PromoteFile(%q, %q) path = %q, want %q", tempPath, finalPath, durableErr.Path, wantPath)
			}
			if _, err := os.Stat(finalPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", finalPath, err)
			}
			_, err = os.Stat(tempPath)
			if tt.wantExist && err != nil {
				t.Fatalf("os.Stat(%q) error = %v, want temp retained", tempPath, err)
			}
			if !tt.wantExist && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", tempPath, err)
			}
		})
	}
}

func TestPromoteFileNoReplaceCreatesFinal(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "nested", "file.txt")

	if err := os.WriteFile(tempPath, []byte("durable payload"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(finalPath), err)
	}

	if err := PromoteFileNoReplace(tempPath, finalPath); err != nil {
		t.Fatalf("PromoteFileNoReplace(%q, %q) error = %v, want nil", tempPath, finalPath, err)
	}

	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", finalPath, err)
	}
	if string(got) != "durable payload" {
		t.Errorf("PromoteFileNoReplace(%q, %q) content = %q, want %q", tempPath, finalPath, string(got), "durable payload")
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", tempPath, err)
	}
}

func TestPromoteFileNoReplaceRefusesExistingFinal(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(tempPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
	}
	if err := os.WriteFile(finalPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", finalPath, err)
	}

	err := PromoteFileNoReplace(tempPath, finalPath)
	if err == nil {
		t.Fatalf("PromoteFileNoReplace(%q, %q) error = nil, want existing final error", tempPath, finalPath)
	}
	var durableErr *Error
	if !errors.As(err, &durableErr) {
		t.Fatalf("PromoteFileNoReplace(%q, %q) error = %T, want *Error", tempPath, finalPath, err)
	}
	if durableErr.Op != "link without replace" {
		t.Fatalf("PromoteFileNoReplace(%q, %q) op = %q, want link without replace", tempPath, finalPath, durableErr.Op)
	}
	if durableErr.Path != finalPath {
		t.Fatalf("PromoteFileNoReplace(%q, %q) path = %q, want final path", tempPath, finalPath, durableErr.Path)
	}
	if got := ClassifyError(err); got != StatusIOError {
		t.Fatalf("ClassifyError(PromoteFileNoReplace existing final) = %q, want %q", got, StatusIOError)
	}
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", finalPath, err)
	}
	if string(got) != "existing" {
		t.Fatalf("PromoteFileNoReplace(%q, %q) final content = %q, want existing", tempPath, finalPath, got)
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want temp retained", tempPath, err)
	}
}

func TestPromoteFileNoReplaceErrorPathsRetainTemp(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string) (tempPath string, finalPath string)
		wantOp    string
		wantPath  func(tempPath string, finalPath string) string
		wantExist bool
	}{
		{
			name: "missing parent",
			setup: func(t *testing.T, dir string) (string, string) {
				tempPath := filepath.Join(dir, "file.tmp")
				if err := os.WriteFile(tempPath, []byte("payload"), 0o644); err != nil {
					t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
				}
				return tempPath, filepath.Join(dir, "missing", "file.txt")
			},
			wantOp:    "link without replace",
			wantPath:  func(_ string, finalPath string) string { return finalPath },
			wantExist: true,
		},
		{
			name: "missing temp",
			setup: func(t *testing.T, dir string) (string, string) {
				return filepath.Join(dir, "missing.tmp"), filepath.Join(dir, "file.txt")
			},
			wantOp:    "open for sync",
			wantPath:  func(tempPath string, _ string) string { return tempPath },
			wantExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tempPath, finalPath := tt.setup(t, dir)

			err := PromoteFileNoReplace(tempPath, finalPath)
			if err == nil {
				t.Fatalf("PromoteFileNoReplace(%q, %q) error = nil, want error", tempPath, finalPath)
			}
			var durableErr *Error
			if !errors.As(err, &durableErr) {
				t.Fatalf("PromoteFileNoReplace(%q, %q) error = %T, want *Error", tempPath, finalPath, err)
			}
			if durableErr.Op != tt.wantOp {
				t.Fatalf("PromoteFileNoReplace(%q, %q) op = %q, want %q", tempPath, finalPath, durableErr.Op, tt.wantOp)
			}
			if wantPath := tt.wantPath(tempPath, finalPath); durableErr.Path != wantPath {
				t.Fatalf("PromoteFileNoReplace(%q, %q) path = %q, want %q", tempPath, finalPath, durableErr.Path, wantPath)
			}
			if _, err := os.Stat(finalPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", finalPath, err)
			}
			_, err = os.Stat(tempPath)
			if tt.wantExist && err != nil {
				t.Fatalf("os.Stat(%q) error = %v, want temp retained", tempPath, err)
			}
			if !tt.wantExist && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist", tempPath, err)
			}
		})
	}
}

func TestPromoteFileNoReplaceReportsTempCleanupFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory write-bit removal semantics are not portable on windows")
	}
	dir := t.TempDir()
	tempDir := filepath.Join(dir, "temp")
	finalDir := filepath.Join(dir, "final")
	if err := os.Mkdir(tempDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", tempDir, err)
	}
	if err := os.Mkdir(finalDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", finalDir, err)
	}
	tempPath := filepath.Join(tempDir, "file.tmp")
	probePath := filepath.Join(tempDir, "probe.tmp")
	finalPath := filepath.Join(finalDir, "file.txt")
	for _, path := range []string{tempPath, probePath} {
		if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
		}
	}
	if err := os.Chmod(tempDir, 0o555); err != nil {
		t.Fatalf("os.Chmod(%q) error = %v, want nil", tempDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(tempDir, 0o755); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("restore os.Chmod(%q) error = %v, want nil", tempDir, err)
		}
	})
	if err := os.Remove(probePath); err == nil {
		t.Skip("filesystem allowed removal from non-writable directory")
	}

	err := PromoteFileNoReplace(tempPath, finalPath)
	if err == nil {
		t.Fatalf("PromoteFileNoReplace(%q, %q) error = nil, want cleanup failure", tempPath, finalPath)
	}
	var durableErr *Error
	if !errors.As(err, &durableErr) {
		t.Fatalf("PromoteFileNoReplace(%q, %q) error = %T, want *Error", tempPath, finalPath, err)
	}
	if durableErr.Op != "remove temp" {
		t.Fatalf("PromoteFileNoReplace(%q, %q) op = %q, want remove temp", tempPath, finalPath, durableErr.Op)
	}
	if durableErr.Path != tempPath {
		t.Fatalf("PromoteFileNoReplace(%q, %q) path = %q, want temp path", tempPath, finalPath, durableErr.Path)
	}
	got, readErr := os.ReadFile(finalPath)
	if readErr != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want final linked before cleanup failure", finalPath, readErr)
	}
	if string(got) != "payload" {
		t.Fatalf("os.ReadFile(%q) = %q, want payload", finalPath, got)
	}
	if _, statErr := os.Stat(tempPath); statErr != nil {
		t.Fatalf("os.Stat(%q) error = %v, want temp retained after cleanup failure", tempPath, statErr)
	}
}

func TestCopyFileNoReplaceRefusesExistingFinal(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(tempPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
	}
	if err := os.WriteFile(finalPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", finalPath, err)
	}

	if err := copyFileNoReplace(tempPath, finalPath); err == nil {
		t.Fatalf("copyFileNoReplace(%q, %q) error = nil, want existing final error", tempPath, finalPath)
	}
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", finalPath, err)
	}
	if string(got) != "existing" {
		t.Fatalf("copyFileNoReplace(%q, %q) final content = %q, want existing", tempPath, finalPath, got)
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want temp retained", tempPath, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", dir, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".supermover-promote-") {
			t.Fatalf("copyFileNoReplace(%q, %q) left temporary file %q after link failure", tempPath, finalPath, entry.Name())
		}
	}
}

func TestCopyFileNoReplacePreservesTempPermissions(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(tempPath, []byte("secret"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
	}

	if err := copyFileNoReplace(tempPath, finalPath); err != nil {
		t.Fatalf("copyFileNoReplace(%q, %q) error = %v, want nil", tempPath, finalPath, err)
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", finalPath, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("copyFileNoReplace(%q, %q) mode = %v, want 0600", tempPath, finalPath, info.Mode().Perm())
	}
}

func TestCopyFileNoReplacePreservesTempModTime(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "file.txt")
	want := time.Date(2026, 5, 16, 8, 0, 0, 123, time.UTC)

	if err := os.WriteFile(tempPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
	}
	if err := os.Chtimes(tempPath, want, want); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", tempPath, err)
	}

	if err := copyFileNoReplace(tempPath, finalPath); err != nil {
		t.Fatalf("copyFileNoReplace(%q, %q) error = %v, want nil", tempPath, finalPath, err)
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", finalPath, err)
	}
	if !info.ModTime().Equal(want) {
		t.Fatalf("copyFileNoReplace(%q, %q) mtime = %s, want %s", tempPath, finalPath, info.ModTime().Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
}

func TestCopyFileNoReplaceUsesTemporaryFinal(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(tempPath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
	}

	if err := copyFileNoReplace(tempPath, finalPath); err != nil {
		t.Fatalf("copyFileNoReplace(%q, %q) error = %v, want nil", tempPath, finalPath, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", dir, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".supermover-promote-") {
			t.Fatalf("copyFileNoReplace(%q, %q) left temporary file %q", tempPath, finalPath, entry.Name())
		}
	}
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", finalPath, err)
	}
	if string(got) != "payload" {
		t.Fatalf("copyFileNoReplace(%q, %q) content = %q, want payload", tempPath, finalPath, got)
	}
}

func TestPromoteFileValidationFailure(t *testing.T) {
	err := PromoteFile("", "final")
	if err == nil {
		t.Fatal(`PromoteFile("", "final") error = nil, want validation error`)
	}
	if got := ClassifyError(err); got != StatusValidationFailure {
		t.Errorf(`ClassifyError(PromoteFile("", "final")) = %q, want %q`, got, StatusValidationFailure)
	}
	if !errors.Is(err, ErrValidationFailure) {
		t.Errorf(`errors.Is(PromoteFile("", "final"), ErrValidationFailure) = false, want true`)
	}
}

func TestPromoteFileValidationFailures(t *testing.T) {
	tests := []struct {
		name      string
		tempPath  string
		finalPath string
	}{
		{name: "empty temp", tempPath: "", finalPath: "final"},
		{name: "empty final", tempPath: "temp", finalPath: ""},
		{name: "same clean path", tempPath: filepath.Join("dir", "file"), finalPath: filepath.Join("dir", ".", "file")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PromoteFile(tt.tempPath, tt.finalPath)
			if err == nil {
				t.Fatalf("PromoteFile(%q, %q) error = nil, want validation error", tt.tempPath, tt.finalPath)
			}
			if !errors.Is(err, ErrValidationFailure) {
				t.Fatalf("errors.Is(PromoteFile(%q, %q), ErrValidationFailure) = false, want true: %v", tt.tempPath, tt.finalPath, err)
			}
			if got := ClassifyError(err); got != StatusValidationFailure {
				t.Fatalf("ClassifyError(PromoteFile(%q, %q)) = %q, want %q", tt.tempPath, tt.finalPath, got, StatusValidationFailure)
			}
		})
	}
}

func TestPromoteFileNoReplaceValidationFailures(t *testing.T) {
	tests := []struct {
		name      string
		tempPath  string
		finalPath string
	}{
		{name: "empty temp", tempPath: "", finalPath: "final"},
		{name: "empty final", tempPath: "temp", finalPath: ""},
		{name: "same clean path", tempPath: filepath.Join("dir", "file"), finalPath: filepath.Join("dir", ".", "file")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PromoteFileNoReplace(tt.tempPath, tt.finalPath)
			if err == nil {
				t.Fatalf("PromoteFileNoReplace(%q, %q) error = nil, want validation error", tt.tempPath, tt.finalPath)
			}
			if !errors.Is(err, ErrValidationFailure) {
				t.Fatalf("errors.Is(PromoteFileNoReplace(%q, %q), ErrValidationFailure) = false, want true: %v", tt.tempPath, tt.finalPath, err)
			}
			if got := ClassifyError(err); got != StatusValidationFailure {
				t.Fatalf("ClassifyError(PromoteFileNoReplace(%q, %q)) = %q, want %q", tt.tempPath, tt.finalPath, got, StatusValidationFailure)
			}
		})
	}
}

func TestDurableErrorFormatsAndUnwraps(t *testing.T) {
	withPath := &Error{Status: StatusIOError, Op: "sync file", Path: "payload.bin", Err: os.ErrNotExist}
	if got := withPath.Error(); got != "sync file payload.bin: file does not exist" {
		t.Fatalf("Error() with path = %q, want sync file payload.bin: file does not exist", got)
	}
	if !errors.Is(withPath, os.ErrNotExist) {
		t.Fatalf("errors.Is(Error, os.ErrNotExist) = false, want true")
	}

	withoutPath := &Error{Status: StatusValidationFailure, Op: "promote", Err: ErrValidationFailure}
	if got := withoutPath.Error(); got != "promote: validation failure" {
		t.Fatalf("Error() without path = %q, want promote: validation failure", got)
	}
	if !errors.Is(withoutPath, ErrValidationFailure) {
		t.Fatalf("errors.Is(Error, ErrValidationFailure) = false, want true")
	}
}

func TestWrapClassifiesAndPreservesSentinels(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		want     Status
		sentinel error
	}{
		{name: "nil", err: nil, want: StatusOK},
		{name: "disk full", err: syscall.ENOSPC, want: StatusDiskFull, sentinel: ErrDiskFull},
		{name: "interrupted", err: syscall.EINTR, want: StatusInterrupted, sentinel: ErrInterrupted},
		{name: "invalid", err: fs.ErrInvalid, want: StatusValidationFailure, sentinel: ErrValidationFailure},
		{name: "io", err: os.ErrNotExist, want: StatusIOError, sentinel: os.ErrNotExist},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := wrap("write", "target", tt.err)
			if tt.err == nil {
				if err != nil {
					t.Fatalf("wrap(nil) error = %v, want nil", err)
				}
				return
			}
			var durableErr *Error
			if !errors.As(err, &durableErr) {
				t.Fatalf("wrap(%v) error = %T, want *Error", tt.err, err)
			}
			if durableErr.Status != tt.want {
				t.Fatalf("wrap(%v).Status = %q, want %q", tt.err, durableErr.Status, tt.want)
			}
			if durableErr.Op != "write" || durableErr.Path != "target" {
				t.Fatalf("wrap(%v) = op/path %q/%q, want write/target", tt.err, durableErr.Op, durableErr.Path)
			}
			if !errors.Is(err, tt.sentinel) {
				t.Fatalf("errors.Is(wrap(%v), %v) = false, want true", tt.err, tt.sentinel)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Status
	}{
		{name: "nil", err: nil, want: StatusOK},
		{name: "disk full", err: syscall.ENOSPC, want: StatusDiskFull},
		{name: "quota", err: syscall.EDQUOT, want: StatusDiskFull},
		{name: "interrupted", err: syscall.EINTR, want: StatusInterrupted},
		{name: "validation", err: ErrValidationFailure, want: StatusValidationFailure},
		{name: "other", err: os.ErrNotExist, want: StatusIOError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got != tt.want {
				t.Errorf("ClassifyError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
