package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/store"
)

// Swapped in tests.
var (
	provisionWorkspace = api.ProvisionWorkspace
	connectWithAccount = api.ConnectWithAccount
)

type autoConnectRequest struct {
	BaseURL        string `json:"base_url"`
	DockerUsername string `json:"docker_username"`
}

// handleAutoConnect is the extension's first run: no account, no token, no
// typing. The control plane mints a workspace and a scoped key, and the
// daemon stores them exactly as it stores an exchanged enrollment.
//
// It is deliberately not idempotent-by-accident: an already-connected daemon
// refuses, so a stray call can never replace a working connection (or a
// claimed workspace) with a fresh empty one.
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

	baseURL, err := api.ValidateBaseURL(request.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	workspace, err := provisionWorkspace(ctx, baseURL, request.DockerUsername)
	if err != nil {
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
	status := d.statusLocked()
	d.mu.Unlock()
	d.logger.Info("connected via automatic signup", "org_id", workspace.OrgID, "provisional", workspace.Provisional)
	writeJSON(w, http.StatusOK, status)
}

// connectWithCredentials resolves an account's email and password at the
// control plane and stores the workspace it names. The password lives only
// in this request: it is never written to the store, never logged, and never
// returned to the UI.
func (d *Daemon) connectWithCredentials(w http.ResponseWriter, r *http.Request, request connectRequest) {
	baseURL, err := api.ValidateBaseURL(request.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(request.Email) == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Both email and password are required.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	workspace, err := connectWithAccount(ctx, baseURL, request.Email, request.Password)
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
