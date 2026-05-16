package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScanRecordsFilesystemMetadata(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "file.txt"), 0o644)
	mustWrite(t, filepath.Join(root, "script.sh"), 0o755)
	if err := os.Mkdir(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, ".hidden", "note"), 0o600)
	if err := os.Symlink("file.txt", filepath.Join(root, "link")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable on windows: %v", err)
		}
		t.Fatal(err)
	}

	result, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	entries := byPath(result.Entries)
	if entries["file.txt"].Kind != KindRegular {
		t.Fatalf("file kind = %q", entries["file.txt"].Kind)
	}
	if entries["script.sh"].Executable != true {
		t.Fatal("expected executable bit on script")
	}
	if entries[".hidden"].Hidden != true || entries[".hidden/note"].Hidden != true {
		t.Fatal("expected hidden dir and child to be hidden")
	}
	if entries["link"].Kind != KindSymlink || entries["link"].SymlinkTarget != "file.txt" {
		t.Fatalf("unexpected symlink entry: %#v", entries["link"])
	}
	if len(result.Audit) != 0 {
		t.Fatalf("unexpected audit records: %#v", result.Audit)
	}
}

func TestScanWarnsForSpecialFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipe test is unix-oriented")
	}
	root := t.TempDir()
	pipe := filepath.Join(root, "pipe")
	if err := unixMkfifo(pipe); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	result, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	entries := byPath(result.Entries)
	if entries["pipe"].Kind != KindSpecial {
		t.Fatalf("pipe kind = %q", entries["pipe"].Kind)
	}
	if len(result.Audit) != 1 {
		t.Fatalf("expected one audit record, got %#v", result.Audit)
	}
	if result.Audit[0].Kind != "special_file" {
		t.Fatalf("audit kind = %q", result.Audit[0].Kind)
	}
}

func mustWrite(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), mode); err != nil {
		t.Fatal(err)
	}
}

func byPath(entries []Entry) map[string]Entry {
	out := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		out[entry.Path] = entry
	}
	return out
}
