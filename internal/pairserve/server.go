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
	"time"

	"github.com/khicago/supermover/internal/discovery"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
)

const (
	DefaultListen = "127.0.0.1:0"
	ServiceType   = "_supermover._tcp"

	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 2 * time.Second
)

var ErrInvalidOptions = errors.New("invalid pairserve options")

type Options struct {
	Profile      profile.Profile
	Listen       string
	Nonce        string
	PairingCode  string
	ChallengeID  string
	ChallengeTTL time.Duration
	Now          time.Time
	Ready        func(ReadyInfo)
}

type ReadyInfo struct {
	Address          string
	VerificationCode string
	ExpiresAt        time.Time
	TargetDeviceID   string
}

type Server struct {
	listen      string
	nonce       string
	bootstrap   pairing.Bootstrap
	pairingCode string
	ready       func(ReadyInfo)
}

type DiscoveryResponse struct {
	ProtocolVersion string                  `json:"protocol_version"`
	Advertisement   discovery.Advertisement `json:"advertisement"`
	Trusted         bool                    `json:"trusted"`
	Capabilities    []string                `json:"capabilities"`
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
	if nonce == "" {
		var err error
		nonce, err = randomNonce()
		if err != nil {
			return nil, fmt.Errorf("%w: generate discovery nonce: %v", ErrInvalidOptions, err)
		}
	}
	if err := discovery.NewLowInfoAdvertisement(ServiceType, protocol.Version, nonce, []string{"pair"}).Validate(); err != nil {
		return nil, fmt.Errorf("%w: discovery advertisement: %v", ErrInvalidOptions, err)
	}
	targetDeviceID, err := pairing.TargetDeviceID(opts.Profile)
	if err != nil {
		return nil, fmt.Errorf("%w: target device id: %v", ErrInvalidOptions, err)
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
	ttl := opts.ChallengeTTL
	if ttl <= 0 {
		ttl = pairing.DefaultChallengeTTL
	}
	bootstrap := pairing.Bootstrap{
		ProtocolVersion:  protocol.Version,
		Status:           "pairing_ready",
		TargetDeviceID:   targetDeviceID,
		ChallengeID:      challengeID,
		VerificationHash: pairing.VerificationHash(targetDeviceID, challengeID, pairingCode),
		ExpiresAt:        now.Add(ttl).UTC(),
		Trusted:          false,
		TransferEnabled:  false,
	}
	if err := pairing.ValidateBootstrap(bootstrap, targetDeviceID, pairingCode, now); err != nil {
		return nil, fmt.Errorf("%w: pairing bootstrap: %v", ErrInvalidOptions, err)
	}
	return &Server{
		listen:      listen,
		nonce:       nonce,
		bootstrap:   bootstrap,
		pairingCode: pairingCode,
		ready:       opts.Ready,
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
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if !equalVerificationCode(r.Header.Get(pairing.VerificationCodeHeader), s.pairingCode) {
		writeError(w, http.StatusForbidden, protocol.ErrorCodeForbidden, "verification code required")
		return
	}
	writeJSON(w, http.StatusOK, s.bootstrap)
}

func equalVerificationCode(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
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
