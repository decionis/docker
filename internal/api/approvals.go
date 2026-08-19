package api

import (
	"context"
	"net/url"
	"strconv"
)

// PendingApproval is one decision still waiting on a person: an outcome the
// policy did not approve, with nothing settled against it yet. Fields are
// rendered as the protocol returns them — nothing here re-labels an
// ESCALATE as a REVIEW, or re-scores anything.
type PendingApproval struct {
	EvaluationID   string  `json:"evaluation_id"`
	DecisionType   string  `json:"decision_type"`
	Outcome        string  `json:"outcome"`
	Mode           string  `json:"mode"`
	PolicyVersion  string  `json:"policy_version"`
	Amount         *string `json:"amount"`
	Channel        *string `json:"channel"`
	DossierID      *string `json:"dossier_id"`
	CreatedAt      string  `json:"created_at"`
	OverrideStatus *string `json:"override_status"`
}

type awaitingApprovalResponse struct {
	OrgID    string            `json:"org_id"`
	Count    int               `json:"count"`
	Awaiting []PendingApproval `json:"awaiting"`
}

// ListPendingApprovals reads the decisions awaiting a person
// (GET /v1/protocol/decisions/awaiting-approval). The extension's scoped key
// carries `decision:read`, which is what this route requires.
func (c *Client) ListPendingApprovals(ctx context.Context, limit int) ([]PendingApproval, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := url.Values{}
	query.Set("org_id", c.config.OrgID)
	query.Set("limit", strconv.Itoa(limit))
	requestURL := c.config.BaseURL + "/v1/protocol/decisions/awaiting-approval?" + query.Encode()

	var response awaitingApprovalResponse
	if err := c.getJSON(ctx, "awaiting approval", requestURL, 1<<20, &response); err != nil {
		return nil, err
	}
	return response.Awaiting, nil
}
