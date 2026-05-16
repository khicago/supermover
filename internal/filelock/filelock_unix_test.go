//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLockRejectsSymlinkLeaf(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.lock")
	link := filepath.Join(dir, "link.lock")
	if err := os.WriteFile(target, []byte("lock"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	unlock, err := Lock(link)
	if err == nil {
		unlock()
		t.Fatalf("Lock(%q) error = nil, want symlink rejection", link)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lock(%q) error = %v, want symlink rejection not missing parent", link, err)
	}
}

func TestLockInDirRejectsSymlinkDirectory(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(dir, "locks")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	unlock, err := LockInDir(link, "target.lock")
	if err == nil {
		unlock()
		t.Fatalf("LockInDir(%q, target.lock) error = nil, want symlink directory rejection", link)
	}
	if _, err := os.Stat(filepath.Join(outside, "target.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(outside lock) error = %v, want os.ErrNotExist", err)
	}
}
