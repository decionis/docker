package api

import (
	"context"
	"net/url"
	"strconv"
)

// PendingApproval is one entry awaiting a person's attention, as published
// by the control plane's signal queue. Fields are rendered as returned;
// nothing here re-scores or re-labels them.
type PendingApproval struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Severity       string `json:"severity"`
	DecisionDomain string `json:"decision_domain"`
	TriggerReason  string `json:"trigger_reason"`
	DecisionID     string `json:"decision_id"`
	SurfacedAt     string `json:"surfaced_at"`
	ExpiresAt      string `json:"expires_at"`
}

// ListPendingApprovals reads the org's queue of entries still awaiting
// review (GET /v1/signals/queue). The extension's scoped key carries
// `signals:read`, which is what this route requires.
func (c *Client) ListPendingApprovals(ctx context.Context, limit int) ([]PendingApproval, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := url.Values{}
	query.Set("org_id", c.config.OrgID)
	query.Set("status", "pending")
	query.Set("limit", strconv.Itoa(limit))
	requestURL := c.config.BaseURL + "/v1/signals/queue?" + query.Encode()

	var approvals []PendingApproval
	if err := c.getJSON(ctx, "signal queue", requestURL, 1<<20, &approvals); err != nil {
		return nil, err
	}
	return approvals, nil
}
