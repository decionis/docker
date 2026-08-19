package daemon

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/store"
)

// startOneClick drives POST /api/connect/start and returns the parsed
// authorize URL plus the state the daemon minted.
func startOneClick(t *testing.T, d *Daemon, body string) (authorize *url.URL, state string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/connect/start", strings.NewReader(body))
	d.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("connect start: got %d body %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("connect start payload: %v", err)
	}
	parsed, err := url.Parse(payload.AuthorizeURL)
	if err != nil {
		t.Fatalf("authorize_url did not parse: %v", err)
	}
	return parsed, parsed.Query().Get("state")
}

func enrollRedirect(d *Daemon, token, state string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	target := "/enroll?token=" + url.QueryEscape(token) + "&state=" + url.QueryEscape(state)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	d.LoopbackHandler().ServeHTTP(rec, req)
	return rec
}

// oneClickDaemon builds a daemon whose exchange (and probe) land on a fake
// control plane; it returns the daemon, the plane's base URL, and a counter
// of exchange calls.
func oneClickDaemon(t *testing.T, exchangeStatus int, exchangeBody string) (*Daemon, string, *int) {
	t.Helper()
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/connectors/enrollments/exchange" {
			calls++
			w.WriteHeader(exchangeStatus)
			_, _ = io.WriteString(w, exchangeBody)
			return
		}
		// The post-connect probe.
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"response":{"reports":[],"summary":{}},"mode":"ENFORCEMENT"}`)
	}))
	t.Cleanup(upstream.Close)

	d := New("test", store.New(t.TempDir()), slog.New(slog.NewTextHandler(io.Discard, nil)), func(config api.Config) (HostedAPI, error) {
		return api.NewClient(config)
	})
	return d, upstream.URL, &calls
}

func TestConnectStartMintsStateAndAuthorizeURL(t *testing.T) {
	d := New("test", store.New(t.TempDir()), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	authorize, state := startOneClick(t, d, "")

	if got := authorize.Scheme; got != "https" {
		t.Fatalf("authorize scheme = %q", got)
	}
	if authorize.Host != "api.decionis.com" {
		t.Fatalf("authorize host = %q", authorize.Host)
	}
	if authorize.Path != "/v1/public/connect/docker-desktop/start" {
		t.Fatalf("authorize path = %q", authorize.Path)
	}
	if got := authorize.Query().Get("port"); got != "53719" {
		t.Fatalf("port = %q", got)
	}
	if len(state) < 40 {
		t.Fatalf("state too short: %d chars", len(state))
	}
	// Upstream validates [A-Za-z0-9_-]{16,128}.
	for _, r := range state {
		alnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if !alnum {
			t.Fatalf("state contains %q outside the upstream alphabet", r)
		}
	}
}

func TestConnectStartRejectsBadBaseURL(t *testing.T) {
	d := New("test", store.New(t.TempDir()), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/connect/start", strings.NewReader(`{"base_url":"http://evil.example"}`))
	d.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestEnrollRedirectHappyPathConnects(t *testing.T) {
	exchange := `{"org_id":"11111111-1111-4111-8111-111111111111","raw_key":"dcn_live_minted","connector_slug":"docker-desktop"}`
	d, upstreamBase, calls := oneClickDaemon(t, http.StatusOK, exchange)

	authorize, state := startOneClick(t, d, `{"base_url":"`+upstreamBase+`"}`)
	if !strings.HasPrefix(authorize.String(), upstreamBase) {
		t.Fatalf("authorize URL %q not on custom base", authorize)
	}

	rec := enrollRedirect(d, "dcn_enroll_token_abc", state)
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll redirect: got %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "connected") {
		t.Fatalf("success page missing connected copy: %s", body)
	}
	if strings.Contains(body, "dcn_enroll_token_abc") || strings.Contains(body, "dcn_live_minted") {
		t.Fatal("result page leaked token material")
	}
	if *calls != 1 {
		t.Fatalf("exchange calls = %d, want 1", *calls)
	}

	// The daemon is now connected.
	statusRec := httptest.NewRecorder()
	d.Handler().ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if !strings.Contains(statusRec.Body.String(), `"connected":true`) {
		t.Fatalf("status after one-click: %s", statusRec.Body.String())
	}
}

func TestEnrollRedirectStateIsSingleUse(t *testing.T) {
	exchange := `{"org_id":"11111111-1111-4111-8111-111111111111","raw_key":"dcn_live_minted","connector_slug":"docker-desktop"}`
	d, upstreamBase, calls := oneClickDaemon(t, http.StatusOK, exchange)

	_, state := startOneClick(t, d, `{"base_url":"`+upstreamBase+`"}`)
	first := enrollRedirect(d, "dcn_enroll_token_abc", state)
	if first.Code != http.StatusOK {
		t.Fatalf("first redirect: %d", first.Code)
	}
	replay := enrollRedirect(d, "dcn_enroll_token_abc", state)
	if replay.Code == http.StatusOK {
		t.Fatal("replayed redirect must not succeed")
	}
	if *calls != 1 {
		t.Fatalf("exchange calls after replay = %d, want 1", *calls)
	}
}

func TestEnrollRedirectRejectsWrongState(t *testing.T) {
	d, upstreamBase, calls := oneClickDaemon(t, http.StatusOK, `{}`)
	_, _ = startOneClick(t, d, `{"base_url":"`+upstreamBase+`"}`)

	rec := enrollRedirect(d, "dcn_enroll_token_abc", "attacker-supplied-state-0123456789")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong state: got %d, want 403", rec.Code)
	}
	if *calls != 0 {
		t.Fatalf("exchange must not be called on state mismatch (calls=%d)", *calls)
	}
	// The mismatch burned the pending attempt: a fresh start is required.
	again := enrollRedirect(d, "dcn_enroll_token_abc", "attacker-supplied-state-0123456789")
	if again.Code != http.StatusConflict {
		t.Fatalf("after burn: got %d, want 409", again.Code)
	}
}

func TestEnrollRedirectWithoutPendingIsRefused(t *testing.T) {
	d := New("test", store.New(t.TempDir()), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	rec := enrollRedirect(d, "dcn_enroll_token_abc", "some-state-0123456789")
	if rec.Code != http.StatusConflict {
		t.Fatalf("no pending: got %d, want 409", rec.Code)
	}
}

func TestEnrollRedirectExpiredPendingIsRefused(t *testing.T) {
	d, upstreamBase, calls := oneClickDaemon(t, http.StatusOK, `{}`)
	_, state := startOneClick(t, d, `{"base_url":"`+upstreamBase+`"}`)

	d.mu.Lock()
	d.pendingConnect.expires = time.Now().Add(-time.Minute)
	d.mu.Unlock()

	rec := enrollRedirect(d, "dcn_enroll_token_abc", state)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expired: got %d, want 409", rec.Code)
	}
	if *calls != 0 {
		t.Fatalf("exchange must not run for an expired attempt (calls=%d)", *calls)
	}
}

func TestEnrollRedirectExchangeFailureRendersRetryPage(t *testing.T) {
	d, upstreamBase, _ := oneClickDaemon(t, http.StatusNotFound, `{"error":"enrollment_invalid"}`)
	_, state := startOneClick(t, d, `{"base_url":"`+upstreamBase+`"}`)

	rec := enrollRedirect(d, "dcn_enroll_token_abc", state)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("exchange failure: got %d, want 502", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Return to Docker Desktop") {
		t.Fatalf("failure page missing retry copy: %s", rec.Body.String())
	}

	// Still disconnected.
	statusRec := httptest.NewRecorder()
	d.Handler().ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if strings.Contains(statusRec.Body.String(), `"connected":true`) {
		t.Fatal("daemon must stay disconnected after a failed exchange")
	}
}

func TestLoopbackHandlerServesNothingElse(t *testing.T) {
	d := New("test", store.New(t.TempDir()), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	for _, path := range []string{"/", "/api/status", "/api/connection", "/enroll/extra"} {
		rec := httptest.NewRecorder()
		d.LoopbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("loopback %s: got %d, want 404", path, rec.Code)
		}
	}
}

