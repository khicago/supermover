package targetlock

import (
	"path/filepath"
	"sync"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/filelock"
	"github.com/khicago/supermover/internal/pathguard"
)

var locks sync.Map

func LockTarget(targetDir string) (func(), error) {
	target, locksDir, err := prepareTargetLock(targetDir)
	if err != nil {
		return nil, err
	}
	targetValue, _ := locks.LoadOrStore(target+"\x00target", &sync.Mutex{})
	targetMu := targetValue.(*sync.Mutex)
	targetMu.Lock()
	unlockTargetFile, err := filelock.LockInDir(locksDir, "target.lock")
	if err != nil {
		targetMu.Unlock()
		return nil, err
	}
	return func() {
		unlockTargetFile()
		targetMu.Unlock()
	}, nil
}

func LockTargetSession(targetDir, sessionID string) (func(), error) {
	target, locksDir, err := prepareTargetLock(targetDir)
	if err != nil {
		return nil, err
	}
	targetValue, _ := locks.LoadOrStore(target+"\x00target", &sync.Mutex{})
	targetMu := targetValue.(*sync.Mutex)
	targetMu.Lock()
	unlockTargetFile, err := filelock.LockInDir(locksDir, "target.lock")
	if err != nil {
		targetMu.Unlock()
		return nil, err
	}

	sessionValue, _ := locks.LoadOrStore(target+"\x00session\x00"+sessionID, &sync.Mutex{})
	sessionMu := sessionValue.(*sync.Mutex)
	sessionMu.Lock()
	unlockSessionFile, err := filelock.LockInDir(locksDir, sessionID+".lock")
	if err != nil {
		sessionMu.Unlock()
		unlockTargetFile()
		targetMu.Unlock()
		return nil, err
	}
	return func() {
		unlockSessionFile()
		sessionMu.Unlock()
		unlockTargetFile()
		targetMu.Unlock()
	}, nil
}

func prepareTargetLock(targetDir string) (string, string, error) {
	target, err := pathguard.CanonicalPath(targetDir)
	if err != nil {
		return "", "", err
	}
	locksDir := filepath.Join(control.ControlDir(targetDir), "locks")
	if err := pathguard.EnsurePlainDirectory(targetDir, locksDir, 0o700); err != nil {
		return "", "", err
	}
	return target, locksDir, nil
}
