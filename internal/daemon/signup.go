package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/store"
)

// publicAPI is the pre-auth surface: the calls made before any credential
// exists. It takes no URLs — the client it comes from was built with a
// validated base URL, so a raw caller-supplied string never reaches
// request construction.
type publicAPI interface {
	ProvisionWorkspace(ctx context.Context, dockerUsername, installationID string) (*api.Workspace, error)
	ConnectWithAccount(ctx context.Context, email, password string) (*api.Workspace, error)
	ExchangeEnrollment(ctx context.Context, enrollmentToken string) (*api.EnrollmentExchange, error)
}

// newPublicClient validates the base URL and returns the pre-auth surface.
// Swapped in tests.
var newPublicClient = func(baseURL string) (publicAPI, error) {
	return api.NewPublicClient(baseURL)
}

type autoConnectRequest struct {
	BaseURL        string `json:"base_url"`
	DockerUsername string `json:"docker_username"`
}

// autoConnectPayload is the status payload plus what only an automatic
// signup can learn. A fresh mint sets none of the extras, so its JSON is
// byte-for-byte the plain status an older UI already understands.
type autoConnectPayload struct {
	statusPayload
	// Reused: this install's existing workspace came back with a fresh key
	// (OrgName names it for the UI's notice).
	Reused  bool   `json:"reused,omitempty"`
	OrgName string `json:"org_name,omitempty"`
	// Claimed: the workspace now belongs to an account, so nothing was
	// minted or stored — the UI offers sign-in, with SignInURL as the plain
	// browser fallback.
	Claimed   bool   `json:"claimed,omitempty"`
	SignInURL string `json:"sign_in_url,omitempty"`
}

// handleAutoConnect is the extension's first run: no account, no token, no
// typing. The control plane mints a workspace and a scoped key, and the
// daemon stores them exactly as it stores an exchanged enrollment.
//
// Every call sends the install's persisted installation id, so a repeat run
// (data volume survived, credentials didn't — or the user disconnected)
// resolves to the workspace this install already minted instead of minting
// another. It is deliberately not idempotent-by-accident: an
// already-connected daemon refuses, so a stray call can never replace a
// working connection (or a claimed workspace) with a fresh empty one.
func (d *Daemon) handleAutoConnect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var request autoConnectRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Body must be JSON; every field is optional.")
		return
	}

	d.mu.Lock()
	connected := d.client != nil
	d.mu.Unlock()
	if connected {
		writeError(w, http.StatusConflict, "already_connected",
			"This extension is already connected. Disconnect first to create a new workspace.")
		return
	}

	client, err := newPublicClient(request.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	baseURL := clientBaseURL(client, request.BaseURL)

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	workspace, err := client.ProvisionWorkspace(ctx, request.DockerUsername, d.ensureInstallationID())
	if err != nil {
		var claimed *api.WorkspaceClaimedError
		if errors.As(err, &claimed) {
			// Not a failure: the workspace exists and has an owner. Nothing
			// is stored — possession of the installation id must not mint
			// credentials for an account — and the UI switches to sign-in.
			d.mu.Lock()
			payload := autoConnectPayload{statusPayload: d.statusLocked()}
			d.mu.Unlock()
			payload.Claimed = true
			payload.SignInURL = browserSafeURL(claimed.SignInURL)
			d.logger.Info("automatic signup found a claimed workspace", "org_id", claimed.OrgID)
			writeJSON(w, http.StatusOK, payload)
			return
		}
		d.logger.Info("automatic signup failed")
		writeError(w, http.StatusBadGateway, "signup_unavailable",
			"Decionis could not create a workspace right now. Try again, or connect an existing account under Advanced.")
		return
	}

	if code := d.storeWorkspace(ctx, baseURL, workspace); code != "" {
		writeError(w, http.StatusInternalServerError, code, "The new workspace could not be stored.")
		return
	}

	d.mu.Lock()
	payload := autoConnectPayload{statusPayload: d.statusLocked()}
	d.mu.Unlock()
	if workspace.Reused {
		// The control plane revoked every earlier key for this org when it
		// minted the fresh one; the store above already holds the fresh key.
		payload.Reused = true
		payload.OrgName = workspace.OrgName
		d.logger.Info("reconnected to existing workspace", "org_id", workspace.OrgID, "provisional", workspace.Provisional)
	} else {
		d.logger.Info("connected via automatic signup", "org_id", workspace.OrgID, "provisional", workspace.Provisional)
	}
	writeJSON(w, http.StatusOK, payload)
}

// ensureInstallationID returns this install's installation id, minting and
// persisting one on first use. The id is a possession secret
// (rules/security.rules.md Rule 2.4): it lives only in the daemon's private
// data volume — the same store as the org API key — and is never logged or
// sent to the UI. A store that cannot persist it degrades to today's
// behavior (a fresh workspace per provision), never to a failure.
func (d *Daemon) ensureInstallationID() string {
	existing, err := d.store.LoadInstallationID()
	if err != nil {
		d.logger.Error("installation id unreadable", "detail", "storage error")
	}
	if existing != "" {
		return existing
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// No CSPRNG output means no id at all — a predictable id would be a
		// forgeable possession secret.
		d.logger.Error("installation id generation failed")
		return ""
	}
	minted := "dcn_install_" + hex.EncodeToString(buf)
	if err := d.store.SaveInstallationID(minted); err != nil {
		d.logger.Error("installation id save failed", "detail", "storage error")
	}
	return minted
}

// browserSafeURL passes through a URL only when it is safe to hand to the
// system browser: https anywhere, or plain http on loopback (development
// control planes). Anything else is dropped — the UI still offers its
// in-app sign-in paths.
func browserSafeURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	switch parsed.Scheme {
	case "https":
		return parsed.String()
	case "http":
		host := parsed.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return parsed.String()
		}
	}
	return ""
}

// connectWithCredentials resolves an account's email and password at the
// control plane and stores the workspace it names. The password lives only
// in this request: it is never written to the store, never logged, and never
// returned to the UI.
func (d *Daemon) connectWithCredentials(w http.ResponseWriter, r *http.Request, request connectRequest) {
	client, err := newPublicClient(request.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	baseURL := clientBaseURL(client, request.BaseURL)
	if strings.TrimSpace(request.Email) == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Both email and password are required.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	workspace, err := client.ConnectWithAccount(ctx, request.Email, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, api.ErrCredentialsInvalid):
			d.logger.Info("account connect rejected")
			writeError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
		case errors.Is(err, api.ErrCredentialsLocked):
			writeError(w, http.StatusTooManyRequests, "account_locked", err.Error())
		case errors.Is(err, api.ErrNoWorkspace):
			writeError(w, http.StatusForbidden, "no_workspace", err.Error())
		default:
			d.logger.Info("account connect failed")
			writeError(w, http.StatusBadGateway, "upstream_unreachable",
				"The control plane could not be reached; nothing was saved.")
		}
		return
	}

	if code := d.storeWorkspace(ctx, baseURL, workspace); code != "" {
		writeError(w, http.StatusInternalServerError, code, "The minted credentials could not be stored.")
		return
	}

	d.mu.Lock()
	status := d.statusLocked()
	d.mu.Unlock()
	d.logger.Info("connected via account sign-in", "org_id", workspace.OrgID)
	writeJSON(w, http.StatusOK, status)
}

// storeWorkspace persists a minted workspace and swaps in the live client,
// then probes for freshness. Returns "" on success or a stable error code.
func (d *Daemon) storeWorkspace(ctx context.Context, baseURL string, workspace *api.Workspace) string {
	connection := store.Connection{BaseURL: baseURL, OrgID: workspace.OrgID}
	if err := d.store.Save(connection, workspace.RawKey); err != nil {
		d.logger.Error("connection save failed", "detail", "storage error")
		return "storage_failed"
	}

	client, err := d.newClient(api.Config{
		BaseURL: baseURL,
		OrgID:   workspace.OrgID,
		APIKey:  workspace.RawKey,
	})
	if err != nil {
		return "internal_error"
	}

	d.mu.Lock()
	d.client = client
	d.connection = connection
	d.cache = nil
	d.lastError = ""
	d.mu.Unlock()

	// Best-effort probe: sets freshness, never discards minted credentials.
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
	return ""
}

// clientBaseURL reports the normalized destination for storage. It prefers
// the client's own validated value and falls back to re-validating the raw
// input, so what is stored is always the normalized form.
func clientBaseURL(client publicAPI, raw string) string {
	if real, ok := client.(*api.Client); ok {
		return real.BaseURL()
	}
	normalized, err := api.ValidateBaseURL(raw)
	if err != nil {
		return api.DefaultBaseURL
	}
	return normalized
}
