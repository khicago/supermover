package receiver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/protocol"
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
	store := &chunkCaptureStore{}
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

type chunkCaptureStore struct {
	seenBytes int
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
	return protocol.ChunkUploadResponse{
		SessionID:     req.SessionID,
		Path:          req.Path,
		CommittedSize: int64(len(req.Data)),
		ChunkState:    protocol.ChunkStateAccepted,
		Complete:      true,
	}, nil
}

func (s *chunkCaptureStore) Commit(protocol.CommitSessionRequest) (protocol.CommitSessionResponse, error) {
	return protocol.CommitSessionResponse{}, errors.New("Commit should not be called")
}
