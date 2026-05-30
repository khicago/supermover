package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pairserve"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/receiverserve"
)

type serveReadyEvidence struct {
	Address          string                            `json:"address"`
	VerificationCode string                            `json:"verification_code,omitempty"`
	OperatorToken    string                            `json:"operator_token,omitempty"`
	Mode             string                            `json:"mode"`
	ReceiverAddress  string                            `json:"receiver_address,omitempty"`
	ReceiverRoutes   bool                              `json:"receiver_routes,omitempty"`
	PushNetwork      bool                              `json:"push_network,omitempty"`
	Trusted          bool                              `json:"trusted"`
	Transfer         bool                              `json:"transfer"`
	ExpiresAt        string                            `json:"expires_at,omitempty"`
	PairingRequest   *pairserve.PairingRequestSnapshot `json:"pairing_request,omitempty"`
}

type serveReadyState struct {
	mu         sync.Mutex
	path       string
	writer     func(string, serveReadyEvidence) error
	evidence   serveReadyEvidence
	hasPairing bool
}

func (r Runner) runServe(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("serve", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path; unpaired profiles serve pairing only, paired receiver material must be complete")
	listen := fs.String("listen", pairserve.DefaultListen, "--listen pairing listen address; receiver address comes from profile network.receiver_url")
	readyFile := fs.String("ready-file", "", "--ready-file optional path for structured serve readiness evidence")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "serve: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "serve: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if strings.TrimSpace(*listen) == "" {
		fmt.Fprintln(stderr, "serve: --listen is required")
		return 2
	}
	if err := preflightServeReadyFile(*readyFile); err != nil {
		fmt.Fprintf(stderr, "serve: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 2
	}
	if _, err := targetDirFromProfile(p); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 2
	}
	enableReceiver := false
	if serveReceiverMaterialPresent(p) && profileHasPairingPins(p) {
		if _, err := pairing.ValidateProfileTrust(p); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		if err := p.ValidateNetworkServerMaterial(); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		enableReceiver = true
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	serveCtx, stopServe := context.WithCancel(ctx)
	defer stopServe()
	var outputMu sync.Mutex
	readyState := newServeReadyState(*readyFile)
	pairingServer, err := r.newPairingServe(p, *listen, stderr, &outputMu, enableReceiver, readyState)
	if err != nil {
		if errors.Is(err, pairserve.ErrInvalidOptions) {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	pairingListener, err := pairingServer.Listen()
	if err != nil {
		fmt.Fprintf(stderr, "serve: pairing: %v\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	defer pairingListener.Close()
	var receiverServer *receiverserve.Server
	var receiverListener net.Listener
	if enableReceiver {
		receiver, err := r.newReceiverServe(p, stderr, &outputMu, readyState)
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		listener := r.receiverListenerForTest
		if listener == nil {
			var err error
			openedListener, err := receiver.Listen()
			if err != nil {
				fmt.Fprintf(stderr, "serve: receiver: %v\n", safeDiagnosticLine(err.Error()))
				return 1
			}
			listener = openedListener
		}
		defer listener.Close()
		receiverServer = receiver
		receiverListener = listener
	}
	serverCount := 1
	if enableReceiver {
		serverCount = 2
	}
	errCh := make(chan serveResult, serverCount)
	if receiverServer != nil {
		go func() {
			errCh <- serveResult{name: "receiver", err: receiverServer.Serve(serveCtx, receiverListener)}
		}()
	}
	go func() {
		errCh <- serveResult{name: "pairing", err: pairingServer.Serve(serveCtx, pairingListener)}
	}()
	var firstErr serveResult
	for completed := 0; completed < serverCount; completed++ {
		result := <-errCh
		if result.err != nil && firstErr.err == nil {
			firstErr = result
			stopServe()
		}
	}
	if firstErr.err != nil {
		if firstErr.name == "pairing" && errors.Is(firstErr.err, pairserve.ErrInvalidOptions) {
			fmt.Fprintf(stderr, "serve: %v\n", firstErr.err)
			return 2
		}
		fmt.Fprintf(stderr, "serve: %s: %v\n", firstErr.name, firstErr.err)
		return 1
	}
	return 0
}

func preflightServeReadyFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("serve ready-file directory %q is a symlink", parent)
	}
	if !info.IsDir() {
		return fmt.Errorf("serve ready-file directory %q is not a directory", parent)
	}
	if existing, err := os.Lstat(path); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("serve ready-file %q is a symlink", path)
		}
		if existing.IsDir() {
			return fmt.Errorf("serve ready-file %q is a directory", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func newServeReadyState(path string) *serveReadyState {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	return &serveReadyState{
		path:   path,
		writer: writeServeReadyFile,
	}
}

func writeServeReadyFile(path string, doc serveReadyEvidence) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	parent := filepath.Dir(path)
	temp, err := os.CreateTemp(parent, ".serve-ready-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := durable.PromoteFile(tempPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func (s *serveReadyState) recordPairing(update serveReadyEvidence) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evidence.Address = update.Address
	s.evidence.VerificationCode = update.VerificationCode
	s.evidence.OperatorToken = update.OperatorToken
	s.evidence.Mode = update.Mode
	s.evidence.ExpiresAt = update.ExpiresAt
	if !s.evidence.ReceiverRoutes {
		s.evidence.Trusted = false
		s.evidence.Transfer = false
	}
	s.hasPairing = true
	s.writeLocked()
}

func (s *serveReadyState) recordPairingRequest(update pairserve.PairingRequestSnapshot) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evidence.PairingRequest = &update
	s.writeLocked()
}

func (s *serveReadyState) recordReceiver(update serveReadyEvidence) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if update.Address != "" {
		s.evidence.ReceiverAddress = update.Address
	}
	if update.ReceiverAddress != "" {
		s.evidence.ReceiverAddress = update.ReceiverAddress
	}
	s.evidence.ReceiverRoutes = update.ReceiverRoutes
	s.evidence.PushNetwork = update.PushNetwork
	s.evidence.Trusted = update.Trusted
	s.evidence.Transfer = update.Transfer
	if s.evidence.Mode == "" {
		s.evidence.Mode = "pairing"
	}
	if !s.hasPairing {
		return
	}
	s.writeLocked()
}

func (s *serveReadyState) writeLocked() {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return
	}
	writer := s.writer
	if writer == nil {
		writer = writeServeReadyFile
	}
	_ = writer(s.path, s.evidence)
}

func (r Runner) newPairingServe(p profile.Profile, listen string, stderr io.Writer, outputMu *sync.Mutex, receiverEnabled bool, readyState *serveReadyState) (*pairserve.Server, error) {
	return pairserve.New(pairserve.Options{
		Profile: p,
		Listen:  listen,
		Ready: func(info pairserve.ReadyInfo) {
			mode := "pairing-only"
			if receiverEnabled {
				mode = "pairing"
			}
			outputMu.Lock()
			fmt.Fprintf(stderr, "serve: listening address=%s mode=%s verification_code=%s expires_at=%s trusted=false transfer=false\n", info.Address, mode, info.VerificationCode, info.ExpiresAt.Format(time.RFC3339Nano))
			outputMu.Unlock()
			if readyState != nil {
				readyState.recordPairing(serveReadyEvidence{
					Address:          info.Address,
					VerificationCode: info.VerificationCode,
					OperatorToken:    info.OperatorToken,
					Mode:             mode,
					Trusted:          false,
					Transfer:         false,
					ExpiresAt:        info.ExpiresAt.Format(time.RFC3339Nano),
				})
			}
			if r.ServePairingReady != nil {
				r.ServePairingReady(info)
			}
			if r.ServeReady != nil {
				r.ServeReady(info.Address)
			}
		},
		RequestChanged: func(request pairserve.PairingRequestSnapshot) {
			outputMu.Lock()
			fmt.Fprintf(stderr, "serve: pairing request id=%s status=%s trusted=false transfer=false\n", request.ID, request.Status)
			outputMu.Unlock()
			if readyState != nil {
				readyState.recordPairingRequest(request)
			}
			if r.ServePairingRequestChanged != nil {
				r.ServePairingRequestChanged(request)
			}
		},
	})
}

func (r Runner) newReceiverServe(p profile.Profile, stderr io.Writer, outputMu *sync.Mutex, readyState *serveReadyState) (*receiverserve.Server, error) {
	return receiverserve.New(receiverserve.Options{
		Profile: p,
		Now: func() time.Time {
			if r.Now.IsZero() {
				return time.Now()
			}
			return r.Now
		},
		Ready: func(info receiverserve.ReadyInfo) {
			outputMu.Lock()
			fmt.Fprintf(stderr, "serve: receiver listening address=%s mode=receiver-tls trusted=true receiver_routes=true push_network=true\n", info.Address)
			outputMu.Unlock()
			if readyState != nil {
				readyState.recordReceiver(serveReadyEvidence{
					Address:         info.Address,
					Mode:            "receiver-tls",
					ReceiverAddress: info.Address,
					ReceiverRoutes:  true,
					PushNetwork:     true,
					Trusted:         true,
					Transfer:        true,
				})
			}
			if r.ServeReceiverReady != nil {
				r.ServeReceiverReady(info)
			}
		},
	})
}
