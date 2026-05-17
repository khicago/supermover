package receiverserve

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/receiver"
	"github.com/khicago/supermover/internal/tlsidentity"
	"github.com/khicago/supermover/internal/transport"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 2 * time.Second
)

type Options struct {
	Profile profile.Profile
	Now     func() time.Time
	Ready   func(ReadyInfo)
}

type ReadyInfo struct {
	Address string
	Peer    transport.AuthenticatedPeer
}

type Server struct {
	address   string
	handler   http.Handler
	tlsConfig *tls.Config
	peer      transport.AuthenticatedPeer
	ready     func(ReadyInfo)
}

func New(opts Options) (*Server, error) {
	if err := opts.Profile.ValidateNetworkServerMaterial(); err != nil {
		return nil, fmt.Errorf("profile network receiver material: %w", err)
	}
	address, err := listenAddress(opts.Profile)
	if err != nil {
		return nil, err
	}
	identity := opts.Profile.Network.LocalTLSIdentity
	certificate, err := tlsidentity.Load(identity)
	if err != nil {
		return nil, fmt.Errorf("load local TLS identity: %w", err)
	}
	receiverTLS, err := receiver.NewTLSReceiverFromProfile(receiver.TLSReceiverOptions{
		Profile:      opts.Profile,
		Certificates: []tls.Certificate{certificate},
		Now:          opts.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("build TLS receiver: %w", err)
	}
	return &Server{
		address:   address,
		handler:   receiverTLS.Handler,
		tlsConfig: receiverTLS.TLSConfig,
		peer:      receiverTLS.Peer,
		ready:     opts.Ready,
	}, nil
}

func ListenAndServe(ctx context.Context, opts Options) error {
	server, err := New(opts)
	if err != nil {
		return err
	}
	return server.ListenAndServe(ctx)
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	listener, err := s.Listen()
	if err != nil {
		return err
	}
	return s.Serve(ctx, listener)
}

func (s *Server) Listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return nil, fmt.Errorf("listen %q: %w", s.address, err)
	}
	return listener, nil
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		ctx = context.Background()
	}
	tlsListener := tls.NewListener(listener, s.tlsConfig)
	httpServer := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(tlsListener)
	}()
	if s.ready != nil {
		s.ready(ReadyInfo{
			Address: listener.Addr().String(),
			Peer:    s.peer,
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

func listenAddress(p profile.Profile) (string, error) {
	if p.Network == nil || strings.TrimSpace(p.Network.ReceiverURL) == "" {
		return "", errors.New("network.receiver_url is required for network receiver serve")
	}
	parsed, err := url.Parse(p.Network.ReceiverURL)
	if err != nil {
		return "", fmt.Errorf("parse network.receiver_url: %w", err)
	}
	if parsed.Host == "" {
		return "", errors.New("network.receiver_url host is required for network receiver serve")
	}
	return parsed.Host, nil
}
