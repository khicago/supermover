package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/privacy/padding"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transport"
)

func TestAuthenticatedHandlerRejectsMissingTrustContext(t *testing.T) {
	targetRoot := t.TempDir()
	handler := NewAuthenticatedHandler(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{
			name:   "begin",
			method: http.MethodPost,
			path:   "/v1/sessions",
			body:   validBeginRequest([]byte("abc")),
		},
		{
			name:   "chunk",
			method: http.MethodPost,
			path:   "/v1/chunks",
			body: protocol.ChunkUploadRequest{
				SessionID: "session-1",
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("abc"),
				Final:     true,
			},
		},
		{
			name:   "chunk batch",
			method: http.MethodPost,
			path:   "/v1/chunk-batches",
			body: protocol.ChunkBatchUploadRequest{
				SessionID: "session-1",
				Chunks: []protocol.ChunkUploadRequest{
					{
						SessionID: "session-1",
						Path:      "docs/a.txt",
						Offset:    0,
						Data:      []byte("abc"),
						Final:     true,
					},
				},
			},
		},
		{
			name:   "commit",
			method: http.MethodPost,
			path:   "/v1/commit",
			body: protocol.CommitSessionRequest{
				SessionID: "session-1",
				EndedAt:   time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC),
			},
		},
		{
			name:   "status",
			method: http.MethodGet,
			path:   "/v1/sessions/session-1/status",
			body:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doJSON(t, handler, tt.method, tt.path, tt.body)

			if rec.Code != http.StatusForbidden {
				t.Fatalf(
					"%s %s without trust status = %d, want %d body %s",
					tt.method,
					tt.path,
					rec.Code,
					http.StatusForbidden,
					rec.Body.String(),
				)
			}
			assertErrorResponse(t, rec, http.StatusForbidden, protocol.ErrorCodeForbidden)
		})
	}
	if _, err := os.Stat(filepath.Join(targetRoot, control.DirName, "sessions")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sessions dir after unauthenticated requests error = %v, want not exist", err)
	}
}

func TestAuthenticatedHandlerLoopbackRejectsMissingTrustContext(t *testing.T) {
	targetRoot := t.TempDir()
	handler := NewAuthenticatedHandler(
		FileStore{TargetRoot: targetRoot},
		AuthenticatedHandlerOptions{
			ProfileID:      "profile.default",
			TargetID:       "local:profile.default",
			SourceDeviceID: "sha256:abcdef0123456789",
			TargetDeviceID: "sha256:0123456789abcdef",
		},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	beginBody, err := json.Marshal(validBeginRequest([]byte("abc")))
	if err != nil {
		t.Fatalf("json.Marshal(begin) error = %v, want nil", err)
	}
	tests := []struct {
		name   string
		method string
		path   string
		body   io.Reader
	}{
		{
			name:   "begin",
			method: http.MethodPost,
			path:   "/v1/sessions",
			body:   bytes.NewReader(beginBody),
		},
		{
			name:   "status",
			method: http.MethodGet,
			path:   "/v1/sessions/session-1/status",
			body:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, server.URL+tt.path, tt.body)
			if err != nil {
				t.Fatalf("http.NewRequest(%s %s) error = %v, want nil", tt.method, tt.path, err)
			}
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("server.Client().Do(%s %s) error = %v, want nil", tt.method, tt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf(
					"%s %s without trust status = %d, want %d body %s",
					tt.method,
					tt.path,
					resp.StatusCode,
					http.StatusForbidden,
					string(body),
				)
			}
		})
	}
}

func TestAuthenticatedHandlerRejectsWrongIdentity(t *testing.T) {
	targetRoot := t.TempDir()
	handler := NewAuthenticatedHandler(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil)
	req = req.WithContext(transport.ContextWithAuthenticatedPeer(req.Context(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:fedcba9876543210",
		TargetDeviceID: "sha256:0123456789abcdef",
	}))
	rec := doJSONRequest(t, handler, req, validBeginRequest([]byte("abc")))

	if rec.Code != http.StatusForbidden {
		t.Fatalf(
			"POST /v1/sessions wrong identity status = %d, want %d body %s",
			rec.Code,
			http.StatusForbidden,
			rec.Body.String(),
		)
	}
}

func TestAuthenticatedHandlerAllowsMatchingIdentity(t *testing.T) {
	targetRoot := t.TempDir()
	handler := NewAuthenticatedHandler(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil)
	req = req.WithContext(transport.ContextWithAuthenticatedPeer(req.Context(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}))
	rec := doJSONRequest(t, handler, req, validBeginRequest([]byte("abc")))

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"POST /v1/sessions matching identity status = %d, want %d body %s",
			rec.Code,
			http.StatusAccepted,
			rec.Body.String(),
		)
	}
}

func TestAuthenticatedHandlerRejectsBeginScopeMismatch(t *testing.T) {
	targetRoot := t.TempDir()
	handler := NewAuthenticatedHandler(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	begin := validBeginRequest([]byte("abc"))
	begin.TargetDeviceID = "sha256:fedcba9876543210"
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil)
	req = req.WithContext(transport.ContextWithAuthenticatedPeer(req.Context(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}))
	rec := doJSONRequest(t, handler, req, begin)

	if rec.Code != http.StatusForbidden {
		t.Fatalf(
			"POST /v1/sessions scope mismatch status = %d, want %d body %s",
			rec.Code,
			http.StatusForbidden,
			rec.Body.String(),
		)
	}
}

func TestAuthenticatedHandlerMalformedJSONRemainsBadRequest(t *testing.T) {
	targetRoot := t.TempDir()
	handler := NewAuthenticatedHandler(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(`{"session_id":`))
	req = req.WithContext(transport.ContextWithAuthenticatedPeer(req.Context(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"POST /v1/sessions malformed JSON status = %d, want %d body %s",
			rec.Code,
			http.StatusBadRequest,
			rec.Body.String(),
		)
	}
	assertErrorResponse(t, rec, http.StatusBadRequest, protocol.ErrorCodeBadRequest)
}

func TestAuthenticatedHandlerRejectsOversizedBodyBeforeDelegation(t *testing.T) {
	targetRoot := t.TempDir()
	handler := NewAuthenticatedHandler(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/chunks",
		io.LimitReader(strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)), maxRequestBodyBytes+1),
	)
	req = req.WithContext(transport.ContextWithAuthenticatedPeer(req.Context(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/chunks oversized status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusBadRequest, protocol.ErrorCodeBadRequest)
}

func TestAuthenticatedHandlerRejectsExistingSessionScopeMismatch(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	trusted := transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}
	begin := validBeginRequest([]byte("abc"))
	begin.TargetID = "local:other"
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", begin, err)
	}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      trusted.ProfileID,
		TargetID:       "local:profile.default",
		SourceDeviceID: trusted.SourceDeviceID,
		TargetDeviceID: trusted.TargetDeviceID,
	})
	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{
			name:   "chunk",
			method: http.MethodPost,
			path:   "/v1/chunks",
			body: protocol.ChunkUploadRequest{
				SessionID: begin.SessionID,
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("abc"),
				Final:     true,
			},
		},
		{
			name:   "chunk batch",
			method: http.MethodPost,
			path:   "/v1/chunk-batches",
			body: protocol.ChunkBatchUploadRequest{
				SessionID: begin.SessionID,
				Chunks: []protocol.ChunkUploadRequest{
					{
						SessionID: begin.SessionID,
						Path:      "docs/a.txt",
						Offset:    0,
						Data:      []byte("abc"),
						Final:     true,
					},
				},
			},
		},
		{
			name:   "commit",
			method: http.MethodPost,
			path:   "/v1/commit",
			body: protocol.CommitSessionRequest{
				SessionID: begin.SessionID,
				EndedAt:   time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC),
			},
		},
		{
			name:   "status",
			method: http.MethodGet,
			path:   "/v1/sessions/" + begin.SessionID + "/status",
			body:   nil,
		},
		{
			name:   "network transfer artifact",
			method: http.MethodGet,
			path:   "/v1/sessions/" + begin.SessionID + "/artifacts/network-transfer",
			body:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req = req.WithContext(transport.ContextWithAuthenticatedPeer(context.Background(), trusted))
			rec := doJSONRequest(t, handler, req, tt.body)

			if rec.Code != http.StatusForbidden {
				t.Fatalf(
					"%s %s wrong session scope status = %d, want %d body %s",
					tt.method,
					tt.path,
					rec.Code,
					http.StatusForbidden,
					rec.Body.String(),
				)
			}
		})
	}
}

func TestAuthenticatedHandlerUnknownSessionStillUsesStoreNotFound(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-missing/status", nil)
	req = req.WithContext(transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET status unknown session = %d, want %d body %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusNotFound, protocol.ErrorCodeNotFound)
}

func TestAuthenticatedHandlerUnknownSessionRejectsMutationBeforeStore(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	tests := []struct {
		name string
		path string
		body any
	}{
		{
			name: "chunk",
			path: "/v1/chunks",
			body: protocol.ChunkUploadRequest{
				SessionID: "session-missing",
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("abc"),
				Final:     true,
			},
		},
		{
			name: "chunk batch",
			path: "/v1/chunk-batches",
			body: protocol.ChunkBatchUploadRequest{
				SessionID: "session-missing",
				Chunks: []protocol.ChunkUploadRequest{
					{
						SessionID: "session-missing",
						Path:      "docs/a.txt",
						Offset:    0,
						Data:      []byte("abc"),
						Final:     true,
					},
				},
			},
		},
		{
			name: "commit",
			path: "/v1/commit",
			body: protocol.CommitSessionRequest{
				SessionID: "session-missing",
				EndedAt:   time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil).WithContext(ctx)
			rec := doJSONRequest(t, handler, req, tt.body)

			if rec.Code != http.StatusNotFound {
				t.Fatalf(
					"POST %s missing session status = %d, want %d body %s",
					tt.path,
					rec.Code,
					http.StatusNotFound,
					rec.Body.String(),
				)
			}
			assertErrorResponse(t, rec, http.StatusNotFound, protocol.ErrorCodeNotFound)
		})
	}
	missingSessionDir := filepath.Join(store.TargetRoot, control.DirName, "sessions", "session-missing")
	if _, err := os.Stat(missingSessionDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing session dir error = %v, want not exist", err)
	}
}

func TestAuthenticatedHandlerCorruptSessionMetaIsIOError(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	begin := validBeginRequest([]byte("abc"))
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", begin, err)
	}
	metaPath := store.metaPath(begin.SessionID)
	if err := os.WriteFile(metaPath, []byte(`{"protocol_version":`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(corrupt meta) error = %v, want nil", err)
	}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+begin.SessionID+"/status", nil)
	req = req.WithContext(transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET status corrupt meta = %d, want %d body %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusInternalServerError, protocol.ErrorCodeIO)
}

func TestNewAuthenticatedHandlerRejectsUnsafeTargetRoot(t *testing.T) {
	parent := t.TempDir()
	fileRoot := filepath.Join(parent, "target-file")
	if err := os.WriteFile(fileRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(file root) error = %v, want nil", err)
	}
	symlinkRoot := filepath.Join(parent, "target-link")
	if err := os.Symlink(t.TempDir(), symlinkRoot); err != nil {
		t.Skipf("os.Symlink(target root) unavailable: %v", err)
	}
	controlLinkRoot := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(controlLinkRoot, pathguard.ReservedControlDir)); err != nil {
		t.Skipf("os.Symlink(.supermover) unavailable: %v", err)
	}
	tests := []struct {
		name string
		root string
	}{
		{name: "reserved segment", root: filepath.Join(parent, pathguard.ReservedControlDir, "target")},
		{name: "file root", root: fileRoot},
		{name: "symlink root", root: symlinkRoot},
		{name: "symlinked control plane", root: controlLinkRoot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthenticatedHandler(FileStore{TargetRoot: tt.root}, AuthenticatedHandlerOptions{
				ProfileID:      "profile.default",
				TargetID:       "local:profile.default",
				SourceDeviceID: "sha256:abcdef0123456789",
				TargetDeviceID: "sha256:0123456789abcdef",
			})
			req := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/status", nil)
			req = req.WithContext(transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
				ProfileID:      "profile.default",
				TargetID:       "local:profile.default",
				SourceDeviceID: "sha256:abcdef0123456789",
				TargetDeviceID: "sha256:0123456789abcdef",
			}))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf(
					"GET status unsafe target root = %d, want %d body %s",
					rec.Code,
					http.StatusInternalServerError,
					rec.Body.String(),
				)
			}
		})
	}
}

func doJSONRequest(t *testing.T, handler http.Handler, req *http.Request, value any) *httptest.ResponseRecorder {
	t.Helper()
	body := httptest.NewRequest(req.Method, req.URL.String(), nil)
	body = body.WithContext(req.Context())
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value); err != nil {
		t.Fatalf("json.NewEncoder(...).Encode(%+v) error = %v, want nil", value, err)
	}
	body.Body = io.NopCloser(&buf)
	body.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, body)
	return rec
}

func TestAuthenticatedHandlerCommitReplayRemainsIdempotent(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	begin := validBeginRequest([]byte("abc"))
	beginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, beginReq, begin); rec.Code != http.StatusAccepted {
		t.Fatalf("begin status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	beginReplayReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, beginReplayReq, begin); rec.Code != http.StatusAccepted {
		t.Fatalf("begin replay status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	chunk := protocol.ChunkUploadRequest{
		SessionID: begin.SessionID,
		Path:      "docs/a.txt",
		Offset:    0,
		Data:      []byte("abc"),
		Final:     true,
	}
	chunkReq := httptest.NewRequest(http.MethodPost, "/v1/chunks", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, chunkReq, chunk); rec.Code != http.StatusAccepted {
		t.Fatalf("chunk status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	commit := protocol.CommitSessionRequest{
		SessionID: begin.SessionID,
		EndedAt:   time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC),
	}
	commitReq := httptest.NewRequest(http.MethodPost, "/v1/commit", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, commitReq, commit); rec.Code != http.StatusOK {
		t.Fatalf("commit status = %d, want %d body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	commitReplayReq := httptest.NewRequest(http.MethodPost, "/v1/commit", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, commitReplayReq, commit); rec.Code != http.StatusOK {
		t.Fatalf("commit replay status = %d, want %d body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestAuthenticatedHandlerRejectsVerifiedPaddedLevel2ChunkWhenBatchingRequired(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	begin := validBeginRequest([]byte("abc"))
	begin.PrivacyPolicy = testPrivacyPolicy(64)
	beginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, beginReq, begin); rec.Code != http.StatusAccepted {
		t.Fatalf("begin status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	chunk := protocol.ChunkUploadRequest{
		SessionID: begin.SessionID,
		Path:      "docs/a.txt",
		Offset:    0,
		Data:      []byte("abc"),
		Digest:    digest([]byte("abc")),
		Final:     true,
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(chunk); err != nil {
		t.Fatalf("json.Encode(chunk) error = %v, want nil", err)
	}
	wire, _, err := padding.Pad(body.Bytes(), padding.Config{BucketBytes: 64, MaxFrameBytes: protocol.MaxPaddedChunkRequestBodyBytes})
	if err != nil {
		t.Fatalf("padding.Pad(chunk) error = %v, want nil", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chunks", bytes.NewReader(wire)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(protocol.FrameEncodingHeader, protocol.FrameEncodingPaddingV1)
	req.Header.Set(protocol.FrameSessionIDHeader, begin.SessionID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("padded chunk status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusBadRequest, protocol.ErrorCodeBadRequest)
}

func TestAuthenticatedHandlerAcceptsVerifiedPaddedLevel2ChunkBatch(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	begin := validBeginRequest([]byte("abcd"))
	begin.PrivacyPolicy = testPrivacyPolicy(64)
	beginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, beginReq, begin); rec.Code != http.StatusAccepted {
		t.Fatalf("begin status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: begin.SessionID,
		Chunks: []protocol.ChunkUploadRequest{
			{
				SessionID: begin.SessionID,
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("ab"),
				Digest:    digest([]byte("ab")),
			},
			{
				SessionID: begin.SessionID,
				Path:      "docs/a.txt",
				Offset:    2,
				Data:      []byte("cd"),
				Digest:    digest([]byte("cd")),
				Final:     true,
			},
		},
	}
	req := newPaddedBatchRequest(t, ctx, batch, begin.SessionID, 64)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("padded chunk batch status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var resp protocol.ChunkBatchUploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(batch response) error = %v, want nil body %s", err, rec.Body.String())
	}
	if len(resp.Chunks) != 2 {
		t.Fatalf("padded chunk batch response chunks = %d, want 2", len(resp.Chunks))
	}
}

func TestAuthenticatedHandlerRejectsUnframedLevel2ChunkBatch(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	begin := validBeginRequest([]byte("abc"))
	begin.PrivacyPolicy = testPrivacyPolicy(64)
	beginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, beginReq, begin); rec.Code != http.StatusAccepted {
		t.Fatalf("begin status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: begin.SessionID,
		Chunks: []protocol.ChunkUploadRequest{
			{
				SessionID: begin.SessionID,
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("abc"),
				Digest:    digest([]byte("abc")),
				Final:     true,
			},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk-batches", nil).WithContext(ctx)

	rec := doJSONRequest(t, handler, req, batch)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unframed level2 chunk batch status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusBadRequest, protocol.ErrorCodeBadRequest)
}

func TestAuthenticatedHandlerRejectsPaddedChunkBatchBeyondSessionPolicyBounds(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	begin := validBeginRequest([]byte("abcd"))
	begin.PrivacyPolicy = testPrivacyPolicy(64)
	begin.PrivacyPolicy.BatchMaxCount = 1
	beginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx)
	if rec := doJSONRequest(t, handler, beginReq, begin); rec.Code != http.StatusAccepted {
		t.Fatalf("begin status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: begin.SessionID,
		Chunks: []protocol.ChunkUploadRequest{
			{
				SessionID: begin.SessionID,
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("ab"),
				Digest:    digest([]byte("ab")),
			},
			{
				SessionID: begin.SessionID,
				Path:      "docs/a.txt",
				Offset:    2,
				Data:      []byte("cd"),
				Digest:    digest([]byte("cd")),
				Final:     true,
			},
		},
	}
	req := newPaddedBatchRequest(t, ctx, batch, begin.SessionID, 64)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("padded chunk batch beyond policy status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusBadRequest, protocol.ErrorCodeBadRequest)
}

func TestAuthenticatedHandlerRejectsPaddedChunkBatchScopeMismatch(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	trusted := transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	}
	other := validBeginRequest([]byte("abc"))
	other.SessionID = "session-other"
	other.TargetID = "local:other"
	other.PrivacyPolicy = testPrivacyPolicy(64)
	if _, err := store.Begin(other); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", other, err)
	}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      trusted.ProfileID,
		TargetID:       trusted.TargetID,
		SourceDeviceID: trusted.SourceDeviceID,
		TargetDeviceID: trusted.TargetDeviceID,
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), trusted)
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: other.SessionID,
		Chunks: []protocol.ChunkUploadRequest{
			{
				SessionID: other.SessionID,
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("abc"),
				Digest:    digest([]byte("abc")),
				Final:     true,
			},
		},
	}
	req := newPaddedBatchRequest(t, ctx, batch, other.SessionID, 64)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("padded chunk batch cross-session scope status = %d, want %d body %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusForbidden, protocol.ErrorCodeForbidden)
}

func TestAuthenticatedHandlerRejectsPaddedChunkBatchFrameSessionMismatch(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	handler := NewAuthenticatedHandler(store, AuthenticatedHandlerOptions{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
	})
	begin := validBeginRequest([]byte("abc"))
	begin.PrivacyPolicy = testPrivacyPolicy(64)
	if rec := doJSONRequest(t, handler, httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx), begin); rec.Code != http.StatusAccepted {
		t.Fatalf("begin status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	other := validBeginRequest([]byte("abc"))
	other.SessionID = "session-other"
	other.PrivacyPolicy = testPrivacyPolicy(64)
	if rec := doJSONRequest(t, handler, httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx), other); rec.Code != http.StatusAccepted {
		t.Fatalf("begin other status = %d, want %d body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	batch := protocol.ChunkBatchUploadRequest{
		SessionID: begin.SessionID,
		Chunks: []protocol.ChunkUploadRequest{
			{
				SessionID: begin.SessionID,
				Path:      "docs/a.txt",
				Offset:    0,
				Data:      []byte("abc"),
				Digest:    digest([]byte("abc")),
				Final:     true,
			},
		},
	}
	req := newPaddedBatchRequest(t, ctx, batch, other.SessionID, 64)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("padded chunk batch frame/body mismatch status = %d, want %d body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusBadRequest, protocol.ErrorCodeBadRequest)
}

func newPaddedBatchRequest(
	t *testing.T,
	ctx context.Context,
	batch protocol.ChunkBatchUploadRequest,
	frameSessionID string,
	bucket int,
) *http.Request {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(batch); err != nil {
		t.Fatalf("json.Encode(batch) error = %v, want nil", err)
	}
	wire, _, err := padding.Pad(body.Bytes(), padding.Config{BucketBytes: bucket, MaxFrameBytes: protocol.MaxPaddedBatchRequestBodyBytes})
	if err != nil {
		t.Fatalf("padding.Pad(batch) error = %v, want nil", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk-batches", bytes.NewReader(wire)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(protocol.FrameEncodingHeader, protocol.FrameEncodingPaddingV1)
	req.Header.Set(protocol.FrameSessionIDHeader, frameSessionID)
	return req
}

func TestNewAuthenticatedHandlerFromProfileBindsPairingTrust(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile.default", "Profile", source, target)
	p.Target.TargetID = "target.default"
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	writePairingReceiptForReceiver(t, target, p)
	handler, err := NewAuthenticatedHandlerFromProfile(p)
	if err != nil {
		t.Fatalf("NewAuthenticatedHandlerFromProfile() error = %v, want nil", err)
	}
	ctx := transport.ContextWithAuthenticatedPeer(context.Background(), transport.AuthenticatedPeer{
		ProfileID:      p.ProfileID,
		TargetID:       p.Target.TargetID,
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: p.Target.DevicePublicKey,
	})
	begin := validBeginRequest([]byte("abc"))
	begin.ProfileID = p.ProfileID
	begin.TargetID = p.Target.TargetID
	begin.TargetDeviceID = p.Target.DevicePublicKey
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil).WithContext(ctx)
	rec := doJSONRequest(t, handler, req, begin)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"POST /v1/sessions profile-bound status = %d, want %d body %s",
			rec.Code,
			http.StatusAccepted,
			rec.Body.String(),
		)
	}
}

func TestNewAuthenticatedHandlerFromProfileRejectsUnpairedProfile(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile.default", "Profile", source, target)
	p.Target.TargetID = "target.default"

	if _, err := NewAuthenticatedHandlerFromProfile(p); err == nil {
		t.Fatal("NewAuthenticatedHandlerFromProfile(unpaired) error = nil, want error")
	}
}

func TestNewAuthenticatedHandlerFromProfileRejectsUnsafeTargetRoot(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(target, pathguard.ReservedControlDir)); err != nil {
		t.Skipf("os.Symlink(.supermover) unavailable: %v", err)
	}
	p := profile.NewDefault("profile.default", "Profile", source, target)
	p.Target.TargetID = "target.default"
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"

	if _, err := NewAuthenticatedHandlerFromProfile(p); err == nil {
		t.Fatal("NewAuthenticatedHandlerFromProfile(unsafe target root) error = nil, want error")
	}
}

func writePairingReceiptForReceiver(t *testing.T, target string, p profile.Profile) {
	t.Helper()
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               p.Target.PairingReceiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:abcdef0123456789",
		TargetDeviceID:   p.Target.DevicePublicKey,
		DevicePublicKey:  p.Target.DevicePublicKey,
		Method:           "sas",
		VerifiedAt:       p.Target.PairedAt,
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	path, err := control.Path(target, control.ArtifactPairingReceipt, receipt.ID)
	if err != nil {
		t.Fatalf("control.Path(pairing receipt) error = %v, want nil", err)
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(pairing receipt) error = %v, want nil", err)
	}
}
