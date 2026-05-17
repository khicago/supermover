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
	"github.com/khicago/supermover/internal/transaction"
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

func (s routeErrorStore) Commit(req protocol.CommitSessionRequest) (protocol.CommitSessionResponse, error) {
	if s.commitErr != nil {
		return protocol.CommitSessionResponse{}, s.commitErr
	}
	return protocol.CommitSessionResponse{SessionID: req.SessionID, State: protocol.SessionStatePublished, ReceiptID: req.SessionID}, nil
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
