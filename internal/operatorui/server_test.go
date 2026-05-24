package operatorui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
)

const testAccessToken = "operator-dashboard-test-token"

func TestNewRejectsNonLoopbackListenAddress(t *testing.T) {
	p := dashboardProfile(t)
	if _, err := New(Options{Profile: p, Listen: "0.0.0.0:8787"}); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("New(non-loopback) error = %v, want loopback refusal", err)
	}
}

func TestHandlerReportsLatestPublishedSnapshotAlignment(t *testing.T) {
	p := dashboardProfile(t)
	target := p.Target.LocalPath
	writeTargetFile(t, target, "docs/readme.txt", []byte("matching"))
	writePublishedManifest(t, target, "session-ok", []control.ManifestEntry{
		{Path: "docs", TargetPath: "docs", Kind: "dir"},
		{Path: "docs/readme.txt", TargetPath: "docs/readme.txt", Kind: "file", Size: 8, Digest: digest([]byte("matching"))},
	})

	handler, err := newHandler(Options{Profile: p, Now: fixedNow}, testAccessToken)
	if err != nil {
		t.Fatalf("newHandler() error = %v, want nil", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/integrity?token="+testAccessToken, nil)
	req.Header.Set(integrityRequestHeader, "1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/integrity status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	var report IntegrityReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v, want nil", err)
	}
	if report.Status != StatusAligned || !report.ReadOnly || report.Verification.Manifest.SessionID != "session-ok" {
		t.Fatalf("GET /api/integrity report = %+v, want aligned latest published snapshot", report)
	}
	if report.LiveExtraPaths.SessionID != "session-ok" {
		t.Fatalf("GET /api/integrity live extra-path session = %q, want selected verification session", report.LiveExtraPaths.SessionID)
	}
	if report.LiveExtraPaths.Summary.TargetDrifts != 0 {
		t.Fatalf("GET /api/integrity live extra paths = %+v, want none", report.LiveExtraPaths.Summary)
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/?token="+testAccessToken, nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Run full verification") {
		t.Fatalf("GET / status/body = %d/%q, want dashboard page", page.Code, page.Body.String())
	}
	if got := page.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") || !strings.Contains(got, "connect-src 'self'") {
		t.Fatalf("GET / CSP = %q, want restrictive local-resource policy", got)
	}
	if got := page.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("GET / Cache-Control = %q, want no-store", got)
	}
	blockedPage := httptest.NewRecorder()
	handler.ServeHTTP(blockedPage, httptest.NewRequest(http.MethodGet, "/", nil))
	if blockedPage.Code != http.StatusForbidden {
		t.Fatalf("GET / without dashboard token status = %d, want 403", blockedPage.Code)
	}
	blocked := httptest.NewRecorder()
	handler.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/api/integrity?token="+testAccessToken, nil))
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("GET /api/integrity without dashboard header status = %d, want 403", blocked.Code)
	}
	blocked = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/integrity", nil)
	req.Header.Set(integrityRequestHeader, "1")
	handler.ServeHTTP(blocked, req)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("GET /api/integrity without dashboard token status = %d, want 403", blocked.Code)
	}
}

func TestHandlerReportsExtraTargetPathAsReviewRequired(t *testing.T) {
	p := dashboardProfile(t)
	target := p.Target.LocalPath
	writeTargetFile(t, target, "keep.txt", []byte("keep"))
	writeTargetFile(t, target, "extra.txt", []byte("extra"))
	writePublishedManifest(t, target, "session-extra", []control.ManifestEntry{
		{Path: "keep.txt", TargetPath: "keep.txt", Kind: "file", Size: 4, Digest: digest([]byte("keep"))},
	})

	handler, err := newHandler(Options{Profile: p, Now: fixedNow}, testAccessToken)
	if err != nil {
		t.Fatalf("newHandler() error = %v, want nil", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/integrity?token="+testAccessToken, nil)
	req.Header.Set(integrityRequestHeader, "1")
	handler.ServeHTTP(rec, req)
	var report IntegrityReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v, want nil", err)
	}
	if report.Status != StatusReviewRequired || report.LiveExtraPaths.Summary.TargetDrifts != 1 {
		t.Fatalf("GET /api/integrity report = %+v, want one extra-path review finding", report)
	}
	if report.LiveExtraPaths.Drifts[0].Path != "extra.txt" || report.LiveExtraPaths.Drifts[0].Change != "extra" {
		t.Fatalf("GET /api/integrity extra paths = %+v, want extra.txt extra", report.LiveExtraPaths.Drifts)
	}
}

func TestHandlerRefusesConcurrentFullVerification(t *testing.T) {
	raw, err := newHandler(Options{Profile: dashboardProfile(t), Now: fixedNow}, testAccessToken)
	if err != nil {
		t.Fatalf("newHandler() error = %v, want nil", err)
	}
	handler := raw.(*handler)
	if !handler.startCheck() {
		t.Fatal("startCheck() = false, want initial request permitted")
	}
	defer handler.finishCheck()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/integrity?token="+testAccessToken, nil)
	req.Header.Set(integrityRequestHeader, "1")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "already running") {
		t.Fatalf("concurrent GET /api/integrity status/body = %d/%q, want busy refusal", rec.Code, rec.Body.String())
	}
}

func dashboardProfile(t *testing.T) profile.Profile {
	t.Helper()
	source := t.TempDir()
	target := t.TempDir()
	return profile.NewDefault("profile-local", "operator dashboard", source, target)
}

func writeTargetFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writePublishedManifest(t *testing.T, target, sessionID string, entries []control.ManifestEntry) {
	t.Helper()
	manifestPath, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v", err)
	}
	if err := control.WriteFile(manifestPath, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   entries,
	}); err != nil {
		t.Fatalf("control.WriteFile(manifest) error = %v", err)
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v", err)
	}
	if err := control.WriteFile(receiptPath, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: "profile-local",
		TargetID:  "local:profile-local",
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	}); err != nil {
		t.Fatalf("control.WriteFile(receipt) error = %v", err)
	}
}

func digest(data []byte) string {
	return "sha256:" + sha256Hex(data)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
}
