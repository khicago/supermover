package durable

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncDirBestEffortReportsOpenError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	err := SyncDirBestEffort(missing)
	if err == nil {
		t.Skip("SyncDirBestEffort is a no-op on this platform")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("SyncDirBestEffort(missing) error = %v, want os.ErrNotExist", err)
	}
	var durableErr *Error
	if !errors.As(err, &durableErr) {
		t.Fatalf("SyncDirBestEffort(missing) error = %T, want *durable.Error", err)
	}
	if durableErr.Op != "open parent for sync" || durableErr.Path != missing {
		t.Fatalf("durable error = %+v, want open parent for sync at missing path", durableErr)
	}
}
