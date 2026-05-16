package audit

import "testing"

func TestNewRecordStableID(t *testing.T) {
	a := New("./dir/file.txt", "target/file.txt", SeverityWarning, "special_file", "named pipe")
	b := New("dir/file.txt", "./target/file.txt", SeverityWarning, "special_file", "named pipe")

	if a.ID == "" {
		t.Fatal("expected ID")
	}
	if a.ID != b.ID {
		t.Fatalf("expected stable ID, got %q and %q", a.ID, b.ID)
	}
	if a.Disposition != DispositionOpen {
		t.Fatalf("expected open disposition, got %q", a.Disposition)
	}
}

func TestWithHelpersCopyMaps(t *testing.T) {
	r := New("p", "", SeverityInfo, "k", "r")
	detected := map[string]string{"mode": "0600"}

	r = WithDetected(r, detected)
	detected["mode"] = "0777"

	if r.Detected["mode"] != "0600" {
		t.Fatalf("detected metadata was not copied: %#v", r.Detected)
	}
}
