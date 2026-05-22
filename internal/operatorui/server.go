package operatorui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/verify"
)

const (
	DefaultListen = "127.0.0.1:8787"
	SchemaV1      = "supermover.operator_integrity.v1"

	StatusAligned        = "aligned"
	StatusReviewRequired = "review_required"

	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 2 * time.Second

	integrityRequestHeader = "X-Supermover-Dashboard"
)

type Options struct {
	Profile profile.Profile
	Listen  string
	Now     func() time.Time
	Ready   func(ReadyInfo)
}

type ReadyInfo struct {
	Address string
	URL     string
}

type IntegrityReport struct {
	Schema         string             `json:"schema"`
	CheckedAt      string             `json:"checked_at"`
	Status         string             `json:"status"`
	Assessment     string             `json:"assessment"`
	ReadOnly       bool               `json:"read_only"`
	Scope          Scope              `json:"scope"`
	Verification   verify.Report      `json:"verification"`
	LiveExtraPaths verify.DriftReport `json:"live_extra_paths"`
}

type Scope struct {
	ProfileID  string `json:"profile_id"`
	TargetID   string `json:"target_id"`
	TargetRoot string `json:"target_root"`
	SessionID  string `json:"session_id,omitempty"`
}

type Server struct {
	listen      string
	accessToken string
	handler     http.Handler
	ready       func(ReadyInfo)
}

func New(opts Options) (*Server, error) {
	listen := strings.TrimSpace(opts.Listen)
	if listen == "" {
		listen = DefaultListen
	}
	if err := validateLoopbackListen(listen); err != nil {
		return nil, err
	}
	accessToken, err := newAccessToken()
	if err != nil {
		return nil, err
	}
	handler, err := newHandler(opts, accessToken)
	if err != nil {
		return nil, err
	}
	return &Server{listen: listen, accessToken: accessToken, handler: handler, ready: opts.Ready}, nil
}

func newHandler(opts Options, accessToken string) (http.Handler, error) {
	targetRoot := strings.TrimSpace(opts.Profile.Target.LocalPath)
	if targetRoot == "" {
		return nil, errors.New("target.local_path is required for operator dashboard")
	}
	if err := control.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return nil, err
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &handler{
		profile:     opts.Profile,
		targetRoot:  targetRoot,
		now:         now,
		accessToken: accessToken,
	}, nil
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.listen)
	if err != nil {
		return fmt.Errorf("listen %q: %w", s.listen, err)
	}
	defer listener.Close()
	return s.Serve(ctx, listener)
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		ctx = context.Background()
	}
	httpServer := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ErrorLog:          log.New(httpErrorSink{}, "", 0),
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()
	if s.ready != nil {
		address := listener.Addr().String()
		s.ready(ReadyInfo{
			Address: address,
			URL:     "http://" + address + "/?token=" + url.QueryEscape(s.accessToken),
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

type handler struct {
	profile     profile.Profile
	targetRoot  string
	now         func() time.Time
	accessToken string

	mu       sync.Mutex
	checking bool
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/":
		if !h.authorized(r) {
			http.Error(w, "dashboard access token is required", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	case "/assets/app.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write([]byte(appJS))
	case "/assets/app.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(appCSS))
	case "/api/integrity":
		if !h.authorized(r) || r.Header.Get(integrityRequestHeader) != "1" {
			http.Error(w, "dashboard request header is required", http.StatusForbidden)
			return
		}
		h.handleIntegrity(w)
	default:
		http.NotFound(w, r)
	}
}

func (h *handler) authorized(r *http.Request) bool {
	token := r.URL.Query().Get("token")
	return subtle.ConstantTimeCompare([]byte(token), []byte(h.accessToken)) == 1
}

func (h *handler) handleIntegrity(w http.ResponseWriter) {
	if !h.startCheck() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "verification is already running"})
		return
	}
	defer h.finishCheck()
	report, err := h.evaluate()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(report)
}

func (h *handler) startCheck() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.checking {
		return false
	}
	h.checking = true
	return true
}

func (h *handler) finishCheck() {
	h.mu.Lock()
	h.checking = false
	h.mu.Unlock()
}

func (h *handler) evaluate() (IntegrityReport, error) {
	checkedAt := h.now().UTC()
	verification, err := verify.BuildReport(verify.Options{
		TargetRoot: h.targetRoot,
		ProfileID:  h.profile.ProfileID,
		TargetID:   h.profile.Target.TargetID,
	})
	if err != nil {
		return IntegrityReport{}, err
	}
	extraPaths, err := verify.DetectTargetDrift(verify.DriftOptions{
		TargetRoot: h.targetRoot,
		SessionID:  verification.Manifest.SessionID,
		ProfileID:  h.profile.ProfileID,
		TargetID:   h.profile.Target.TargetID,
		Now:        checkedAt,
		// BuildReport above already checks every declared entry, including file digests.
		ExtraPathsOnly: true,
	})
	if err != nil {
		return IntegrityReport{}, err
	}
	status := StatusAligned
	assessment := "matches_latest_published_snapshot"
	if verificationNeedsReview(verification) || extraPaths.NeedsReview() {
		status = StatusReviewRequired
		assessment = "inspect_verification_and_live_extra_paths"
	}
	return IntegrityReport{
		Schema:     SchemaV1,
		CheckedAt:  checkedAt.Format(time.RFC3339Nano),
		Status:     status,
		Assessment: assessment,
		ReadOnly:   true,
		Scope: Scope{
			ProfileID:  h.profile.ProfileID,
			TargetID:   h.profile.Target.TargetID,
			TargetRoot: verification.TargetRoot,
			SessionID:  verification.Manifest.SessionID,
		},
		Verification:   verification,
		LiveExtraPaths: extraPaths,
	}, nil
}

func newAccessToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate dashboard access token: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func verificationNeedsReview(report verify.Report) bool {
	return report.Summary.ManifestCount == 0 ||
		report.Summary.ErrorFindings > 0 ||
		report.Summary.WarningFindings > 0 ||
		report.Summary.Warnings > 0 ||
		report.Summary.SoftDeletes > 0 ||
		report.Summary.TargetDrifts > 0 ||
		report.Summary.ArtifactProblems > 0
}

func validateLoopbackListen(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("dashboard listen address must be host:port: %w", err)
	}
	if strings.TrimSpace(port) == "" {
		return errors.New("dashboard listen port is required")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("dashboard listen address must use a loopback IP")
	}
	return nil
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
}

type httpErrorSink struct{}

func (httpErrorSink) Write(p []byte) (int, error) {
	return len(p), nil
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Supermover Target Verification</title>
<link rel="stylesheet" href="/assets/app.css">
</head>
<body>
<main>
<header><h1>Target Verification</h1><p>Read-only check against the latest published migration snapshot.</p></header>
<section class="notice">A full verification reads target file content. It runs once when this page opens and again only when you request it.</section>
<button id="run" type="button">Run full verification</button>
<p id="state" class="pending">Running verification...</p>
<div id="summary" class="cards"></div>
<details><summary>Raw verification evidence</summary><pre id="raw"></pre></details>
</main>
<script src="/assets/app.js"></script>
</body>
</html>`

const appJS = `(function () {
  "use strict";
  var token = new URLSearchParams(window.location.search).get("token") || "";
  var run = document.getElementById("run");
  var state = document.getElementById("state");
  var summary = document.getElementById("summary");
  var raw = document.getElementById("raw");
  function card(label, value) {
    var node = document.createElement("div");
    node.className = "card";
    var heading = document.createElement("span");
    heading.textContent = label;
    var content = document.createElement("strong");
    content.textContent = String(value);
    node.appendChild(heading);
    node.appendChild(content);
    return node;
  }
  async function check() {
    run.disabled = true;
    state.className = "pending";
    state.textContent = "Running full verification...";
    summary.replaceChildren();
    try {
      var response = await fetch("/api/integrity?token=" + encodeURIComponent(token), {cache: "no-store", headers: {"X-Supermover-Dashboard": "1"}});
      var data = await response.json();
      if (!response.ok) { throw new Error(data.error || "verification failed"); }
      state.className = data.status === "aligned" ? "aligned" : "review";
      state.textContent = data.status === "aligned" ? "Aligned with latest published snapshot" : "Review required";
      summary.appendChild(card("Session", data.scope.session_id || "-"));
      summary.appendChild(card("Expected files", data.verification.summary.files_expected));
      summary.appendChild(card("Verified files", data.verification.summary.files_verified));
      summary.appendChild(card("Extra target paths", data.live_extra_paths.summary.target_drifts));
      summary.appendChild(card("Artifact problems", data.live_extra_paths.summary.artifact_problems + data.verification.summary.artifact_problems));
      raw.textContent = JSON.stringify(data, null, 2);
    } catch (err) {
      state.className = "review";
      state.textContent = "Verification failed: " + err.message;
      raw.textContent = "";
    } finally {
      run.disabled = false;
    }
  }
  run.addEventListener("click", check);
  check();
}());`

const appCSS = `:root { color-scheme: light; font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #17212b; background: #f4f6f8; }
body { margin: 0; }
main { max-width: 880px; margin: 0 auto; padding: 48px 24px; }
h1 { margin: 0 0 8px; font-size: 32px; }
p { color: #52606d; }
.notice { margin: 24px 0; padding: 16px; border: 1px solid #ccd5dd; border-radius: 8px; background: #fff; color: #44515d; }
button { appearance: none; padding: 12px 18px; border: 0; border-radius: 7px; color: #fff; background: #1454d4; font-weight: 600; cursor: pointer; }
button:disabled { opacity: .55; cursor: wait; }
#state { margin: 24px 0; padding: 14px 16px; border-radius: 7px; font-weight: 600; }
.pending { background: #e9eef5; }
.aligned { background: #daf4e6; color: #11643c; }
.review { background: #fde5dc; color: #a23b13; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(152px, 1fr)); gap: 12px; margin-bottom: 24px; }
.card { padding: 14px; background: #fff; border: 1px solid #dfe5ea; border-radius: 8px; }
.card span { display: block; color: #52606d; font-size: 13px; margin-bottom: 8px; }
.card strong { font-size: 21px; }
details { margin-top: 28px; }
pre { padding: 14px; overflow: auto; background: #17212b; color: #e6edf3; border-radius: 8px; font-size: 12px; }`
