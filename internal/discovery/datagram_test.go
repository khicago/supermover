package discovery

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestEncodeSparseAdvertisementRejectsExtraTXT(t *testing.T) {
	ad := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair"})
	ad.UnauthenticatedTXT = map[string]string{"svc": "_supermover._tcp"}

	_, err := EncodeSparseAdvertisement(ad)
	if !errors.Is(err, ErrInvalidAdvertisement) {
		t.Fatalf("EncodeSparseAdvertisement() error = %v, want ErrInvalidAdvertisement", err)
	}
}

func TestDecodeSparseAdvertisementRejectsUnknownHighInfoFields(t *testing.T) {
	payload := []byte(`{"service_type":"_supermover._tcp","protocol_version":"supermover/1","ephemeral_nonce":"abcdef0123456789","capability_flags":["pair"],"hostname":"alice-mbp.local"}`)

	_, err := DecodeSparseAdvertisement(payload)
	if !errors.Is(err, ErrInvalidAdvertisement) {
		t.Fatalf("DecodeSparseAdvertisement() error = %v, want ErrInvalidAdvertisement", err)
	}
}

func TestDecodeSparseAdvertisementRejectsOversizedPayload(t *testing.T) {
	payload := bytes.Repeat([]byte(" "), DefaultMaxAdvertisementBytes+1)

	_, err := DecodeSparseAdvertisement(payload)

	if !errors.Is(err, ErrInvalidAdvertisement) {
		t.Fatalf("DecodeSparseAdvertisement(oversized) error = %v, want ErrInvalidAdvertisement", err)
	}
}

func TestDatagramAdvertiseBrowseLoopback(t *testing.T) {
	now := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	advertiseConn := listenUDP4(t)
	defer advertiseConn.Close()
	browseConn := listenUDP4(t)
	defer browseConn.Close()

	ad := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair", "l2"})
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- DatagramAdvertiser{
			Conn:          advertiseConn,
			Destination:   browseConn.LocalAddr(),
			Advertisement: ad,
			Interval:      5 * time.Millisecond,
		}.Advertise(ctx)
	}()

	candidates, err := Browse(ctx, DatagramSource{
		Conn:            browseConn,
		ServiceType:     "_supermover._tcp",
		ProtocolVersion: "supermover/1",
		TTL:             time.Minute,
	}, now)
	if err != nil {
		t.Fatalf("Browse(datagram) error = %v, want nil", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("Browse(datagram) len = %d, want 1: %#v", len(candidates), candidates)
	}
	candidate := candidates[0]
	if candidate.Hint.Trusted {
		t.Fatalf("Browse(datagram) candidate = %+v, must remain untrusted", candidate)
	}
	if candidate.Class != CandidateClassDuplicate {
		t.Fatalf("Browse(datagram) class = %q, want duplicate from repeated advertisements", candidate.Class)
	}
	if candidate.Hint.Address != advertiseConn.LocalAddr().String() {
		t.Fatalf("Browse(datagram) address = %q, want packet source %q", candidate.Hint.Address, advertiseConn.LocalAddr().String())
	}
	payload, err := EncodeSparseAdvertisement(candidate.Hint.Advertisement)
	if err != nil {
		t.Fatalf("EncodeSparseAdvertisement() error = %v, want nil", err)
	}
	for _, forbidden := range []string{"hostname", "username", "path", "inventory", "file_count"} {
		if bytes.Contains(payload, []byte(forbidden)) {
			t.Fatalf("sparse payload = %s, must not contain %q", payload, forbidden)
		}
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("DatagramAdvertiser.Advertise() error = %v, want nil", err)
	}
}

func TestDatagramSourceHonorsContextWithoutPackets(t *testing.T) {
	conn := listenUDP4(t)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	got, err := Browse(ctx, DatagramSource{Conn: conn}, time.Now())
	if err != nil {
		t.Fatalf("Browse(datagram empty) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("Browse(datagram empty) = %#v, want no candidates", got)
	}
}

func TestDatagramSourceDropsHighInfoPacketByDefault(t *testing.T) {
	advertiseConn := listenUDP4(t)
	defer advertiseConn.Close()
	browseConn := listenUDP4(t)
	defer browseConn.Close()

	payload := []byte(`{"service_type":"_supermover._tcp","protocol_version":"supermover/1","ephemeral_nonce":"abcdef0123456789","capability_flags":["pair"],"path":"home/sample-user"}`)
	if _, err := advertiseConn.WriteTo(payload, browseConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo() error = %v, want nil", err)
	}
	valid, err := EncodeSparseAdvertisement(NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair"}))
	if err != nil {
		t.Fatalf("EncodeSparseAdvertisement() error = %v, want nil", err)
	}
	if _, err := advertiseConn.WriteTo(valid, browseConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(valid) error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	invalidPackets := 0

	got, err := Browse(ctx, DatagramSource{Conn: browseConn, InvalidPackets: &invalidPackets}, time.Now())
	if err != nil {
		t.Fatalf("Browse(high-info datagram default) error = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("Browse(high-info datagram default) len = %d, want one valid candidate: %#v", len(got), got)
	}
	if invalidPackets != 1 {
		t.Fatalf("invalid packet count = %d, want 1", invalidPackets)
	}
}

func TestDatagramSourceStrictRejectsHighInfoPacket(t *testing.T) {
	advertiseConn := listenUDP4(t)
	defer advertiseConn.Close()
	browseConn := listenUDP4(t)
	defer browseConn.Close()

	payload := []byte(`{"service_type":"_supermover._tcp","protocol_version":"supermover/1","ephemeral_nonce":"abcdef0123456789","capability_flags":["pair"],"path":"home/sample-user"}`)
	if _, err := advertiseConn.WriteTo(payload, browseConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo() error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := Browse(ctx, DatagramSource{Conn: browseConn, Strict: true}, time.Now())
	if !errors.Is(err, ErrInvalidAdvertisement) {
		t.Fatalf("Browse(high-info datagram strict) error = %v, want ErrInvalidAdvertisement", err)
	}
}

func TestDatagramSourceFiltersServiceAndProtocol(t *testing.T) {
	advertiseConn := listenUDP4(t)
	defer advertiseConn.Close()
	browseConn := listenUDP4(t)
	defer browseConn.Close()
	ad := NewLowInfoAdvertisement("_other._tcp", "supermover/1", "abcdef0123456789", []string{"pair"})
	payload, err := EncodeSparseAdvertisement(ad)
	if err != nil {
		t.Fatalf("EncodeSparseAdvertisement() error = %v, want nil", err)
	}
	if _, err := advertiseConn.WriteTo(payload, browseConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo() error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	got, err := Browse(ctx, DatagramSource{Conn: browseConn, ServiceType: "_supermover._tcp", ProtocolVersion: "supermover/1"}, time.Now())
	if err != nil {
		t.Fatalf("Browse(filtered datagram) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("Browse(filtered datagram) = %#v, want filtered out", got)
	}
}

func listenUDP4(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("ListenUDP() error = %v", err)
	}
	if strings.TrimSpace(conn.LocalAddr().String()) == "" {
		t.Fatalf("ListenUDP() local address is empty")
	}
	return conn
}
