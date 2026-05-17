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

	tests := []struct {
		name string
		rel  string
	}{
		{name: "parent traversal", rel: "../a.txt"},
		{name: "absolute path", rel: "/tmp/a.txt"},
		{name: "windows absolute volume", rel: "C:/tmp/a.txt"},
		{name: "windows drive relative", rel: "C:tmp/a.txt"},
		{name: "windows unc", rel: "//server/share"},
		{name: "backslash path", rel: `docs\a.txt`},
		{name: "clean current directory", rel: "."},
		{name: "nested traversal escapes", rel: "safe/../../a.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SafeJoin(root, tt.rel)
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("SafeJoin(%q, %q) error = %v, want ErrUnsafePath", root, tt.rel, err)
			}
		})
	}
}

func TestSafeJoinAllowsHiddenDataBelowRoot(t *testing.T) {
	tests := []string{
		".env",
		".config/settings.json",
		filepath.ToSlash(filepath.Join("docs", ".hidden", "file.txt")),
	}

	for _, rel := range tests {
		t.Run(rel, func(t *testing.T) {
			root := t.TempDir()
			got, err := SafeJoin(root, rel)
			if err != nil {
				t.Fatalf("SafeJoin(%q, %q) error = %v, want nil", root, rel, err)
			}
			want := filepath.Join(root, filepath.FromSlash(rel))
			if got != want {
				t.Fatalf("SafeJoin(%q, %q) = %q, want %q", root, rel, got, want)
			}
		})
	}
}

func TestValidateSlashRelativePathLength(t *testing.T) {
	if err := ValidateSlashRelativePath("abcd", 3); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("ValidateSlashRelativePath(too long) error = %v, want ErrUnsafePath", err)
	}
	if err := ValidateSlashRelativePath("abcd", 4); err != nil {
		t.Fatalf("ValidateSlashRelativePath(max length) error = %v, want nil", err)
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
	tests := []struct {
		name   string
		target string
	}{
		{name: "empty", target: ""},
		{name: "blank", target: " \t\n"},
		{name: "absolute slash", target: "/abs"},
		{name: "absolute backslash", target: `\abs`},
		{name: "backslash component", target: `bad\path`},
		{name: "parent traversal", target: "../outside"},
		{name: "interior traversal", target: "a/../b"},
		{name: "dot segment", target: "./file"},
		{name: "empty segment", target: "a//b"},
		{name: "windows absolute volume", target: "C:/Users/example"},
		{name: "windows drive relative", target: "c:relative"},
		{name: "windows unc", target: "//server/share"},
		{name: "reserved control dir", target: ".supermover/receipt.json"},
		{name: "reserved control dir case insensitive", target: ".Supermover/receipt.json"},
		{name: "too long", target: strings.Repeat("a", MaxSymlinkTargetLen+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRelativeSymlinkTarget(tt.target); !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("ValidateRelativeSymlinkTarget(%q) error = %v, want ErrUnsafePath", tt.target, err)
			}
		})
	}
}

func TestValidateRelativeSymlinkTargetAcceptsSafeRelativeValues(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "nested file", target: "dir/file.txt"},
		{name: "hidden source data", target: "dir/.hidden/file.txt"},
		{name: "control name below data dir", target: "docs/.supermover/file.txt"},
		{name: "maximum length", target: strings.Repeat("a", MaxSymlinkTargetLen)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRelativeSymlinkTarget(tt.target); err != nil {
				t.Fatalf("ValidateRelativeSymlinkTarget(%q) error = %v, want nil", tt.target, err)
			}
		})
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

func TestEnsurePlainDirectoryCreatesPlainParents(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sessions", "session-1", "stage")

	if err := EnsurePlainDirectory(root, dir, 0o755); err != nil {
		t.Fatalf("EnsurePlainDirectory(%q, %q) error = %v, want nil", root, dir, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatalf("os.Lstat(%q) error = %v, want nil", dir, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("os.Lstat(%q) mode = %v, want plain directory", dir, info.Mode())
	}
}

func TestEnsurePlainDirectoryRejectsUnsafeDirectories(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string) (string, string)
	}{
		{
			name: "path escapes root",
			setup: func(t *testing.T, root string) (string, string) {
				return root, filepath.Join(filepath.Dir(root), "outside")
			},
		},
		{
			name: "root is symlink",
			setup: func(t *testing.T, root string) (string, string) {
				realRoot := filepath.Join(filepath.Dir(root), "real-root")
				if err := os.Mkdir(realRoot, 0o755); err != nil {
					t.Fatalf("os.Mkdir(%q) error = %v, want nil", realRoot, err)
				}
				linkRoot := filepath.Join(filepath.Dir(root), "link-root")
				if err := os.Symlink(realRoot, linkRoot); err != nil {
					t.Skipf("os.Symlink() unavailable: %v", err)
				}
				return linkRoot, filepath.Join(linkRoot, "child")
			},
		},
		{
			name: "file component",
			setup: func(t *testing.T, root string) (string, string) {
				filePath := filepath.Join(root, "not-a-dir")
				if err := os.WriteFile(filePath, []byte("file"), 0o644); err != nil {
					t.Fatalf("os.WriteFile(%q) error = %v, want nil", filePath, err)
				}
				return root, filepath.Join(filePath, "child")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			root, dir := tt.setup(t, root)

			err := EnsurePlainDirectory(root, dir, 0o755)
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("EnsurePlainDirectory(%q, %q) error = %v, want ErrUnsafePath", root, dir, err)
			}
		})
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

func TestSafeJoinDirectoryChecksDirectoryAndRejectsEscapes(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name    string
		rel     string
		setup   func(t *testing.T, root string)
		wantErr bool
	}{
		{
			name: "existing nested directory",
			rel:  "a/b",
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(existing dir) error = %v, want nil", err)
				}
			},
		},
		{name: "reject traversal", rel: "../outside", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t, root)
			}
			got, err := SafeJoinDirectory(root, tt.rel)
			if tt.wantErr {
				if !errors.Is(err, ErrUnsafePath) {
					t.Fatalf("SafeJoinDirectory(%q, %q) error = %v, want ErrUnsafePath", root, tt.rel, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SafeJoinDirectory(%q, %q) error = %v, want nil", root, tt.rel, err)
			}
			info, err := os.Lstat(got)
			if err != nil {
				t.Fatalf("os.Lstat(%q) error = %v, want nil", got, err)
			}
			if !info.IsDir() {
				t.Fatalf("os.Lstat(%q).IsDir() = false, want true", got)
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
