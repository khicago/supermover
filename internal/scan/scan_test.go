package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/khicago/supermover/internal/audit"
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
	if entries["file.txt"].Digest != "sha256:2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881" {
		t.Fatalf("file digest = %q, want sha256 for scanned content", entries["file.txt"].Digest)
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

func TestScanJSONDoesNotExposeObservedIdentity(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "file.txt"), 0o644)

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan(%q) error = %v, want nil", root, err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(Scan(%q)) error = %v, want nil", root, err)
	}

	var report struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("json.Unmarshal(Scan(%q)) error = %v, want nil", root, err)
	}
	if len(report.Entries) == 0 {
		t.Fatalf("json.Unmarshal(Scan(%q)).Entries length = 0, want scanned entries", root)
	}
	for _, entry := range report.Entries {
		if _, ok := entry["observed"]; ok {
			t.Fatalf("json.Marshal(Scan(%q)) entry keys = %#v, want no observed field", root, entry)
		}
		if _, ok := entry["Observed"]; ok {
			t.Fatalf("json.Marshal(Scan(%q)) entry keys = %#v, want no Observed field", root, entry)
		}
		if entry["kind"] == string(KindRegular) {
			if got, ok := entry["digest"].(string); !ok || !strings.HasPrefix(got, "sha256:") {
				t.Fatalf("json.Marshal(Scan(%q)) entry keys = %#v, want regular digest", root, entry)
			}
		}
	}
	if !strings.Contains(string(data), `"file.txt"`) {
		t.Fatalf("json.Marshal(Scan(%q)) = %s, want scanned file path", root, data)
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

func TestScanErrorRecordIDExcludesRuntimeErrorText(t *testing.T) {
	first := audit.WithDetected(
		audit.New("secret", "", audit.SeverityWarning, "scan_error", "walk error"),
		map[string]string{"error": "permission denied", "path": "/tmp/source/secret"},
	)
	second := audit.WithDetected(
		audit.New("secret", "", audit.SeverityWarning, "scan_error", "walk error"),
		map[string]string{"error": "operation not permitted", "path": "/tmp/source/secret"},
	)

	if first.ID != second.ID {
		t.Errorf("scan_error stable ID = %q and %q, want equal despite runtime error text", first.ID, second.ID)
	}
	if first.Detected["error"] == second.Detected["error"] {
		t.Errorf("scan_error detected error = %q and %q, want runtime error text preserved separately", first.Detected["error"], second.Detected["error"])
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
