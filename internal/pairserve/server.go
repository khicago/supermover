package pairserve

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/khicago/supermover/internal/discovery"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transport"
)

const (
	DefaultListen = "127.0.0.1:0"
	ServiceType   = "_supermover._tcp"

	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 2 * time.Second
	defaultRequestTTL = 2 * time.Minute
	maxRequestBytes   = 16 * 1024
)

var ErrInvalidOptions = errors.New("invalid pairserve options")

const OperatorTokenHeader = "X-Supermover-Operator-Token"

type Options struct {
	Profile        profile.Profile
	Listen         string
	Nonce          string
	PairingCode    string
	ChallengeID    string
	ChallengeTTL   time.Duration
	RequestTTL     time.Duration
	OperatorToken  string
	Now            time.Time
	Ready          func(ReadyInfo)
	RequestChanged func(PairingRequestSnapshot)
}

type ReadyInfo struct {
	Address          string
	VerificationCode string
	ExpiresAt        time.Time
	TargetDeviceID   string
	OperatorToken    string
}

type Server struct {
	listen         string
	nonce          string
	bootstrap      pairing.Bootstrap
	pairingCode    string
	ready          func(ReadyInfo)
	operatorToken  string
	requestTTL     time.Duration
	requestChanged func(PairingRequestSnapshot)
	requestMu      sync.Mutex
	request        *pairingRequestState
}

type DiscoveryResponse struct {
	ProtocolVersion string                  `json:"protocol_version"`
	Advertisement   discovery.Advertisement `json:"advertisement"`
	Trusted         bool                    `json:"trusted"`
	Capabilities    []string                `json:"capabilities"`
}

type PairingRequestCreate struct {
	SourceProfileID   string `json:"source_profile_id"`
	SourceProfileName string `json:"source_profile_name,omitempty"`
	SourceDeviceID    string `json:"source_device_id"`
}

type PairingRequestSnapshot struct {
	ProtocolVersion   string `json:"protocol_version"`
	ID                string `json:"id"`
	Status            string `json:"status"`
	SourceProfileID   string `json:"source_profile_id"`
	SourceProfileName string `json:"source_profile_name,omitempty"`
	SourceDeviceID    string `json:"source_device_id"`
	RequestedAt       string `json:"requested_at"`
	ExpiresAt         string `json:"expires_at"`
	DecidedAt         string `json:"decided_at,omitempty"`
}

type PairingRequestResponse struct {
	ProtocolVersion string                 `json:"protocol_version"`
	Request         PairingRequestSnapshot `json:"request"`
	Bootstrap       *pairing.Bootstrap     `json:"bootstrap,omitempty"`
}

type pairingRequestState struct {
	snapshot  PairingRequestSnapshot
	bootstrap pairing.Bootstrap
}

func New(opts Options) (*Server, error) {
	if err := opts.Profile.Validate(); err != nil {
		return nil, fmt.Errorf("%w: profile: %v", ErrInvalidOptions, err)
	}
	if err := validateTargetRoot(opts.Profile.Target.LocalPath); err != nil {
		return nil, fmt.Errorf("%w: target.local_path: %v", ErrInvalidOptions, err)
	}
	listen := strings.TrimSpace(opts.Listen)
	if listen == "" {
		return nil, fmt.Errorf("%w: listen address is required", ErrInvalidOptions)
	}
	nonce := strings.TrimSpace(opts.Nonce)
	var err error
	if nonce == "" {
		nonce, err = randomNonce()
		if err != nil {
			return nil, fmt.Errorf("%w: generate discovery nonce: %v", ErrInvalidOptions, err)
		}
	}
	if err := discovery.NewLowInfoAdvertisement(ServiceType, protocol.Version, nonce, []string{"pair"}).Validate(); err != nil {
		return nil, fmt.Errorf("%w: discovery advertisement: %v", ErrInvalidOptions, err)
	}
	pairingCode := strings.TrimSpace(opts.PairingCode)
	if pairingCode == "" {
		pairingCode, err = pairing.NewVerificationCode()
		if err != nil {
			return nil, fmt.Errorf("%w: generate pairing code: %v", ErrInvalidOptions, err)
		}
	}
	challengeID := strings.TrimSpace(opts.ChallengeID)
	if challengeID == "" {
		challengeID, err = pairing.NewChallengeID()
		if err != nil {
			return nil, fmt.Errorf("%w: generate pairing challenge: %v", ErrInvalidOptions, err)
		}
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	targetDeviceID, transportIdentityBound, err := pairing.BootstrapTargetDeviceID(opts.Profile, now)
	if err != nil {
		return nil, fmt.Errorf("%w: target device id: %v", ErrInvalidOptions, err)
	}
	ttl := opts.ChallengeTTL
	if ttl <= 0 {
		ttl = pairing.DefaultChallengeTTL
	}
	bootstrap := pairing.Bootstrap{
		ProtocolVersion:        protocol.Version,
		Status:                 "pairing_ready",
		TargetDeviceID:         targetDeviceID,
		ChallengeID:            challengeID,
		VerificationHash:       pairing.VerificationHash(targetDeviceID, challengeID, pairingCode),
		ExpiresAt:              now.Add(ttl).UTC(),
		Trusted:                false,
		TransferEnabled:        false,
		TransportIdentityBound: transportIdentityBound,
	}
	if err := pairing.ValidateBootstrap(bootstrap, targetDeviceID, pairingCode, now); err != nil {
		return nil, fmt.Errorf("%w: pairing bootstrap: %v", ErrInvalidOptions, err)
	}
	operatorToken := strings.TrimSpace(opts.OperatorToken)
	if operatorToken == "" {
		operatorToken, err = randomOperatorToken()
		if err != nil {
			return nil, fmt.Errorf("%w: generate operator token: %v", ErrInvalidOptions, err)
		}
	}
	requestTTL := opts.RequestTTL
	if requestTTL <= 0 {
		requestTTL = defaultRequestTTL
	}
	return &Server{
		listen:         listen,
		nonce:          nonce,
		bootstrap:      bootstrap,
		pairingCode:    pairingCode,
		ready:          opts.Ready,
		operatorToken:  operatorToken,
		requestTTL:     requestTTL,
		requestChanged: opts.RequestChanged,
	}, nil
}

func ListenAndServe(ctx context.Context, opts Options) error {
	server, err := New(opts)
	if err != nil {
		return err
	}
	listener, err := server.Listen()
	if err != nil {
		return err
	}
	return server.Serve(ctx, listener)
}

func (s *Server) Listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", s.listen)
	if err != nil {
		return nil, fmt.Errorf("listen %q: %w", s.listen, err)
	}
	return listener, nil
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		ctx = context.Background()
	}
	httpServer := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()
	if s.ready != nil {
		s.ready(ReadyInfo{
			Address:          listener.Addr().String(),
			VerificationCode: s.pairingCode,
			ExpiresAt:        s.bootstrap.ExpiresAt,
			TargetDeviceID:   s.bootstrap.TargetDeviceID,
			OperatorToken:    s.operatorToken,
		})
	}
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/discovery", s.handleDiscovery)
	mux.HandleFunc("/v1/pairing", s.handlePairing)
	mux.HandleFunc("/v1/pairing/requests", s.handlePairingRequests)
	mux.HandleFunc("/v1/pairing/requests/", s.handlePairingRequest)
	mux.HandleFunc("/", handleFallback)
	return mux
}

func (s *Server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	ad := discovery.NewLowInfoAdvertisement(ServiceType, protocol.Version, s.nonce, []string{"pair"})
	writeJSON(w, http.StatusOK, DiscoveryResponse{
		ProtocolVersion: protocol.Version,
		Advertisement:   ad,
		Trusted:         false,
		Capabilities:    []string{"pair"},
	})
}

func (s *Server) handlePairing(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusConflict, protocol.ErrorCodeForbidden, "pairing request approval required")
}

func (s *Server) handlePairingRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if !equalVerificationCode(r.Header.Get(pairing.VerificationCodeHeader), s.pairingCode) {
		writeError(w, http.StatusForbidden, protocol.ErrorCodeForbidden, "verification code required")
		return
	}
	var input PairingRequestCreate
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, "invalid pairing request")
		return
	}
	snapshot, err := s.createPairingRequest(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrorCodeBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, PairingRequestResponse{
		ProtocolVersion: protocol.Version,
		Request:         snapshot,
	})
}

func (s *Server) handlePairingRequest(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/pairing/requests/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if !equalVerificationCode(r.Header.Get(pairing.VerificationCodeHeader), s.pairingCode) {
			writeError(w, http.StatusForbidden, protocol.ErrorCodeForbidden, "verification code required")
			return
		}
		snapshot, bootstrap, ok := s.pairingRequestStatus(parts[0])
		if !ok {
			writeError(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "pairing request not found")
			return
		}
		response := PairingRequestResponse{ProtocolVersion: protocol.Version, Request: snapshot}
		if snapshot.Status == "approved" {
			response.Bootstrap = &bootstrap
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	if len(parts) == 2 && parts[0] != "" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if !equalOperatorToken(r.Header.Get(OperatorTokenHeader), s.operatorToken) {
			writeError(w, http.StatusForbidden, protocol.ErrorCodeForbidden, "operator approval token required")
			return
		}
		var status string
		switch parts[1] {
		case "approve":
			status = "approved"
		case "reject":
			status = "rejected"
		default:
			writeError(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "route not found")
			return
		}
		snapshot, bootstrap, ok := s.decidePairingRequest(parts[0], status)
		if !ok {
			writeError(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "pending pairing request not found")
			return
		}
		response := PairingRequestResponse{ProtocolVersion: protocol.Version, Request: snapshot}
		if snapshot.Status == "approved" {
			response.Bootstrap = &bootstrap
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	writeError(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "route not found")
}

func equalVerificationCode(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func equalOperatorToken(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) createPairingRequest(input PairingRequestCreate) (PairingRequestSnapshot, error) {
	sourceProfileID := strings.TrimSpace(input.SourceProfileID)
	sourceDeviceID := strings.TrimSpace(input.SourceDeviceID)
	if sourceProfileID == "" {
		return PairingRequestSnapshot{}, errors.New("source_profile_id is required")
	}
	if err := transport.DeviceID(sourceDeviceID).Validate(); err != nil {
		return PairingRequestSnapshot{}, fmt.Errorf("source_device_id: %v", err)
	}
	requestID, err := pairing.NewChallengeID()
	if err != nil {
		return PairingRequestSnapshot{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.requestTTL)
	if s.bootstrap.ExpiresAt.Before(expiresAt) {
		expiresAt = s.bootstrap.ExpiresAt
	}
	snapshot := PairingRequestSnapshot{
		ProtocolVersion:   protocol.Version,
		ID:                requestID,
		Status:            "pending",
		SourceProfileID:   sourceProfileID,
		SourceProfileName: strings.TrimSpace(input.SourceProfileName),
		SourceDeviceID:    sourceDeviceID,
		RequestedAt:       now.Format(time.RFC3339Nano),
		ExpiresAt:         expiresAt.Format(time.RFC3339Nano),
	}
	s.requestMu.Lock()
	s.request = &pairingRequestState{snapshot: snapshot, bootstrap: s.bootstrap}
	s.requestMu.Unlock()
	s.notifyRequestChanged(snapshot)
	return snapshot, nil
}

func (s *Server) pairingRequestStatus(id string) (PairingRequestSnapshot, pairing.Bootstrap, bool) {
	s.requestMu.Lock()
	if s.request == nil || s.request.snapshot.ID != id {
		s.requestMu.Unlock()
		return PairingRequestSnapshot{}, pairing.Bootstrap{}, false
	}
	expired, changed := s.expireRequestLocked(time.Now().UTC())
	snapshot := s.request.snapshot
	bootstrap := s.request.bootstrap
	s.requestMu.Unlock()
	if changed {
		s.notifyRequestChanged(expired)
	}
	return snapshot, bootstrap, true
}

func (s *Server) decidePairingRequest(id string, status string) (PairingRequestSnapshot, pairing.Bootstrap, bool) {
	s.requestMu.Lock()
	if s.request == nil || s.request.snapshot.ID != id {
		s.requestMu.Unlock()
		return PairingRequestSnapshot{}, pairing.Bootstrap{}, false
	}
	expired, changed := s.expireRequestLocked(time.Now().UTC())
	if s.request.snapshot.Status != "pending" {
		s.requestMu.Unlock()
		if changed {
			s.notifyRequestChanged(expired)
		}
		return PairingRequestSnapshot{}, pairing.Bootstrap{}, false
	}
	s.request.snapshot.Status = status
	s.request.snapshot.DecidedAt = time.Now().UTC().Format(time.RFC3339Nano)
	snapshot := s.request.snapshot
	bootstrap := s.request.bootstrap
	s.requestMu.Unlock()
	if changed {
		s.notifyRequestChanged(expired)
	}
	s.notifyRequestChanged(snapshot)
	return snapshot, bootstrap, true
}

func (s *Server) expireRequestLocked(now time.Time) (PairingRequestSnapshot, bool) {
	if s.request == nil || s.request.snapshot.Status != "pending" {
		return PairingRequestSnapshot{}, false
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, s.request.snapshot.ExpiresAt)
	if err != nil || !expiresAt.After(now) {
		s.request.snapshot.Status = "expired"
		s.request.snapshot.DecidedAt = now.UTC().Format(time.RFC3339Nano)
		return s.request.snapshot, true
	}
	return PairingRequestSnapshot{}, false
}

func (s *Server) notifyRequestChanged(snapshot PairingRequestSnapshot) {
	if s.requestChanged != nil {
		s.requestChanged(snapshot)
	}
}

func handleFallback(w http.ResponseWriter, r *http.Request) {
	if receiverTransferRoute(r.URL.Path) {
		writeError(w, http.StatusForbidden, protocol.ErrorCodeForbidden, "receiver transfer endpoint disabled until paired authenticated transport is implemented")
		return
	}
	writeError(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "route not found")
}

func receiverTransferRoute(path string) bool {
	return path == "/v1/sessions" ||
		path == "/v1/chunks" ||
		path == "/v1/chunk-batches" ||
		path == "/v1/commit" ||
		(strings.HasPrefix(path, "/v1/sessions/") && strings.HasSuffix(path, "/status"))
}

func validateTargetRoot(root string) error {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "." || root == "" {
		return errors.New("target root is required")
	}
	if containsReservedControlSegment(root) {
		return fmt.Errorf("%w: target root must not be the reserved control directory", pathguard.ErrUnsafePath)
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
	if err := pathguard.EnsureDirectory(root, filepath.Join(root, pathguard.ReservedControlDir)); err != nil {
		return err
	}
	return nil
}

func containsReservedControlSegment(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.EqualFold(segment, pathguard.ReservedControlDir) {
			return true
		}
	}
	return false
}

func randomNonce() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func randomOperatorToken() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code protocol.ErrorCode, message string) {
	writeJSON(w, status, protocol.ErrorResponse{Code: code, Message: message})
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, protocol.ErrorCodeBadRequest, "method not allowed")
}
