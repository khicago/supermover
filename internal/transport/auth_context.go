package transport

import (
	"context"
	"errors"
	"fmt"
)

type authenticatedPeerContextKey struct{}

type AuthenticatedPeer struct {
	ProfileID      string
	TargetID       string
	SourceDeviceID string
	TargetDeviceID string
}

func (p AuthenticatedPeer) Validate() error {
	if err := ValidateProfileID(p.ProfileID); err != nil {
		return fmt.Errorf("profile id: %w", err)
	}
	if err := ValidateProfileID(p.TargetID); err != nil {
		return fmt.Errorf("target id: %w", err)
	}
	if err := DeviceID(p.SourceDeviceID).Validate(); err != nil {
		return fmt.Errorf("source device id: %w", err)
	}
	if err := DeviceID(p.TargetDeviceID).Validate(); err != nil {
		return fmt.Errorf("target device id: %w", err)
	}
	if p.SourceDeviceID == p.TargetDeviceID {
		return errors.New("source and target device ids must differ")
	}
	return nil
}

// ContextWithAuthenticatedPeer attaches an already authenticated transport peer.
// Production HTTP adapters should prefer NewTLSAuthenticatedPeerHandler so the
// request identity is derived from r.TLS rather than caller-controlled metadata.
func ContextWithAuthenticatedPeer(ctx context.Context, peer AuthenticatedPeer) context.Context {
	return context.WithValue(ctx, authenticatedPeerContextKey{}, peer)
}

func AuthenticatedPeerFromContext(ctx context.Context) (AuthenticatedPeer, bool) {
	peer, ok := ctx.Value(authenticatedPeerContextKey{}).(AuthenticatedPeer)
	return peer, ok
}
