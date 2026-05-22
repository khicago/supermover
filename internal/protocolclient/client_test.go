package protocolclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/privacy/padding"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/receiver"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/transport"
)

func TestClientRunStreamsRegularFileToReceiverInBoundedChunks(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	chunkSize := 32 * 1024
	sourceFile := filepath.Join(source, "big.bin")
	if err := writePatternFile(sourceFile, chunkSize*2+123); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}

	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()

	got, err := Client{BaseURL: server.URL, ChunkSize: chunkSize}.Run(context.Background(), validTransferRequest(source, scanResult, "session-large"))
	if err != nil {
		t.Fatalf("Client.Run(large file) error = %v, want nil", err)
	}
	if got.Chunks != 3 {
		t.Fatalf("Client.Run(large file).Chunks = %d, want 3", got.Chunks)
	}
	if got.Bytes != int64(chunkSize*2+123) {
		t.Fatalf("Client.Run(large file).Bytes = %d, want %d", got.Bytes, chunkSize*2+123)
	}

	recorder.mu.Lock()
	chunkSizes := append([]int(nil), recorder.chunkSizes...)
	chunkOffsets := append([]int64(nil), recorder.chunkOffsets...)
	recorder.mu.Unlock()
	if len(chunkSizes) != 3 {
		t.Fatalf("recorded chunk count = %d, want 3", len(chunkSizes))
	}
	for i, size := range chunkSizes {
		if size > chunkSize {
			t.Fatalf("chunk[%d] size = %d, want <= %d", i, size, chunkSize)
		}
	}
	wantOffsets := []int64{0, int64(chunkSize), int64(chunkSize * 2)}
	for i, want := range wantOffsets {
		if chunkOffsets[i] != want {
			t.Fatalf("chunk[%d] offset = %d, want %d", i, chunkOffsets[i], want)
		}
	}
	if hashFile(t, filepath.Join(target, "big.bin")) != hashFile(t, sourceFile) {
		t.Fatalf("published file digest differs from source")
	}
}

func TestClientRunPadsChunkFramesWithoutChangingPayloadIdentity(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("abcdefghij"))
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-padded-chunks"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 8

	got, err := newZeroJitterClient(server.URL, 4, server.Client()).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Client.Run(padded chunks) error = %v, want nil", err)
	}
	if got.Bytes != 10 || got.Chunks != 3 {
		t.Fatalf("Client.Run(padded chunks) bytes/chunks = %d/%d, want 10/3", got.Bytes, got.Chunks)
	}
	if got.Privacy.PaddedChunks != 3 || got.Privacy.PaddingBucketBytes != 64 || got.Privacy.PaddingBytes <= 0 {
		t.Fatalf("Client.Run(padded chunks).Privacy = %+v, want padded chunk overhead with bucket 64", got.Privacy)
	}
	if got.Privacy.FrameWireBytes <= got.Privacy.FramePlainBytes {
		t.Fatalf("Client.Run(padded chunks).Privacy = %+v, want wire bytes greater than plain frame bytes", got.Privacy)
	}

	recorder.mu.Lock()
	frameEncodings := append([]string(nil), recorder.batchFrameEncodings...)
	frameWireBytes := append([]int(nil), recorder.batchFrameWireBytes...)
	framePaddingBytes := append([]int(nil), recorder.batchFramePaddingBytes...)
	chunkData := append([][]byte(nil), recorder.chunkData...)
	recorder.mu.Unlock()
	if len(frameEncodings) != 1 {
		t.Fatalf("recorded padded batch frames = %d, want 1", len(frameEncodings))
	}
	for i := range frameEncodings {
		if frameEncodings[i] != protocol.FrameEncodingPaddingV1 {
			t.Fatalf("batch[%d] frame encoding = %q, want %q", i, frameEncodings[i], protocol.FrameEncodingPaddingV1)
		}
		if frameWireBytes[i]%64 != 0 {
			t.Fatalf("batch[%d] wire bytes = %d, want multiple of 64", i, frameWireBytes[i])
		}
		if framePaddingBytes[i] < 0 {
			t.Fatalf("batch[%d] padding bytes = %d, want non-negative padding", i, framePaddingBytes[i])
		}
	}
	if string(bytes.Join(chunkData, nil)) != "abcdefghij" {
		t.Fatalf("recorded payload = %q, want original source bytes", string(bytes.Join(chunkData, nil)))
	}
	if hashFile(t, filepath.Join(target, "data.bin")) != hashFile(t, filepath.Join(source, "data.bin")) {
		t.Fatalf("published file digest differs from source")
	}
}

func TestClientRunBatchesLevel2ChunksWithinSingleFile(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("abcdefghijkl"))
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-batched-chunks"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 2

	got, err := newZeroJitterClient(server.URL, 3, server.Client()).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Client.Run(batched chunks) error = %v, want nil", err)
	}
	if got.Bytes != 12 || got.Chunks != 4 {
		t.Fatalf("Client.Run(batched chunks) bytes/chunks = %d/%d, want 12/4", got.Bytes, got.Chunks)
	}
	if got.Privacy.PaddedChunks != 4 || got.Privacy.PaddingBucketBytes != 64 || got.Privacy.PaddingBytes <= 0 {
		t.Fatalf("Client.Run(batched chunks).Privacy = %+v, want overhead counted for four chunk records", got.Privacy)
	}

	recorder.mu.Lock()
	batchCounts := append([]int(nil), recorder.batchCounts...)
	batchEncodings := append([]string(nil), recorder.batchFrameEncodings...)
	batchWireBytes := append([]int(nil), recorder.batchFrameWireBytes...)
	chunkOffsets := append([]int64(nil), recorder.chunkOffsets...)
	chunkData := append([][]byte(nil), recorder.chunkData...)
	recorder.mu.Unlock()
	if got, want := batchCounts, []int{2, 2}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("batch counts = %+v, want %+v", got, want)
	}
	for i, encoding := range batchEncodings {
		if encoding != protocol.FrameEncodingPaddingV1 {
			t.Fatalf("batch[%d] frame encoding = %q, want %q", i, encoding, protocol.FrameEncodingPaddingV1)
		}
		if batchWireBytes[i]%64 != 0 {
			t.Fatalf("batch[%d] wire bytes = %d, want multiple of 64", i, batchWireBytes[i])
		}
	}
	if got, want := chunkOffsets, []int64{0, 3, 6, 9}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("batched chunk offsets = %+v, want %+v", got, want)
	}
	if string(bytes.Join(chunkData, nil)) != "abcdefghijkl" {
		t.Fatalf("batched payload = %q, want original source bytes", string(bytes.Join(chunkData, nil)))
	}
	if hashFile(t, filepath.Join(target, "data.bin")) != hashFile(t, filepath.Join(source, "data.bin")) {
		t.Fatalf("published file digest differs from source")
	}
}

func TestClientRunReportsProgressAfterValidatedBatchResponses(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("abcdefghijkl"))
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-progress-batches"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 2
	var events []ProgressEvent
	req.Progress = func(_ context.Context, event ProgressEvent) error {
		events = append(events, event)
		return nil
	}

	got, err := newZeroJitterClient(server.URL, 3, server.Client()).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Client.Run(progress batches) error = %v, want nil", err)
	}
	if got.Bytes != 12 || got.Chunks != 4 {
		t.Fatalf("Client.Run(progress batches) bytes/chunks = %d/%d, want 12/4", got.Bytes, got.Chunks)
	}

	var statusEvents, chunkEvents int
	for _, event := range events {
		switch event.Stage {
		case ProgressStageStatus:
			statusEvents++
			if event.SessionID != "session-progress-batches" || event.State != protocol.SessionStateValidated {
				t.Fatalf("status progress event = %+v, want session and validated state", event)
			}
			if len(event.ResumeFrom) != 1 || event.ResumeFrom[0].Path != "data.bin" {
				t.Fatalf("status progress resume_from = %+v, want data.bin file status", event.ResumeFrom)
			}
		case ProgressStageChunk:
			chunkEvents++
			if len(event.Chunks) != 2 {
				t.Fatalf("chunk progress event chunks = %+v, want two batch entries", event.Chunks)
			}
			if event.PrivacyTotal.PaddedChunks == 0 || event.PrivacyTotal.BatchFrames == 0 || event.PrivacyTotal.PaddingBucketBytes != 64 {
				t.Fatalf("chunk progress privacy = %+v, want cumulative level 2 overhead", event.PrivacyTotal)
			}
		default:
			t.Fatalf("progress event stage = %q, want status or chunk", event.Stage)
		}
	}
	if statusEvents != 1 || chunkEvents != 2 {
		t.Fatalf("progress events status/chunk = %d/%d, want 1/2 events=%+v", statusEvents, chunkEvents, events)
	}
	lastChunk := events[len(events)-1]
	if lastChunk.BytesTotal != 12 || lastChunk.ChunksTotal != 4 {
		t.Fatalf("last chunk progress totals = %d/%d, want 12/4", lastChunk.BytesTotal, lastChunk.ChunksTotal)
	}
}

func TestClientRunStopsWhenProgressCallbackFailsWithAccumulatedPrivacy(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("abcdefgh"))
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-progress-fails"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 1
	progressErr := errors.New("progress sink failed")
	req.Progress = func(_ context.Context, event ProgressEvent) error {
		if event.Stage == ProgressStageChunk {
			return progressErr
		}
		return nil
	}

	got, err := newZeroJitterClient(server.URL, 4, server.Client()).Run(context.Background(), req)
	if !errors.Is(err, progressErr) {
		t.Fatalf("Client.Run(progress failure) error = %v, want progressErr", err)
	}
	if got.Bytes != 4 || got.Chunks != 1 {
		t.Fatalf("Client.Run(progress failure) bytes/chunks = %d/%d, want first accepted chunk", got.Bytes, got.Chunks)
	}
	if got.Privacy.PaddedChunks == 0 || got.Privacy.BatchFrames == 0 {
		t.Fatalf("Client.Run(progress failure).Privacy = %+v, want retained accepted payload overhead", got.Privacy)
	}
	var transferErr *TransferError
	if !errors.As(err, &transferErr) || transferErr.Privacy.PaddedChunks == 0 {
		t.Fatalf("Client.Run(progress failure) error = %T %+v, want TransferError with privacy overhead", err, err)
	}
}

func TestClientRunAppliesJitterOncePerReceiverRequest(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("abcdefghijkl"))
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-jitter-requests"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 2
	sourceSeq := &sequenceJitterSource{values: []int{0, 1, 2, 3, 4, 5}}
	sleeper := &recordingJitterSleeper{}

	got, err := (Client{
		BaseURL:       server.URL,
		ChunkSize:     3,
		Doer:          server.Client(),
		jitterSource:  sourceSeq,
		jitterSleeper: sleeper,
	}).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Client.Run(jitter requests) error = %v, want nil", err)
	}
	if got.Privacy.JitteredRequests != 6 || got.Privacy.JitterDelayMillis != 15 || got.Privacy.MaxJitterDelayMillis != 5 || got.Privacy.JitterBudgetMillis != 250 {
		t.Fatalf("Client.Run(jitter requests).Privacy = %+v, want six receiver-request jitter samples", got.Privacy)
	}
	wantDelays := []time.Duration{0, time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond, 4 * time.Millisecond, 5 * time.Millisecond}
	if !equalDurations(sleeper.delays, wantDelays) {
		t.Fatalf("jitter delays = %v, want %v", sleeper.delays, wantDelays)
	}
	recorder.mu.Lock()
	statusRequests := len(recorder.statusPaths)
	batchRequests := len(recorder.batchCounts)
	chunksInBatches := len(recorder.chunkData)
	commits := recorder.commits
	recorder.mu.Unlock()
	if statusRequests != 2 || batchRequests != 2 || chunksInBatches != 4 || commits != 1 {
		t.Fatalf("receiver request counts status/batches/chunks/commits = %d/%d/%d/%d, want 2/2/4/1", statusRequests, batchRequests, chunksInBatches, commits)
	}
	if sourceSeq.calls != 6 {
		t.Fatalf("jitter source calls = %d, want one per begin/status/batch/batch/status/commit receiver request", sourceSeq.calls)
	}
}

func TestClientRunPreservesDirectoriesAndSymlinks(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "docs"), 0o755); err != nil {
		t.Fatalf("os.Mkdir(docs) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(source, "docs", "file.txt"), []byte("hello"), 0o640); err != nil {
		t.Fatalf("os.WriteFile(file) error = %v, want nil", err)
	}
	if err := os.Symlink("file.txt", filepath.Join(source, "docs", "link.txt")); err != nil {
		t.Skipf("os.Symlink not available: %v", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}

	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()

	if _, err := (Client{BaseURL: server.URL, ChunkSize: 2}).Run(context.Background(), validTransferRequest(source, scanResult, "session-symlink")); err != nil {
		t.Fatalf("Client.Run(dir/symlink tree) error = %v, want nil", err)
	}
	if info, err := os.Lstat(filepath.Join(target, "docs")); err != nil || !info.IsDir() {
		t.Fatalf("published docs directory info = %v, err = %v, want directory", info, err)
	}
	if got, err := os.Readlink(filepath.Join(target, "docs", "link.txt")); err != nil || got != "file.txt" {
		t.Fatalf("published symlink target = %q, err = %v, want file.txt", got, err)
	}

	recorder.mu.Lock()
	begin := recorder.begin
	chunkPaths := append([]string(nil), recorder.chunkPaths...)
	recorder.mu.Unlock()
	if !manifestHas(begin.Manifest, "docs", protocol.FileKindDir) {
		t.Fatalf("begin manifest missing docs directory: %+v", begin.Manifest.Entries)
	}
	if !manifestHas(begin.Manifest, "docs/link.txt", protocol.FileKindSymlink) {
		t.Fatalf("begin manifest missing docs/link.txt symlink: %+v", begin.Manifest.Entries)
	}
	for _, path := range chunkPaths {
		if path != "docs/file.txt" {
			t.Fatalf("chunk uploaded for %q, want only docs/file.txt", path)
		}
	}
}

func TestClientRunRejectsSymlinkChangedBeforeCommit(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "data.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(data.bin) error = %v, want nil", err)
	}
	linkPath := filepath.Join(source, "link.bin")
	if err := os.Symlink("data.bin", linkPath); err != nil {
		t.Skipf("os.Symlink not available: %v", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	var commits int
	status := newStatusTracker()
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 8,
		Doer: withReceiverStatus(scanResult, "session-symlink-changed", nil, status.committed, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-symlink-changed",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				if err := os.Remove(linkPath); err != nil {
					return nil, err
				}
				if err := os.Symlink("other.bin", linkPath); err != nil {
					return nil, err
				}
				status.set(chunk.Path, chunk.Offset+int64(len(chunk.Data)))
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: chunk.Offset + int64(len(chunk.Data)),
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	_, err = client.Run(context.Background(), validTransferRequest(source, scanResult, "session-symlink-changed"))
	if !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("Client.Run(changed symlink) error = %v, want ErrSourceChanged", err)
	}
	if commits != 0 {
		t.Fatalf("commit calls = %d, want 0", commits)
	}
}

func TestClientRunRejectsSourceParentSymlinkBeforeUpload(t *testing.T) {
	source := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "docs"), 0o755); err != nil {
		t.Fatalf("os.Mkdir(source/docs) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(source, "docs", "data.bin"), []byte("inside"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source data) error = %v, want nil", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	if err := os.WriteFile(filepath.Join(outside, "data.bin"), []byte("outside"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(outside data) error = %v, want nil", err)
	}
	if err := os.RemoveAll(filepath.Join(source, "docs")); err != nil {
		t.Fatalf("os.RemoveAll(source/docs) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "docs")); err != nil {
		t.Skipf("os.Symlink not available: %v", err)
	}
	var calls atomic.Int64
	client := Client{
		BaseURL: "http://receiver.invalid",
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errors.New("network must not be called")
		}),
	}

	_, err = client.Run(context.Background(), validTransferRequest(source, scanResult, "session-parent-symlink"))
	if !errors.Is(err, pathguard.ErrUnsafePath) {
		t.Fatalf("Client.Run(parent symlink) error = %v, want ErrUnsafePath", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("network calls = %d, want 0", calls.Load())
	}
}

func TestClientRunRejectsSourceParentSymlinkAfterBegin(t *testing.T) {
	source := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "docs"), 0o755); err != nil {
		t.Fatalf("os.Mkdir(source/docs) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(source, "docs", "data.bin"), []byte("inside"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source data) error = %v, want nil", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	if err := os.WriteFile(filepath.Join(outside, "data.bin"), []byte("outside"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(outside data) error = %v, want nil", err)
	}
	var chunks int
	client := Client{
		BaseURL: "http://receiver.invalid",
		Doer: withReceiverStatus(scanResult, "session-parent-symlink-after-begin", nil, nil, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				if err := os.RemoveAll(filepath.Join(source, "docs")); err != nil {
					return nil, err
				}
				if err := os.Symlink(outside, filepath.Join(source, "docs")); err != nil {
					return nil, err
				}
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-parent-symlink-after-begin",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				chunks++
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	_, err = client.Run(context.Background(), validTransferRequest(source, scanResult, "session-parent-symlink-after-begin"))
	if !errors.Is(err, pathguard.ErrUnsafePath) {
		t.Fatalf("Client.Run(parent symlink after begin) error = %v, want ErrUnsafePath", err)
	}
	if chunks != 0 {
		t.Fatalf("chunk calls = %d, want 0", chunks)
	}
}

func TestClientRunRejectsDirectoryChangedBeforeCommit(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "docs"), 0o755); err != nil {
		t.Fatalf("os.Mkdir(source/docs) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(source, "docs", "data.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source data) error = %v, want nil", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	var commits int
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 16,
		Doer: withReceiverStatus(scanResult, "session-dir-changed", nil, nil, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-dir-changed",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				if err := os.RemoveAll(filepath.Join(source, "docs")); err != nil {
					return nil, err
				}
				if err := os.WriteFile(filepath.Join(source, "docs"), []byte("not a dir"), 0o600); err != nil {
					return nil, err
				}
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: chunk.Offset + int64(len(chunk.Data)),
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	_, err = client.Run(context.Background(), validTransferRequest(source, scanResult, "session-dir-changed"))
	if !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("Client.Run(changed dir) error = %v, want ErrSourceChanged", err)
	}
	if commits != 0 {
		t.Fatalf("commit calls = %d, want 0", commits)
	}
}

func TestClientRunRejectsRegularFileChangedBeforeCommit(t *testing.T) {
	source := t.TempDir()
	path := filepath.Join(source, "data.bin")
	original := []byte("payload")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("os.WriteFile(original) error = %v, want nil", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	originalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(original) error = %v, want nil", err)
	}
	mutated := []byte("PAYLOAD")
	var commits int
	status := newStatusTracker()
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 16,
		Doer: withReceiverStatus(scanResult, "session-file-changed", nil, status.committed, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-file-changed",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				if err := os.WriteFile(path, mutated, 0o600); err != nil {
					return nil, err
				}
				if err := os.Chtimes(path, originalInfo.ModTime(), originalInfo.ModTime()); err != nil {
					return nil, err
				}
				status.set(chunk.Path, chunk.Offset+int64(len(chunk.Data)))
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: chunk.Offset + int64(len(chunk.Data)),
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	_, err = client.Run(context.Background(), validTransferRequest(source, scanResult, "session-file-changed"))
	if !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("Client.Run(changed file) error = %v, want ErrSourceChanged", err)
	}
	if commits != 0 {
		t.Fatalf("commit calls = %d, want 0", commits)
	}
}

func TestClientRunHonorsCancellationBeforeCommit(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var commits int
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 16,
		Doer: withReceiverStatus(scanResult, "session-cancel-before-commit", nil, nil, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-cancel-before-commit",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				cancel()
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: chunk.Offset + int64(len(chunk.Data)),
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	_, err := client.Run(ctx, validTransferRequest(source, scanResult, "session-cancel-before-commit"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Client.Run(cancel before commit) error = %v, want context.Canceled", err)
	}
	if commits != 0 {
		t.Fatalf("commit calls = %d, want 0", commits)
	}
}

func TestClientRunRejectsOversizedSuccessResponse(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	client := Client{
		BaseURL: "http://receiver.invalid",
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/v1/sessions" {
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Status:     "202 Accepted",
					Body:       io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), maxResponseBodyBytes+1))),
				}, nil
			}
			return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
		}),
	}

	_, err := client.Run(context.Background(), validTransferRequest(source, scanResult, "session-oversized-success"))
	if err == nil || !strings.Contains(err.Error(), "response body too large") {
		t.Fatalf("Client.Run(oversized success) error = %v, want response body too large", err)
	}
}

func TestClientRunRejectsMismatchedScanRoot(t *testing.T) {
	_, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "data.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(other data) error = %v, want nil", err)
	}
	var calls atomic.Int64
	client := Client{
		BaseURL: "http://receiver.invalid",
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errors.New("network must not be called")
		}),
	}

	_, err := client.Run(context.Background(), validTransferRequest(other, scanResult, "session-root-mismatch"))
	if err == nil || !strings.Contains(err.Error(), "does not match scan root") {
		t.Fatalf("Client.Run(scan root mismatch) error = %v, want scan root mismatch", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("network calls = %d, want 0", calls.Load())
	}
}

func TestClientRunResumesInterruptedUploadFromReceiverStatus(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	chunkSize := 16
	sourceFile := filepath.Join(source, "large.bin")
	if err := writePatternFile(sourceFile, chunkSize*3+5); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-resume-interrupted"))
	req.PrivacyPolicy.BatchMaxCount = 1

	first := Client{
		BaseURL:       server.URL,
		ChunkSize:     chunkSize,
		jitterSource:  zeroJitterSource{},
		jitterSleeper: &recordingJitterSleeper{},
		Doer: &failAfterAcceptedChunkDoer{
			base:      server.Client(),
			failAfter: 1,
		},
	}
	firstGot, err := first.Run(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption after accepted chunk") {
		t.Fatalf("first Client.Run(interrupted) error = %v, want simulated interruption", err)
	}
	if firstGot.Privacy.PaddedChunks != 1 || firstGot.Privacy.PaddingBytes == 0 {
		t.Fatalf("first Client.Run(interrupted).Privacy = %+v, want persisted padding overhead for accepted chunk", firstGot.Privacy)
	}
	var transferErr *TransferError
	if !errors.As(err, &transferErr) || transferErr.Privacy.PaddedChunks != 1 {
		t.Fatalf("first Client.Run(interrupted) error = %T %+v, want TransferError with one padded chunk", err, err)
	}
	recorder.reset()

	got, err := newZeroJitterClient(server.URL, chunkSize, server.Client()).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("second Client.Run(resume) error = %v, want nil", err)
	}
	if got.Bytes != int64(chunkSize*2+5) {
		t.Fatalf("second Client.Run(resume).Bytes = %d, want only remaining bytes %d", got.Bytes, chunkSize*2+5)
	}
	recorder.mu.Lock()
	chunkOffsets := append([]int64(nil), recorder.chunkOffsets...)
	statusPaths := append([]string(nil), recorder.statusPaths...)
	recorder.mu.Unlock()
	if len(statusPaths) == 0 {
		t.Fatalf("status calls = 0, want receiver status before resume")
	}
	if len(chunkOffsets) == 0 || chunkOffsets[0] != int64(chunkSize) {
		t.Fatalf("resume chunk offsets = %+v, want first offset %d", chunkOffsets, chunkSize)
	}
	if got.Privacy.PaddedChunks != 3 || got.Privacy.PaddingBucketBytes == 0 {
		t.Fatalf("second Client.Run(resume).Privacy = %+v, want only resumed chunks padded", got.Privacy)
	}
	if hashFile(t, filepath.Join(target, "large.bin")) != hashFile(t, sourceFile) {
		t.Fatalf("published file digest differs from source")
	}
}

func TestClientRunResumesAfterInterruptedMultiRecordBatch(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	chunkSize := 16
	sourceFile := filepath.Join(source, "large.bin")
	if err := writePatternFile(sourceFile, chunkSize*4); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-resume-interrupted-batch"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 2

	first := Client{
		BaseURL:       server.URL,
		ChunkSize:     chunkSize,
		jitterSource:  zeroJitterSource{},
		jitterSleeper: &recordingJitterSleeper{},
		Doer: &failAfterAcceptedChunkDoer{
			base:      server.Client(),
			failAfter: 1,
		},
	}
	firstGot, err := first.Run(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption after accepted chunk") {
		t.Fatalf("first Client.Run(interrupted batch) error = %v, want simulated interruption", err)
	}
	if firstGot.Privacy.BatchFrames != 1 || firstGot.Privacy.BatchedChunks != 2 || firstGot.Privacy.MaxBatchCount != 2 {
		t.Fatalf("first Client.Run(interrupted batch).Privacy = %+v, want one two-record batch", firstGot.Privacy)
	}
	recorder.reset()

	got, err := newZeroJitterClient(server.URL, chunkSize, server.Client()).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("second Client.Run(resume batch) error = %v, want nil", err)
	}
	if got.Bytes != int64(chunkSize*2) {
		t.Fatalf("second Client.Run(resume batch).Bytes = %d, want only remaining bytes %d", got.Bytes, chunkSize*2)
	}
	recorder.mu.Lock()
	batchCounts := append([]int(nil), recorder.batchCounts...)
	chunkOffsets := append([]int64(nil), recorder.chunkOffsets...)
	recorder.mu.Unlock()
	if len(batchCounts) == 0 || batchCounts[0] != 2 {
		t.Fatalf("resume batch counts = %+v, want first resumed batch count 2", batchCounts)
	}
	if len(chunkOffsets) == 0 || chunkOffsets[0] != int64(chunkSize*2) {
		t.Fatalf("resume chunk offsets = %+v, want first offset %d", chunkOffsets, chunkSize*2)
	}
	if hashFile(t, filepath.Join(target, "large.bin")) != hashFile(t, sourceFile) {
		t.Fatalf("published file digest differs from source")
	}
}

func TestClientRunUsesReceiverStatusOverBeginResumeHint(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	sessionID := "session-status-over-begin"
	var committed int64 = 3
	var chunkOffsets []int64
	var chunkData [][]byte
	var commits int
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 2,
		Doer: withReceiverStatus(scanResult, sessionID, nil, func(path string) int64 {
			if path == "data.bin" {
				return committed
			}
			return 0
		}, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
					ResumeFrom: []protocol.FileStatus{
						{Path: "data.bin", ExpectedSize: 7, CommittedSize: 0, ExpectedDigest: scanEntry(t, scanResult, "data.bin").Digest, Complete: false},
					},
				}), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				chunkOffsets = append(chunkOffsets, chunk.Offset)
				chunkData = append(chunkData, append([]byte(nil), chunk.Data...))
				committed = chunk.Offset + int64(len(chunk.Data))
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: committed,
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	if _, err := client.Run(context.Background(), validTransferRequest(source, scanResult, sessionID)); err != nil {
		t.Fatalf("Client.Run(status over begin) error = %v, want nil", err)
	}
	if len(chunkOffsets) == 0 || chunkOffsets[0] != 3 {
		t.Fatalf("chunk offsets = %+v, want first offset 3 from status", chunkOffsets)
	}
	uploaded := string(bytes.Join(chunkData, nil))
	if uploaded != "load" {
		t.Fatalf("uploaded suffix = %q, want %q", uploaded, "load")
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1", commits)
	}
}

func TestClientRunRefreshesReceiverStatusAfterOffsetConflict(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	sessionID := "session-refresh-after-conflict"
	var committed int64 = 3
	var statusCalls int
	var chunkOffsets []int64
	var chunkData [][]byte
	var commits int
	client := Client{
		BaseURL:       "http://receiver.invalid",
		ChunkSize:     2,
		jitterSource:  zeroJitterSource{},
		jitterSleeper: &recordingJitterSleeper{},
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/sessions/" + sessionID + "/status":
				statusCalls++
				return jsonHTTPResponse(http.StatusOK, statusFromScan(scanResult, sessionID, protocol.SessionStateValidated, func(path string) int64 {
					if path == "data.bin" {
						return committed
					}
					return 0
				})), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				chunkOffsets = append(chunkOffsets, chunk.Offset)
				if len(chunkOffsets) == 1 {
					committed = 5
					return jsonHTTPResponse(http.StatusConflict, protocol.ErrorResponse{
						Code:    protocol.ErrorCodeConflict,
						Message: "expected offset 5",
					}), nil
				}
				chunkData = append(chunkData, append([]byte(nil), chunk.Data...))
				committed = chunk.Offset + int64(len(chunk.Data))
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: committed,
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		}),
	}

	if _, err := client.Run(context.Background(), validTransferRequest(source, scanResult, sessionID)); err != nil {
		t.Fatalf("Client.Run(refresh after conflict) error = %v, want nil", err)
	}
	if got, want := chunkOffsets, []int64{3, 5}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("chunk offsets = %+v, want %+v", got, want)
	}
	if string(bytes.Join(chunkData, nil)) != "ad" {
		t.Fatalf("uploaded suffix after refresh = %q, want %q", string(bytes.Join(chunkData, nil)), "ad")
	}
	if statusCalls < 2 {
		t.Fatalf("status calls = %d, want initial plus conflict refresh", statusCalls)
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1", commits)
	}
}

func TestClientRunTreatsZeroByteCompleteStatusAsConflictRecoveryProgress(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "empty.txt", nil)
	sessionID := "session-zero-byte-conflict-refresh"
	var complete bool
	var statusCalls int
	var chunkCalls int
	var commits int
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 2,
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/sessions/" + sessionID + "/status":
				statusCalls++
				return jsonHTTPResponse(http.StatusOK, zeroByteStatusFromScan(scanResult, sessionID, complete)), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				if bytes.Contains(body, []byte(`"data":null`)) {
					return nil, fmt.Errorf("zero-byte completion encoded data as null, want empty base64 string")
				}
				if _, err := decodeChunkRequestBytes(req, body, &chunk); err != nil {
					return nil, err
				}
				if chunk.Offset != 0 || len(chunk.Data) != 0 || chunk.Digest != protocol.EmptySHA256Digest || !chunk.Final {
					return nil, fmt.Errorf("zero-byte chunk = %+v, want final explicit empty completion", chunk)
				}
				chunkCalls++
				complete = true
				return jsonHTTPResponse(http.StatusConflict, protocol.ErrorResponse{
					Code:    protocol.ErrorCodeConflict,
					Message: "completion accepted by a concurrent receiver attempt",
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		}),
	}

	got, err := client.Run(context.Background(), validTransferRequest(source, scanResult, sessionID))
	if err != nil {
		t.Fatalf("Client.Run(zero-byte conflict refresh) error = %v, want nil", err)
	}
	if got.Files != 1 || got.Bytes != 0 || got.Chunks != 0 {
		t.Fatalf("Client.Run(zero-byte conflict refresh) result files/bytes/chunks = %d/%d/%d, want 1/0/0 after recovered completion", got.Files, got.Bytes, got.Chunks)
	}
	if statusCalls < 2 {
		t.Fatalf("status calls = %d, want initial plus conflict refresh", statusCalls)
	}
	if chunkCalls != 1 || commits != 1 {
		t.Fatalf("chunkCalls=%d commits=%d, want 1/1", chunkCalls, commits)
	}
}

func TestClientRunAdvancesAfterDuplicateChunkResponse(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	sessionID := "session-duplicate-advanced"
	var committed int64 = 3
	var chunkOffsets []int64
	var chunkStates []protocol.ChunkState
	var commits int
	client := Client{
		BaseURL:       "http://receiver.invalid",
		ChunkSize:     2,
		jitterSource:  zeroJitterSource{},
		jitterSleeper: &recordingJitterSleeper{},
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/sessions/" + sessionID + "/status":
				return jsonHTTPResponse(http.StatusOK, statusFromScan(scanResult, sessionID, protocol.SessionStateValidated, func(path string) int64 {
					if path == "data.bin" {
						return committed
					}
					return 0
				})), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				chunkOffsets = append(chunkOffsets, chunk.Offset)
				if len(chunkOffsets) == 1 {
					committed = 5
					chunkStates = append(chunkStates, protocol.ChunkStateDuplicate)
					return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
						SessionID:     chunk.SessionID,
						Path:          chunk.Path,
						CommittedSize: committed,
						ChunkState:    protocol.ChunkStateDuplicate,
						Complete:      false,
					}), nil
				}
				committed = chunk.Offset + int64(len(chunk.Data))
				chunkStates = append(chunkStates, protocol.ChunkStateAccepted)
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: committed,
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		}),
	}

	if _, err := client.Run(context.Background(), validTransferRequest(source, scanResult, sessionID)); err != nil {
		t.Fatalf("Client.Run(duplicate advanced) error = %v, want nil", err)
	}
	if got, want := chunkOffsets, []int64{3, 5}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("chunk offsets = %+v, want %+v", got, want)
	}
	if got, want := chunkStates, []protocol.ChunkState{protocol.ChunkStateDuplicate, protocol.ChunkStateAccepted}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("chunk states = %+v, want %+v", got, want)
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1", commits)
	}
}

func TestClientRunAdvancesAfterDuplicateChunkBatchResponse(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("abcdefgh"))
	sessionID := "session-duplicate-batch-advanced"
	const bucket = 64
	var committed int64
	var batchOffsets [][]int64
	var batchStates [][]protocol.ChunkState
	var commits int
	client := Client{
		BaseURL:       "http://receiver.invalid",
		ChunkSize:     2,
		jitterSource:  zeroJitterSource{},
		jitterSleeper: &recordingJitterSleeper{},
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/sessions/" + sessionID + "/status":
				return jsonHTTPResponse(http.StatusOK, statusFromScan(scanResult, sessionID, protocol.SessionStateValidated, func(path string) int64 {
					if path == "data.bin" {
						return committed
					}
					return 0
				})), nil
			case "/v1/chunk-batches":
				var batch protocol.ChunkBatchUploadRequest
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				if _, err := decodeChunkBatchRequestBytesWithBucket(req, body, &batch, bucket); err != nil {
					return nil, err
				}
				offsets := make([]int64, 0, len(batch.Chunks))
				states := make([]protocol.ChunkState, 0, len(batch.Chunks))
				resp := protocol.ChunkBatchUploadResponse{
					SessionID: batch.SessionID,
					Chunks:    make([]protocol.ChunkUploadResponse, 0, len(batch.Chunks)),
				}
				if len(batchOffsets) == 0 {
					committed = 6
					for _, chunk := range batch.Chunks {
						offsets = append(offsets, chunk.Offset)
						states = append(states, protocol.ChunkStateDuplicate)
						resp.Chunks = append(resp.Chunks, protocol.ChunkUploadResponse{
							SessionID:     chunk.SessionID,
							Path:          chunk.Path,
							CommittedSize: committed,
							ChunkState:    protocol.ChunkStateDuplicate,
							Complete:      false,
						})
					}
				} else {
					for _, chunk := range batch.Chunks {
						offsets = append(offsets, chunk.Offset)
						committed = chunk.Offset + int64(len(chunk.Data))
						states = append(states, protocol.ChunkStateAccepted)
						resp.Chunks = append(resp.Chunks, protocol.ChunkUploadResponse{
							SessionID:     chunk.SessionID,
							Path:          chunk.Path,
							CommittedSize: committed,
							ChunkState:    protocol.ChunkStateAccepted,
							Complete:      chunk.Final,
						})
					}
				}
				batchOffsets = append(batchOffsets, offsets)
				batchStates = append(batchStates, states)
				return jsonHTTPResponse(http.StatusAccepted, resp), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		}),
	}
	req := withLevel2Privacy(validTransferRequest(source, scanResult, sessionID))
	req.PrivacyPolicy.PaddingBucket = bucket
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 2

	if _, err := client.Run(context.Background(), req); err != nil {
		t.Fatalf("Client.Run(duplicate batch advanced) error = %v, want nil", err)
	}
	if len(batchOffsets) != 2 {
		t.Fatalf("batch calls = %d, want 2 offsets=%+v", len(batchOffsets), batchOffsets)
	}
	if got, want := batchOffsets[0], []int64{0, 2}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("first batch offsets = %+v, want %+v", got, want)
	}
	if got, want := batchOffsets[1], []int64{6}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("second batch offsets = %+v, want %+v", got, want)
	}
	if got := batchStates[0]; len(got) != 2 || got[0] != protocol.ChunkStateDuplicate || got[1] != protocol.ChunkStateDuplicate {
		t.Fatalf("first batch states = %+v, want duplicate responses", got)
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1", commits)
	}
}

func TestClientRunKeepsConflictRefreshBudgetAfterDuplicateBatchAdvance(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("abcdefghij"))
	sessionID := "session-duplicate-batch-before-conflict"
	const bucket = 64
	var committed int64
	var batchCalls int
	var statusCalls int
	var batchOffsets [][]int64
	var commits int
	client := Client{
		BaseURL:       "http://receiver.invalid",
		ChunkSize:     2,
		jitterSource:  zeroJitterSource{},
		jitterSleeper: &recordingJitterSleeper{},
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/sessions/" + sessionID + "/status":
				statusCalls++
				return jsonHTTPResponse(http.StatusOK, statusFromScan(scanResult, sessionID, protocol.SessionStateValidated, func(path string) int64 {
					if path == "data.bin" {
						return committed
					}
					return 0
				})), nil
			case "/v1/chunk-batches":
				var batch protocol.ChunkBatchUploadRequest
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				if _, err := decodeChunkBatchRequestBytesWithBucket(req, body, &batch, bucket); err != nil {
					return nil, err
				}
				offsets := make([]int64, 0, len(batch.Chunks))
				for _, chunk := range batch.Chunks {
					offsets = append(offsets, chunk.Offset)
				}
				batchOffsets = append(batchOffsets, offsets)
				batchCalls++
				if batchCalls == 1 {
					committed = 6
					resp := protocol.ChunkBatchUploadResponse{SessionID: batch.SessionID}
					for _, chunk := range batch.Chunks {
						resp.Chunks = append(resp.Chunks, protocol.ChunkUploadResponse{
							SessionID:     chunk.SessionID,
							Path:          chunk.Path,
							CommittedSize: committed,
							ChunkState:    protocol.ChunkStateDuplicate,
							Complete:      false,
						})
					}
					return jsonHTTPResponse(http.StatusAccepted, resp), nil
				}
				if batchCalls == 2 {
					committed = 8
					return jsonHTTPResponse(http.StatusConflict, protocol.ErrorResponse{
						Code:    protocol.ErrorCodeConflict,
						Message: "expected offset 8",
					}), nil
				}
				resp := protocol.ChunkBatchUploadResponse{SessionID: batch.SessionID}
				for _, chunk := range batch.Chunks {
					committed = chunk.Offset + int64(len(chunk.Data))
					resp.Chunks = append(resp.Chunks, protocol.ChunkUploadResponse{
						SessionID:     chunk.SessionID,
						Path:          chunk.Path,
						CommittedSize: committed,
						ChunkState:    protocol.ChunkStateAccepted,
						Complete:      chunk.Final,
					})
				}
				return jsonHTTPResponse(http.StatusAccepted, resp), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		}),
	}
	req := withLevel2Privacy(validTransferRequest(source, scanResult, sessionID))
	req.PrivacyPolicy.PaddingBucket = bucket
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 2

	if _, err := client.Run(context.Background(), req); err != nil {
		t.Fatalf("Client.Run(duplicate batch then conflict) error = %v, want nil", err)
	}
	if got, want := batchOffsets, [][]int64{{0, 2}, {6, 8}, {8}}; len(got) != len(want) ||
		len(got[0]) != len(want[0]) || got[0][0] != want[0][0] || got[0][1] != want[0][1] ||
		len(got[1]) != len(want[1]) || got[1][0] != want[1][0] || got[1][1] != want[1][1] ||
		len(got[2]) != len(want[2]) || got[2][0] != want[2][0] {
		t.Fatalf("batch offsets = %+v, want %+v", got, want)
	}
	if statusCalls < 2 {
		t.Fatalf("status calls = %d, want initial plus conflict refresh", statusCalls)
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1", commits)
	}
}

func TestClientRunConvergesWhenReceiverPublishesDuringConflictRefresh(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	sessionID := "session-published-during-refresh"
	var state = protocol.SessionStateValidated
	var committed int64 = 3
	var chunks int
	var commits int
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 2,
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/sessions/" + sessionID + "/status":
				status := statusFromScan(scanResult, sessionID, state, func(path string) int64 {
					if state == protocol.SessionStatePublished {
						return 0
					}
					if path == "data.bin" {
						return committed
					}
					return 0
				})
				return jsonHTTPResponse(http.StatusOK, status), nil
			case "/v1/chunks":
				chunks++
				state = protocol.SessionStatePublished
				return jsonHTTPResponse(http.StatusConflict, protocol.ErrorResponse{
					Code:    protocol.ErrorCodeConflict,
					Message: "session is terminal",
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		}),
	}

	if _, err := client.Run(context.Background(), validTransferRequest(source, scanResult, sessionID)); err != nil {
		t.Fatalf("Client.Run(published during refresh) error = %v, want nil", err)
	}
	if chunks != 1 {
		t.Fatalf("chunk calls = %d, want 1 conflict before published refresh", chunks)
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1 idempotent commit", commits)
	}
}

func TestClientRunRejectsStaleReceiverStatus(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	req := validTransferRequest(source, scanResult, "session-stale-status")
	beginReq, _, err := BuildBeginRequest(req)
	if err != nil {
		t.Fatalf("BuildBeginRequest(%+v) error = %v, want nil", req, err)
	}
	file := beginReq.Manifest.Entries[0]
	tests := []struct {
		name   string
		status protocol.SessionStatusResponse
	}{
		{
			name: "offset beyond size",
			status: protocol.SessionStatusResponse{
				SessionID: req.SessionID,
				State:     protocol.SessionStateValidated,
				Files: []protocol.FileStatus{{
					Path:           file.Path,
					ExpectedSize:   file.Size,
					CommittedSize:  file.Size + 1,
					ExpectedDigest: file.Digest,
				}},
			},
		},
		{
			name: "digest mismatch",
			status: protocol.SessionStatusResponse{
				SessionID: req.SessionID,
				State:     protocol.SessionStateValidated,
				Files: []protocol.FileStatus{{
					Path:           file.Path,
					ExpectedSize:   file.Size,
					CommittedSize:  0,
					ExpectedDigest: digestBytes([]byte("other")),
				}},
			},
		},
		{
			name: "unknown path",
			status: protocol.SessionStatusResponse{
				SessionID: req.SessionID,
				State:     protocol.SessionStateValidated,
				Files: []protocol.FileStatus{{
					Path:           "other.bin",
					ExpectedSize:   file.Size,
					CommittedSize:  0,
					ExpectedDigest: file.Digest,
				}},
			},
		},
		{
			name: "duplicate path",
			status: protocol.SessionStatusResponse{
				SessionID: req.SessionID,
				State:     protocol.SessionStateValidated,
				Files: []protocol.FileStatus{
					{Path: file.Path, ExpectedSize: file.Size, CommittedSize: 0, ExpectedDigest: file.Digest},
					{Path: file.Path, ExpectedSize: file.Size, CommittedSize: 0, ExpectedDigest: file.Digest},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var chunks int
			var commits int
			client := Client{
				BaseURL: "http://receiver.invalid",
				Doer: doerFunc(func(httpReq *http.Request) (*http.Response, error) {
					switch httpReq.URL.Path {
					case "/v1/sessions":
						return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
							SessionID: req.SessionID,
							State:     protocol.SessionStateValidated,
						}), nil
					case "/v1/sessions/" + req.SessionID + "/status":
						return jsonHTTPResponse(http.StatusOK, tt.status), nil
					case "/v1/chunks":
						chunks++
						return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{}), nil
					case "/v1/commit":
						commits++
						return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
					default:
						return nil, fmt.Errorf("unexpected path %s", httpReq.URL.Path)
					}
				}),
			}

			_, err := client.Run(context.Background(), req)
			if !errors.Is(err, ErrReceiverStatusInvalid) {
				t.Fatalf("Client.Run(%s) error = %v, want ErrReceiverStatusInvalid", tt.name, err)
			}
			if chunks != 0 || commits != 0 {
				t.Fatalf("chunks=%d commits=%d, want 0/0", chunks, commits)
			}
		})
	}
}

func TestClientRunRejectsSourceChangedBeforeResume(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	path := filepath.Join(source, "data.bin")
	originalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", path, err)
	}
	if err := os.WriteFile(path, []byte("PAYLOAD"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(mutated) error = %v, want nil", err)
	}
	if err := os.Chtimes(path, originalInfo.ModTime(), originalInfo.ModTime()); err != nil {
		t.Fatalf("os.Chtimes(mutated) error = %v, want nil", err)
	}
	var chunks int
	var commits int
	sessionID := "session-source-changed-before-resume"
	client := Client{
		BaseURL: "http://receiver.invalid",
		Doer: withReceiverStatus(scanResult, sessionID, nil, func(path string) int64 {
			if path == "data.bin" {
				return 3
			}
			return 0
		}, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				chunks++
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	_, err = client.Run(context.Background(), validTransferRequest(source, scanResult, sessionID))
	if !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("Client.Run(source changed before resume) error = %v, want ErrSourceChanged", err)
	}
	if chunks != 0 || commits != 0 {
		t.Fatalf("chunks=%d commits=%d, want 0/0", chunks, commits)
	}
}

func TestClientRunTreatsAlreadyPublishedSessionAsComplete(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := validTransferRequest(source, scanResult, "session-already-published")
	if _, err := (Client{BaseURL: server.URL, ChunkSize: 3, Doer: server.Client()}).Run(context.Background(), req); err != nil {
		t.Fatalf("first Client.Run(publish) error = %v, want nil", err)
	}
	recorder.reset()

	if _, err := (Client{BaseURL: server.URL, ChunkSize: 3, Doer: server.Client()}).Run(context.Background(), req); err != nil {
		t.Fatalf("second Client.Run(published replay) error = %v, want nil", err)
	}
	recorder.mu.Lock()
	chunks := len(recorder.chunkOffsets)
	commits := recorder.commits
	recorder.mu.Unlock()
	if chunks != 0 {
		t.Fatalf("chunk calls after published replay = %d, want 0", chunks)
	}
	if commits != 1 {
		t.Fatalf("commit calls after published replay = %d, want 1 idempotent commit", commits)
	}
	if hashFile(t, filepath.Join(target, "data.bin")) != hashFile(t, filepath.Join(source, "data.bin")) {
		t.Fatalf("published file digest differs from source")
	}
}

func TestClientRunRetriesStagedSessionCommitWithoutChunks(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	sessionID := "session-staged-retry"
	var chunks int
	var commits int
	client := Client{
		BaseURL: "http://receiver.invalid",
		Doer: withReceiverStatus(scanResult, sessionID, func() protocol.SessionState {
			return protocol.SessionStateStaged
		}, func(path string) int64 {
			if path == "data.bin" {
				return int64(len("payload"))
			}
			return 0
		}, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStateStaged,
				}), nil
			case "/v1/chunks":
				chunks++
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: sessionID,
					State:     protocol.SessionStatePublished,
					ReceiptID: sessionID,
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	if _, err := client.Run(context.Background(), validTransferRequest(source, scanResult, sessionID)); err != nil {
		t.Fatalf("Client.Run(staged retry) error = %v, want nil", err)
	}
	if chunks != 0 {
		t.Fatalf("chunk calls = %d, want 0", chunks)
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1", commits)
	}
}

func TestClientRunRefusesNeedsRepairSession(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	target := t.TempDir()
	store := receiver.FileStore{TargetRoot: target}
	req := validTransferRequest(source, scanResult, "session-needs-repair")
	beginReq, _, err := BuildBeginRequest(req)
	if err != nil {
		t.Fatalf("BuildBeginRequest(%+v) error = %v, want nil", req, err)
	}
	if _, err := store.Begin(beginReq); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", beginReq, err)
	}
	chunk := protocol.ChunkUploadRequest{
		SessionID: req.SessionID,
		Path:      "data.bin",
		Offset:    0,
		Data:      []byte("pay"),
		Digest:    digestBytes([]byte("pay")),
	}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: req.EndedAt}
	if _, err := store.Commit(commitReq); err == nil {
		t.Fatalf("FileStore.Commit(incomplete) error = nil, want integrity failure")
	}
	recorder := &recordingHandler{next: receiver.NewHandler(store)}
	server := httptest.NewServer(recorder)
	defer server.Close()

	_, err = (Client{BaseURL: server.URL, ChunkSize: 3, Doer: server.Client()}).Run(context.Background(), req)
	if !errors.Is(err, ErrReceiverNeedsRepair) {
		t.Fatalf("Client.Run(needs_repair) error = %v, want ErrReceiverNeedsRepair", err)
	}
	recorder.mu.Lock()
	chunks := len(recorder.chunkOffsets)
	commits := recorder.commits
	recorder.mu.Unlock()
	if chunks != 0 || commits != 0 {
		t.Fatalf("chunks=%d commits=%d, want 0/0", chunks, commits)
	}
}

func TestClientRunUsesCommitTimeWhenEndedAtUnset(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	start := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	commitTime := start.Add(5 * time.Minute)
	calls := 0
	var gotCommit protocol.CommitSessionRequest
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 16,
		Now: func() time.Time {
			calls++
			if calls == 1 {
				return start
			}
			return commitTime
		},
		Doer: withReceiverStatus(scanResult, "session-commit-time", nil, func(path string) int64 {
			if path == "data.bin" {
				return int64(len("payload"))
			}
			return 0
		}, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-commit-time",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: chunk.Offset + int64(len(chunk.Data)),
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/commit":
				if err := json.NewDecoder(req.Body).Decode(&gotCommit); err != nil {
					return nil, err
				}
				_ = req.Body.Close()
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{
					SessionID: "session-commit-time",
					State:     protocol.SessionStatePublished,
					ReceiptID: "session-commit-time",
				}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}
	req := validTransferRequest(source, scanResult, "session-commit-time")
	req.EndedAt = time.Time{}

	if _, err := client.Run(context.Background(), req); err != nil {
		t.Fatalf("Client.Run(default ended_at) error = %v, want nil", err)
	}
	if !gotCommit.EndedAt.Equal(commitTime) {
		t.Fatalf("commit ended_at = %s, want %s", gotCommit.EndedAt, commitTime)
	}
}

func TestClientRunBlocksScanErrorBeforeNetwork(t *testing.T) {
	source := t.TempDir()
	var calls atomic.Int64
	client := Client{
		BaseURL: "http://receiver.invalid",
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errors.New("network must not be called")
		}),
	}
	result := scan.Result{
		Root: filepath.ToSlash(source),
		Audit: []audit.Record{
			audit.New("secret.txt", "", audit.SeverityWarning, "scan_error", "walk error"),
		},
	}

	_, err := client.Run(context.Background(), validTransferRequest(source, result, "session-scan-error"))
	if err == nil || !errors.Is(err, ErrScanBlocked) {
		t.Fatalf("Client.Run(scan_error) error = %v, want source scan error", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("network calls = %d, want 0", calls.Load())
	}
}

func TestClientRunStopsOnChunkIntegrityFailure(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	var chunks int
	var commits int
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 4,
		Doer: withReceiverStatus(scanResult, "session-integrity", nil, nil, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-integrity",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				chunks++
				return jsonHTTPResponse(http.StatusUnprocessableEntity, protocol.ErrorResponse{
					Code:    protocol.ErrorCodeIntegrity,
					Message: "chunk digest mismatch",
				}), nil
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
			default:
				return jsonHTTPResponse(http.StatusNotFound, protocol.ErrorResponse{Code: protocol.ErrorCodeNotFound}), nil
			}
		})),
	}

	_, err := client.Run(context.Background(), validTransferRequest(source, scanResult, "session-integrity"))
	var remote *RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Client.Run(chunk integrity) error = %T %v, want RemoteError", err, err)
	}
	if remote.StatusCode != http.StatusUnprocessableEntity || remote.Code != protocol.ErrorCodeIntegrity {
		t.Fatalf("RemoteError = %+v, want status 422 code integrity_failure", remote)
	}
	if chunks != 1 {
		t.Fatalf("chunk calls = %d, want 1", chunks)
	}
	if commits != 0 {
		t.Fatalf("commit calls = %d, want 0", commits)
	}
}

func TestClientRunHonorsCancellationDuringChunkUpload(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var commits int
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 4,
		Doer: withReceiverStatus(scanResult, "session-cancel", nil, nil, doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-cancel",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/chunks":
				cancel()
				<-req.Context().Done()
				return nil, req.Context().Err()
			case "/v1/commit":
				commits++
				return jsonHTTPResponse(http.StatusOK, protocol.CommitSessionResponse{}), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		})),
	}

	_, err := client.Run(ctx, validTransferRequest(source, scanResult, "session-cancel"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Client.Run(cancel) error = %v, want context.Canceled", err)
	}
	if commits != 0 {
		t.Fatalf("commit calls = %d, want 0", commits)
	}
}

func TestClientRunHonorsCancellationDuringJitterBeforeRequest(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	ctx, cancel := context.WithCancel(context.Background())
	sleeper := &cancelingJitterSleeper{cancel: cancel}
	var networkCalls atomic.Int32
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-jitter-cancel"))
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 4,
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			networkCalls.Add(1)
			return nil, errors.New("network must not be called")
		}),
		jitterSource:  &sequenceJitterSource{values: []int{10}},
		jitterSleeper: sleeper,
	}

	got, err := client.Run(ctx, req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Client.Run(jitter cancel) error = %v, want context.Canceled", err)
	}
	if !got.Privacy.Empty() {
		t.Fatalf("Client.Run(jitter cancel).Privacy = %+v, want no applied overhead before completed jitter request", got.Privacy)
	}
	if sleeper.delay != 10*time.Millisecond {
		t.Fatalf("jitter sleep delay = %v, want 10ms", sleeper.delay)
	}
	if networkCalls.Load() != 0 {
		t.Fatalf("network calls = %d, want 0", networkCalls.Load())
	}
}

func TestClientRunHonorsDeadlineDuringJitterBeforeRequest(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	ctx := newManualDeadlineContext()
	sleeper := &expiringJitterSleeper{ctx: ctx}
	var networkCalls atomic.Int32
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-jitter-deadline"))
	client := Client{
		BaseURL:   "http://receiver.invalid",
		ChunkSize: 4,
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			networkCalls.Add(1)
			return nil, errors.New("network must not be called")
		}),
		jitterSource:  &sequenceJitterSource{values: []int{20}},
		jitterSleeper: sleeper,
	}

	_, err := client.Run(ctx, req)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Client.Run(jitter deadline) error = %v, want context.DeadlineExceeded", err)
	}
	if sleeper.delay != 20*time.Millisecond {
		t.Fatalf("jitter sleep delay = %v, want 20ms", sleeper.delay)
	}
	if networkCalls.Load() != 0 {
		t.Fatalf("network calls = %d, want 0", networkCalls.Load())
	}
}

func TestClientRunReturnsPrivacyOverheadWhenStatusFailsAfterPaddedUpload(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.bin", []byte("payload"))
	statusCalls := 0
	jitterSource := &sequenceJitterSource{values: []int{1, 2, 3, 4, 5}}
	client := Client{
		BaseURL:       "http://receiver.invalid",
		ChunkSize:     4,
		jitterSource:  jitterSource,
		jitterSleeper: &recordingJitterSleeper{},
		Doer: doerFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/sessions":
				return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
					SessionID: "session-status-fails-after-upload",
					State:     protocol.SessionStateValidated,
				}), nil
			case "/v1/sessions/session-status-fails-after-upload/status":
				statusCalls++
				if statusCalls == 1 {
					return jsonHTTPResponse(http.StatusOK, statusFromScan(scanResult, "session-status-fails-after-upload", protocol.SessionStateValidated, func(string) int64 { return 0 })), nil
				}
				return jsonHTTPResponse(http.StatusInternalServerError, protocol.ErrorResponse{
					Code:    protocol.ErrorCodeIO,
					Message: "status unavailable",
				}), nil
			case "/v1/chunks":
				var chunk protocol.ChunkUploadRequest
				if err := decodeChunkRequest(req, &chunk); err != nil {
					return nil, err
				}
				return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
					SessionID:     chunk.SessionID,
					Path:          chunk.Path,
					CommittedSize: chunk.Offset + int64(len(chunk.Data)),
					ChunkState:    protocol.ChunkStateAccepted,
					Complete:      chunk.Final,
				}), nil
			case "/v1/chunk-batches":
				var batch protocol.ChunkBatchUploadRequest
				if err := decodeChunkBatchRequest(req, &batch); err != nil {
					return nil, err
				}
				resp := protocol.ChunkBatchUploadResponse{
					SessionID: batch.SessionID,
					Chunks:    make([]protocol.ChunkUploadResponse, 0, len(batch.Chunks)),
				}
				for _, chunk := range batch.Chunks {
					resp.Chunks = append(resp.Chunks, protocol.ChunkUploadResponse{
						SessionID:     chunk.SessionID,
						Path:          chunk.Path,
						CommittedSize: chunk.Offset + int64(len(chunk.Data)),
						ChunkState:    protocol.ChunkStateAccepted,
						Complete:      chunk.Final,
					})
				}
				return jsonHTTPResponse(http.StatusAccepted, resp), nil
			default:
				return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
			}
		}),
	}

	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-status-fails-after-upload"))
	req.PrivacyPolicy.BatchMaxCount = 1
	got, err := client.Run(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "status unavailable") {
		t.Fatalf("Client.Run(status fails after upload) error = %v, want status unavailable", err)
	}
	if got.Privacy.PaddedChunks == 0 || got.Privacy.PaddingBytes == 0 {
		t.Fatalf("Client.Run(status fails after upload).Privacy = %+v, want retained padding overhead", got.Privacy)
	}
	if got.Privacy.JitteredRequests != 5 || got.Privacy.JitterDelayMillis != 15 || got.Privacy.MaxJitterDelayMillis != 5 || got.Privacy.JitterBudgetMillis != 250 {
		t.Fatalf("Client.Run(status fails after upload).Privacy = %+v, want retained jitter overhead", got.Privacy)
	}
	var transferErr *TransferError
	if !errors.As(err, &transferErr) || transferErr.Privacy.PaddedChunks == 0 || transferErr.Privacy.JitteredRequests != 5 {
		t.Fatalf("Client.Run(status fails after upload) error = %T %+v, want TransferError with padding and jitter overhead", err, err)
	}
}

func TestClientRunUploadsZeroByteRegularFileThroughChunkPath(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "empty.txt", nil)
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()

	got, err := (Client{BaseURL: server.URL, ChunkSize: 16}).Run(context.Background(), validTransferRequest(source, scanResult, "session-empty"))
	if err != nil {
		t.Fatalf("Client.Run(zero byte file) error = %v, want nil", err)
	}
	if got.Files != 1 || got.Bytes != 0 || got.Chunks != 1 {
		t.Fatalf("Client.Run(zero byte file) files/bytes/chunks = %d/%d/%d, want 1/0/1", got.Files, got.Bytes, got.Chunks)
	}
	info, err := os.Stat(filepath.Join(target, "empty.txt"))
	if err != nil {
		t.Fatalf("os.Stat(published empty file) error = %v, want nil", err)
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("published empty file mode=%v size=%d, want regular zero-byte file", info.Mode(), info.Size())
	}

	recorder.mu.Lock()
	begin := recorder.begin
	chunkSizes := append([]int(nil), recorder.chunkSizes...)
	chunkOffsets := append([]int64(nil), recorder.chunkOffsets...)
	chunkPaths := append([]string(nil), recorder.chunkPaths...)
	chunkDigests := append([]string(nil), recorder.chunkDigests...)
	chunkFinals := append([]bool(nil), recorder.chunkFinals...)
	batchCounts := append([]int(nil), recorder.batchCounts...)
	commits := recorder.commits
	recorder.mu.Unlock()
	entry := manifestEntryByPath(begin.Manifest, "empty.txt")
	if entry.Kind != protocol.FileKindFile || entry.Size != 0 || entry.Digest != protocol.EmptySHA256Digest {
		t.Fatalf("manifest entry = %+v, want zero-byte file with empty digest", entry)
	}
	if len(batchCounts) != 0 {
		t.Fatalf("batch requests = %+v, want direct chunk upload for level 1", batchCounts)
	}
	if len(chunkSizes) != 1 || chunkSizes[0] != 0 || chunkOffsets[0] != 0 || chunkPaths[0] != "empty.txt" || chunkDigests[0] != protocol.EmptySHA256Digest || !chunkFinals[0] {
		t.Fatalf("recorded zero-byte chunk sizes=%+v offsets=%+v paths=%+v digests=%+v finals=%+v, want one final empty chunk", chunkSizes, chunkOffsets, chunkPaths, chunkDigests, chunkFinals)
	}
	if commits != 1 {
		t.Fatalf("commit calls = %d, want 1", commits)
	}
}

func TestClientRunUploadsZeroByteRegularFileWithLevel2PrivacyOverhead(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "empty.txt", nil)
	target := t.TempDir()
	recorder := &recordingHandler{next: receiver.NewHandler(receiver.FileStore{TargetRoot: target})}
	server := httptest.NewServer(recorder)
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-empty-private"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 8

	got, err := newZeroJitterClient(server.URL, 16, server.Client()).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Client.Run(level 2 zero byte file) error = %v, want nil", err)
	}
	if got.Files != 1 || got.Bytes != 0 || got.Chunks != 1 {
		t.Fatalf("Client.Run(level 2 zero byte file) files/bytes/chunks = %d/%d/%d, want 1/0/1", got.Files, got.Bytes, got.Chunks)
	}
	if got.Privacy.BatchFrames != 1 || got.Privacy.BatchedChunks != 1 || got.Privacy.PaddedChunks != 1 || got.Privacy.PaddingBucketBytes != 64 || got.Privacy.PaddingBytes <= 0 {
		t.Fatalf("Client.Run(level 2 zero byte file).Privacy = %+v, want one padded batch record", got.Privacy)
	}
	if got.Privacy.FrameWireBytes <= got.Privacy.FramePlainBytes {
		t.Fatalf("Client.Run(level 2 zero byte file).Privacy = %+v, want wire bytes greater than plain frame bytes", got.Privacy)
	}

	recorder.mu.Lock()
	batchCounts := append([]int(nil), recorder.batchCounts...)
	batchEncodings := append([]string(nil), recorder.batchFrameEncodings...)
	batchWireBytes := append([]int(nil), recorder.batchFrameWireBytes...)
	chunkSizes := append([]int(nil), recorder.chunkSizes...)
	chunkOffsets := append([]int64(nil), recorder.chunkOffsets...)
	chunkDigests := append([]string(nil), recorder.chunkDigests...)
	chunkFinals := append([]bool(nil), recorder.chunkFinals...)
	frameEncodings := append([]string(nil), recorder.frameEncodings...)
	recorder.mu.Unlock()
	if len(batchCounts) != 1 || batchCounts[0] != 1 {
		t.Fatalf("batch counts = %+v, want one single-record batch", batchCounts)
	}
	if len(batchEncodings) != 1 || batchEncodings[0] != protocol.FrameEncodingPaddingV1 || batchWireBytes[0]%64 != 0 {
		t.Fatalf("batch encodings/wire bytes = %+v/%+v, want padded batch frame multiple of 64", batchEncodings, batchWireBytes)
	}
	if len(frameEncodings) != 0 {
		t.Fatalf("single chunk frame encodings = %+v, want no direct chunk request under batching", frameEncodings)
	}
	if len(chunkSizes) != 1 || chunkSizes[0] != 0 || chunkOffsets[0] != 0 || chunkDigests[0] != protocol.EmptySHA256Digest || !chunkFinals[0] {
		t.Fatalf("batched zero-byte chunk sizes=%+v offsets=%+v digests=%+v finals=%+v, want one final empty chunk", chunkSizes, chunkOffsets, chunkDigests, chunkFinals)
	}
	if info, err := os.Stat(filepath.Join(target, "empty.txt")); err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("published empty file info=%v err=%v, want regular zero-byte file", info, err)
	}
}

func TestClientRunReturnsZeroByteBatchUploadFailureBeforeCommit(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "empty.txt", nil)
	target := t.TempDir()
	next := receiver.NewHandler(receiver.FileStore{TargetRoot: target})
	var commits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chunk-batches":
			http.Error(w, "batch upload unavailable", http.StatusServiceUnavailable)
		case "/v1/commit":
			commits++
			next.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	req := withLevel2Privacy(validTransferRequest(source, scanResult, "session-empty-batch-failure"))
	req.PrivacyPolicy.PaddingBucket = 64
	req.PrivacyPolicy.BatchMaxBytes = 4096
	req.PrivacyPolicy.BatchMaxCount = 8

	_, err := newZeroJitterClient(server.URL, 16, server.Client()).Run(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "/v1/chunk-batches") {
		t.Fatalf("Client.Run(level 2 zero byte batch failure) error = %v, want /v1/chunk-batches failure", err)
	}
	if commits != 0 {
		t.Fatalf("commit calls after rejected zero-byte batch = %d, want 0", commits)
	}
}

func TestBuildBeginRequestSkipsSpecialAndReservedEntriesWithWarnings(t *testing.T) {
	source := t.TempDir()
	result := scan.Result{
		Root: filepath.ToSlash(source),
		Entries: []scan.Entry{
			{Path: ".", Kind: scan.KindDir},
			{Path: ".supermover", Kind: scan.KindDir, Mode: 0o755},
			{Path: ".supermover/session.json", Kind: scan.KindRegular, Size: 12, Digest: digestBytes([]byte("ignored"))},
			{Path: "pipe", Kind: scan.KindSpecial, Mode: os.ModeNamedPipe},
		},
	}

	begin, warnings, err := BuildBeginRequest(validTransferRequest(source, result, "session-warnings"))
	if err != nil {
		t.Fatalf("BuildBeginRequest(special/reserved) error = %v, want nil", err)
	}
	if len(begin.Manifest.Entries) != 0 {
		t.Fatalf("manifest entries = %+v, want none", begin.Manifest.Entries)
	}
	if !hasWarning(warnings, "reserved_control_plane_skipped") {
		t.Fatalf("warnings = %+v, want reserved_control_plane_skipped", warnings)
	}
	if !hasWarning(warnings, "special_not_uploaded") {
		t.Fatalf("warnings = %+v, want special_not_uploaded", warnings)
	}
}

type recordingHandler struct {
	next                   http.Handler
	mu                     sync.Mutex
	begin                  protocol.BeginSessionRequest
	statusPaths            []string
	batchSizes             []int
	batchCounts            []int
	batchFrameEncodings    []string
	batchFrameWireBytes    []int
	batchFramePaddingBytes []int
	chunkSizes             []int
	chunkOffsets           []int64
	chunkPaths             []string
	chunkDigests           []string
	chunkFinals            []bool
	chunkData              [][]byte
	frameEncodings         []string
	frameWireBytes         []int
	framePaddingBytes      []int
	commits                int
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/sessions" {
		body := readAndRestoreBody(r)
		var req protocol.BeginSessionRequest
		_ = json.Unmarshal(body, &req)
		h.mu.Lock()
		h.begin = req
		h.mu.Unlock()
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/status") {
		h.mu.Lock()
		h.statusPaths = append(h.statusPaths, r.URL.Path)
		h.mu.Unlock()
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/chunks" {
		body := readAndRestoreBody(r)
		var req protocol.ChunkUploadRequest
		stats, err := h.decodeChunkRequestBytes(r, body, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.mu.Lock()
		h.chunkSizes = append(h.chunkSizes, len(req.Data))
		h.chunkOffsets = append(h.chunkOffsets, req.Offset)
		h.chunkPaths = append(h.chunkPaths, req.Path)
		h.chunkDigests = append(h.chunkDigests, req.Digest)
		h.chunkFinals = append(h.chunkFinals, req.Final)
		h.chunkData = append(h.chunkData, append([]byte(nil), req.Data...))
		h.frameEncodings = append(h.frameEncodings, r.Header.Get(protocol.FrameEncodingHeader))
		h.frameWireBytes = append(h.frameWireBytes, len(body))
		h.framePaddingBytes = append(h.framePaddingBytes, stats.PaddingBytes)
		h.mu.Unlock()
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/chunk-batches" {
		body := readAndRestoreBody(r)
		var req protocol.ChunkBatchUploadRequest
		stats, err := h.decodeChunkBatchRequestBytes(r, body, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.mu.Lock()
		h.batchSizes = append(h.batchSizes, len(body))
		h.batchCounts = append(h.batchCounts, len(req.Chunks))
		h.batchFrameEncodings = append(h.batchFrameEncodings, r.Header.Get(protocol.FrameEncodingHeader))
		h.batchFrameWireBytes = append(h.batchFrameWireBytes, len(body))
		h.batchFramePaddingBytes = append(h.batchFramePaddingBytes, stats.PaddingBytes)
		for _, chunk := range req.Chunks {
			h.chunkSizes = append(h.chunkSizes, len(chunk.Data))
			h.chunkOffsets = append(h.chunkOffsets, chunk.Offset)
			h.chunkPaths = append(h.chunkPaths, chunk.Path)
			h.chunkDigests = append(h.chunkDigests, chunk.Digest)
			h.chunkFinals = append(h.chunkFinals, chunk.Final)
			h.chunkData = append(h.chunkData, append([]byte(nil), chunk.Data...))
		}
		h.mu.Unlock()
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/commit" {
		h.mu.Lock()
		h.commits++
		h.mu.Unlock()
	}
	h.next.ServeHTTP(w, r)
}

func (h *recordingHandler) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statusPaths = nil
	h.batchSizes = nil
	h.batchCounts = nil
	h.batchFrameEncodings = nil
	h.batchFrameWireBytes = nil
	h.batchFramePaddingBytes = nil
	h.chunkSizes = nil
	h.chunkOffsets = nil
	h.chunkPaths = nil
	h.chunkDigests = nil
	h.chunkFinals = nil
	h.chunkData = nil
	h.frameEncodings = nil
	h.frameWireBytes = nil
	h.framePaddingBytes = nil
	h.commits = 0
}

func readAndRestoreBody(r *http.Request) []byte {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return body
}

func decodeChunkRequest(req *http.Request, chunk *protocol.ChunkUploadRequest) error {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	_ = req.Body.Close()
	_, err = decodeChunkRequestBytes(req, body, chunk)
	return err
}

func decodeChunkBatchRequest(req *http.Request, batch *protocol.ChunkBatchUploadRequest) error {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	_ = req.Body.Close()
	_, err = decodeChunkBatchRequestBytesWithBucket(req, body, batch, transport.DefaultPrivacyPolicy(transport.PrivacyLevel2).PaddingBucket)
	return err
}

func (h *recordingHandler) decodeChunkRequestBytes(req *http.Request, body []byte, chunk *protocol.ChunkUploadRequest) (padding.Stats, error) {
	return decodeChunkRequestBytesWithBucket(req, body, chunk, h.paddingBucket())
}

func (h *recordingHandler) decodeChunkBatchRequestBytes(req *http.Request, body []byte, batch *protocol.ChunkBatchUploadRequest) (padding.Stats, error) {
	return decodeChunkBatchRequestBytesWithBucket(req, body, batch, h.paddingBucket())
}

func (h *recordingHandler) paddingBucket() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.begin.PrivacyPolicy.PaddingBucket != 0 {
		return h.begin.PrivacyPolicy.PaddingBucket
	}
	return transport.DefaultPrivacyPolicy(transport.PrivacyLevel2).PaddingBucket
}

func decodeChunkRequestBytes(req *http.Request, body []byte, chunk *protocol.ChunkUploadRequest) (padding.Stats, error) {
	return decodeChunkRequestBytesWithBucket(req, body, chunk, transport.DefaultPrivacyPolicy(transport.PrivacyLevel2).PaddingBucket)
}

func decodeChunkRequestBytesWithBucket(req *http.Request, body []byte, chunk *protocol.ChunkUploadRequest, bucket int) (padding.Stats, error) {
	stats := padding.Stats{}
	encoding := req.Header.Get(protocol.FrameEncodingHeader)
	if encoding == protocol.FrameEncodingPaddingV1 {
		plain, frameStats, err := padding.Unpad(body, padding.Config{
			BucketBytes:   bucket,
			MaxFrameBytes: protocol.MaxPaddedBatchRequestBodyBytes,
		})
		if err != nil {
			return padding.Stats{}, err
		}
		body = plain
		stats = frameStats
	}
	return stats, json.Unmarshal(body, chunk)
}

func decodeChunkBatchRequestBytesWithBucket(req *http.Request, body []byte, batch *protocol.ChunkBatchUploadRequest, bucket int) (padding.Stats, error) {
	stats := padding.Stats{}
	encoding := req.Header.Get(protocol.FrameEncodingHeader)
	if encoding == protocol.FrameEncodingPaddingV1 {
		plain, frameStats, err := padding.Unpad(body, padding.Config{
			BucketBytes:   bucket,
			MaxFrameBytes: protocol.MaxPaddedChunkRequestBodyBytes,
		})
		if err != nil {
			return padding.Stats{}, err
		}
		body = plain
		stats = frameStats
	}
	return stats, json.Unmarshal(body, batch)
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newZeroJitterClient(baseURL string, chunkSize int, doer Doer) Client {
	return Client{
		BaseURL:       baseURL,
		ChunkSize:     chunkSize,
		Doer:          doer,
		jitterSource:  zeroJitterSource{},
		jitterSleeper: &recordingJitterSleeper{},
	}
}

type zeroJitterSource struct{}

func (zeroJitterSource) DelayMillis(int) (int, error) {
	return 0, nil
}

type sequenceJitterSource struct {
	values []int
	calls  int
}

func (s *sequenceJitterSource) DelayMillis(int) (int, error) {
	if s.calls >= len(s.values) {
		return 0, errors.New("jitter sequence exhausted")
	}
	value := s.values[s.calls]
	s.calls++
	return value, nil
}

type recordingJitterSleeper struct {
	delays []time.Duration
}

func (s *recordingJitterSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.delays = append(s.delays, delay)
	return nil
}

type cancelingJitterSleeper struct {
	cancel func()
	delay  time.Duration
}

func (s *cancelingJitterSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	s.delay = delay
	if s.cancel != nil {
		s.cancel()
	}
	<-ctx.Done()
	return ctx.Err()
}

type expiringJitterSleeper struct {
	ctx   *manualDeadlineContext
	delay time.Duration
}

func (s *expiringJitterSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	s.delay = delay
	if s.ctx != nil {
		s.ctx.expire()
	}
	<-ctx.Done()
	return ctx.Err()
}

type manualDeadlineContext struct {
	done chan struct{}
}

func newManualDeadlineContext() *manualDeadlineContext {
	return &manualDeadlineContext{done: make(chan struct{})}
}

func (c *manualDeadlineContext) Deadline() (time.Time, bool) {
	return time.Time{}, true
}

func (c *manualDeadlineContext) Done() <-chan struct{} {
	return c.done
}

func (c *manualDeadlineContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (c *manualDeadlineContext) Value(any) any {
	return nil
}

func (c *manualDeadlineContext) expire() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

func equalDurations(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type failAfterAcceptedChunkDoer struct {
	base      Doer
	failAfter int
	chunks    int
}

func (d *failAfterAcceptedChunkDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.base.Do(req)
	if err != nil {
		return resp, err
	}
	if req.Method == http.MethodPost && (req.URL.Path == "/v1/chunks" || req.URL.Path == "/v1/chunk-batches") {
		d.chunks++
		if d.chunks >= d.failAfter {
			if resp != nil && resp.Body != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			return nil, errors.New("simulated interruption after accepted chunk")
		}
	}
	return resp, nil
}

type statusTracker struct {
	mu      sync.Mutex
	offsets map[string]int64
}

func newStatusTracker() *statusTracker {
	return &statusTracker{offsets: make(map[string]int64)}
}

func (s *statusTracker) set(path string, offset int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offsets[path] = offset
}

func (s *statusTracker) committed(path string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.offsets[path]
}

func withReceiverStatus(result scan.Result, sessionID string, state func() protocol.SessionState, committed func(string) int64, next Doer) Doer {
	if state == nil {
		state = func() protocol.SessionState { return protocol.SessionStateValidated }
	}
	if committed == nil {
		committed = func(string) int64 { return 0 }
	}
	statusPath := "/v1/sessions/" + sessionID + "/status"
	return doerFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == statusPath {
			return jsonHTTPResponse(http.StatusOK, statusFromScan(result, sessionID, state(), committed)), nil
		}
		return next.Do(req)
	})
}

func statusFromScan(result scan.Result, sessionID string, state protocol.SessionState, committed func(string) int64) protocol.SessionStatusResponse {
	files := make([]protocol.FileStatus, 0)
	for _, entry := range result.Entries {
		if entry.Kind != scan.KindRegular {
			continue
		}
		offset := committed(entry.Path)
		files = append(files, protocol.FileStatus{
			Path:           entry.Path,
			ExpectedSize:   entry.Size,
			CommittedSize:  offset,
			ExpectedDigest: entry.Digest,
			Complete:       offset == entry.Size,
		})
	}
	return protocol.SessionStatusResponse{
		SessionID: sessionID,
		State:     state,
		Files:     files,
	}
}

func zeroByteStatusFromScan(result scan.Result, sessionID string, complete bool) protocol.SessionStatusResponse {
	status := statusFromScan(result, sessionID, protocol.SessionStateValidated, func(string) int64 { return 0 })
	for i := range status.Files {
		if status.Files[i].ExpectedSize == 0 {
			status.Files[i].CommittedSize = 0
			status.Files[i].Complete = complete
			status.Files[i].ExpectedDigest = protocol.EmptySHA256Digest
		}
	}
	return status
}

func jsonHTTPResponse(status int, value any) *http.Response {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(value)
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&buf),
	}
}

func validTransferRequest(source string, result scan.Result, sessionID string) TransferRequest {
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	return TransferRequest{
		SourceRoot:     source,
		Scan:           result,
		SessionID:      sessionID,
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
		PrivacyPolicy:  transport.DefaultPrivacyPolicy(transport.PrivacyLevel1),
		RootID:         "root1",
		CreatedAt:      now,
		EndedAt:        now.Add(time.Minute),
	}
}

func withLevel2Privacy(req TransferRequest) TransferRequest {
	req.PrivacyPolicy = transport.DefaultPrivacyPolicy(transport.PrivacyLevel2)
	return req
}

func scanSingleFileSource(t *testing.T, name string, data []byte) (string, scan.Result) {
	t.Helper()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, name), data, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", name, err)
	}
	result, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	return source, result
}

func scanEntry(t *testing.T, result scan.Result, path string) scan.Entry {
	t.Helper()
	for _, entry := range result.Entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("scan entry %q missing from %+v", path, result.Entries)
	return scan.Entry{}
}

func writePatternFile(path string, size int) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	pattern := []byte("supermover-streaming-test-pattern\n")
	written := 0
	for written < size {
		n := min(len(pattern), size-written)
		if _, err := file.Write(pattern[:n]); err != nil {
			return err
		}
		written += n
	}
	return nil
}

func hashFile(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v, want nil", path, err)
	}
	defer file.Close()
	return digestFromReader(t, file)
}

func digestFromReader(t *testing.T, reader io.Reader) string {
	t.Helper()
	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		t.Fatalf("io.Copy(hash) error = %v, want nil", err)
	}
	return digestFromHash(h)
}

func manifestHas(manifest protocol.TransferManifest, path string, kind protocol.FileKind) bool {
	for _, entry := range manifest.Entries {
		if entry.Path == path && entry.Kind == kind {
			return true
		}
	}
	return false
}

func manifestEntryByPath(manifest protocol.TransferManifest, path string) protocol.ManifestEntry {
	for _, entry := range manifest.Entries {
		if entry.Path == path {
			return entry
		}
	}
	return protocol.ManifestEntry{}
}

func hasWarning(records []audit.Record, kind string) bool {
	for _, record := range records {
		if record.Kind == kind {
			return true
		}
	}
	return false
}
