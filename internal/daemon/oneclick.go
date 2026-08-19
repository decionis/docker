package daemon

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/store"
)

// One-click connect (RFC 8252 loopback flow). The UI asks the daemon to
// start a connect attempt; the daemon mints a random state, remembers it as
// the single pending attempt, and hands back the control plane's browser
// URL. The browser signs the user in, the control plane mints a single-use
// enrollment token and redirects to http://127.0.0.1:<port>/enroll on the
// host, which Docker Desktop publishes through to the daemon's loopback
// listener. The daemon exchanges the token exactly like a pasted one.
const (
	// oneClickLoopbackPort is fixed: the compose file publishes it on the
	// host's 127.0.0.1 and the authorize URL names it. Uncommon on purpose —
	// dev-tool defaults (3000, 8080, rclone's 53682) collide too easily.
	oneClickLoopbackPort = 53719

	oneClickPendingTTL = 10 * time.Minute
)

type pendingOneClick struct {
	state   string
	baseURL string
	expires time.Time
}

type connectStartRequest struct {
	BaseURL string `json:"base_url"`
}

// handleConnectStart mints the pending state and returns the browser URL.
func (d *Daemon) handleConnectStart(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var request connectStartRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	// io.EOF alone means an empty body, which is fine — base_url is optional.
	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Body must be JSON; base_url is optional.")
		return
	}

	baseURL, err := api.ValidateBaseURL(request.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not generate a connect state.")
		return
	}
	state := base64.RawURLEncoding.EncodeToString(buf)

	d.mu.Lock()
	// A new start replaces any prior attempt: exactly one pending at a time.
	d.pendingConnect = &pendingOneClick{
		state:   state,
		baseURL: baseURL,
		expires: time.Now().Add(oneClickPendingTTL),
	}
	d.mu.Unlock()

	authorizeURL := fmt.Sprintf(
		"%s/v1/public/connect/docker-desktop/start?port=%d&state=%s",
		baseURL, oneClickLoopbackPort, state,
	)
	d.logger.Info("one-click connect started", "base_url", baseURL)
	writeJSON(w, http.StatusOK, map[string]string{"authorize_url": authorizeURL})
}

// LoopbackHandler serves the host-published loopback listener. Only one
// route exists: the enrollment redirect. Everything else is 404.
func (d *Daemon) LoopbackHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /enroll", d.handleEnrollRedirect)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})
	return d.recoverPanics(mux)
}

// handleEnrollRedirect consumes the browser redirect. The state must match
// the single pending attempt (constant-time, single-use — it is cleared
// before the exchange so a replayed redirect can never exchange twice), and
// the token is redeemed through the exact same path as a pasted enrollment
// token. The response is a human-facing page; the token it arrived with is
// consumed by the exchange, so the copy in the browser history is dead.
func (d *Daemon) handleEnrollRedirect(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	token := strings.TrimSpace(query.Get("token"))
	state := strings.TrimSpace(query.Get("state"))

	d.mu.Lock()
	pending := d.pendingConnect
	d.pendingConnect = nil // single-use, success or not
	d.mu.Unlock()

	switch {
	case pending == nil:
		d.logger.Info("one-click redirect with no pending attempt")
		writeConnectPage(w, http.StatusConflict, "This connect attempt is no longer active.",
			"Return to Docker Desktop and press “Continue in browser” again.")
		return
	case time.Now().After(pending.expires):
		d.logger.Info("one-click redirect after expiry")
		writeConnectPage(w, http.StatusConflict, "This connect attempt expired.",
			"Return to Docker Desktop and press “Continue in browser” again.")
		return
	case token == "" || len(token) > 512 || state == "":
		writeConnectPage(w, http.StatusBadRequest, "This link is incomplete.",
			"Return to Docker Desktop and start the connect again.")
		return
	case subtle.ConstantTimeCompare([]byte(state), []byte(pending.state)) != 1:
		d.logger.Info("one-click redirect with mismatched state")
		writeConnectPage(w, http.StatusForbidden, "This link doesn’t match the pending connect.",
			"Return to Docker Desktop and start the connect again.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()
	if code := d.establishFromEnrollment(ctx, pending.baseURL, token); code != "" {
		message := "The control plane could not be reached. Return to Docker Desktop and try again."
		if code == "enrollment_invalid" || code == "enrollment_used" {
			message = "The sign-in produced a token that could not be redeemed. Return to Docker Desktop and try again."
		}
		writeConnectPage(w, http.StatusBadGateway, "Connecting didn’t finish.", message)
		return
	}

	d.logger.Info("one-click connect completed")
	writeConnectPage(w, http.StatusOK, "Docker Desktop is connected.",
		"You can close this tab and return to Docker Desktop — the Decionis feed is live.")
}

// establishFromEnrollment redeems the token, stores the minted credentials,
// and swaps the live client — the shared core of the pasted-token and
// one-click paths. It returns "" on success or a stable error code.
func (d *Daemon) establishFromEnrollment(ctx context.Context, baseURL, token string) string {
	exchange, err := exchangeEnrollment(ctx, baseURL, token)
	if err != nil {
		switch {
		case errors.Is(err, api.ErrEnrollmentInvalid):
			return "enrollment_invalid"
		case errors.Is(err, api.ErrEnrollmentUsed):
			return "enrollment_used"
		default:
			return "upstream_unreachable"
		}
	}

	connection := store.Connection{BaseURL: baseURL, OrgID: exchange.OrgID}
	if err := d.store.Save(connection, exchange.RawKey); err != nil {
		d.logger.Error("connection save failed", "detail", "storage error")
		return "storage_failed"
	}

	client, err := d.newClient(api.Config{BaseURL: baseURL, OrgID: exchange.OrgID, APIKey: exchange.RawKey})
	if err != nil {
		return "internal_error"
	}

	d.mu.Lock()
	d.client = client
	d.connection = connection
	d.cache = nil
	d.lastError = ""
	d.mu.Unlock()

	// Best-effort probe: sets freshness/status, never discards minted creds.
	if _, err := client.ListReports(ctx, "ENFORCEMENT", 1); err != nil {
		code := "upstream_unreachable"
		if api.IsAuthError(err) {
			code = "unauthorized"
		}
		d.mu.Lock()
		d.lastError = code
		d.mu.Unlock()
	} else {
		now := time.Now()
		d.mu.Lock()
		d.lastSync = now
		d.mu.Unlock()
	}

	d.logger.Info("connected via enrollment", "org_id", exchange.OrgID, "connector", exchange.ConnectorSlug)
	return ""
}

// writeConnectPage renders the tiny human-facing result page. No external
// resources, no scripts, and never any token or credential material.
func writeConnectPage(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w,
		`<!doctype html><html><head><meta charset="utf-8"><title>Decionis</title></head><body style="font-family:system-ui,sans-serif;max-width:480px;margin:80px auto;padding:0 24px;color:#1c1c1e"><h2>%s</h2><p style="color:#52525b;line-height:1.6">%s</p></body></html>`,
		title, detail,
	)
}
