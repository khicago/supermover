package durable

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPromoteFileRenamesSyncedTempToFinal(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "nested", "file.txt")

	if err := os.WriteFile(tempPath, []byte("durable payload"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
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

func TestPromoteFileNoReplaceCreatesFinal(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "file.tmp")
	finalPath := filepath.Join(dir, "nested", "file.txt")

	if err := os.WriteFile(tempPath, []byte("durable payload"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", tempPath, err)
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

	if err := PromoteFileNoReplace(tempPath, finalPath); err == nil {
		t.Fatalf("PromoteFileNoReplace(%q, %q) error = nil, want existing final error", tempPath, finalPath)
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
