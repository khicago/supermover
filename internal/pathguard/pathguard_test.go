package pathguard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoinParentRejectsSymlinkParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	_, err := SafeJoinParent(root, "docs/a.txt")
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("SafeJoinParent(%q, docs/a.txt) error = %v, want ErrUnsafePath", root, err)
	}
}

func TestSafeJoinRejectsEscapes(t *testing.T) {
	root := t.TempDir()

	tests := []string{"../a.txt", "/tmp/a.txt", "."}
	for _, rel := range tests {
		t.Run(rel, func(t *testing.T) {
			_, err := SafeJoin(root, rel)
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("SafeJoin(%q, %q) error = %v, want ErrUnsafePath", root, rel, err)
			}
		})
	}
}

func TestIsReservedControlPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: ".supermover", want: true},
		{path: ".supermover/sessions/session-1/receipt.json", want: true},
		{path: ".Supermover/warnings/w1.json", want: true},
		{path: "safe/../.supermover/sessions/session-1/receipt.json", want: true},
		{path: "safe/../.Supermover/warnings/w1.json", want: true},
		{path: "docs/.supermover/file.txt", want: false},
		{path: ".supermover-backup/file.txt", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsReservedControlPath(tt.path)
			if got != tt.want {
				t.Fatalf("IsReservedControlPath(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidateRelativeSymlinkTargetRejectsUnsafeValues(t *testing.T) {
	for _, target := range []string{"", "/abs", `bad\path`, "../outside", "a/../b", "./file", "a//b", "C:/Users/example", "c:relative", "//server/share", ".supermover/receipt.json", ".Supermover/receipt.json"} {
		t.Run(target, func(t *testing.T) {
			if err := ValidateRelativeSymlinkTarget(target); !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("ValidateRelativeSymlinkTarget(%q) error = %v, want ErrUnsafePath", target, err)
			}
		})
	}
}

func TestValidateRelativeSymlinkTargetAcceptsSafeRelativeValue(t *testing.T) {
	if err := ValidateRelativeSymlinkTarget("dir/file.txt"); err != nil {
		t.Fatalf("ValidateRelativeSymlinkTarget(dir/file.txt) error = %v, want nil", err)
	}
}

func TestEnsurePlainDirectoryRejectsSymlinkComponent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	err := EnsurePlainDirectory(root, filepath.Join(root, "link", "child"), 0o755)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("EnsurePlainDirectory(symlink component) error = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(outside child) error = %v, want os.ErrNotExist", err)
	}
}

func TestEnsurePlainDirectoryRejectsSymlinkComponentWithExistingTargetSubtree(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "sessions", "session-1", "stage"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(outside subtree) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ReservedControlDir)); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	err := EnsurePlainDirectory(root, filepath.Join(root, ReservedControlDir, "sessions", "session-1", "stage"), 0o755)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("EnsurePlainDirectory(existing subtree below symlink) error = %v, want ErrUnsafePath", err)
	}
}

func TestCanonicalPathResolvesExistingSymlinkPrefix(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", real, err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	got, err := CanonicalPath(filepath.Join(link, "future", "file.txt"))
	if err != nil {
		t.Fatalf("CanonicalPath(%q) error = %v, want nil", link, err)
	}
	realCanonical, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v, want nil", real, err)
	}
	want := filepath.Join(realCanonical, "future", "file.txt")
	if got != want {
		t.Fatalf("CanonicalPath(%q) = %q, want %q", link, got, want)
	}
}

func TestCanonicalPathFallsBackToCleanAbsWhenNoPrefixExists(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "missing", "..", "future")

	got, err := CanonicalPath(input)
	if err != nil {
		t.Fatalf("CanonicalPath(%q) error = %v, want nil", input, err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("CanonicalPath(%q) = %q, want absolute path", input, got)
	}
	if strings.Contains(got, "..") {
		t.Fatalf("CanonicalPath(%q) = %q, want cleaned path", input, got)
	}
}
