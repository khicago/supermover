package protocolclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/privacy/batching"
	"github.com/khicago/supermover/internal/privacy/jitter"
	"github.com/khicago/supermover/internal/privacy/padding"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/transport"
)

const (
	maxResponseBodyBytes              = 1024 * 1024
	maxReceiverStatusRefreshesPerFile = 1
	batchRequestEnvelopeBytes         = len(`{"session_id":"","chunks":[]}`) + protocol.MaxSessionIDLen
	batchRequestPerChunkBytes         = 1
)

var (
	ErrScanBlocked           = errors.New("source scan contains errors")
	ErrSourceChanged         = errors.New("source changed during upload")
	ErrReceiverNeedsRepair   = errors.New("receiver session needs repair")
	ErrReceiverState         = errors.New("receiver session state does not allow upload")
	ErrReceiverStatusInvalid = errors.New("receiver status is invalid")
)

type TransferError struct {
	Privacy PrivacyOverhead
	Err     error
}

func (e *TransferError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *TransferError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	BaseURL   string
	Doer      Doer
	ChunkSize int
	Now       func() time.Time

	jitterSource  jitter.Source
	jitterSleeper jitter.Sleeper
}

type TransferRequest struct {
	SourceRoot     string
	Scan           scan.Result
	SessionID      string
	ManifestID     string
	ProfileID      string
	TargetID       string
	SourceDeviceID string
	TargetDeviceID string
	PrivacyPolicy  transport.PrivacyPolicy
	RootID         string
	CreatedAt      time.Time
	EndedAt        time.Time
	Progress       ProgressCallback
}

type Result struct {
	SessionID  string
	Files      int
	Bytes      int64
	Chunks     int
	Begin      protocol.BeginSessionResponse
	ResumeFrom []protocol.FileStatus
	Commit     protocol.CommitSessionResponse
	Warnings   []audit.Record
	Privacy    PrivacyOverhead
}

type ProgressCallback func(context.Context, ProgressEvent) error

type ProgressStage string

const (
	ProgressStageStatus ProgressStage = "status"
	ProgressStageChunk  ProgressStage = "chunk"
)

type ProgressEvent struct {
	SessionID    string
	Stage        ProgressStage
	State        protocol.SessionState
	ResumeFrom   []protocol.FileStatus
	Chunks       []ChunkProgress
	BytesTotal   int64
	ChunksTotal  int
	PrivacyTotal PrivacyOverhead
}

type ChunkProgress struct {
	Path                  string
	PayloadBytes          int64
	ReceiverCommittedSize int64
	Complete              bool
	State                 protocol.ChunkState
}

type PrivacyOverhead struct {
	FramePlainBytes      int64
	FrameWireBytes       int64
	PaddingBytes         int64
	PaddedChunks         int
	PaddingBucketBytes   int
	BatchFrames          int
	BatchedChunks        int
	MaxBatchCount        int
	MaxBatchPlainBytes   int
	JitteredRequests     int
	JitterDelayMillis    int64
	MaxJitterDelayMillis int
	JitterBudgetMillis   int
}

type RemoteError struct {
	Method     string
	Path       string
	StatusCode int
	Code       protocol.ErrorCode
	Message    string
}

type receiverSnapshot struct {
	State protocol.SessionState
	Files map[string]protocol.FileStatus
}

type privacyDoer struct {
	next   Doer
	jitter *jitter.Scheduler
}

func (d *privacyDoer) Do(req *http.Request) (*http.Response, error) {
	if d == nil || d.next == nil {
		return nil, errors.New("privacy doer is not configured")
	}
	if d.jitter != nil {
		if _, err := d.jitter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return d.next.Do(req)
}

func (d *privacyDoer) jitterStats() jitter.Stats {
	if d == nil || d.jitter == nil {
		return jitter.Stats{}
	}
	return d.jitter.Stats()
}

func (e *RemoteError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if e.Code != "" {
		return fmt.Sprintf("%s %s: receiver returned %d %s: %s", e.Method, e.Path, e.StatusCode, e.Code, message)
	}
	return fmt.Sprintf("%s %s: receiver returned %d: %s", e.Method, e.Path, e.StatusCode, message)
}

func BuildBeginRequest(req TransferRequest) (protocol.BeginSessionRequest, []audit.Record, error) {
	if err := rejectScanErrors(req.Scan); err != nil {
		return protocol.BeginSessionRequest{}, nil, err
	}
	manifestID := strings.TrimSpace(req.ManifestID)
	if manifestID == "" {
		manifestID = req.SessionID
	}
	warnings := append([]audit.Record(nil), req.Scan.Audit...)
	entries := make([]protocol.ManifestEntry, 0, len(req.Scan.Entries))
	reservedSkipped := 0
	firstReserved := ""
	for _, entry := range req.Scan.Entries {
		if entry.Path == "." {
			continue
		}
		if pathguard.IsReservedControlPath(entry.Path) {
			reservedSkipped++
			if firstReserved == "" {
				firstReserved = entry.Path
			}
			continue
		}
		next, ok, record, err := manifestEntry(entry)
		if err != nil {
			return protocol.BeginSessionRequest{}, nil, err
		}
		if !ok {
			warnings = append(warnings, record)
			continue
		}
		entries = append(entries, next)
	}
	if reservedSkipped > 0 {
		warnings = append(warnings, audit.WithSuggestedConfig(
			audit.WithDetected(
				audit.New(control.DirName, "", audit.SeverityWarning, "reserved_control_plane_skipped", "source .supermover is reserved for target control artifacts and was not uploaded"),
				map[string]string{"entries": fmt.Sprintf("%d", reservedSkipped), "first_path": firstReserved},
			),
			map[string]string{
				"append_migration_path": control.DirName,
				"reason":                "review whether the source .supermover directory is application data that should be migrated separately",
			},
		))
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	begin := protocol.BeginSessionRequest{
		ProtocolVersion: protocol.Version,
		SessionID:       req.SessionID,
		ProfileID:       req.ProfileID,
		TargetID:        req.TargetID,
		SourceDeviceID:  req.SourceDeviceID,
		TargetDeviceID:  req.TargetDeviceID,
		PrivacyPolicy:   req.PrivacyPolicy,
		RootID:          req.RootID,
		CreatedAt:       req.CreatedAt,
		Manifest: protocol.TransferManifest{
			ID:      manifestID,
			Entries: entries,
		},
	}
	if err := begin.Validate(); err != nil {
		return protocol.BeginSessionRequest{}, nil, err
	}
	return begin, warnings, nil
}

func scanEntriesByPath(result scan.Result) map[string]scan.Entry {
	entries := make(map[string]scan.Entry, len(result.Entries))
	for _, entry := range result.Entries {
		entries[entry.Path] = entry
	}
	return entries
}

func (c Client) Run(ctx context.Context, req TransferRequest) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	explicitEndedAt := req.EndedAt
	req = c.withDefaultTimes(req)
	req.EndedAt = time.Time{}
	beginReq, warnings, err := BuildBeginRequest(req)
	if err != nil {
		return Result{}, err
	}
	if err := validateSourceRoot(req.SourceRoot); err != nil {
		return Result{}, err
	}
	if err := validateScanRoot(req.SourceRoot, req.Scan); err != nil {
		return Result{}, err
	}
	endpoint, err := c.endpoint()
	if err != nil {
		return Result{}, err
	}
	chunkSize, err := c.chunkSize()
	if err != nil {
		return Result{}, err
	}
	paddingCfg, padChunks, err := chunkPaddingConfig(req.PrivacyPolicy)
	if err != nil {
		return Result{}, err
	}
	batchingCfg, batchChunks, err := chunkBatchingConfig(req.PrivacyPolicy)
	if err != nil {
		return Result{}, err
	}
	jitterScheduler, err := c.jitterScheduler(req.PrivacyPolicy)
	if err != nil {
		return Result{}, err
	}
	if batchChunks {
		chunkSize = effectiveBatchChunkSize(chunkSize, batchingCfg)
	}
	privacyDoer := &privacyDoer{
		next:   c.doer(),
		jitter: jitterScheduler,
	}
	doer := privacyDoer
	sourceEntries := scanEntriesByPath(req.Scan)

	if err := validateSourceEvidence(req.SourceRoot, beginReq.Manifest, sourceEntries, false); err != nil {
		return Result{}, err
	}
	result := Result{
		SessionID: beginReq.SessionID,
		Warnings:  warnings,
	}
	fail := func(err error) (Result, error) {
		if err == nil {
			return result, nil
		}
		result.Privacy.setJitterStats(privacyDoer.jitterStats())
		return result, transferError(result.Privacy, err)
	}
	var beginResp protocol.BeginSessionResponse
	if err := postJSON(ctx, doer, endpoint, "/v1/sessions", http.StatusAccepted, beginReq, &beginResp); err != nil {
		return fail(err)
	}
	if beginResp.SessionID != beginReq.SessionID {
		return fail(fmt.Errorf("begin response session_id = %q, want %q", beginResp.SessionID, beginReq.SessionID))
	}
	if !beginResp.State.Valid() {
		return fail(fmt.Errorf("begin response state = %q", beginResp.State))
	}
	result.Begin = beginResp
	statusResp, err := getStatus(ctx, doer, endpoint, beginReq.SessionID)
	if err != nil {
		return fail(err)
	}
	snapshot, err := normalizeReceiverSnapshot(beginReq.SessionID, beginReq.Manifest, statusResp)
	if err != nil {
		return fail(err)
	}
	if err := ensureReceiverCanProceed(snapshot.State); err != nil {
		return fail(err)
	}
	result.ResumeFrom = append([]protocol.FileStatus(nil), statusResp.Files...)
	if err := notifyProgress(ctx, req.Progress, ProgressEvent{
		SessionID:  beginReq.SessionID,
		Stage:      ProgressStageStatus,
		State:      snapshot.State,
		ResumeFrom: append([]protocol.FileStatus(nil), statusResp.Files...),
	}); err != nil {
		return fail(err)
	}
	if snapshot.State == protocol.SessionStateValidated {
		for _, entry := range beginReq.Manifest.Entries {
			if entry.Kind != protocol.FileKindFile {
				continue
			}
			fileStatus := snapshot.Files[entry.Path]
			if fileStatus.Complete {
				continue
			}
			sourceEntry, ok := sourceEntries[entry.Path]
			if !ok {
				return fail(fmt.Errorf("source scan entry %q is missing", entry.Path))
			}
			if fileStatus.CommittedSize > 0 {
				if err := validateRegularSourceDigest(req.SourceRoot, entry, sourceEntry); err != nil {
					return fail(err)
				}
			}
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			stats, err := c.streamFile(ctx, doer, endpoint, req.SourceRoot, beginReq.SessionID, beginReq.Manifest, entry, sourceEntry, chunkSize, fileStatus.CommittedSize, paddingCfg, padChunks, batchingCfg, batchChunks, func(ctx context.Context, update streamProgressUpdate) error {
				result.Privacy.add(update.privacy)
				result.Bytes += update.bytes
				result.Chunks += update.chunks
				return notifyProgress(ctx, req.Progress, ProgressEvent{
					SessionID:    beginReq.SessionID,
					Stage:        ProgressStageChunk,
					State:        snapshot.State,
					Chunks:       update.progress,
					BytesTotal:   result.Bytes,
					ChunksTotal:  result.Chunks,
					PrivacyTotal: result.Privacy,
				})
			})
			result.Privacy.add(stats.privacy)
			if err != nil {
				return fail(err)
			}
			result.Files++
			result.Bytes += stats.bytes
			result.Chunks += stats.chunks
		}
		statusResp, err = getStatus(ctx, doer, endpoint, beginReq.SessionID)
		if err != nil {
			return fail(err)
		}
		snapshot, err = normalizeReceiverSnapshot(beginReq.SessionID, beginReq.Manifest, statusResp)
		if err != nil {
			return fail(err)
		}
		if err := ensureReceiverCanProceed(snapshot.State); err != nil {
			return fail(err)
		}
	}
	if err := ensureReceiverReadyToCommit(snapshot, beginReq.Manifest); err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := validateSourceEvidence(req.SourceRoot, beginReq.Manifest, sourceEntries, true); err != nil {
		return fail(err)
	}

	commitReq := protocol.CommitSessionRequest{
		SessionID: beginReq.SessionID,
		EndedAt:   c.endedAt(explicitEndedAt),
	}
	if err := commitReq.Validate(); err != nil {
		return fail(err)
	}
	var commitResp protocol.CommitSessionResponse
	if err := postJSON(ctx, doer, endpoint, "/v1/commit", http.StatusOK, commitReq, &commitResp); err != nil {
		return fail(err)
	}
	if commitResp.SessionID != beginReq.SessionID {
		return fail(fmt.Errorf("commit response session_id = %q, want %q", commitResp.SessionID, beginReq.SessionID))
	}
	if commitResp.State != protocol.SessionStatePublished {
		return fail(fmt.Errorf("commit response state = %q, want %q", commitResp.State, protocol.SessionStatePublished))
	}
	result.Commit = commitResp
	result.Privacy.setJitterStats(privacyDoer.jitterStats())
	return result, nil
}

type streamStats struct {
	bytes   int64
	chunks  int
	privacy PrivacyOverhead
}

type streamProgressUpdate struct {
	bytes    int64
	chunks   int
	privacy  PrivacyOverhead
	progress []ChunkProgress
}

type streamProgressCallback func(context.Context, streamProgressUpdate) error

func (c Client) streamFile(ctx context.Context, doer Doer, endpoint *url.URL, sourceRoot, sessionID string, manifest protocol.TransferManifest, entry protocol.ManifestEntry, sourceEntry scan.Entry, chunkSize int, startOffset int64, paddingCfg padding.Config, padChunks bool, batchingCfg batching.Config, batchChunks bool, progress streamProgressCallback) (streamStats, error) {
	if startOffset < 0 || startOffset > entry.Size {
		return streamStats{}, fmt.Errorf("%w: receiver committed offset %d for %q outside size %d", ErrReceiverStatusInvalid, startOffset, entry.Path, entry.Size)
	}
	sourcePath, err := pathguard.SafeJoin(sourceRoot, entry.Path)
	if err != nil {
		return streamStats{}, err
	}
	if err := pathguard.EnsureDirectory(sourceRoot, filepath.Dir(sourcePath)); err != nil {
		return streamStats{}, err
	}
	before, err := os.Lstat(sourcePath)
	if err != nil {
		return streamStats{}, fmt.Errorf("stat source file %q: %w", entry.Path, err)
	}
	if sourceEntry.Kind != scan.KindRegular {
		return streamStats{}, fmt.Errorf("source scan entry %q kind = %q, want %q", entry.Path, sourceEntry.Kind, scan.KindRegular)
	}
	if !sourceEntry.MatchesObservedRegular(before) {
		return streamStats{}, fmt.Errorf("%w: %q no longer matches scan evidence before upload", ErrSourceChanged, entry.Path)
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return streamStats{}, fmt.Errorf("open source file %q: %w", entry.Path, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return streamStats{}, fmt.Errorf("stat opened source file %q: %w", entry.Path, err)
	}
	if !sourceEntry.MatchesObservedRegular(opened) || !os.SameFile(before, opened) {
		return streamStats{}, fmt.Errorf("%w: %q changed before upload", ErrSourceChanged, entry.Path)
	}

	buffer := make([]byte, chunkSize)
	fullHash := sha256.New()
	if startOffset > 0 {
		if _, err := io.CopyN(fullHash, file, startOffset); err != nil {
			return streamStats{}, fmt.Errorf("%w: read %q committed prefix through offset %d: %v", ErrSourceChanged, entry.Path, startOffset, err)
		}
		if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
			return streamStats{}, fmt.Errorf("seek source file %q to offset %d: %w", entry.Path, startOffset, err)
		}
	}
	var stats streamStats
	offset := startOffset
	hashedOffset := startOffset
	statusRefreshes := 0
	pending := pendingChunkBatch{sessionID: sessionID}
	if entry.Size == 0 {
		chunkReq := protocol.ChunkUploadRequest{
			SessionID: sessionID,
			Path:      entry.Path,
			Offset:    0,
			Data:      []byte{},
			Digest:    protocol.EmptySHA256Digest,
			Final:     true,
		}
		if err := chunkReq.Validate(); err != nil {
			return stats, err
		}
		var nextOffset int64
		if !batchChunks {
			nextOffset, _, err = sendSingleChunk(ctx, doer, endpoint, sessionID, manifest, entry, file, fullHash, chunkReq, 0, 0, statusRefreshes, paddingCfg, padChunks, &stats, progress)
		} else {
			recordSize, encodeErr := encodedChunkSize(chunkReq)
			if encodeErr != nil {
				return stats, encodeErr
			}
			pending.append(chunkReq, recordSize)
			nextOffset, _, err = sendPendingChunkBatch(ctx, doer, endpoint, manifest, entry, file, fullHash, pending, 0, statusRefreshes, paddingCfg, padChunks, &stats, progress)
		}
		if err != nil {
			return stats, err
		}
		if nextOffset != 0 {
			return stats, fmt.Errorf("%w: zero-byte completion committed_size = %d, want 0", ErrReceiverStatusInvalid, nextOffset)
		}
	}
	for offset < entry.Size {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		want := min(chunkSize, int(entry.Size-offset))
		n, err := io.ReadFull(file, buffer[:want])
		if err != nil {
			return stats, fmt.Errorf("%w: read %q at offset %d: %v", ErrSourceChanged, entry.Path, offset, err)
		}
		payload := append([]byte(nil), buffer[:n]...)
		final := offset+int64(n) == entry.Size
		chunkReq := protocol.ChunkUploadRequest{
			SessionID: sessionID,
			Path:      entry.Path,
			Offset:    offset,
			Data:      payload,
			Digest:    digestBytes(payload),
			Final:     final,
		}
		if err := chunkReq.Validate(); err != nil {
			return stats, err
		}
		if !batchChunks {
			nextOffset, recovered, err := sendSingleChunk(ctx, doer, endpoint, sessionID, manifest, entry, file, fullHash, chunkReq, offset, hashedOffset, statusRefreshes, paddingCfg, padChunks, &stats, progress)
			if err != nil {
				return stats, err
			}
			if nextOffset <= offset {
				return stats, fmt.Errorf("%w: chunk response committed_size = %d did not advance past offset %d", ErrReceiverStatusInvalid, nextOffset, offset)
			}
			if recovered {
				statusRefreshes++
			} else {
				statusRefreshes = 0
			}
			hashedOffset = nextOffset
			offset = nextOffset
			if _, err := file.Seek(nextOffset, io.SeekStart); err != nil {
				return stats, fmt.Errorf("seek source file %q to offset %d: %w", entry.Path, nextOffset, err)
			}
			continue
		}
		recordSize, err := encodedChunkSize(chunkReq)
		if err != nil {
			return stats, err
		}
		canAppend, err := pending.canAppend(recordSize, batchingCfg)
		if err != nil {
			return stats, fmt.Errorf("group chunk batch for %q: %w", entry.Path, err)
		}
		if !canAppend {
			nextOffset, recovered, err := sendPendingChunkBatch(ctx, doer, endpoint, manifest, entry, file, fullHash, pending, hashedOffset, statusRefreshes, paddingCfg, padChunks, &stats, progress)
			pending.reset()
			if err != nil {
				return stats, err
			}
			hashedOffset = nextOffset
			if nextOffset < chunkReq.Offset {
				return stats, fmt.Errorf("%w: batch response committed_size = %d moved before next chunk offset %d", ErrReceiverStatusInvalid, nextOffset, chunkReq.Offset)
			}
			if recovered {
				statusRefreshes++
			} else {
				statusRefreshes = 0
			}
			if recovered || nextOffset > chunkReq.Offset {
				offset = nextOffset
				if _, err := file.Seek(nextOffset, io.SeekStart); err != nil {
					return stats, fmt.Errorf("seek source file %q to offset %d: %w", entry.Path, nextOffset, err)
				}
				continue
			}
		}
		pending.append(chunkReq, recordSize)
		offset += int64(n)
		if final {
			nextOffset, recovered, err := sendPendingChunkBatch(ctx, doer, endpoint, manifest, entry, file, fullHash, pending, hashedOffset, statusRefreshes, paddingCfg, padChunks, &stats, progress)
			pending.reset()
			if err != nil {
				return stats, err
			}
			hashedOffset = nextOffset
			offset = nextOffset
			if recovered && nextOffset < entry.Size {
				statusRefreshes++
				if _, err := file.Seek(nextOffset, io.SeekStart); err != nil {
					return stats, fmt.Errorf("seek source file %q to recovered offset %d: %w", entry.Path, nextOffset, err)
				}
				continue
			}
			if nextOffset != entry.Size {
				return stats, fmt.Errorf("%w: final batch committed_size = %d, want %d", ErrReceiverStatusInvalid, nextOffset, entry.Size)
			}
			statusRefreshes = 0
		}
	}
	if got := digestFromHash(fullHash); got != entry.Digest {
		return stats, fmt.Errorf("%w: %q digest = %s, want %s", ErrSourceChanged, entry.Path, got, entry.Digest)
	}
	after, err := os.Lstat(sourcePath)
	if err != nil {
		return stats, fmt.Errorf("%w: stat source file %q after upload: %v", ErrSourceChanged, entry.Path, err)
	}
	if !sourceEntry.MatchesObservedRegular(after) || !os.SameFile(before, after) {
		return stats, fmt.Errorf("%w: %q changed during upload", ErrSourceChanged, entry.Path)
	}
	return stats, nil
}

type pendingChunkBatch struct {
	sessionID   string
	chunks      []protocol.ChunkUploadRequest
	recordSizes []int
}

func (b *pendingChunkBatch) canAppend(recordSize int, cfg batching.Config) (bool, error) {
	sizes := make([]int, 0, len(b.recordSizes)+1)
	sizes = append(sizes, b.recordSizes...)
	sizes = append(sizes, recordSize)
	batches, _, err := batching.Group(sizes, cfg)
	if err != nil {
		return false, err
	}
	return len(batches) <= 1, nil
}

func (b *pendingChunkBatch) append(chunk protocol.ChunkUploadRequest, recordSize int) {
	b.chunks = append(b.chunks, chunk)
	b.recordSizes = append(b.recordSizes, recordSize)
}

func (b *pendingChunkBatch) reset() {
	b.chunks = nil
	b.recordSizes = nil
}

func (b pendingChunkBatch) request() protocol.ChunkBatchUploadRequest {
	return protocol.ChunkBatchUploadRequest{
		SessionID: b.sessionID,
		Chunks:    b.chunks,
	}
}

func transferError(privacy PrivacyOverhead, err error) error {
	if err == nil || privacy.Empty() {
		return err
	}
	return &TransferError{Privacy: privacy, Err: err}
}

func notifyProgress(ctx context.Context, callback ProgressCallback, event ProgressEvent) error {
	if callback == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	event.ResumeFrom = append([]protocol.FileStatus(nil), event.ResumeFrom...)
	event.Chunks = append([]ChunkProgress(nil), event.Chunks...)
	if err := callback(ctx, event); err != nil {
		return err
	}
	return ctx.Err()
}

func sendSingleChunk(ctx context.Context, doer Doer, endpoint *url.URL, sessionID string, manifest protocol.TransferManifest, entry protocol.ManifestEntry, file *os.File, fullHash hash.Hash, chunkReq protocol.ChunkUploadRequest, offset, hashedOffset int64, statusRefreshes int, paddingCfg padding.Config, padChunks bool, stats *streamStats, progress streamProgressCallback) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	var chunkResp protocol.ChunkUploadResponse
	frameStats, err := postChunkJSON(ctx, doer, endpoint, chunkReq, &chunkResp, paddingCfg, padChunks)
	if padChunks && frameStats.WireBytes != 0 {
		stats.privacy.addFrame(frameStats)
	}
	if err != nil {
		nextOffset, recovered, recoverErr := recoverChunkConflict(ctx, doer, endpoint, sessionID, manifest, entry, file, fullHash, chunkReq.Data, offset, hashedOffset, statusRefreshes, err)
		if recoverErr != nil {
			return 0, false, recoverErr
		}
		if !recovered {
			return 0, false, err
		}
		return nextOffset, true, nil
	}
	nextOffset, err := validateChunkResponse(chunkReq, chunkResp)
	if err != nil {
		return 0, false, err
	}
	if nextOffset > entry.Size {
		return 0, false, fmt.Errorf("%w: chunk response committed_size = %d outside size %d", ErrReceiverStatusInvalid, nextOffset, entry.Size)
	}
	if chunkResp.Complete != (nextOffset == entry.Size) {
		return 0, false, fmt.Errorf("chunk response complete = %t, want %t", chunkResp.Complete, nextOffset == entry.Size)
	}
	if err := advanceHashToOffset(file, fullHash, chunkReq.Data, entry.Path, offset, hashedOffset, nextOffset); err != nil {
		return 0, false, err
	}
	stats.bytes += int64(len(chunkReq.Data))
	stats.chunks++
	if progress != nil {
		update := streamProgressUpdate{
			bytes:   int64(len(chunkReq.Data)),
			chunks:  1,
			privacy: stats.privacy,
			progress: []ChunkProgress{{
				Path:                  chunkReq.Path,
				PayloadBytes:          int64(len(chunkReq.Data)),
				ReceiverCommittedSize: nextOffset,
				Complete:              chunkResp.Complete,
				State:                 chunkResp.ChunkState,
			}},
		}
		if err := progress(ctx, update); err != nil {
			stats.privacy = PrivacyOverhead{}
			return 0, false, err
		}
		stats.bytes = 0
		stats.chunks = 0
		stats.privacy = PrivacyOverhead{}
	}
	return nextOffset, false, nil
}

func sendPendingChunkBatch(ctx context.Context, doer Doer, endpoint *url.URL, manifest protocol.TransferManifest, entry protocol.ManifestEntry, file *os.File, fullHash hash.Hash, pending pendingChunkBatch, hashedOffset int64, statusRefreshes int, paddingCfg padding.Config, padChunks bool, stats *streamStats, progress streamProgressCallback) (int64, bool, error) {
	if len(pending.chunks) == 0 {
		return hashedOffset, false, nil
	}
	if err := ctx.Err(); err != nil {
		return hashedOffset, false, err
	}
	batchReq := pending.request()
	if err := batchReq.Validate(); err != nil {
		return hashedOffset, false, err
	}
	var batchResp protocol.ChunkBatchUploadResponse
	frameStats, err := postChunkBatchJSON(ctx, doer, endpoint, batchReq, &batchResp, paddingCfg, padChunks)
	if padChunks && frameStats.WireBytes != 0 {
		stats.privacy.addBatchFrame(frameStats, len(batchReq.Chunks))
	}
	if err != nil {
		first := batchReq.Chunks[0]
		recoveredOffset, recovered, recoverErr := recoverChunkConflict(ctx, doer, endpoint, batchReq.SessionID, manifest, entry, file, fullHash, first.Data, first.Offset, hashedOffset, statusRefreshes, err)
		if recoverErr != nil {
			return hashedOffset, false, recoverErr
		}
		if !recovered {
			return hashedOffset, false, err
		}
		return recoveredOffset, true, nil
	}
	nextOffset := hashedOffset
	committed, err := validateChunkBatchResponse(batchReq, batchResp)
	if err != nil {
		return nextOffset, false, err
	}
	if committed > entry.Size {
		return nextOffset, false, fmt.Errorf("%w: batch response committed_size = %d outside size %d", ErrReceiverStatusInvalid, committed, entry.Size)
	}
	var update streamProgressUpdate
	update.privacy = stats.privacy
	for i, chunk := range batchReq.Chunks {
		chunkOffset, err := validateChunkResponse(chunk, batchResp.Chunks[i])
		if err != nil {
			return nextOffset, false, err
		}
		if chunkOffset > entry.Size {
			return nextOffset, false, fmt.Errorf("%w: batch chunk response committed_size = %d outside size %d", ErrReceiverStatusInvalid, chunkOffset, entry.Size)
		}
		if batchResp.Chunks[i].Complete != (chunkOffset == entry.Size) {
			return nextOffset, false, fmt.Errorf("batch chunk response complete = %t, want %t", batchResp.Chunks[i].Complete, chunkOffset == entry.Size)
		}
		if err := advanceHashToOffset(file, fullHash, chunk.Data, entry.Path, chunk.Offset, nextOffset, chunkOffset); err != nil {
			return nextOffset, false, err
		}
		nextOffset = chunkOffset
		stats.bytes += int64(len(chunk.Data))
		stats.chunks++
		update.bytes += int64(len(chunk.Data))
		update.chunks++
		update.progress = append(update.progress, ChunkProgress{
			Path:                  chunk.Path,
			PayloadBytes:          int64(len(chunk.Data)),
			ReceiverCommittedSize: chunkOffset,
			Complete:              batchResp.Chunks[i].Complete,
			State:                 batchResp.Chunks[i].ChunkState,
		})
	}
	if progress != nil && update.chunks > 0 {
		if err := progress(ctx, update); err != nil {
			stats.privacy = PrivacyOverhead{}
			return nextOffset, false, err
		}
		stats.bytes = 0
		stats.chunks = 0
		stats.privacy = PrivacyOverhead{}
	}
	return nextOffset, false, nil
}

func validateChunkResponse(req protocol.ChunkUploadRequest, resp protocol.ChunkUploadResponse) (int64, error) {
	if resp.SessionID != req.SessionID {
		return 0, fmt.Errorf("chunk response session_id = %q, want %q", resp.SessionID, req.SessionID)
	}
	if resp.Path != req.Path {
		return 0, fmt.Errorf("chunk response path = %q, want %q", resp.Path, req.Path)
	}
	wantCommitted := req.Offset + int64(len(req.Data))
	switch resp.ChunkState {
	case protocol.ChunkStateAccepted:
		if resp.CommittedSize != wantCommitted {
			return 0, fmt.Errorf("chunk response committed_size = %d, want %d", resp.CommittedSize, wantCommitted)
		}
		if resp.Complete != req.Final {
			return 0, fmt.Errorf("chunk response complete = %t, want %t", resp.Complete, req.Final)
		}
		return wantCommitted, nil
	case protocol.ChunkStateDuplicate:
		if resp.CommittedSize < wantCommitted {
			return 0, fmt.Errorf("duplicate chunk response committed_size = %d, want >= %d", resp.CommittedSize, wantCommitted)
		}
		return resp.CommittedSize, nil
	default:
		return 0, fmt.Errorf("chunk response state = %q", resp.ChunkState)
	}
}

func validateChunkBatchResponse(req protocol.ChunkBatchUploadRequest, resp protocol.ChunkBatchUploadResponse) (int64, error) {
	if resp.SessionID != req.SessionID {
		return 0, fmt.Errorf("batch response session_id = %q, want %q", resp.SessionID, req.SessionID)
	}
	if len(resp.Chunks) != len(req.Chunks) {
		return 0, fmt.Errorf("batch response chunks = %d, want %d", len(resp.Chunks), len(req.Chunks))
	}
	var committed int64
	for i, chunk := range req.Chunks {
		nextOffset, err := validateChunkResponse(chunk, resp.Chunks[i])
		if err != nil {
			return 0, fmt.Errorf("batch response chunks[%d]: %w", i, err)
		}
		if i > 0 && nextOffset < committed {
			return 0, fmt.Errorf("batch response chunks[%d] committed_size = %d moved backward from %d", i, nextOffset, committed)
		}
		committed = nextOffset
	}
	return committed, nil
}

func (o *PrivacyOverhead) add(other PrivacyOverhead) {
	o.FramePlainBytes += other.FramePlainBytes
	o.FrameWireBytes += other.FrameWireBytes
	o.PaddingBytes += other.PaddingBytes
	o.PaddedChunks += other.PaddedChunks
	if other.PaddingBucketBytes != 0 {
		o.PaddingBucketBytes = other.PaddingBucketBytes
	}
	o.BatchFrames += other.BatchFrames
	o.BatchedChunks += other.BatchedChunks
	if other.MaxBatchCount > o.MaxBatchCount {
		o.MaxBatchCount = other.MaxBatchCount
	}
	if other.MaxBatchPlainBytes > o.MaxBatchPlainBytes {
		o.MaxBatchPlainBytes = other.MaxBatchPlainBytes
	}
	o.JitteredRequests += other.JitteredRequests
	o.JitterDelayMillis += other.JitterDelayMillis
	if other.MaxJitterDelayMillis > o.MaxJitterDelayMillis {
		o.MaxJitterDelayMillis = other.MaxJitterDelayMillis
	}
	if other.JitterBudgetMillis > o.JitterBudgetMillis {
		o.JitterBudgetMillis = other.JitterBudgetMillis
	}
}

func (o *PrivacyOverhead) addFrame(stats padding.Stats) {
	o.addFrameRecords(stats, 1)
}

func (o *PrivacyOverhead) addFrameRecords(stats padding.Stats, records int) {
	o.FramePlainBytes += int64(stats.PlainBytes)
	o.FrameWireBytes += int64(stats.WireBytes)
	o.PaddingBytes += int64(stats.PaddingBytes)
	o.PaddedChunks += records
	o.PaddingBucketBytes = stats.BucketBytes
}

func (o *PrivacyOverhead) addBatchFrame(stats padding.Stats, records int) {
	o.addFrameRecords(stats, records)
	o.BatchFrames++
	o.BatchedChunks += records
	if records > o.MaxBatchCount {
		o.MaxBatchCount = records
	}
	if stats.PlainBytes > o.MaxBatchPlainBytes {
		o.MaxBatchPlainBytes = stats.PlainBytes
	}
}

func (o PrivacyOverhead) Empty() bool {
	return o.FramePlainBytes == 0 &&
		o.FrameWireBytes == 0 &&
		o.PaddingBytes == 0 &&
		o.PaddedChunks == 0 &&
		o.PaddingBucketBytes == 0 &&
		o.BatchFrames == 0 &&
		o.BatchedChunks == 0 &&
		o.MaxBatchCount == 0 &&
		o.MaxBatchPlainBytes == 0 &&
		o.JitteredRequests == 0 &&
		o.JitterDelayMillis == 0 &&
		o.MaxJitterDelayMillis == 0 &&
		o.JitterBudgetMillis == 0
}

func (o *PrivacyOverhead) setJitterStats(stats jitter.Stats) {
	if stats.JitteredRequests == 0 {
		return
	}
	o.JitteredRequests = stats.JitteredRequests
	o.JitterDelayMillis = int64(stats.TotalDelayMillis)
	o.MaxJitterDelayMillis = stats.MaxDelayMillis
	o.JitterBudgetMillis = stats.BudgetMillis
}

func chunkPaddingConfig(policy transport.PrivacyPolicy) (padding.Config, bool, error) {
	if policy.Level != transport.PrivacyLevel2 || policy.DisablePadding {
		return padding.Config{}, false, nil
	}
	if err := policy.Validate(); err != nil {
		return padding.Config{}, false, fmt.Errorf("privacy policy: %w", err)
	}
	if policy.PaddingBucket > protocol.MaxPaddingBucketBytes {
		return padding.Config{}, false, fmt.Errorf("privacy policy: padding bucket %d exceeds protocol maximum %d", policy.PaddingBucket, protocol.MaxPaddingBucketBytes)
	}
	cfg := padding.Config{
		BucketBytes:   policy.PaddingBucket,
		MaxFrameBytes: protocol.MaxPaddedChunkRequestBodyBytes,
	}
	if err := padding.Validate(cfg); err != nil {
		return padding.Config{}, false, fmt.Errorf("padding config: %w", err)
	}
	return cfg, true, nil
}

func chunkBatchingConfig(policy transport.PrivacyPolicy) (batching.Config, bool, error) {
	if policy.Level != transport.PrivacyLevel2 || policy.DisableBatching {
		return batching.Config{}, false, nil
	}
	if err := policy.Validate(); err != nil {
		return batching.Config{}, false, fmt.Errorf("privacy policy: %w", err)
	}
	if policy.BatchMaxBytes > protocol.MaxBatchPlainBodyBytes {
		return batching.Config{}, false, fmt.Errorf("privacy policy: batch max bytes %d exceeds protocol maximum %d", policy.BatchMaxBytes, protocol.MaxBatchPlainBodyBytes)
	}
	if _, _, err := padding.PaddedLen(policy.BatchMaxBytes, padding.Config{
		BucketBytes:   policy.PaddingBucket,
		MaxFrameBytes: protocol.MaxPaddedBatchRequestBodyBytes,
	}); err != nil {
		return batching.Config{}, false, fmt.Errorf("privacy policy: padded batch max bytes: %w", err)
	}
	cfg := batching.Config{
		MaxBytes:       policy.BatchMaxBytes,
		MaxCount:       min(policy.BatchMaxCount, protocol.MaxBatchChunks),
		FixedBytes:     batchRequestEnvelopeBytes,
		PerRecordBytes: batchRequestPerChunkBytes,
	}
	if err := batching.Validate(cfg); err != nil {
		return batching.Config{}, false, fmt.Errorf("batching config: %w", err)
	}
	return cfg, true, nil
}

func (c Client) jitterScheduler(policy transport.PrivacyPolicy) (*jitter.Scheduler, error) {
	if policy.Level != transport.PrivacyLevel2 {
		return nil, nil
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("privacy policy: %w", err)
	}
	opts := make([]jitter.Option, 0, 2)
	if c.jitterSource != nil {
		opts = append(opts, jitter.WithSource(c.jitterSource))
	}
	if c.jitterSleeper != nil {
		opts = append(opts, jitter.WithSleeper(c.jitterSleeper))
	}
	scheduler, err := jitter.NewScheduler(jitter.Config{BudgetMillis: policy.JitterBudget}, opts...)
	if err != nil {
		return nil, fmt.Errorf("jitter config: %w", err)
	}
	return scheduler, nil
}

func effectiveBatchChunkSize(chunkSize int, cfg batching.Config) int {
	available := cfg.MaxBytes - cfg.FixedBytes - cfg.PerRecordBytes - 1024
	if available <= 1 {
		return 1
	}
	// Chunk data is JSON/base64 encoded inside the batch body. Halving the
	// remaining budget keeps a single encoded chunk within the configured
	// plain batch body limit without buffering whole files.
	return min(chunkSize, max(1, available/2))
}

func recoverChunkConflict(ctx context.Context, doer Doer, endpoint *url.URL, sessionID string, manifest protocol.TransferManifest, entry protocol.ManifestEntry, file *os.File, fullHash hash.Hash, payload []byte, attemptedOffset, hashedOffset int64, refreshes int, chunkErr error) (int64, bool, error) {
	var remote *RemoteError
	if !errors.As(chunkErr, &remote) || remote.StatusCode != http.StatusConflict || refreshes >= maxReceiverStatusRefreshesPerFile {
		return 0, false, nil
	}
	statusResp, err := getStatus(ctx, doer, endpoint, sessionID)
	if err != nil {
		return 0, false, err
	}
	snapshot, err := normalizeReceiverSnapshot(sessionID, manifest, statusResp)
	if err != nil {
		return 0, false, err
	}
	if err := ensureReceiverCanProceed(snapshot.State); err != nil {
		return 0, false, err
	}
	if snapshot.State == protocol.SessionStatePublished {
		if err := advanceHashToOffset(file, fullHash, payload, entry.Path, attemptedOffset, hashedOffset, entry.Size); err != nil {
			return 0, false, err
		}
		if _, err := file.Seek(entry.Size, io.SeekStart); err != nil {
			return 0, false, fmt.Errorf("seek source file %q to published offset %d: %w", entry.Path, entry.Size, err)
		}
		return entry.Size, true, nil
	}
	fileStatus, ok := snapshot.Files[entry.Path]
	if !ok {
		return 0, false, fmt.Errorf("%w: status missing file %q after chunk conflict", ErrReceiverStatusInvalid, entry.Path)
	}
	if fileStatus.CommittedSize > entry.Size {
		return 0, false, fmt.Errorf("%w: status file %q committed_size = %d outside size %d", ErrReceiverStatusInvalid, entry.Path, fileStatus.CommittedSize, entry.Size)
	}
	if entry.Size == 0 && fileStatus.Complete {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return 0, false, fmt.Errorf("seek source file %q to refreshed offset 0: %w", entry.Path, err)
		}
		return 0, true, nil
	}
	if fileStatus.CommittedSize <= attemptedOffset {
		return 0, false, chunkErr
	}
	if err := advanceHashToOffset(file, fullHash, payload, entry.Path, attemptedOffset, hashedOffset, fileStatus.CommittedSize); err != nil {
		return 0, false, err
	}
	if _, err := file.Seek(fileStatus.CommittedSize, io.SeekStart); err != nil {
		return 0, false, fmt.Errorf("seek source file %q to refreshed offset %d: %w", entry.Path, fileStatus.CommittedSize, err)
	}
	return fileStatus.CommittedSize, true, nil
}

func advanceHashToOffset(file *os.File, fullHash hash.Hash, payload []byte, path string, attemptedOffset, hashedOffset, targetOffset int64) error {
	if targetOffset <= hashedOffset {
		return nil
	}
	payloadEnd := attemptedOffset + int64(len(payload))
	if hashedOffset < payloadEnd {
		start := max(hashedOffset-attemptedOffset, 0)
		end := min(targetOffset-attemptedOffset, int64(len(payload)))
		if end > start {
			if _, err := fullHash.Write(payload[int(start):int(end)]); err != nil {
				return err
			}
		}
	}
	if targetOffset <= payloadEnd {
		return nil
	}
	readStart := max(hashedOffset, payloadEnd)
	if _, err := file.Seek(readStart, io.SeekStart); err != nil {
		return fmt.Errorf("seek source file %q to hash offset %d: %w", path, readStart, err)
	}
	if _, err := io.CopyN(fullHash, file, targetOffset-readStart); err != nil {
		return fmt.Errorf("%w: read %q through receiver offset %d: %v", ErrSourceChanged, path, targetOffset, err)
	}
	return nil
}

func normalizeReceiverSnapshot(sessionID string, manifest protocol.TransferManifest, status protocol.SessionStatusResponse) (receiverSnapshot, error) {
	if status.SessionID != sessionID {
		return receiverSnapshot{}, fmt.Errorf("%w: status session_id = %q, want %q", ErrReceiverStatusInvalid, status.SessionID, sessionID)
	}
	if !status.State.Valid() {
		return receiverSnapshot{}, fmt.Errorf("%w: status state = %q", ErrReceiverStatusInvalid, status.State)
	}
	entries := manifestFileEntries(manifest)
	files := make(map[string]protocol.FileStatus, len(entries))
	for _, file := range status.Files {
		entry, ok := entries[file.Path]
		if !ok {
			return receiverSnapshot{}, fmt.Errorf("%w: status contains unknown file %q", ErrReceiverStatusInvalid, file.Path)
		}
		if _, seen := files[file.Path]; seen {
			return receiverSnapshot{}, fmt.Errorf("%w: status contains duplicate file %q", ErrReceiverStatusInvalid, file.Path)
		}
		if file.ExpectedSize != entry.Size {
			return receiverSnapshot{}, fmt.Errorf("%w: status file %q expected_size = %d, want %d", ErrReceiverStatusInvalid, file.Path, file.ExpectedSize, entry.Size)
		}
		if file.ExpectedDigest != entry.Digest {
			return receiverSnapshot{}, fmt.Errorf("%w: status file %q expected_digest = %q, want %q", ErrReceiverStatusInvalid, file.Path, file.ExpectedDigest, entry.Digest)
		}
		if file.CommittedSize < 0 || file.CommittedSize > entry.Size {
			return receiverSnapshot{}, fmt.Errorf("%w: status file %q committed_size = %d outside [0,%d]", ErrReceiverStatusInvalid, file.Path, file.CommittedSize, entry.Size)
		}
		complete := file.CommittedSize == entry.Size
		if entry.Size == 0 {
			complete = file.Complete
		}
		if file.Complete != complete {
			return receiverSnapshot{}, fmt.Errorf("%w: status file %q complete = %t, want %t", ErrReceiverStatusInvalid, file.Path, file.Complete, complete)
		}
		files[file.Path] = file
	}
	if status.State != protocol.SessionStatePublished {
		for path := range entries {
			if _, ok := files[path]; !ok {
				return receiverSnapshot{}, fmt.Errorf("%w: status missing file %q", ErrReceiverStatusInvalid, path)
			}
		}
	}
	return receiverSnapshot{State: status.State, Files: files}, nil
}

func manifestFileEntries(manifest protocol.TransferManifest) map[string]protocol.ManifestEntry {
	entries := make(map[string]protocol.ManifestEntry)
	for _, entry := range manifest.Entries {
		if entry.Kind == protocol.FileKindFile {
			entries[entry.Path] = entry
		}
	}
	return entries
}

func ensureReceiverCanProceed(state protocol.SessionState) error {
	switch state {
	case protocol.SessionStateValidated, protocol.SessionStateStaged, protocol.SessionStatePublished:
		return nil
	case protocol.SessionStateNeedsRepair:
		return fmt.Errorf("%w: receiver reported %q", ErrReceiverNeedsRepair, state)
	case protocol.SessionStateRolledBack, protocol.SessionStateReceived:
		return fmt.Errorf("%w: receiver reported %q", ErrReceiverState, state)
	default:
		return fmt.Errorf("%w: receiver reported %q", ErrReceiverStatusInvalid, state)
	}
}

func ensureReceiverReadyToCommit(snapshot receiverSnapshot, manifest protocol.TransferManifest) error {
	if err := ensureReceiverCanProceed(snapshot.State); err != nil {
		return err
	}
	if snapshot.State == protocol.SessionStatePublished {
		return nil
	}
	for _, entry := range manifest.Entries {
		if entry.Kind != protocol.FileKindFile {
			continue
		}
		fileStatus, ok := snapshot.Files[entry.Path]
		if !ok {
			return fmt.Errorf("%w: status missing file %q before commit", ErrReceiverStatusInvalid, entry.Path)
		}
		if !fileStatus.Complete || fileStatus.CommittedSize != entry.Size {
			return fmt.Errorf("%w: status file %q committed_size = %d, want complete size %d before commit", ErrReceiverStatusInvalid, entry.Path, fileStatus.CommittedSize, entry.Size)
		}
	}
	return nil
}

func validateSourceEvidence(sourceRoot string, manifest protocol.TransferManifest, sourceEntries map[string]scan.Entry, verifyRegularDigest bool) error {
	for _, entry := range manifest.Entries {
		sourceEntry, ok := sourceEntries[entry.Path]
		if !ok {
			return fmt.Errorf("source scan entry %q is missing", entry.Path)
		}
		sourcePath, err := pathguard.SafeJoin(sourceRoot, entry.Path)
		if err != nil {
			return err
		}
		if err := pathguard.EnsureDirectory(sourceRoot, filepath.Dir(sourcePath)); err != nil {
			return err
		}
		switch entry.Kind {
		case protocol.FileKindDir:
			if sourceEntry.Kind != scan.KindDir {
				return fmt.Errorf("source scan entry %q kind = %q, want %q", entry.Path, sourceEntry.Kind, scan.KindDir)
			}
			info, err := os.Lstat(sourcePath)
			if err != nil {
				return fmt.Errorf("stat source directory %q: %w", entry.Path, err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: source directory %q is a symlink", pathguard.ErrUnsafePath, entry.Path)
			}
			if !info.IsDir() {
				return fmt.Errorf("%w: %q changed from directory", ErrSourceChanged, entry.Path)
			}
			if info.Mode().Perm() != sourceEntry.Mode.Perm() || !info.ModTime().Equal(sourceEntry.ModTime) {
				return fmt.Errorf("%w: source directory %q metadata changed", ErrSourceChanged, entry.Path)
			}
		case protocol.FileKindSymlink:
			if sourceEntry.Kind != scan.KindSymlink {
				return fmt.Errorf("source scan entry %q kind = %q, want %q", entry.Path, sourceEntry.Kind, scan.KindSymlink)
			}
			info, err := os.Lstat(sourcePath)
			if err != nil {
				return fmt.Errorf("stat source symlink %q: %w", entry.Path, err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("%w: %q changed from symlink", ErrSourceChanged, entry.Path)
			}
			if !info.ModTime().Equal(sourceEntry.ModTime) {
				return fmt.Errorf("%w: source symlink %q metadata changed", ErrSourceChanged, entry.Path)
			}
			target, err := os.Readlink(sourcePath)
			if err != nil {
				return fmt.Errorf("read source symlink %q: %w", entry.Path, err)
			}
			target = filepath.ToSlash(target)
			if target != entry.SymlinkTarget {
				return fmt.Errorf("%w: %q symlink target = %q, want %q", ErrSourceChanged, entry.Path, target, entry.SymlinkTarget)
			}
		case protocol.FileKindFile:
			if sourceEntry.Kind != scan.KindRegular {
				return fmt.Errorf("source scan entry %q kind = %q, want %q", entry.Path, sourceEntry.Kind, scan.KindRegular)
			}
			info, err := os.Lstat(sourcePath)
			if err != nil {
				return fmt.Errorf("stat source file %q: %w", entry.Path, err)
			}
			if !sourceEntry.MatchesObservedRegular(info) {
				return fmt.Errorf("%w: %q no longer matches scan evidence", ErrSourceChanged, entry.Path)
			}
			if !verifyRegularDigest {
				continue
			}
			digest, err := digestRegularFile(sourcePath, sourceEntry)
			if err != nil {
				return err
			}
			if digest != entry.Digest {
				return fmt.Errorf("%w: %q digest = %s, want %s", ErrSourceChanged, entry.Path, digest, entry.Digest)
			}
		}
	}
	return nil
}

func validateRegularSourceDigest(sourceRoot string, entry protocol.ManifestEntry, sourceEntry scan.Entry) error {
	sourcePath, err := pathguard.SafeJoin(sourceRoot, entry.Path)
	if err != nil {
		return err
	}
	if err := pathguard.EnsureDirectory(sourceRoot, filepath.Dir(sourcePath)); err != nil {
		return err
	}
	if sourceEntry.Kind != scan.KindRegular {
		return fmt.Errorf("source scan entry %q kind = %q, want %q", entry.Path, sourceEntry.Kind, scan.KindRegular)
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source file %q: %w", entry.Path, err)
	}
	if !sourceEntry.MatchesObservedRegular(info) {
		return fmt.Errorf("%w: %q no longer matches scan evidence", ErrSourceChanged, entry.Path)
	}
	digest, err := digestRegularFile(sourcePath, sourceEntry)
	if err != nil {
		return err
	}
	if digest != entry.Digest {
		return fmt.Errorf("%w: %q digest = %s, want %s", ErrSourceChanged, entry.Path, digest, entry.Digest)
	}
	return nil
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

func validateScanRoot(sourceRoot string, result scan.Result) error {
	if strings.TrimSpace(result.Root) == "" {
		return errors.New("scan root is required")
	}
	sourceAbs, err := filepath.Abs(sourceRoot)
	if err != nil {
		return err
	}
	scanAbs, err := filepath.Abs(filepath.FromSlash(result.Root))
	if err != nil {
		return err
	}
	sourceCanon, err := filepath.EvalSymlinks(sourceAbs)
	if err != nil {
		return fmt.Errorf("resolve source root: %w", err)
	}
	scanCanon, err := filepath.EvalSymlinks(scanAbs)
	if err != nil {
		return fmt.Errorf("resolve scan root: %w", err)
	}
	if sourceCanon != scanCanon {
		return fmt.Errorf("source root %q does not match scan root %q", sourceRoot, result.Root)
	}
	return nil
}

func digestRegularFile(path string, entry scan.Entry) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open source file %q: %w", entry.Path, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat opened source file %q: %w", entry.Path, err)
	}
	if !entry.MatchesObservedRegular(opened) {
		return "", fmt.Errorf("%w: %q changed before digest validation", ErrSourceChanged, entry.Path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("digest source file %q: %w", entry.Path, err)
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("stat source file %q after digest validation: %w", entry.Path, err)
	}
	if !entry.MatchesObservedRegular(after) || !os.SameFile(opened, after) {
		return "", fmt.Errorf("%w: %q changed during digest validation", ErrSourceChanged, entry.Path)
	}
	return digestFromHash(hash), nil
}

func manifestEntry(entry scan.Entry) (protocol.ManifestEntry, bool, audit.Record, error) {
	switch entry.Kind {
	case scan.KindDir:
		return protocol.ManifestEntry{
			Path:    entry.Path,
			Kind:    protocol.FileKindDir,
			Mode:    uint32(entry.Mode.Perm()),
			ModTime: entry.ModTime,
		}, true, audit.Record{}, nil
	case scan.KindRegular:
		digest := entry.Digest
		if entry.Size == 0 {
			digest = protocol.EmptySHA256Digest
		}
		return protocol.ManifestEntry{
			Path:    entry.Path,
			Kind:    protocol.FileKindFile,
			Mode:    uint32(entry.Mode.Perm()),
			Size:    entry.Size,
			Digest:  digest,
			ModTime: entry.ModTime,
		}, true, audit.Record{}, nil
	case scan.KindSymlink:
		return protocol.ManifestEntry{
			Path:          entry.Path,
			Kind:          protocol.FileKindSymlink,
			SymlinkTarget: entry.SymlinkTarget,
		}, true, audit.Record{}, nil
	case scan.KindSpecial:
		return protocol.ManifestEntry{}, false, audit.WithDetected(
			audit.New(entry.Path, entry.Path, audit.SeverityWarning, "special_not_uploaded", "special file upload is not supported by the receiver protocol client"),
			map[string]string{"mode": entry.Mode.String()},
		), nil
	default:
		return protocol.ManifestEntry{}, false, audit.WithDetected(
			audit.New(entry.Path, entry.Path, audit.SeverityWarning, "unsupported_scan_kind", "scan entry kind is not supported by the receiver protocol client"),
			map[string]string{"kind": string(entry.Kind), "mode": entry.Mode.String()},
		), nil
	}
}

func rejectScanErrors(result scan.Result) error {
	for _, record := range result.Audit {
		if record.Kind == "scan_error" {
			return fmt.Errorf("%w at %q; rerun after the source is readable before upload", ErrScanBlocked, record.Path)
		}
	}
	return nil
}

func (c Client) withDefaultTimes(req TransferRequest) TransferRequest {
	now := c.now()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	return req
}

func (c Client) chunkSize() (int, error) {
	switch {
	case c.ChunkSize == 0:
		return protocol.MaxChunkBytes, nil
	case c.ChunkSize < 0:
		return 0, fmt.Errorf("chunk size must be positive")
	case c.ChunkSize > protocol.MaxChunkBytes:
		return 0, fmt.Errorf("chunk size %d exceeds protocol maximum %d", c.ChunkSize, protocol.MaxChunkBytes)
	default:
		return c.ChunkSize, nil
	}
}

func (c Client) doer() Doer {
	if c.Doer != nil {
		return c.Doer
	}
	return http.DefaultClient
}

func (c Client) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

func (c Client) endedAt(explicit time.Time) time.Time {
	if !explicit.IsZero() {
		return explicit.UTC()
	}
	return c.now()
}

func (c Client) endpoint() (*url.URL, error) {
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		return nil, errors.New("base URL is required")
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base URL must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

func postJSON(ctx context.Context, doer Doer, endpoint *url.URL, path string, wantStatus int, body any, dest any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	reqURL := endpoint.ResolveReference(&url.URL{Path: endpoint.Path + path})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), &buf)
	if err != nil {
		return fmt.Errorf("build POST %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return doPOST(ctx, doer, req, path, wantStatus, dest)
}

func postChunkJSON(ctx context.Context, doer Doer, endpoint *url.URL, body protocol.ChunkUploadRequest, dest *protocol.ChunkUploadResponse, cfg padding.Config, pad bool) (padding.Stats, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return padding.Stats{}, fmt.Errorf("encode /v1/chunks: %w", err)
	}
	payload := buf.Bytes()
	reqBody := payload
	var stats padding.Stats
	if pad {
		wire, frameStats, err := padding.Pad(payload, cfg)
		if err != nil {
			return padding.Stats{}, fmt.Errorf("pad /v1/chunks: %w", err)
		}
		reqBody = wire
		stats = frameStats
	}
	reqURL := endpoint.ResolveReference(&url.URL{Path: endpoint.Path + "/v1/chunks"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(reqBody))
	if err != nil {
		return padding.Stats{}, fmt.Errorf("build POST /v1/chunks: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if pad {
		req.Header.Set(protocol.FrameEncodingHeader, protocol.FrameEncodingPaddingV1)
		req.Header.Set(protocol.FrameSessionIDHeader, body.SessionID)
	}
	if err := doPOST(ctx, doer, req, "/v1/chunks", http.StatusAccepted, dest); err != nil {
		return stats, err
	}
	return stats, nil
}

func postChunkBatchJSON(ctx context.Context, doer Doer, endpoint *url.URL, body protocol.ChunkBatchUploadRequest, dest *protocol.ChunkBatchUploadResponse, cfg padding.Config, pad bool) (padding.Stats, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return padding.Stats{}, fmt.Errorf("encode /v1/chunk-batches: %w", err)
	}
	payload := buf.Bytes()
	reqBody := payload
	var stats padding.Stats
	if pad {
		cfg.MaxFrameBytes = protocol.MaxPaddedBatchRequestBodyBytes
		wire, frameStats, err := padding.Pad(payload, cfg)
		if err != nil {
			return padding.Stats{}, fmt.Errorf("pad /v1/chunk-batches: %w", err)
		}
		reqBody = wire
		stats = frameStats
	}
	reqURL := endpoint.ResolveReference(&url.URL{Path: endpoint.Path + "/v1/chunk-batches"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(reqBody))
	if err != nil {
		return padding.Stats{}, fmt.Errorf("build POST /v1/chunk-batches: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if pad {
		req.Header.Set(protocol.FrameEncodingHeader, protocol.FrameEncodingPaddingV1)
		req.Header.Set(protocol.FrameSessionIDHeader, body.SessionID)
	}
	if err := doPOST(ctx, doer, req, "/v1/chunk-batches", http.StatusAccepted, dest); err != nil {
		return stats, err
	}
	return stats, nil
}

func doPOST(ctx context.Context, doer Doer, req *http.Request, path string, wantStatus int, dest any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	resp, err := doer.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	if resp == nil {
		return fmt.Errorf("POST %s: receiver returned nil response", path)
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(strings.NewReader(""))
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return decodeRemoteError(http.MethodPost, path, resp)
	}
	if dest == nil {
		return nil
	}
	data, err := readLimitedResponseBody(resp.Body)
	if err != nil {
		return fmt.Errorf("decode POST %s response: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decode POST %s response: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("response body must contain a single JSON document")
		}
		return fmt.Errorf("decode POST %s response: %w", path, err)
	}
	return nil
}

func getStatus(ctx context.Context, doer Doer, endpoint *url.URL, sessionID string) (protocol.SessionStatusResponse, error) {
	path := "/v1/sessions/" + url.PathEscape(sessionID) + "/status"
	var status protocol.SessionStatusResponse
	if err := getJSON(ctx, doer, endpoint, path, http.StatusOK, &status); err != nil {
		return protocol.SessionStatusResponse{}, err
	}
	return status, nil
}

func getJSON(ctx context.Context, doer Doer, endpoint *url.URL, path string, wantStatus int, dest any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	reqURL := endpoint.ResolveReference(&url.URL{Path: endpoint.Path + path})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("build GET %s: %w", path, err)
	}
	resp, err := doer.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	if resp == nil {
		return fmt.Errorf("GET %s: receiver returned nil response", path)
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(strings.NewReader(""))
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return decodeRemoteError(http.MethodGet, path, resp)
	}
	if dest == nil {
		return nil
	}
	data, err := readLimitedResponseBody(resp.Body)
	if err != nil {
		return fmt.Errorf("decode GET %s response: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decode GET %s response: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("response body must contain a single JSON document")
		}
		return fmt.Errorf("decode GET %s response: %w", path, err)
	}
	return nil
}

func readLimitedResponseBody(reader io.Reader) ([]byte, error) {
	limited := &io.LimitedReader{R: reader, N: maxResponseBodyBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxResponseBodyBytes {
		return nil, fmt.Errorf("response body too large")
	}
	return data, nil
}

func decodeRemoteError(method, path string, resp *http.Response) error {
	var remote protocol.ErrorResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&remote); err != nil {
		remote.Message = strings.TrimSpace(resp.Status)
	}
	return &RemoteError{
		Method:     method,
		Path:       path,
		StatusCode: resp.StatusCode,
		Code:       remote.Code,
		Message:    remote.Message,
	}
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func digestFromHash(h hash.Hash) string {
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func encodedChunkSize(chunk protocol.ChunkUploadRequest) (int, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(chunk); err != nil {
		return 0, err
	}
	return buf.Len(), nil
}
