package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/decionis/docker/internal/api"
)

// WorkspaceReader is the slice of the hosted API this file needs. Kept
// separate from HostedAPI so existing fakes stay valid.
type WorkspaceReader interface {
	GetWorkspaceState(ctx context.Context) (*api.WorkspaceState, error)
	SetEnforcement(ctx context.Context, enabled bool) (*api.WorkspaceState, error)
}

type enforcementRequest struct {
	Enabled bool `json:"enabled"`
}

// handleWorkspace reports enforcement state and the free governed-decision
// allowance. A daemon with no connection has nothing to report.
func (d *Daemon) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	client, ok := d.workspaceClient()
	if !ok {
		writeError(w, http.StatusPreconditionRequired, "not_connected", "Connect to Decionis first.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	state, err := client.GetWorkspaceState(ctx)
	if err != nil {
		d.logger.Info("workspace state unavailable")
		writeError(w, http.StatusBadGateway, "upstream_unreachable",
			"The control plane could not be reached.")
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// handleEnforcement turns enforcement on or off. A refusal because the free
// allowance is spent is passed through as its own code so the UI can offer
// to subscribe instead of showing a generic failure.
func (d *Daemon) handleEnforcement(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var request enforcementRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Body must be JSON with enabled.")
		return
	}

	client, ok := d.workspaceClient()
	if !ok {
		writeError(w, http.StatusPreconditionRequired, "not_connected", "Connect to Decionis first.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	state, err := client.SetEnforcement(ctx, request.Enabled)
	if err != nil {
		if errors.Is(err, api.ErrEnforcementLimitReached) {
			writeError(w, http.StatusPaymentRequired, "governed_limit_reached",
				"This workspace has used its free governed decisions. Subscribe to keep enforcing.")
			return
		}
		d.logger.Info("enforcement change failed", "enabled", request.Enabled)
		writeError(w, http.StatusBadGateway, "upstream_unreachable",
			"The control plane could not be reached; nothing changed.")
		return
	}
	d.logger.Info("enforcement changed", "enabled", state.EnforcementEnabled)
	writeJSON(w, http.StatusOK, state)
}

// workspaceClient returns the live client when it can also read workspace
// state. Fails closed: no connection, or a client that predates this
// capability, reports unavailable rather than guessing.
func (d *Daemon) workspaceClient() (WorkspaceReader, bool) {
	d.mu.Lock()
	client := d.client
	d.mu.Unlock()
	if client == nil {
		return nil, false
	}
	reader, ok := client.(WorkspaceReader)
	return reader, ok
}
