package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	DefaultAdvertiseInterval     = time.Second
	DefaultDatagramReadPoll      = 25 * time.Millisecond
	DefaultMaxAdvertisementBytes = 1024
)

var ErrInvalidDiscoverySource = errors.New("invalid discovery source")

type DatagramConn interface {
	io.Closer
	LocalAddr() net.Addr
	ReadFrom([]byte) (int, net.Addr, error)
	SetReadDeadline(time.Time) error
	WriteTo([]byte, net.Addr) (int, error)
}

type DatagramSource struct {
	Conn            DatagramConn
	ServiceType     string
	ProtocolVersion string
	TTL             time.Duration
	MaxPacketBytes  int
	Strict          bool
	InvalidPackets  *int
}

type DatagramAdvertiser struct {
	Conn          DatagramConn
	Destination   net.Addr
	Advertisement Advertisement
	Interval      time.Duration
}

type sparseAdvertisementWire struct {
	ServiceType     string   `json:"service_type"`
	ProtocolVersion string   `json:"protocol_version"`
	EphemeralNonce  string   `json:"ephemeral_nonce"`
	CapabilityFlags []string `json:"capability_flags"`
}

func EncodeSparseAdvertisement(ad Advertisement) ([]byte, error) {
	if len(ad.UnauthenticatedTXT) > 0 {
		return nil, fmt.Errorf("%w: lan advertisements must contain only service, protocol, nonce, and capabilities", ErrInvalidAdvertisement)
	}
	if err := ad.Validate(); err != nil {
		return nil, err
	}
	wire := sparseAdvertisementWire{
		ServiceType:     ad.ServiceType,
		ProtocolVersion: ad.ProtocolVersion,
		EphemeralNonce:  ad.EphemeralNonce,
		CapabilityFlags: sortedCopy(ad.CapabilityFlags),
	}
	payload, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if len(payload) > DefaultMaxAdvertisementBytes {
		return nil, fmt.Errorf("%w: sparse advertisement exceeds %d bytes", ErrInvalidAdvertisement, DefaultMaxAdvertisementBytes)
	}
	return payload, nil
}

func DecodeSparseAdvertisement(payload []byte) (Advertisement, error) {
	if len(payload) > DefaultMaxAdvertisementBytes {
		return Advertisement{}, fmt.Errorf("%w: sparse advertisement exceeds %d bytes", ErrInvalidAdvertisement, DefaultMaxAdvertisementBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var wire sparseAdvertisementWire
	if err := decoder.Decode(&wire); err != nil {
		return Advertisement{}, fmt.Errorf("%w: decode sparse advertisement: %v", ErrInvalidAdvertisement, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Advertisement{}, fmt.Errorf("%w: trailing sparse advertisement data", ErrInvalidAdvertisement)
	}
	ad := NewLowInfoAdvertisement(wire.ServiceType, wire.ProtocolVersion, wire.EphemeralNonce, wire.CapabilityFlags)
	if err := ad.Validate(); err != nil {
		return Advertisement{}, err
	}
	return ad, nil
}

func (s DatagramSource) Discover(ctx context.Context, now time.Time) ([]AddressHint, error) {
	if s.Conn == nil {
		return nil, fmt.Errorf("%w: nil datagram conn", ErrInvalidDiscoverySource)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = DefaultHintTTL
	}
	maxPacketBytes := s.MaxPacketBytes
	if maxPacketBytes <= 0 {
		maxPacketBytes = DefaultMaxAdvertisementBytes
	}
	buf := make([]byte, maxPacketBytes)
	var hints []AddressHint
	for {
		if err := ctx.Err(); err != nil {
			return hints, nil
		}
		if err := s.Conn.SetReadDeadline(readDeadline(ctx, DefaultDatagramReadPoll)); err != nil {
			return nil, err
		}
		n, addr, err := s.Conn.ReadFrom(buf)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return hints, nil
			}
			return nil, err
		}
		ad, err := DecodeSparseAdvertisement(buf[:n])
		if err != nil {
			if s.Strict {
				return nil, err
			}
			if s.InvalidPackets != nil {
				(*s.InvalidPackets)++
			}
			continue
		}
		if s.ServiceType != "" && ad.ServiceType != s.ServiceType {
			continue
		}
		if s.ProtocolVersion != "" && ad.ProtocolVersion != s.ProtocolVersion {
			continue
		}
		address, err := packetAddress(addr)
		if err != nil {
			return nil, err
		}
		hint, err := NewAddressHint(address, ad, now, ttl)
		if err != nil {
			return nil, err
		}
		hints = append(hints, hint)
	}
}

func (a DatagramAdvertiser) Advertise(ctx context.Context) error {
	if a.Conn == nil {
		return fmt.Errorf("%w: nil datagram conn", ErrInvalidDiscoverySource)
	}
	if a.Destination == nil {
		return fmt.Errorf("%w: nil datagram destination", ErrInvalidDiscoverySource)
	}
	payload, err := EncodeSparseAdvertisement(a.Advertisement)
	if err != nil {
		return err
	}
	interval := a.Interval
	if interval <= 0 {
		interval = DefaultAdvertiseInterval
	}
	if err := writeAdvertisement(ctx, a.Conn, a.Destination, payload); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := writeAdvertisement(ctx, a.Conn, a.Destination, payload); err != nil {
				return err
			}
		}
	}
}

func writeAdvertisement(ctx context.Context, conn DatagramConn, destination net.Addr, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return nil
	}
	n, err := conn.WriteTo(payload, destination)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
	if n != len(payload) {
		return io.ErrShortWrite
	}
	return nil
}

func readDeadline(ctx context.Context, poll time.Duration) time.Time {
	deadline := time.Now().Add(poll)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func packetAddress(addr net.Addr) (string, error) {
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		if udpAddr.IP == nil || udpAddr.Port <= 0 || udpAddr.Port > 65535 {
			return "", fmt.Errorf("%w: invalid packet address", ErrInvalidDiscoverySource)
		}
		return net.JoinHostPort(udpAddr.IP.String(), fmt.Sprintf("%d", udpAddr.Port)), nil
	}
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "", fmt.Errorf("%w: invalid packet address: %v", ErrInvalidDiscoverySource, err)
	}
	return net.JoinHostPort(host, port), nil
}
