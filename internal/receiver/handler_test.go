package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/privacy/padding"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

func TestHandlerBeginChunkStatusCommit(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewHandler(store)
	beginReq := validBeginRequest([]byte("abc"))

	beginRec := doJSON(t, handler, http.MethodPost, "/v1/sessions", beginReq)
	if beginRec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/sessions status = %d, want %d body %s", beginRec.Code, http.StatusAccepted, beginRec.Body.String())
	}

	chunk := protocol.ChunkUploadRequest{SessionID: beginReq.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("abc"), Final: true}
	chunkRec := doJSON(t, handler, http.MethodPost, "/v1/chunks", chunk)
	if chunkRec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/chunks status = %d, want %d body %s", chunkRec.Code, http.StatusAccepted, chunkRec.Body.String())
	}

	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/status", nil))
	if statusRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/sessions/session-1/status status = %d, want %d body %s", statusRec.Code, http.StatusOK, statusRec.Body.String())
	}

	commit := protocol.CommitSessionRequest{SessionID: beginReq.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	commitRec := doJSON(t, handler, http.MethodPost, "/v1/commit", commit)
	if commitRec.Code != http.StatusOK {
		t.Fatalf("POST /v1/commit status = %d, want %d body %s", commitRec.Code, http.StatusOK, commitRec.Body.String())
	}
}

func TestHandlerAcceptsExplicitZeroByteCompletion(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewHandler(store)
	beginReq := validBeginRequest([]byte{})
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions", beginReq); rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/sessions status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	chunk := protocol.ChunkUploadRequest{SessionID: beginReq.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte{}, Digest: protocol.EmptySHA256Digest, Final: true}

	rec := doJSON(t, handler, http.MethodPost, "/v1/chunks", chunk)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/chunks zero-byte completion status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var resp protocol.ChunkUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(zero-byte chunk response) error = %v, want nil body %s", err, rec.Body.String())
	}
	if !resp.Complete || resp.CommittedSize != 0 {
		t.Fatalf("zero-byte chunk response = %+v, want complete size 0", resp)
	}
}

func TestHandlerRejectsPaddedChunkWhenSessionRequiresBatching(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	chunk := protocol.ChunkUploadRequest{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("abc"), Digest: "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", Final: true}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		chunk.SessionID: testPrivacyPolicy(64),
	}

	rec := doPaddedChunk(t, handler, chunk, 64)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunks padded level2 status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.seenBytes != 0 {
		t.Fatalf("AppendChunk(padded level2) saw %d data bytes, want store not called", store.seenBytes)
	}
}

func TestHandlerRejectsUnframedChunkWhenSessionRequiresPadding(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	chunk := protocol.ChunkUploadRequest{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("abc"), Digest: "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", Final: true}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		chunk.SessionID: testPrivacyPolicy(64),
	}

	rec := doJSON(t, handler, http.MethodPost, "/v1/chunks", chunk)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunks unframed level2 status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.seenBytes != 0 {
		t.Fatalf("AppendChunk(unframed level2) saw %d data bytes, want store not called", store.seenBytes)
	}
}

func TestHandlerRejectsMalformedPaddedChunkBeforeStore(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	chunk := protocol.ChunkUploadRequest{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("abc"), Final: true}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		chunk.SessionID: testPrivacyPolicy(64),
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(chunk); err != nil {
		t.Fatalf("json.Encode(chunk) error = %v, want nil", err)
	}
	wire, _, err := padding.Pad(body.Bytes(), padding.Config{BucketBytes: 64, MaxFrameBytes: protocol.MaxPaddedChunkRequestBodyBytes})
	if err != nil {
		t.Fatalf("padding.Pad(chunk) error = %v, want nil", err)
	}
	wire[len(wire)-1] = 1
	req := httptest.NewRequest(http.MethodPost, "/v1/chunks", bytes.NewReader(wire))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(protocol.FrameEncodingHeader, protocol.FrameEncodingPaddingV1)
	req.Header.Set(protocol.FrameSessionIDHeader, chunk.SessionID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunks malformed padded status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.seenBytes != 0 {
		t.Fatalf("AppendChunk(malformed padded) saw %d data bytes, want store not called", store.seenBytes)
	}
}

func TestHandlerRejectsPaddedChunkWhenSessionPolicyBucketDiffers(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	chunk := protocol.ChunkUploadRequest{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("abc"), Final: true}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		chunk.SessionID: testPrivacyPolicy(256),
	}

	rec := doPaddedChunk(t, handler, chunk, 64)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunks wrong policy bucket status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.seenBytes != 0 {
		t.Fatalf("AppendChunk(wrong policy bucket) saw %d data bytes, want store not called", store.seenBytes)
	}
}

func TestHandlerRejectsPaddedChunkWhenFrameSessionDiffersFromBody(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	chunk := protocol.ChunkUploadRequest{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("abc"), Final: true}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		"session-other": testPrivacyPolicy(64),
	}

	rec := doPaddedChunkWithFrameSession(t, handler, chunk, "session-other", 64)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunks mismatched frame session status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.seenBytes != 0 {
		t.Fatalf("AppendChunk(mismatched frame session) saw %d data bytes, want store not called", store.seenBytes)
	}
}

func TestHandlerAcceptsChunkBatchInOrder(t *testing.T) {
	store := &chunkCaptureStore{
		privacyPolicies: map[string]transport.PrivacyPolicy{"session-1": {}},
	}
	handler := NewHandler(store)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: "session-1",
		Chunks: []protocol.ChunkUploadRequest{
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 2, Data: []byte("cd"), Final: true},
		},
	}

	rec := doJSON(t, handler, http.MethodPost, "/v1/chunk-batches", batch)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/chunk-batches status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var resp protocol.ChunkBatchUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(batch response) error = %v, want nil body %s", err, rec.Body.String())
	}
	if resp.SessionID != batch.SessionID {
		t.Fatalf("batch response session_id = %q, want %q", resp.SessionID, batch.SessionID)
	}
	if len(resp.Chunks) != len(batch.Chunks) {
		t.Fatalf("batch response chunks = %d, want %d", len(resp.Chunks), len(batch.Chunks))
	}
	for i, chunk := range batch.Chunks {
		if resp.Chunks[i].Path != chunk.Path {
			t.Fatalf("batch response chunks[%d].path = %q, want %q", i, resp.Chunks[i].Path, chunk.Path)
		}
	}
	if len(store.seenChunks) != len(batch.Chunks) {
		t.Fatalf("AppendChunk calls = %d, want %d", len(store.seenChunks), len(batch.Chunks))
	}
	for i, chunk := range batch.Chunks {
		if string(store.seenChunks[i].Data) != string(chunk.Data) {
			t.Fatalf("AppendChunk call %d data = %q, want %q", i, store.seenChunks[i].Data, chunk.Data)
		}
	}
}

func TestHandlerStopsChunkBatchOnFirstStoreError(t *testing.T) {
	store := &chunkCaptureStore{
		privacyPolicies: map[string]transport.PrivacyPolicy{"session-1": {}},
		errAfter:        1,
		appendErr:       ErrConflict,
	}
	handler := NewHandler(store)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: "session-1",
		Chunks: []protocol.ChunkUploadRequest{
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 2, Data: []byte("cd")},
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 4, Data: []byte("ef")},
		},
	}

	rec := doJSON(t, handler, http.MethodPost, "/v1/chunk-batches", batch)

	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /v1/chunk-batches store error status = %d, want %d body %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if len(store.seenChunks) != 2 {
		t.Fatalf("AppendChunk calls after first error = %d, want 2", len(store.seenChunks))
	}
}

func TestHandlerStopsChunkBatchWhenRequestContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &chunkCaptureStore{
		privacyPolicies: map[string]transport.PrivacyPolicy{"session-1": {}},
		cancelAfter:     1,
		cancel:          cancel,
	}
	handler := NewHandler(store)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: "session-1",
		Chunks: []protocol.ChunkUploadRequest{
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 2, Data: []byte("cd"), Final: true},
		},
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(batch); err != nil {
		t.Fatalf("json.Encode(batch) error = %v, want nil", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk-batches", &body).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST /v1/chunk-batches canceled status = %d, want %d body %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if len(store.seenChunks) != 1 {
		t.Fatalf("AppendChunk calls after cancellation = %d, want 1", len(store.seenChunks))
	}
}

func TestHandlerRejectsUnframedChunkBatchWhenSessionRequiresPadding(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: "session-1",
		Chunks: []protocol.ChunkUploadRequest{
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 2, Data: []byte("cd"), Final: true},
		},
	}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		batch.SessionID: testPrivacyPolicy(64),
	}

	rec := doJSON(t, handler, http.MethodPost, "/v1/chunk-batches", batch)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunk-batches unframed level2 status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if len(store.seenChunks) != 0 {
		t.Fatalf("AppendChunk(unframed level2 batch) calls = %d, want 0", len(store.seenChunks))
	}
}

func TestHandlerAcceptsPaddedChunkBatchWhenSessionRequiresPadding(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: "session-1",
		Chunks: []protocol.ChunkUploadRequest{
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 2, Data: []byte("cd"), Final: true},
		},
	}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		batch.SessionID: testPrivacyPolicy(64),
	}

	rec := doPaddedChunkBatch(t, handler, batch, 64)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/chunk-batches padded status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if len(store.seenChunks) != len(batch.Chunks) {
		t.Fatalf("AppendChunk(padded batch) calls = %d, want %d", len(store.seenChunks), len(batch.Chunks))
	}
	if store.seenBytes != len(batch.Chunks[len(batch.Chunks)-1].Data) {
		t.Fatalf("AppendChunk(padded batch) last data bytes = %d, want plain payload %d", store.seenBytes, len(batch.Chunks[len(batch.Chunks)-1].Data))
	}
}

func TestHandlerRejectsUnframedChunkBatchBeyondPlainProtocolLimit(t *testing.T) {
	handler := NewHandler(FileStore{TargetRoot: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk-batches", io.LimitReader(strings.NewReader(strings.Repeat("x", protocol.MaxBatchPlainBodyBytes+1)), protocol.MaxBatchPlainBodyBytes+1))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunk-batches oversized plain status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlerAcceptsPaddedChunkBatchAbovePlainLimitWithinPolicyWireLimit(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	data := bytes.Repeat([]byte("a"), 8*1024)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: "session-1",
		Chunks: []protocol.ChunkUploadRequest{
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: data, Final: true},
		},
	}
	policy := testPrivacyPolicy(64)
	policy.BatchMaxBytes = protocol.MaxBatchPlainBodyBytes
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		batch.SessionID: policy,
	}

	rec := doPaddedChunkBatch(t, handler, batch, 64)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/chunk-batches padded above plain limit status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if len(store.seenChunks) != 1 || len(store.seenChunks[0].Data) != len(data) {
		t.Fatalf("AppendChunkBatch calls = %d data = %d, want one %d-byte chunk", len(store.seenChunks), store.seenBytes, len(data))
	}
}

func TestHandlerRejectsChunkBatchBeyondSessionPolicyBounds(t *testing.T) {
	tests := []struct {
		name   string
		policy func() transport.PrivacyPolicy
		batch  protocol.ChunkBatchUploadRequest
	}{
		{
			name: "count",
			policy: func() transport.PrivacyPolicy {
				policy := testPrivacyPolicy(64)
				policy.BatchMaxCount = 1
				return policy
			},
			batch: protocol.ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks: []protocol.ChunkUploadRequest{
					{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
					{SessionID: "session-1", Path: "docs/a.txt", Offset: 2, Data: []byte("cd"), Final: true},
				},
			},
		},
		{
			name: "plain_body_bytes",
			policy: func() transport.PrivacyPolicy {
				policy := testPrivacyPolicy(64)
				policy.BatchMaxBytes = 32
				return policy
			},
			batch: protocol.ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks: []protocol.ChunkUploadRequest{
					{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &chunkCaptureStore{}
			handler := NewHandler(store)
			store.privacyPolicies = map[string]transport.PrivacyPolicy{
				tt.batch.SessionID: tt.policy(),
			}

			rec := doPaddedChunkBatch(t, handler, tt.batch, 64)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST /v1/chunk-batches beyond %s bound status = %d, want %d body %s", tt.name, rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if len(store.seenChunks) != 0 {
				t.Fatalf("AppendChunk(beyond %s bound) calls = %d, want 0", tt.name, len(store.seenChunks))
			}
		})
	}
}

func TestHandlerRejectsPaddedChunkBatchWhenFrameSessionDiffersFromBody(t *testing.T) {
	store := &chunkCaptureStore{}
	handler := NewHandler(store)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: "session-1",
		Chunks: []protocol.ChunkUploadRequest{
			{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("ab")},
		},
	}
	store.privacyPolicies = map[string]transport.PrivacyPolicy{
		"session-other": testPrivacyPolicy(64),
	}

	rec := doPaddedChunkBatchWithFrameSession(t, handler, batch, "session-other", 64)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunk-batches mismatched frame session status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if len(store.seenChunks) != 0 {
		t.Fatalf("AppendChunk(mismatched frame session batch) calls = %d, want 0", len(store.seenChunks))
	}
}

func TestHandlerWritesSessionArtifacts(t *testing.T) {
	target := t.TempDir()
	store := FileStore{TargetRoot: target}
	handler := NewHandler(store)
	beginReq := validBeginRequest([]byte("abc"))
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions", beginReq); rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/sessions status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + beginReq.SessionID,
		ProfileID:  beginReq.ProfileID,
		SessionID:  beginReq.SessionID,
		CapturedAt: "2026-05-16T08:00:00Z",
		Profile:    []byte(`{"version":1}`),
	}
	snapshotPayload := marshalControlDoc(t, snapshot)
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/profile", protocol.ProfileSnapshotArtifactRequest{
		SessionID: beginReq.SessionID,
		Document:  snapshotPayload,
	}); rec.Code != http.StatusOK {
		t.Fatalf("POST profile artifact status = %d, want %d body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := readHandlerControlDoc[control.ProfileSnapshot](t, target, control.ArtifactProfileSnapshot, snapshot.ID); got.ID != snapshot.ID {
		t.Fatalf("profile snapshot = %+v, want %q", got, snapshot.ID)
	}

	warning := control.Warning{
		Version:   control.CurrentVersion,
		ID:        beginReq.SessionID + "-001-warning",
		SessionID: beginReq.SessionID,
		Code:      "reserved_control_plane_skipped",
		Message:   "source .supermover was skipped",
		Severity:  "warning",
		Paths:     []string{".supermover"},
		CreatedAt: "2026-05-16T08:00:00Z",
	}
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/warnings", protocol.WarningArtifactRequest{
		SessionID: beginReq.SessionID,
		Documents: [][]byte{marshalControlDoc(t, warning)},
	}); rec.Code != http.StatusOK {
		t.Fatalf("POST warning artifacts status = %d, want %d body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := readHandlerControlDoc[control.Warning](t, target, control.ArtifactWarning, warning.ID); got.Code != warning.Code {
		t.Fatalf("warning = %+v, want code %q", got, warning.Code)
	}

	transfer := control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       beginReq.SessionID,
		ProfileID:       beginReq.ProfileID,
		TargetID:        beginReq.TargetID,
		SourceDeviceID:  beginReq.SourceDeviceID,
		TargetDeviceID:  beginReq.TargetDeviceID,
		ProtocolVersion: protocol.Version,
		PrivacyPolicy:   beginReq.PrivacyPolicy,
		Status:          control.NetworkTransferStarted,
		Stage:           "begin",
		StartedAt:       "2026-05-16T08:00:00Z",
		UpdatedAt:       "2026-05-16T08:00:00Z",
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T08:00:00Z",
			Stage:     "begin",
			Status:    control.NetworkTransferStarted,
		}},
	}
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/network-transfer", protocol.NetworkTransferArtifactRequest{
		SessionID: beginReq.SessionID,
		Document:  marshalControlDoc(t, transfer),
	}); rec.Code != http.StatusOK {
		t.Fatalf("POST network-transfer artifact status = %d, want %d body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := readHandlerControlDoc[control.NetworkTransfer](t, target, control.ArtifactNetworkTransfer, beginReq.SessionID); got.Status != control.NetworkTransferStarted {
		t.Fatalf("network transfer = %+v, want started", got)
	}
	readRec := httptest.NewRecorder()
	handler.ServeHTTP(readRec, httptest.NewRequest(http.MethodGet, "/v1/sessions/"+beginReq.SessionID+"/artifacts/network-transfer", nil))
	if readRec.Code != http.StatusOK {
		t.Fatalf("GET network-transfer artifact status = %d, want %d body %s", readRec.Code, http.StatusOK, readRec.Body.String())
	}
	got, err := control.Read[control.NetworkTransfer](strings.NewReader(readRec.Body.String()))
	if err != nil {
		t.Fatalf("control.Read(GET network-transfer) error = %v, want nil", err)
	}
	if got.SessionID != beginReq.SessionID || got.Status != control.NetworkTransferStarted {
		t.Fatalf("GET network-transfer = %+v, want started artifact for %s", got, beginReq.SessionID)
	}
}

func TestHandlerRejectsMismatchedArtifactSession(t *testing.T) {
	target := t.TempDir()
	handler := NewHandler(FileStore{TargetRoot: target})
	beginReq := validBeginRequest([]byte("abc"))
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions", beginReq); rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/sessions status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	profileSnapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + beginReq.SessionID,
		ProfileID:  beginReq.ProfileID,
		SessionID:  "session-other",
		CapturedAt: "2026-05-16T08:00:00Z",
		Profile:    []byte(`{"version":1}`),
	}
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/profile", protocol.ProfileSnapshotArtifactRequest{
		SessionID: beginReq.SessionID,
		Document:  marshalControlDoc(t, profileSnapshot),
	}); rec.Code != http.StatusConflict {
		t.Fatalf("POST mismatched profile artifact status = %d, want %d body %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/profile", protocol.ProfileSnapshotArtifactRequest{
		SessionID: "session-other",
		Document:  []byte(`{}`),
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("POST path/body mismatched profile artifact status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	warningWrongSession := control.Warning{
		Version:   control.CurrentVersion,
		ID:        "session-other-001-warning",
		SessionID: "session-other",
		Code:      "reserved_control_plane_skipped",
		Message:   "source .supermover was skipped",
		Severity:  "warning",
		Paths:     []string{".supermover"},
		CreatedAt: "2026-05-16T08:00:00Z",
	}
	rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/warnings", protocol.WarningArtifactRequest{
		SessionID: beginReq.SessionID,
		Documents: [][]byte{marshalControlDoc(t, warningWrongSession)},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST mismatched warning artifact status = %d, want %d body %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	warningWrongID := warningWrongSession
	warningWrongID.SessionID = beginReq.SessionID
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/warnings", protocol.WarningArtifactRequest{
		SessionID: beginReq.SessionID,
		Documents: [][]byte{marshalControlDoc(t, warningWrongID)},
	}); rec.Code != http.StatusConflict {
		t.Fatalf("POST cross-session warning id status = %d, want %d body %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/warnings", protocol.WarningArtifactRequest{
		SessionID: "session-other",
		Documents: [][]byte{[]byte(`{}`)},
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("POST path/body mismatched warning artifact status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	transfer := control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-other",
		ProfileID:       beginReq.ProfileID,
		TargetID:        beginReq.TargetID,
		SourceDeviceID:  beginReq.SourceDeviceID,
		TargetDeviceID:  beginReq.TargetDeviceID,
		ProtocolVersion: protocol.Version,
		PrivacyPolicy:   beginReq.PrivacyPolicy,
		Status:          control.NetworkTransferStarted,
		Stage:           "begin",
		StartedAt:       "2026-05-16T08:00:00Z",
		UpdatedAt:       "2026-05-16T08:00:00Z",
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T08:00:00Z",
			Stage:     "begin",
			Status:    control.NetworkTransferStarted,
		}},
	}
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/network-transfer", protocol.NetworkTransferArtifactRequest{
		SessionID: beginReq.SessionID,
		Document:  marshalControlDoc(t, transfer),
	}); rec.Code != http.StatusConflict {
		t.Fatalf("POST mismatched network-transfer artifact status = %d, want %d body %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions/"+beginReq.SessionID+"/artifacts/network-transfer", protocol.NetworkTransferArtifactRequest{
		SessionID: "session-other",
		Document:  []byte(`{}`),
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("POST path/body mismatched network-transfer artifact status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlerMapsValidationAndConflictErrors(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewHandler(store)

	badBegin := doJSON(t, handler, http.MethodPost, "/v1/sessions", protocol.BeginSessionRequest{})
	if badBegin.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/sessions invalid status = %d, want %d", badBegin.Code, http.StatusBadRequest)
	}

	beginReq := validBeginRequest([]byte("abc"))
	if rec := doJSON(t, handler, http.MethodPost, "/v1/sessions", beginReq); rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/sessions status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	gap := protocol.ChunkUploadRequest{SessionID: beginReq.SessionID, Path: "docs/a.txt", Offset: 2, Data: []byte("c")}
	conflict := doJSON(t, handler, http.MethodPost, "/v1/chunks", gap)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("POST /v1/chunks gap status = %d, want %d body %s", conflict.Code, http.StatusConflict, conflict.Body.String())
	}
}

func TestHandlerMapsStoreErrors(t *testing.T) {
	tests := []struct {
		name   string
		store  Store
		method string
		path   string
		body   any
		want   int
		code   protocol.ErrorCode
	}{
		{
			name:   "begin validation",
			store:  routeErrorStore{beginErr: transaction.ErrValidation},
			method: http.MethodPost,
			path:   "/v1/sessions",
			body:   validBeginRequest([]byte("abc")),
			want:   http.StatusBadRequest,
			code:   protocol.ErrorCodeBadRequest,
		},
		{
			name:   "status not found",
			store:  routeErrorStore{statusErr: ErrSessionNotFound},
			method: http.MethodGet,
			path:   "/v1/sessions/session-1/status",
			want:   http.StatusNotFound,
			code:   protocol.ErrorCodeNotFound,
		},
		{
			name:   "chunk conflict",
			store:  routeErrorStore{chunkErr: ErrConflict},
			method: http.MethodPost,
			path:   "/v1/chunks",
			body:   protocol.ChunkUploadRequest{SessionID: "session-1", Path: "docs/a.txt", Offset: 0, Data: []byte("abc")},
			want:   http.StatusConflict,
			code:   protocol.ErrorCodeConflict,
		},
		{
			name:   "commit integrity",
			store:  routeErrorStore{commitErr: ErrIntegrity},
			method: http.MethodPost,
			path:   "/v1/commit",
			body:   protocol.CommitSessionRequest{SessionID: "session-1", EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)},
			want:   http.StatusUnprocessableEntity,
			code:   protocol.ErrorCodeIntegrity,
		},
		{
			name:   "commit io",
			store:  routeErrorStore{commitErr: errors.New("disk refused write")},
			method: http.MethodPost,
			path:   "/v1/commit",
			body:   protocol.CommitSessionRequest{SessionID: "session-1", EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)},
			want:   http.StatusInternalServerError,
			code:   protocol.ErrorCodeIO,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(tt.store)
			var rec *httptest.ResponseRecorder
			if tt.body == nil {
				req := httptest.NewRequest(tt.method, tt.path, nil)
				rec = httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
			} else {
				rec = doJSON(t, handler, tt.method, tt.path, tt.body)
			}
			assertErrorResponse(t, rec, tt.want, tt.code)
		})
	}
}

func TestHandlerRejectsTrailingJSON(t *testing.T) {
	handler := NewHandler(FileStore{TargetRoot: t.TempDir()})
	body := strings.NewReader(`{}` + "\n" + `{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/sessions trailing JSON status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlerRejectsMissingStoreAndUnknownRoute(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		method  string
		path    string
		want    int
		code    protocol.ErrorCode
	}{
		{
			name:    "missing store",
			handler: NewHandler(nil),
			method:  http.MethodGet,
			path:    "/v1/sessions/session-1/status",
			want:    http.StatusInternalServerError,
			code:    protocol.ErrorCodeIO,
		},
		{
			name:    "unknown route",
			handler: NewHandler(FileStore{TargetRoot: t.TempDir()}),
			method:  http.MethodGet,
			path:    "/v1/unknown",
			want:    http.StatusNotFound,
			code:    protocol.ErrorCodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			tt.handler.ServeHTTP(rec, req)
			assertErrorResponse(t, rec, tt.want, tt.code)
		})
	}
}

func TestHandlerRejectsMalformedJSON(t *testing.T) {
	handler := NewHandler(FileStore{TargetRoot: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(`{"session_id":`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/sessions malformed JSON status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var errResp protocol.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("json.Unmarshal(error response) error = %v, want nil body %s", err, rec.Body.String())
	}
	if errResp.Code != protocol.ErrorCodeBadRequest {
		t.Fatalf("POST /v1/sessions malformed JSON error code = %q, want %q", errResp.Code, protocol.ErrorCodeBadRequest)
	}
}

func TestHandlerRejectsOversizedBody(t *testing.T) {
	handler := NewHandler(FileStore{TargetRoot: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/v1/chunks", io.LimitReader(strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)), maxRequestBodyBytes+1))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunks oversized status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlerAcceptsMaxChunkJSONBody(t *testing.T) {
	store := &chunkCaptureStore{
		privacyPolicies: map[string]transport.PrivacyPolicy{"session-1": {}},
	}
	handler := NewHandler(store)
	data := bytes.Repeat([]byte("a"), protocol.MaxChunkBytes)
	body := protocol.ChunkUploadRequest{SessionID: "session-1", Path: "docs/large.bin", Data: data, Final: true}
	rec := doJSON(t, handler, http.MethodPost, "/v1/chunks", body)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/chunks max chunk status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if store.seenBytes != protocol.MaxChunkBytes {
		t.Fatalf("AppendChunk(max chunk) saw %d bytes, want %d", store.seenBytes, protocol.MaxChunkBytes)
	}
}

func TestHandlerMapsTransactionValidationToBadRequest(t *testing.T) {
	handler := NewHandler(FileStore{TargetRoot: t.TempDir()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sessions/bad%2Fid/status", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /v1/sessions/bad%%2Fid/status status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func doJSON(t *testing.T, handler http.Handler, method, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value); err != nil {
		t.Fatalf("json.NewEncoder(...).Encode(%+v) error = %v, want nil", value, err)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func marshalControlDoc(t *testing.T, doc control.Document) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := control.Write(&buf, doc); err != nil {
		t.Fatalf("control.Write(%T) error = %v, want nil", doc, err)
	}
	return buf.Bytes()
}

func readHandlerControlDoc[T control.Document](t *testing.T, target string, artifact control.ArtifactType, id string) T {
	t.Helper()
	path, err := control.Path(target, artifact, id)
	if err != nil {
		t.Fatalf("control.Path(%q, %q) error = %v, want nil", artifact, id, err)
	}
	doc, err := control.ReadFile[T](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return doc
}

func doPaddedChunk(t *testing.T, handler http.Handler, chunk protocol.ChunkUploadRequest, bucket int) *httptest.ResponseRecorder {
	t.Helper()
	return doPaddedChunkWithFrameSession(t, handler, chunk, chunk.SessionID, bucket)
}

func doPaddedChunkWithFrameSession(t *testing.T, handler http.Handler, chunk protocol.ChunkUploadRequest, frameSessionID string, bucket int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(chunk); err != nil {
		t.Fatalf("json.NewEncoder(...).Encode(%+v) error = %v, want nil", chunk, err)
	}
	wire, _, err := padding.Pad(body.Bytes(), padding.Config{
		BucketBytes:   bucket,
		MaxFrameBytes: protocol.MaxPaddedChunkRequestBodyBytes,
	})
	if err != nil {
		t.Fatalf("padding.Pad(chunk) error = %v, want nil", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chunks", bytes.NewReader(wire))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(protocol.FrameEncodingHeader, protocol.FrameEncodingPaddingV1)
	req.Header.Set(protocol.FrameSessionIDHeader, frameSessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func doPaddedChunkBatch(t *testing.T, handler http.Handler, batch protocol.ChunkBatchUploadRequest, bucket int) *httptest.ResponseRecorder {
	t.Helper()
	return doPaddedChunkBatchWithFrameSession(t, handler, batch, batch.SessionID, bucket)
}

func doPaddedChunkBatchWithFrameSession(t *testing.T, handler http.Handler, batch protocol.ChunkBatchUploadRequest, frameSessionID string, bucket int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(batch); err != nil {
		t.Fatalf("json.NewEncoder(...).Encode(%+v) error = %v, want nil", batch, err)
	}
	wire, _, err := padding.Pad(body.Bytes(), padding.Config{
		BucketBytes:   bucket,
		MaxFrameBytes: protocol.MaxPaddedBatchRequestBodyBytes,
	})
	if err != nil {
		t.Fatalf("padding.Pad(batch) error = %v, want nil", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk-batches", bytes.NewReader(wire))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(protocol.FrameEncodingHeader, protocol.FrameEncodingPaddingV1)
	req.Header.Set(protocol.FrameSessionIDHeader, frameSessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func testPrivacyPolicy(bucket int) transport.PrivacyPolicy {
	policy := transport.DefaultPrivacyPolicy(transport.PrivacyLevel2)
	policy.PaddingBucket = bucket
	return policy
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode protocol.ErrorCode) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("response status = %d, want %d body %s", rec.Code, wantStatus, rec.Body.String())
	}
	var got protocol.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(error response) error = %v, want nil body %s", err, rec.Body.String())
	}
	if got.Code != wantCode {
		t.Fatalf("error response code = %q, want %q body %s", got.Code, wantCode, rec.Body.String())
	}
}

type routeErrorStore struct {
	beginErr  error
	statusErr error
	chunkErr  error
	commitErr error
}

func (s routeErrorStore) Begin(req protocol.BeginSessionRequest) (protocol.BeginSessionResponse, error) {
	if s.beginErr != nil {
		return protocol.BeginSessionResponse{}, s.beginErr
	}
	return protocol.BeginSessionResponse{SessionID: req.SessionID, State: protocol.SessionStateValidated}, nil
}

func (s routeErrorStore) Status(sessionID string) (protocol.SessionStatusResponse, error) {
	if s.statusErr != nil {
		return protocol.SessionStatusResponse{}, s.statusErr
	}
	return protocol.SessionStatusResponse{SessionID: sessionID, State: protocol.SessionStateValidated}, nil
}

func (s routeErrorStore) AppendChunk(req protocol.ChunkUploadRequest) (protocol.ChunkUploadResponse, error) {
	if s.chunkErr != nil {
		return protocol.ChunkUploadResponse{}, s.chunkErr
	}
	return protocol.ChunkUploadResponse{SessionID: req.SessionID, Path: req.Path, CommittedSize: int64(len(req.Data)), ChunkState: protocol.ChunkStateAccepted}, nil
}

func (s routeErrorStore) AppendChunkBatch(context.Context, protocol.ChunkBatchUploadRequest) (protocol.ChunkBatchUploadResponse, error) {
	if s.chunkErr != nil {
		return protocol.ChunkBatchUploadResponse{}, s.chunkErr
	}
	return protocol.ChunkBatchUploadResponse{SessionID: "session-1"}, nil
}

func (s routeErrorStore) Commit(req protocol.CommitSessionRequest) (protocol.CommitSessionResponse, error) {
	if s.commitErr != nil {
		return protocol.CommitSessionResponse{}, s.commitErr
	}
	return protocol.CommitSessionResponse{SessionID: req.SessionID, State: protocol.SessionStatePublished, ReceiptID: req.SessionID}, nil
}

func (s routeErrorStore) WriteProfileSnapshot(req protocol.ProfileSnapshotArtifactRequest) (protocol.ArtifactWriteResponse, error) {
	return protocol.ArtifactWriteResponse{SessionID: req.SessionID, Written: 1}, nil
}

func (s routeErrorStore) WriteWarnings(req protocol.WarningArtifactRequest) (protocol.ArtifactWriteResponse, error) {
	return protocol.ArtifactWriteResponse{SessionID: req.SessionID, Written: len(req.Documents)}, nil
}

func (s routeErrorStore) WriteNetworkTransfer(req protocol.NetworkTransferArtifactRequest) (protocol.ArtifactWriteResponse, error) {
	return protocol.ArtifactWriteResponse{SessionID: req.SessionID, Written: 1}, nil
}

type chunkCaptureStore struct {
	seenBytes       int
	seenChunks      []protocol.ChunkUploadRequest
	errAfter        int
	appendErr       error
	cancelAfter     int
	cancel          context.CancelFunc
	privacyPolicies map[string]transport.PrivacyPolicy
}

func (s *chunkCaptureStore) Begin(protocol.BeginSessionRequest) (protocol.BeginSessionResponse, error) {
	return protocol.BeginSessionResponse{}, errors.New("Begin should not be called")
}

func (s *chunkCaptureStore) Status(string) (protocol.SessionStatusResponse, error) {
	return protocol.SessionStatusResponse{}, errors.New("Status should not be called")
}

func (s *chunkCaptureStore) AppendChunk(req protocol.ChunkUploadRequest) (protocol.ChunkUploadResponse, error) {
	if err := req.Validate(); err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	s.seenBytes = len(req.Data)
	s.seenChunks = append(s.seenChunks, req)
	if s.cancel != nil && len(s.seenChunks) == s.cancelAfter {
		s.cancel()
	}
	if s.appendErr != nil && len(s.seenChunks) > s.errAfter {
		return protocol.ChunkUploadResponse{}, s.appendErr
	}
	return protocol.ChunkUploadResponse{
		SessionID:     req.SessionID,
		Path:          req.Path,
		CommittedSize: int64(len(req.Data)),
		ChunkState:    protocol.ChunkStateAccepted,
		Complete:      true,
	}, nil
}

func (s *chunkCaptureStore) AppendChunkBatch(ctx context.Context, req protocol.ChunkBatchUploadRequest) (protocol.ChunkBatchUploadResponse, error) {
	resp := protocol.ChunkBatchUploadResponse{
		SessionID: req.SessionID,
		Chunks:    make([]protocol.ChunkUploadResponse, 0, len(req.Chunks)),
	}
	for _, chunk := range req.Chunks {
		if err := ctx.Err(); err != nil {
			return protocol.ChunkBatchUploadResponse{}, err
		}
		chunkResp, err := s.AppendChunk(chunk)
		if err != nil {
			return protocol.ChunkBatchUploadResponse{}, err
		}
		resp.Chunks = append(resp.Chunks, chunkResp)
	}
	return resp, nil
}

func (s *chunkCaptureStore) Commit(protocol.CommitSessionRequest) (protocol.CommitSessionResponse, error) {
	return protocol.CommitSessionResponse{}, errors.New("Commit should not be called")
}

func (s *chunkCaptureStore) WriteProfileSnapshot(protocol.ProfileSnapshotArtifactRequest) (protocol.ArtifactWriteResponse, error) {
	return protocol.ArtifactWriteResponse{}, errors.New("WriteProfileSnapshot should not be called")
}

func (s *chunkCaptureStore) WriteWarnings(protocol.WarningArtifactRequest) (protocol.ArtifactWriteResponse, error) {
	return protocol.ArtifactWriteResponse{}, errors.New("WriteWarnings should not be called")
}

func (s *chunkCaptureStore) WriteNetworkTransfer(protocol.NetworkTransferArtifactRequest) (protocol.ArtifactWriteResponse, error) {
	return protocol.ArtifactWriteResponse{}, errors.New("WriteNetworkTransfer should not be called")
}

func (s *chunkCaptureStore) SessionPrivacyPolicy(sessionID string) (transport.PrivacyPolicy, error) {
	policy, ok := s.privacyPolicies[sessionID]
	if !ok {
		return transport.PrivacyPolicy{}, ErrSessionNotFound
	}
	return policy, nil
}
