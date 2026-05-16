package health

import (
	"fmt"
	"os"
	"strings"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/transaction"
)

type Options struct {
	TargetRoot string
}

type Report struct {
	TargetRoot string          `json:"target_root"`
	Healthy    bool            `json:"healthy"`
	Summary    Summary         `json:"summary"`
	Items      []RecoveryItem  `json:"items,omitempty"`
	Invalid    []InvalidRecord `json:"invalid,omitempty"`
}

type Summary struct {
	IncompleteSessions int `json:"incomplete_sessions"`
	InvalidRecords     int `json:"invalid_records"`
}

type RecoveryItem struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	Path      string `json:"path"`
	UpdatedAt string `json:"updated_at"`
}

type InvalidRecord struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

func BuildReport(opts Options) (Report, error) {
	targetRoot := strings.TrimSpace(opts.TargetRoot)
	if targetRoot == "" {
		return Report{}, fmt.Errorf("target root is required")
	}
	info, err := os.Stat(targetRoot)
	if err != nil {
		return Report{}, fmt.Errorf("stat target root %q: %w", targetRoot, err)
	}
	if !info.IsDir() {
		return Report{}, fmt.Errorf("target root %q is not a directory", targetRoot)
	}

	scan, err := transaction.ScanRecovery(transaction.NewLayout(control.ControlDir(targetRoot)))
	if err != nil {
		return Report{}, err
	}
	report := Report{TargetRoot: targetRoot}
	for _, item := range scan.Items {
		report.Items = append(report.Items, RecoveryItem{
			SessionID: item.Record.ID,
			State:     string(item.Record.State),
			Action:    string(item.Action),
			Reason:    item.Reason,
			Path:      item.Path,
			UpdatedAt: item.Record.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}
	for _, invalid := range scan.Invalid {
		report.Invalid = append(report.Invalid, InvalidRecord{
			Path:  invalid.Path,
			Error: invalid.Err.Error(),
		})
	}
	report.Summary.IncompleteSessions = len(report.Items)
	report.Summary.InvalidRecords = len(report.Invalid)
	report.Healthy = report.Summary.IncompleteSessions == 0 && report.Summary.InvalidRecords == 0
	return report, nil
}
