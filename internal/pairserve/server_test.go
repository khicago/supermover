package pairserve

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
)

func TestNewValidatesTargetRootWithoutMutatingControlPlane(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	_, err := New(Options{
		Profile: p,
		Listen:  DefaultListen,
		Nonce:   "abcdef0123456789",
	})
	if err != nil {
		t.Fatalf("New(valid options) error = %v, want nil", err)
	}
	if _, err := os.Lstat(filepath.Join(target, pathguard.ReservedControlDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("New(valid options) .supermover state error = %v, want not exist", err)
	}
}

func TestNewRejectsUnsafeTargetRoot(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, source string) string
		want  string
	}{
		{
			name: "missing",
			setup: func(t *testing.T, source string) string {
				return filepath.Join(t.TempDir(), "missing")
			},
			want: "no such file",
		},
		{
			name: "file",
			setup: func(t *testing.T, source string) string {
				path := filepath.Join(t.TempDir(), "target-file")
				if err := os.WriteFile(path, []byte("file"), 0o644); err != nil {
					t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
				}
				return path
			},
			want: "not a directory",
		},
		{
			name: "symlink root",
			setup: func(t *testing.T, source string) string {
				realRoot := t.TempDir()
				linkRoot := filepath.Join(t.TempDir(), "target-link")
				if err := os.Symlink(realRoot, linkRoot); err != nil {
					t.Skipf("os.Symlink() unavailable: %v", err)
				}
				return linkRoot
			},
			want: "symlink",
		},
		{
			name: "symlink control plane",
			setup: func(t *testing.T, source string) string {
				target := t.TempDir()
				if err := os.Symlink(t.TempDir(), filepath.Join(target, pathguard.ReservedControlDir)); err != nil {
					t.Skipf("os.Symlink() unavailable: %v", err)
				}
				return target
			},
			want: "symlink",
		},
		{
			name: "reserved root",
			setup: func(t *testing.T, source string) string {
				target := filepath.Join(t.TempDir(), pathguard.ReservedControlDir)
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatalf("os.Mkdir(%q) error = %v, want nil", target, err)
				}
				return target
			},
			want: "reserved control directory",
		},
		{
			name: "below reserved root",
			setup: func(t *testing.T, source string) string {
				target := filepath.Join(t.TempDir(), pathguard.ReservedControlDir, "nested-target")
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatalf("os.MkdirAll(%q) error = %v, want nil", target, err)
				}
				return target
			},
			want: "reserved control directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := t.TempDir()
			p := profile.NewDefault("profile-local", "Local profile", source, tt.setup(t, source))

			_, err := New(Options{Profile: p, Listen: DefaultListen, Nonce: "abcdef0123456789"})
			if err == nil {
				t.Fatalf("New(%s target root) error = nil, want error", tt.name)
			}
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("New(%s target root) error = %v, want ErrInvalidOptions", tt.name, err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("New(%s target root) error = %v, want %q", tt.name, err, tt.want)
			}
		})
	}
}

func TestHandlerExposesOnlyLowInfoDiscoveryAndPairing(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Secret Profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	server, err := New(Options{Profile: p, Listen: DefaultListen, Nonce: "abcdef0123456789"})
	if err != nil {
		t.Fatalf("New(valid options) error = %v, want nil", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/v1/discovery")
	if err != nil {
		t.Fatalf("GET /v1/discovery error = %v, want nil", err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/discovery status = %d body = %q, want 200", resp.StatusCode, body)
	}
	assertPairserveLowInfoResponse(t, body, p, source, target)
	if !strings.Contains(body, `"trusted":false`) {
		t.Fatalf("GET /v1/discovery body = %q, want trusted=false", body)
	}

	resp, err = http.Get(httpServer.URL + "/v1/pairing")
	if err != nil {
		t.Fatalf("GET /v1/pairing without code error = %v, want nil", err)
	}
	body = readBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET /v1/pairing without code status = %d body = %q, want 403", resp.StatusCode, body)
	}
	assertLowInfoResponse(t, body, p, source, target)
	if strings.Contains(body, p.Target.DevicePublicKey) {
		t.Fatalf("GET /v1/pairing without code body = %q, must not expose pinned device identity", body)
	}

	req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/v1/pairing", nil)
	if err != nil {
		t.Fatalf("NewRequest(/v1/pairing) error = %v, want nil", err)
	}
	req.Header.Set(pairing.VerificationCodeHeader, server.pairingCode)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/pairing with code error = %v, want nil", err)
	}
	body = readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/pairing with code status = %d body = %q, want 200", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"trusted":false`) {
		t.Fatalf("GET /v1/pairing with code body = %q, want trusted=false", body)
	}
	if _, err := os.Lstat(filepath.Join(target, pathguard.ReservedControlDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("handler .supermover state error = %v, want not exist", err)
	}
}

func TestHandlerRefusesReceiverTransferRoutes(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	server, err := New(Options{Profile: p, Listen: DefaultListen, Nonce: "abcdef0123456789"})
	if err != nil {
		t.Fatalf("New(valid options) error = %v, want nil", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	routes := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/v1/sessions"},
		{method: http.MethodPost, path: "/v1/chunks"},
		{method: http.MethodPost, path: "/v1/chunk-batches"},
		{method: http.MethodPost, path: "/v1/commit"},
		{method: http.MethodGet, path: "/v1/sessions/session-1/status"},
	}
	for _, route := range routes {
		req, err := http.NewRequest(route.method, httpServer.URL+route.path, strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("NewRequest(%s %s) error = %v, want nil", route.method, route.path, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s error = %v, want nil", route.method, route.path, err)
		}
		body := readBody(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s status = %d body = %q, want 403", route.method, route.path, resp.StatusCode, body)
		}
		if !strings.Contains(body, "disabled") {
			t.Fatalf("%s %s body = %q, want disabled transfer error", route.method, route.path, body)
		}
	}
	if _, err := os.Lstat(filepath.Join(target, pathguard.ReservedControlDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("transfer routes .supermover state error = %v, want not exist", err)
	}
}

func TestPairingEndpointRejectsWrongVerificationCode(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	server, err := New(Options{
		Profile:     p,
		Listen:      DefaultListen,
		Nonce:       "abcdef0123456789",
		PairingCode: "correct-token",
	})
	if err != nil {
		t.Fatalf("New(valid options) error = %v, want nil", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/v1/pairing", nil)
	if err != nil {
		t.Fatalf("NewRequest(/v1/pairing) error = %v, want nil", err)
	}
	req.Header.Set(pairing.VerificationCodeHeader, "wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/pairing wrong code error = %v, want nil", err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET /v1/pairing wrong code status = %d body = %q, want 403", resp.StatusCode, body)
	}
	if strings.Contains(body, server.bootstrap.TargetDeviceID) || strings.Contains(body, server.bootstrap.VerificationHash) {
		t.Fatalf("GET /v1/pairing wrong code body = %q, must not expose bootstrap verifier material", body)
	}
}

func TestHandlerMethodNotAllowedSetsAllowHeader(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	server, err := New(Options{Profile: p, Listen: DefaultListen, Nonce: "abcdef0123456789"})
	if err != nil {
		t.Fatalf("New(valid options) error = %v, want nil", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	resp, err := http.Post(httpServer.URL+"/v1/discovery", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST /v1/discovery error = %v, want nil", err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /v1/discovery status = %d body = %q, want 405", resp.StatusCode, body)
	}
	if resp.Header.Get("Allow") != http.MethodGet {
		t.Fatalf("POST /v1/discovery Allow = %q, want GET", resp.Header.Get("Allow"))
	}
	assertLowInfoResponse(t, body, p, source, target)
}

func TestServeCancelsCleanly(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	server, err := New(Options{Profile: p, Listen: DefaultListen, Nonce: "abcdef0123456789"})
	if err != nil {
		t.Fatalf("New(valid options) error = %v, want nil", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v, want nil", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, listener)
	}()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve(cancelled) error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve(cancelled) did not return")
	}
}

func TestListenAndServeFailsOnOccupiedPort(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v, want nil", err)
	}
	defer listener.Close()

	err = ListenAndServe(context.Background(), Options{
		Profile: p,
		Listen:  listener.Addr().String(),
		Nonce:   "abcdef0123456789",
	})
	if err == nil {
		t.Fatalf("ListenAndServe(occupied port) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("ListenAndServe(occupied port) error = %v, want listen error", err)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(response body) error = %v, want nil", err)
	}
	return string(body)
}

func assertLowInfoResponse(t *testing.T, body string, p profile.Profile, source string, target string) {
	t.Helper()
	for _, forbidden := range []string{
		p.ProfileID,
		p.Name,
		p.Target.TargetID,
		p.Target.LocalPath,
		p.Target.PairingReceiptID,
		source,
		target,
		"file_count",
		"manifest",
		"inventory",
		"hostname",
	} {
		if forbidden == "" {
			continue
		}
		if strings.Contains(body, forbidden) {
			t.Fatalf("response body = %q, must not contain %q", body, forbidden)
		}
	}
}

func assertPairserveLowInfoResponse(t *testing.T, body string, p profile.Profile, source string, target string) {
	t.Helper()
	assertLowInfoResponse(t, body, p, source, target)
	if !strings.Contains(body, `"trusted":false`) {
		t.Fatalf("response body = %q, want trusted=false", body)
	}
	if strings.Contains(body, "file_count") || strings.Contains(body, "inventory") {
		t.Fatalf("response body = %q, must not expose inventory", body)
	}
}
