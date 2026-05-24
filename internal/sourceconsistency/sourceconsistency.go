package sourceconsistency

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/scan"
)

const (
	Schema               = "supermover.acceptance.current_source_consistency.v1"
	ModeCurrentVerified  = "current_source_verified"
	ModeCurrentMismatch  = "current_source_mismatch"
	StatusPass           = "pass"
	StatusBlocked        = "blocked"
	defaultCompareDetail = "Current source tree matches the transfer baseline used for the network push session."
)

type Baseline struct {
	Schema    string          `json:"schema"`
	ProfileID string          `json:"profile_id"`
	RootID    string          `json:"root_id"`
	RootPath  string          `json:"root_path"`
	SessionID string          `json:"session_id,omitempty"`
	CreatedAt string          `json:"created_at"`
	Entries   []BaselineEntry `json:"entries"`
}

func (b Baseline) Validate() error {
	return validateBaseline(b)
}

type BaselineEntry struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	Mode          uint32 `json:"mode,omitempty"`
	Size          int64  `json:"size,omitempty"`
	ModTime       string `json:"mod_time,omitempty"`
	Digest        string `json:"digest,omitempty"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
}

type Report struct {
	Schema        string     `json:"schema"`
	Status        string     `json:"status"`
	Mode          string     `json:"mode"`
	Profile       string     `json:"profile"`
	ProfileID     string     `json:"profile_id,omitempty"`
	RootID        string     `json:"root_id,omitempty"`
	RootPath      string     `json:"root_path,omitempty"`
	SessionID     string     `json:"session_id,omitempty"`
	ComparedAt    string     `json:"compared_at"`
	EntryCount    int        `json:"entry_count"`
	MismatchCount int        `json:"mismatch_count"`
	Detail        string     `json:"detail,omitempty"`
	Mismatches    []Mismatch `json:"mismatches,omitempty"`
}

type Mismatch struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Message  string `json:"message"`
}

type BuildBaselineOptions struct {
	Profile   profile.Profile
	SessionID string
	Now       func() time.Time
	Entries   []scan.Entry
}

type CompareOptions struct {
	ProfilePath string
	Baseline    Baseline
	Now         func() time.Time
}

func BuildBaseline(opts BuildBaselineOptions) (Baseline, error) {
	if len(opts.Profile.Roots) != 1 {
		return Baseline{}, fmt.Errorf("source consistency requires exactly one root")
	}
	root := opts.Profile.Roots[0]
	rootPath, err := filepath.Abs(root.Path)
	if err != nil {
		return Baseline{}, err
	}
	if err := validateSourceRoot(rootPath); err != nil {
		return Baseline{}, err
	}
	result, err := scanForBaseline(rootPath, opts.Entries)
	if err != nil {
		return Baseline{}, err
	}
	now := nowFunc(opts.Now)()
	baseline := Baseline{
		Schema:    Schema,
		ProfileID: opts.Profile.ProfileID,
		RootID:    root.ID,
		RootPath:  filepath.ToSlash(rootPath),
		SessionID: strings.TrimSpace(opts.SessionID),
		CreatedAt: now.UTC().Format(time.RFC3339Nano),
	}
	for _, entry := range result.Entries {
		if entry.Path == "." {
			continue
		}
		if pathguard.IsReservedControlPath(entry.Path) {
			continue
		}
		next, ok := baselineEntry(entry)
		if !ok {
			continue
		}
		baseline.Entries = append(baseline.Entries, next)
	}
	sort.Slice(baseline.Entries, func(i, j int) bool {
		return baseline.Entries[i].Path < baseline.Entries[j].Path
	})
	return baseline, nil
}

func Compare(opts CompareOptions) (Report, error) {
	if err := validateBaseline(opts.Baseline); err != nil {
		return Report{}, err
	}
	rootPath, err := filepath.Abs(filepath.FromSlash(opts.Baseline.RootPath))
	if err != nil {
		return Report{}, err
	}
	if err := validateSourceRoot(rootPath); err != nil {
		return Report{}, err
	}
	now := nowFunc(opts.Now)()
	report := Report{
		Schema:     Schema,
		Status:     StatusPass,
		Mode:       ModeCurrentVerified,
		Profile:    strings.TrimSpace(opts.ProfilePath),
		ProfileID:  opts.Baseline.ProfileID,
		RootID:     opts.Baseline.RootID,
		RootPath:   filepath.ToSlash(rootPath),
		SessionID:  opts.Baseline.SessionID,
		ComparedAt: now.UTC().Format(time.RFC3339Nano),
		EntryCount: len(opts.Baseline.Entries),
		Detail:     defaultCompareDetail,
	}
	currentScan, err := scan.Scan(rootPath)
	if err != nil {
		return Report{}, fmt.Errorf("scan current source root: %w", err)
	}
	if err := rejectScanErrors(currentScan); err != nil {
		return Report{}, err
	}
	baselineEntries, err := baselineEntryMap(opts.Baseline.Entries)
	if err != nil {
		return Report{}, err
	}
	currentEntries := filteredBaselineEntries(currentScan.Entries)
	for _, path := range sortedBaselinePaths(baselineEntries, currentEntries) {
		expected, expectedOK := baselineEntries[path]
		actual, actualOK := currentEntries[path]
		switch {
		case expectedOK && !actualOK:
			report.Mismatches = append(report.Mismatches, Mismatch{
				Path:     path,
				Kind:     "missing",
				Expected: expected.Kind,
				Actual:   "missing",
				Message:  "current source path is missing",
			})
		case !expectedOK && actualOK:
			report.Mismatches = append(report.Mismatches, Mismatch{
				Path:     path,
				Kind:     "unexpected_path",
				Expected: "absent",
				Actual:   actual.Kind,
				Message:  "current source path was added after the transfer baseline",
			})
		default:
			if mismatch, ok := compareBaselineEntry(expected, actual); ok {
				report.Mismatches = append(report.Mismatches, mismatch)
			}
		}
	}
	report.MismatchCount = len(report.Mismatches)
	if report.MismatchCount > 0 {
		report.Status = StatusBlocked
		report.Mode = ModeCurrentMismatch
		report.Detail = "Current source tree no longer matches the transfer baseline used for the network push session."
	}
	return report, nil
}

func validateBaseline(baseline Baseline) error {
	if strings.TrimSpace(baseline.Schema) != Schema {
		return fmt.Errorf("source consistency baseline schema must be %q", Schema)
	}
	if strings.TrimSpace(baseline.RootPath) == "" {
		return errors.New("source consistency baseline root_path is required")
	}
	if strings.TrimSpace(baseline.RootID) == "" {
		return errors.New("source consistency baseline root_id is required")
	}
	seen := map[string]struct{}{}
	for i, entry := range baseline.Entries {
		if strings.TrimSpace(entry.Path) == "" {
			return fmt.Errorf("source consistency baseline entries[%d].path is required", i)
		}
		if strings.TrimSpace(entry.Kind) == "" {
			return fmt.Errorf("source consistency baseline entries[%d].kind is required", i)
		}
		if _, exists := seen[entry.Path]; exists {
			return fmt.Errorf("source consistency baseline has duplicate path %q", entry.Path)
		}
		seen[entry.Path] = struct{}{}
	}
	return nil
}

func compareBaselineEntry(expected BaselineEntry, actual BaselineEntry) (Mismatch, bool) {
	if actual.Kind != expected.Kind {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "kind_mismatch",
			Expected: expected.Kind,
			Actual:   actual.Kind,
			Message:  "current source path kind changed",
		}, true
	}
	switch expected.Kind {
	case "file":
		return compareRegular(expected, actual)
	case "dir":
		return compareDirectory(expected, actual)
	case "symlink":
		return compareSymlink(expected, actual)
	default:
		return Mismatch{
			Path:     expected.Path,
			Kind:     "unsupported_kind",
			Expected: expected.Kind,
			Actual:   expected.Kind,
			Message:  "baseline contains unsupported kind",
		}, true
	}
}

func compareRegular(expected BaselineEntry, actual BaselineEntry) (Mismatch, bool) {
	if actual.Size != expected.Size {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "size_mismatch",
			Expected: fmt.Sprintf("%d", expected.Size),
			Actual:   fmt.Sprintf("%d", actual.Size),
			Message:  "current source file size changed",
		}, true
	}
	if actual.Mode != expected.Mode {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "mode_mismatch",
			Expected: fmt.Sprintf("%#o", expected.Mode),
			Actual:   fmt.Sprintf("%#o", actual.Mode),
			Message:  "current source file permissions changed",
		}, true
	}
	if actual.ModTime != expected.ModTime {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "mtime_mismatch",
			Expected: expected.ModTime,
			Actual:   actual.ModTime,
			Message:  "current source file modification time changed",
		}, true
	}
	if actual.Digest != expected.Digest {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "digest_mismatch",
			Expected: expected.Digest,
			Actual:   actual.Digest,
			Message:  "current source file digest changed",
		}, true
	}
	return Mismatch{}, false
}

func compareDirectory(expected BaselineEntry, actual BaselineEntry) (Mismatch, bool) {
	if actual.Mode != expected.Mode {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "mode_mismatch",
			Expected: fmt.Sprintf("%#o", expected.Mode),
			Actual:   fmt.Sprintf("%#o", actual.Mode),
			Message:  "current source directory permissions changed",
		}, true
	}
	if actual.ModTime != expected.ModTime {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "mtime_mismatch",
			Expected: expected.ModTime,
			Actual:   actual.ModTime,
			Message:  "current source directory modification time changed",
		}, true
	}
	return Mismatch{}, false
}

func compareSymlink(expected BaselineEntry, actual BaselineEntry) (Mismatch, bool) {
	if actual.SymlinkTarget != expected.SymlinkTarget {
		return Mismatch{
			Path:     expected.Path,
			Kind:     "symlink_target_mismatch",
			Expected: expected.SymlinkTarget,
			Actual:   actual.SymlinkTarget,
			Message:  "current source symlink target changed",
		}, true
	}
	return Mismatch{}, false
}

func baselineEntry(entry scan.Entry) (BaselineEntry, bool) {
	switch entry.Kind {
	case scan.KindRegular:
		return BaselineEntry{
			Path:    entry.Path,
			Kind:    "file",
			Mode:    uint32(entry.Mode.Perm()),
			Size:    entry.Size,
			ModTime: entry.ModTime.UTC().Format(time.RFC3339Nano),
			Digest:  entryDigest(entry),
		}, true
	case scan.KindDir:
		return BaselineEntry{
			Path:    entry.Path,
			Kind:    "dir",
			Mode:    uint32(entry.Mode.Perm()),
			ModTime: entry.ModTime.UTC().Format(time.RFC3339Nano),
		}, true
	case scan.KindSymlink:
		return BaselineEntry{
			Path:          entry.Path,
			Kind:          "symlink",
			SymlinkTarget: entry.SymlinkTarget,
		}, true
	default:
		return BaselineEntry{}, false
	}
}

func filteredBaselineEntries(entries []scan.Entry) map[string]BaselineEntry {
	out := map[string]BaselineEntry{}
	for _, entry := range entries {
		if entry.Path == "." || pathguard.IsReservedControlPath(entry.Path) {
			continue
		}
		next, ok := baselineEntry(entry)
		if !ok {
			continue
		}
		out[next.Path] = next
	}
	return out
}

func baselineEntryMap(entries []BaselineEntry) (map[string]BaselineEntry, error) {
	out := make(map[string]BaselineEntry, len(entries))
	for _, entry := range entries {
		if _, exists := out[entry.Path]; exists {
			return nil, fmt.Errorf("source consistency baseline has duplicate path %q", entry.Path)
		}
		out[entry.Path] = entry
	}
	return out, nil
}

func sortedBaselinePaths(a map[string]BaselineEntry, b map[string]BaselineEntry) []string {
	paths := make([]string, 0, len(a)+len(b))
	seen := map[string]struct{}{}
	for path := range a {
		paths = append(paths, path)
		seen[path] = struct{}{}
	}
	for path := range b {
		if _, exists := seen[path]; exists {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func scanForBaseline(root string, entries []scan.Entry) (scan.Result, error) {
	if len(entries) == 0 {
		result, err := scan.Scan(root)
		if err != nil {
			return scan.Result{}, err
		}
		if err := rejectScanErrors(result); err != nil {
			return scan.Result{}, err
		}
		return result, nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return scan.Result{}, err
	}
	out := scan.Result{
		Root:    filepath.ToSlash(absRoot),
		Entries: append([]scan.Entry(nil), entries...),
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		return out.Entries[i].Path < out.Entries[j].Path
	})
	if err := rejectScanErrors(out); err != nil {
		return scan.Result{}, err
	}
	return out, nil
}

func validateSourceRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("source root is required")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("stat source root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source root must not be a symlink")
	}
	if !info.IsDir() {
		return fmt.Errorf("source root must be a directory")
	}
	return nil
}

func rejectScanErrors(result scan.Result) error {
	for _, record := range result.Audit {
		if record.Kind == "scan_error" {
			return fmt.Errorf("source scan error at %q; rerun after the source is readable before recording source consistency", record.Path)
		}
	}
	return nil
}

func entryDigest(entry scan.Entry) string {
	if entry.Size == 0 {
		return protocol.EmptySHA256Digest
	}
	return entry.Digest
}

func nowFunc(now func() time.Time) func() time.Time {
	if now != nil {
		return now
	}
	return func() time.Time {
		return time.Now().UTC()
	}
}
