package receiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/khicago/supermover/internal/protocol"
)

const maxRequestBodyBytes = protocol.MaxChunkBytes + 128*1024

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
	case r.Method == http.MethodPost && r.URL.Path == "/v1/commit":
		h.handleCommit(w, r)
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
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.Store.AppendChunk(req)
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

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	defer r.Body.Close()
	body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("request body must contain a single JSON document")
		}
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, protocol.ErrValidation):
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
