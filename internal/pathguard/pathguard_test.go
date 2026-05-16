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
