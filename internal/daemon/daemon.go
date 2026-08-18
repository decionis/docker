// Package daemon is the Decionis Docker daemon: the Docker Desktop
// extension's backend. It serves the extension UI over the extension socket,
// holds the org API key (the UI never does — rules/security.rules.md Rules
// 2.2–2.3), proxies the published hosted-API reads with bounded inputs and
// finite timeouts, and verifies Decision Dossier proof bundles offline.
//
// Fail closed everywhere (Rule 0.3): an unreachable control plane, a
// malformed response, or a missing JWKS is an error state or an unverified
// dossier — never a silent success.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/decionis/docker/internal/api"
	"github.com/decionis/docker/internal/dossier"
	"github.com/decionis/docker/internal/store"
)

// maxRequestBody bounds every UI request body (rules/security.rules.md 3.3).
const maxRequestBody = 100 << 10

// decisionsCacheTTL is how long a reports fetch is served without re-hitting
// the control plane.
const decisionsCacheTTL = 10 * time.Second

const upstreamTimeout = 15 * time.Second

var validModes = map[string]bool{"SHADOW": true, "PARALLEL": true, "ENFORCEMENT": true}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// HostedAPI is the surface the daemon needs from the hosted control plane.
// *api.Client implements it; tests substitute fakes.
type HostedAPI interface {
	ListReports(ctx context.Context, mode string, limit int) (*api.ReportsResponse, error)
	GetDossier(ctx context.Context, dossierID string) (map[string]any, string, error)
	FetchJwks(ctx context.Context, jwksURL string) (*dossier.Jwks, error)
}

// ClientFactory builds a hosted-API client from a connection config.
type ClientFactory func(config api.Config) (HostedAPI, error)

// DefaultClientFactory uses the real hosted-API client.
func DefaultClientFactory(config api.Config) (HostedAPI, error) {
	return api.NewClient(config)
}

type decisionsCache struct {
	mode      string
	limit     int
	fetchedAt time.Time
	response  *api.ReportsResponse
}

// Daemon carries the daemon's state. Construct with New.
type Daemon struct {
	version   string
	store     *store.Store
	logger    *slog.Logger
	newClient ClientFactory

	mu         sync.Mutex
	client     HostedAPI
	connection store.Connection
	lastSync   time.Time
	lastError  string
	cache      *decisionsCache
}

// New builds a daemon. The logger must never receive credentials; nothing in
// this package hands it any.
func New(version string, st *store.Store, logger *slog.Logger, factory ClientFactory) *Daemon {
	if factory == nil {
		factory = DefaultClientFactory
	}
	return &Daemon{version: version, store: st, logger: logger, newClient: factory}
}

// LoadStoredConnection restores a persisted connection at startup. Failures
// leave the daemon disconnected (fail closed) and are logged without secrets.
func (d *Daemon) LoadStoredConnection() {
	connection, apiKey, ok, err := d.store.Load()
	if err != nil {
		d.logger.Error("stored connection unreadable", "detail", err.Error())
		return
	}
	if !ok {
		return
	}
	client, err := d.newClient(api.Config{BaseURL: connection.BaseURL, OrgID: connection.OrgID, APIKey: apiKey})
	if err != nil {
		d.logger.Error("stored connection invalid", "detail", err.Error())
		return
	}
	d.mu.Lock()
	d.client = client
	d.connection = connection
	d.mu.Unlock()
	d.logger.Info("connection restored", "base_url", connection.BaseURL, "org_id", connection.OrgID)
}

// Handler returns the daemon's HTTP handler.
func (d *Daemon) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/version", d.handleVersion)
	mux.HandleFunc("GET /api/status", d.handleStatus)
	mux.HandleFunc("PUT /api/connection", d.handleConnect)
	mux.HandleFunc("DELETE /api/connection", d.handleDisconnect)
	mux.HandleFunc("GET /api/decisions", d.handleDecisions)
	mux.HandleFunc("GET /api/dossiers/{id}", d.handleDossier)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Unknown route.")
	})
	return d.recoverPanics(mux)
}

func (d *Daemon) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				// Generic on purpose: no panic values, stacks, or bodies
				// in responses or logs (rules/security.rules.md 3.6).
				d.logger.Error("panic recovered", "route", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal_error", "Internal error.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

type statusPayload struct {
	DaemonVersion string  `json:"daemon_version"`
	Connected     bool    `json:"connected"`
	BaseURL       string  `json:"base_url,omitempty"`
	OrgID         string  `json:"org_id,omitempty"`
	LastSync      *string `json:"last_sync"`
	LastError     *string `json:"last_error"`
}

func (d *Daemon) statusLocked() statusPayload {
	payload := statusPayload{DaemonVersion: d.version, Connected: d.client != nil}
	if d.client != nil {
		payload.BaseURL = d.connection.BaseURL
		payload.OrgID = d.connection.OrgID
	}
	if !d.lastSync.IsZero() {
		formatted := d.lastSync.UTC().Format(time.RFC3339)
		payload.LastSync = &formatted
	}
	if d.lastError != "" {
		lastError := d.lastError
		payload.LastError = &lastError
	}
	return payload
}

func (d *Daemon) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": d.version})
}

func (d *Daemon) handleStatus(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	writeJSON(w, http.StatusOK, d.statusLocked())
}

type connectRequest struct {
	BaseURL string `json:"base_url"`
	OrgID   string `json:"org_id"`
	APIKey  string `json:"api_key"`
	// EnrollmentToken is the one-paste alternative: a single-use
	// dcn_enroll_* token exchanged at the control plane for the org id and a
	// freshly minted scoped key (the connector self-provisioning mechanism).
	EnrollmentToken string `json:"enrollment_token"`
}

// exchangeEnrollment is swapped in tests.
var exchangeEnrollment = api.ExchangeEnrollment

func (d *Daemon) handleConnect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request connectRequest
	if err := decoder.Decode(&request); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body exceeds the 100 KB limit.")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "Body must be JSON with base_url, org_id, api_key.")
		return
	}
	if strings.TrimSpace(request.EnrollmentToken) != "" {
		if strings.TrimSpace(request.OrgID) != "" || strings.TrimSpace(request.APIKey) != "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "Provide either enrollment_token or org_id + api_key, not both.")
			return
		}
		d.connectViaEnrollment(w, r, request)
		return
	}
	if !uuidPattern.MatchString(strings.TrimSpace(request.OrgID)) {
		writeError(w, http.StatusBadRequest, "invalid_request", "org_id must be a UUID.")
		return
	}
	if strings.TrimSpace(request.APIKey) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "api_key is required.")
		return
	}

	config := api.Config{BaseURL: request.BaseURL, OrgID: strings.TrimSpace(request.OrgID), APIKey: request.APIKey}
	client, err := d.newClient(config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Prove the credentials against the control plane before storing anything.
	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()
	if _, err := client.ListReports(ctx, "ENFORCEMENT", 1); err != nil {
		if api.IsAuthError(err) {
			d.logger.Info("connection rejected by control plane", "org_id", config.OrgID)
			writeError(w, http.StatusUnauthorized, "unauthorized", "The control plane rejected the credentials.")
			return
		}
		d.logger.Info("connection probe failed", "org_id", config.OrgID)
		writeError(w, http.StatusBadGateway, "upstream_unreachable", "The control plane could not be reached; nothing was saved.")
		return
	}

	connection := store.Connection{BaseURL: strings.TrimRight(strings.TrimSpace(request.BaseURL), "/"), OrgID: config.OrgID}
	if connection.BaseURL == "" {
		connection.BaseURL = api.DefaultBaseURL
	}
	if err := d.store.Save(connection, request.APIKey); err != nil {
		d.logger.Error("connection save failed", "detail", "storage error")
		writeError(w, http.StatusInternalServerError, "storage_failed", "The connection could not be stored.")
		return
	}

	d.mu.Lock()
	d.client = client
	d.connection = connection
	d.cache = nil
	d.lastError = ""
	status := d.statusLocked()
	d.mu.Unlock()
	d.logger.Info("connected", "base_url", connection.BaseURL, "org_id", connection.OrgID)
	writeJSON(w, http.StatusOK, status)
}

// connectViaEnrollment redeems a single-use enrollment token: the control
// plane returns the org id and a freshly minted scoped key (the same
// self-provisioning mechanism Decionis connectors use). Unlike the manual
// path, credentials are stored as soon as the exchange succeeds — they are
// server-minted (no typo risk) and the token is consumed by the exchange, so
// discarding them on a transient probe failure would strand the user. The
// follow-up probe only sets status.
func (d *Daemon) connectViaEnrollment(w http.ResponseWriter, r *http.Request, request connectRequest) {
	// The base URL is user input; it passes the transport-policy gate here at
	// the boundary and only the normalized result reaches the network layer.
	baseURL, err := api.ValidateBaseURL(request.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	exchange, err := exchangeEnrollment(ctx, baseURL, request.EnrollmentToken)
	if err != nil {
		switch {
		case errors.Is(err, api.ErrEnrollmentInvalid):
			d.logger.Info("enrollment exchange rejected")
			writeError(w, http.StatusUnauthorized, "enrollment_invalid", "The enrollment token is invalid or expired.")
		case errors.Is(err, api.ErrEnrollmentUsed):
			writeError(w, http.StatusConflict, "enrollment_used", "This enrollment token has already been exchanged — mint a new one.")
		default:
			d.logger.Info("enrollment exchange failed")
			writeError(w, http.StatusBadGateway, "upstream_unreachable",
				"The control plane could not be reached. If a retry reports the token as already exchanged, mint a new one.")
		}
		return
	}

	connection := store.Connection{BaseURL: baseURL, OrgID: exchange.OrgID}
	if err := d.store.Save(connection, exchange.RawKey); err != nil {
		d.logger.Error("connection save failed", "detail", "storage error")
		writeError(w, http.StatusInternalServerError, "storage_failed", "The minted credentials could not be stored.")
		return
	}

	client, err := d.newClient(api.Config{BaseURL: connection.BaseURL, OrgID: exchange.OrgID, APIKey: exchange.RawKey})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "The minted credentials could not be loaded.")
		return
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

	d.mu.Lock()
	status := d.statusLocked()
	d.mu.Unlock()
	d.logger.Info("connected via enrollment", "org_id", exchange.OrgID, "connector", exchange.ConnectorSlug)
	writeJSON(w, http.StatusOK, status)
}

func (d *Daemon) handleDisconnect(w http.ResponseWriter, _ *http.Request) {
	if err := d.store.Clear(); err != nil {
		writeError(w, http.StatusInternalServerError, "storage_failed", "Stored connection could not be removed.")
		return
	}
	d.mu.Lock()
	d.client = nil
	d.connection = store.Connection{}
	d.cache = nil
	d.lastSync = time.Time{}
	d.lastError = ""
	d.mu.Unlock()
	d.logger.Info("disconnected")
	w.WriteHeader(http.StatusNoContent)
}

type decisionsPayload struct {
	FetchedAt string               `json:"fetched_at"`
	Response  *api.ReportsResponse `json:"response"`
}

func (d *Daemon) handleDecisions(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "ENFORCEMENT"
	}
	if !validModes[mode] {
		writeError(w, http.StatusBadRequest, "invalid_request", "mode must be SHADOW, PARALLEL, or ENFORCEMENT.")
		return
	}
	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 500 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be an integer between 1 and 500.")
			return
		}
		limit = parsed
	}

	d.mu.Lock()
	client := d.client
	cache := d.cache
	d.mu.Unlock()
	if client == nil {
		writeError(w, http.StatusConflict, "not_connected", "Connect an org first.")
		return
	}
	if cache != nil && cache.mode == mode && cache.limit == limit && time.Since(cache.fetchedAt) < decisionsCacheTTL {
		writeJSON(w, http.StatusOK, decisionsPayload{FetchedAt: cache.fetchedAt.UTC().Format(time.RFC3339), Response: cache.response})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()
	response, err := client.ListReports(ctx, mode, limit)
	if err != nil {
		code, status := "upstream_unreachable", http.StatusBadGateway
		message := "The control plane could not be reached."
		if api.IsAuthError(err) {
			code, status = "unauthorized", http.StatusUnauthorized
			message = "The control plane rejected the stored credentials."
		}
		d.mu.Lock()
		d.lastError = code
		d.mu.Unlock()
		d.logger.Info("decisions fetch failed", "code", code)
		writeError(w, status, code, message)
		return
	}

	now := time.Now()
	d.mu.Lock()
	d.cache = &decisionsCache{mode: mode, limit: limit, fetchedAt: now, response: response}
	d.lastSync = now
	d.lastError = ""
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, decisionsPayload{FetchedAt: now.UTC().Format(time.RFC3339), Response: response})
}

type dossierPayload struct {
	DossierID       string                            `json:"dossier_id"`
	JwksURL         string                            `json:"jwks_url,omitempty"`
	Verification    dossier.VerifyResult              `json:"verification"`
	Reproducibility dossier.ReproducibilityAssessment `json:"reproducibility"`
	Payload         map[string]any                    `json:"payload"`
}

func (d *Daemon) handleDossier(w http.ResponseWriter, r *http.Request) {
	dossierID := r.PathValue("id")
	if dossierID == "" || len(dossierID) > 200 {
		writeError(w, http.StatusBadRequest, "invalid_request", "dossier id is required.")
		return
	}
	d.mu.Lock()
	client := d.client
	d.mu.Unlock()
	if client == nil {
		writeError(w, http.StatusConflict, "not_connected", "Connect an org first.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()
	payload, servedFrom, err := client.GetDossier(ctx, dossierID)
	if err != nil {
		if api.IsAuthError(err) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "The control plane rejected the stored credentials.")
			return
		}
		var statusErr *api.StatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
			writeError(w, http.StatusNotFound, "dossier_not_found", "No dossier with that id is visible to this org.")
			return
		}
		writeError(w, http.StatusBadGateway, "upstream_unreachable", "The dossier could not be fetched.")
		return
	}

	jwksURL := dossier.ResolveJwksURL(payload, servedFrom)
	var verification dossier.VerifyResult
	if jwksURL == "" {
		verification = dossier.VerifyProofBundle(payload, nil)
	} else if jwks, jwksErr := client.FetchJwks(ctx, jwksURL); jwksErr != nil {
		// Fail closed: no JWKS means unverified, stated as such — never a
		// guess (rules/security.rules.md Rule 0.3).
		verification = dossier.VerifyResult{
			Verified:  false,
			Available: true,
			Checks: []dossier.VerifyCheck{{
				Key:      "jwks",
				Label:    "Public JWKS",
				Verified: false,
				Severity: "fail",
				Detail:   "The public JWKS could not be fetched; signature verification was not performed.",
			}},
		}
	} else {
		verification = dossier.VerifyProofBundle(payload, jwks)
	}

	writeJSON(w, http.StatusOK, dossierPayload{
		DossierID:       dossierID,
		JwksURL:         jwksURL,
		Verification:    verification,
		Reproducibility: dossier.AssessReproducibility(payload),
		Payload:         payload,
	})
}
