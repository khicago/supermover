package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/privacy/padding"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

const maxRequestBodyBytes = protocol.MaxChunkRequestBodyBytes

type Handler struct {
	Store Store
}

func NewHandler(store Store) http.Handler {
	return Handler{Store: store}
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrorCodeIO, "receiver store is not configured")
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
		h.handleBegin(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/status"):
		h.handleStatus(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chunks":
		h.handleChunk(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chunk-batches":
		h.handleChunkBatch(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/commit":
		h.handleCommit(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/artifacts/profile"):
		h.handleProfileSnapshotArtifact(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/artifacts/warnings"):
		h.handleWarningArtifacts(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/artifacts/network-transfer"):
		h.handleNetworkTransferArtifact(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/artifacts/network-transfer"):
		h.handleReadNetworkTransferArtifact(w, r)
	default:
		writeError(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "route not found")
	}
}

func (h Handler) handleBegin(w http.ResponseWriter, r *http.Request) {
	var req protocol.BeginSessionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.Store.Begin(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/sessions/"), "/status")
	resp, err := h.Store.Status(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h Handler) handleChunk(w http.ResponseWriter, r *http.Request) {
	var req protocol.ChunkUploadRequest
	if !decodeChunkJSON(w, r, h.Store, &req) {
		return
	}
	resp, err := h.Store.AppendChunk(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h Handler) handleChunkBatch(w http.ResponseWriter, r *http.Request) {
	var req protocol.ChunkBatchUploadRequest
	if !decodeChunkBatchJSON(w, r, h.Store, &req) {
		return
	}
	if err := r.Context().Err(); err != nil {
		writeStoreError(w, err)
		return
	}
	batchStore, ok := h.Store.(BatchStore)
	if !ok {
		writeStoreError(w, errors.New("receiver store does not support chunk batches"))
		return
	}
	resp, err := batchStore.AppendChunkBatch(r.Context(), req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h Handler) handleCommit(w http.ResponseWriter, r *http.Request) {
	var req protocol.CommitSessionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.Store.Commit(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h Handler) handleProfileSnapshotArtifact(w http.ResponseWriter, r *http.Request) {
	var req protocol.ProfileSnapshotArtifactRequest
	if !decodeArtifactJSON(w, r, artifactSessionID(r), &req) {
		return
	}
	resp, err := h.Store.WriteProfileSnapshot(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h Handler) handleWarningArtifacts(w http.ResponseWriter, r *http.Request) {
	var req protocol.WarningArtifactRequest
	if !decodeArtifactJSON(w, r, artifactSessionID(r), &req) {
		return
	}
	resp, err := h.Store.WriteWarnings(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h Handler) handleNetworkTransferArtifact(w http.ResponseWriter, r *http.Request) {
	var req protocol.NetworkTransferArtifactRequest
	if !decodeArtifactJSON(w, r, artifactSessionID(r), &req) {
		return
	}
	resp, err := h.Store.WriteNetworkTransfer(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h Handler) handleReadNetworkTransferArtifact(w http.ResponseWriter, r *http.Request) {
	reader, ok := h.Store.(NetworkTransferReader)
	if !ok {
		writeStoreError(w, errors.New("receiver store does not support network transfer artifact reads"))
		return
	}
	sessionID := artifactSessionID(r)
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		writeStoreError(w, err)
		return
	}
	doc, err := reader.ReadNetworkTransfer(sessionID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if doc.SessionID != sessionID {
		writeStoreError(w, fmt.Errorf("%w: network transfer session_id %q does not match path session_id %q", ErrConflict, doc.SessionID, sessionID))
		return
	}
	var buf bytes.Buffer
	if err := control.Write(&buf, doc); err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	return decodeJSONWithLimit(w, r, dest, maxRequestBodyBytes)
}

type artifactRequest interface {
	Validate() error
}

func decodeArtifactJSON(w http.ResponseWriter, r *http.Request, pathSessionID string, dest artifactRequest) bool {
	if strings.TrimSpace(pathSessionID) == "" {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "session id is required")
		return false
	}
	if err := decodeJSONWithLimitError(w, r, dest, protocol.MaxArtifactDocumentBytes+256*1024); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	if err := dest.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	if requestSessionID(dest) != pathSessionID {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "request session_id does not match path session id")
		return false
	}
	return true
}

func artifactSessionID(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	id, _, _ := strings.Cut(path, "/artifacts/")
	return id
}

func requestSessionID(req any) string {
	switch v := req.(type) {
	case *protocol.ProfileSnapshotArtifactRequest:
		return v.SessionID
	case *protocol.WarningArtifactRequest:
		return v.SessionID
	case *protocol.NetworkTransferArtifactRequest:
		return v.SessionID
	default:
		return ""
	}
}

func decodeChunkJSON(w http.ResponseWriter, r *http.Request, store Store, dest *protocol.ChunkUploadRequest) bool {
	policy, ok := chunkSessionPrivacyPolicy(w, r, store)
	if !ok {
		return false
	}
	body, err := decodeChunkRequestBody(w, r, policy)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	if err := decodeSingleJSON(body, dest); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	if policy.SessionID != "" && dest.SessionID != policy.SessionID {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "chunk session_id does not match padded frame session id")
		return false
	}
	if policy.SessionID != "" && privacyPolicyRequiresBatching(policy.Policy) {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "session privacy policy requires chunk batch frames")
		return false
	}
	if policy.SessionID == "" && !requestHasVerifiedPaddedFrame(r) {
		storedPolicy, ok, err := storedChunkSessionPrivacyPolicy(store, dest.SessionID)
		if err != nil {
			var scopeErr requestScopeError
			if errors.As(err, &scopeErr) {
				writeError(w, scopeErr.Status, scopeErr.Code, scopeErr.Err.Error())
			} else {
				writeStoreError(w, err)
			}
			return false
		}
		if ok && privacyPolicyRequiresPaddedChunks(storedPolicy) {
			writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "session privacy policy requires padded chunk frames")
			return false
		}
		if ok && privacyPolicyRequiresBatching(storedPolicy) {
			writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "session privacy policy requires chunk batch frames")
			return false
		}
	}
	return true
}

func decodeChunkBatchJSON(w http.ResponseWriter, r *http.Request, store Store, dest *protocol.ChunkBatchUploadRequest) bool {
	policy, ok := chunkSessionPrivacyPolicy(w, r, store)
	if !ok {
		return false
	}
	body, err := decodeBatchRequestBody(w, r, policy)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	if err := decodeSingleJSON(body, dest); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	if err := dest.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	if policy.SessionID != "" && dest.SessionID != policy.SessionID {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "batch session_id does not match padded frame session id")
		return false
	}
	if policy.SessionID != "" {
		if err := validateChunkBatchBounds(*dest, len(body), policy.Policy); err != nil {
			writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
			return false
		}
	}
	if policy.SessionID == "" && !requestHasVerifiedPaddedFrame(r) {
		storedPolicy, ok, err := storedChunkSessionPrivacyPolicy(store, dest.SessionID)
		if err != nil {
			var scopeErr requestScopeError
			if errors.As(err, &scopeErr) {
				writeError(w, scopeErr.Status, scopeErr.Code, scopeErr.Err.Error())
			} else {
				writeStoreError(w, err)
			}
			return false
		}
		if ok && privacyPolicyRequiresPaddedChunks(storedPolicy) {
			writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "session privacy policy requires padded batch frames")
			return false
		}
		if ok {
			if err := validateChunkBatchBounds(*dest, len(body), storedPolicy); err != nil {
				writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
				return false
			}
		}
	}
	return true
}

func chunkSessionPrivacyPolicy(w http.ResponseWriter, r *http.Request, store Store) (sessionPrivacyPolicy, bool) {
	policy, err := lookupChunkSessionPrivacyPolicy(r, store)
	if err != nil {
		var scopeErr requestScopeError
		if errors.As(err, &scopeErr) {
			writeError(w, scopeErr.Status, scopeErr.Code, scopeErr.Err.Error())
		} else {
			writeStoreError(w, err)
		}
		return sessionPrivacyPolicy{}, false
	}
	return policy, true
}

func lookupChunkSessionPrivacyPolicy(r *http.Request, store Store) (sessionPrivacyPolicy, error) {
	if strings.TrimSpace(r.Header.Get(protocol.FrameEncodingHeader)) == "" {
		return sessionPrivacyPolicy{}, nil
	}
	sessionID := strings.TrimSpace(r.Header.Get(protocol.FrameSessionIDHeader))
	if sessionID == "" {
		return sessionPrivacyPolicy{}, badRequestScopeError(fmt.Errorf("%s is required", protocol.FrameSessionIDHeader))
	}
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		return sessionPrivacyPolicy{}, badRequestScopeError(err)
	}
	policyStore, ok := store.(SessionPrivacyPolicyStore)
	if !ok {
		return sessionPrivacyPolicy{}, badRequestScopeError(errors.New("receiver store does not expose session privacy policy"))
	}
	policy, err := policyStore.SessionPrivacyPolicy(sessionID)
	if err != nil {
		return sessionPrivacyPolicy{}, scopeErrorFromStore(err)
	}
	return sessionPrivacyPolicy{SessionID: sessionID, Policy: policy}, nil
}

func storedChunkSessionPrivacyPolicy(store Store, sessionID string) (transport.PrivacyPolicy, bool, error) {
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		return transport.PrivacyPolicy{}, false, badRequestScopeError(err)
	}
	policyStore, ok := store.(SessionPrivacyPolicyStore)
	if !ok {
		return transport.PrivacyPolicy{}, false, nil
	}
	policy, err := policyStore.SessionPrivacyPolicy(sessionID)
	if err != nil {
		return transport.PrivacyPolicy{}, true, scopeErrorFromStore(err)
	}
	return policy, true, nil
}

func privacyPolicyRequiresPaddedChunks(policy transport.PrivacyPolicy) bool {
	return policy.Level == transport.PrivacyLevel2 && !policy.DisablePadding
}

func privacyPolicyRequiresBatching(policy transport.PrivacyPolicy) bool {
	return policy.Level == transport.PrivacyLevel2 && !policy.DisableBatching
}

func validateChunkBatchBounds(req protocol.ChunkBatchUploadRequest, plainBodyBytes int, policy transport.PrivacyPolicy) error {
	if policy.Level != transport.PrivacyLevel2 || policy.DisableBatching {
		return nil
	}
	if policy.BatchMaxCount <= 0 || policy.BatchMaxBytes <= 0 {
		return errors.New("session privacy policy requires positive batch bounds")
	}
	if len(req.Chunks) > policy.BatchMaxCount {
		return fmt.Errorf("batch chunks %d exceeds session batch_max_count %d", len(req.Chunks), policy.BatchMaxCount)
	}
	if plainBodyBytes > policy.BatchMaxBytes {
		return fmt.Errorf("batch body bytes %d exceeds session batch_max_bytes %d", plainBodyBytes, policy.BatchMaxBytes)
	}
	return nil
}

func scopeErrorFromStore(err error) error {
	switch {
	case errors.Is(err, protocol.ErrValidation), errors.Is(err, transaction.ErrValidation):
		return badRequestScopeError(err)
	case errors.Is(err, ErrSessionNotFound):
		return notFoundScopeError(err)
	default:
		return ioScopeError(err)
	}
}

func decodeJSONWithLimit(w http.ResponseWriter, r *http.Request, dest any, limit int64) bool {
	if err := decodeJSONWithLimitError(w, r, dest, limit); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	return true
}

func decodeJSONWithLimitError(w http.ResponseWriter, r *http.Request, dest any, limit int64) error {
	body, err := readRequestBody(w, r, limit)
	if err != nil {
		return err
	}
	return decodeSingleJSON(body, dest)
}

type sessionPrivacyPolicy struct {
	SessionID string
	Policy    transport.PrivacyPolicy
}

type verifiedPaddedFrameContextKey struct{}

func markVerifiedPaddedFrame(r *http.Request) {
	*r = *r.WithContext(context.WithValue(r.Context(), verifiedPaddedFrameContextKey{}, true))
}

func requestHasVerifiedPaddedFrame(r *http.Request) bool {
	verified, _ := r.Context().Value(verifiedPaddedFrameContextKey{}).(bool)
	return verified
}

func decodeChunkRequestBody(w http.ResponseWriter, r *http.Request, policy sessionPrivacyPolicy) ([]byte, error) {
	body, err := readRequestBody(w, r, protocol.MaxPaddedChunkRequestBodyBytes)
	if err != nil {
		return nil, err
	}
	return decodeFrameBody(r, body, policy, protocol.MaxPaddedChunkRequestBodyBytes)
}

func decodeBatchRequestBody(w http.ResponseWriter, r *http.Request, policy sessionPrivacyPolicy) ([]byte, error) {
	limit := int64(protocol.MaxBatchPlainBodyBytes)
	maxFrameBytes := protocol.MaxBatchPlainBodyBytes
	if strings.TrimSpace(r.Header.Get(protocol.FrameEncodingHeader)) != "" {
		limit = int64(protocol.MaxPaddedBatchRequestBodyBytes)
		maxFrameBytes = protocol.MaxPaddedBatchRequestBodyBytes
		if policy.SessionID != "" && policy.Policy.Level == transport.PrivacyLevel2 && policy.Policy.BatchMaxBytes > 0 && policy.Policy.PaddingBucket > 0 {
			paddedLen, _, err := padding.PaddedLen(policy.Policy.BatchMaxBytes, padding.Config{
				BucketBytes:   policy.Policy.PaddingBucket,
				MaxFrameBytes: protocol.MaxPaddedBatchRequestBodyBytes,
			})
			if err != nil {
				return nil, fmt.Errorf("session privacy policy padded batch bounds: %w", err)
			}
			limit = int64(paddedLen)
			maxFrameBytes = paddedLen
		}
	} else if policy.SessionID != "" && policy.Policy.Level == transport.PrivacyLevel2 && policy.Policy.BatchMaxBytes > 0 {
		limit = int64(policy.Policy.BatchMaxBytes)
		maxFrameBytes = policy.Policy.BatchMaxBytes
	}
	body, err := readRequestBody(w, r, limit)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(r.Header.Get(protocol.FrameEncodingHeader)) == "" && len(body) > protocol.MaxBatchPlainBodyBytes {
		return nil, fmt.Errorf("batch body bytes %d exceeds maximum %d", len(body), protocol.MaxBatchPlainBodyBytes)
	}
	return decodeFrameBody(r, body, policy, maxFrameBytes)
}

func readRequestBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	defer r.Body.Close()
	body := http.MaxBytesReader(w, r.Body, limit)
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func decodeFrameBody(r *http.Request, body []byte, policy sessionPrivacyPolicy, maxFrameBytes int) ([]byte, error) {
	encoding := strings.TrimSpace(r.Header.Get(protocol.FrameEncodingHeader))
	if encoding == "" {
		return body, nil
	}
	if encoding != protocol.FrameEncodingPaddingV1 {
		return nil, errors.New("unsupported frame encoding")
	}
	sessionID := strings.TrimSpace(r.Header.Get(protocol.FrameSessionIDHeader))
	if sessionID == "" {
		return nil, fmt.Errorf("%s is required", protocol.FrameSessionIDHeader)
	}
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		return nil, err
	}
	if policy.SessionID != "" && sessionID != policy.SessionID {
		return nil, errors.New("frame session id does not match authenticated request scope")
	}
	framePolicy := policy.Policy
	if framePolicy.Level == 0 {
		return nil, errors.New("session privacy policy is required for padded chunk frames")
	}
	if err := framePolicy.Validate(); err != nil {
		return nil, fmt.Errorf("session privacy policy: %w", err)
	}
	if !privacyPolicyRequiresPaddedChunks(framePolicy) {
		return nil, errors.New("session privacy policy does not allow padded chunk frames")
	}
	if framePolicy.PaddingBucket > protocol.MaxPaddingBucketBytes {
		return nil, fmt.Errorf("session padding bucket exceeds maximum %d", protocol.MaxPaddingBucketBytes)
	}
	plain, _, err := padding.Unpad(body, padding.Config{
		BucketBytes:   framePolicy.PaddingBucket,
		MaxFrameBytes: maxFrameBytes,
	})
	if err != nil {
		return nil, err
	}
	return plain, nil
}

func clearFrameHeaders(header http.Header) {
	header.Del(protocol.FrameEncodingHeader)
	header.Del(protocol.FrameSessionIDHeader)
}

func decodeSingleJSON(data []byte, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("request body must contain a single JSON document")
		}
		return err
	}
	return nil
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, protocol.ErrValidation), errors.Is(err, transaction.ErrValidation):
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
	case errors.Is(err, ErrSessionNotFound):
		writeError(w, http.StatusNotFound, protocol.ErrorCodeNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		writeError(w, http.StatusConflict, protocol.ErrorCodeConflict, err.Error())
	case errors.Is(err, ErrIntegrity):
		writeError(w, http.StatusUnprocessableEntity, protocol.ErrorCodeIntegrity, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, protocol.ErrorCodeIO, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code protocol.ErrorCode, message string) {
	writeJSON(w, status, protocol.ErrorResponse{Code: code, Message: message})
}
