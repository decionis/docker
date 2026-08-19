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
	StartClaim(ctx context.Context) (*api.ClaimStart, error)
	ListPendingApprovals(ctx context.Context, limit int) ([]api.PendingApproval, error)
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

// handleClaimStart mints a claim URL for this workspace and hands it to the
// UI to open in a browser. The claim token never touches the daemon's
// store: it is short-lived, single-use, and only meaningful in the browser
// that finishes the claim.
func (d *Daemon) handleClaimStart(w http.ResponseWriter, r *http.Request) {
	client, ok := d.workspaceClient()
	if !ok {
		writeError(w, http.StatusPreconditionRequired, "not_connected", "Connect to Decionis first.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	start, err := client.StartClaim(ctx)
	if err != nil {
		if errors.Is(err, api.ErrAlreadyClaimed) {
			writeError(w, http.StatusConflict, "already_claimed",
				"This workspace already belongs to an account.")
			return
		}
		d.logger.Info("claim start failed")
		writeError(w, http.StatusBadGateway, "upstream_unreachable",
			"The control plane could not be reached; nothing changed.")
		return
	}
	d.logger.Info("claim started")
	writeJSON(w, http.StatusOK, start)
}

// handleApprovals lists what is waiting on a person.
//
// A control plane that cannot answer yields an error, never an empty list:
// "nothing is waiting" and "we could not find out" must not look the same
// in a governance surface.
func (d *Daemon) handleApprovals(w http.ResponseWriter, r *http.Request) {
	client, ok := d.workspaceClient()
	if !ok {
		writeError(w, http.StatusPreconditionRequired, "not_connected", "Connect to Decionis first.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), upstreamTimeout)
	defer cancel()

	approvals, err := client.ListPendingApprovals(ctx, 50)
	if err != nil {
		d.logger.Info("pending approvals unavailable")
		writeError(w, http.StatusBadGateway, "upstream_unreachable",
			"The pending queue could not be read.")
		return
	}
	if approvals == nil {
		approvals = []api.PendingApproval{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": approvals, "count": len(approvals)})
}
