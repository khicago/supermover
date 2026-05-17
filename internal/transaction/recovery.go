package transaction

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type RecoveryAction string

const (
	ActionNone     RecoveryAction = "none"
	ActionRecover  RecoveryAction = "recover"
	ActionRollback RecoveryAction = "rollback"
	ActionRepair   RecoveryAction = "repair"
)

type RecoveryItem struct {
	Record SessionRecord
	Path   string
	Action RecoveryAction
	Reason string
}

type RecoveryScan struct {
	Items   []RecoveryItem
	Invalid []RecoveryProblem
}

type RecoveryProblem struct {
	SessionID string
	Path      string
	Err       error
}

func ClassifyRecoveryAction(state State) (RecoveryAction, string) {
	switch state {
	case StateReceived, StateValidated:
		return ActionRollback, "session did not reach durable staging"
	case StateStaged:
		return ActionRecover, "session staged but not published"
	case StateNeedsRepair:
		return ActionRepair, "session was marked for operator repair"
	case StatePublished, StateRolledBack:
		return ActionNone, "terminal state"
	default:
		return ActionRepair, "unknown state"
	}
}

func ScanRecovery(layout Layout) (RecoveryScan, error) {
	var scan RecoveryScan
	sessionsDir := layout.SessionsDir()
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return scan, nil
		}
		return scan, fmt.Errorf("read sessions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(sessionsDir, entry.Name(), "session.json")
		record, err := ReadSessionRecord(path)
		if err != nil {
			scan.Invalid = append(scan.Invalid, RecoveryProblem{SessionID: entry.Name(), Path: path, Err: err})
			continue
		}
		action, reason := ClassifyRecoveryAction(record.State)
		if action == ActionNone {
			continue
		}
		scan.Items = append(scan.Items, RecoveryItem{
			Record: record,
			Path:   path,
			Action: action,
			Reason: reason,
		})
	}

	sort.Slice(scan.Items, func(i, j int) bool {
		return scan.Items[i].Record.ID < scan.Items[j].Record.ID
	})
	sort.Slice(scan.Invalid, func(i, j int) bool {
		return scan.Invalid[i].Path < scan.Invalid[j].Path
	})
	return scan, nil
}
