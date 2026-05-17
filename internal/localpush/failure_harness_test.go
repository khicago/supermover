package localpush

import (
	"errors"
	"testing"

	"github.com/khicago/supermover/internal/control"
)

type oneShotFault struct {
	err   error
	fired int
}

func newOneShotFault(err error) *oneShotFault {
	return &oneShotFault{err: err}
}

func (f *oneShotFault) Fire() error {
	if f.fired > 0 {
		return nil
	}
	f.fired++
	return f.err
}

func installBeforePublishStagedFault(t *testing.T, fault *oneShotFault, match func(control.ManifestEntry, string) bool) {
	t.Helper()
	previous := beforePublishStaged
	beforePublishStaged = func(entry control.ManifestEntry, targetPath string) error {
		if previous != nil {
			if err := previous(entry, targetPath); err != nil {
				return err
			}
		}
		if !match(entry, targetPath) {
			return nil
		}
		return fault.Fire()
	}
	t.Cleanup(func() { beforePublishStaged = previous })
}

func TestOneShotFaultFiresOnlyOnce(t *testing.T) {
	want := errors.New("simulated write failure")
	fault := newOneShotFault(want)

	if err := fault.Fire(); !errors.Is(err, want) {
		t.Fatalf("first Fire() error = %v, want %v", err, want)
	}
	if err := fault.Fire(); err != nil {
		t.Fatalf("second Fire() error = %v, want nil", err)
	}
	if fault.fired != 1 {
		t.Fatalf("fault fired = %d, want 1", fault.fired)
	}
}

func TestBeforePublishStagedFaultRestoresHook(t *testing.T) {
	setBeforePublishStagedHook(nil)
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })

	t.Run("installed", func(t *testing.T) {
		fault := newOneShotFault(errors.New("simulated promote failure"))
		installBeforePublishStagedFault(t, fault, func(control.ManifestEntry, string) bool {
			return true
		})

		if beforePublishStaged == nil {
			t.Fatalf("beforePublishStaged hook = nil, want installed fault")
		}
		if err := beforePublishStaged(control.ManifestEntry{Path: "file.txt"}, "file.txt"); err == nil {
			t.Fatalf("beforePublishStaged() error = nil, want injected failure")
		}
	})

	if beforePublishStaged != nil {
		t.Fatalf("beforePublishStaged hook = %p after subtest, want nil", beforePublishStaged)
	}
}

func TestBeforePublishStagedFaultPreservesExistingError(t *testing.T) {
	realErr := errors.New("real publish preflight failure")
	setBeforePublishStagedHook(func(control.ManifestEntry, string) error {
		return realErr
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })

	fault := newOneShotFault(errors.New("simulated promote failure"))
	installBeforePublishStagedFault(t, fault, func(control.ManifestEntry, string) bool {
		return true
	})

	if err := beforePublishStaged(control.ManifestEntry{Path: "file.txt"}, "file.txt"); !errors.Is(err, realErr) {
		t.Fatalf("beforePublishStaged() error = %v, want existing error %v", err, realErr)
	}
	if fault.fired != 0 {
		t.Fatalf("fault fired = %d, want 0 when existing hook fails first", fault.fired)
	}
}
