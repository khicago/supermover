package receiver

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

type AuthenticatedHandlerOptions struct {
	ProfileID      string
	TargetID       string
	SourceDeviceID string
	TargetDeviceID string
}

type TLSReceiverOptions struct {
	Profile      profile.Profile
	Certificates []tls.Certificate
	Now          func() time.Time
}

type TLSReceiver struct {
	Handler   http.Handler
	TLSConfig *tls.Config
	Peer      transport.AuthenticatedPeer
}

func NewAuthenticatedHandlerFromProfile(p profile.Profile) (http.Handler, error) {
	trust, err := pairing.ValidateProfileTrust(p)
	if err != nil {
		return nil, err
	}
	targetRoot := strings.TrimSpace(p.Target.LocalPath)
	store := FileStore{TargetRoot: targetRoot}
	opts := AuthenticatedHandlerOptions{
		ProfileID:      p.ProfileID,
		TargetID:       p.Target.TargetID,
		SourceDeviceID: trust.Receipt.SourceDeviceID,
		TargetDeviceID: trust.TargetDeviceID,
	}
	if err := validateAuthOptions(store, opts); err != nil {
		return nil, err
	}
	return NewAuthenticatedHandler(store, opts), nil
}

func NewTLSReceiverFromProfile(opts TLSReceiverOptions) (TLSReceiver, error) {
	trust, err := pairing.ValidateProfileTrust(opts.Profile)
	if err != nil {
		return TLSReceiver{}, err
	}
	peer := transport.AuthenticatedPeer{
		ProfileID:      opts.Profile.ProfileID,
		TargetID:       opts.Profile.Target.TargetID,
		SourceDeviceID: trust.Receipt.SourceDeviceID,
		TargetDeviceID: trust.TargetDeviceID,
	}
	targetRoot := cleanTargetRoot(opts.Profile.Target.LocalPath)
	handler := NewAuthenticatedHandler(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	})
	if err := validateAuthOptions(FileStore{TargetRoot: targetRoot}, AuthenticatedHandlerOptions{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	}); err != nil {
		return TLSReceiver{}, err
	}
	tlsConfig, err := transport.ServerTLSConfig(transport.ServerTLSOptions{
		Certificates: opts.Certificates,
		Peer:         peer,
		Time:         opts.Now,
	})
	if err != nil {
		return TLSReceiver{}, err
	}
	wrapped, err := transport.NewTLSAuthenticatedPeerHandler(handler, transport.AuthenticatedPeerTLSOptions{
		Peer: peer,
		Time: opts.Now,
	})
	if err != nil {
		return TLSReceiver{}, err
	}
	return TLSReceiver{Handler: wrapped, TLSConfig: tlsConfig, Peer: peer}, nil
}

type authenticatedHandler struct {
	next       http.Handler
	store      FileStore
	want       transport.AuthenticatedPeer
	optionsErr error
}

func NewAuthenticatedHandler(store FileStore, opts AuthenticatedHandlerOptions) http.Handler {
	store.TargetRoot = cleanTargetRoot(store.TargetRoot)
	handler := &authenticatedHandler{
		next:  NewHandler(store),
		store: store,
		want: transport.AuthenticatedPeer{
			ProfileID:      opts.ProfileID,
			TargetID:       opts.TargetID,
			SourceDeviceID: opts.SourceDeviceID,
			TargetDeviceID: opts.TargetDeviceID,
		},
	}
	if err := validateAuthOptions(store, opts); err != nil {
		handler.optionsErr = err
	}
	return handler
}

func (h *authenticatedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.optionsErr != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrorCodeIO, h.optionsErr.Error())
		return
	}
	peer, ok := transport.AuthenticatedPeerFromContext(r.Context())
	if !ok {
		writeError(
			w,
			http.StatusForbidden,
			protocol.ErrorCodeForbidden,
			"authenticated transport identity is required",
		)
		return
	}
	if err := validatePeer(peer); err != nil {
		writeError(w, http.StatusForbidden, protocol.ErrorCodeForbidden, err.Error())
		return
	}
	if peer != h.want {
		writeError(
			w,
			http.StatusForbidden,
			protocol.ErrorCodeForbidden,
			"authenticated transport identity does not match profile pins",
		)
		return
	}
	if err := h.validateRequestScope(w, r, peer); err != nil {
		var scopeErr requestScopeError
		if errors.As(err, &scopeErr) {
			writeError(w, scopeErr.Status, scopeErr.Code, scopeErr.Err.Error())
			return
		}
		writeError(w, http.StatusForbidden, protocol.ErrorCodeForbidden, err.Error())
		return
	}
	h.next.ServeHTTP(w, r)
}

func validateAuthOptions(store FileStore, opts AuthenticatedHandlerOptions) error {
	if err := validateReceiverTargetRoot(store.TargetRoot); err != nil {
		return err
	}
	return validatePeer(transport.AuthenticatedPeer{
		ProfileID:      opts.ProfileID,
		TargetID:       opts.TargetID,
		SourceDeviceID: opts.SourceDeviceID,
		TargetDeviceID: opts.TargetDeviceID,
	})
}

func cleanTargetRoot(root string) string {
	return filepath.Clean(strings.TrimSpace(root))
}

func validatePeer(peer transport.AuthenticatedPeer) error {
	return peer.Validate()
}

func validateReceiverTargetRoot(root string) error {
	root = cleanTargetRoot(root)
	if root == "." || root == "" {
		return fmt.Errorf("%w: target root is required", protocol.ErrValidation)
	}
	for _, segment := range strings.Split(filepath.ToSlash(root), "/") {
		if strings.EqualFold(segment, pathguard.ReservedControlDir) {
			return fmt.Errorf("%w: target root must not be the reserved control directory", pathguard.ErrUnsafePath)
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: target root %q is a symlink", pathguard.ErrUnsafePath, root)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: target root %q is not a directory", pathguard.ErrUnsafePath, root)
	}
	controlDir := filepath.Join(root, pathguard.ReservedControlDir)
	if err := pathguard.EnsureDirectory(root, controlDir); err != nil {
		return err
	}
	return nil
}

func (h *authenticatedHandler) validateRequestScope(
	w http.ResponseWriter,
	r *http.Request,
	peer transport.AuthenticatedPeer,
) error {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
		var req protocol.BeginSessionRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			return badRequestScopeError(err)
		}
		if req.ProfileID != peer.ProfileID ||
			req.TargetID != peer.TargetID ||
			req.SourceDeviceID != peer.SourceDeviceID ||
			req.TargetDeviceID != peer.TargetDeviceID {
			return errors.New("begin session identity does not match authenticated transport")
		}
		r.Body = encodeBody(req)
		return nil
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chunks":
		var req protocol.ChunkUploadRequest
		policy, body, err := h.decodeScopedChunkBody(w, r, peer, decodeChunkRequestBody)
		if err != nil {
			return err
		}
		if err := decodeSingleJSON(body, &req); err != nil {
			return badRequestScopeError(err)
		}
		if policy.SessionID != "" && req.SessionID != policy.SessionID {
			return badRequestScopeError(errors.New("chunk session_id does not match padded frame session id"))
		}
		if policy.SessionID != "" && privacyPolicyRequiresBatching(policy.Policy) {
			return badRequestScopeError(errors.New("session privacy policy requires chunk batch frames"))
		}
		if policy.SessionID == "" {
			if err := h.validateStoredSessionScope(req.SessionID, peer); err != nil {
				return err
			}
			storedPolicy, ok, err := storedChunkSessionPrivacyPolicy(h.store, req.SessionID)
			if err != nil {
				return err
			}
			if ok && privacyPolicyRequiresBatching(storedPolicy) {
				return badRequestScopeError(errors.New("session privacy policy requires chunk batch frames"))
			}
		}
		h.rewriteScopedChunkBody(r, body, policy)
		return nil
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chunk-batches":
		var req protocol.ChunkBatchUploadRequest
		policy, body, err := h.decodeScopedChunkBody(w, r, peer, decodeBatchRequestBody)
		if err != nil {
			return err
		}
		if err := decodeSingleJSON(body, &req); err != nil {
			return badRequestScopeError(err)
		}
		if err := req.Validate(); err != nil {
			return badRequestScopeError(err)
		}
		if policy.SessionID != "" && req.SessionID != policy.SessionID {
			return badRequestScopeError(errors.New("batch session_id does not match padded frame session id"))
		}
		if policy.SessionID != "" {
			if err := validateChunkBatchBounds(req, len(body), policy.Policy); err != nil {
				return badRequestScopeError(err)
			}
		}
		if policy.SessionID == "" {
			if err := h.validateStoredSessionScope(req.SessionID, peer); err != nil {
				return err
			}
		}
		h.rewriteScopedChunkBody(r, body, policy)
		return nil
	case r.Method == http.MethodPost && r.URL.Path == "/v1/commit":
		var req protocol.CommitSessionRequest
		if err := decodeRequestBody(w, r, &req); err != nil {
			return badRequestScopeError(err)
		}
		if err := h.validateStoredSessionScope(req.SessionID, peer); err != nil {
			return err
		}
		r.Body = encodeBody(req)
		return nil
	case r.Method == http.MethodPost &&
		strings.HasPrefix(r.URL.Path, "/v1/sessions/") &&
		strings.Contains(r.URL.Path, "/artifacts/"):
		sessionID := artifactSessionID(r)
		if err := transaction.ValidateSessionID(sessionID); err != nil {
			return badRequestScopeError(err)
		}
		return h.validateStoredSessionScope(sessionID, peer)
	case r.Method == http.MethodGet &&
		strings.HasPrefix(r.URL.Path, "/v1/sessions/") &&
		strings.Contains(r.URL.Path, "/artifacts/"):
		sessionID := artifactSessionID(r)
		if err := transaction.ValidateSessionID(sessionID); err != nil {
			return badRequestScopeError(err)
		}
		return h.validateStoredSessionScope(sessionID, peer)
	case r.Method == http.MethodGet &&
		strings.HasPrefix(r.URL.Path, "/v1/sessions/") &&
		strings.HasSuffix(r.URL.Path, "/status"):
		sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/sessions/"), "/status")
		return h.validateStoredSessionScope(sessionID, peer)
	default:
		return nil
	}
}

func (h *authenticatedHandler) decodeScopedChunkBody(
	w http.ResponseWriter,
	r *http.Request,
	peer transport.AuthenticatedPeer,
	decode func(http.ResponseWriter, *http.Request, sessionPrivacyPolicy) ([]byte, error),
) (sessionPrivacyPolicy, []byte, error) {
	policy, err := lookupChunkSessionPrivacyPolicy(r, h.store)
	if err != nil {
		return sessionPrivacyPolicy{}, nil, err
	}
	if policy.SessionID != "" {
		if err := h.validateStoredSessionScope(policy.SessionID, peer); err != nil {
			return sessionPrivacyPolicy{}, nil, err
		}
	}
	body, err := decode(w, r, policy)
	if err != nil {
		return sessionPrivacyPolicy{}, nil, badRequestScopeError(err)
	}
	return policy, body, nil
}

func (h *authenticatedHandler) rewriteScopedChunkBody(r *http.Request, body []byte, policy sessionPrivacyPolicy) {
	r.Body = io.NopCloser(bytes.NewReader(body))
	clearFrameHeaders(r.Header)
	if policy.SessionID != "" {
		markVerifiedPaddedFrame(r)
	}
}

func (h *authenticatedHandler) validateStoredSessionScope(sessionID string, peer transport.AuthenticatedPeer) error {
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		return badRequestScopeError(err)
	}
	meta, err := h.store.readStoredSessionMeta(sessionID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrSessionNotFound) {
			return notFoundScopeError(fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID))
		}
		return ioScopeError(err)
	}
	if meta.ProfileID != peer.ProfileID ||
		meta.TargetID != peer.TargetID ||
		meta.SourceDeviceID != peer.SourceDeviceID ||
		meta.TargetDeviceID != peer.TargetDeviceID {
		return errors.New("session identity does not match authenticated transport")
	}
	return nil
}

func (s FileStore) readStoredSessionMeta(sessionID string) (sessionMeta, error) {
	return readMeta(s.metaPath(sessionID))
}

type requestScopeError struct {
	Status int
	Code   protocol.ErrorCode
	Err    error
}

func (e requestScopeError) Error() string {
	return e.Err.Error()
}

func badRequestScopeError(err error) error {
	return requestScopeError{Status: http.StatusBadRequest, Code: protocol.ErrorCodeBadRequest, Err: err}
}

func notFoundScopeError(err error) error {
	return requestScopeError{Status: http.StatusNotFound, Code: protocol.ErrorCodeNotFound, Err: err}
}

func ioScopeError(err error) error {
	return requestScopeError{Status: http.StatusInternalServerError, Code: protocol.ErrorCodeIO, Err: err}
}

func decodeRequestBody(w http.ResponseWriter, r *http.Request, dest any) error {
	body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain a single JSON document")
		}
		return err
	}
	return nil
}

func encodeBody(value any) io.ReadCloser {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(value)
	return io.NopCloser(&buf)
}
