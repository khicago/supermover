package pathguard

import (
	"errors"
	"os"
	"path/filepath"
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
